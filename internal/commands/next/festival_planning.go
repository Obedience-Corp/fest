// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	fcontext "github.com/Obedience-Corp/fest/internal/context"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// festivalPlanningCommands lists the commands that turn a scaffold into an
// executable plan, in the order an agent runs them. Filling markers is an edit
// rather than a command, and 'fest wizard fill' needs a terminal, so neither
// appears here.
var festivalPlanningCommands = []string{
	"fest understand planning",
	"fest understand structure",
	"fest create phase --name PHASE_NAME --type TYPE",
	"fest create sequence --name SEQUENCE_NAME",
	"fest create task --name TASK_NAME",
	"fest validate",
	"fest promote",
}

// routeUnplannedFestival emits the planning step for a festival that is still
// in planning status and has no phase to walk. Reporting it as a step is what
// keeps `fest create festival` followed by `fest next` working: the scaffold's
// own unfilled markers are expected at that point, and the selector would
// otherwise call a phase-less festival complete.
//
// Once a phase exists, normal routing takes over and this returns
// handled=false.
func routeUnplannedFestival(ctx context.Context, festivalPath, festivalStatus string, opts RenderOptions) (bool, error) {
	if festivalStatus != config.StatusPlanning {
		return false, nil
	}
	phaseCount, err := countFestivalPhases(festivalPath)
	if err != nil {
		return false, err
	}
	if phaseCount > 0 {
		return false, nil
	}
	result, err := buildFestivalPlanningResult(ctx, festivalPath, festivalStatus, phaseCount)
	if err != nil {
		return false, err
	}
	return true, emitFestivalPlanningStep(result, opts)
}

// emitFestivalPlanningStep renders the planning step in the selected output
// mode. The modes that can only name a task file have nothing to name here.
func emitFestivalPlanningStep(result *selection.NextTaskResult, opts RenderOptions) error {
	switch {
	case opts.Path, opts.CD, opts.ProjectDir:
		return errors.NotFound("no task available: this festival has no plan yet")
	case opts.Short:
		fmt.Println(selection.FormatShort(result))
	case opts.JSON:
		out, jsonErr := selection.FormatJSON(result)
		if jsonErr != nil {
			return errors.Parse("formatting JSON", jsonErr)
		}
		fmt.Println(out)
	case opts.Verbose:
		fmt.Print(selection.FormatVerbose(result, opts.showInlineContext()))
	default:
		fmt.Print(selection.FormatText(result, opts.showInlineContext()))
	}
	return nil
}

// buildFestivalPlanningResult assembles the planning step: the goal when one is
// written, every unfilled marker with its hint text, and the build commands.
func buildFestivalPlanningResult(ctx context.Context, festivalPath, festivalStatus string, phaseCount int) (*selection.NextTaskResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	scanned, err := validator.ScanTemplateMarkers(festivalPath)
	if err != nil {
		return nil, errors.Wrap(err, "scanning template markers")
	}

	files := make([]selection.PlanningMarkerFile, 0, len(scanned))
	total := 0
	for _, file := range scanned {
		markers := make([]selection.PlanningMarker, 0, len(file.Markers))
		for _, marker := range file.Markers {
			markers = append(markers, selection.PlanningMarker{
				Line: marker.Line,
				Hint: marker.Content,
			})
		}
		files = append(files, selection.PlanningMarkerFile{
			File:    file.RelPath,
			Count:   file.MarkerCount,
			Markers: markers,
		})
		total += file.MarkerCount
	}

	return &selection.NextTaskResult{
		Kind:   selection.KindFestivalPlanning,
		Reason: festivalPlanningReason(len(files)),
		Location: &selection.LocationInfo{
			FestivalPath: festivalPath,
			CurrentPath:  festivalPath,
		},
		FestivalPlanning: &selection.FestivalPlanningResult{
			Status:       festivalStatus,
			PhaseCount:   phaseCount,
			Goal:         festivalGoal(festivalPath),
			MarkerTotal:  total,
			MarkerFiles:  files,
			NextCommands: festivalPlanningCommands,
		},
	}, nil
}

// festivalGoal returns the festival's stated goal, from fest.yaml when
// `fest create festival --goal` recorded one and from FESTIVAL_GOAL.md
// otherwise. It returns empty while the goal is still a template marker, so
// the step never presents a placeholder as the objective.
func festivalGoal(festivalPath string) string {
	if cfg, err := config.LoadFestivalConfig(festivalPath, ""); err == nil {
		if goal := cfg.Metadata.Goal; goal != "" && !validator.ContainsTemplateMarker(goal) {
			return goal
		}
	}
	content, err := os.ReadFile(filepath.Join(festivalPath, "FESTIVAL_GOAL.md"))
	if err != nil {
		return ""
	}
	goal := fcontext.ExtractPrimaryGoal(content)
	if goal == "" || validator.ContainsTemplateMarker(goal) {
		return ""
	}
	return goal
}

// festivalPlanningReason states why there is no task to hand out.
func festivalPlanningReason(markerFiles int) string {
	if markerFiles > 0 {
		return fmt.Sprintf(
			"Festival is in planning: %d documents still hold unfilled markers and no phases exist yet",
			markerFiles)
	}
	return "Festival is in planning and has no phases yet"
}

// countFestivalPhases counts the numbered phase directories in a festival.
func countFestivalPhases(festivalPath string) (int, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return 0, errors.IO("reading festival directory", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && shared.IsNumberedDir(entry.Name()) {
			count++
		}
	}
	return count, nil
}
