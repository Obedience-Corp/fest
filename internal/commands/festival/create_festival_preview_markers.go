package festival

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/markers"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// festivalPreviewMarker is one template marker a real create would leave
// unfilled in a planned file.
type festivalPreviewMarker struct {
	File string `json:"file"`
	Hint string `json:"hint"`
	Line int    `json:"line"`

	// rel is the festival-relative form of File, used for human output only.
	rel string
}

// festivalMarkerPreview reports the markers a real create would produce, derived
// from templates rendered in memory.
type festivalMarkerPreview struct {
	unfilled []festivalPreviewMarker
	total    int
	filled   int
}

// previewMarkerSource is one planned file with the exact content a real create
// would write to it.
type previewMarkerSource struct {
	rel     string
	content string
}

// buildFestivalMarkerPreview renders every marker-bearing planned file in memory
// and reports the markers a real create would leave behind. --markers and
// --markers-file are applied so an agent can confirm a marker plan is complete
// before creating anything.
func buildFestivalMarkerPreview(ctx context.Context, cfg *createConfig) (*festivalMarkerPreview, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	input, err := resolvePreviewMarkerInput(ctx, cfg)
	if err != nil {
		return nil, err
	}

	sources, err := collectPreviewMarkerSources(ctx, cfg)
	if err != nil {
		return nil, err
	}

	displayPath := createDisplayPathFunc(cfg)
	preview := &festivalMarkerPreview{unfilled: []festivalPreviewMarker{}}
	for _, source := range sources {
		found := markers.Parse(source.content)
		if len(found) == 0 {
			continue
		}
		preview.total += len(found)
		for _, value := range markers.ApplyInput(found, input) {
			if value.Value != value.Marker.FullMatch {
				preview.filled++
				continue
			}
			preview.unfilled = append(preview.unfilled, festivalPreviewMarker{
				File: displayPath(filepath.Join(cfg.destDir, source.rel)),
				Hint: value.Marker.Hint,
				Line: value.Marker.LineNumber,
				rel:  filepath.ToSlash(source.rel),
			})
		}
	}

	return preview, nil
}

// resolvePreviewMarkerInput loads the hint→value mappings a real create would
// apply. --skip-markers reports every marker unfilled, matching the count-only
// behavior of the real create path.
func resolvePreviewMarkerInput(ctx context.Context, cfg *createConfig) (map[string]string, error) {
	if cfg.skipMarkers {
		return nil, nil
	}
	if cfg.opts.Markers != "" {
		input, err := markers.ParseJSON(cfg.opts.Markers)
		if err != nil {
			return nil, errors.Wrap(err, "parsing --markers JSON")
		}
		return input, nil
	}
	if cfg.opts.MarkersFile != "" {
		return markers.ReadJSONFile(ctx, cfg.opts.MarkersFile)
	}
	return nil, nil
}

// collectPreviewMarkerSources renders the planned files that a real create runs
// marker processing over, in creation order: core templates, gate templates,
// then the PHASE_GOAL.md of each auto-scaffolded phase. fest.yaml is generated
// from resolved config rather than a template, so it carries no markers.
func collectPreviewMarkerSources(ctx context.Context, cfg *createConfig) ([]previewMarkerSource, error) {
	var sources []previewMarkerSource

	mgr := tpl.NewManager()
	for _, coreTemplate := range festivalCoreTemplates {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
		content, found, err := buildCoreFileContent(ctx, cfg, mgr, coreTemplate.Template, coreTemplate.Output)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		sources = append(sources, previewMarkerSource{rel: coreTemplate.Output, content: content})
	}

	gateSources, err := collectPreviewGateMarkerSources(ctx, cfg)
	if err != nil {
		return nil, err
	}
	sources = append(sources, gateSources...)

	phaseSources, err := collectPreviewPhaseMarkerSources(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return append(sources, phaseSources...), nil
}

// collectPreviewGateMarkerSources renders the gate templates copyGateTemplates
// would copy, applying the same marker rendering the copy applies.
func collectPreviewGateMarkerSources(ctx context.Context, cfg *createConfig) ([]previewMarkerSource, error) {
	plan, err := collectGateTemplatePlan(ctx, cfg.tmplRoot)
	if err != nil {
		return nil, err
	}

	var sources []previewMarkerSource
	for _, group := range plan.groups {
		for _, file := range group.files {
			if err := ctx.Err(); err != nil {
				return nil, errors.Wrap(err, "context cancelled")
			}
			content, err := os.ReadFile(file.source)
			if err != nil {
				return nil, errors.IO("reading gate template", err).WithField("path", file.source)
			}
			sources = append(sources, previewMarkerSource{
				rel:     file.dest,
				content: applyFestivalMarkerRendering(cfg, string(content)),
			})
		}
	}
	return sources, nil
}

// collectPreviewPhaseMarkerSources renders the PHASE_GOAL.md of each auto phase
// through the same helpers RunCreatePhase uses, so the reported phase markers
// match the ones the real create would leave.
func collectPreviewPhaseMarkerSources(ctx context.Context, cfg *createConfig) ([]previewMarkerSource, error) {
	if cfg.festivalType == nil {
		return nil, nil
	}

	var sources []previewMarkerSource
	for i, phaseSpec := range cfg.festivalType.GetAutoPhases() {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}

		phaseCfg := previewPhaseConfig(cfg, i, phaseSpec)
		content, err := renderPhaseGoalContent(ctx, phaseCfg)
		if err != nil {
			return nil, err
		}
		content, err = buildPhaseGoalContent(phaseCfg, content)
		if err != nil {
			return nil, err
		}
		sources = append(sources, previewMarkerSource{
			rel:     filepath.Join(phaseCfg.phaseID, "PHASE_GOAL.md"),
			content: content,
		})
	}
	return sources, nil
}

