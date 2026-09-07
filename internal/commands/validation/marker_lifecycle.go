package validation

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// This file holds the one rule whose weight depends on where a festival sits in
// its lifecycle: unfilled template markers are the plan waiting to be written
// until the festival is promoted, and a defect afterwards.

func finalizeValidationResult(result *ValidationResult) {
	canonical := toValidatorResult(result)
	result.Score = validator.CalculateScore(canonical)
	result.MarkersPending = canonical.HasPendingMarkers()
	result.Valid = validationResultIsClean(result)
}

// validationResultIsClean reports whether the festival passes validation.
//
// Unfilled template markers are the one issue class whose weight depends on
// the festival's lifecycle status. Until the festival is promoted they are an
// expected state: markers_pending records them, their festival-root findings
// report as warnings, and validation still passes. Once the festival is ready
// or active the plan is supposed to be written, so the same markers report as
// errors and validation fails.
func validationResultIsClean(result *ValidationResult) bool {
	for _, issue := range result.Issues {
		if validator.IsPendingMarker(issue.Code) {
			if !result.markersBlocking {
				continue
			}
			return false
		}
		if issue.Level == LevelError || issue.Level == LevelWarning {
			return false
		}
	}

	if len(result.Warnings) > 0 {
		return false
	}

	return !checklistHasFailures(result.Checklist)
}

// validationHasBlockingFailures drives the exit status and the failure banner.
// It follows the same lifecycle rule as validationResultIsClean so that valid,
// the banner, and the exit code never disagree.
func validationHasBlockingFailures(result *ValidationResult) bool {
	for _, issue := range result.Issues {
		if validator.IsPendingMarker(issue.Code) {
			if result.markersBlocking {
				return true
			}
			continue
		}
		if issue.Level == LevelError {
			return true
		}
	}

	return checklistHasFailures(result.Checklist)
}

// applyMarkerLifecycleLevels reports festival-root markers at warning level
// until the festival is promoted out of planning.
//
// The canonical scanner reads severity from the phase that contains a file, and
// a festival-root document belongs to no phase, so it falls back to the
// implementation default and every fresh scaffold reports errors. Root markers
// are the plan itself, and writing the plan is what planning status is for.
func applyMarkerLifecycleLevels(ctx context.Context, festivalPath string, issues []ValidationIssue) {
	if config.FestivalPromoted(ctx, festivalPath) {
		return
	}
	for i := range issues {
		if issues[i].Code == CodeUnfilledTemplate && isFestivalRootIssuePath(issues[i].Path) {
			issues[i].Level = LevelWarning
		}
	}
}

// isFestivalRootIssuePath reports whether an issue path names a file at the
// festival root. Marker issue paths are relative to the festival, so a root
// document has no path separator.
func isFestivalRootIssuePath(issuePath string) bool {
	return !strings.Contains(issuePath, string(filepath.Separator)) &&
		!strings.Contains(issuePath, "/")
}
