package festival

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/campledger"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	festcontract "github.com/Obedience-Corp/fest/internal/contract"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/registry"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/validator"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/Obedience-Corp/obey-shared/contract"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// CreateFestivalOptions holds options for the create festival command.
type CreateFestivalOptions struct {
	Name        string
	Goal        string
	Tags        string
	Project     string // Project directory path
	Type        string // Festival type (standard, implementation, research, ritual)
	VarsFile    string
	Markers     string // Inline JSON with hint→value mappings
	MarkersFile string // JSON file path with hint→value mappings
	Seed        string // Inline seed/input-spec content for the ingest phase
	SeedFile    string // File whose contents seed the ingest phase
	SkipMarkers bool   // Skip marker processing
	DryRun      bool   // Preview the scaffold tree and its template markers without writing anything
	JSONOutput  bool
	Dest        string // "planning" (default) or "ritual" (for ritual-type festivals)
	AgentMode   bool   // Strict mode for AI agents
}

type createFestivalResult struct {
	OK                bool               `json:"ok"`
	Action            string             `json:"action"`
	Festival          map[string]string  `json:"festival,omitempty"`
	CreatedPath       string             `json:"created_path,omitempty"`
	Created           []string           `json:"created,omitempty"`
	AutoPhasesCreated []string           `json:"auto_phases_created,omitempty"`
	GatesDirectory    string             `json:"gates_directory,omitempty"`
	FestYAML          string             `json:"fest_yaml,omitempty"`
	GateTemplates     []string           `json:"gate_templates,omitempty"`
	SeedPath          string             `json:"seed_path,omitempty"`
	ProjectPath       string             `json:"project_path,omitempty"`
	ProjectLinked     bool               `json:"project_linked,omitempty"`
	Markers           []map[string]any   `json:"markers,omitempty"`
	MarkersFilled     int                `json:"markers_filled,omitempty"`
	MarkersTotal      int                `json:"markers_total,omitempty"`
	MarkersUnfilled   int                `json:"markers_unfilled,omitempty"`
	UnfilledMarkers   []markerFileResult `json:"unfilled_markers,omitempty"`
	Validation        *ValidationSummary `json:"validation,omitempty"`
	RolledBack        *bool              `json:"rolled_back,omitempty"`
	RollbackError     string             `json:"rollback_error,omitempty"`
	Errors            []map[string]any   `json:"errors,omitempty"`
	Warnings          []string           `json:"warnings,omitempty"`
	Extra             map[string]any     `json:"extra,omitempty"`
}

type markerFileResult struct {
	File        string                 `json:"file"`
	Count       int                    `json:"count"`
	MarkerTypes []string               `json:"marker_types,omitempty"`
	Level       string                 `json:"level,omitempty"`
	Markers     []markerOccurrenceInfo `json:"markers,omitempty"`
}

type markerOccurrenceInfo struct {
	Line       int    `json:"line"`
	MarkerType string `json:"marker_type"`
	Content    string `json:"content"`
}

type festivalCoreTemplate struct {
	Template string
	Output   string
}

var festivalCoreTemplates = []festivalCoreTemplate{
	{Template: "festival/OVERVIEW.md", Output: "FESTIVAL_OVERVIEW.md"},
	{Template: "festival/GOAL.md", Output: "FESTIVAL_GOAL.md"},
	{Template: "festival/RULES.md", Output: "FESTIVAL_RULES.md"},
	{Template: "festival/TODO.md", Output: "TODO.md"},
}

var festivalGatePhaseTypes = []string{"planning", "implementation", "research", "review", "non_coding_action"}

// createConfig holds all resolved configuration for festival creation.
// It is populated by resolveCreateConfig() and passed to subsequent pipeline stages.
type createConfig struct {
	opts             *CreateFestivalOptions
	display          *ui.UI
	campaignRoot     string
	festivalsRoot    string
	tmplRoot         string
	slug             string
	festivalID       string
	destCategory     string
	dirName          string
	destDir          string
	vars             map[string]any
	agentCfg         *config.AgentConfig
	skipMarkers      bool
	tmplCtx          *tpl.Context
	festivalType     *types.FestivalType
	festivalTypeName string
	seedContent      string
	seedRequested    bool
}

