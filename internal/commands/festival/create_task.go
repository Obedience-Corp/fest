package festival

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// CreateTaskOptions holds options for the create task command.
type CreateTaskOptions struct {
	After       int
	Names       []string
	Path        string
	VarsFile    string
	Markers     string // Inline JSON with hint→value mappings
	MarkersFile string // JSON file path with hint→value mappings
	SkipMarkers bool   // Skip marker processing
	DryRun      bool   // Show markers without creating file
	JSONOutput  bool
	AgentMode   bool // Strict mode for AI agents
}

type createTaskResult struct {
	OK              bool               `json:"ok"`
	Action          string             `json:"action"`
	Task            map[string]any     `json:"task,omitempty"`
	Created         []string           `json:"created,omitempty"`
	Renumber        []string           `json:"renumbered,omitempty"`
	Markers         []map[string]any   `json:"markers,omitempty"`
	MarkersFilled   int                `json:"markers_filled,omitempty"`
	MarkersTotal    int                `json:"markers_total,omitempty"`
	MarkersUnfilled int                `json:"markers_unfilled,omitempty"`
	MarkersWarning  string             `json:"markers_warning,omitempty"`
	Validation      *ValidationSummary `json:"validation,omitempty"`
	Errors          []map[string]any   `json:"errors,omitempty"`
	Warnings        []string           `json:"warnings,omitempty"`
	Suggestions     []string           `json:"suggestions,omitempty"`
}

// NewCreateTaskCommand adds 'create task'
func NewCreateTaskCommand() *cobra.Command {
	opts := &CreateTaskOptions{}
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Insert a new task file in a sequence",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Create new task file(s) with automatic numbering and template rendering.

IMPORTANT: AI agents execute TASK FILES, not goals. If your sequences only
have SEQUENCE_GOAL.md without task files, agents won't know HOW to execute.

BATCH CREATION: Use multiple --name flags to create sequential tasks at once.
This avoids the common mistake of all tasks getting numbered 01_.

TEMPLATE VARIABLES (automatically set from --name):
  {{ task_name }}            Name of the task
  {{ task_number }}          Sequential number (01, 02, ...)
  {{ task_id }}              Full filename (e.g., "01_design.md")
  {{ parent_sequence_id }}   Parent sequence ID
  {{ parent_phase_id }}      Parent phase ID
  {{ full_path }}            Complete path from festival root

EXAMPLES:
  # Create single task in current sequence (appends at the end)
  fest create task --name "design endpoints" --json

  # Create multiple tasks at once (sequential numbering, appended at end)
  fest create task --name "requirements" --name "design" --name "implement"
  # Creates: 01_requirements.md, 02_design.md, 03_implement.md

  # Insert tasks after position 2 (existing tasks renumber down, reported)
  fest create task --after 2 --name "new step" --name "another step"
  # Creates: 03_new_step.md, 04_another_step.md

  # Insert at the beginning
  fest create task --after 0 --name "prerequisite"

  # Create task in specific sequence
  fest create task --name "setup" --path ./002_IMPLEMENT/01_api --json

MARKER FILLING (for AI agents):
  # Fill all REPLACE markers in one command
  fest create task --name "setup" --markers '{"Brief description": "Auth middleware", "Yes/No": "Yes"}'

  # Preview template markers first (dry-run)
  fest create task --name "setup" --dry-run --json

  # Skip marker filling (leave REPLACE tags)
  fest create task --name "setup" --skip-markers

Run 'fest understand tasks' for detailed guidance on task file creation.
Run 'fest validate tasks' to verify task files exist in implementation sequences.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return shared.StartCreateTaskTUI(cmd.Context())
			}
			if len(opts.Names) == 0 {
				return errors.Validation("--name is required (or run without flags to open TUI)")
			}
			// Validate all names are non-empty
			for _, name := range opts.Names {
				if strings.TrimSpace(name) == "" {
					return errors.Validation("task names cannot be empty")
				}
			}
			return RunCreateTask(cmd.Context(), opts)
		},
	}
	cmd.Flags().IntVar(&opts.After, "after", -1, "Insert after this task number (-1 or omit to append at end; 0 inserts at beginning)")
	cmd.Flags().StringSliceVar(&opts.Names, "name", nil, "Task name(s) - can be specified multiple times for batch creation")
	cmd.Flags().StringVar(&opts.Path, "path", ".", "Path to sequence directory (directory containing numbered task files)")
	cmd.Flags().StringVar(&opts.VarsFile, "vars-file", "", "JSON vars for rendering")
	cmd.Flags().StringVar(&opts.Markers, "markers", "", "JSON string with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.MarkersFile, "markers-file", "", "JSON file with REPLACE marker hint→value mappings")
	cmd.Flags().BoolVar(&opts.SkipMarkers, "skip-markers", false, "Skip REPLACE marker processing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show template markers without creating file")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict mode: require markers, auto-validate, block on errors, JSON output")
	return cmd
}

