package festival

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/registry"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// CreateFestivalOptions holds options for the create festival command.
type CreateFestivalOptions struct {
	Name        string
	Goal        string
	Tags        string
	Project     string // Project directory path
	Type        string // Festival type (standard, implementation, research, quick, ritual)
	VarsFile    string
	Markers     string // Inline JSON with hint→value mappings
	MarkersFile string // JSON file path with hint→value mappings
	SkipMarkers bool   // Skip marker processing
	DryRun      bool   // Show markers without creating file
	JSONOutput  bool
	Dest        string // "active" or "planning"
	AgentMode   bool   // Strict mode for AI agents
}

type createFestivalResult struct {
	OK                bool                     `json:"ok"`
	Action            string                   `json:"action"`
	Festival          map[string]string        `json:"festival,omitempty"`
	Created           []string                 `json:"created,omitempty"`
	AutoPhasesCreated []string                 `json:"auto_phases_created,omitempty"`
	GatesDirectory    string                   `json:"gates_directory,omitempty"`
	FestYAML          string                   `json:"fest_yaml,omitempty"`
	GateTemplates     []string                 `json:"gate_templates,omitempty"`
	ProjectPath       string                   `json:"project_path,omitempty"`
	ProjectLinked     bool                     `json:"project_linked,omitempty"`
	Markers           []map[string]interface{} `json:"markers,omitempty"`
	MarkersFilled     int                      `json:"markers_filled,omitempty"`
	MarkersTotal      int                      `json:"markers_total,omitempty"`
	Validation        *ValidationSummary       `json:"validation,omitempty"`
	Errors            []map[string]any         `json:"errors,omitempty"`
	Warnings          []string                 `json:"warnings,omitempty"`
	Extra             map[string]interface{}   `json:"extra,omitempty"`
}

// createConfig holds all resolved configuration for festival creation.
// It is populated by resolveCreateConfig() and passed to subsequent pipeline stages.
type createConfig struct {
	opts             *CreateFestivalOptions
	display          *ui.UI
	festivalsRoot    string
	tmplRoot         string
	slug             string
	festivalID       string
	destCategory     string
	dirName          string
	destDir          string
	vars             map[string]interface{}
	agentCfg         *config.AgentConfig
	skipMarkers      bool
	tmplCtx          *tpl.Context
	festivalType     *types.FestivalType
	festivalTypeName string
}

