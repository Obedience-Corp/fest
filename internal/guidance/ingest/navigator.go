// Package ingest provides the IngestNavigator for navigating
// festival ingest phases with loop mechanics.
package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func init() {
	guidance.RegisterNavigator(guidance.ModeIngest, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx)
	})
}

// Navigator implements guidance.Navigator for ingest phases.
// It guides users through the festival ingest workflow with
// support for user-approved loops when clarification is needed.
//
// When a WORKFLOW.md file exists in the phase directory, the Navigator
// delegates to a WorkflowNavigator for step-based guidance. Otherwise,
// it uses the default ingest steps.
type Navigator struct {
	*guidance.BaseNavigator
	steps       []IngestStep
	currentStep int
	loopState   *LoopState

	// workflowNav is used when WORKFLOW.md exists in the phase directory
	workflowNav   *workflow.Navigator
	useWorkflowMd bool
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// IngestStep defines a step in the ingest workflow.
type IngestStep struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Instructions []string               `json:"instructions"`
	OutputFile   string                 `json:"output_file"` // File to create in output/
	Checklist    []string               `json:"checklist"`
	Autonomy     guidance.AutonomyLevel `json:"autonomy"`
	Order        int                    `json:"order"`
}

// LoopState tracks the user-approved iteration loop.
type LoopState struct {
	IterationCount  int    `yaml:"iteration_count" json:"iteration_count"`
	CurrentStep     int    `yaml:"current_step" json:"current_step"`
	PendingApproval bool   `yaml:"pending_approval" json:"pending_approval"`
	Approved        bool   `yaml:"approved" json:"approved"`
	LastGatePassed  bool   `yaml:"last_gate_passed" json:"last_gate_passed"`
	RejectReason    string `yaml:"reject_reason,omitempty" json:"reject_reason,omitempty"`
	MaxIterations   int    `yaml:"max_iterations" json:"max_iterations"`
}

// DefaultIngestSteps returns the standard ingest workflow.
// Ingest acts as a "compiler front-end" - normalizing raw input into planning-ready IR.
func DefaultIngestSteps() []IngestStep {
	return []IngestStep{
		{
			ID:          "distill_intent",
			Title:       "Distill Core Intent",
			Description: "Extract the primary intent from raw input materials",
			Instructions: []string{
				"Read ALL files in input/ directory",
				"Ignore proposed solutions unless they materially constrain intent",
				"Rewrite intent in clear, unambiguous language",
				"Prefer declarative statements over narratives",
				"Do NOT include implementation details",
			},
			OutputFile: "intent_packet.md",
			Checklist: []string{
				"Single primary intent identified",
				"Secondary intents (if any) listed explicitly",
				"No implementation details included",
				"Language is precise and testable",
			},
			Autonomy: guidance.AutonomyMedium,
			Order:    1,
		},
		{
			ID:          "identify_constraints",
			Title:       "Identify Constraints",
			Description: "Enumerate all constraints that bound the solution space",
			Instructions: []string{
				"Extract constraints from input material",
				"Include both explicit and implied constraints",
				"Categorize where useful (technical, organizational, temporal, legal)",
				"Separate hard constraints from soft preferences",
				"Note any conflicting constraints",
			},
			OutputFile: "constraints.md",
			Checklist: []string{
				"Hard constraints listed",
				"Soft preferences separated from constraints",
				"Conflicting constraints noted",
				"Source of each constraint is identifiable",
			},
			Autonomy: guidance.AutonomyMedium,
			Order:    2,
		},
		{
			ID:          "define_scope",
			Title:       "Define Scope",
			Description: "Establish clear boundaries for what is and isn't included",
			Instructions: []string{
				"Review intent_packet.md and constraints.md",
				"Define explicit scope boundaries",
				"List what is in scope",
				"List what is explicitly out of scope",
				"Document any scope assumptions",
			},
			OutputFile: "scope.md",
			Checklist: []string{
				"Scope boundaries are explicit",
				"In-scope items listed",
				"Out-of-scope items listed",
				"Assumptions documented",
			},
			Autonomy: guidance.AutonomyMedium,
			Order:    3,
		},
		{
			ID:          "surface_unknowns",
			Title:       "Surface Unknowns and Risks",
			Description: "Identify gaps, ambiguities, and risks that could block planning",
			Instructions: []string{
				"Review all files in output/",
				"List unanswered questions explicitly",
				"Distinguish blocking unknowns from deferrable ones",
				"Identify major risks introduced by uncertainty",
				"Do NOT propose speculative solutions",
			},
			OutputFile: "risks_and_unknowns.md",
			Checklist: []string{
				"Blocking unknowns identified",
				"Deferred unknowns labeled",
				"Major risks articulated",
				"No speculative solutions proposed",
			},
			Autonomy: guidance.AutonomyMedium,
			Order:    4,
		},
		{
			ID:          "ingest_gate",
			Title:       "Validate Ingest Output",
			Description: "Determine if output is sufficient to proceed to planning",
			Instructions: []string{
				"Ask: Could a planning agent plan using ONLY the output/ directory?",
				"Verify intent is explicit and singular",
				"Verify constraints meaningfully bound the solution space",
				"Verify scope is neither trivial nor unconstrained",
				"Verify blocking unknowns are explicitly identified",
				"If YES to all, mark as approved",
				"If NO, trigger another ingest iteration",
			},
			OutputFile: "", // No output file, this is validation
			Checklist: []string{
				"Intent packet is usable without referring to raw input",
				"Constraints are actionable",
				"Scope boundaries are explicit",
				"Blocking unknowns are documented",
			},
			Autonomy: guidance.AutonomyLow, // Requires human decision
			Order:    5,
		},
	}
}

