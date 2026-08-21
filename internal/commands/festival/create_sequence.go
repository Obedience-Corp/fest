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

// CreateSequenceOptions holds options for the create sequence command.
type CreateSequenceOptions struct {
	After       int
	Name        string
	Path        string
	Festival    string // --festival: path to festival directory (use when not inside a festival)
	VarsFile    string
	Markers     string // Inline JSON with hint→value mappings
	MarkersFile string // JSON file path with hint→value mappings
	SkipMarkers bool   // Skip marker processing
	DryRun      bool   // Show markers without creating file
	JSONOutput  bool
	NoPrompt    bool
	AgentMode   bool // Strict mode for AI agents
}

type createSequenceResult struct {
	OK            bool               `json:"ok"`
	Action        string             `json:"action"`
	Sequence      map[string]any     `json:"sequence,omitempty"`
	Created       []string           `json:"created,omitempty"`
	Renumber      []string           `json:"renumbered,omitempty"`
	Markers       []map[string]any   `json:"markers,omitempty"`
	MarkersFilled int                `json:"markers_filled,omitempty"`
	MarkersTotal  int                `json:"markers_total,omitempty"`
	Validation    *ValidationSummary `json:"validation,omitempty"`
	Errors        []map[string]any   `json:"errors,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Suggestions   []string           `json:"suggestions,omitempty"`
}

// NewCreateSequenceCommand adds 'create sequence'
func NewCreateSequenceCommand() *cobra.Command {
	opts := &CreateSequenceOptions{}
	cmd := &cobra.Command{
		Use:   "sequence",
		Short: "Insert a new sequence and render its goal file",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Create a new sequence directory with SEQUENCE_GOAL.md.

IMPORTANT: After creating a sequence, you must also create TASK FILES.
The SEQUENCE_GOAL.md defines WHAT to achieve, but AI agents need task
files to know HOW to execute. See 'fest understand tasks'.

TEMPLATE VARIABLES (automatically set):
  {{ sequence_name }}        Name of the sequence
  {{ sequence_number }}      Sequential number (01, 02, ...)
  {{ sequence_id }}          Full ID (e.g., "01_api_endpoints")
  {{ parent_phase_id }}      Parent phase ID

EXAMPLES:
  # Create sequence in current phase
  fest create sequence --name "api endpoints" --json

  # Create sequence at specific position
  fest create sequence --name "frontend" --after 2 --json

NEXT STEPS after creating a sequence:
  # Add task files (required for implementation sequences)
  fest create task --name "design" --json
  fest create task --name "implement" --json

  # Add quality gates
  fest gates apply --approve

Run 'fest validate tasks' to verify task files exist.

The optional "Create task files now?" prompt is skipped when stdin is not a
TTY (agent shells). Pass --json or --no-prompt to skip it explicitly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return shared.StartCreateSequenceTUI(cmd.Context())
			}
			if strings.TrimSpace(opts.Name) == "" {
				return errors.Validation("--name is required (or run without flags to open TUI)")
			}
			return RunCreateSequence(cmd.Context(), opts)
		},
	}
	cmd.Flags().IntVar(&opts.After, "after", -1, "Insert after this sequence number (-1 or omit to append at end)")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Sequence name (required)")
	cmd.Flags().StringVar(&opts.Path, "path", ".", "Path to phase directory (directory containing numbered sequences)")
	cmd.Flags().StringVar(&opts.Festival, "festival", "", "Path to festival directory (use when not inside a festival)")
	cmd.Flags().StringVar(&opts.VarsFile, "vars-file", "", "JSON vars for rendering")
	cmd.Flags().StringVar(&opts.Markers, "markers", "", "JSON string with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.MarkersFile, "markers-file", "", "JSON file with REPLACE marker hint→value mappings")
	cmd.Flags().BoolVar(&opts.SkipMarkers, "skip-markers", false, "Skip REPLACE marker processing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show template markers without creating file")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().BoolVar(&opts.NoPrompt, "no-prompt", false, "Skip interactive prompts (also skipped when stdin is not a TTY)")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict mode: require markers, auto-validate, block on errors, JSON output")
	return cmd
}

// RunCreateSequence executes the create sequence command logic.
func RunCreateSequence(ctx context.Context, opts *CreateSequenceOptions) error {
	// Check context early
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreateSequence")
	}

	// Agent mode implies JSON output and no prompts
	if opts.AgentMode {
		opts.JSONOutput = true
		opts.NoPrompt = true
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// --festival overrides --path when provided
	path := opts.Path
	if opts.Festival != "" {
		if opts.Path != "." {
			display.Warning("Both --path and --festival provided; using --festival")
		}
		path = opts.Festival
	}

	// Convert to absolute path first so resolution functions can walk the tree
	absPath, err := filepath.Abs(path)
	if err != nil {
		return emitCreateSequenceError(opts, errors.Wrap(err, "resolving path").WithField("path", path))
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
		return emitCreateSequenceError(opts, err)
	}

	opts.After = resolveSequenceInsertAfter(ctx, absPath, opts.After)
	newNumber := opts.After + 1
	seqID := tpl.FormatSequenceID(newNumber, opts.Name)
	seqDir := filepath.Join(absPath, seqID)
	goalPath := filepath.Join(seqDir, "SEQUENCE_GOAL.md")

	// Load vars
	vars := map[string]any{}
	if strings.TrimSpace(opts.VarsFile) != "" {
		v, err := loadVarsFile(opts.VarsFile)
		if err != nil {
			return emitCreateSequenceError(opts, errors.Wrap(err, "reading vars-file").WithField("path", opts.VarsFile))
		}
		vars = v
	}

	// Build full template context with hierarchy (festival → phase → sequence)
	tmplCtx, ctxErr := tpl.BuildContextFromPath(absPath, festivalPath)
	if ctxErr != nil {
		// Fall back to minimal context
		tmplCtx = tpl.NewContext()
	}
	tmplCtx.SetSequence(newNumber, opts.Name)
	tmplCtx.ComputeStructureVariables()
	for k, v := range vars {
		tmplCtx.SetCustom(k, v)
	}

	content, err := renderSequenceGoalContent(ctx, tmplRoot, tmplCtx, opts.Name)
	if err != nil {
		return emitCreateSequenceError(opts, err)
	}
	content, err = finalizeSequenceGoalContent(content, seqID, opts.Name, filepath.Base(absPath), newNumber, tmplCtx, festivalPath, effectiveSkipMarkers)
	if err != nil {
		return emitCreateSequenceError(opts, err)
	}

	if opts.DryRun {
		planned := filepath.ToSlash(filepath.Join(seqID, "SEQUENCE_GOAL.md"))
		return emitCreateDryRun(opts.JSONOutput, []string{planned}, markerResultFromContent(content))
	}

	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
	if err := ren.InsertSequence(ctx, absPath, opts.After, opts.Name); err != nil {
		return emitCreateSequenceError(opts, errors.Wrap(err, "inserting sequence"))
	}
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		return emitCreateSequenceError(opts, errors.IO("creating sequence dir", err).WithField("path", seqDir))
	}
	if err := os.WriteFile(goalPath, []byte(content), 0644); err != nil {
		return emitCreateSequenceError(opts, errors.IO("writing sequence goal", err).WithField("path", goalPath))
	}

	var markersFilled, markersTotal int
	markerResult, err := ProcessMarkers(ctx, MarkerOptions{
		FilePath:    goalPath,
		Markers:     opts.Markers,
		MarkersFile: opts.MarkersFile,
		SkipMarkers: effectiveSkipMarkers,
		DryRun:      false,
		JSONOutput:  opts.JSONOutput,
	})
	if err != nil {
		return emitCreateSequenceError(opts, errors.Wrap(err, "processing markers"))
	}
	if markerResult != nil {
		markersFilled = markerResult.Filled
		markersTotal = markerResult.Total
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
				return emitCreateSequenceError(opts, errors.Validation("validation errors detected - fix issues before proceeding"))
			}
		}
	}

	if opts.JSONOutput {
		remainingMarkers := markersTotal - markersFilled
		warnings := []string{}
		if remainingMarkers > 0 {
			warnings = append(warnings,
				fmt.Sprintf("CRITICAL: %d unfilled markers - sequence cannot be executed until resolved", remainingMarkers),
				"Run 'fest wizard fill SEQUENCE_GOAL.md' to fill markers interactively",
			)
		}
		warnings = append(warnings,
			"Sequences need task files for AI execution. Goals define WHAT, tasks define HOW.",
			"Create tasks with: fest create task --name \"...\"",
			"Learn more: fest understand tasks",
		)

		// Add discovery commands for agents
		suggestions := []string{
			"fest status        - View festival progress",
			"fest next          - Find what to work on next",
			"fest show plan     - View the execution plan",
			"fest validate      - Check completion status",
		}

		result := createSequenceResult{
			OK:     true,
			Action: "create_sequence",
			Sequence: map[string]any{
				"number": newNumber,
				"id":     seqID,
				"name":   opts.Name,
			},
			Created:       []string{goalPath},
			Renumber:      []string{},
			MarkersFilled: markersFilled,
			MarkersTotal:  markersTotal,
			Validation:    validationResult,
			Warnings:      warnings,
			Suggestions:   suggestions,
		}
		return emitCreateSequenceJSON(opts, result)
	}

	// Show marker warning FIRST (before success message) for visibility
	remainingMarkers := markersTotal - markersFilled
	if remainingMarkers > 0 {
		fmt.Println()
		display.Error("🚫 CRITICAL: %d unfilled markers - sequence cannot be executed until resolved", remainingMarkers)
		display.Info("   Run 'fest wizard fill SEQUENCE_GOAL.md' to fill markers interactively")
		display.Info("   Or edit SEQUENCE_GOAL.md directly to replace [REPLACE: ...] markers")
		fmt.Println()
	}

	display.Success("Created sequence: %s", seqID)
	display.Info("  └── %s", "SEQUENCE_GOAL.md")

	fmt.Println()

	// Show education message
	display.Warning("Sequences need task files to be executable.")
	fmt.Printf("  %s\n", ui.Info("SEQUENCE_GOAL.md defines WHAT to accomplish."))
	fmt.Printf("  %s\n", ui.Info("Task files (01_*.md, 02_*.md) define HOW to do it."))
	fmt.Println()
	fmt.Println(ui.H2("Next Steps"))
	if remainingMarkers > 0 {
		fmt.Printf("  %s\n", ui.Info("1. Edit SEQUENCE_GOAL.md to define sequence objectives"))
		fmt.Printf("  %s\n", ui.Info("2. fest create task --name \"your_task_name\""))
	} else {
		fmt.Printf("  %s\n", ui.Info("1. fest create task --name \"your_task_name\""))
	}
	fmt.Printf("  %s\n", ui.Info("💡 Run 'fest understand tasks' to learn more about task structure."))
	fmt.Println()
	fmt.Println(ui.H2("Discover More Commands"))
	fmt.Printf("  %s %s\n", ui.Value("fest status"), ui.Dim("View festival progress"))
	fmt.Printf("  %s %s\n", ui.Value("fest next"), ui.Dim("Find what to work on next"))
	fmt.Printf("  %s %s\n", ui.Value("fest show plan"), ui.Dim("View the execution plan"))
	fmt.Println()

	// Optional prompt: skip --no-prompt / --json / --skip-markers, and when
	// stdin is not a TTY so agent shells cannot hang on [Y/n] (issue #343).
	if shouldPromptCreateTaskFiles(opts) {
		if display.Confirm("Create task files now?") {
			fmt.Println()
			fmt.Println(ui.H2("Create Tasks"))
			fmt.Printf("  %s\n", ui.Info("To create tasks, run:"))
			fmt.Printf("  %s\n", ui.Value("fest create task --name \"your_task_name\""))
			fmt.Println()
			fmt.Printf("  %s\n", ui.Info("Or start the interactive TUI:"))
			fmt.Printf("  %s\n", ui.Value("fest create task"))
		}
	}

	return nil
}

func emitCreateSequenceError(opts *CreateSequenceOptions, err error) error {
	if opts.JSONOutput {
		_ = emitCreateSequenceJSON(opts, createSequenceResult{
			OK:     false,
			Action: "create_sequence",
			Errors: []map[string]any{{
				"code":    "error",
				"message": err.Error(),
			}},
		})
		return nil
	}
	return err
}

func emitCreateSequenceJSON(opts *CreateSequenceOptions, res createSequenceResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// resolveSequenceInsertAfter maps the --after default (-1) to append-at-end.
// Parse failure falls back to 0 (insert at beginning), matching historical
// create-sequence behavior.
func resolveSequenceInsertAfter(ctx context.Context, phaseDir string, after int) int {
	if after != -1 {
		return after
	}
	parser := festival.NewParser()
	sequences, parseErr := parser.ParseSequences(ctx, phaseDir)
	if parseErr == nil && len(sequences) > 0 {
		maxNum := 0
		for _, s := range sequences {
			if s.Number > maxNum {
				maxNum = s.Number
			}
		}
		return maxNum
	}
	return 0
}

func renderSequenceGoalContent(ctx context.Context, tmplRoot string, tmplCtx *tpl.Context, name string) (string, error) {
	catalog, _ := tpl.LoadCatalog(ctx, tmplRoot)
	mgr := tpl.NewManager()
	var content string
	var renderErr error
	if catalog != nil {
		content, renderErr = mgr.RenderByID(ctx, catalog, "SEQUENCE_GOAL", tmplCtx)
	}
	if renderErr != nil || content == "" {
		tpath := filepath.Join(tmplRoot, "sequences", "GOAL.md")
		if _, err := os.Stat(tpath); err == nil {
			loader := tpl.NewLoader()
			t, err := loader.Load(ctx, tpath)
			if err != nil {
				return "", errors.Wrap(err, "loading sequence goal template")
			}
			if strings.Contains(t.Content, "{{") {
				out, err := mgr.Render(t, tmplCtx)
				if err != nil {
					return "", errors.Wrap(err, "rendering sequence goal")
				}
				content = out
			} else {
				content = t.Content
			}
		}
	}
	if content == "" {
		content = fmt.Sprintf("# Sequence Goal: %s\n\n## Objective\n\n[REPLACE: Describe the sequence objective]\n\n## Tasks\n\n- [ ] [REPLACE: Task 1]\n- [ ] [REPLACE: Task 2]\n", name)
	}
	return content, nil
}

func finalizeSequenceGoalContent(content, seqID, name, parentPhaseID string, newNumber int, tmplCtx *tpl.Context, festivalPath string, skipMarkers bool) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		fm := frontmatter.NewSequenceFrontmatter(seqID, name, parentPhaseID, newNumber)
		contentWithFM, fmErr := frontmatter.InjectString(content, fm)
		if fmErr != nil {
			return "", errors.Wrap(fmErr, "injecting frontmatter")
		}
		content = contentWithFM
	}
	if skipMarkers {
		return content, nil
	}
	renderer := tpl.NewRenderer()
	var configMarkers map[string]string
	if festivalPath != "" {
		festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, "")
		if cfgErr == nil && festCfg != nil {
			configMarkers = extractConfigMarkers(festCfg)
		}
	}
	rendered, err := renderer.RenderWithMarkerReplacement(content, tmplCtx, configMarkers)
	if err == nil {
		content = rendered
	}
	return content, nil
}