// NewCreateFestivalCommand adds 'create festival'
func NewCreateFestivalCommand() *cobra.Command {
	opts := &CreateFestivalOptions{}
	cmd := &cobra.Command{
		Use:   "festival",
		Short: "Create a new festival scaffold under festivals/(active|planning)",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no flags were provided, open TUI for this flow
			if cmd.Flags().NFlag() == 0 {
				return shared.StartCreateFestivalTUI(cmd.Context())
			}
			// Otherwise, require name and proceed
			if strings.TrimSpace(opts.Name) == "" {
				return errors.Validation("--name is required (or run without flags to open TUI)")
			}
			return RunCreateFestival(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.Name, "name", "", "Festival name (required)")
	cmd.Flags().StringVar(&opts.Goal, "goal", "", "Festival goal")
	cmd.Flags().StringVar(&opts.Tags, "tags", "", "Comma-separated tags")
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Project directory path (auto-links to festival)")
	cmd.Flags().StringVar(&opts.Type, "type", "", "Festival type (standard, implementation, research, quick, ritual)")
	cmd.Flags().StringVar(&opts.VarsFile, "vars-file", "", "JSON file with variables")
	cmd.Flags().StringVar(&opts.Markers, "markers", "", "JSON string with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.MarkersFile, "markers-file", "", "JSON file with REPLACE marker hint→value mappings")
	cmd.Flags().BoolVar(&opts.SkipMarkers, "skip-markers", false, "Skip REPLACE marker processing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show template markers without creating file")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().StringVar(&opts.Dest, "dest", "active", "Destination under festivals/: active, planning, or ritual")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict mode: require markers, auto-validate, block on errors, JSON output")
	return cmd
}

// resolveCreateConfig resolves and validates all configuration needed for festival
// creation including workspace resolution, template root, slug/ID generation,
// vars loading, festival type loading, and template context building.
func resolveCreateConfig(ctx context.Context, opts *CreateFestivalOptions) (*createConfig, error) {
	if opts.AgentMode {
		opts.JSONOutput = true
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())
	cwd, _ := os.Getwd()

	festivalsRoot, err := workspace.FindFestivals(cwd)
	if err != nil {
		return nil, err
	}
	if festivalsRoot == "" {
		return nil, errors.NotFound("festivals directory").
			WithHint("Check the path and try again, or run from a different directory")
	}

	agentCfg := LoadEffectiveAgentConfig(festivalsRoot, "")
	skipMarkers := config.EffectiveSkipMarkers(agentCfg, opts.AgentMode, opts.SkipMarkers)
	tmplRoot := filepath.Join(festivalsRoot, ".festival", "templates")

	vars := map[string]interface{}{}
	if strings.TrimSpace(opts.VarsFile) != "" {
		v, err := loadVarsFile(opts.VarsFile)
		if err != nil {
			return nil, errors.Wrap(err, "reading vars-file").WithField("path", opts.VarsFile)
		}
		vars = v
	}

	slug := Slugify(opts.Name)
	destCategory := strings.ToLower(strings.TrimSpace(opts.Dest))
	if opts.Type == "ritual" && destCategory == "active" {
		destCategory = "ritual"
	}
	if destCategory != "planning" && destCategory != "active" && destCategory != "ritual" {
		destCategory = "active"
	}

	var festivalID string
	if opts.Type == "ritual" {
		festivalID, err = id.GenerateRitualID(ctx, opts.Name, festivalsRoot)
	} else {
		festivalID, err = id.GenerateID(ctx, opts.Name, festivalsRoot)
	}
	if err != nil {
		return nil, errors.Wrap(err, "generating festival ID").WithField("name", opts.Name)
	}

	tmplCtx := tpl.NewContext()
	tmplCtx.SetFestival(opts.Name, opts.Goal, parseTags(opts.Tags))
	tmplCtx.SetFestivalID(festivalID)
	tmplCtx.ComputeStructureVariables()
	for k, v := range vars {
		tmplCtx.SetCustom(k, v)
	}

	dirName := fmt.Sprintf("%s-%s", slug, festivalID)
	destDir := filepath.Join(festivalsRoot, destCategory, dirName)

	var festivalType *types.FestivalType
	var festivalTypeName string
	if opts.Type != "" {
		typesCfg, typeErr := types.LoadFestivalTypesConfig(ctx)
		if typeErr != nil {
			return nil, errors.Wrap(typeErr, "loading festival types config")
		}
		ft, typeErr := typesCfg.GetFestivalType(opts.Type)
		if typeErr != nil {
			return nil, typeErr
		}
		festivalType = ft
		festivalTypeName = ft.Name
	}

	return &createConfig{
		opts:             opts,
		display:          display,
		festivalsRoot:    festivalsRoot,
		tmplRoot:         tmplRoot,
		slug:             slug,
		festivalID:       festivalID,
		destCategory:     destCategory,
		dirName:          dirName,
		destDir:          destDir,
		vars:             vars,
		agentCfg:         agentCfg,
		skipMarkers:      skipMarkers,
		tmplCtx:          tmplCtx,
		festivalType:     festivalType,
		festivalTypeName: festivalTypeName,
	}, nil
}

// createResult accumulates outputs from each pipeline stage of festival creation.
type createResult struct {
	created           []string
	copiedGates       []string
	festConfigPath    string
	festConfig        *config.FestivalConfig
	autoPhasesCreated []string
	projectPath       string
	projectLinked     bool
	markersFilled     int
	markersTotal      int
	allMarkers        []map[string]interface{}
	validationResult  *ValidationSummary
}

// RunCreateFestival executes the create festival command logic.
func RunCreateFestival(ctx context.Context, opts *CreateFestivalOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreateFestival")
	}

	cfg, err := resolveCreateConfig(ctx, opts)
	if err != nil {
		return emitCreateFestivalError(opts, err)
	}

	if err := scaffoldFestivalDirectory(ctx, cfg); err != nil {
		return emitCreateFestivalError(opts, err)
	}

	created, copiedGates, err := renderFestivalTemplates(ctx, cfg)
	if err != nil {
		return emitCreateFestivalError(opts, err)
	}

	res, err := writeFestYaml(ctx, cfg)
	if err != nil {
		return emitCreateFestivalError(opts, err)
	}
	created = append(created, res.festConfigPath)

	created, res.autoPhasesCreated = autoScaffoldPhases(ctx, cfg, created)

	recordInitialSize(ctx, cfg, res.festConfig)
	registerFestival(ctx, cfg)

	res.created = created
	res.copiedGates = copiedGates
	if err := processAllMarkers(ctx, cfg, res); err != nil {
		return emitCreateFestivalError(opts, err)
	}

	if opts.DryRun && res.markersTotal > 0 {
		result := &MarkerResult{Markers: res.allMarkers, Total: res.markersTotal}
		if err := PrintDryRunMarkers(result, opts.JSONOutput); err != nil {
			return emitCreateFestivalError(opts, err)
		}
		return nil
	}

	if err := validateIfConfigured(ctx, cfg, res); err != nil {
		return emitCreateFestivalError(opts, err)
	}

	return emitCreateOutput(cfg, res)
}

// scaffoldFestivalDirectory creates the festival directory structure.
func scaffoldFestivalDirectory(ctx context.Context, cfg *createConfig) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if err := os.MkdirAll(cfg.destDir, 0755); err != nil {
		return errors.IO("creating festival directory", err).WithField("path", cfg.destDir)
	}
	return nil
}