// previewPhaseConfig rebuilds the phase config an auto-scaffolded phase would
// resolve to, without the directory writes resolvePhaseConfig performs. The
// template context mirrors buildPhaseTemplateContext against the fest.yaml the
// real create writes before scaffolding phases.
func previewPhaseConfig(cfg *createConfig, index int, phaseSpec types.PhaseSpec) *phaseConfig {
	opts := autoPhaseOptions(cfg, index, phaseSpec)
	phaseNumber := index + 1
	phaseID := tpl.FormatPhaseID(phaseNumber, opts.Name)

	tmplCtx := tpl.NewContext()
	tmplCtx.SetFestival(cfg.opts.Name, cfg.opts.Goal, nil)
	tmplCtx.SetFestivalID(cfg.festivalID)
	tmplCtx.SetPhase(phaseNumber, opts.Name, opts.PhaseType)
	if opts.Description != "" {
		tmplCtx.SetPhaseObjective(opts.Description)
	}
	if opts.Context != "" {
		tmplCtx.SetPhaseContext(opts.Context)
	}
	tmplCtx.ComputeStructureVariables()

	return &phaseConfig{
		opts:                 opts,
		display:              cfg.display,
		absPath:              cfg.destDir,
		festivalsRoot:        cfg.festivalsRoot,
		festivalPath:         cfg.destDir,
		agentCfg:             cfg.agentCfg,
		effectiveSkipMarkers: cfg.skipMarkers,
		tmplRoot:             cfg.tmplRoot,
		newNumber:            phaseNumber,
		phaseID:              phaseID,
		phaseDir:             filepath.Join(cfg.destDir, phaseID),
		vars:                 map[string]any{},
		tmplCtx:              tmplCtx,
	}
}

// printFestivalPreviewMarkers reports the markers a real create would leave, so
// a human can see what they will need to fill before creating anything.
func printFestivalPreviewMarkers(preview *festivalMarkerPreview) {
	if preview.total == 0 {
		return
	}

	fmt.Println()
	if len(preview.unfilled) == 0 {
		fmt.Println(ui.H2("Replace Markers in Template"))
		fmt.Printf("  %s\n", ui.Success(fmt.Sprintf("All %d markers would be filled by the supplied marker values.", preview.total)))
		return
	}

	fmt.Println(ui.H2("Replace Markers in Template"))
	for _, group := range groupPreviewMarkers(preview.unfilled) {
		fmt.Printf("  %s %s\n", ui.Value(group.file), ui.Info(fmt.Sprintf("(%d)", len(group.markers))))
		for i, marker := range group.markers {
			fmt.Printf("    %s %s\n",
				ui.Value(fmt.Sprintf("%d.", i+1)),
				ui.Warning(fmt.Sprintf("[line %d] %s", marker.Line, marker.Hint)))
		}
	}
	fmt.Println()
	fmt.Println(ui.Info(fmt.Sprintf("%d of %d markers would be filled.", preview.filled, preview.total)))
	fmt.Println(ui.Info("Use --markers '{\"hint\": \"value\", ...}' or --markers-file to fill the rest."))
}

// previewMarkerGroup collects the unfilled markers of one planned file.
type previewMarkerGroup struct {
	file    string
	markers []festivalPreviewMarker
}

// groupPreviewMarkers groups markers by file, preserving first-seen file order.
func groupPreviewMarkers(list []festivalPreviewMarker) []previewMarkerGroup {
	var groups []previewMarkerGroup
	index := make(map[string]int, len(list))
	for _, marker := range list {
		file := marker.rel
		if file == "" {
			file = marker.File
		}
		if at, ok := index[file]; ok {
			groups[at].markers = append(groups[at].markers, marker)
			continue
		}
		index[file] = len(groups)
		groups = append(groups, previewMarkerGroup{file: file, markers: []festivalPreviewMarker{marker}})
	}
	return groups
}
