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

func newValidateStructureCmd(parentOpts *validateOptions) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "structure [festival-path]",
		Short: "Validate naming conventions and hierarchy",
		Long: `Validate that festival structure follows naming conventions:

  • Phases: NNN_PHASE_NAME (3-digit prefix, UPPERCASE)
  • Sequences: NN_sequence_name (2-digit prefix, lowercase)
  • Tasks: NN_task_name.md (2-digit prefix, lowercase, .md extension)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateStructure(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

func runValidateStructure(ctx context.Context, opts *validateOptions) error {
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
		Action:   "validate_structure",
		Festival: filepath.Base(festivalPath),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}

	validateStructureChecks(ctx, festivalPath, result)
	finalizeValidationResult(result)

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printValidationSection(display, "STRUCTURE", result.Issues)
	return nil
}

func validateStructureChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	issues, err := validator.ValidateStructure(ctx, festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeNamingConvention,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate structure: %v", err),
		})
		return
	}
	result.Issues = append(result.Issues, convertIssues(issues)...)
}