// renderFestivalTemplates renders the core festival files (OVERVIEW, GOAL, RULES,
// TODO) from templates and copies gate templates organized by phase type.
func renderFestivalTemplates(ctx context.Context, cfg *createConfig) ([]string, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "context cancelled")
	}

	mgr := tpl.NewManager()
	var created []string

	core := []struct{ Template, Out string }{
		{"festival/OVERVIEW.md", "FESTIVAL_OVERVIEW.md"},
		{"festival/GOAL.md", "FESTIVAL_GOAL.md"},
		{"festival/RULES.md", "FESTIVAL_RULES.md"},
		{"festival/TODO.md", "TODO.md"},
	}

	for _, c := range core {
		outPath, err := renderCoreFile(ctx, cfg, mgr, c.Template, c.Out)
		if err != nil {
			return nil, nil, err
		}
		if outPath != "" {
			created = append(created, outPath)
		}
	}

	copiedGates, gateCreated, err := copyGateTemplates(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	created = append(created, gateCreated...)

	return created, copiedGates, nil
}

// renderCoreFile renders a single core festival file from a template.
// Returns the output path, or empty string if the template was not found.
func renderCoreFile(ctx context.Context, cfg *createConfig, mgr *tpl.Manager, templateName, outName string) (string, error) {
	tpath := filepath.Join(cfg.tmplRoot, templateName)
	if _, err := os.Stat(tpath); err != nil {
		if !cfg.opts.JSONOutput {
			cfg.display.Warning("Template not found, skipping: %s", templateName)
		}
		return "", nil
	}

	loader := tpl.NewLoader()
	t, err := loader.Load(ctx, tpath)
	if err != nil {
		return "", errors.Wrap(err, "loading template").WithField("template", templateName)
	}

	content := renderOrCopyTemplate(mgr, t, cfg.tmplCtx)

	if outName == "FESTIVAL_GOAL.md" && !strings.HasPrefix(strings.TrimSpace(content), "---") {
		fm := frontmatter.NewFrontmatter(frontmatter.TypeFestival, cfg.festivalID, cfg.opts.Name)
		fm.Status = frontmatter.StatusPlanning
		contentWithFM, fmErr := frontmatter.InjectString(content, fm)
		if fmErr != nil {
			return "", errors.Wrap(fmErr, "injecting frontmatter")
		}
		content = contentWithFM
	}

	if !cfg.skipMarkers {
		renderer := tpl.NewRenderer()
		rendered, renderErr := renderer.RenderWithMarkerReplacement(content, cfg.tmplCtx, nil)
		if renderErr == nil {
			content = rendered
		}
	}

	outPath := filepath.Join(cfg.destDir, outName)
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", errors.IO("writing file", err).WithField("path", outPath)
	}
	return outPath, nil
}

