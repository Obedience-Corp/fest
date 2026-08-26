package validation

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	gatescore "github.com/Obedience-Corp/fest/internal/gates"
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
	finalizeValidationResult(result)

	if opts.jsonOutput {
		return emitValidateJSON(result)
	}

	printValidationSection(display, "QUALITY GATES", result.Issues, !result.Valid)
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

	if autoFix {
		fixes, warnings, fixErr := applyMissingQualityGates(ctx, festivalPath, issues)
		if fixErr != nil {
			result.Issues = append(result.Issues, ValidationIssue{
				Level:   LevelError,
				Code:    CodeMissingQualityGate,
				Path:    festivalPath,
				Message: fmt.Sprintf("Failed to apply quality gate fixes: %v", fixErr),
			})
			return
		}
		result.FixesApplied = append(result.FixesApplied, fixes...)
		result.Warnings = append(result.Warnings, warnings...)

		issues, err = validator.ValidateQualityGates(ctx, festivalPath)
		if err != nil {
			result.Issues = append(result.Issues, ValidationIssue{
				Level:   LevelError,
				Code:    CodeMissingQualityGate,
				Path:    festivalPath,
				Message: fmt.Sprintf("Failed to re-validate quality gates after fixes: %v", err),
			})
			return
		}
	}

	result.Issues = append(result.Issues, convertIssues(issues)...)
}

func applyMissingQualityGates(ctx context.Context, festivalPath string, issues []validator.Issue) ([]FixApplied, []string, error) {
	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return nil, nil, err
	}

	generator, err := gatescore.NewTaskGenerator(ctx, festivalPath)
	if err != nil {
		return nil, nil, err
	}

	seenSequences := make(map[string]bool)
	var fixes []FixApplied
	var warnings []string

	for _, issue := range issues {
		if issue.Code != validator.CodeMissingQualityGate || !issue.AutoFixable || issue.Path == "" {
			continue
		}

		sequencePath := filepath.Join(festivalPath, issue.Path)
		if seenSequences[sequencePath] {
			continue
		}
		seenSequences[sequencePath] = true

		phasePath := filepath.Dir(sequencePath)
		phaseType := gatescore.DetectPhaseType(phasePath)
		configGates := festCfg.GetGatesForPhaseType(phaseType)
		if len(configGates) == 0 {
			continue
		}

		sequenceGates := make([]gatescore.GateTask, len(configGates))
		for i, gate := range configGates {
			sequenceGates[i] = gatescore.GateTaskFromQualityGateTask(gate)
		}

		results, seqWarnings, err := generator.GenerateForSequence(
			ctx,
			sequencePath,
			sequenceGates,
			gatescore.GenerateOptions{
				DryRun:  false,
				Force:   false,
				Verbose: shared.IsVerbose(),
			},
			festivalPath,
		)
		if err != nil {
			return nil, warnings, err
		}
		warnings = append(warnings, seqWarnings...)

		for _, change := range results {
			if change.Type != "create" {
				continue
			}
			fixes = append(fixes, FixApplied{
				Code:   CodeMissingQualityGate,
				Path:   change.Path,
				Action: "Added missing quality gate",
			})
		}
	}

	return fixes, warnings, nil
}
