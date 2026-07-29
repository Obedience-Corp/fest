//go:build no_charm

package tui

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/festival"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// tuiCreateWorkflow is the no_charm linear path for standalone WORKFLOW.md.
func tuiCreateWorkflow(ctx context.Context, display *ui.UI) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := refuseExistingStandaloneWorkflow("."); err != nil {
		return err
	}

	display.Info("New standalone workflow (WORKFLOW.md in current directory)")
	name := strings.TrimSpace(display.Prompt("Workflow name"))
	if name == "" {
		return errors.Validation("workflow name is required")
	}
	title := strings.TrimSpace(display.PromptDefault("Title (default: name)", name))
	if title == "" {
		title = name
	}
	desc := strings.TrimSpace(display.PromptDefault("Intent (optional)", ""))

	display.Info("Steps: enter Name|Goal per line; empty line ends.")
	display.Info("Leave empty for starter steps: Plan → Implement → Verify")
	var lines []string
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(display.Prompt("  step (empty to finish)"))
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	stepsText := strings.Join(lines, "\n")
	if strings.TrimSpace(stepsText) == "" {
		// Match Charm's starter recipe when the user enters no lines.
		stepsText = defaultWorkflowStepsText
		display.Info("Using starter steps: Plan, Implement, Verify")
	}
	// Validate before confirm (parity with Charm live validation).
	if _, err := parseWorkflowStepsText(stepsText); err != nil {
		return err
	}

	draft := &workflowDraft{
		Name:        name,
		Title:       title,
		Description: desc,
		StepsText:   stepsText,
	}
	display.Info(workflowConfirmSummary(draft))
	if !display.Confirm("Create WORKFLOW.md now?") {
		display.Info("Cancelled.")
		return nil
	}

	stepsJSON, err := workflowStepsJSON(draft)
	if err != nil {
		return err
	}
	// Match CLI BindCreateWorkflowFlags defaults (position=after).
	opts := &festival.CreateWorkflowOptions{
		Name:     name,
		Steps:    stepsJSON,
		Position: "after",
	}
	// RunCreateWorkflow owns success output (created / next: fest next).
	return festival.RunCreateWorkflow(ctx, opts)
}
