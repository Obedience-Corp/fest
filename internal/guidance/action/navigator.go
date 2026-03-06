// Package action provides the ActionNavigator for navigating
// festival action phases with operational tasks and verification gates.
package action

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

func init() {
	guidance.RegisterNavigator(guidance.ModeAction, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx)
	})
}

// Navigator implements guidance.Navigator for action phases.
// Action phases handle operational tasks like deployments,
// configurations, migrations, and other non-coding work.
type Navigator struct {
	*guidance.BaseNavigator
	actions       []*ActionItem
	currentAction int
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// ActionItem represents an operational task.
type ActionItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Commands      []string `json:"commands"`       // Commands to execute
	Verification  []string `json:"verification"`   // How to verify success
	Rollback      []string `json:"rollback"`       // Rollback steps if failed
	RequiresHuman bool     `json:"requires_human"` // Must be executed by human
	Status        string   `json:"status"`
}

// NewNavigator creates a new action navigator.
func NewNavigator(gctx *guidance.GuidanceContext) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModeAction)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		actions:       make([]*ActionItem, 0),
		currentAction: 0,
	}, nil
}

// Initialize loads state and action items.
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

	// Load actions from phase structure
	n.actions = n.loadActions()

	// Restore position from state
	state := n.StateManager.State()
	n.currentAction = state.CurrentStep

	// Restore action statuses from state
	for _, action := range n.actions {
		if status := n.StateManager.GetTaskStatus(action.ID); status != "" {
			action.Status = status
		}
	}

	// Update total tasks in state
	state.TotalTasks = len(n.actions)

	return nil
}

// loadActions discovers action items from filesystem.
// TODO: Parse task files in phase directory for action items.
func (n *Navigator) loadActions() []*ActionItem {
	// Future implementation will:
	// - Scan phase directory for action task files
	// - Extract commands, verification, rollback from frontmatter
	// - Build ActionItem structs from task metadata
	return nil
}

// GetNext returns the next action to perform.
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

	for i, action := range n.actions {
		if action.Status == "" || action.Status == guidance.StatusPending {
			n.currentAction = i

			autonomy := guidance.AutonomyMedium
			if action.RequiresHuman {
				autonomy = guidance.AutonomyLow
			}

			instructions := []string{"Execute the following actions:"}
			instructions = append(instructions, action.Commands...)

			return &guidance.NextStep{
				Mode:              guidance.ModeAction,
				StepType:          guidance.StepTypeActionStep,
				ID:                action.ID,
				Title:             action.Title,
				Objective:         action.Description,
				AutonomyLevel:     autonomy,
				Instructions:      instructions,
				ContextFiles:      n.getContextFiles(),
				CompletionCommand: fmt.Sprintf("fest action --complete %s", action.ID),
				Metadata: map[string]any{
					"commands":       action.Commands,
					"verification":   action.Verification,
					"rollback":       action.Rollback,
					"requires_human": action.RequiresHuman,
					"action_number":  i + 1,
					"total_actions":  len(n.actions),
					"fail_command":   fmt.Sprintf("fest action --fail %s", action.ID),
					"skip_command":   fmt.Sprintf("fest action --skip %s", action.ID),
				},
			}, nil
		}
	}

	return nil, nil // All actions complete
}

// getContextFiles returns relevant files for action context.
func (n *Navigator) getContextFiles() []string {
	files := []string{
		filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md"),
	}

	if n.Ctx.PhasePath != "" {
		files = append(files, filepath.Join(n.Ctx.PhasePath, "PHASE_GOAL.md"))
	}

	return files
}

// MarkComplete marks an action as complete.
func (n *Navigator) MarkComplete(ctx context.Context, actionID string) error {
	return n.markAction(ctx, actionID, guidance.StatusCompleted)
}

// MarkSkipped marks an action as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, actionID string) error {
	return n.markAction(ctx, actionID, guidance.StatusSkipped)
}

// MarkFailed marks an action as failed.
func (n *Navigator) MarkFailed(ctx context.Context, actionID string) error {
	return n.markAction(ctx, actionID, guidance.StatusFailed)
}

// markAction updates an action's status.
func (n *Navigator) markAction(ctx context.Context, actionID, status string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	for i, action := range n.actions {
		if action.ID == actionID {
			action.Status = status
			n.StateManager.SetTaskStatus(actionID, status)
			n.currentAction = i + 1
			n.StateManager.SetPosition(0, 0, n.currentAction)

			return n.StateManager.Save(ctx)
		}
	}

	return errors.NotFound("action not found").WithField("actionID", actionID)
}

// Advance moves to the next action.
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

	step, err := n.GetNext(ctx)
	if err != nil {
		return err
	}

	if step == nil {
		return guidance.ErrAlreadyComplete
	}

	return n.MarkComplete(ctx, step.ID)
}

// GetProgress returns action progress.
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

	progress := guidance.NewProgress(guidance.ModeAction)
	progress.Total = len(n.actions)

	for _, action := range n.actions {
		switch action.Status {
		case guidance.StatusCompleted:
			progress.Completed++
		case guidance.StatusFailed:
			progress.Failed++
		case guidance.StatusSkipped:
			progress.Skipped++
		default:
			progress.Pending++
		}
	}

	progress.Calculate()

	if n.currentAction < len(n.actions) {
		progress.CurrentTask = n.actions[n.currentAction].Title
	}

	return progress, nil
}

