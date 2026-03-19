package validator

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/gates"
)

// ValidateQualityGates checks that implementation sequences have quality gate tasks.
// Only implementation phases are validated — other phase types are skipped.
// Required implementation gates: testing, review, iterate, commit.
func ValidateQualityGates(ctx context.Context, festivalPath string) ([]Issue, error) {
	issues := []Issue{}

	parser := festival.NewParser()
	phases, err := parser.ParsePhases(ctx, festivalPath)
	if err != nil {
		return issues, errors.Wrap(err, "parsing phases").WithField("path", festivalPath)
	}

	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return issues, errors.Wrap(err, "loading festival config").WithField("path", festivalPath)
	}

	defaultPolicy := gates.DefaultPolicy()

	for _, phase := range phases {
		// Detect the actual phase type from frontmatter or directory name
		phaseType := gates.DetectPhaseType(phase.Path)

		// Only validate implementation phases
		if phaseType != "implementation" {
			continue
		}

		// Get the expected gates for this phase type from fest.yaml, with a policy fallback.
		expectedGates := festCfg.GetGatesForPhaseType(phaseType)
		if len(expectedGates) == 0 {
			for _, gate := range gates.GetGatesForPhaseType(phaseType) {
				expectedGates = append(expectedGates, config.QualityGateTask{
					ID:       gate.ID,
					Template: gate.Template,
					Name:     gate.Name,
					Enabled:  gate.Enabled,
				})
			}
		}

		expectedGateIDs := make(map[string]bool)
		for _, gate := range expectedGates {
			if !gate.Enabled {
				continue
			}
			expectedGateIDs[gate.ID] = true
		}

		sequences, err := parser.ParseSequences(ctx, phase.Path)
		if err != nil {
			return issues, errors.Wrap(err, "parsing sequences").WithField("phase", phase.Name)
		}

		for _, seq := range sequences {
			// Check sequence exclusion patterns from default policy
			if isExcludedSequence(seq.Name, defaultPolicy.ExcludePatterns) {
				continue
			}

			tasks, err := parser.ParseTasks(ctx, seq.Path)
			if err != nil {
				return issues, errors.Wrap(err, "parsing tasks").WithField("sequence", seq.Name)
			}

			foundGateIDs := make(map[string]bool)
			for _, task := range tasks {
				if gateID := gates.GetGateID(task.Path); gateID != "" {
					foundGateIDs[gateID] = true
				}
			}

			var missing []string
			for gateID := range expectedGateIDs {
				if !foundGateIDs[gateID] {
					missing = append(missing, gateID)
				}
			}

			sort.Strings(missing)
		if len(missing) > 0 && len(tasks) > 0 {
				rel, _ := filepath.Rel(festivalPath, seq.Path)
				// Phase-type-aware error message
				phaseTypeTitle := titleCase(phaseType)
				issues = append(issues, Issue{
					Level:       LevelError,
					Code:        CodeMissingQualityGate,
					Path:        rel,
					Message:     fmt.Sprintf("%s sequence missing quality gates: %s", phaseTypeTitle, strings.Join(missing, ", ")),
					Fix:         fmt.Sprintf("fest gates apply --sequence %s --approve", rel),
					AutoFixable: true,
				})
			}
		}
	}

	return issues, nil
}

// titleCase converts a string to title case (first letter uppercase)
func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Handle snake_case by replacing underscores with spaces and capitalizing
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}
