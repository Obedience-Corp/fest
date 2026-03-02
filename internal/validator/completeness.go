package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// CompletenessValidator validates that required files exist.
type CompletenessValidator struct{}

// NewCompletenessValidator creates a new completeness validator.
func NewCompletenessValidator() *CompletenessValidator {
	return &CompletenessValidator{}
}

// Validate checks that all required files exist.
func (v *CompletenessValidator) Validate(ctx context.Context, path string) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var issues []Issue

	// Check FESTIVAL_OVERVIEW.md
	overviewPath := filepath.Join(path, "FESTIVAL_OVERVIEW.md")
	if !fileExists(overviewPath) {
		issues = append(issues, Issue{
			Level:   LevelError,
			Code:    CodeMissingFile,
			Path:    overviewPath,
			Message: "FESTIVAL_OVERVIEW.md is required",
			Fix:     "Create FESTIVAL_OVERVIEW.md with project goals and success criteria",
		})
	}

	// Check FESTIVAL_RULES.md (warning, not error)
	rulesPath := filepath.Join(path, "FESTIVAL_RULES.md")
	if !fileExists(rulesPath) {
		issues = append(issues, Issue{
			Level:   LevelWarning,
			Code:    CodeMissingFile,
			Path:    rulesPath,
			Message: "FESTIVAL_RULES.md is recommended",
		})
	}

	parser := festival.NewParser()
	phases, _ := parser.ParsePhases(ctx, path)

	for _, phase := range phases {
		if err := ctx.Err(); err != nil {
			return issues, err
		}

		// Check PHASE_GOAL.md
		phaseGoalPath := filepath.Join(phase.Path, "PHASE_GOAL.md")
		if !fileExists(phaseGoalPath) {
			issues = append(issues, Issue{
				Level:   LevelError,
				Code:    CodeMissingGoal,
				Path:    phaseGoalPath,
				Message: fmt.Sprintf("PHASE_GOAL.md required in %s", phase.FullName),
				Fix:     fmt.Sprintf("fest create phase --name %q --json", phase.Name),
			})
		}

		// Check sequences
		sequences, _ := parser.ParseSequences(ctx, phase.Path)

		// Implementation phases require at least one sequence
		if isImplementationPhaseFromPath(phase.Path, phase.Name) && len(sequences) == 0 {
			issues = append(issues, Issue{
				Level:   LevelError,
				Code:    CodeMissingSequence,
				Path:    phase.Path,
				Message: fmt.Sprintf("Implementation phase %s requires at least one sequence directory", phase.FullName),
				Fix:     fmt.Sprintf("fest create sequence --name \"implementation\" --path %s", phase.Path),
			})
		}

		for _, seq := range sequences {
			seqGoalPath := filepath.Join(seq.Path, "SEQUENCE_GOAL.md")
			if !fileExists(seqGoalPath) {
				issues = append(issues, Issue{
					Level:   LevelError,
					Code:    CodeMissingGoal,
					Path:    seqGoalPath,
					Message: fmt.Sprintf("SEQUENCE_GOAL.md required in %s", seq.FullName),
					Fix:     fmt.Sprintf("fest create sequence --name %q --json", seq.Name),
				})
			}
		}
	}

	return issues, nil
}

// ValidateCompleteness checks that all required files exist in a festival.
func ValidateCompleteness(ctx context.Context, path string) ([]Issue, error) {
	return NewCompletenessValidator().Validate(ctx, path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// isImplementationPhaseFromPath checks PHASE_GOAL.md frontmatter for the phase type.
// Falls back to the name-based heuristic for legacy festivals without frontmatter.
func isImplementationPhaseFromPath(phasePath, phaseName string) bool {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	content, err := os.ReadFile(goalPath)
	if err == nil {
		fm, _, fmErr := frontmatter.Parse(content)
		if fmErr == nil && fm != nil && fm.PhaseType != "" {
			return fm.PhaseType == frontmatter.PhaseTypeImplementation
		}
	}
	// Fallback: name-based heuristic for legacy festivals without frontmatter
	return isImplementationPhase(phaseName)
}

// isImplementationPhase returns true if a phase name indicates an implementation-type phase
// that requires numbered sequences and task files. Workflow phases (planning, research, ingest)
// and freeform phases (review, non_coding_action) are excluded.
// This is the legacy fallback used when PHASE_GOAL.md has no frontmatter.
func isImplementationPhase(phaseName string) bool {
	normalized := strings.ToUpper(phaseName)
	nonImpl := []string{"RESEARCH", "PLANNING", "PLAN", "DESIGN", "INGEST", "REVIEW", "NON_CODING", "ACTION"}
	for _, skip := range nonImpl {
		if strings.Contains(normalized, skip) {
			return false
		}
	}
	return true
}
