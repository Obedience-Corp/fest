// Package execution provides the ExecutionNavigator for navigating
// implementation phases in festivals.
package execution

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/embedded/templates/agent"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

// PlanBuilder defines the interface for building execution plans.
// This allows for dependency injection and testing.
type PlanBuilder interface {
	BuildPlan(ctx context.Context) (*ExecutionPlan, error)
}

// Navigator implements guidance.Navigator for implementation phases.
// It orchestrates task execution, progress tracking, and agent instructions.
type Navigator struct {
	*guidance.BaseNavigator
	planBuilder PlanBuilder
	plan        *ExecutionPlan
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// NewNavigator creates a new execution navigator.
// The planBuilder parameter allows for dependency injection; pass nil
// to use the default plan builder (which requires SetPlanBuilder before Initialize).
func NewNavigator(gctx *guidance.GuidanceContext, planBuilder PlanBuilder) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModeExecute)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		planBuilder:   planBuilder,
	}, nil
}

// SetPlanBuilder sets the plan builder. Call this before Initialize
// if a nil planBuilder was passed to NewNavigator.
func (n *Navigator) SetPlanBuilder(pb PlanBuilder) {
	n.planBuilder = pb
}

// Initialize loads state and builds the execution plan.
// Must be called before other methods.
func (n *Navigator) Initialize(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Validate plan builder is set
	if n.planBuilder == nil {
		return errors.Validation("plan builder required").
			WithHint("call SetPlanBuilder before Initialize")
	}

	// Initialize base (loads state)
	if err := n.BaseNavigator.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing base navigator")
	}

	// Build execution plan
	plan, err := n.planBuilder.BuildPlan(ctx)
	if err != nil {
		return errors.Wrap(err, "building execution plan")
	}
	n.plan = plan

	// Sync state from plan's task statuses
	for _, phase := range plan.Phases {
		for _, seq := range phase.Sequences {
			for _, step := range seq.Steps {
				for _, task := range step.Tasks {
					if task.Status != "" && task.Status != StatusPending {
						n.StateManager.SetTaskStatus(task.ID, task.Status)
					}
				}
			}
		}
	}

	// Update total tasks in state
	if plan.Summary != nil {
		n.StateManager.State().TotalTasks = plan.Summary.TotalTasks
	}

	return nil
}

// GetPlan returns the execution plan. Returns nil if not initialized.
//
// Backward Compatibility Notes:
//
// This method provides access to the execution plan for code that hasn't
// yet migrated to the Navigator interface. New code should prefer using
// Navigator methods (GetNext, MarkComplete, GetProgress, etc.) rather
// than accessing the plan directly.
//
// Migration path:
// 1. Replace execute.Runner with guidance.Navigator
// 2. Replace direct plan access with Navigator methods
// 3. Remove GetPlan() calls once migration is complete
func (n *Navigator) GetPlan() *ExecutionPlan {
	return n.plan
}

// --- GetNext implementation ---

// GetNext returns the next step/task based on current position.
// Returns nil (with no error) if all work is complete.
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

	// Find the next step group
	stepGroup, phaseIdx, seqIdx, stepIdx := n.getNextStepGroup()
	if stepGroup == nil {
		return nil, nil // All complete
	}

	// Update position in state
	n.StateManager.SetPosition(phaseIdx, seqIdx, stepIdx)

	// Build and return the NextStep
	return n.buildNextStep(stepGroup, phaseIdx, seqIdx)
}

// getNextStepGroup traverses the plan to find the next pending or in-progress step group.
// Returns nil if all tasks are complete.
func (n *Navigator) getNextStepGroup() (*StepGroup, int, int, int) {
	if n.plan == nil || len(n.plan.Phases) == 0 {
		return nil, 0, 0, 0
	}

	for phaseIdx, phase := range n.plan.Phases {
		for seqIdx, seq := range phase.Sequences {
			for stepIdx, step := range seq.Steps {
				if n.isStepGroupPending(step) {
					return step, phaseIdx, seqIdx, stepIdx
				}
			}
		}
	}

	return nil, 0, 0, 0
}

// isStepGroupPending returns true if any task in the step group is pending or in-progress.
func (n *Navigator) isStepGroupPending(step *StepGroup) bool {
	for _, task := range step.Tasks {
		status := n.StateManager.GetTaskStatus(task.ID)
		if status == StatusPending || status == StatusInProgress {
			return true
		}
	}
	return false
}

