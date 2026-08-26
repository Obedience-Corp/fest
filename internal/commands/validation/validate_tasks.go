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

func newValidateTasksCmd(parentOpts *validateOptions) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "tasks [festival-path]",
		Short: "Validate task files exist (CRITICAL)",
		Long: `Validate that implementation sequences have TASK FILES.

THIS IS THE MOST COMMON MISTAKE: Creating sequences with only
SEQUENCE_GOAL.md but no task files.

  Goals define WHAT to achieve.
  Tasks define HOW to execute.

AI agents EXECUTE TASK FILES. Without them, agents know the objective
but don't know what specific work to perform.

CORRECT STRUCTURE:
  02_api/
  ├── SEQUENCE_GOAL.md          ← Defines objective
  ├── 01_design_endpoints.md    ← Task: design work
  ├── 02_implement_crud.md      ← Task: implementation
  └── 03_testing_and_verify.md  ← Quality gate

INCORRECT STRUCTURE (common mistake):
  02_api/
  └── SEQUENCE_GOAL.md          ← No task files!`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateTasks(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

func runValidateTasks(ctx context.Context, opts *validateOptions) error {
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
		Action:   "validate_tasks",
		Festival: filepath.Base(festivalPath),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}

	validateTaskFilesChecks(ctx, festivalPath, result)
	finalizeValidationResult(result)

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printTaskValidationResult(display, result)
	return nil
}

func validateTaskFilesChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	issues, err := validator.ValidateTasks(ctx, festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeMissingTaskFiles,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate tasks: %v", err),
		})
		return
	}
	result.Issues = append(result.Issues, convertIssues(issues)...)
}

func printTaskValidationSection(display *ui.UI, issues []ValidationIssue, overallFailed bool) {
	printSectionHeader("Task Files", issues)
	fmt.Println(ui.Dim("Critical for AI execution"))

	if len(issues) == 0 {
		if !overallFailed {
			display.Success("All implementation sequences have task files")
		}
		return
	}

	display.Error("Implementation sequences need task files, not just goals")
	fmt.Println()
	fmt.Println(ui.Info("Goals define what to achieve; tasks define how to execute."))
	fmt.Println(ui.Info("AI agents execute task files."))
	fmt.Println()
	fmt.Println(ui.H3("Sequences without tasks"))

	for _, issue := range issues {
		fmt.Printf("  - %s\n", ui.Dim(issue.Path))
	}

	fmt.Println()
	fmt.Println(ui.H3("For each sequence, create task files"))
	fmt.Println("  fest create task --name \"design\" --json")
	fmt.Println("  fest create task --name \"implement\" --json")
	fmt.Println("  fest create task --name \"test\" --json")
}

func printTaskValidationResult(display *ui.UI, result *ValidationResult) {
	printTaskValidationSection(display, result.Issues, !result.Valid)
}
