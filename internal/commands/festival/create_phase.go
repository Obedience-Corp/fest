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

// CreatePhaseOptions holds options for the create phase command.
type CreatePhaseOptions struct {
	After       int
	Name        string
	PhaseType   string
	Description string // Phase objective/description (auto-fills Primary Goal marker)
	Path        string
	Festival    string // --festival: path to festival root (overrides --path)
	VarsFile    string
	Markers     string // Inline JSON with hint→value mappings
	MarkersFile string // JSON file path with hint→value mappings
	SkipMarkers bool   // Skip marker processing
	DryRun      bool   // Show markers without creating file
	JSONOutput  bool
	AgentMode   bool // Strict mode for AI agents
	Quiet       bool // Suppress stdout output (used during auto-scaffolding)
}

type createPhaseResult struct {
	OK            bool                     `json:"ok"`
	Action        string                   `json:"action"`
	Phase         map[string]any   `json:"phase,omitempty"`
	Created       []string                 `json:"created,omitempty"`
	Renumber      []string                 `json:"renumbered,omitempty"`
	Markers       []map[string]any `json:"markers,omitempty"`
	MarkersFilled int                      `json:"markers_filled,omitempty"`
	MarkersTotal  int                      `json:"markers_total,omitempty"`
	Validation    *ValidationSummary       `json:"validation,omitempty"`
	Errors        []map[string]any         `json:"errors,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Suggestions   []string                 `json:"suggestions,omitempty"`
}

// selectPhaseTemplate returns the appropriate template ID and filename for a given phase type.
// Returns (templateID, templateFilename, error) tuple.
// Phase-type templates are stored in phases/{phase_type}/GOAL.md
// Returns error for unknown phase types (no fallback - phase type is required).
func selectPhaseTemplate(phaseType string) (string, string, error) {
	pt := strings.ToLower(phaseType)
	switch pt {
	case "planning", "implementation", "research", "review", "non_coding_action", "ingest":
		return fmt.Sprintf("phase-goal-%s", pt), filepath.Join("phases", pt, "GOAL.md"), nil
	default:
		return "", "", fmt.Errorf("unknown phase type %q: must be one of planning, implementation, research, review, non_coding_action, ingest", phaseType)
	}
}

// NewCreatePhaseCommand adds 'create phase'
func NewCreatePhaseCommand() *cobra.Command {
	opts := &CreatePhaseOptions{}
	cmd := &cobra.Command{
		Use:   "phase",
		Short: "Insert a new phase and render its goal file",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return shared.StartCreatePhaseTUI(cmd.Context())
			}
			if strings.TrimSpace(opts.Name) == "" {
				return errors.Validation("--name is required (or run without flags to open TUI)")
			}
			return RunCreatePhase(cmd.Context(), opts)
		},
	}
	cmd.Flags().IntVar(&opts.After, "after", -1, "Insert after this phase number (-1 or omit to append at end)")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Phase name (required)")
	cmd.Flags().StringVar(&opts.PhaseType, "type", "planning", "Phase type (planning|implementation|research|review|ingest|non_coding_action)")
	cmd.Flags().StringVar(&opts.Description, "description", "", "Phase objective (auto-fills Primary Goal marker)")
	cmd.Flags().StringVar(&opts.Path, "path", ".", "Path to festival root (directory containing numbered phases)")
	cmd.Flags().StringVar(&opts.Festival, "festival", "", "Path to festival directory (use when not inside a festival)")
	cmd.Flags().StringVar(&opts.VarsFile, "vars-file", "", "JSON vars for rendering")
	cmd.Flags().StringVar(&opts.Markers, "markers", "", "JSON string with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.MarkersFile, "markers-file", "", "JSON file with REPLACE marker hint→value mappings")
	cmd.Flags().BoolVar(&opts.SkipMarkers, "skip-markers", false, "Skip REPLACE marker processing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show template markers without creating file")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict mode: require markers, auto-validate, block on errors, JSON output")
	return cmd
}

// phaseConfig holds all resolved configuration for phase creation.
// It is populated by resolvePhaseConfig() and passed to subsequent pipeline stages.
type phaseConfig struct {
	opts                 *CreatePhaseOptions
	display              *ui.UI
	absPath              string
	festivalsRoot        string
	festivalPath         string
	agentCfg             *config.AgentConfig
	effectiveSkipMarkers bool
	tmplRoot             string
	newNumber            int
	phaseID              string
	phaseDir             string
	vars                 map[string]any
	tmplCtx              *tpl.Context
	configMarkers        map[string]string
}

// phaseResult accumulates outputs from each pipeline stage of phase creation.
type phaseResult struct {
	goalPath         string
	markersFilled    int
	markersTotal     int
	validationResult *ValidationSummary
}

// RunCreatePhase executes the create phase command logic.
func RunCreatePhase(ctx context.Context, opts *CreatePhaseOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreatePhase")
	}

	cfg, err := resolvePhaseConfig(ctx, opts)
	if err != nil {
		return emitCreatePhaseError(opts, err)
	}

	content, err := renderPhaseGoalContent(ctx, cfg)
	if err != nil {
		return emitCreatePhaseError(opts, err)
	}

	res, err := writePhaseGoal(ctx, cfg, content)
	if err != nil {
		return emitCreatePhaseError(opts, err)
	}

	copyPhaseStructure(ctx, cfg)

	if err := processPhaseMarkers(ctx, cfg, res); err != nil {
		return emitCreatePhaseError(opts, err)
	}

	if opts.DryRun && res.markersTotal > 0 {
		return nil
	}

	if err := validatePhaseIfConfigured(ctx, cfg, res); err != nil {
		return emitCreatePhaseError(opts, err)
	}

	return emitPhaseOutput(cfg, res)
}

// resolvePhaseConfig resolves and validates all configuration needed for phase creation.
func resolvePhaseConfig(ctx context.Context, opts *CreatePhaseOptions) (*phaseConfig, error) {
	if opts.AgentMode {
		opts.JSONOutput = true
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

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err, "resolving path").WithField("path", path)
	}

	festivalsRoot := ResolveFestivalsRoot(absPath)
	festivalPath := ResolveFestivalPath(absPath)
	agentCfg := LoadEffectiveAgentConfig(festivalsRoot, festivalPath)
	effectiveSkipMarkers := config.EffectiveSkipMarkers(agentCfg, opts.AgentMode, opts.SkipMarkers)

	tmplRoot, err := tpl.LocalTemplateRoot(absPath)
	if err != nil {
		return nil, err
	}

	newNumber, phaseID, phaseDir, err := detectAndInsertPhase(ctx, absPath, opts)
	if err != nil {
		return nil, err
	}

	vars := map[string]any{}
	if strings.TrimSpace(opts.VarsFile) != "" {
		v, err := loadVarsFile(opts.VarsFile)
		if err != nil {
			return nil, errors.Wrap(err, "reading vars-file").WithField("path", opts.VarsFile)
		}
		vars = v
	}

	tmplCtx := buildPhaseTemplateContext(absPath, festivalPath, opts, newNumber, vars)

	var configMarkers map[string]string
	if festivalPath != "" {
		festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, "")
		if cfgErr == nil && festCfg != nil {
			configMarkers = extractConfigMarkers(festCfg)
		}
	}

	return &phaseConfig{
		opts: opts, display: display, absPath: absPath,
		festivalsRoot: festivalsRoot, festivalPath: festivalPath,
		agentCfg: agentCfg, effectiveSkipMarkers: effectiveSkipMarkers,
		tmplRoot: tmplRoot, newNumber: newNumber, phaseID: phaseID,
		phaseDir: phaseDir, vars: vars, tmplCtx: tmplCtx,
		configMarkers: configMarkers,
	}, nil
}

// detectAndInsertPhase handles auto-detecting the phase number and inserting
// the new phase with renumbering.
func detectAndInsertPhase(ctx context.Context, absPath string, opts *CreatePhaseOptions) (int, string, string, error) {
	if opts.After == -1 {
		parser := festival.NewParser()
		phases, parseErr := parser.ParsePhases(ctx, absPath)
		if parseErr == nil && len(phases) > 0 {
			maxNum := 0
			for _, p := range phases {
				if p.Number > maxNum {
					maxNum = p.Number
				}
			}
			opts.After = maxNum
		} else {
			opts.After = 0
		}
	}

	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
	if err := ren.InsertPhase(ctx, absPath, opts.After, opts.Name); err != nil {
		return 0, "", "", errors.Wrap(err, "inserting phase")
	}

	newNumber := opts.After + 1
	phaseID := tpl.FormatPhaseID(newNumber, opts.Name)
	phaseDir := filepath.Join(absPath, phaseID)
	return newNumber, phaseID, phaseDir, nil
}

// buildPhaseTemplateContext constructs the template context for phase creation.
func buildPhaseTemplateContext(absPath, festivalPath string, opts *CreatePhaseOptions, newNumber int, vars map[string]any) *tpl.Context {
	tmplCtx, ctxErr := tpl.BuildContextFromPath(absPath, festivalPath)
	if ctxErr != nil {
		tmplCtx = tpl.NewContext()
	}
	tmplCtx.SetPhase(newNumber, opts.Name, opts.PhaseType)
	if opts.Description != "" {
		tmplCtx.SetPhaseObjective(opts.Description)
	}
	tmplCtx.ComputeStructureVariables()
	for k, v := range vars {
		tmplCtx.SetCustom(k, v)
	}
	return tmplCtx
}

// renderPhaseGoalContent renders the PHASE_GOAL.md content from templates.
func renderPhaseGoalContent(ctx context.Context, cfg *phaseConfig) (string, error) {
	templateID, templateFilename, phaseTypeErr := selectPhaseTemplate(cfg.opts.PhaseType)
	if phaseTypeErr != nil {
		return "", errors.Validation(phaseTypeErr.Error()).WithField("phase_type", cfg.opts.PhaseType)
	}

	catalog, _ := tpl.LoadCatalog(ctx, cfg.tmplRoot)
	mgr := tpl.NewManager()
	var content string
	var renderErr error

	if catalog != nil {
		content, renderErr = mgr.RenderByID(ctx, catalog, templateID, cfg.tmplCtx)
	}
	if renderErr != nil || content == "" {
		tpath := filepath.Join(cfg.tmplRoot, templateFilename)
		if _, err := os.Stat(tpath); err == nil {
			loader := tpl.NewLoader()
			t, err := loader.Load(ctx, tpath)
			if err != nil {
				return "", errors.Wrap(err, "loading phase goal template").WithField("template", templateFilename)
			}
			if strings.Contains(t.Content, "{{") {
				out, err := mgr.Render(t, cfg.tmplCtx)
				if err != nil {
					return "", errors.Wrap(err, "rendering phase goal")
				}
				content = out
			} else {
				content = t.Content
			}
		}
	}

	if content == "" {
		content = fmt.Sprintf("# Phase Goal: %s\n\n## Objective\n\n[REPLACE: Describe the phase objective]\n\n## Success Criteria\n\n- [ ] [REPLACE: Criterion 1]\n- [ ] [REPLACE: Criterion 2]\n", cfg.opts.Name)
	}

	return content, nil
}

// writePhaseGoal prepares and writes the PHASE_GOAL.md file with frontmatter and markers.
func writePhaseGoal(ctx context.Context, cfg *phaseConfig, content string) (*phaseResult, error) {
	if err := os.MkdirAll(cfg.phaseDir, 0755); err != nil {
		return nil, errors.IO("creating phase dir", err).WithField("path", cfg.phaseDir)
	}

	content = stripTemplateFrontmatter(content)

	parentFestivalID := filepath.Base(cfg.absPath)
	fm := frontmatter.NewPhaseFrontmatter(cfg.phaseID, cfg.opts.Name, parentFestivalID, cfg.newNumber, frontmatter.PhaseType(cfg.opts.PhaseType))
	contentWithFM, fmErr := frontmatter.InjectString(content, fm)
	if fmErr != nil {
		return nil, errors.Wrap(fmErr, "injecting frontmatter")
	}
	content = contentWithFM

	if !cfg.effectiveSkipMarkers {
		renderer := tpl.NewRenderer()
		rendered, renderErr := renderer.RenderWithMarkerReplacement(content, cfg.tmplCtx, cfg.configMarkers)
		if renderErr == nil {
			content = rendered
		}
	}

	goalPath := filepath.Join(cfg.phaseDir, "PHASE_GOAL.md")
	if err := os.WriteFile(goalPath, []byte(content), 0644); err != nil {
		return nil, errors.IO("writing phase goal", err).WithField("path", goalPath)
	}

	return &phaseResult{goalPath: goalPath}, nil
}

// copyPhaseStructure copies additional phase structure files from the template directory.
func copyPhaseStructure(ctx context.Context, cfg *phaseConfig) {
	templateDir := filepath.Join(cfg.tmplRoot, "phases", cfg.opts.PhaseType)
	entries, readErr := os.ReadDir(templateDir)
	if readErr != nil {
		return
	}

	renderer := tpl.NewRenderer()
	for _, entry := range entries {
		if entry.Name() == "GOAL.md" || entry.Name() == "gates" {
			continue
		}

		src := filepath.Join(templateDir, entry.Name())
		dst := filepath.Join(cfg.phaseDir, entry.Name())

		if _, statErr := os.Stat(dst); statErr == nil {
			if shared.IsVerbose() {
				cfg.display.Info("Skipping %s: already exists", entry.Name())
			}
			continue
		}

		if entry.IsDir() {
			if copyErr := copyDirectoryWithMarkers(ctx, src, dst, renderer, cfg.tmplCtx, cfg.configMarkers); copyErr != nil {
				if shared.IsVerbose() {
					cfg.display.Warning("Failed to copy directory %s: %v", entry.Name(), copyErr)
				}
			}
		} else {
			fileContent, readFileErr := os.ReadFile(src)
			if readFileErr != nil {
				if shared.IsVerbose() {
					cfg.display.Warning("Failed to read %s: %v", entry.Name(), readFileErr)
				}
				continue
			}
			processed, procErr := renderer.RenderWithMarkerReplacement(string(fileContent), cfg.tmplCtx, cfg.configMarkers)
			if procErr != nil {
				processed = string(fileContent)
			}
			if writeErr := os.WriteFile(dst, []byte(processed), 0644); writeErr != nil {
				if shared.IsVerbose() {
					cfg.display.Warning("Failed to write %s: %v", entry.Name(), writeErr)
				}
			}
		}
	}
}

// processPhaseMarkers processes REPLACE markers in the phase goal file.
func processPhaseMarkers(ctx context.Context, cfg *phaseConfig, res *phaseResult) error {
	markerResult, err := ProcessMarkers(ctx, MarkerOptions{
		FilePath:    res.goalPath,
		Markers:     cfg.opts.Markers,
		MarkersFile: cfg.opts.MarkersFile,
		SkipMarkers: cfg.effectiveSkipMarkers,
		DryRun:      cfg.opts.DryRun,
		JSONOutput:  cfg.opts.JSONOutput,
	})
	if err != nil {
		return errors.Wrap(err, "processing markers")
	}

	if cfg.opts.DryRun && markerResult != nil {
		if err := PrintDryRunMarkers(markerResult, cfg.opts.JSONOutput); err != nil {
			return err
		}
	}

	if markerResult != nil {
		res.markersFilled = markerResult.Filled
		res.markersTotal = markerResult.Total
	}
	return nil
}

// validatePhaseIfConfigured runs post-create validation if agent config requires it.
func validatePhaseIfConfigured(ctx context.Context, cfg *phaseConfig, res *phaseResult) error {
	if !config.ShouldValidate(cfg.agentCfg, cfg.opts.AgentMode) || cfg.festivalPath == "" {
		return nil
	}

	validationResult, err := RunPostCreateValidation(ctx, cfg.festivalPath)
	if err != nil {
		if !cfg.opts.JSONOutput {
			cfg.display.Warning("Validation failed: %v", err)
		}
		return nil
	}
	res.validationResult = validationResult

	if validationResult != nil && !validationResult.OK {
		if config.ShouldBlockOnErrors(cfg.agentCfg, cfg.opts.AgentMode) {
			return errors.Validation("validation errors detected - fix issues before proceeding")
		}
	}
	return nil
}

// emitPhaseOutput handles both JSON and human-readable output for phase creation.
func emitPhaseOutput(cfg *phaseConfig, res *phaseResult) error {
	opts := cfg.opts
	remainingMarkers := res.markersTotal - res.markersFilled

	if opts.JSONOutput {
		return emitPhaseJSON(cfg, res, remainingMarkers)
	}

	if opts.Quiet {
		return nil
	}

	if remainingMarkers > 0 {
		fmt.Println()
		cfg.display.Error("🚫 CRITICAL: %d unfilled markers - festival cannot be executed until resolved", remainingMarkers)
		cfg.display.Info("   Run 'fest wizard fill PHASE_GOAL.md' to fill markers interactively")
		cfg.display.Info("   Or edit PHASE_GOAL.md directly to replace [REPLACE: ...] markers")
		fmt.Println()
	}

	cfg.display.Success("Created phase: %s", cfg.phaseID)
	cfg.display.Info("  └── %s", "PHASE_GOAL.md")

	fmt.Println()
	fmt.Println(ui.H2("Next Steps"))
	fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("1. cd %s", cfg.phaseDir)))
	if remainingMarkers > 0 {
		fmt.Printf("  %s\n", ui.Info("2. Edit PHASE_GOAL.md to define phase objectives"))
	}
	if opts.PhaseType == "research" {
		fmt.Printf("  %s\n", ui.Info("3. Create subdirectories for research topics"))
		fmt.Printf("  %s\n", ui.Info("4. fest research create --type investigation --title \"topic\""))
	} else {
		fmt.Printf("  %s\n", ui.Info("3. fest create sequence --name \"requirements\""))
		fmt.Printf("  %s\n", ui.Info("4. fest validate (check completion status)"))
	}
	fmt.Println()
	fmt.Println(ui.H2("Discover More Commands"))
	fmt.Printf("  %s %s\n", ui.Value("fest status"), ui.Dim("View festival progress"))
	fmt.Printf("  %s %s\n", ui.Value("fest next"), ui.Dim("Find what to work on next"))
	fmt.Printf("  %s %s\n", ui.Value("fest show plan"), ui.Dim("View the execution plan"))
	return nil
}

// emitPhaseJSON emits JSON output for phase creation.
func emitPhaseJSON(cfg *phaseConfig, res *phaseResult, remainingMarkers int) error {
	warnings := []string{}
	if remainingMarkers > 0 {
		warnings = append(warnings,
			fmt.Sprintf("CRITICAL: %d unfilled markers - festival cannot be executed until resolved", remainingMarkers),
			"Run 'fest wizard fill PHASE_GOAL.md' to fill markers interactively",
		)
	}
	warnings = append(warnings, "Next: Create sequences with 'fest create sequence --name SEQUENCE_NAME'")

	suggestions := []string{
		"fest status        - View festival progress",
		"fest next          - Find what to work on next",
		"fest show plan     - View the execution plan",
		"fest validate      - Check completion status",
	}

	return emitCreatePhaseJSON(cfg.opts, createPhaseResult{
		OK:     true,
		Action: "create_phase",
		Phase: map[string]any{
			"number": cfg.newNumber,
			"id":     cfg.phaseID,
			"name":   cfg.opts.Name,
			"type":   cfg.opts.PhaseType,
		},
		Created:       []string{res.goalPath},
		Renumber:      []string{},
		MarkersFilled: res.markersFilled,
		MarkersTotal:  res.markersTotal,
		Validation:    res.validationResult,
		Warnings:      warnings,
		Suggestions:   suggestions,
	})
}

func emitCreatePhaseError(opts *CreatePhaseOptions, err error) error {
	if opts.JSONOutput {
		_ = emitCreatePhaseJSON(opts, createPhaseResult{
			OK:     false,
			Action: "create_phase",
			Errors: []map[string]any{{
				"code":    "error",
				"message": err.Error(),
			}},
		})
		return nil
	}
	return err
}

func emitCreatePhaseJSON(opts *CreatePhaseOptions, res createPhaseResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// stripTemplateFrontmatter removes any existing frontmatter from content.
// This is used to strip template metadata frontmatter before injecting proper document frontmatter.
func stripTemplateFrontmatter(content string) string {
	// Check if content starts with frontmatter
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		trimmed := strings.TrimSpace(content)
		// Find the closing --- to strip template metadata frontmatter
		rest := trimmed[3:] // skip opening ---
		endIdx := strings.Index(rest, "---")
		if endIdx != -1 {
			// Skip past the closing --- and any following newlines
			afterFrontmatter := rest[endIdx+3:]
			afterFrontmatter = strings.TrimLeft(afterFrontmatter, "\n\r")
			return afterFrontmatter
		}
	}
	return content
}

// copyDirectoryWithMarkers recursively copies a directory, processing markers in all files.
func copyDirectoryWithMarkers(ctx context.Context, src, dst string, renderer tpl.Renderer, tmplCtx *tpl.Context, configMarkers map[string]string) error {
	// Check context
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return errors.IO("creating directory", err).WithField("path", dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.IO("reading directory", err).WithField("path", src)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Don't overwrite existing files
		if _, statErr := os.Stat(dstPath); statErr == nil {
			continue
		}

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDirectoryWithMarkers(ctx, srcPath, dstPath, renderer, tmplCtx, configMarkers); err != nil {
				return err
			}
		} else {
			// Copy file with marker processing
			content, readErr := os.ReadFile(srcPath)
			if readErr != nil {
				return errors.IO("reading file", readErr).WithField("path", srcPath)
			}

			// Process markers in content
			processed, procErr := renderer.RenderWithMarkerReplacement(string(content), tmplCtx, configMarkers)
			if procErr != nil {
				// Fall back to original content if processing fails
				processed = string(content)
			}

			if err := os.WriteFile(dstPath, []byte(processed), 0644); err != nil {
				return errors.IO("writing file", err).WithField("path", dstPath)
			}
		}
	}

	return nil
}
