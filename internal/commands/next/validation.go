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
// Warnings and info-level issues pass through without blocking. The marker
// allowance lives in validator.HasBlockingIssues so fest promote applies the
// same rule.
func hasBlockingIssues(result *validator.Result, currentPhaseType, currentPhaseName, festivalStatus string) bool {
	return validator.HasBlockingIssues(result, currentPhaseType, currentPhaseName, festivalStatus)
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
