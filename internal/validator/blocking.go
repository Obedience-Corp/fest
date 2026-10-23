package validator

import (
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
)

// HasBlockingIssues returns true if the result contains error-level issues.
// Warnings and info-level issues pass through without blocking.
//
// Unfilled template marker errors are skipped where markers are still an
// expected state; see MarkerIsExpected. Every other error-level issue blocks.
func HasBlockingIssues(result *Result, currentPhaseType, currentPhaseName, festivalStatus string) bool {
	return len(BlockingIssues(result, currentPhaseType, currentPhaseName, festivalStatus)) > 0
}

// BlockingIssues returns the error-level issues that block work, applying the
// same marker allowance as HasBlockingIssues.
func BlockingIssues(result *Result, currentPhaseType, currentPhaseName, festivalStatus string) []Issue {
	var blocking []Issue
	for _, issue := range result.Issues {
		if issue.Level != LevelError {
			continue
		}
		if issue.Code == CodeUnfilledTemplate &&
			MarkerIsExpected(issue.Path, currentPhaseType, currentPhaseName, festivalStatus) {
			continue
		}
		blocking = append(blocking, issue)
	}
	return blocking
}

// MarkerIsExpected reports whether an unfilled-marker issue is a normal state
// for this festival rather than a defect that should stop work.
//
// Festival-root documents (FESTIVAL_GOAL.md and its siblings, paths with no
// separator) carry markers from the moment they are scaffolded until the plan
// is written, so they are expected until the festival is promoted. Once it is
// ready or active those markers are real errors. A festival whose status
// cannot be read is treated as unpromoted here and stopped a moment later by
// lifecycle.EnforcePreActive, which fails closed on an undetermined status.
//
// Inside a phase, markers are expected only while that phase is the current one
// and its type is preparatory (ingest, planning, research). Markers in any
// other phase directory, an implementation phase most of all, still block.
func MarkerIsExpected(issuePath, currentPhaseType, currentPhaseName, festivalStatus string) bool {
	if isFestivalRootPath(issuePath) {
		return festivalStatus != config.StatusReady && festivalStatus != config.StatusActive
	}
	if currentPhaseName == "" || !isPreparatoryPhase(currentPhaseType) {
		return false
	}
	return strings.HasPrefix(issuePath, currentPhaseName+string(filepath.Separator)) ||
		strings.HasPrefix(issuePath, currentPhaseName+"/")
}

// isFestivalRootPath reports whether a validator issue path names a file at the
// festival root. Issue paths are relative to the festival, so a root file has
// no path separator (e.g. "FESTIVAL_GOAL.md") while a phase file starts with
// its phase directory (e.g. "001_INGEST/...").
func isFestivalRootPath(issuePath string) bool {
	return !strings.Contains(issuePath, string(filepath.Separator)) &&
		!strings.Contains(issuePath, "/")
}

// isPreparatoryPhase returns true for phase types where unfilled template
// markers are expected and should not block work.
func isPreparatoryPhase(phaseType string) bool {
	switch phaseType {
	case "ingest", "planning", "research":
		return true
	}
	return false
}
