package validation

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/validator"
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
	issues, err := validator.ValidateQualityGates(ctx, festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeMissingQualityGate,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate quality gates: %v", err),
		})
		return
	}

	converted := convertIssues(issues)
	result.Issues = append(result.Issues, converted...)

	if autoFix {
		for _, issue := range converted {
			if issue.AutoFixable {
				result.FixesApplied = append(result.FixesApplied, FixApplied{
					Code:   issue.Code,
					Path:   issue.Path,
					Action: "Quality gates would be added (--fix not yet implemented)",
				})
			}
		}
	}
}
