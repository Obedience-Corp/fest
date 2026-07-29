//go:build !no_charm

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	uitheme "github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	// Register TUI hooks with the shared package
	shared.NewTUICommand = NewTUICommand
	shared.StartCreateTUI = StartCreateTUI
	shared.StartCreateFestivalTUI = StartCreateFestivalTUI
	shared.StartCreatePhaseTUI = StartCreatePhaseTUI
	shared.StartCreateSequenceTUI = StartCreateSequenceTUI
	shared.StartCreateTaskTUI = StartCreateTaskTUI
}

// NewTUICommand (charm version) provides a richer interactive UI using Charm libs
func NewTUICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive UI (Charm) for creating festivals, phases, sequences, and tasks",
		Annotations: map[string]string{
			"agent_allowed": "false",
			"agent_reason":  "Full interactive Charm TUI",
			"interactive":   "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCharmTUI(cmd.Context())
		},
	}
	return cmd
}

func runCharmTUI(ctx context.Context) error {
	// Validate workspace is resolved (scope middleware already did this)
	if _, ok := scope.WorkspaceFrom(ctx); !ok {
		var initNow bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().Title("No festivals/ directory detected. Initialize here?").Value(&initNow),
			),
		)
		if err := uitheme.RunForm(ctx, form); err != nil {
			if uitheme.IsCancelled(err) {
				return nil // Clean exit on Ctrl-C
			}
			return err
		}
		if initNow {
			if err := shared.RunInit(ctx, ".", &shared.InitOpts{}); err != nil {
				return err
			}
		} else {
			return errors.NotFound("festivals directory")
		}
	}

	// main menu loop — Create… reuses the same Create what? menu as `fest create`
	for {
		var action string
		menu := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Description("j/k or ↑/↓ navigate • enter select • esc/ctrl-c quit").
					Options(
						huh.NewOption("Create…", "create"),
						huh.NewOption("Generate Festival Goal", "festival_goal"),
						huh.NewOption("Quit", "quit"),
					).
					Value(&action),
			),
		)

		if err := uitheme.RunForm(ctx, menu); err != nil {
			if uitheme.IsCancelled(err) {
				return nil // Clean exit on Ctrl-C
			}
			return err
		}

		switch action {
		case "create":
			if err := StartCreateTUI(ctx); err != nil {
				return err
			}
		case "festival_goal":
			if err := charmGenerateFestivalGoal(ctx); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// notEmpty validates that a string is not empty
func notEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.Validation("value required")
	}
	return nil
}

// toOptions converts a slice of strings to huh options
func toOptions(values []string) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(values))
	for _, v := range values {
		opts = append(opts, huh.NewOption(v, v))
	}
	return opts
}

// fallbackDot returns "." if string is empty, otherwise returns the string
func fallbackDot(s string) string {
	if strings.TrimSpace(s) == "" {
		return "."
	}
	return s
}

// StartCreateTUI shows a create-only menu (Charm implementation).
// Outer Create what? menu: Festival wizard + Workflow peer + hierarchy creates.
func StartCreateTUI(ctx context.Context) error {
	for {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		var action string
		// Short option labels; details live in the select Description. RunForm
		// draws a full-width frame around the menu.
		menu := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Create what?").
					Description("j/k or ↑/↓ navigate · enter select · esc/ctrl-c quit").
					Options(
						huh.NewOption("Festival", "festival"),
						huh.NewOption("Standalone workflow", "workflow"),
						huh.NewOption("Phase", "phase"),
						huh.NewOption("Sequence", "sequence"),
						huh.NewOption("Task", "task"),
						huh.NewOption("Back", "back"),
					).
					Value(&action),
			),
		)
		if err := uitheme.RunForm(ctx, menu); err != nil {
			if uitheme.IsCancelled(err) {
				return nil // Clean exit on Ctrl-C
			}
			return err
		}
		switch action {
		case "festival":
			if err := charmCreateFestival(ctx); err != nil {
				return err
			}
		case "workflow":
			// Full human wizard deferred (intent). Peer must appear on the menu.
			fmt.Println()
			fmt.Println("Standalone workflow TUI is not ready yet.")
			fmt.Println("  Agents/humans can create with:")
			fmt.Println("    fest create workflow <name>")
			fmt.Println("  Or with structured steps:")
			fmt.Println("    fest create workflow <name> --steps-file steps.json")
			fmt.Println()
		case "phase":
			if err := charmCreatePhase(ctx); err != nil {
				return err
			}
		case "sequence":
			if err := charmCreateSequence(ctx); err != nil {
				return err
			}
		case "task":
			if err := charmCreateTask(ctx); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// StartCreateFestivalTUI starts the festival creation TUI directly
func StartCreateFestivalTUI(ctx context.Context) error { return charmCreateFestival(ctx) }

// StartCreatePhaseTUI starts the phase creation TUI directly
func StartCreatePhaseTUI(ctx context.Context) error { return charmCreatePhase(ctx) }

// StartCreateSequenceTUI starts the sequence creation TUI directly
func StartCreateSequenceTUI(ctx context.Context) error { return charmCreateSequence(ctx) }

// StartCreateTaskTUI starts the task creation TUI directly
func StartCreateTaskTUI(ctx context.Context) error { return charmCreateTask(ctx) }