// buildNextStep converts a StepGroup to a guidance.NextStep.
func (n *Navigator) buildNextStep(step *StepGroup, phaseIdx, seqIdx int) (*guidance.NextStep, error) {
	if len(step.Tasks) == 0 {
		return nil, errors.Validation("step group has no tasks")
	}

	// Get phase and sequence for context
	phase := n.plan.Phases[phaseIdx]
	seq := phase.Sequences[seqIdx]

	// Build context files
	contextFiles := n.buildContextFiles(phase.Path, seq.Path)

	// Single task case
	if len(step.Tasks) == 1 {
		task := step.Tasks[0]
		return n.buildNextStepFromTask(task, contextFiles), nil
	}

	// Parallel task group case
	return n.buildParallelNextStep(step, contextFiles), nil
}

// buildNextStepFromTask creates a NextStep from a single TaskInfo.
func (n *Navigator) buildNextStepFromTask(task *TaskInfo, contextFiles []string) *guidance.NextStep {
	stepType := n.getStepType(task)
	autonomy := n.getAutonomyLevel(task.AutonomyLevel)

	nextStep := guidance.NewNextStep(guidance.ModeExecute, stepType, task.ID, task.Name, task.Path)
	nextStep.AutonomyLevel = autonomy
	nextStep.ContextFiles = contextFiles
	nextStep.Dependencies = task.Dependencies
	nextStep.CompletionCommand = "fest progress --task " + task.Path + " --complete"

	return nextStep
}

// buildParallelNextStep creates a NextStep for a parallel task group.
func (n *Navigator) buildParallelNextStep(step *StepGroup, contextFiles []string) *guidance.NextStep {
	// Use first task as the primary step
	primary := step.Tasks[0]
	stepType := n.getStepType(primary)
	autonomy := n.getAutonomyLevel(primary.AutonomyLevel)

	nextStep := guidance.NewNextStep(guidance.ModeExecute, stepType, primary.ID, primary.Name, primary.Path)
	nextStep.AutonomyLevel = autonomy
	nextStep.ContextFiles = contextFiles
	nextStep.Parallel = true

	// Add all tasks (including primary) to the group
	for _, task := range step.Tasks {
		taskStep := n.buildNextStepFromTask(task, nil) // Don't duplicate context files
		nextStep.Group = append(nextStep.Group, taskStep)
	}

	return nextStep
}

// getStepType returns the step type based on task properties.
func (n *Navigator) getStepType(task *TaskInfo) string {
	if task.IsGate {
		return guidance.StepTypeGate
	}
	return guidance.StepTypeTask
}

// getAutonomyLevel converts string autonomy to guidance.AutonomyLevel.
func (n *Navigator) getAutonomyLevel(level string) guidance.AutonomyLevel {
	switch level {
	case "high":
		return guidance.AutonomyHigh
	case "medium":
		return guidance.AutonomyMedium
	case "low":
		return guidance.AutonomyLow
	default:
		return guidance.AutonomyMedium
	}
}

// buildContextFiles returns the context files for the current position.
func (n *Navigator) buildContextFiles(phasePath, seqPath string) []string {
	var files []string

	// Add festival goal
	festivalGoal := filepath.Join(n.plan.FestivalPath, "FESTIVAL_GOAL.md")
	files = append(files, festivalGoal)

	// Add phase goal
	if phasePath != "" {
		phaseGoal := filepath.Join(phasePath, "PHASE_GOAL.md")
		files = append(files, phaseGoal)
	}

	// Add sequence goal
	if seqPath != "" {
		seqGoal := filepath.Join(seqPath, "SEQUENCE_GOAL.md")
		files = append(files, seqGoal)
	}

	return files
}

// MarkComplete marks the specified step/task as complete.
// Delegates to BaseNavigator.MarkTaskStatus with StatusCompleted.
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

	return n.BaseNavigator.MarkTaskStatus(ctx, stepID, StatusCompleted)
}

// MarkSkipped marks the specified step/task as skipped.
// Delegates to BaseNavigator.MarkTaskStatus with StatusSkipped.
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

	return n.BaseNavigator.MarkTaskStatus(ctx, stepID, StatusSkipped)
}

// MarkFailed marks the specified step/task as failed.
// Delegates to BaseNavigator.MarkTaskStatus with StatusFailed.
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

	return n.BaseNavigator.MarkTaskStatus(ctx, stepID, StatusFailed)
}

// Advance moves to the next position (phase, sequence, step).
// In execution mode, this is typically called after marking tasks complete.
// It finds the next pending step and updates the position.
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

	// Find the next step group
	stepGroup, phaseIdx, seqIdx, stepIdx := n.getNextStepGroup()
	if stepGroup == nil {
		return guidance.ErrAlreadyComplete
	}

	// Update position
	n.StateManager.SetPosition(phaseIdx, seqIdx, stepIdx)

	// Save state
	return n.StateManager.Save(ctx)
}

