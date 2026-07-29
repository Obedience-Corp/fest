//go:build no_charm

package tui

import (
	"context"
	"fmt"
	"os"
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
	if _, err := os.Stat("WORKFLOW.md"); err == nil {
		return errors.Validation("WORKFLOW.md already exists in the current directory").
			WithHint("remove it or change directory before creating")
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
	if len(lines) == 0 {
		return errors.Validation("at least one workflow step is required")
	}

	draft := &workflowDraft{
		Name:        name,
		Title:       title,
		Description: desc,
		StepsText:   strings.Join(lines, "\n"),
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
	opts := &festival.CreateWorkflowOptions{
		Name:  name,
		Steps: stepsJSON,
	}
	if err := festival.RunCreateWorkflow(ctx, opts); err != nil {
		return err
	}
	fmt.Println()
	display.Info("Standalone workflow ready. next: fest next")
	return nil
}