// NewNavigator creates a new ingest navigator.
func NewNavigator(gctx *guidance.GuidanceContext) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModeIngest)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		steps:         DefaultIngestSteps(),
		currentStep:   0,
		loopState: &LoopState{
			IterationCount: 1,
			MaxIterations:  3,
			Approved:       false,
		},
	}, nil
}

// Initialize loads state and determines current position.
func (n *Navigator) Initialize(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if err := n.BaseNavigator.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing base navigator")
	}

	// Check if WORKFLOW.md exists in phase directory
	phaseDir := n.Ctx.PhasePath
	if phaseDir == "" {
		phaseDir = n.Ctx.FestivalPath
	}

	workflowPath := filepath.Join(phaseDir, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		// WORKFLOW.md exists, initialize WorkflowNavigator
		wfNav, err := workflow.NewNavigator(n.Ctx, guidance.ModeIngest)
		if err != nil {
			return errors.Wrap(err, "creating workflow navigator")
		}
		if err := wfNav.Initialize(ctx); err != nil {
			return errors.Wrap(err, "initializing workflow navigator")
		}
		n.workflowNav = wfNav
		n.useWorkflowMd = true
		return nil
	}

	// No WORKFLOW.md, use default steps
	// Restore state
	state := n.StateManager.State()
	n.currentStep = state.CurrentStep

	// Restore loop state from phase state if available
	if state.PhaseState != nil {
		if ingestState, ok := state.PhaseState[string(guidance.ModeIngest)].(map[string]any); ok {
			if ic, ok := ingestState["iteration_count"].(int); ok {
				n.loopState.IterationCount = ic
			}
			if cs, ok := ingestState["current_step"].(int); ok {
				n.loopState.CurrentStep = cs
			}
			if pa, ok := ingestState["pending_approval"].(bool); ok {
				n.loopState.PendingApproval = pa
			}
			if ap, ok := ingestState["approved"].(bool); ok {
				n.loopState.Approved = ap
			}
		}
	}

	// Update total tasks in state
	state.TotalTasks = len(n.steps)

	return nil
}

// GetNext returns the next ingest step.
func (n *Navigator) GetNext(ctx context.Context) (*guidance.NextStep, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetNext(ctx)
	}

	// If already approved, ingest is complete
	if n.loopState.Approved {
		return nil, nil
	}

	if n.currentStep >= len(n.steps) {
		return nil, nil // Ingest complete
	}

	step := n.steps[n.currentStep]

	// At the gate step, return special approval step
	if step.ID == "ingest_gate" {
		return n.buildApprovalStep(), nil
	}

	return n.buildNextStep(step), nil
}