// GetProgress returns current progress information.
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

	// Build progress from state
	progress := n.buildProgress()

	return progress, nil
}

// buildProgress creates a Progress struct from the current state and plan.
func (n *Navigator) buildProgress() *guidance.Progress {
	state := n.StateManager.State()
	progress := guidance.NewProgress(guidance.ModeExecute)

	progress.Total = state.TotalTasks
	progress.Completed = state.CompletedTasks
	progress.Failed = state.FailedTasks
	progress.Skipped = state.SkippedTasks
	progress.InProgress = n.countInProgress()
	progress.Pending = state.TotalTasks - state.CompletedTasks - state.SkippedTasks - state.FailedTasks - progress.InProgress

	progress.Calculate()

	// Add phase-level progress
	for _, phase := range n.plan.Phases {
		phaseProgress := n.buildPhaseProgress(phase)
		progress.AddPhase(phase.Name, phaseProgress)
	}

	// Set current position
	if state.CurrentPhase < len(n.plan.Phases) {
		phase := n.plan.Phases[state.CurrentPhase]
		progress.CurrentPhase = phase.Name

		if state.CurrentSequence < len(phase.Sequences) {
			seq := phase.Sequences[state.CurrentSequence]
			progress.CurrentSequence = seq.Name

			if state.CurrentStep < len(seq.Steps) {
				step := seq.Steps[state.CurrentStep]
				if len(step.Tasks) > 0 {
					progress.CurrentTask = step.Tasks[0].Name
				}
			}
		}
	}

	return progress
}

// countInProgress counts tasks that are currently in progress.
func (n *Navigator) countInProgress() int {
	count := 0
	for _, phase := range n.plan.Phases {
		for _, seq := range phase.Sequences {
			for _, step := range seq.Steps {
				for _, task := range step.Tasks {
					if n.StateManager.GetTaskStatus(task.ID) == StatusInProgress {
						count++
					}
				}
			}
		}
	}
	return count
}

// buildPhaseProgress creates progress info for a single phase.
func (n *Navigator) buildPhaseProgress(phase *PhaseExecution) *guidance.PhaseProgressInfo {
	info := &guidance.PhaseProgressInfo{
		Name:   phase.Name,
		Number: phase.Number,
	}

	// Count tasks in this phase
	completed := 0
	total := 0

	for _, seq := range phase.Sequences {
		seqInfo := &guidance.SequenceProgressInfo{
			Name:   seq.Name,
			Number: seq.Number,
		}

		for _, step := range seq.Steps {
			for _, task := range step.Tasks {
				total++
				seqInfo.Total++
				status := n.StateManager.GetTaskStatus(task.ID)
				if status == StatusCompleted {
					completed++
					seqInfo.Completed++
				}
			}
		}

		seqInfo.Percentage = calculatePercentage(seqInfo.Completed, seqInfo.Total)
		seqInfo.Status = n.deriveStatus(seqInfo.Completed, seqInfo.Total)
		info.Sequences = append(info.Sequences, seqInfo)
	}

	info.Completed = completed
	info.Total = total
	info.Percentage = calculatePercentage(completed, total)
	info.Status = n.deriveStatus(completed, total)

	return info
}

// calculatePercentage calculates percentage safely.
func calculatePercentage(completed, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(completed) / float64(total) * 100
}

// deriveStatus derives status from completion counts.
func (n *Navigator) deriveStatus(completed, total int) string {
	if total == 0 {
		return StatusPending
	}
	if completed >= total {
		return StatusCompleted
	}
	if completed > 0 {
		return StatusInProgress
	}
	return StatusPending
}

// FormatInstructions generates agent-friendly instructions for the current state.
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

	// Get the next step
	nextStep, err := n.GetNext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "getting next step")
	}

	// If no next step, all work is complete
	if nextStep == nil {
		return n.formatComplete()
	}

	// Build instruction data and render
	return n.formatInstructions(ctx, nextStep)
}

// formatComplete renders the completion template.
func (n *Navigator) formatComplete() (string, error) {
	data := map[string]string{
		"Header":  "# Execution Complete\n────────────────────",
		"Message": "All tasks have been executed.\n\nPlease review the festival and ensure that the festival methodology was followed,\nand that tasks were not simply marked completed without having actually been completed.",
	}

	return agent.Render("execute/complete", data)
}

