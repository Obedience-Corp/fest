// Package review provides the ReviewNavigator for navigating
// festival review phases with checklist-based workflows.
package review

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

func init() {
	guidance.RegisterNavigator(guidance.ModeReview, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx)
	})
}

// Navigator implements guidance.Navigator for review phases.
// Review phases are checklist-based with sign-off gates.
type Navigator struct {
	*guidance.BaseNavigator
	checklist   []*ChecklistItem
	currentItem int
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// ChecklistItem represents a review checkpoint.
type ChecklistItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`   // pending, passed, failed, skipped
	Notes       string `json:"notes"`    // Reviewer notes
	Severity    string `json:"severity"` // blocker, major, minor
}

// NewNavigator creates a new review navigator.
func NewNavigator(gctx *guidance.GuidanceContext) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModeReview)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		checklist:     make([]*ChecklistItem, 0),
		currentItem:   0,
	}, nil
}

// Initialize loads state and checklist.
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

	// Load checklist
	n.checklist = n.loadChecklist()

	// Restore position from state
	state := n.StateManager.State()
	n.currentItem = state.CurrentStep

	// Restore item statuses from state
	for _, item := range n.checklist {
		if status := n.StateManager.GetTaskStatus(item.ID); status != "" {
			item.Status = status
		}
	}

	// Update total tasks in state
	state.TotalTasks = len(n.checklist)

	return nil
}

// loadChecklist loads the default code review checklist.
func (n *Navigator) loadChecklist() []*ChecklistItem {
	return []*ChecklistItem{
		{
			ID:          "context_propagation",
			Description: "Context propagation throughout",
			Status:      "pending",
			Severity:    "blocker",
		},
		{
			ID:          "error_handling",
			Description: "Proper error handling with wrapping",
			Status:      "pending",
			Severity:    "blocker",
		},
		{
			ID:          "file_size",
			Description: "No large files or functions (SRP enforced)",
			Status:      "pending",
			Severity:    "major",
		},
		{
			ID:          "interfaces",
			Description: "Interfaces are small and focused",
			Status:      "pending",
			Severity:    "major",
		},
		{
			ID:          "dependencies",
			Description: "Dependencies are injected",
			Status:      "pending",
			Severity:    "major",
		},
		{
			ID:          "test_coverage",
			Description: "Tests focus on behavior, not implementation",
			Status:      "pending",
			Severity:    "major",
		},
		{
			ID:          "magic_values",
			Description: "No magic strings or numbers",
			Status:      "pending",
			Severity:    "minor",
		},
	}
}

// GetNext returns the next unchecked item.
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

	for i, item := range n.checklist {
		if item.Status == "" || item.Status == "pending" {
			n.currentItem = i
			return &guidance.NextStep{
				Mode:          guidance.ModeReview,
				StepType:      guidance.StepTypeReviewStep,
				ID:            item.ID,
				Title:         "Review: " + item.Description,
				Objective:     "Verify: " + item.Description,
				AutonomyLevel: guidance.AutonomyLow, // Review requires human judgment
				Instructions: []string{
					"Check if the criteria is met in the code",
					"Mark as passed if criteria is satisfied",
					"Mark as failed if criteria is not met",
					"Add notes explaining your findings",
				},
				ContextFiles:      n.getContextFiles(),
				CompletionCommand: fmt.Sprintf("fest review --pass %s", item.ID),
				Metadata: map[string]any{
					"severity":     item.Severity,
					"item_number":  i + 1,
					"total_items":  len(n.checklist),
					"fail_command": fmt.Sprintf("fest review --fail %s", item.ID),
					"skip_command": fmt.Sprintf("fest review --skip %s", item.ID),
				},
			}, nil
		}
	}

	return nil, nil // All items reviewed
}

// getContextFiles returns relevant files for review context.
func (n *Navigator) getContextFiles() []string {
	files := []string{
		filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md"),
	}

	if n.Ctx.PhasePath != "" {
		files = append(files, filepath.Join(n.Ctx.PhasePath, "PHASE_GOAL.md"))
	}

	return files
}

// MarkComplete marks an item as passed.
func (n *Navigator) MarkComplete(ctx context.Context, itemID string) error {
	return n.markItem(ctx, itemID, "passed")
}

// MarkSkipped marks an item as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, itemID string) error {
	return n.markItem(ctx, itemID, "skipped")
}

// MarkFailed marks an item as failed.
func (n *Navigator) MarkFailed(ctx context.Context, itemID string) error {
	return n.markItem(ctx, itemID, "failed")
}

// markItem updates an item's status.
func (n *Navigator) markItem(ctx context.Context, itemID, status string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	for i, item := range n.checklist {
		if item.ID == itemID {
			item.Status = status

			// Map to guidance status
			guidanceStatus := guidance.StatusPending
			switch status {
			case "passed":
				guidanceStatus = guidance.StatusCompleted
			case "failed":
				guidanceStatus = guidance.StatusFailed
			case "skipped":
				guidanceStatus = guidance.StatusSkipped
			}

			n.StateManager.SetTaskStatus(itemID, guidanceStatus)
			n.currentItem = i + 1
			n.StateManager.SetPosition(0, 0, n.currentItem)

			return n.StateManager.Save(ctx)
		}
	}

	return errors.NotFound("checklist item not found").WithField("itemID", itemID)
}

// Advance moves to the next item.
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

// GetProgress returns review progress.
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

	progress := guidance.NewProgress(guidance.ModeReview)
	progress.Total = len(n.checklist)

	for _, item := range n.checklist {
		switch item.Status {
		case "passed", "skipped":
			progress.Completed++
		case "failed":
			progress.Failed++
		default:
			progress.Pending++
		}
	}

	progress.Calculate()

	if n.currentItem < len(n.checklist) {
		progress.CurrentTask = n.checklist[n.currentItem].Description
	}

	return progress, nil
}

// FormatInstructions generates agent-friendly review instructions.
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

// formatComplete renders the completion message.
func (n *Navigator) formatComplete() string {
	var sb strings.Builder

	sb.WriteString("# Review Complete\n")
	sb.WriteString("────────────────────\n\n")

	if n.HasBlockers() {
		sb.WriteString("**BLOCKERS DETECTED**\n\n")
		sb.WriteString("The following blocker items failed:\n")
		for _, item := range n.checklist {
			if item.Severity == "blocker" && item.Status == "failed" {
				fmt.Fprintf(&sb, "  ✗ %s\n", item.Description)
			}
		}
		sb.WriteString("\nAddress blockers before proceeding.\n")
	} else {
		sb.WriteString("All checklist items have been reviewed.\n\n")
		sb.WriteString("Run `fest status` to review results.\n")
	}

	return sb.String()
}

// formatInstructions renders the review instructions.
func (n *Navigator) formatInstructions(step *guidance.NextStep, progress *guidance.Progress) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Review Phase Guidance\n")
	sb.WriteString("────────────────────────────────\n\n")

	// Progress line
	fmt.Fprintf(&sb, "Progress       %s\n", progress.Summary())

	// Show failed count if any
	if progress.Failed > 0 {
		fmt.Fprintf(&sb, "Failed         %d items\n", progress.Failed)
	}
	sb.WriteString("\n")

	// Current item section
	sb.WriteString("## Current Checklist Item\n")
	sb.WriteString("─────────────────────────\n")
	if md, ok := step.Metadata["item_number"].(int); ok {
		fmt.Fprintf(&sb, "Item           %d of %d\n", md, step.Metadata["total_items"])
	}
	fmt.Fprintf(&sb, "Criteria       %s\n", step.Objective)
	fmt.Fprintf(&sb, "Severity       %s\n\n", step.Metadata["severity"])

	// Instructions section
	sb.WriteString("## Instructions\n")
	sb.WriteString("────────────────\n")
	for _, instruction := range step.Instructions {
		fmt.Fprintf(&sb, "  • %s\n", instruction)
	}
	sb.WriteString("\n")

	// Commands section
	sb.WriteString("## Commands\n")
	sb.WriteString("────────────\n")
	fmt.Fprintf(&sb, "  ✓ Pass:  %s\n", step.CompletionCommand)
	fmt.Fprintf(&sb, "  ✗ Fail:  %s\n", step.Metadata["fail_command"])
	fmt.Fprintf(&sb, "  → Skip:  %s\n", step.Metadata["skip_command"])

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

// HasBlockers returns true if any blocker items failed.
func (n *Navigator) HasBlockers() bool {
	for _, item := range n.checklist {
		if item.Severity == "blocker" && item.Status == "failed" {
			return true
		}
	}
	return false
}

// GetChecklist returns all checklist items.
func (n *Navigator) GetChecklist() []*ChecklistItem {
	return n.checklist
}

// AddNote adds a note to a checklist item.
func (n *Navigator) AddNote(itemID, note string) error {
	for _, item := range n.checklist {
		if item.ID == itemID {
			item.Notes = note
			return nil
		}
	}
	return errors.NotFound("checklist item not found").WithField("itemID", itemID)
}