// buildNextStep creates a NextStep from an IngestStep.
func (n *Navigator) buildNextStep(step IngestStep) *guidance.NextStep {
	return &guidance.NextStep{
		Mode:              guidance.ModeIngest,
		StepType:          guidance.StepTypeIngestStep,
		ID:                step.ID,
		Title:             step.Title,
		Objective:         step.Description,
		Instructions:      step.Instructions,
		AutonomyLevel:     step.Autonomy,
		ContextFiles:      n.getContextFiles(),
		CompletionCommand: fmt.Sprintf("fest ingest next --complete %s", step.ID),
		Metadata: map[string]any{
			"output_file":    step.OutputFile,
			"checklist":      step.Checklist,
			"step_number":    step.Order,
			"total_steps":    len(n.steps),
			"iteration":      n.loopState.IterationCount,
			"max_iterations": n.loopState.MaxIterations,
			"input_dir":      filepath.Join(n.Ctx.FestivalPath, "input"),
			"output_dir":     filepath.Join(n.Ctx.FestivalPath, "ingest"),
		},
	}
}

// buildApprovalStep creates the special approval step for the ingest gate.
func (n *Navigator) buildApprovalStep() *guidance.NextStep {
	gateStep := n.steps[len(n.steps)-1] // Last step is the gate

	return &guidance.NextStep{
		Mode:      guidance.ModeIngest,
		StepType:  guidance.StepTypeApproval,
		ID:        "ingest_approval",
		Title:     fmt.Sprintf("Ingest Approval (Iteration %d of %d)", n.loopState.IterationCount, n.loopState.MaxIterations),
		Objective: "Review all ingest outputs and approve or request revisions",
		Instructions: []string{
			"Review intent.md - Does it capture the core objective?",
			"Review constraints.md - Are all constraints identified?",
			"Review scope.md - Are boundaries clear and complete?",
			"Review unknowns.md - Are questions documented with resolution paths?",
			"Approve to proceed to planning, or reject for another iteration",
		},
		AutonomyLevel:     guidance.AutonomyLow,
		ContextFiles:      n.getApprovalContextFiles(),
		CompletionCommand: "fest ingest approve",
		Metadata: map[string]any{
			"output_file":     gateStep.OutputFile,
			"step_number":     gateStep.Order,
			"total_steps":     len(n.steps),
			"loop_iteration":  n.loopState.IterationCount,
			"max_loops":       n.loopState.MaxIterations,
			"approve_command": "fest ingest approve",
			"reject_command":  "fest ingest reject --reason \"<reason>\"",
		},
	}
}

// getApprovalContextFiles returns files relevant to the approval decision.
func (n *Navigator) getApprovalContextFiles() []string {
	files := n.getContextFiles()

	// Add specific output files for review
	ingestDir := filepath.Join(n.Ctx.FestivalPath, "ingest")
	files = append(files,
		filepath.Join(ingestDir, "intent.md"),
		filepath.Join(ingestDir, "constraints.md"),
		filepath.Join(ingestDir, "scope.md"),
		filepath.Join(ingestDir, "unknowns.md"),
	)

	return files
}

// getContextFiles returns relevant files for ingest context.
func (n *Navigator) getContextFiles() []string {
	var files []string

	// Festival-level docs
	festivalGoal := filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md")
	files = append(files, festivalGoal)

	// Input directory
	inputDir := filepath.Join(n.Ctx.FestivalPath, "input")
	files = append(files, inputDir)

	// Ingest output directory
	ingestDir := filepath.Join(n.Ctx.FestivalPath, "ingest")
	files = append(files, ingestDir)

	return files
}