// formatInstructions renders the instructions template with all sections.
func (n *Navigator) formatInstructions(ctx context.Context, nextStep *guidance.NextStep) (string, error) {
	// Get progress for the progress line
	progress, err := n.GetProgress(ctx)
	if err != nil {
		return "", errors.Wrap(err, "getting progress")
	}

	// Get context files
	contextFiles, err := n.GetContextFiles(ctx)
	if err != nil {
		return "", errors.Wrap(err, "getting context files")
	}

	// Build template data
	data := map[string]string{
		"Header":            "# Agent Execution Instructions\n────────────────────────────────",
		"ActionInstruction": n.formatActionInstruction(nextStep),
		"TasksSection":      n.formatTasksSection(nextStep),
		"ProgressLine":      n.formatProgressLine(progress),
		"PositionSection":   n.formatPositionSection(progress),
		"ContextSection":    n.formatContextSection(contextFiles),
		"ProgressCmd":       nextStep.GetCompletionCommand(),
	}

	return agent.Render("execute/instructions", data)
}

// formatActionInstruction generates the action instruction based on step type.
func (n *Navigator) formatActionInstruction(step *guidance.NextStep) string {
	if step.IsGate() {
		return "Read the gate file and verify all criteria are met before marking complete."
	}
	return "Read the task file and follow its instructions exactly."
}

// formatTasksSection generates the tasks section showing pending tasks.
func (n *Navigator) formatTasksSection(step *guidance.NextStep) string {
	var sb strings.Builder
	sb.WriteString("## Tasks to Execute\n")
	sb.WriteString("────────────────\n")

	if step.IsParallel() {
		// Multiple parallel tasks
		for _, task := range step.Group {
			n.formatSingleTask(&sb, task)
		}
	} else {
		// Single task
		n.formatSingleTask(&sb, step)
	}

	return sb.String()
}

// formatSingleTask formats a single task entry.
func (n *Navigator) formatSingleTask(sb *strings.Builder, step *guidance.NextStep) {
	// Task indicator
	indicator := "○"
	if step.IsGate() {
		indicator = "◆"
	}

	fmt.Fprintf(sb, "%s %s\n", indicator, step.Title)
	fmt.Fprintf(sb, "  Path           %s\n", step.Path)
	fmt.Fprintf(sb, "  Autonomy       %s\n", step.AutonomyLevel)

	if len(step.Dependencies) > 0 {
		fmt.Fprintf(sb, "  Dependencies   %s\n", strings.Join(step.Dependencies, ", "))
	}

	sb.WriteString("\n")
}

// formatProgressLine generates the progress summary line.
func (n *Navigator) formatProgressLine(progress *guidance.Progress) string {
	return fmt.Sprintf("Progress       %.1f%% (%d/%d tasks)", progress.Percentage, progress.Completed, progress.Total)
}

// formatPositionSection generates the current position section.
func (n *Navigator) formatPositionSection(progress *guidance.Progress) string {
	var sb strings.Builder
	sb.WriteString("## Current Position\n")
	sb.WriteString("────────────────\n")

	if progress.CurrentPhase != "" {
		fmt.Fprintf(&sb, "Phase          %s\n", progress.CurrentPhase)
	}
	if progress.CurrentSequence != "" {
		fmt.Fprintf(&sb, "Sequence       %s\n", progress.CurrentSequence)
	}
	if progress.CurrentTask != "" {
		fmt.Fprintf(&sb, "Task           %s\n", progress.CurrentTask)
	}

	return sb.String()
}

// formatContextSection generates the context files section.
func (n *Navigator) formatContextSection(files []string) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Context Files\n")
	sb.WriteString("─────────────\n")

	for _, file := range files {
		fmt.Fprintf(&sb, "  - %s\n", file)
	}

	return sb.String()
}

// GetContextFiles returns relevant context files for the current position.
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

	state := n.StateManager.State()

	// Get current phase and sequence paths
	var phasePath, seqPath string
	if state.CurrentPhase < len(n.plan.Phases) {
		phase := n.plan.Phases[state.CurrentPhase]
		phasePath = phase.Path

		if state.CurrentSequence < len(phase.Sequences) {
			seq := phase.Sequences[state.CurrentSequence]
			seqPath = seq.Path
		}
	}

	return n.buildContextFiles(phasePath, seqPath), nil
}

// --- Registration helpers ---
// Registration happens from the execute package to avoid import cycles.
// See internal/execute/navigator_registration.go

// NavigatorFactoryFunc creates the factory function for navigator registration.
// This is called from execute package during initialization.
func NavigatorFactoryFunc(planBuilderFactory func(festivalPath string) PlanBuilder) guidance.NavigatorFactory {
	return func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		pb := planBuilderFactory(gctx.FestivalPath)
		return NewNavigator(gctx, pb)
	}
}