// renderOrCopyTemplate renders a template if it contains variables, otherwise returns content as-is.
func renderOrCopyTemplate(mgr *tpl.Manager, t *tpl.Template, tmplCtx *tpl.Context) string {
	requires := t.Metadata != nil && len(t.Metadata.RequiredVariables) > 0
	if requires || strings.Contains(t.Content, "{{") {
		out, err := mgr.Render(t, tmplCtx)
		if err == nil {
			return out
		}
	}
	return t.Content
}

// copyGateTemplates copies quality gate templates from the template directory
// into the festival's gates/ directory, organized by phase type.
func copyGateTemplates(ctx context.Context, cfg *createConfig) ([]string, []string, error) {
	gatesDir := filepath.Join(cfg.destDir, "gates")
	srcPhasesDir := filepath.Join(cfg.tmplRoot, "phases")
	phaseTypes := []string{"planning", "implementation", "research", "review", "non_coding_action"}

	var copiedGates, created []string
	for _, phaseType := range phaseTypes {
		srcGatesDir := filepath.Join(srcPhasesDir, phaseType, "gates")
		if _, err := os.Stat(srcGatesDir); os.IsNotExist(err) {
			continue
		}

		destGatesDir := filepath.Join(gatesDir, phaseType)
		if err := os.MkdirAll(destGatesDir, 0755); err != nil {
			return nil, nil, errors.IO("creating gates directory", err).WithField("path", destGatesDir)
		}

		gateEntries, err := os.ReadDir(srcGatesDir)
		if err != nil {
			return nil, nil, errors.IO("reading gates directory", err).WithField("path", srcGatesDir)
		}

		for _, gateEntry := range gateEntries {
			if gateEntry.IsDir() || !strings.HasSuffix(gateEntry.Name(), ".md") {
				continue
			}
			srcPath := filepath.Join(srcGatesDir, gateEntry.Name())
			destPath := filepath.Join(destGatesDir, gateEntry.Name())

			content, err := os.ReadFile(srcPath)
			if err != nil {
				return nil, nil, errors.IO("reading gate template", err).WithField("path", srcPath)
			}

			processed := string(content)
			if !cfg.skipMarkers {
				renderer := tpl.NewRenderer()
				rendered, renderErr := renderer.RenderWithMarkerReplacement(processed, cfg.tmplCtx, nil)
				if renderErr == nil {
					processed = rendered
				}
			}

			if err := os.WriteFile(destPath, []byte(processed), 0644); err != nil {
				return nil, nil, errors.IO("writing gate template", err).WithField("path", destPath)
			}
			copiedGates = append(copiedGates, destPath)
			created = append(created, destPath)
		}
	}
	return copiedGates, created, nil
}

