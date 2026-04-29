// Package next provides the fest next command for task navigation.
package next

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// hasBlockingIssues returns true if the result contains error-level issues.
// Warnings and info-level issues pass through without blocking.
//
// When the current phase is a preparatory type (ingest, planning, research),
// unfilled template marker errors are skipped ONLY for festival-root files
// (path has no separator) or files inside the preparatory phase directory.
// Marker errors from other phases (e.g. implementation) still block.
func hasBlockingIssues(result *validator.Result, currentPhaseType, currentPhaseName string) bool {
	preparatory := isPreparatoryPhase(currentPhaseType)
	for _, issue := range result.Issues {
		if issue.Level != validator.LevelError {
			continue
		}
		if issue.Code == validator.CodeUnfilledTemplate && preparatory {
			if isMarkerInPreparatoryScope(issue.Path, currentPhaseName) {
				continue
			}
		}
		return true
	}
	return false
}

// isMarkerInPreparatoryScope returns true if the marker issue belongs to a
// festival-root file or to the current preparatory phase directory.
// Festival-root files have no path separator (e.g. "FESTIVAL_GOAL.md").
// Phase files start with the phase directory name (e.g. "001_INGEST/...").
func isMarkerInPreparatoryScope(issuePath, currentPhaseName string) bool {
	// Festival-root files (no separator) — always in scope for preparatory phases
	if !strings.Contains(issuePath, string(filepath.Separator)) && !strings.Contains(issuePath, "/") {
		return true
	}
	// Files inside the current preparatory phase directory
	if currentPhaseName != "" && strings.HasPrefix(issuePath, currentPhaseName+string(filepath.Separator)) {
		return true
	}
	// Also check with forward slash for cross-platform safety
	if currentPhaseName != "" && strings.HasPrefix(issuePath, currentPhaseName+"/") {
		return true
	}
	return false
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
	sb.WriteString("STOP — FESTIVAL VALIDATION FAILED\n")
	sb.WriteString("──────────────────────────────────\n")
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

