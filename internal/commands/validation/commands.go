package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	hookslib "github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/resident"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/validator"
	"github.com/spf13/cobra"
)

// Validation issue levels
const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelInfo    = "info"
)

// Validation issue codes
const (
	CodeMissingFile        = "missing_file"
	CodeMissingTaskFiles   = "missing_task_files"
	CodeMissingSequence    = "missing_sequence"
	CodeMissingQualityGate = "missing_quality_gates"
	CodeNamingConvention   = "naming_convention"
	CodeUnfilledTemplate   = "unfilled_template"
	CodeMissingGoal        = "missing_goal"
	CodeNumberingGap       = "numbering_gap"
	CodeHookShadowDrift    = "hook_shadow_drift"
	CodeHookResolveError   = "hook_resolve_error"
)

// ValidationIssue represents a single validation problem
type ValidationIssue struct {
	Level       string `json:"level"`
	Code        string `json:"code"`
	Path        string `json:"path"`
	Message     string `json:"message"`
	Fix         string `json:"fix,omitempty"`
	AutoFixable bool   `json:"auto_fixable"`
}

// Checklist represents post-completion checklist results
type Checklist struct {
	TemplatesFilled *bool `json:"templates_filled"`
	GoalsAchievable *bool `json:"goals_achievable"` // null = manual check required
	TaskFilesExist  *bool `json:"task_files_exist"`
	OrderCorrect    *bool `json:"order_correct"`
	ParallelCorrect *bool `json:"parallel_correct"`
}

// FixApplied represents a fix that was automatically applied
type FixApplied struct {
	Code   string `json:"code"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// MarkerInfo tracks template marker details
type MarkerInfo struct {
	TotalFiles       int      `json:"total_files"`
	TotalCount       int      `json:"total_count"`
	FilesWithMarkers []string `json:"files,omitempty"`
}

// ValidationResult represents the complete validation result
type ValidationResult struct {
	OK           bool              `json:"ok"`
	Action       string            `json:"action"`
	Festival     string            `json:"festival"`
	Valid        bool              `json:"valid"`
	Score        int               `json:"score"`
	Issues       []ValidationIssue `json:"issues,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Checklist    *Checklist        `json:"checklist,omitempty"`
	FixesApplied []FixApplied      `json:"fixes_applied,omitempty"`
	Suggestions  []string          `json:"suggestions,omitempty"`
	MarkerInfo   *MarkerInfo       `json:"marker_info,omitempty"`
}

func finalizeValidationResult(result *ValidationResult) {
	result.Score = calculateScore(result)
	result.Valid = validationResultIsClean(result)
}

func validationResultIsClean(result *ValidationResult) bool {
	for _, issue := range result.Issues {
		if issue.Level == LevelError || issue.Level == LevelWarning {
			return false
		}
	}

	if len(result.Warnings) > 0 {
		return false
	}

	return !checklistHasFailures(result.Checklist)
}

func validationHasBlockingFailures(result *ValidationResult) bool {
	for _, issue := range result.Issues {
		if issue.Level == LevelError {
			return true
		}
	}

	return checklistHasFailures(result.Checklist)
}

func checklistHasFailures(checklist *Checklist) bool {
	if checklist == nil {
		return false
	}

	// NOTE: update this slice when adding new Checklist fields.
	checks := []*bool{
		checklist.TemplatesFilled,
		checklist.GoalsAchievable,
		checklist.TaskFilesExist,
		checklist.OrderCorrect,
		checklist.ParallelCorrect,
	}
	for _, check := range checks {
		if check != nil && !*check {
			return true
		}
	}

	return false
}

// convertIssues converts validator.Issue slice to the command-layer ValidationIssue slice.
// This bridges between the validator package's canonical types and the CLI output types.
func convertIssues(issues []validator.Issue) []ValidationIssue {
	out := make([]ValidationIssue, len(issues))
	for i, issue := range issues {
		out[i] = ValidationIssue{
			Level:       issue.Level,
			Code:        issue.Code,
			Path:        issue.Path,
			Message:     issue.Message,
			Fix:         issue.Fix,
			AutoFixable: issue.AutoFixable,
		}
	}
	return out
}