// resolveTaskInsertAfter maps the --after default (-1) to append-at-end by
// detecting the highest existing task number in the sequence directory,
// matching create sequence's behavior.
func resolveTaskInsertAfter(ctx context.Context, sequenceDir string, after int) int {
	if after != -1 {
		return after
	}
	parser := festival.NewParser()
	tasks, err := parser.ParseTasks(ctx, sequenceDir)
	if err != nil || len(tasks) == 0 {
		return 0
	}
	maxNum := 0
	for _, t := range tasks {
		if t.Number > maxNum {
			maxNum = t.Number
		}
	}
	return maxNum
}

// RunCreateTask executes the create task command logic.
func RunCreateTask(ctx context.Context, opts *CreateTaskOptions) error {
	// Check context early
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreateTask")
	}

	// Agent mode implies JSON output
	if opts.AgentMode {
		opts.JSONOutput = true
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// Convert to absolute path first so resolution functions can walk the tree
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return emitCreateTaskError(opts, errors.Wrap(err, "resolving path").WithField("path", opts.Path))
	}

	// Resolve paths for config loading
	festivalsRoot := ResolveFestivalsRoot(absPath)
	festivalPath := ResolveFestivalPath(absPath)

	// Load effective agent config (workspace + festival merged)
	agentCfg := LoadEffectiveAgentConfig(festivalsRoot, festivalPath)

	// Determine effective skip-markers behavior
	effectiveSkipMarkers := config.EffectiveSkipMarkers(agentCfg, opts.AgentMode, opts.SkipMarkers)

	// Resolve template root
	tmplRoot, err := tpl.LocalTemplateRoot(absPath)
	if err != nil {
		return emitCreateTaskError(opts, err)
	}

	// Load vars once for all tasks
	vars := map[string]any{}
	if strings.TrimSpace(opts.VarsFile) != "" {
		v, err := loadVarsFile(opts.VarsFile)
		if err != nil {
			return emitCreateTaskError(opts, errors.Wrap(err, "reading vars-file").WithField("path", opts.VarsFile))
		}
		vars = v
	}

	// Load template catalog once
	catalog, _ := tpl.LoadCatalog(ctx, tmplRoot)
	mgr := tpl.NewManager()
	loader := tpl.NewLoader()

	opts.After = resolveTaskInsertAfter(ctx, absPath, opts.After)

	// Track all created tasks for output
	var createdTasks []map[string]any
	var createdPaths []string
	var renumbered []string
	var totalMarkersFilled, totalMarkersCount int
	currentAfter := opts.After

	// Create each task sequentially
	for _, name := range opts.Names {
		// Check context on each iteration
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Wrap(ctxErr, "context cancelled").WithOp("RunCreateTask")
		}

		// Insert task at current position
		ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
		if err := ren.InsertTask(ctx, absPath, currentAfter, name); err != nil {
			return emitCreateTaskError(opts, errors.Wrap(err, "inserting task").WithField("name", name))
		}
		for _, ch := range ren.Changes() {
			if ch.Type == festival.ChangeRename {
				renumbered = append(renumbered,
					fmt.Sprintf("%s -> %s", filepath.Base(ch.OldPath), filepath.Base(ch.NewPath)))
			}
		}

		// Compute new task id
		newNumber := currentAfter + 1
		taskID := tpl.FormatTaskID(newNumber, name)
		taskPath := filepath.Join(absPath, taskID)

		// Build full template context with hierarchy (festival → phase → sequence → task)
		tmplCtx, ctxErr := tpl.BuildTaskContext(absPath, festivalPath, newNumber, name)
		if ctxErr != nil {
			// Fall back to minimal context
			tmplCtx = tpl.NewContext()
			tmplCtx.SetTask(newNumber, name)
			tmplCtx.ComputeStructureVariables()
		}
		for k, v := range vars {
			tmplCtx.SetCustom(k, v)
		}

		// Render or copy TASK template
		var content string
		var renderErr error
		if catalog != nil {
			content, renderErr = mgr.RenderByID(ctx, catalog, "TASK", tmplCtx)
		}
		if renderErr != nil || content == "" {
			// Fall back to default filename
			tpath := filepath.Join(tmplRoot, "tasks", "TASK.md")
			if _, err := os.Stat(tpath); err == nil {
				t, err := loader.Load(ctx, tpath)
				if err != nil {
					return emitCreateTaskError(opts, errors.Wrap(err, "loading task template"))
				}
				// Render if it appears templated; else copy
				if strings.Contains(t.Content, "{{") {
					out, err := mgr.Render(t, tmplCtx)
					if err != nil {
						return emitCreateTaskError(opts, errors.Wrap(err, "rendering task"))
					}
					content = out
				} else {
					content = t.Content
				}
			}
		}

		// If no template content was found, create a minimal placeholder
		// Note: Task metadata (number, status, autonomy) is in frontmatter, not in markdown
		if content == "" {
			content = fmt.Sprintf("# Task: %s\n\n## Objective\n\n[REPLACE: Describe the task objective]\n\n## Steps\n\n1. [REPLACE: Step 1]\n2. [REPLACE: Step 2]\n\n## Definition of Done\n\n- [ ] [REPLACE: Completion criterion]\n", name)
		}

		// Write task file (the file was created by InsertTask, but we need to write content)
		if content != "" {
			// Inject frontmatter if content doesn't already have it
			if !strings.HasPrefix(strings.TrimSpace(content), "---") {
				parentSequenceID := filepath.Base(absPath)
				fm := frontmatter.NewTaskFrontmatter(taskID, name, parentSequenceID, newNumber, frontmatter.AutonomyMedium)
				contentWithFM, fmErr := frontmatter.InjectString(content, fm)
				if fmErr != nil {
					return emitCreateTaskError(opts, errors.Wrap(fmErr, "injecting frontmatter"))
				}
				content = contentWithFM
			}

			// Auto-fill [REPLACE: ...] markers from context (before writing)
			// This fills Category A (structure) markers automatically
			if !effectiveSkipMarkers {
				renderer := tpl.NewRenderer()
				// Load config markers for Category B markers (lint_command, test_command, etc.)
				var configMarkers map[string]string
				if festivalPath != "" {
					festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, "")
					if cfgErr == nil && festCfg != nil {
						configMarkers = extractConfigMarkers(festCfg)
					}
				}
				renderedContent, renderErr := renderer.RenderWithMarkerReplacement(content, tmplCtx, configMarkers)
				if renderErr == nil {
					content = renderedContent
				}
			}

			if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
				return emitCreateTaskError(opts, errors.IO("writing task", err).WithField("path", taskPath))
			}

			// Process REPLACE markers in the created file
			markerResult, err := ProcessMarkers(ctx, MarkerOptions{
				FilePath:    taskPath,
				Markers:     opts.Markers,
				MarkersFile: opts.MarkersFile,
				SkipMarkers: effectiveSkipMarkers,
				DryRun:      opts.DryRun,
				JSONOutput:  opts.JSONOutput,
			})
			if err != nil {
				return emitCreateTaskError(opts, errors.Wrap(err, "processing markers"))
			}

			// For dry-run, output markers and exit
			if opts.DryRun && markerResult != nil {
				if err := PrintDryRunMarkers(markerResult, opts.JSONOutput); err != nil {
					return emitCreateTaskError(opts, err)
				}
				return nil
			}

			// Track marker results for reporting
			if markerResult != nil && markerResult.Total > 0 {
				totalMarkersFilled += markerResult.Filled
				totalMarkersCount += markerResult.Total
			}
		}

		// Track created task
		createdTasks = append(createdTasks, map[string]any{
			"number": newNumber,
			"id":     taskID,
			"name":   name,
		})
		createdPaths = append(createdPaths, taskPath)

		// Increment position for next task
		currentAfter = newNumber
	}

	// Run post-create validation if configured
	var validationResult *ValidationSummary
	shouldValidate := config.ShouldValidate(agentCfg, opts.AgentMode)
	if shouldValidate && festivalPath != "" {
		validationResult, err = RunPostCreateValidation(ctx, festivalPath)
		if err != nil {
			// Don't fail on validation errors, just report
			if !opts.JSONOutput {
				display.Warning("Validation failed: %v", err)
			}
		}

		// Block on errors if configured
		if validationResult != nil && !validationResult.OK {
			if config.ShouldBlockOnErrors(agentCfg, opts.AgentMode) {
				return emitCreateTaskError(opts, errors.Validation("validation errors detected - fix issues before proceeding"))
			}
		}
	}

	// Output results
	remainingMarkers := totalMarkersCount - totalMarkersFilled

	if opts.JSONOutput {
		warnings := []string{}
		if remainingMarkers > 0 {
			warnings = append(warnings,
				fmt.Sprintf("CRITICAL: %d unfilled markers - task cannot be executed until resolved", remainingMarkers),
				"Edit task file directly to replace [REPLACE: ...] markers",
			)
		}
		warnings = append(warnings, "Edit task file to define implementation steps")

		// Add discovery commands for agents
		suggestions := []string{
			"fest status        - View festival progress",
			"fest next          - Find what to work on next",
			"fest show plan     - View the execution plan",
			"fest validate      - Check completion status",
			"fest progress      - See detailed progress",
		}

		result := createTaskResult{
			OK:              true,
			Action:          "create_task",
			Created:         createdPaths,
			Renumber:        renumbered,
			MarkersFilled:   totalMarkersFilled,
			MarkersTotal:    totalMarkersCount,
			MarkersUnfilled: remainingMarkers,
			Validation:      validationResult,
			Warnings:        warnings,
			Suggestions:     suggestions,
		}
		if remainingMarkers > 0 {
			result.MarkersWarning = fmt.Sprintf("%d unfilled markers across %d tasks. Fill with: fest wizard fill", remainingMarkers, len(createdTasks))
		}
		// For single task, use Task field for backward compatibility
		if len(createdTasks) == 1 {
			result.Task = createdTasks[0]
		}
		return emitCreateTaskJSON(opts, result)
	}

	// Show marker warning FIRST (before success message) for visibility
	if remainingMarkers > 0 {
		fmt.Println()
		if len(createdTasks) > 1 {
			display.Warning("%d unfilled markers across %d tasks", remainingMarkers, len(createdTasks))
		} else {
			display.Warning("%d unfilled markers in task", remainingMarkers)
		}
		display.Info("   Fill markers: edit task files directly or use %s", ui.Value("fest wizard fill"))
		fmt.Println()
	}

	// Human-readable output
	if len(createdTasks) == 1 {
		display.Success("Created task: %s", createdTasks[0]["id"])
		display.Info("  └── %s", createdPaths[0])
	} else {
		display.Success("Created %d tasks:", len(createdTasks))
		for _, task := range createdTasks {
			display.Info("  └── %s", task["id"])
		}
	}
	if len(renumbered) > 0 {
		display.Warning("Renumbered %d existing task(s):", len(renumbered))
		for _, r := range renumbered {
			display.Info("  └── %s", r)
		}
	}

	// Report validation results
	if validationResult != nil {
		if validationResult.OK {
			display.Success("Validation passed (score: %d)", validationResult.Score)
		} else {
			display.Warning("Validation issues found (score: %d, errors: %d, warnings: %d)",
				validationResult.Score, validationResult.Errors, validationResult.Warnings)
			for _, issue := range validationResult.Issues {
				display.Info("  • [%s] %s: %s", issue.Level, issue.Path, issue.Message)
			}
		}
	}

	fmt.Println()
	fmt.Println(ui.H2("Next Steps"))
	if remainingMarkers > 0 {
		fmt.Printf("  %s\n", ui.Info("1. Edit task file to define implementation steps"))
		fmt.Printf("  %s\n", ui.Info("2. fest create task --name \"next_step\" (add more tasks)"))
	} else {
		fmt.Printf("  %s\n", ui.Info("• Add more tasks: fest create task --name \"next_step\""))
	}
	fmt.Printf("  %s\n", ui.Info("• Add quality gates: fest gates apply --approve"))
	fmt.Printf("  %s\n", ui.Info("• Validate progress: fest validate"))
	fmt.Println()
	fmt.Println(ui.H2("Discover More Commands"))
	fmt.Printf("  %s %s\n", ui.Value("fest status"), ui.Dim("View festival progress"))
	fmt.Printf("  %s %s\n", ui.Value("fest next"), ui.Dim("Find what to work on next"))
	fmt.Printf("  %s %s\n", ui.Value("fest show plan"), ui.Dim("View the execution plan"))
	return nil
}