// writeFestYaml generates and writes the fest.yaml configuration file for the
// festival, including metadata, type config, ritual config, and project path.
func writeFestYaml(ctx context.Context, cfg *createConfig) (*createResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	festConfig := config.DefaultFestivalConfig()
	now := time.Now().UTC()

	festConfig.Metadata = config.FestivalMetadata{
		ID:           cfg.festivalID,
		UUID:         uuid.New().String(),
		Name:         cfg.opts.Name,
		Goal:         cfg.opts.Goal,
		FestivalType: cfg.festivalTypeName,
		CreatedAt:    now,
		StatusHistory: []config.StatusChange{
			{
				Status:    cfg.destCategory,
				Timestamp: now,
				Path:      cfg.destDir,
				Notes:     "Festival created",
			},
		},
	}

	res := &createResult{festConfig: festConfig}

	if cfg.opts.Project != "" {
		resolveProjectLink(cfg, festConfig, res)
	}

	if cfg.festivalType != nil {
		populateTypeConfig(festConfig, cfg.festivalType)
	}

	if cfg.opts.Type == "ritual" {
		festConfig.RitualConfig = &config.RitualConfig{
			Schedule: "manual",
			RunCount: 0,
		}
	}

	res.festConfigPath = filepath.Join(cfg.destDir, config.FestivalConfigFileName)
	if err := config.SaveFestivalConfig(cfg.destDir, festConfig); err != nil {
		return nil, errors.Wrap(err, "writing fest.yaml").WithField("path", res.festConfigPath)
	}

	return res, nil
}

// resolveProjectLink resolves and optionally links a project path for the festival.
func resolveProjectLink(cfg *createConfig, festConfig *config.FestivalConfig, res *createResult) {
	workspaceRoot := filepath.Dir(cfg.festivalsRoot)
	resolved, err := ResolveProjectPath(cfg.opts.Project, workspaceRoot)
	if err != nil {
		if !cfg.opts.JSONOutput {
			cfg.display.Warning("Could not resolve project path: %v", err)
		}
		return
	}

	res.projectPath = resolved
	festConfig.ProjectPath = resolved

	if validateErr := ValidateProjectPath(resolved); validateErr != nil {
		if !cfg.opts.JSONOutput {
			cfg.display.Warning("Project path doesn't exist yet: %s", resolved)
			cfg.display.Info("Link will be created when path exists")
		}
		return
	}

	nav, navErr := navigation.LoadNavigation()
	if navErr == nil {
		nav.SetLinkWithPath(cfg.dirName, resolved, cfg.destDir)
		if saveErr := nav.Save(); saveErr == nil {
			res.projectLinked = true
			if !cfg.opts.JSONOutput {
				cfg.display.Success("Linked to project: %s", resolved)
			}
		}
	}
}

// populateTypeConfig fills in the TypeConfig section of a festival config
// from a festival type definition.
func populateTypeConfig(festConfig *config.FestivalConfig, festivalType *types.FestivalType) {
	autoPhaseNames := make([]string, 0, len(festivalType.GetAutoPhases()))
	for _, p := range festivalType.GetAutoPhases() {
		autoPhaseNames = append(autoPhaseNames, p.Name)
	}

	pendingPhases := make([]config.PendingPhase, 0, len(festivalType.GetPendingPhases()))
	for _, p := range festivalType.GetPendingPhases() {
		pendingPhases = append(pendingPhases, config.PendingPhase{
			Name:      p.Name,
			Type:      p.Type,
			Role:      p.Role,
			Trigger:   p.Trigger,
			Generator: p.Generator,
		})
	}

	festConfig.TypeConfig = &config.TypeConfigMetadata{
		AutoPhases:    autoPhaseNames,
		PendingPhases: pendingPhases,
		SkipIngestion: festivalType.SkipIngestion,
	}
}

// autoScaffoldPhases creates phases from the festival type's auto-phase definitions.
func autoScaffoldPhases(ctx context.Context, cfg *createConfig, created []string) ([]string, []string) {
	if cfg.festivalType == nil {
		return created, nil
	}

	var autoPhasesCreated []string
	autoPhases := cfg.festivalType.GetAutoPhases()
	for i, phaseSpec := range autoPhases {
		phaseOpts := &CreatePhaseOptions{
			After:       i,
			Name:        phaseSpec.Name,
			PhaseType:   mapPhaseSpecType(phaseSpec.Type),
			Description: phaseSpec.Description,
			Path:        cfg.destDir,
			SkipMarkers: cfg.skipMarkers,
			JSONOutput:  false,
			AgentMode:   false,
			Quiet:       true,
		}

		if phaseErr := RunCreatePhase(ctx, phaseOpts); phaseErr != nil {
			if !cfg.opts.JSONOutput {
				cfg.display.Warning("Failed to auto-create phase %s: %v", phaseSpec.Name, phaseErr)
			}
			continue
		}

		phaseID := tpl.FormatPhaseID(i+1, phaseSpec.Name)
		autoPhasesCreated = append(autoPhasesCreated, phaseID)
		created = append(created, filepath.Join(cfg.destDir, phaseID, "PHASE_GOAL.md"))
	}
	return created, autoPhasesCreated
}

