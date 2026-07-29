//go:build !no_charm

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/festival"
	uitheme "github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/charmbracelet/huh"
)

// charmCreateWorkflow runs the multi-step human wizard for standalone WORKFLOW.md.
// Creates in the current directory via festival.RunCreateWorkflow (same path as
// fest create workflow <name> --steps '{...}').
func charmCreateWorkflow(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Refuse early when cwd already has a workflow doc or runtime (avoid multi-step then fail).
	if err := refuseExistingStandaloneWorkflow("."); err != nil {
		return err
	}

	draft := workflowDraft{}
	step := 1
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch step {
		case 1:
			ok, err := stepWorkflowIdentity(ctx, &draft)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			step = 2
		case 2:
			action, err := stepWorkflowSteps(ctx, &draft)
			if err != nil {
				return err
			}
			switch action {
			case "next":
				step = 3
			case "back":
				step = 1
			default:
				return nil
			}
		case 3:
			action, err := stepWorkflowConfirm(ctx, &draft)
			if err != nil {
				return err
			}
			switch action {
			case "create":
				return submitWorkflowCreate(ctx, &draft)
			case "back":
				step = 2
			default:
				return nil
			}
		default:
			return fmt.Errorf("unknown workflow create step %d", step)
		}
	}
}

func stepWorkflowIdentity(ctx context.Context, d *workflowDraft) (bool, error) {
	// Inputs and nav are separate so Back is never blocked by name validation.
	name, title, desc := d.Name, d.Title, d.Description
	input := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New standalone workflow · Identity").
				Description("Creates WORKFLOW.md in the current directory (thin start — not a festival)."),
			huh.NewInput().
				Title("Name").
				Description("Used for workflow_id (wf-<slug>)").
				Placeholder("my-feature-loop").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Title").
				Description("WORKFLOW.md heading (default: name)").
				Placeholder("optional").
				Value(&title),
			huh.NewInput().
				Title("Intent").
				Description("One-line purpose (optional)").
				Placeholder("What this loop is for").
				Value(&desc),
		),
	)
	if err := uitheme.RunForm(ctx, input); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}

	nav := "next"
	navForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Continue?").
				Description("j/k · enter · esc cancel").
				Options(
					huh.NewOption("Next: steps", "next"),
					huh.NewOption("Back", "back"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&nav),
		),
	)
	if err := uitheme.RunForm(ctx, navForm); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	switch nav {
	case "next":
		d.Name = strings.TrimSpace(name)
		d.Title = strings.TrimSpace(title)
		d.Description = strings.TrimSpace(desc)
		return true, nil
	default:
		return false, nil
	}
}

func stepWorkflowSteps(ctx context.Context, d *workflowDraft) (string, error) {
	text := d.StepsText
	if text == "" {
		text = defaultWorkflowStepsText
	}
	input := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New standalone workflow · Steps").
				Description("One step per line: Name|Goal\nExample: Review PR|Check tests and leave comments"),
			huh.NewText().
				Title("Steps").
				Description("Name|Goal per line · empty goal gets a placeholder").
				Value(&text).
				CharLimit(4000).
				Validate(func(s string) error {
					_, err := parseWorkflowStepsText(s)
					return err
				}),
		),
	)
	if err := uitheme.RunForm(ctx, input); err != nil {
		if uitheme.IsCancelled(err) {
			return "cancel", nil
		}
		return "", err
	}
	d.StepsText = text

	nav := "next"
	navForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Continue?").
				Description("j/k · enter · esc cancel").
				Options(
					huh.NewOption("Next: confirm", "next"),
					huh.NewOption("Back", "back"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&nav),
		),
	)
	if err := uitheme.RunForm(ctx, navForm); err != nil {
		if uitheme.IsCancelled(err) {
			return "cancel", nil
		}
		return "", err
	}
	return nav, nil
}

func stepWorkflowConfirm(ctx context.Context, d *workflowDraft) (string, error) {
	action := "create"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New standalone workflow · Confirm").
				Description(workflowConfirmSummary(d)),
			huh.NewSelect[string]().
				Title("Create WORKFLOW.md?").
				Description("j/k · enter · esc cancel").
				Options(
					huh.NewOption("Create", "create"),
					huh.NewOption("Back", "back"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&action),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return "cancel", nil
		}
		return "", err
	}
	return action, nil
}

func submitWorkflowCreate(ctx context.Context, d *workflowDraft) error {
	stepsJSON, err := workflowStepsJSON(d)
	if err != nil {
		return err
	}
	// Match CLI BindCreateWorkflowFlags defaults (position=after) so festival-mode
	// dispatch from a festival cwd works the same as `fest create workflow --steps`.
	opts := &festival.CreateWorkflowOptions{
		Name:     strings.TrimSpace(d.Name),
		Steps:    stepsJSON,
		Position: "after",
	}
	// RunCreateWorkflow owns success output (created / next: fest next).
	return festival.RunCreateWorkflow(ctx, opts)
}