// MarkComplete marks the current step as complete and advances.
func (n *Navigator) MarkComplete(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkComplete(ctx, stepID)
	}

	// Special handling for ingest_approval - this triggers approval
	if stepID == "ingest_approval" {
		return n.Approve(ctx)
	}

	// Find and validate step
	stepFound := false
	for i, step := range n.steps {
		if step.ID == stepID {
			if i != n.currentStep {
				return errors.Validation("cannot complete step out of order").
					WithField("expected", n.steps[n.currentStep].ID).
					WithField("received", stepID)
			}
			stepFound = true
			break
		}
	}

	if !stepFound {
		return errors.NotFound("ingest step not found").WithField("stepID", stepID)
	}

	// Mark complete and advance
	n.StateManager.SetTaskStatus(stepID, guidance.StatusCompleted)
	n.currentStep++
	n.StateManager.SetPosition(0, 0, n.currentStep)

	return n.StateManager.Save(ctx)
}

// MarkSkipped marks a step as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkSkipped(ctx, stepID)
	}

	n.StateManager.SetTaskStatus(stepID, guidance.StatusSkipped)
	n.currentStep++
	n.StateManager.SetPosition(0, 0, n.currentStep)

	return n.StateManager.Save(ctx)
}

// MarkFailed marks a step as failed.
func (n *Navigator) MarkFailed(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkFailed(ctx, stepID)
	}

	n.StateManager.SetTaskStatus(stepID, guidance.StatusFailed)
	return n.StateManager.Save(ctx)
}

// Advance moves to the next step.
func (n *Navigator) Advance(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.Advance(ctx)
	}

	// Check if already approved
	if n.loopState.Approved {
		return guidance.ErrAlreadyComplete
	}

	if n.currentStep >= len(n.steps) {
		return guidance.ErrAlreadyComplete
	}

	// Mark current as complete and advance
	currentStep := n.steps[n.currentStep]
	return n.MarkComplete(ctx, currentStep.ID)
}

// GetProgress returns ingest progress.
func (n *Navigator) GetProgress(ctx context.Context) (*guidance.Progress, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetProgress(ctx)
	}

	progress := guidance.NewProgress(guidance.ModeIngest)
	progress.Total = len(n.steps)
	progress.Completed = n.currentStep
	progress.Pending = len(n.steps) - n.currentStep
	progress.Calculate()

	if n.currentStep < len(n.steps) {
		progress.CurrentTask = n.steps[n.currentStep].Title
	}

	// Note: Loop info is available via GetLoopState() method

	return progress, nil
}

// FormatInstructions generates agent-friendly ingest instructions.
func (n *Navigator) FormatInstructions(ctx context.Context) (string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return "", err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.FormatInstructions(ctx)
	}

	// Check if approved or all steps complete
	if n.loopState.Approved || n.currentStep >= len(n.steps) {
		return n.formatComplete(), nil
	}

	step := n.steps[n.currentStep]
	progress, _ := n.GetProgress(ctx)

	return n.formatInstructions(step, progress), nil
}

// formatComplete renders the completion message.
func (n *Navigator) formatComplete() string {
	var sb strings.Builder

	sb.WriteString("# Ingest Complete\n")
	sb.WriteString("────────────────────\n\n")
	sb.WriteString("All ingest steps completed. Ready for planning.\n\n")
	sb.WriteString("Run `fest status --mode plan` to switch to planning mode.\n")

	return sb.String()
}

