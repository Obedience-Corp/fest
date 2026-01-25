// Package planning provides the PlanningNavigator for navigating
// festival planning phases.
package planning

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

func init() {
	guidance.RegisterNavigator(guidance.ModePlan, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx)
	})
}

// Navigator implements guidance.Navigator for planning phases.
// It guides users through the festival planning workflow from
// requirements gathering to task specification.
type Navigator struct {
	*guidance.BaseNavigator
	steps       []PlanningStep
	currentStep int
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// PlanningStep defines a step in the planning workflow.
type PlanningStep struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Instructions []string               `json:"instructions"`
	Deliverables []string               `json:"deliverables"`
	Autonomy     guidance.AutonomyLevel `json:"autonomy"`
	Order        int                    `json:"order"`
}

// DefaultPlanningSteps returns the standard planning workflow.
func DefaultPlanningSteps() []PlanningStep {
	return []PlanningStep{
		{
			ID:          "gather_requirements",
			Title:       "Gather Requirements",
			Description: "Collect and document all requirements and constraints",
			Instructions: []string{
				"Review input materials in the input/ directory",
				"Document functional requirements",
				"Document non-functional requirements",
				"Identify constraints and dependencies",
			},
			Deliverables: []string{"requirements.md"},
			Autonomy:     guidance.AutonomyMedium,
			Order:        1,
		},
		{
			ID:          "define_scope",
			Title:       "Define Scope",
			Description: "Establish clear boundaries for the festival",
			Instructions: []string{
				"Define what is in scope",
				"Define what is explicitly out of scope",
				"Identify assumptions",
				"Document risks and unknowns",
			},
			Deliverables: []string{"scope.md"},
			Autonomy:     guidance.AutonomyMedium,
			Order:        2,
		},
		{
			ID:          "design_architecture",
			Title:       "Design Architecture",
			Description: "Define the high-level architecture and components",
			Instructions: []string{
				"Identify major components",
				"Define component interactions",
				"Create architecture diagrams",
				"Document technical decisions",
			},
			Deliverables: []string{"architecture.md", "diagrams/"},
			Autonomy:     guidance.AutonomyMedium,
			Order:        3,
		},
		{
			ID:          "create_phases",
			Title:       "Create Phase Structure",
			Description: "Break down the festival into numbered phases",
			Instructions: []string{
				"Identify logical groupings of work",
				"Create phase directories with PHASE_GOAL.md",
				"Define phase types (planning, implementation, etc.)",
				"Establish phase dependencies",
			},
			Deliverables: []string{"PHASE_GOAL.md files"},
			Autonomy:     guidance.AutonomyHigh,
			Order:        4,
		},
		{
			ID:          "create_sequences",
			Title:       "Create Sequences",
			Description: "Define sequences within each phase",
			Instructions: []string{
				"Break phases into logical sequences",
				"Create sequence directories with SEQUENCE_GOAL.md",
				"Define sequence objectives",
				"Order sequences appropriately",
			},
			Deliverables: []string{"SEQUENCE_GOAL.md files"},
			Autonomy:     guidance.AutonomyHigh,
			Order:        5,
		},
		{
			ID:          "create_tasks",
			Title:       "Create Task Documents",
			Description: "Specify individual task documents",
			Instructions: []string{
				"Create task markdown files",
				"Define objectives and instructions",
				"Set autonomy levels",
				"Identify dependencies between tasks",
			},
			Deliverables: []string{"Task .md files"},
			Autonomy:     guidance.AutonomyHigh,
			Order:        6,
		},
		{
			ID:          "add_quality_gates",
			Title:       "Add Quality Gates",
			Description: "Configure quality gates for sequences",
			Instructions: []string{
				"Review gate requirements per phase type",
				"Run `fest gates generate` to add gates",
				"Customize gate criteria as needed",
			},
			Deliverables: []string{"Quality gate task files"},
			Autonomy:     guidance.AutonomyHigh,
			Order:        7,
		},
		{
			ID:          "validate_plan",
			Title:       "Validate Plan",
			Description: "Review and validate the complete plan",
			Instructions: []string{
				"Run `fest validate` to check structure",
				"Review all GOAL.md files",
				"Verify task coverage",
				"Check for missing dependencies",
			},
			Deliverables: []string{"Validation report"},
			Autonomy:     guidance.AutonomyLow, // Requires human review
			Order:        8,
		},
		{
			ID:          "planning_approval",
			Title:       "Planning Approval",
			Description: "Obtain approval to proceed to execution",
			Instructions: []string{
				"Present plan for review",
				"Address feedback",
				"Mark plan as approved",
				"Transition to execution mode",
			},
			Deliverables: []string{"Approved plan"},
			Autonomy:     guidance.AutonomyLow,
			Order:        9,
		},
	}
}

