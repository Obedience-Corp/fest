package validation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/validator"
	"github.com/spf13/cobra"
)

func newValidateChecklistCmd(parentOpts *validateOptions) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "checklist [festival-path]",
		Short: "Post-completion questionnaire",
		Long: `Run through post-completion checklist to ensure methodology compliance.

Checks (auto-verified where possible):
  1. Did you fill out ALL templates? (auto-check for markers)
  2. Does this plan achieve project goals? (manual review)
  3. Are items in order of operation? (auto-check)
  4. Did you follow parallelization standards? (auto-check)
  5. Did you create TASK FILES for implementation? (auto-check)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateChecklist(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

func runValidateChecklist(_ context.Context, opts *validateOptions) error {
	festivalPath, err := resolveFestivalPath(opts.path)
	if err != nil {
		return emitValidateError(opts, err)
	}

	result := &ValidationResult{
		OK:        true,
		Action:    "validate_checklist",
		Festival:  filepath.Base(festivalPath),
		Valid:     true,
		Issues:    []ValidationIssue{},
		Checklist: &Checklist{},
	}

	// Run all checks and populate checklist
	templatesFilled := validator.CheckTemplatesFilled(festivalPath)
	result.Checklist.TemplatesFilled = &templatesFilled

	// Goals achievable is a manual check - always null
	result.Checklist.GoalsAchievable = nil

	taskFilesExist := validator.CheckTaskFilesExist(festivalPath)
	result.Checklist.TaskFilesExist = &taskFilesExist

	orderCorrect := validator.CheckOrderCorrect(festivalPath)
	result.Checklist.OrderCorrect = &orderCorrect

	parallelCorrect := validator.CheckParallelCorrect(festivalPath)
	result.Checklist.ParallelCorrect = &parallelCorrect
	finalizeValidationResult(result)

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printChecklistResult(result.Checklist)
	return nil
}

// Output functions for checklist

func printChecklistResult(checklist *Checklist) {
	fmt.Println()
	fmt.Println(ui.H1("Post-Completion Checklist"))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	printCheckItem(1, "Templates Filled",
		"Did you fill out ALL templates?",
		checklist.TemplatesFilled,
		"Search for [REPLACE:], [FILL:], or {{ }} markers and replace with actual content")

	printCheckItem(2, "Goals Achievable",
		"Does this plan achieve project goals?",
		checklist.GoalsAchievable,
		"Review FESTIVAL_OVERVIEW.md and verify alignment")

	printCheckItem(3, "Task Files Exist",
		"Did you create TASK FILES for implementation?",
		checklist.TaskFilesExist,
		"Run 'fest understand tasks' to learn about task files")

	printCheckItem(4, "Order Correct",
		"Are items in order of operation?",
		checklist.OrderCorrect,
		"Lower numbers execute first (01_ before 02_)")

	printCheckItem(5, "Parallel Correct",
		"Did you follow parallelization standards?",
		checklist.ParallelCorrect,
		"Same-numbered items (01_a, 01_b) run in parallel")

	// Summary
	passed := 0
	total := 0
	if checklist.TemplatesFilled != nil {
		total++
		if *checklist.TemplatesFilled {
			passed++
		}
	}
	if checklist.TaskFilesExist != nil {
		total++
		if *checklist.TaskFilesExist {
			passed++
		}
	}
	if checklist.OrderCorrect != nil {
		total++
		if *checklist.OrderCorrect {
			passed++
		}
	}
	if checklist.ParallelCorrect != nil {
		total++
		if *checklist.ParallelCorrect {
			passed++
		}
	}

	fmt.Println()
	fmt.Printf("%s %s\n", ui.Label("Summary"), ui.Value(fmt.Sprintf("%d/%d auto-checks passed", passed, total)))
	fmt.Println(ui.Dim("1 manual check required"))
}

func printCheckItem(num int, title, question string, result *bool, guidance string) {
	fmt.Println()
	fmt.Println(ui.H3(fmt.Sprintf("%d. %s", num, title)))
	fmt.Printf("%s %s\n", ui.Label("Question"), ui.Dim(question))

	statusLabel := ""
	switch {
	case result == nil:
		statusLabel = ui.Warning("MANUAL CHECK REQUIRED")
	case *result:
		statusLabel = ui.Success("PASS")
	default:
		statusLabel = ui.Error("FAIL")
	}
	fmt.Printf("%s %s\n", ui.Label("Status"), statusLabel)

	if result == nil {
		fmt.Printf("%s %s\n", ui.Label("Guidance"), ui.Dim(guidance))
		return
	}

	if !*result {
		fmt.Printf("%s %s\n", ui.Label("Fix"), ui.Dim(guidance))
	}
}
