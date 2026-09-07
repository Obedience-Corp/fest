// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// festivalPlanningActions lists the commands that move a festival from a
// scaffold to an executable plan, in the order a user runs them.
var festivalPlanningActions = []string{
	"fest wizard fill .",
	"fest create phase --name PHASE_NAME",
	"fest validate",
	"fest promote",
}

// routeUnplannedFestival emits the planning step for a festival that is still
// in planning status and has no phase to walk. Reporting it as a step is what
// keeps `fest create festival` followed by `fest next` working: the scaffold's
// own unfilled markers are expected at that point, and the selector would
// otherwise call a phase-less festival complete.
//
// Returns handled=false when normal routing should continue.
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
	return true, emitNextResult(ctx, festivalPath, result, opts)
}

// routeEmptyPlanningFestival emits the planning step for a festival that is
// still in planning status, already has phases, and holds no task or workflow
// step to run. Without it the selector reports "festival complete" for a plan
// that was never written.
//
// Returns handled=false when normal routing should continue.
func routeEmptyPlanningFestival(ctx context.Context, festivalPath, festivalStatus string, result *selection.NextTaskResult, opts RenderOptions) (bool, error) {
	if festivalStatus != config.StatusPlanning || result == nil || !result.FestivalComplete {
		return false, nil
	}
	// Only intercept when the total is known to be zero. A nil Progress means
	// the progress manager could not answer, and guessing there would hide a
	// genuinely finished festival.
	if result.Progress == nil || result.Progress.TotalTasks > 0 {
		return false, nil
	}
	phaseCount, err := countFestivalPhases(festivalPath)
	if err != nil {
		return false, err
	}
	planning, err := buildFestivalPlanningResult(ctx, festivalPath, festivalStatus, phaseCount)
	if err != nil {
		return false, err
	}
	return true, emitNextResult(ctx, festivalPath, planning, opts)
}

// buildFestivalPlanningResult assembles the planning step, including the
// inventory of files that still hold unfilled template markers.
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
		files = append(files, selection.PlanningMarkerFile{
			File:  file.RelPath,
			Count: file.MarkerCount,
		})
		total += file.MarkerCount
	}

	return &selection.NextTaskResult{
		Reason: festivalPlanningReason(len(files), phaseCount),
		Location: &selection.LocationInfo{
			FestivalPath: festivalPath,
			CurrentPath:  festivalPath,
		},
		FestivalPlanning: &selection.FestivalPlanningResult{
			Status:      festivalStatus,
			PhaseCount:  phaseCount,
			MarkerTotal: total,
			MarkerFiles: files,
			NextActions: festivalPlanningActions,
		},
	}, nil
}

// festivalPlanningReason states why there is no task to hand out.
func festivalPlanningReason(markerFiles, phaseCount int) string {
	switch {
	case markerFiles > 0 && phaseCount == 0:
		return fmt.Sprintf("Festival is in planning: %d documents still hold unfilled markers and no phases exist yet", markerFiles)
	case markerFiles > 0:
		return fmt.Sprintf("Festival is in planning: %d documents still hold unfilled markers and no step is ready to run", markerFiles)
	case phaseCount == 0:
		return "Festival is in planning and has no phases yet"
	default:
		return "Festival is in planning and has no task or workflow step to run yet"
	}
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
