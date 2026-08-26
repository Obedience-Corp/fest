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

func newValidateCompletenessCmd(parentOpts *validateOptions) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "completeness [festival-path]",
		Short: "Validate required files exist",
		Long: `Validate that all required files exist:

  • FESTIVAL_OVERVIEW.md (required)
  • PHASE_GOAL.md in each phase (required)
  • SEQUENCE_GOAL.md in each sequence (required)
  • FESTIVAL_RULES.md (recommended)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateCompleteness(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

func runValidateCompleteness(ctx context.Context, opts *validateOptions) error {
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
		Action:   "validate_completeness",
		Festival: filepath.Base(festivalPath),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}

	validateCompletenessChecks(ctx, festivalPath, result)
	finalizeValidationResult(result)

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printValidationSection(display, "COMPLETENESS", result.Issues, !result.Valid)
	return nil
}

func validateCompletenessChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	issues, err := validator.ValidateCompleteness(ctx, festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeMissingFile,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate completeness: %v", err),
		})
		return
	}
	result.Issues = append(result.Issues, convertIssues(issues)...)
}