func emitCreateTaskError(opts *CreateTaskOptions, err error) error {
	if opts.JSONOutput {
		_ = emitCreateTaskJSON(opts, createTaskResult{
			OK:     false,
			Action: "create_task",
			Errors: []map[string]any{{
				"code":    "error",
				"message": err.Error(),
			}},
		})
		return nil
	}
	return err
}

func emitCreateTaskJSON(opts *CreateTaskOptions, res createTaskResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// extractConfigMarkers extracts marker values from festival config.
// Maps config fields to [REPLACE: ...] marker hints for Category B (config-based) markers.
func extractConfigMarkers(cfg *config.FestivalConfig) map[string]string {
	if cfg == nil {
		return nil
	}

	markers := make(map[string]string)

	// Map project path if set
	if cfg.ProjectPath != "" {
		markers["project_path"] = cfg.ProjectPath
	}

	// Map metadata fields
	if cfg.Metadata.Name != "" {
		markers["festival_name"] = cfg.Metadata.Name
	}
	if cfg.Metadata.ID != "" {
		markers["festival_id"] = cfg.Metadata.ID
	}

	// Future: Add more config-based markers here as they're added to fest.yaml
	// Examples:
	//   markers["lint_command"] = cfg.Commands.Lint
	//   markers["test_command"] = cfg.Commands.Test
	//   markers["build_command"] = cfg.Commands.Build

	return markers
}