// NewNavigator creates a new planning navigator.
func NewNavigator(gctx *guidance.GuidanceContext) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModePlan)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		steps:         DefaultPlanningSteps(),
		currentStep:   0,
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

	// Restore current step from state
	state := n.StateManager.State()
	n.currentStep = state.CurrentStep

	// Update total tasks in state
	state.TotalTasks = len(n.steps)

	return nil
}

// GetNext returns the next planning step.
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

	if n.currentStep >= len(n.steps) {
		return nil, nil // Planning complete
	}

	step := n.steps[n.currentStep]

	return &guidance.NextStep{
		Mode:          guidance.ModePlan,
		StepType:      guidance.StepTypePlanningStep,
		ID:            step.ID,
		Title:         step.Title,
		Objective:     step.Description,
		Instructions:  step.Instructions,
		AutonomyLevel: step.Autonomy,
		ContextFiles:  n.getContextFiles(),
		CompletionCommand: fmt.Sprintf("fest plan next --complete %s", step.ID),
		Metadata: map[string]any{
			"deliverables": step.Deliverables,
			"step_number":  step.Order,
			"total_steps":  len(n.steps),
		},
	}, nil
}

// getContextFiles returns relevant files for planning context.
func (n *Navigator) getContextFiles() []string {
	var files []string

	// Festival-level docs
	festivalGoal := filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md")
	festivalOverview := filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_OVERVIEW.md")
	files = append(files, festivalGoal, festivalOverview)

	// Input directory for requirements
	inputDir := filepath.Join(n.Ctx.FestivalPath, "input")
	files = append(files, inputDir)

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
		return errors.NotFound("planning step not found").WithField("stepID", stepID)
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

	if n.currentStep >= len(n.steps) {
		return guidance.ErrAlreadyComplete
	}

	// Mark current as complete and advance
	currentStep := n.steps[n.currentStep]
	return n.MarkComplete(ctx, currentStep.ID)
}

// GetProgress returns planning progress.
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

	progress := guidance.NewProgress(guidance.ModePlan)
	progress.Total = len(n.steps)
	progress.Completed = n.currentStep
	progress.Pending = len(n.steps) - n.currentStep
	progress.Calculate()

	if n.currentStep < len(n.steps) {
		progress.CurrentTask = n.steps[n.currentStep].Title
	}

	return progress, nil
}

// FormatInstructions generates agent-friendly planning instructions.
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

	if n.currentStep >= len(n.steps) {
		return n.formatComplete(), nil
	}

	step := n.steps[n.currentStep]
	progress, _ := n.GetProgress(ctx)

	return n.formatInstructions(step, progress), nil
}

// formatComplete renders the completion message.
func (n *Navigator) formatComplete() string {
	var sb strings.Builder

	sb.WriteString("# Planning Complete\n")
	sb.WriteString("────────────────────\n\n")
	sb.WriteString("All planning steps completed. Ready for execution.\n\n")
	sb.WriteString("Run `fest status --mode execute` to switch to execution mode.\n")

	return sb.String()
}

// formatInstructions renders the planning instructions.
func (n *Navigator) formatInstructions(step PlanningStep, progress *guidance.Progress) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Festival Planning Guidance\n")
	sb.WriteString("────────────────────────────────\n\n")

	// Progress line
	fmt.Fprintf(&sb, "Progress       %s\n\n", progress.Summary())

	// Current step section
	sb.WriteString("## Current Step\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "Step           %d of %d\n", step.Order, len(n.steps))
	fmt.Fprintf(&sb, "Title          %s\n", step.Title)
	fmt.Fprintf(&sb, "Description    %s\n", step.Description)
	fmt.Fprintf(&sb, "Autonomy       %s\n\n", step.Autonomy)

	// Instructions section
	sb.WriteString("## Instructions\n")
	sb.WriteString("────────────────\n")
	for _, instruction := range step.Instructions {
		fmt.Fprintf(&sb, "  • %s\n", instruction)
	}
	sb.WriteString("\n")

	// Deliverables section
	sb.WriteString("## Deliverables\n")
	sb.WriteString("────────────────\n")
	for _, deliverable := range step.Deliverables {
		fmt.Fprintf(&sb, "  - %s\n", deliverable)
	}
	sb.WriteString("\n")

	// Context files section
	sb.WriteString("## Context Files\n")
	sb.WriteString("─────────────\n")
	for _, file := range n.getContextFiles() {
		fmt.Fprintf(&sb, "  - %s\n", file)
	}
	sb.WriteString("\n")

	// Completion command
	sb.WriteString("## When Complete\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "Run: fest plan next --complete %s\n", step.ID)

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

	return n.getContextFiles(), nil
}
