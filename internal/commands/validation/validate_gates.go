package validation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/gates"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newValidateQualityGatesCmd(parentOpts *validateOptions) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "quality-gates [festival-path]",
		Short: "Validate quality gates exist",
		Long: `Validate that implementation sequences have quality gate tasks.

Only implementation phases are checked. Other phase types are skipped.

Required implementation gates:
  • testing
  • review
  • iterate
  • commit

Use --fix to automatically add missing quality gates.
Sequences matching excluded patterns (*_planning, *_research, etc.) are skipped.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateQualityGates(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&opts.fix, "fix", false, "Automatically add missing quality gates")

	return cmd
}

func runValidateQualityGates(ctx context.Context, opts *validateOptions) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())
	if ctx == nil {
		ctx = context.Background()
	}

	festivalPath, err := resolveFestivalPath(opts.path)
	if err != nil {
		return emitValidateError(opts, err)
	}

	result := &ValidationResult{
		OK:       true,
		Action:   "validate_quality_gates",
		Festival: filepath.Base(festivalPath),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}

	validateQualityGatesChecks(ctx, festivalPath, result, opts.fix)

	result.Score = calculateScore(result)
	for _, issue := range result.Issues {
		if issue.Level == LevelError {
			result.Valid = false
			break
		}
	}

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printValidationSection(display, "QUALITY GATES", result.Issues)
	return nil
}

func validateQualityGatesChecks(ctx context.Context, festivalPath string, result *ValidationResult, autoFix bool) {
	parser := festival.NewParser()
	phases, _ := parser.ParsePhases(ctx, festivalPath)
	defaultPolicy := gates.DefaultPolicy()

	for _, phase := range phases {
		// Detect the actual phase type from frontmatter or directory name
		phaseType := gates.DetectPhaseType(phase.Path)

		// Only validate implementation phases
		if phaseType != "implementation" {
			continue
		}

		// Get the expected gates for this phase type
		expectedGates := gates.GetGatesForPhaseType(phaseType)
		gateIDs := make(map[string]bool)
		for _, gate := range expectedGates {
			if gate.Enabled {
				gateIDs[gate.ID] = true
			}
		}

		sequences, _ := parser.ParseSequences(ctx, phase.Path)
		for _, seq := range sequences {
			// Check sequence exclusion patterns from default policy
			if isExcludedSequence(seq.Name, defaultPolicy.ExcludePatterns) {
				continue
			}

			// Check for quality gate tasks
			tasks, _ := parser.ParseTasks(ctx, seq.Path)
			hasGates := false
			for _, task := range tasks {
				// Check if task name contains any gate ID
				for gateID := range gateIDs {
					if strings.Contains(strings.ToLower(task.Name), strings.ReplaceAll(gateID, "_", "")) ||
						strings.Contains(task.Name, gateID) {
						hasGates = true
						break
					}
				}
			}

			if !hasGates && len(tasks) > 0 {
				relPath, _ := filepath.Rel(festivalPath, seq.Path)
				// Phase-type-aware error message
				phaseTypeTitle := titleCase(phaseType)
				result.Issues = append(result.Issues, ValidationIssue{
					Level:       LevelError,
					Code:        CodeMissingQualityGate,
					Path:        relPath,
					Message:     fmt.Sprintf("%s sequence missing quality gates", phaseTypeTitle),
					Fix:         "fest gates apply --sequence " + relPath + " --approve",
					AutoFixable: true,
				})

				if autoFix {
					// TODO: Call fest gates apply
					result.FixesApplied = append(result.FixesApplied, FixApplied{
						Code:   CodeMissingQualityGate,
						Path:   relPath,
						Action: "Quality gates would be added (--fix not yet implemented)",
					})
				}
			}
		}
	}
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

// isExcludedSequence checks if a sequence name matches exclusion patterns
func isExcludedSequence(name string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple glob matching
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(name, suffix) {
				return true
			}
		} else if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		} else if name == pattern {
			return true
		}
	}
	return false
}
