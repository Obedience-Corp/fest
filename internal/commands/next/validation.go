// Package next provides the fest next command for task navigation.
package next

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/validator"
)

// hasBlockingIssues returns true if the result contains error-level issues.
// Warnings and info-level issues pass through without blocking.
// When the current phase is a preparatory type (ingest, planning, research),
// unfilled template markers are skipped — those phases exist to fill them.
func hasBlockingIssues(result *validator.Result, currentPhaseType string) bool {
	for _, issue := range result.Issues {
		if issue.Level != validator.LevelError {
			continue
		}
		// Ingest/planning/research phases are allowed to proceed despite
		// unfilled markers — filling them is part of those phases' work.
		if issue.Code == validator.CodeUnfilledTemplate && isPreparatoryPhase(currentPhaseType) {
			continue
		}
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

	sb.WriteString("\nRun 'fest validate' for full details, then fix the issues.\n")
	sb.WriteString("Do not proceed with tasks until the festival passes validation.\n")
	fmt.Print(sb.String())
	return errors.ErrAlreadyPrinted
}

// checkPreActiveStatus blocks implementation/review phases when the festival is in planning or ready status.
func checkPreActiveStatus(ctx context.Context, festivalPath, phasePath string) error {
	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return nil // Can't load config — don't block
	}

	status := festCfg.Metadata.CurrentStatus()
	if status != "planning" && status != "ready" {
		return nil
	}

	// Only block if we can detect the phase type
	if phasePath == "" {
		// At festival root — find the first *incomplete* phase (the current one)
		found, _, findErr := findFirstIncompletePhase(ctx, festivalPath)
		if findErr != nil || found == "" {
			return nil
		}
		phasePath = found
	}

	phaseType := guidance.DetectPhaseType(phasePath)
	if phaseType != "implementation" && phaseType != "review" {
		return nil
	}

	festName := filepath.Base(festivalPath)

	if status == "ready" {
		var sb strings.Builder
		sb.WriteString("STOP — FESTIVAL NOT YET ACTIVE\n")
		sb.WriteString("──────────────────────────────\n")
		fmt.Fprintf(&sb, "Festival %q is in ready status.\n", festName)
		sb.WriteString("Lifecycle: planning -> ready -> [active] -> completed\n\n")
		sb.WriteString("Before executing, confirm:\n")
		sb.WriteString("  - Is this the correct festival you should be working on?\n")
		sb.WriteString("  - Did the user approve implementation of this festival?\n\n")
		sb.WriteString("If approved, promote to active: fest promote\n")
		fmt.Print(sb.String())
		return errors.ErrAlreadyPrinted
	}

	var sb strings.Builder
	sb.WriteString("STOP — FESTIVAL NOT YET ACTIVE\n")
	sb.WriteString("──────────────────────────────\n")
	fmt.Fprintf(&sb, "Festival %q is in planning status.\n", festName)
	sb.WriteString("Lifecycle: [planning] -> ready -> active -> completed\n\n")
	sb.WriteString("Implementation phases require active status.\n")
	sb.WriteString("Promote to ready first: fest promote\n")
	fmt.Print(sb.String())
	return errors.ErrAlreadyPrinted
}