// recordInitialSize records the initial content size for token delta tracking.
func recordInitialSize(ctx context.Context, cfg *createConfig, festConfig *config.FestivalConfig) {
	initialSize, sizeErr := progress.ComputeDirectorySize(ctx, cfg.destDir)
	if sizeErr == nil && initialSize > 0 {
		festConfig.Metadata.InitialSizeBytes = initialSize
		_ = config.SaveFestivalConfig(cfg.destDir, festConfig)
	}
}

// registerFestival records the festival in the ID registry with event logging.
func registerFestival(ctx context.Context, cfg *createConfig) {
	regPath := registry.GetEventsPath(cfg.festivalsRoot)
	reg, regErr := registry.Load(ctx, regPath)
	if regErr != nil {
		return
	}
	now := time.Now().UTC()
	regEntry := registry.RegistryEntry{
		ID:        cfg.festivalID,
		Name:      cfg.opts.Name,
		Status:    cfg.destCategory,
		Path:      cfg.destDir,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = reg.AddWithEvent(ctx, regEntry)
}

// processAllMarkers processes REPLACE markers in all created files.
func processAllMarkers(ctx context.Context, cfg *createConfig, res *createResult) error {
	for _, filePath := range res.created {
		markerResult, err := ProcessMarkers(ctx, MarkerOptions{
			FilePath:    filePath,
			Markers:     cfg.opts.Markers,
			MarkersFile: cfg.opts.MarkersFile,
			SkipMarkers: cfg.skipMarkers,
			DryRun:      cfg.opts.DryRun,
			JSONOutput:  cfg.opts.JSONOutput,
		})
		if err != nil {
			return errors.Wrap(err, "processing markers")
		}
		if markerResult != nil {
			res.markersFilled += markerResult.Filled
			res.markersTotal += markerResult.Total
			res.allMarkers = append(res.allMarkers, markerResult.Markers...)
		}
	}
	return nil
}

// validateIfConfigured runs post-create validation if agent config requires it.
func validateIfConfigured(ctx context.Context, cfg *createConfig, res *createResult) error {
	if !config.ShouldValidate(cfg.agentCfg, cfg.opts.AgentMode) {
		return nil
	}

	validationResult, err := RunPostCreateValidation(ctx, cfg.destDir)
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

// emitCreateOutput handles both JSON and human-readable output for festival creation.
func emitCreateOutput(cfg *createConfig, res *createResult) error {
	opts := cfg.opts
	remainingMarkers := res.markersTotal - res.markersFilled
	gatesDir := filepath.Join(cfg.destDir, "gates")

	if opts.JSONOutput {
		return emitCreateJSON(cfg, res, gatesDir, remainingMarkers)
	}

	if remainingMarkers > 0 {
		fmt.Println()
		cfg.display.Error("🚫 CRITICAL: %d unfilled markers - festival cannot be executed until resolved", remainingMarkers)
		cfg.display.Info("   Run 'fest validate' to see which files need editing")
		cfg.display.Info("   Run 'fest wizard fill FESTIVAL_GOAL.md' to fill markers interactively")
		fmt.Println()
	}

	cfg.display.Success("Created festival: %s (%s)", cfg.dirName, cfg.destCategory)
	cfg.display.Info("  ID: %s", cfg.festivalID)
	if cfg.festivalTypeName != "" {
		cfg.display.Info("  Type: %s", cfg.festivalTypeName)
	}
	if len(res.autoPhasesCreated) > 0 {
		cfg.display.Success("Auto-created %d phase(s): %s", len(res.autoPhasesCreated), strings.Join(res.autoPhasesCreated, ", "))
	}
	for _, p := range res.created {
		cfg.display.Info("  • %s", p)
	}

	if len(res.copiedGates) > 0 {
		cfg.display.Success("Created gates/ directory with %d templates organized by phase type", len(res.copiedGates))
		cfg.display.Info("  Quality gates configured in fest.yaml")
	}

	if res.projectPath != "" {
		if res.projectLinked {
			cfg.display.Success("Project path: %s (linked)", res.projectPath)
		} else {
			cfg.display.Info("Project path: %s (not linked - path doesn't exist yet)", res.projectPath)
		}
	}

	fmt.Println()
	fmt.Println(ui.H2("Next Steps"))
	fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("1. cd %s", cfg.destDir)))
	if remainingMarkers > 0 {
		fmt.Printf("  %s\n", ui.Info("2. Edit FESTIVAL_GOAL.md to define your objectives"))
		fmt.Printf("  %s\n", ui.Info("3. fest create phase --name \"PLAN\" --after 0"))
		fmt.Printf("  %s\n", ui.Info("4. fest validate (check completion status)"))
	} else {
		fmt.Printf("  %s\n", ui.Info("2. fest create phase --name \"PLAN\" --after 0"))
		fmt.Printf("  %s\n", ui.Info("3. fest create phase --name \"IMPLEMENT\" --after 1"))
		fmt.Printf("  %s\n", ui.Info("4. After creating tasks: fest gates apply --approve"))
	}
	return nil
}