type validateOptions struct {
	path       string
	jsonOutput bool
	fix        bool
}

// NewValidateCommand creates the validate command group
func NewValidateCommand() *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "validate [festival-path]",
		Short: "Check festival structure - find missing task files and issues",
		Long: `Validate that a festival follows the methodology correctly.

Unlike 'fest index validate' which checks index-to-filesystem sync,
this command validates METHODOLOGY COMPLIANCE:

  • Required files exist (FESTIVAL_OVERVIEW.md, goal files)
  • Implementation sequences have TASK FILES (not just goals)
  • Quality gates are present in implementation sequences
  • Naming conventions are followed
  • Templates have been filled out (no [FILL:] markers)

AI agents execute TASK FILES, not goals. If your sequences only have
SEQUENCE_GOAL.md without task files, agents won't know HOW to execute.

Use --fix to automatically apply safe fixes (like adding quality gates).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runValidateAll(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&opts.fix, "fix", false, "Automatically apply safe fixes")

	// Add subcommands
	cmd.AddCommand(newValidateStructureCmd(opts))
	cmd.AddCommand(newValidateCompletenessCmd(opts))
	cmd.AddCommand(newValidateTasksCmd(opts))
	cmd.AddCommand(newValidateQualityGatesCmd(opts))
	cmd.AddCommand(newValidateChecklistCmd(opts))
	cmd.AddCommand(newValidateOrderingCmd(opts))

	return cmd
}

// resolveFestivalPath resolves the festival root directory
// Supports link resolution - can be run from a linked project directory.
func resolveFestivalPath(pathArg string) (string, error) {
	if pathArg != "" {
		absPath, err := filepath.Abs(pathArg)
		if err != nil {
			return "", errors.Wrap(err, "resolving path").WithField("path", pathArg)
		}

		// Check if path exists
		if !validateDirExists(absPath) {
			return "", errors.NotFound("path").WithField("path", absPath)
		}

		// Check if it's a festival directory
		if !isFestivalDir(absPath) {
			return "", errors.Validation("path is not a festival directory (expected FESTIVAL_OVERVIEW.md or phase directories like 001_PHASE_NAME)").WithField("path", absPath)
		}

		return absPath, nil
	}

	// Try current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.IO("getting working directory", err)
	}

	// Use shared link resolver - handles linked projects, festival directories, etc.
	resolvedPath, err := shared.ResolveFestivalPath(cwd, "")
	if err == nil && resolvedPath != "" {
		if isFestivalDir(resolvedPath) {
			return resolvedPath, nil
		}
	}

	// Check if we're directly in a festival directory
	if isFestivalDir(cwd) {
		return cwd, nil
	}

	// Try to find festivals/ root and look for active festivals
	root, err := tpl.FindFestivalsRoot(cwd)
	if err == nil {
		// Check if we're inside an active festival
		rel, _ := filepath.Rel(root, cwd)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) >= 2 && parts[0] == "active" {
			festivalPath := filepath.Join(root, parts[0], parts[1])
			if isFestivalDir(festivalPath) {
				return festivalPath, nil
			}
		}
	}

	// Not in a festival - provide helpful error
	return "", errors.Validation("not inside a festival directory or linked project - run from inside a festival, use 'fest link' to link a project, or provide a path: fest validate /path/to/festival")
}

// isFestivalDir checks if a directory appears to be a festival root
func isFestivalDir(path string) bool {
	// Check for FESTIVAL_OVERVIEW.md or phases
	if validateFileExists(filepath.Join(path, "FESTIVAL_OVERVIEW.md")) {
		return true
	}

	// Check for phase directories
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	phasePattern := regexp.MustCompile(`^\d{3}_`)
	for _, entry := range entries {
		if entry.IsDir() && phasePattern.MatchString(entry.Name()) {
			return true
		}
	}

	return false
}

func validateFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func validateDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// runValidateAll runs all validation checks
func runValidateAll(ctx context.Context, opts *validateOptions) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())
	if ctx == nil {
		ctx = context.Background()
	}

	festivalPath, err := resolveFestivalPath(opts.path)
	if err != nil {
		// A lifecycle resident is camp's directory, not a broken festival. Report
		// it and exit clean rather than telling the user their workitem is
		// malformed. Only the marker-present branch is informational: a plain
		// non-festival directory still errors exactly as before.
		if target, m := residentTarget(opts.path); m != nil {
			return emitResidentInfo(opts, display, target, m)
		}
		return emitValidateError(opts, err)
	}

	result := &ValidationResult{
		OK:       true,
		Action:   "validate",
		Festival: filepath.Base(festivalPath),
		Valid:    true,
		Issues:   []ValidationIssue{},
	}

	// Run all validation checks
	validateStructureChecks(ctx, festivalPath, result)
	validateCompletenessChecks(ctx, festivalPath, result)
	validateTaskFilesChecks(ctx, festivalPath, result)
	validateQualityGatesChecks(ctx, festivalPath, result, opts.fix)
	validateTemplateChecks(festivalPath, result)
	validateOrderingChecks(ctx, festivalPath, result)
	validateAutoLinkChecks(ctx, festivalPath, result)
	validateHooksChecks(ctx, festivalPath, result)
	validateWorkflowDocsChecks(ctx, festivalPath, result)

	// Add suggestions based on issues
	addSuggestions(result)
	finalizeValidationResult(result)

	if opts.jsonOutput {
		if err := emitValidateJSON(result); err != nil {
			return err
		}
		if validationHasBlockingFailures(result) {
			os.Exit(1)
		}
		return nil
	}

	// Human-readable output
	printValidationResult(display, festivalPath, result)

	if validationHasBlockingFailures(result) {
		os.Exit(1)
	}
	return nil
}

// validateTemplateChecks delegates to the canonical validator and builds MarkerInfo
// from the returned issues. This follows the same pattern as other check functions.
func validateTemplateChecks(festivalPath string, result *ValidationResult) {
	issues, err := validator.ValidateTemplateMarkers(festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeUnfilledTemplate,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate template markers: %v", err),
		})
		return
	}

	converted := convertIssues(issues)
	result.Issues = append(result.Issues, converted...)

	// Build MarkerInfo from the converted issues
	if len(converted) > 0 {
		markerInfo := &MarkerInfo{
			FilesWithMarkers: make([]string, 0, len(converted)),
		}
		for _, issue := range converted {
			if issue.Code == CodeUnfilledTemplate {
				markerInfo.TotalFiles++
				markerInfo.FilesWithMarkers = append(markerInfo.FilesWithMarkers, issue.Path)
				// Count from message: "File contains N unfilled template markers (...)"
				var count int
				if _, scanErr := fmt.Sscanf(issue.Message, "File contains %d unfilled template markers", &count); scanErr == nil {
					markerInfo.TotalCount += count
				} else {
					markerInfo.TotalCount++ // fallback: at least 1
				}
			}
		}
		result.MarkerInfo = markerInfo
	}
}

// validateAutoLinkChecks runs auto-link validation for the CLI `fest validate` command.
// This is separate from validator.FullValidate (api.go) which runs the same check for
// programmatic/API consumers. Both call validator.ValidateAutoLink but produce different
// result types (ValidationIssue here vs validator.Issue in the API path).
func validateAutoLinkChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return // No fest.yaml — skip auto-link validation gracefully
	}
	issues, err := validator.ValidateAutoLink(ctx, festivalPath, festCfg)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    "autolink_error",
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to validate auto-link: %v", err),
		})
		return
	}
	result.Issues = append(result.Issues, convertIssues(issues)...)
}

// Score calculation

func calculateScore(result *ValidationResult) int {
	if len(result.Issues) == 0 {
		return 100
	}

	errorCount := 0
	warningCount := 0

	for _, issue := range result.Issues {
		switch issue.Level {
		case LevelError:
			errorCount++
		case LevelWarning:
			warningCount++
		}
	}

	// Base score of 100, minus 15 per error, minus 5 per warning
	score := 100 - (errorCount * 15) - (warningCount * 5)
	if score < 0 {
		score = 0
	}

	return score
}

func addSuggestions(result *ValidationResult) {
	hasMissingTasks := false
	hasMissingGates := false
	hasUnfilledTemplates := false
	hasNumberingGaps := false
	hasWorkflowIssues := false

	for _, issue := range result.Issues {
		switch issue.Code {
		case CodeMissingTaskFiles:
			hasMissingTasks = true
		case CodeMissingQualityGate:
			hasMissingGates = true
		case CodeUnfilledTemplate:
			hasUnfilledTemplates = true
		case CodeNumberingGap:
			hasNumberingGaps = true
		case CodeWorkflowNumbering:
			hasWorkflowIssues = true
		}
	}

	if hasMissingTasks {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest understand tasks' to learn about task file creation")
	}
	if hasMissingGates {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest gates apply --approve' to add quality gates")
	}
	if hasUnfilledTemplates {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest markers list' to see all unfilled template markers",
			"Use 'fest markers fill' or edit files manually to replace markers with actual content")
	}
	if hasNumberingGaps {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest renumber' to automatically fix numbering gaps")
	}
	if hasWorkflowIssues {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest workflow validate <path>' against affected WORKFLOW.md files for detailed remediation")
	}
}

// Output functions

func emitValidateJSON(result *ValidationResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func emitValidateError(opts *validateOptions, err error) error {
	if opts.jsonOutput {
		result := &ValidationResult{
			OK:     false,
			Action: "validate",
			Valid:  false,
			Issues: []ValidationIssue{{
				Level:   LevelError,
				Code:    "error",
				Message: err.Error(),
			}},
		}
		return emitValidateJSON(result)
	}
	return err
}

// validateHooksChecks emits non-blocking warnings for shadow drift and undeclared bindings.
func validateHooksChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	eff, err := hookslib.LoadAndResolve(ctx, festivalPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level: LevelWarning, Code: CodeHookResolveError, Path: festivalPath,
			Message: fmt.Sprintf("could not resolve hooks: %v", err),
		})
		return
	}
	if eff == nil {
		return
	}
	for name, h := range eff.Hooks {
		for _, s := range h.Shadowed {
			result.Issues = append(result.Issues, ValidationIssue{
				Level: LevelWarning, Code: CodeHookShadowDrift, Path: festivalPath,
				Message: fmt.Sprintf("hook %q at layer %s overrides a differing definition from layer %s", name, h.Source, s.Source),
			})
		}
	}

	// Undeclared bindings skip with a warning (spec 03, D10).
	result.Issues = append(result.Issues,
		convertIssues(validator.ScanUndeclaredBindings(ctx, festivalPath, eff))...)
}

// residentTarget returns the path validate was aimed at and its workitem marker,
// or a nil marker when the target is not a resident.
func residentTarget(pathArg string) (string, *resident.Marker) {
	target := pathArg
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", nil
		}
		target = cwd
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return target, nil
	}
	m, readErr := resident.Read(abs)
	if readErr != nil {
		return abs, nil
	}
	return abs, m
}

// emitResidentInfo reports a resident through the normal validate result shape so
// --json consumers see the same envelope, with a single info-level issue.
func emitResidentInfo(opts *validateOptions, display *ui.UI, target string, m *resident.Marker) error {
	label := m.Type
	if label == "" {
		label = "workitem"
	}
	message := "lifecycle resident (" + label + ") owned by camp; not a festival, nothing to validate"

	result := &ValidationResult{
		OK:       true,
		Action:   "validate",
		Festival: filepath.Base(target),
		Valid:    true,
		Issues: []ValidationIssue{{
			Level:   LevelInfo,
			Code:    "lifecycle-resident",
			Path:    target,
			Message: message,
		}},
	}
	if opts.jsonOutput {
		return emitValidateJSON(result)
	}
	display.Info("%s", message)
	return nil
}
