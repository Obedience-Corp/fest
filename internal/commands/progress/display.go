// Package progress implements the fest progress command for tracking execution progress.
package progress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func showProgressOverview(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	// Determine scope based on location
	switch loc.Type {
	case "sequence":
		return showSequenceProgress(ctx, mgr, loc, opts)
	case "phase":
		return showPhaseProgress(ctx, mgr, loc, opts)
	case "festival", "task":
		return showFestivalProgress(ctx, mgr, loc, opts)
	default:
		return showFestivalProgress(ctx, mgr, loc, opts)
	}
}

func showFestivalProgress(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	festProgress, err := mgr.GetFestivalProgress(ctx, loc.Festival.Path)
	if err != nil {
		return errors.Wrap(err, "calculating progress")
	}

	if opts.json {
		if err := shared.EncodeJSON(os.Stdout, festProgress); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Human-readable output
	fmt.Println(ui.H1("Festival Progress"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(festProgress.FestivalName, ui.FestivalColor))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Overall progress bar
	overall := festProgress.Overall
	fmt.Printf("\n%s %s %s %s\n",
		ui.Label("Overall"),
		renderProgressBar(overall.Percentage),
		ui.Value(fmt.Sprintf("%d%%", overall.Percentage)),
		ui.Dim(fmt.Sprintf("(%d/%d tasks)", overall.Completed, overall.Total)))

	if overall.Blocked > 0 {
		fmt.Printf("%s %s\n",
			ui.StateIcon("blocked"),
			ui.Value(fmt.Sprintf("%d task(s) blocked", overall.Blocked), ui.WarningColor))
	}

	// Time Tracking section
	fmt.Printf("\n%s\n", ui.H2("Time Tracking"))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Work time (sum of all task time) - agent runtime
	if overall.TimeSpentMin > 0 {
		fmt.Printf("%s %s %s\n",
			ui.Label("Work time"),
			ui.Value(ui.FormatDuration(overall.TimeSpentMin)),
			ui.Dim("(agent runtime)"))
	} else {
		fmt.Printf("%s %s %s\n",
			ui.Label("Work time"),
			ui.Dim("0m"),
			ui.Dim("(agent runtime)"))
	}

	// Duration (calendar time since festival creation)
	durationStr := progress.FormatDurationWithStatus(festProgress.TimeMetrics)
	fmt.Printf("%s %s %s\n",
		ui.Label("Duration"),
		ui.Value(durationStr),
		ui.Dim("(calendar time)"))

	// Phase-type time breakdown
	if len(festProgress.PhaseTypeTimes) > 0 {
		fmt.Printf("\n%s\n", ui.H3("Time by Phase Type"))
		for _, pt := range festProgress.PhaseTypeTimes {
			fmt.Printf("  %s %s\n",
				ui.Label(fmt.Sprintf("%-16s", pt.PhaseType)),
				ui.Value(ui.FormatDuration(pt.Minutes)))
		}
	}

	// Content delta (token proxy)
	festConfig, cfgErr := config.LoadFestivalConfig(loc.Festival.Path, "")
	if cfgErr == nil && festConfig.Metadata.InitialSizeBytes > 0 {
		delta, deltaErr := progress.ComputeContentDelta(ctx, loc.Festival.Path, festConfig.Metadata.InitialSizeBytes)
		if deltaErr == nil {
			fmt.Printf("\n%s\n", ui.H3("Content Delta"))
			fmt.Printf("  %s %s\n",
				ui.Label("Initial"),
				ui.Dim(fmt.Sprintf("%s (~%d tokens)", formatBytes(delta.InitialBytes), delta.InitialBytes/4)))
			fmt.Printf("  %s %s\n",
				ui.Label("Current"),
				ui.Dim(fmt.Sprintf("%s (~%d tokens)", formatBytes(delta.CurrentBytes), delta.CurrentBytes/4)))
			sign := "+"
			if delta.DeltaBytes < 0 {
				sign = ""
			}
			fmt.Printf("  %s %s\n",
				ui.Label("Delta"),
				ui.Value(fmt.Sprintf("%s%s (~%s%d tokens)", sign, formatBytes(delta.DeltaBytes), sign, delta.DeltaTokens)))
		}
	}

	// Phase breakdown
	if len(festProgress.Phases) > 0 {
		fmt.Printf("\n%s\n", ui.H2("Phases"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, phase := range festProgress.Phases {
			phasePath := filepath.Join(loc.Festival.Path, phase.PhaseName)

			// Check if this is a workflow phase
			if shared.HasWorkflowFile(phasePath) {
				steps, err := shared.LoadWorkflowStepsForPhase(ctx, loc.Festival.Path, phasePath)
				if err == nil && len(steps) > 0 {
					completed, total := shared.WorkflowStepCounts(steps)
					percentage := 0
					if total > 0 {
						percentage = completed * 100 / total
					}
					state := "pending"
					if completed == total && total > 0 {
						state = "completed"
					} else if completed > 0 {
						state = "in_progress"
					}
					fmt.Printf("%s %s %s %s\n",
						ui.StateIcon(state),
						ui.Value(phase.PhaseName, ui.PhaseColor),
						ui.Dim(fmt.Sprintf("%3d%%", percentage)),
						ui.Dim(fmt.Sprintf("(%d/%d steps)", completed, total)))
					continue
				}
			}

			// Implementation phase - show task counts
			state := "pending"
			if phase.Progress.Completed == phase.Progress.Total && phase.Progress.Total > 0 {
				state = "completed"
			} else if phase.Progress.InProgress > 0 || phase.Progress.Completed > 0 {
				state = "in_progress"
			}
			fmt.Printf("%s %s %s %s\n",
				ui.StateIcon(state),
				ui.Value(phase.PhaseName, ui.PhaseColor),
				ui.Dim(fmt.Sprintf("%3d%%", phase.Progress.Percentage)),
				ui.Dim(fmt.Sprintf("(%d/%d tasks)", phase.Progress.Completed, phase.Progress.Total)))
		}
	}

	// Show blockers if any
	if len(overall.Blockers) > 0 {
		fmt.Printf("\n%s\n", ui.H2("Blockers"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, blocker := range overall.Blockers {
			fmt.Printf("%s %s %s\n",
				ui.StateIcon("blocked"),
				ui.Value(blocker.TaskID, ui.TaskColor),
				ui.Dim(blocker.BlockerMessage))
		}
	}

	return nil
}

func showPhaseProgress(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	phasePath := filepath.Join(loc.Festival.Path, loc.Phase)

	// Check if this is a workflow phase
	if shared.HasWorkflowFile(phasePath) {
		return showWorkflowPhaseProgress(ctx, loc, opts)
	}

	phaseProgress, err := mgr.GetPhaseProgress(ctx, phasePath)
	if err != nil {
		return errors.Wrap(err, "calculating phase progress")
	}

	if opts.json {
		if err := shared.EncodeJSON(os.Stdout, phaseProgress); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Human-readable output
	fmt.Println(ui.H2("Phase Progress"))
	fmt.Printf("%s %s\n", ui.Label("Phase"), ui.Value(phaseProgress.PhaseName, ui.PhaseColor))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(loc.Festival.Name, ui.FestivalColor))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Phase progress bar
	prog := phaseProgress.Progress
	fmt.Printf("\n%s %s %s %s\n",
		ui.Label("Progress"),
		renderProgressBar(prog.Percentage),
		ui.Value(fmt.Sprintf("%d%%", prog.Percentage)),
		ui.Dim(fmt.Sprintf("(%d/%d tasks)", prog.Completed, prog.Total)))

	if prog.InProgress > 0 {
		fmt.Printf("%s %s\n",
			ui.Label("In progress"),
			ui.Value(fmt.Sprintf("%d", prog.InProgress), ui.InProgressColor))
	}

	if prog.Blocked > 0 {
		fmt.Printf("%s %s\n",
			ui.StateIcon("blocked"),
			ui.Value(fmt.Sprintf("%d task(s) blocked", prog.Blocked), ui.WarningColor))
	}

	if prog.TimeSpentMin > 0 {
		fmt.Printf("%s %s\n",
			ui.Label("Time spent"),
			ui.Value(fmt.Sprintf("%d min", prog.TimeSpentMin)))
	}

	// Show blockers if any
	if len(prog.Blockers) > 0 {
		fmt.Printf("\n%s\n", ui.H3("Blockers"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, blocker := range prog.Blockers {
			fmt.Printf("%s %s %s\n",
				ui.StateIcon("blocked"),
				ui.Value(blocker.TaskID, ui.TaskColor),
				ui.Dim(blocker.BlockerMessage))
		}
	}

	return nil
}

func showWorkflowPhaseProgress(ctx context.Context, loc *show.LocationInfo, opts *progressOptions) error {
	phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
	steps, err := shared.LoadWorkflowStepsForPhase(ctx, loc.Festival.Path, phasePath)
	if err != nil {
		return errors.Wrap(err, "loading workflow steps")
	}

	completed, total := shared.WorkflowStepCounts(steps)
	percentage := 0
	if total > 0 {
		percentage = completed * 100 / total
	}

	if opts.json {
		result := map[string]any{
			"phase_name": loc.Phase,
			"phase_type": "workflow",
			"steps":      steps,
			"progress": map[string]any{
				"completed":  completed,
				"total":      total,
				"percentage": percentage,
			},
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Human-readable output
	fmt.Println(ui.H2("Workflow Progress"))
	fmt.Printf("%s %s\n", ui.Label("Phase"), ui.Value(loc.Phase, ui.PhaseColor))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(loc.Festival.Name, ui.FestivalColor))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Progress bar
	fmt.Printf("\n%s %s %s %s\n",
		ui.Label("Progress"),
		renderProgressBar(percentage),
		ui.Value(fmt.Sprintf("%d%%", percentage)),
		ui.Dim(fmt.Sprintf("(%d/%d steps)", completed, total)))

	// Render steps
	fmt.Printf("\n%s\n", ui.H3("Steps"))
	fmt.Print(shared.RenderWorkflowSteps(steps, false))

	return nil
}

func showSequenceProgress(ctx context.Context, mgr *progress.Manager, loc *show.LocationInfo, opts *progressOptions) error {
	seqPath := filepath.Join(loc.Festival.Path, loc.Phase, loc.Sequence)
	seqProgress, err := mgr.GetSequenceProgress(ctx, seqPath)
	if err != nil {
		return errors.Wrap(err, "calculating sequence progress")
	}

	if opts.json {
		if err := shared.EncodeJSON(os.Stdout, seqProgress); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Human-readable output
	fmt.Println(ui.H2("Sequence Progress"))
	fmt.Printf("%s %s\n", ui.Label("Sequence"), ui.Value(seqProgress.SequenceName, ui.SequenceColor))
	fmt.Printf("%s %s\n", ui.Label("Phase"), ui.Value(loc.Phase, ui.PhaseColor))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(loc.Festival.Name, ui.FestivalColor))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Sequence progress bar
	prog := seqProgress.Progress
	fmt.Printf("\n%s %s %s %s\n",
		ui.Label("Progress"),
		renderProgressBar(prog.Percentage),
		ui.Value(fmt.Sprintf("%d%%", prog.Percentage)),
		ui.Dim(fmt.Sprintf("(%d/%d tasks)", prog.Completed, prog.Total)))

	if prog.InProgress > 0 {
		fmt.Printf("%s %s\n",
			ui.Label("In progress"),
			ui.Value(fmt.Sprintf("%d", prog.InProgress), ui.InProgressColor))
	}

	if prog.Pending > 0 {
		fmt.Printf("%s %s\n",
			ui.Label("Pending"),
			ui.Value(fmt.Sprintf("%d", prog.Pending), ui.PendingColor))
	}

	if prog.Blocked > 0 {
		fmt.Printf("%s %s\n",
			ui.StateIcon("blocked"),
			ui.Value(fmt.Sprintf("%d task(s) blocked", prog.Blocked), ui.WarningColor))
	}

	if prog.TimeSpentMin > 0 {
		fmt.Printf("%s %s\n",
			ui.Label("Time spent"),
			ui.Value(fmt.Sprintf("%d min", prog.TimeSpentMin)))
	}

	// Show blockers if any
	if len(prog.Blockers) > 0 {
		fmt.Printf("\n%s\n", ui.H3("Blockers"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, blocker := range prog.Blockers {
			fmt.Printf("%s %s %s\n",
				ui.StateIcon("blocked"),
				ui.Value(blocker.TaskID, ui.TaskColor),
				ui.Dim(blocker.BlockerMessage))
		}
	}

	return nil
}

func renderProgressBar(percentage int) string {
	opts := ui.DefaultProgressBarOptions()
	opts.Current = percentage
	opts.Total = 100
	opts.Width = ProgressBarWidth
	opts.ShowPercentage = false
	opts.ShowFraction = false
	opts.FilledColor = ui.SuccessColor
	opts.EmptyColor = ui.BorderColor
	return ui.RenderProgressBar(opts)
}

// formatBytes formats a byte count in a human-readable way.
func formatBytes(b int64) string {
	abs := b
	if abs < 0 {
		abs = -abs
	}
	if abs < 1024 {
		return fmt.Sprintf("%d bytes", b)
	}
	return fmt.Sprintf("%.1f KB", float64(b)/1024)
}