// NewCreateFestivalCommand adds 'create festival'
func NewCreateFestivalCommand() *cobra.Command {
	opts := &CreateFestivalOptions{}
	cmd := &cobra.Command{
		Use:   "festival",
		Short: "Create a new festival scaffold under festivals/planning",
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
	cmd.Flags().StringVar(&opts.Type, "type", "", "Festival type (standard, implementation, research, ritual); see 'fest types list --level festival'")
	cmd.Flags().StringVar(&opts.VarsFile, "vars-file", "", "JSON file with variables")
	cmd.Flags().StringVar(&opts.Markers, "markers", "", "JSON string with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.MarkersFile, "markers-file", "", "JSON file with REPLACE marker hint→value mappings")
	cmd.Flags().StringVar(&opts.Seed, "seed", "", "Inline seed content written to the ingest phase input_specs/ (requires a type with an ingest phase)")
	cmd.Flags().StringVar(&opts.SeedFile, "seed-file", "", "File whose contents seed the ingest phase input_specs/ (mutually exclusive with --seed)")
	cmd.Flags().BoolVar(&opts.SkipMarkers, "skip-markers", false, "Skip REPLACE marker processing")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview the festival file tree and its template markers without writing anything")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().StringVar(&opts.Dest, "dest", "planning", "Destination under festivals/: planning or ritual (use 'fest promote' to advance to active)")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict mode: process markers, auto-validate, rollback on blocking errors, JSON output")
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
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "getting working directory")
	}

	festivalsRoot, err := workspace.FindFestivals(cwd)
	if err != nil {
		return nil, err
	}
	if festivalsRoot == "" {
		return nil, errors.NotFound("festivals directory").
			WithHint("Check the path and try again, or run from a different directory")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	agentCfg := LoadEffectiveAgentConfig(festivalsRoot, "")
	skipMarkers := config.EffectiveSkipMarkers(agentCfg, opts.AgentMode, opts.SkipMarkers)
	tmplRoot := filepath.Join(festivalsRoot, ".festival", "templates")

	vars := map[string]any{}
	if strings.TrimSpace(opts.VarsFile) != "" {
		v, err := loadVarsFile(opts.VarsFile)
		if err != nil {
			return nil, errors.Wrap(err, "reading vars-file").WithField("path", opts.VarsFile)
		}
		vars = v
	}

	slug := Slugify(opts.Name)
	destCategory := strings.ToLower(strings.TrimSpace(opts.Dest))
	if opts.Type == "ritual" {
		destCategory = "ritual"
	}
	if destCategory == "active" {
		return nil, errors.Validation("cannot create festival directly in active/").
			WithHint("Festivals must be created in planning/ first. Use 'fest promote' to advance through the lifecycle: planning → ready → active")
	}
	if destCategory != "planning" && destCategory != "ritual" {
		return nil, errors.Validation(fmt.Sprintf("invalid destination: %q", destCategory)).
			WithHint("Valid destinations: planning (default), ritual (for ritual-type festivals)")
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
		campaignRoot:     campaignRoot,
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
	linkSkipReason    string
	markersFilled     int
	markersTotal      int
	validationResult  *ValidationSummary
	registered        bool
	seedPath          string
}

// RunCreateFestival executes the create festival command logic.
func RunCreateFestival(ctx context.Context, opts *CreateFestivalOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreateFestival")
	}

	if opts.Seed != "" && opts.SeedFile != "" {
		return emitCreateFestivalError(opts, errors.Validation(
			"--seed and --seed-file are mutually exclusive"))
	}

	cfg, err := resolveCreateConfig(ctx, opts)
	if err != nil {
		return emitCreateFestivalError(opts, err)
	}

	if err := resolveAndValidateSeed(cfg); err != nil {
		return emitCreateFestivalError(opts, err)
	}

	if opts.DryRun {
		return previewCreateFestival(ctx, cfg)
	}

	if err := scaffoldFestivalDirectory(ctx, cfg); err != nil {
		return emitCreateFestivalError(opts, err)
	}

	created, copiedGates, err := renderFestivalTemplates(ctx, cfg)
	if err != nil {
		return emitCreateFestivalCreatedError(ctx, cfg, nil, err)
	}

	res, err := writeFestYaml(ctx, cfg)
	if err != nil {
		return emitCreateFestivalCreatedError(ctx, cfg, nil, err)
	}
	created = append(created, res.festConfigPath)

	created, res.autoPhasesCreated = autoScaffoldPhases(ctx, cfg, created)

	res.created = created
	res.copiedGates = copiedGates

	// Seed before the initial-size snapshot so seeded content is part of the
	// creation baseline, not later counted as post-create growth. The seed file
	// is registered in res.created after marker processing so user content is
	// never marker-substituted.
	if err := writeSeedFile(cfg, res); err != nil {
		return emitCreateFestivalCreatedError(ctx, cfg, res, err)
	}

	recordInitialSize(ctx, cfg, res.festConfig)

	if err := processAllMarkers(ctx, cfg, res); err != nil {
		return emitCreateFestivalCreatedError(ctx, cfg, res, err)
	}

	if res.seedPath != "" {
		res.created = append(res.created, res.seedPath)
	}

	if err := validateIfConfigured(ctx, cfg, res); err != nil {
		if opts.AgentMode && res.validationResult != nil && !res.validationResult.OK && config.ShouldBlockOnErrors(cfg.agentCfg, opts.AgentMode) {
			return emitCreateFestivalValidationFailure(ctx, cfg, res, err)
		}
		return emitCreateFestivalCreatedError(ctx, cfg, res, err)
	}

	registerFestival(ctx, cfg, res)

	// Ensure fest contract entries are present in .campaign/watchers.yaml.
	// This is idempotent: if fest init already wrote them, WriteEntries
	// will replace them with the same values. This handles the case where
	// fest init ran before camp init (so .campaign/ didn't exist yet),
	// or where the contract file was deleted and needs regeneration.
	writeContractEntries(cfg.campaignRoot)

	// Campaign ledger: festival created (high-intent). Dry-run already returned.
	if cfg.destDir != "" {
		emit := campledger.NewFromFestival(ctx, cfg.destDir, campledger.WarnToStderr())
		emit.Emit(ctx, ledgerkit.KindCreated, campledger.FestivalScope(cfg.destDir, ""),
			campledger.WithPayload(map[string]any{
				"status": "created",
				"type":   "festival",
			}),
		)
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
	var missingCoreTemplates []string

	for _, coreTemplate := range festivalCoreTemplates {
		outPath, err := renderCoreFile(ctx, cfg, mgr, coreTemplate.Template, coreTemplate.Output)
		if err != nil {
			return nil, nil, err
		}
		if outPath != "" {
			created = append(created, outPath)
		} else {
			missingCoreTemplates = append(missingCoreTemplates, coreTemplate.Template)
		}
	}

	if len(missingCoreTemplates) > 0 {
		return nil, nil, errors.Template(
			fmt.Sprintf("missing required core festival templates: %s", strings.Join(missingCoreTemplates, ", "))).
			WithField("templates", strings.Join(missingCoreTemplates, ", ")).
			WithField("template_root", cfg.tmplRoot).
			WithHint("Copy .festival/templates/festival/ from a working campaign, or run 'fest init' to seed the template directory.")
	}

	copiedGates, gateCreated, err := copyGateTemplates(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	created = append(created, gateCreated...)

	return created, copiedGates, nil
}

// renderCoreFile renders a single core festival file from a template and writes
// it into the festival directory.
// Returns the output path, or empty string if the template was not found.
// The caller is responsible for treating a missing core template as an error.
func renderCoreFile(ctx context.Context, cfg *createConfig, mgr *tpl.Manager, templateName, outName string) (string, error) {
	content, found, err := buildCoreFileContent(ctx, cfg, mgr, templateName, outName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}

	outPath := filepath.Join(cfg.destDir, outName)
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", errors.IO("writing file", err).WithField("path", outPath)
	}
	return outPath, nil
}

// buildCoreFileContent produces the exact bytes renderCoreFile would write, with
// no filesystem writes. The dry-run preview relies on this to report the markers
// a real create would leave behind.
// Returns found=false when the template is absent.
func buildCoreFileContent(ctx context.Context, cfg *createConfig, mgr *tpl.Manager, templateName, outName string) (string, bool, error) {
	tpath := filepath.Join(cfg.tmplRoot, templateName)
	info, err := os.Stat(tpath)
	if err != nil || info.IsDir() {
		return "", false, nil
	}

	loader := tpl.NewLoader()
	t, loadErr := loader.Load(ctx, tpath)
	if loadErr != nil {
		return "", false, errors.Wrap(loadErr, "loading template").WithField("template", templateName)
	}

	content := renderOrCopyTemplate(mgr, t, cfg.tmplCtx)

	if outName == "FESTIVAL_GOAL.md" && !strings.HasPrefix(strings.TrimSpace(content), "---") {
		fm := frontmatter.NewFrontmatter(frontmatter.TypeFestival, cfg.festivalID, cfg.opts.Name)
		fm.Status = frontmatter.StatusPlanning
		contentWithFM, fmErr := frontmatter.InjectString(content, fm)
		if fmErr != nil {
			return "", false, errors.Wrap(fmErr, "injecting frontmatter")
		}
		content = contentWithFM
	}

	return applyFestivalMarkerRendering(cfg, content), true, nil
}

// applyFestivalMarkerRendering fills the markers a festival-level template
// context can resolve, leaving the rest for --markers input or a human.
func applyFestivalMarkerRendering(cfg *createConfig, content string) string {
	if cfg.skipMarkers {
		return content
	}
	renderer := tpl.NewRenderer()
	rendered, err := renderer.RenderWithMarkerReplacement(content, cfg.tmplCtx, nil)
	if err != nil {
		return content
	}
	return rendered
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
	var copiedGates, created []string
	for _, phaseType := range festivalGatePhaseTypes {
		srcGatesDir := filepath.Join(srcPhasesDir, phaseType, "gates")
		if _, err := os.Stat(srcGatesDir); os.IsNotExist(err) {
			if !cfg.opts.JSONOutput {
				cfg.display.Info("No gate templates for phase type '%s' (skipped)", phaseType)
			}
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

			processed := applyFestivalMarkerRendering(cfg, string(content))

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
		resolveProjectLink(ctx, cfg, festConfig, res)
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
	if err := config.SaveFestivalConfig(cfg.destDir, cfg.campaignRoot, festConfig); err != nil {
		return nil, errors.Wrap(err, "writing fest.yaml").WithField("path", res.festConfigPath)
	}

	return res, nil
}

// resolveProjectLink resolves and optionally links a project path for the festival.
func resolveProjectLink(ctx context.Context, cfg *createConfig, festConfig *config.FestivalConfig, res *createResult) {
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

	updateErr := navigation.Update(ctx, func(nav *navigation.Navigation) error {
		// A project holds one festival link at a time. Creating a festival must
		// never silently evict another festival's link; skip and tell the user.
		if existing, _, hasConflict := nav.ProjectConflict(cfg.dirName, resolved); hasConflict {
			res.linkSkipReason = "already linked to " + existing
			if !cfg.opts.JSONOutput {
				cfg.display.Warning("Project is already linked to festival %s; link not changed", existing)
				cfg.display.Info("Run 'fest link --force %s' from this festival to take over the link", resolved)
			}
			return nil
		}
		nav.SetLinkWithPath(cfg.dirName, resolved, cfg.destDir)
		res.projectLinked = true
		return nil
	})
	if updateErr != nil {
		res.projectLinked = false
		return
	}
	if res.projectLinked && !cfg.opts.JSONOutput {
		cfg.display.Success("Linked to project: %s", resolved)
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
		phaseOpts := autoPhaseOptions(cfg, i, phaseSpec)

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

// autoPhaseOptions builds the create-phase options an auto-scaffolded phase runs
// with. The dry-run preview builds its phase template context from the same
// options so the markers it reports match the ones a real create would leave.
func autoPhaseOptions(cfg *createConfig, index int, phaseSpec types.PhaseSpec) *CreatePhaseOptions {
	phaseContext := ""
	if cfg.seedRequested && phaseSpec.Type == "ingest" {
		phaseContext = "Seeded input is available in input_specs/seed.md and should be transformed into structured output specs for planning."
	}
	return &CreatePhaseOptions{
		After:       index,
		Name:        phaseSpec.Name,
		PhaseType:   mapPhaseSpecType(phaseSpec.Type),
		Description: phaseSpec.Description,
		Context:     phaseContext,
		Path:        cfg.destDir,
		SkipMarkers: cfg.skipMarkers,
		JSONOutput:  false,
		AgentMode:   false,
		Quiet:       true,
	}
}

// resolveAndValidateSeed resolves --seed/--seed-file content and verifies the
// festival type has an ingest phase to seed BEFORE any scaffolding happens, so a
// type with no ingest phase fails fast instead of leaving a half-created
// festival. input_specs/ is an ingest-phase concept; types without one are
// refused rather than silently misplaced.
func resolveAndValidateSeed(cfg *createConfig) error {
	content, ok, err := resolveSeedContent(cfg.opts)
	if err != nil || !ok {
		return err
	}

	if _, found := ingestAutoPhaseID(cfg.festivalType); !found {
		typeName := cfg.festivalTypeName
		if typeName == "" {
			typeName = "(none)"
		}
		return errors.Validation("festival type has no ingest phase to seed").
			WithField("type", typeName).
			WithHint("use --markers, or choose a festival type with an ingest phase")
	}

	cfg.seedContent = content
	cfg.seedRequested = true
	return nil
}

// writeSeedFile writes the pre-resolved seed content into the festival's ingest
// phase input_specs/. The ingest phase is guaranteed to exist by
// resolveAndValidateSeed.
func writeSeedFile(cfg *createConfig, res *createResult) error {
	if !cfg.seedRequested {
		return nil
	}

	ingestPhaseID, _ := ingestAutoPhaseID(cfg.festivalType)
	// autoScaffoldPhases swallows per-phase failures, so confirm the ingest
	// phase was actually created before seeding into it; otherwise seeding
	// would write under a missing phase and falsely report success.
	if !slices.Contains(res.autoPhasesCreated, ingestPhaseID) {
		return errors.New("ingest phase was not scaffolded; cannot seed").
			WithField("phase", ingestPhaseID)
	}

	inputSpecsDir := filepath.Join(cfg.destDir, ingestPhaseID, "input_specs")
	if err := os.MkdirAll(inputSpecsDir, 0755); err != nil {
		return errors.IO("creating input_specs directory", err).WithField("path", inputSpecsDir)
	}

	seedPath := filepath.Join(inputSpecsDir, "seed.md")
	if err := os.WriteFile(seedPath, []byte(cfg.seedContent), 0644); err != nil {
		return errors.IO("writing seed file", err).WithField("path", seedPath)
	}

	res.seedPath = seedPath
	return nil
}

// resolveSeedContent returns the seed content from --seed or --seed-file.
// The bool is false when no seed flag was provided.
func resolveSeedContent(opts *CreateFestivalOptions) (string, bool, error) {
	if opts.Seed != "" {
		return opts.Seed, true, nil
	}
	if opts.SeedFile != "" {
		data, err := os.ReadFile(opts.SeedFile)
		if err != nil {
			return "", false, errors.IO("reading seed file", err).WithField("path", opts.SeedFile)
		}
		return string(data), true, nil
	}
	return "", false, nil
}

// ingestAutoPhaseID returns the phase ID of the festival type's auto-scaffolded
// ingest phase, matching the IDs autoScaffoldPhases produces.
func ingestAutoPhaseID(festivalType *types.FestivalType) (string, bool) {
	if festivalType == nil {
		return "", false
	}
	for i, phaseSpec := range festivalType.GetAutoPhases() {
		if phaseSpec.Type == "ingest" {
			return tpl.FormatPhaseID(i+1, phaseSpec.Name), true
		}
	}
	return "", false
}

// recordInitialSize records the initial content size for token delta tracking.
func recordInitialSize(ctx context.Context, cfg *createConfig, festConfig *config.FestivalConfig) {
	initialSize, sizeErr := progress.ComputeDirectorySize(ctx, cfg.destDir)
	if sizeErr == nil && initialSize > 0 {
		festConfig.Metadata.InitialSizeBytes = initialSize
		_ = config.SaveFestivalConfig(cfg.destDir, cfg.campaignRoot, festConfig)
	}
}

// registerFestival records the festival in the ID registry with event logging.
// On success it marks res.registered so rollback only cleans up the registry
// when a write actually happened.
func registerFestival(ctx context.Context, cfg *createConfig, res *createResult) {
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
	if err := reg.AddWithEvent(ctx, regEntry); err == nil && res != nil {
		res.registered = true
	}
}

// processAllMarkers processes REPLACE markers in all created files.
func processAllMarkers(ctx context.Context, cfg *createConfig, res *createResult) error {
	for _, filePath := range res.created {
		markerResult, err := ProcessMarkers(ctx, MarkerOptions{
			FilePath:    filePath,
			Markers:     cfg.opts.Markers,
			MarkersFile: cfg.opts.MarkersFile,
			SkipMarkers: cfg.skipMarkers,
			DryRun:      false,
			JSONOutput:  cfg.opts.JSONOutput,
		})
		if err != nil {
			return errors.Wrap(err, "processing markers")
		}
		if markerResult != nil {
			res.markersFilled += markerResult.Filled
			res.markersTotal += markerResult.Total
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

	// Get campaign root for relative path display
	campaignRoot := cfg.campaignRoot
	displayPath := func(p string) string {
		if campaignRoot != "" {
			return pathutil.DisplayPath(p, campaignRoot)
		}
		return p
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
		cfg.display.Info("  • %s", displayPath(p))
	}

	if len(res.copiedGates) > 0 {
		cfg.display.Success("Created gates/ directory with %d templates organized by phase type", len(res.copiedGates))
		cfg.display.Info("  Quality gates configured in fest.yaml")
	}

	if res.projectPath != "" {
		if res.projectLinked {
			cfg.display.Success("Project path: %s (linked)", displayPath(res.projectPath))
		} else if res.linkSkipReason != "" {
			cfg.display.Info("Project path: %s (not linked - %s)", displayPath(res.projectPath), res.linkSkipReason)
		} else {
			cfg.display.Info("Project path: %s (not linked - path doesn't exist yet)", displayPath(res.projectPath))
		}
	}

	fmt.Println()
	fmt.Println(ui.H2("Next (you)"))
	// cd instruction stays absolute for shell usage
	fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("1. cd %s", cfg.destDir)))
	fmt.Printf("  %s\n", ui.Info("2. Hand this to your agent: fest next"))
	fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("3. Or review structure: fest show %s", cfg.slug)))
	if cfg.destCategory == "planning" {
		fmt.Printf("  %s\n", ui.Info("4. When the plan is solid: fest promote  (planning → ready → active)"))
	}

	fmt.Println()
	fmt.Println(ui.H2("Agent work (not yours)"))
	if remainingMarkers > 0 {
		fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("· Resolve %d REPLACE markers (fest wizard fill / agent --markers)", remainingMarkers)))
	}
	if len(res.autoPhasesCreated) > 0 {
		fmt.Printf("  %s\n", ui.Info(fmt.Sprintf("· Drive auto phases already scaffolded: %s", strings.Join(res.autoPhasesCreated, ", "))))
	} else {
		fmt.Printf("  %s\n", ui.Info("· Add phases as needed: fest create phase --name PHASE_NAME"))
	}
	fmt.Printf("  %s\n", ui.Info("· fest validate --fix if structure drifts"))
	return nil
}

// emitCreateJSON emits JSON output for festival creation.
func emitCreateJSON(cfg *createConfig, res *createResult, gatesDir string, remainingMarkers int) error {
	displayPath := createDisplayPathFunc(cfg)

	warnings := []string{}
	if remainingMarkers > 0 {
		warnings = append(warnings,
			fmt.Sprintf("CRITICAL: %d unfilled markers - festival cannot be executed until resolved", remainingMarkers),
			"Run 'fest validate' to see which files need editing",
			"Run 'fest wizard fill FESTIVAL_GOAL.md' to fill markers interactively",
		)
	}
	if len(res.autoPhasesCreated) > 0 {
		warnings = append(warnings, "Next (human): hand to agent with fest next; auto phases already scaffolded")
	} else {
		warnings = append(warnings, "Next: Create phases with 'fest create phase --name PHASE_NAME'")
	}

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

	// Convert created file paths to relative
	relCreated := make([]string, len(res.created))
	for i, p := range res.created {
		relCreated[i] = displayPath(p)
	}

	relGates := make([]string, len(res.copiedGates))
	for i, p := range res.copiedGates {
		relGates[i] = displayPath(p)
	}

	return emitCreateFestivalJSON(cfg.opts, createFestivalResult{
		OK:                true,
		Action:            "create_festival",
		Festival:          festivalMap,
		CreatedPath:       displayPath(cfg.destDir),
		Created:           relCreated,
		AutoPhasesCreated: res.autoPhasesCreated,
		GatesDirectory:    displayPath(gatesDir),
		FestYAML:          displayPath(res.festConfigPath),
		GateTemplates:     relGates,
		SeedPath:          displayPath(res.seedPath),
		ProjectPath:       displayPath(res.projectPath),
		ProjectLinked:     res.projectLinked,
		MarkersFilled:     res.markersFilled,
		MarkersTotal:      res.markersTotal,
		Validation:        res.validationResult,
		Warnings:          warnings,
	})
}

func emitCreateFestivalCreatedError(ctx context.Context, cfg *createConfig, res *createResult, err error) error {
	if cfg != nil && cfg.opts != nil && cfg.opts.AgentMode {
		return emitCreateFestivalFailure(ctx, cfg, res, err, nil)
	}
	if cfg != nil && cfg.opts != nil {
		return emitCreateFestivalError(cfg.opts, err)
	}
	return err
}

func emitCreateFestivalValidationFailure(ctx context.Context, cfg *createConfig, res *createResult, err error) error {
	unfilledMarkers, scanErr := validator.ScanTemplateMarkers(cfg.destDir)
	if scanErr != nil && err == nil {
		err = errors.Wrap(scanErr, "scanning unfilled markers")
	}
	return emitCreateFestivalFailure(ctx, cfg, res, err, unfilledMarkers)
}

func emitCreateFestivalFailure(ctx context.Context, cfg *createConfig, res *createResult, err error, unfilledMarkers []validator.MarkerFileResult) error {
	displayPath := createDisplayPathFunc(cfg)
	relCreated := createdDisplayPaths(cfg, res)
	relGates := gateDisplayPaths(cfg, res)
	markerDetails := markerFileResultsFromValidator(unfilledMarkers)
	markersUnfilled := countMarkerFileResults(markerDetails)
	if markersUnfilled == 0 && res != nil {
		markersUnfilled = res.markersTotal - res.markersFilled
	}

	rolledBack := true
	rollbackErr := rollbackCreatedFestival(ctx, cfg, res)
	if rollbackErr != nil {
		rolledBack = false
	}
	message := "festival creation failed"
	if err != nil {
		message = err.Error()
	}

	result := createFestivalResult{
		OK:                false,
		Action:            "create_festival",
		Festival:          festivalMapForConfig(cfg),
		CreatedPath:       displayPath(cfg.destDir),
		Created:           relCreated,
		AutoPhasesCreated: nil,
		GatesDirectory:    displayPath(filepath.Join(cfg.destDir, "gates")),
		FestYAML:          displayPath(festConfigPath(res)),
		GateTemplates:     relGates,
		ProjectPath:       displayPath(projectPath(res)),
		ProjectLinked:     projectLinked(res),
		MarkersFilled:     markersFilled(res),
		MarkersTotal:      markersTotal(res),
		MarkersUnfilled:   markersUnfilled,
		UnfilledMarkers:   markerDetails,
		Validation:        validationSummary(res),
		RolledBack:        &rolledBack,
		Errors: []map[string]any{{
			"code":    "error",
			"message": message,
		}},
	}
	if rollbackErr != nil {
		result.RollbackError = rollbackErr.Error()
	}

	_ = emitCreateFestivalJSON(cfg.opts, result)
	return errors.ErrAlreadyPrinted
}

func rollbackCreatedFestival(ctx context.Context, cfg *createConfig, res *createResult) error {
	var failures []string

	if res != nil && res.projectLinked {
		if err := navigation.Update(ctx, func(nav *navigation.Navigation) error {
			nav.RemoveLink(cfg.dirName)
			return nil
		}); err != nil {
			failures = append(failures, fmt.Sprintf("removing navigation link: %v", err))
		}
	}

	if res != nil && res.registered {
		regPath := registry.GetEventsPath(cfg.festivalsRoot)
		reg, err := registry.Load(ctx, regPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("loading registry for rollback: %v", err))
		} else if reg.Exists(ctx, cfg.festivalID) {
			if err := reg.DeleteWithEvent(ctx, cfg.festivalID); err != nil {
				failures = append(failures, fmt.Sprintf("deleting registry entry: %v", err))
			}
		}
	}

	if err := os.RemoveAll(cfg.destDir); err != nil {
		failures = append(failures, fmt.Sprintf("removing created festival directory: %v", err))
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func createDisplayPathFunc(cfg *createConfig) func(string) string {
	return func(p string) string {
		if p == "" {
			return ""
		}
		if cfg != nil && cfg.campaignRoot != "" {
			return pathutil.DisplayPath(p, cfg.campaignRoot)
		}
		return p
	}
}

func festivalMapForConfig(cfg *createConfig) map[string]string {
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
	return festivalMap
}

func createdDisplayPaths(cfg *createConfig, res *createResult) []string {
	if res == nil || len(res.created) == 0 {
		return nil
	}
	displayPath := createDisplayPathFunc(cfg)
	relCreated := make([]string, len(res.created))
	for i, p := range res.created {
		relCreated[i] = displayPath(p)
	}
	return relCreated
}

func gateDisplayPaths(cfg *createConfig, res *createResult) []string {
	if res == nil || len(res.copiedGates) == 0 {
		return nil
	}
	displayPath := createDisplayPathFunc(cfg)
	relGates := make([]string, len(res.copiedGates))
	for i, p := range res.copiedGates {
		relGates[i] = displayPath(p)
	}
	return relGates
}

func markerFileResultsFromValidator(results []validator.MarkerFileResult) []markerFileResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]markerFileResult, 0, len(results))
	for _, result := range results {
		markers := make([]markerOccurrenceInfo, 0, len(result.Markers))
		for _, marker := range result.Markers {
			markers = append(markers, markerOccurrenceInfo{
				Line:       marker.Line,
				MarkerType: marker.MarkerType,
				Content:    marker.Content,
			})
		}
		markerTypes := append([]string(nil), result.MarkerTypes...)
		sort.Strings(markerTypes)
		out = append(out, markerFileResult{
			File:        result.RelPath,
			Count:       result.MarkerCount,
			MarkerTypes: markerTypes,
			Level:       result.Level,
			Markers:     markers,
		})
	}
	return out
}

