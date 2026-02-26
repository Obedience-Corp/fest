// Package promote implements the fest promote command for lifecycle transitions.
package promote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/status"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// validTransitions defines the allowed lifecycle promotions.
var validTransitions = map[string]string{
	"planning": "ready",
	"ready":    "active",
	"active":   "completed",
}

type promoteOptions struct {
	force    bool
	json     bool
	noCommit bool
}

// NewPromoteCommand creates the fest promote command.
func NewPromoteCommand() *cobra.Command {
	opts := &promoteOptions{}
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a festival to the next lifecycle status",
		Long: `Promote moves a festival through the lifecycle: planning → ready → active → completed.

Each transition validates readiness:
  planning → ready:    Festival goal must be defined
  ready → active:      Festival is ready to begin execution
  active → completed:  All tasks must be completed`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPromote(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "skip readiness validation")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&opts.noCommit, "no-commit", false, "skip auto-commit after promotion")

	return cmd
}

func runPromote(ctx context.Context, opts *promoteOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Detect current location
	loc, err := show.DetectCurrentLocation(ctx, cwd)
	if err != nil {
		return errors.Wrap(err, "detecting location")
	}
	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	festival := loc.Festival
	currentStatus := festival.Status

	// Determine next status
	nextStatus, ok := validTransitions[currentStatus]
	if !ok {
		if opts.json {
			return shared.EncodeJSON(os.Stdout, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("cannot promote festival with status %q", currentStatus),
				"status":  currentStatus,
			})
		}
		return errors.Validation("cannot promote festival").
			WithField("status", currentStatus).
			WithField("hint", "only planning, ready, and active festivals can be promoted")
	}

	// Validate readiness unless forced
	if !opts.force {
		if err := validateReadiness(ctx, festival, currentStatus, nextStatus); err != nil {
			if opts.json {
				return shared.EncodeJSON(os.Stdout, map[string]any{
					"success": false,
					"error":   err.Error(),
					"from":    currentStatus,
					"to":      nextStatus,
					"hint":    "use --force to skip validation",
				})
			}
			fmt.Printf("%s %s\n", ui.Warning("Promotion blocked"), ui.Dim(err.Error()))
			fmt.Printf("\n  %s\n", ui.Dim("Use --force to skip validation"))
			return nil
		}
	}

	// Execute the move using existing atomic status change
	newPath, err := status.AtomicStatusChange(ctx, festival.Path, currentStatus, nextStatus)
	if err != nil {
		return errors.Wrap(err, "promoting festival")
	}

	// Update FESTIVAL_GOAL.md frontmatter with the new status
	festivalGoalPath := filepath.Join(newPath, "FESTIVAL_GOAL.md")
	if _, statErr := os.Stat(festivalGoalPath); statErr == nil {
		if fmErr := status.UpdateGoalFrontmatter(ctx, festivalGoalPath, frontmatter.Status(nextStatus)); fmErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update FESTIVAL_GOAL.md frontmatter:"), ui.Dim(fmErr.Error()))
		}
	}

	// Update fest.yaml metadata with the new status
	var festivalID string
	if festCfg, cfgErr := config.LoadFestivalConfig(newPath, ""); cfgErr == nil {
		festivalID = festCfg.Metadata.ID
		festCfg.Metadata.AddStatusChange(nextStatus, newPath, "")
		if saveErr := config.SaveFestivalConfig(newPath, "", festCfg); saveErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update fest.yaml status:"), ui.Dim(saveErr.Error()))
		}
	}

	// Update navigation links after successful move
	status.UpdateNavigationAfterMove(festival.Name, nextStatus, newPath)

	// Auto-commit the status change unless --no-commit was specified
	var commitHash string
	if !opts.noCommit {
		hash, commitErr := status.AutoCommitStatusChange(ctx, festival.Name, festivalID, currentStatus, nextStatus)
		if commitErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: auto-commit failed:"), ui.Dim(commitErr.Error()))
		} else if hash != "" {
			commitHash = hash
		}
	}

	// Output result
	if opts.json {
		result := map[string]any{
			"success":  true,
			"festival": festival.Name,
			"from":     currentStatus,
			"to":       nextStatus,
			"new_path": newPath,
		}
		if commitHash != "" {
			result["commit"] = commitHash
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	fmt.Println(ui.H2("Festival Promoted"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(festival.Name, ui.FestivalColor))
	fmt.Printf("%s %s → %s\n",
		ui.Label("Status"),
		ui.GetStateStyle(currentStatus).Render(currentStatus),
		ui.GetStateStyle(nextStatus).Render(nextStatus))
	fmt.Printf("%s %s\n", ui.Label("New path"), ui.Dim(newPath))
	if commitHash != "" {
		fmt.Printf("%s %s\n", ui.Label("Commit"), ui.Dim(commitHash))
	}

	return nil
}

// validateReadiness checks if a festival is ready for the next lifecycle status.
func validateReadiness(ctx context.Context, festival *show.FestivalInfo, from, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch to {
	case "ready":
		return validatePlannedToReady(festival)
	case "active":
		return nil // ready → active has no additional requirements
	case "completed":
		return validateActiveToCompleted(ctx, festival)
	default:
		return nil
	}
}

// validatePlannedToReady checks that a festival is ready to move to the ready status.
func validatePlannedToReady(festival *show.FestivalInfo) error {
	// Festival goal must exist
	goalPath := filepath.Join(festival.Path, "FESTIVAL_GOAL.md")
	if _, err := os.Stat(goalPath); err != nil {
		return fmt.Errorf("FESTIVAL_GOAL.md is missing")
	}
	return nil
}

// validateActiveToCompleted checks that a festival is ready to be completed.
func validateActiveToCompleted(ctx context.Context, festival *show.FestivalInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mgr, mgrErr := progress.NewManager(ctx, festival.Path)
	if mgrErr != nil {
		return fmt.Errorf("could not initialize progress manager: %w", mgrErr)
	}
	festProgress, err := mgr.GetFestivalProgress(ctx, festival.Path)
	if err != nil {
		return fmt.Errorf("could not calculate progress: %w", err)
	}

	if festProgress.Overall.Total == 0 {
		return fmt.Errorf("festival has no tasks")
	}

	if festProgress.Overall.Completed < festProgress.Overall.Total {
		return fmt.Errorf("%d of %d tasks incomplete",
			festProgress.Overall.Total-festProgress.Overall.Completed,
			festProgress.Overall.Total)
	}

	return nil
}