// FormatInstructions generates agent-friendly action instructions.
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

	if len(n.actions) == 0 {
		return n.formatNoActions(), nil
	}

	step, err := n.GetNext(ctx)
	if err != nil {
		return "", err
	}

	if step == nil {
		return n.formatComplete(), nil
	}

	progress, _ := n.GetProgress(ctx)

	return n.formatInstructions(step, progress), nil
}

// formatNoActions renders message when no actions found.
func (n *Navigator) formatNoActions() string {
	var sb strings.Builder

	sb.WriteString("# Action Phase\n")
	sb.WriteString("────────────────────\n\n")
	sb.WriteString("No action items discovered.\n\n")
	sb.WriteString("Action items are typically defined in task files with commands.\n")

	return sb.String()
}

// formatComplete renders the completion message.
func (n *Navigator) formatComplete() string {
	var sb strings.Builder

	sb.WriteString("# Actions Complete\n")
	sb.WriteString("────────────────────\n\n")

	// Check for failed actions
	hasFailures := false
	for _, action := range n.actions {
		if action.Status == guidance.StatusFailed {
			hasFailures = true
			break
		}
	}

	if hasFailures {
		sb.WriteString("**WARNING: Some actions failed**\n\n")
		sb.WriteString("Failed actions:\n")
		for _, action := range n.actions {
			if action.Status == guidance.StatusFailed {
				fmt.Fprintf(&sb, "  ✗ %s\n", action.Title)
				if len(action.Rollback) > 0 {
					fmt.Fprintf(&sb, "    Rollback: %s\n", action.Rollback[0])
				}
			}
		}
		sb.WriteString("\nConsider rollback steps before proceeding.\n")
	} else {
		sb.WriteString("All action items have been completed successfully.\n\n")
		sb.WriteString("Run `fest status` to review results.\n")
	}

	return sb.String()
}

// formatInstructions renders the action instructions.
func (n *Navigator) formatInstructions(step *guidance.NextStep, progress *guidance.Progress) string {
	var sb strings.Builder

	sb.WriteString(guidance.InstructionHeader)
	// Header
	sb.WriteString("# Action Phase Guidance\n")
	sb.WriteString("────────────────────────────────\n\n")

	// Progress line
	fmt.Fprintf(&sb, "Progress       %s\n", progress.Summary())

	// Show failed count if any
	if progress.Failed > 0 {
		fmt.Fprintf(&sb, "Failed         %d actions\n", progress.Failed)
	}
	sb.WriteString("\n")

	// Current action section
	sb.WriteString("## Current Action\n")
	sb.WriteString("─────────────────\n")
	if md, ok := step.Metadata["action_number"].(int); ok {
		fmt.Fprintf(&sb, "Action         %d of %d\n", md, step.Metadata["total_actions"])
	}
	fmt.Fprintf(&sb, "Title          %s\n", step.Title)
	fmt.Fprintf(&sb, "Objective      %s\n", step.Objective)

	// Requires human flag
	if rh, ok := step.Metadata["requires_human"].(bool); ok && rh {
		sb.WriteString("Requires       Human execution\n")
	}
	sb.WriteString("\n")

	// Commands section
	if commands, ok := step.Metadata["commands"].([]string); ok && len(commands) > 0 {
		sb.WriteString("## Commands\n")
		sb.WriteString("───────────\n")
		for _, cmd := range commands {
			fmt.Fprintf(&sb, "  $ %s\n", cmd)
		}
		sb.WriteString("\n")
	}

	// Verification section
	if verification, ok := step.Metadata["verification"].([]string); ok && len(verification) > 0 {
		sb.WriteString("## Verification\n")
		sb.WriteString("───────────────\n")
		for _, v := range verification {
			fmt.Fprintf(&sb, "  ✓ %s\n", v)
		}
		sb.WriteString("\n")
	}

	// Rollback section
	if rollback, ok := step.Metadata["rollback"].([]string); ok && len(rollback) > 0 {
		sb.WriteString("## Rollback (if needed)\n")
		sb.WriteString("───────────────────────\n")
		for _, r := range rollback {
			fmt.Fprintf(&sb, "  ← %s\n", r)
		}
		sb.WriteString("\n")
	}

	// Commands section
	sb.WriteString("## Mark Complete\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "  ✓ Complete: %s\n", step.CompletionCommand)
	fmt.Fprintf(&sb, "  ✗ Failed:   %s\n", step.Metadata["fail_command"])
	fmt.Fprintf(&sb, "  → Skip:     %s\n", step.Metadata["skip_command"])

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

// GetRollbackSteps returns rollback instructions for a failed action.
func (n *Navigator) GetRollbackSteps(actionID string) []string {
	for _, action := range n.actions {
		if action.ID == actionID {
			return action.Rollback
		}
	}
	return nil
}

// GetActions returns all action items.
func (n *Navigator) GetActions() []*ActionItem {
	return n.actions
}

// AddAction adds a new action item.
func (n *Navigator) AddAction(action *ActionItem) {
	n.actions = append(n.actions, action)
}

// HasFailedActions returns true if any actions have failed.
func (n *Navigator) HasFailedActions() bool {
	for _, action := range n.actions {
		if action.Status == guidance.StatusFailed {
			return true
		}
	}
	return false
}