// emitCreateJSON emits JSON output for festival creation.
func emitCreateJSON(cfg *createConfig, res *createResult, gatesDir string, remainingMarkers int) error {
	warnings := []string{}
	if remainingMarkers > 0 {
		warnings = append(warnings,
			fmt.Sprintf("CRITICAL: %d unfilled markers - festival cannot be executed until resolved", remainingMarkers),
			"Run 'fest validate' to see which files need editing",
			"Run 'fest wizard fill FESTIVAL_GOAL.md' to fill markers interactively",
		)
	}
	warnings = append(warnings, "Next: Create phases with 'fest create phase --name PHASE_NAME'")

	festivalMap := map[string]string{
		"name":      cfg.opts.Name,
		"slug":      cfg.slug,
		"dest":      cfg.destCategory,
		"id":        cfg.festivalID,
		"directory": cfg.dirName,
	}
	if cfg.festivalTypeName != "" {
		festivalMap["type"] = cfg.festivalTypeName
	}

	return emitCreateFestivalJSON(cfg.opts, createFestivalResult{
		OK:                true,
		Action:            "create_festival",
		Festival:          festivalMap,
		Created:           res.created,
		AutoPhasesCreated: res.autoPhasesCreated,
		GatesDirectory:    gatesDir,
		FestYAML:          res.festConfigPath,
		GateTemplates:     res.copiedGates,
		ProjectPath:       res.projectPath,
		ProjectLinked:     res.projectLinked,
		MarkersFilled:     res.markersFilled,
		MarkersTotal:      res.markersTotal,
		Validation:        res.validationResult,
		Warnings:          warnings,
	})
}

func parseTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := []string{}
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func emitCreateFestivalError(opts *CreateFestivalOptions, err error) error {
	if opts.JSONOutput {
		_ = emitCreateFestivalJSON(opts, createFestivalResult{
			OK:     false,
			Action: "create_festival",
			Errors: []map[string]any{{
				"code":    "error",
				"message": err.Error(),
			}},
		})
		return nil
	}
	return err
}

func emitCreateFestivalJSON(opts *CreateFestivalOptions, res createFestivalResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "festival"
	}
	return slug
}

// mapPhaseSpecType maps festival type phase spec type to create phase command type.
// Phase types in festival_types.yaml must match template directory names exactly
// (planning, implementation, research, review, ingest, non_coding_action).
func mapPhaseSpecType(specType string) string {
	switch strings.ToLower(specType) {
	case "planning", "implementation", "research", "review", "ingest", "non_coding_action":
		return strings.ToLower(specType)
	default:
		return "planning"
	}
}