func countMarkerFileResults(results []markerFileResult) int {
	total := 0
	for _, result := range results {
		total += result.Count
	}
	return total
}

func festConfigPath(res *createResult) string {
	if res == nil {
		return ""
	}
	return res.festConfigPath
}

func projectPath(res *createResult) string {
	if res == nil {
		return ""
	}
	return res.projectPath
}

func projectLinked(res *createResult) bool {
	return res != nil && res.projectLinked
}

func markersFilled(res *createResult) int {
	if res == nil {
		return 0
	}
	return res.markersFilled
}

func markersTotal(res *createResult) int {
	if res == nil {
		return 0
	}
	return res.markersTotal
}

func validationSummary(res *createResult) *ValidationSummary {
	if res == nil {
		return nil
	}
	return res.validationResult
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
		if opts.AgentMode {
			return errors.ErrAlreadyPrinted
		}
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

// writeContractEntries writes fest's entries to the campaign contract file.
// Skips gracefully if campaignRoot is empty (standalone fest workspace).
func writeContractEntries(campaignRoot string) {
	if campaignRoot == "" {
		return
	}

	contractPath := contract.ContractPath(campaignRoot)
	if writeErr := contract.WriteEntries(contractPath, contract.OwnerFest, festcontract.FestEntries()); writeErr != nil {
		if shared.IsVerbose() {
			fmt.Fprintf(os.Stderr, "Warning: could not write contract entries: %v\n", writeErr)
		}
	}
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