// formatInstructions renders the ingest instructions.
func (n *Navigator) formatInstructions(step IngestStep, progress *guidance.Progress) string {
	var sb strings.Builder

	sb.WriteString(guidance.InstructionHeader)
	// Header
	sb.WriteString("# Festival Ingest Guidance\n")
	sb.WriteString("────────────────────────────────\n\n")

	// Progress line
	fmt.Fprintf(&sb, "Progress       %s\n", progress.Summary())
	fmt.Fprintf(&sb, "Loop           %d of %d\n\n", n.loopState.IterationCount, n.loopState.MaxIterations)

	// Current step section
	sb.WriteString("## Current Step\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "Step           %d of %d\n", step.Order, len(n.steps))
	fmt.Fprintf(&sb, "Title          %s\n", step.Title)
	fmt.Fprintf(&sb, "Description    %s\n", step.Description)
	fmt.Fprintf(&sb, "Output File    %s\n", step.OutputFile)
	fmt.Fprintf(&sb, "Autonomy       %s\n\n", step.Autonomy)

	// Instructions section
	sb.WriteString("## Instructions\n")
	sb.WriteString("────────────────\n")
	for _, instruction := range step.Instructions {
		fmt.Fprintf(&sb, "  • %s\n", instruction)
	}
	sb.WriteString("\n")

	// Context files section
	sb.WriteString("## Context Files\n")
	sb.WriteString("─────────────\n")
	for _, file := range n.getContextFiles() {
		fmt.Fprintf(&sb, "  - %s\n", file)
	}
	sb.WriteString("\n")

	// Loop controls (only show at gate step)
	if step.ID == "ingest_gate" {
		sb.WriteString("## Loop Controls\n")
		sb.WriteString("────────────────\n")
		sb.WriteString("  • fest ingest approve   # Approve and proceed\n")
		sb.WriteString("  • fest ingest reject    # Request revisions\n")
		sb.WriteString("\n")
	}

	// Completion command
	sb.WriteString("## When Complete\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "Run: fest ingest next --complete %s\n", step.ID)

	return sb.String()
}

// GetContextFiles returns context files.
func (n *Navigator) GetContextFiles(ctx context.Context) ([]string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetContextFiles(ctx)
	}

	return n.getContextFiles(), nil
}

// Approve approves the current ingest loop and proceeds.
func (n *Navigator) Approve(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Can only approve at the gate step or when pending approval
	if !n.loopState.PendingApproval && n.currentStep < len(n.steps) && n.steps[n.currentStep].ID != "ingest_gate" {
		return errors.Validation("can only approve at ingest gate step").
			WithField("current_step", n.steps[n.currentStep].ID)
	}

	n.loopState.Approved = true
	n.loopState.PendingApproval = false
	n.loopState.LastGatePassed = true

	// Mark approval task as complete
	n.StateManager.SetTaskStatus("ingest_approval", guidance.StatusCompleted)

	// Save loop state
	return n.saveLoopState(ctx)
}

// Reject rejects the current ingest and starts a new loop iteration.
func (n *Navigator) Reject(ctx context.Context, reason string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Can only reject at the gate step or when pending approval
	if !n.loopState.PendingApproval && n.currentStep < len(n.steps) && n.steps[n.currentStep].ID != "ingest_gate" {
		return errors.Validation("can only reject at ingest gate step").
			WithField("current_step", n.steps[n.currentStep].ID)
	}

	// Check if we've exceeded max iterations
	if n.loopState.IterationCount >= n.loopState.MaxIterations {
		return errors.Validation("maximum loop iterations reached").
			WithField("max_iterations", n.loopState.MaxIterations).
			WithField("iteration", n.loopState.IterationCount)
	}

	// Increment iteration and reset
	n.loopState.IterationCount++
	n.loopState.CurrentStep = 0
	n.loopState.PendingApproval = false
	n.loopState.LastGatePassed = false
	n.loopState.RejectReason = reason
	n.currentStep = 0

	// Reset all step statuses for new iteration
	for _, step := range n.steps {
		n.StateManager.SetTaskStatus(step.ID, guidance.StatusPending)
	}

	n.StateManager.SetPosition(0, 0, 0)

	return n.saveLoopState(ctx)
}

// saveLoopState persists the loop state.
func (n *Navigator) saveLoopState(ctx context.Context) error {
	n.StateManager.UpdatePhaseState(guidance.ModeIngest, "iteration_count", n.loopState.IterationCount)
	n.StateManager.UpdatePhaseState(guidance.ModeIngest, "current_step", n.loopState.CurrentStep)
	n.StateManager.UpdatePhaseState(guidance.ModeIngest, "pending_approval", n.loopState.PendingApproval)
	n.StateManager.UpdatePhaseState(guidance.ModeIngest, "approved", n.loopState.Approved)

	return n.StateManager.Save(ctx)
}

// GetLoopState returns the current loop state.
func (n *Navigator) GetLoopState() *LoopState {
	return n.loopState
}
