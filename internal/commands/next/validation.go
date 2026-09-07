// Package next provides the fest next command for task navigation.
package next

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// hasBlockingIssues returns true if the result contains error-level issues.
// Warnings and info-level issues pass through without blocking.
//
// Unfilled template marker errors are skipped where markers are still an
// expected state; see markerIsExpected. Every other error-level issue blocks.
func hasBlockingIssues(result *validator.Result, currentPhaseType, currentPhaseName, festivalStatus string) bool {
	for _, issue := range result.Issues {
		if issue.Level != validator.LevelError {
			continue
		}
		if issue.Code == validator.CodeUnfilledTemplate &&
			markerIsExpected(issue.Path, currentPhaseType, currentPhaseName, festivalStatus) {
			continue
		}
		return true
	}
	return false
}

// markerIsExpected reports whether an unfilled-marker issue is a normal state
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
func markerIsExpected(issuePath, currentPhaseType, currentPhaseName, festivalStatus string) bool {
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
// markers are expected and should not block fest next.
func isPreparatoryPhase(phaseType string) bool {
	switch phaseType {
	case "ingest", "planning", "research":
		return true
	}
	return false
}

// emitValidationBlock prints a blocking message when the festival fails validation.
func emitValidationBlock(festivalPath string, result *validator.Result) error {
	var sb strings.Builder
	sb.WriteString("STOP: FESTIVAL VALIDATION FAILED\n")
	sb.WriteString(strings.Repeat("─", 32) + "\n")
	sb.WriteString("This festival has issues that must be fixed before continuing.\n\n")

	// Collect errors and warnings separately
	var errs, warns []validator.Issue
	for _, issue := range result.Issues {
		switch issue.Level {
		case validator.LevelError:
			errs = append(errs, issue)
		case validator.LevelWarning:
			warns = append(warns, issue)
		}
	}

	if len(errs) > 0 {
		sb.WriteString("Errors:\n")
		for _, issue := range errs {
			path := issue.Path
			if rel, err := filepath.Rel(festivalPath, path); err == nil {
				path = rel
			}
			fmt.Fprintf(&sb, "  ✗ %s: %s\n", path, issue.Message)
		}
	}

	if len(warns) > 0 {
		if len(errs) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Warnings:\n")
		for _, issue := range warns {
			path := issue.Path
			if rel, err := filepath.Rel(festivalPath, path); err == nil {
				path = rel
			}
			fmt.Fprintf(&sb, "  ⚠ %s: %s\n", path, issue.Message)
		}
	}

	// Check for auto-link specific issues and provide targeted guidance
	hasAutoLinkErrors := false
	for _, issue := range errs {
		if strings.HasPrefix(issue.Code, "autolink_") {
			hasAutoLinkErrors = true
			break
		}
	}

	if hasAutoLinkErrors {
		sb.WriteString("\nAuto-link fix:\n")
		sb.WriteString("  Add fest_working_dir to SEQUENCE_GOAL.md frontmatter:\n")
		sb.WriteString("    fest_working_dir: \"projects/your-project\"\n\n")
		sb.WriteString("  To disable auto-link validation, set in fest.yaml:\n")
		sb.WriteString("    auto_link:\n")
		sb.WriteString("      enabled: false\n")
	}

	sb.WriteString("\nRun 'fest validate' for full details, then fix the issues.\n")
	sb.WriteString("Do not proceed with tasks until the festival passes validation.\n")
	fmt.Print(sb.String())
	return errors.ErrAlreadyPrinted
}
