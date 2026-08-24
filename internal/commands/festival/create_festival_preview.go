package festival

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
)

type festivalPreviewEntry struct {
	path  string
	isDir bool
}

type festivalPreview struct {
	entries              []festivalPreviewEntry
	missingCoreTemplates []string
}

type createFestivalPreviewResult struct {
	OK           bool              `json:"ok"`
	Action       string            `json:"action"`
	DryRun       bool              `json:"dry_run"`
	Festival     map[string]string `json:"festival"`
	TargetPath   string            `json:"target_path"`
	PlannedPaths []string          `json:"planned_paths"`
	Tree         string            `json:"tree"`
	// MissingCoreTemplates lists core festival templates that are absent from
	// the template root. A real create would error on these.
	MissingCoreTemplates []string `json:"missing_core_templates,omitempty"`
	// Markers lists every template marker the create would contain, filled or
	// not, so one dry-run reports the whole set regardless of supplied input.
	Markers         []festivalPreviewMarker `json:"markers"`
	MarkersTotal    int                     `json:"markers_total"`
	MarkersFilled   int                     `json:"markers_filled"`
	MarkersUnfilled int                     `json:"markers_unfilled"`
}

type festivalPreviewTreeNode struct {
	name     string
	isDir    bool
	children []*festivalPreviewTreeNode
	byName   map[string]*festivalPreviewTreeNode
}

func previewCreateFestival(ctx context.Context, cfg *createConfig) error {
	preview, err := buildFestivalPreview(ctx, cfg)
	if err != nil {
		return emitCreateFestivalError(cfg.opts, err)
	}

	markerPreview, err := buildFestivalMarkerPreview(ctx, cfg)
	if err != nil {
		return emitCreateFestivalError(cfg.opts, err)
	}

	displayPath := createDisplayPathFunc(cfg)
	tree := renderFestivalPreviewTree(cfg.dirName, preview.entries)
	plannedPaths := make([]string, 0, len(preview.entries))
	for _, entry := range preview.entries {
		path := displayPath(filepath.Join(cfg.destDir, entry.path))
		if entry.isDir {
			path += string(filepath.Separator)
		}
		plannedPaths = append(plannedPaths, path)
	}

	result := createFestivalPreviewResult{
		OK:                   true,
		Action:               "create_festival_preview",
		DryRun:               true,
		Festival:             festivalMapForConfig(cfg),
		TargetPath:           displayPath(cfg.destDir),
		PlannedPaths:         plannedPaths,
		Tree:                 tree,
		MissingCoreTemplates: preview.missingCoreTemplates,
		Markers:              markerPreview.markers,
		MarkersTotal:         markerPreview.total,
		MarkersFilled:        markerPreview.filled,
		MarkersUnfilled:      markerPreview.unfilled,
	}

	if cfg.opts.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	fmt.Println(ui.H2("Dry Run — No Files Created"))
	fmt.Printf("Would create %s%c\n", result.TargetPath, filepath.Separator)
	fmt.Println(tree)
	if len(preview.missingCoreTemplates) > 0 {
		cfg.display.Error("MISSING core templates: %s — create would fail", strings.Join(preview.missingCoreTemplates, ", "))
		cfg.display.Info("  Copy .festival/templates/festival/ from a working campaign, or run 'fest init' to seed the template directory.")
	}
	printFestivalPreviewMarkers(markerPreview)
	return nil
}

func buildFestivalPreview(ctx context.Context, cfg *createConfig) (*festivalPreview, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	preview := &festivalPreview{}
	addFile := func(path string) {
		preview.entries = append(preview.entries, festivalPreviewEntry{path: filepath.Clean(path)})
	}
	addDir := func(path string) {
		preview.entries = append(preview.entries, festivalPreviewEntry{path: filepath.Clean(path), isDir: true})
	}

	for _, coreTemplate := range festivalCoreTemplates {
		if info, err := os.Stat(filepath.Join(cfg.tmplRoot, coreTemplate.Template)); err == nil && !info.IsDir() {
			addFile(coreTemplate.Output)
		} else {
			preview.missingCoreTemplates = append(preview.missingCoreTemplates, coreTemplate.Template)
		}
	}
	addFile("fest.yaml")

	if cfg.festivalType != nil {
		for i, phaseSpec := range cfg.festivalType.GetAutoPhases() {
			if err := ctx.Err(); err != nil {
				return nil, errors.Wrap(err, "context cancelled")
			}

			phaseID := tpl.FormatPhaseID(i+1, phaseSpec.Name)
			addDir(phaseID)
			addFile(filepath.Join(phaseID, "PHASE_GOAL.md"))

			phaseType := mapPhaseSpecType(phaseSpec.Type)
			templateDir := filepath.Join(cfg.tmplRoot, "phases", phaseType)
			entries, err := collectPhasePreviewEntries(ctx, templateDir)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				entry.path = filepath.Join(phaseID, entry.path)
				preview.entries = append(preview.entries, entry)
			}
		}
	}

	if cfg.seedRequested {
		if ingestPhaseID, ok := ingestAutoPhaseID(cfg.festivalType); ok {
			addDir(filepath.Join(ingestPhaseID, "input_specs"))
			addFile(filepath.Join(ingestPhaseID, "input_specs", "seed.md"))
		}
	}

	gateEntries, err := collectGatePreviewEntries(ctx, cfg.tmplRoot)
	if err != nil {
		return nil, err
	}
	preview.entries = append(preview.entries, gateEntries...)
	preview.entries = deduplicateFestivalPreviewEntries(preview.entries)

	return preview, nil
}

func collectPhasePreviewEntries(ctx context.Context, templateDir string) ([]festivalPreviewEntry, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.IO("reading phase template directory", err).WithField("path", templateDir)
	}

	var planned []festivalPreviewEntry
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
		if entry.Name() == "GOAL.md" || entry.Name() == "gates" {
			continue
		}

		path := filepath.Join(templateDir, entry.Name())
		if entry.IsDir() {
			walked, err := collectTemplateTree(ctx, path, entry.Name())
			if err != nil {
				return nil, err
			}
			planned = append(planned, walked...)
			continue
		}
		planned = append(planned, festivalPreviewEntry{path: entry.Name()})
	}
	return planned, nil
}

func collectTemplateTree(ctx context.Context, root, relativeRoot string) ([]festivalPreviewEntry, error) {
	planned := []festivalPreviewEntry{{path: relativeRoot, isDir: true}}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		planned = append(planned, festivalPreviewEntry{
			path:  filepath.Join(relativeRoot, rel),
			isDir: entry.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, errors.IO("reading phase template tree", err).WithField("path", root)
	}
	return planned, nil
}

func collectGatePreviewEntries(ctx context.Context, tmplRoot string) ([]festivalPreviewEntry, error) {
	plan, err := collectGateTemplatePlan(ctx, tmplRoot)
	if err != nil {
		return nil, err
	}

	var planned []festivalPreviewEntry
	for _, group := range plan.groups {
		planned = append(planned,
			festivalPreviewEntry{path: "gates", isDir: true},
			festivalPreviewEntry{path: filepath.Join("gates", group.phaseType), isDir: true},
		)
		for _, file := range group.files {
			planned = append(planned, festivalPreviewEntry{path: file.dest})
		}
	}
	return planned, nil
}

// gateTemplateFile pairs a gate template source with the festival-relative path
// creation would copy it to.
type gateTemplateFile struct {
	source string
	dest   string
}

// gateTemplateGroup holds the gate templates creation would copy for one phase
// type. The group exists even when it carries no files, because creation still
// makes the gates/<phase type>/ directory.
type gateTemplateGroup struct {
	phaseType string
	files     []gateTemplateFile
}

// gateTemplatePlan describes the gate directories creation would make and the
// gate templates it would copy into them, in copy order.
type gateTemplatePlan struct {
	groups []gateTemplateGroup
}

// collectGateTemplatePlan resolves the gate work copyGateTemplates would do,
// without touching the destination. Both the preview tree and the preview marker
// report read from it so they cannot drift from the real copy.
func collectGateTemplatePlan(ctx context.Context, tmplRoot string) (*gateTemplatePlan, error) {
	plan := &gateTemplatePlan{}
	for _, phaseType := range festivalGatePhaseTypes {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}

		sourceDir := filepath.Join(tmplRoot, "phases", phaseType, "gates")
		entries, err := os.ReadDir(sourceDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.IO("reading gate template directory", err).WithField("path", sourceDir)
		}

		group := gateTemplateGroup{phaseType: phaseType}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			group.files = append(group.files, gateTemplateFile{
				source: filepath.Join(sourceDir, entry.Name()),
				dest:   filepath.Join("gates", phaseType, entry.Name()),
			})
		}
		plan.groups = append(plan.groups, group)
	}
	return plan, nil
}

func deduplicateFestivalPreviewEntries(entries []festivalPreviewEntry) []festivalPreviewEntry {
	seen := make(map[string]int, len(entries))
	result := make([]festivalPreviewEntry, 0, len(entries))
	for _, entry := range entries {
		key := filepath.Clean(entry.path)
		if index, ok := seen[key]; ok {
			result[index].isDir = result[index].isDir || entry.isDir
			continue
		}
		seen[key] = len(result)
		entry.path = key
		result = append(result, entry)
	}
	return result
}

func renderFestivalPreviewTree(rootName string, entries []festivalPreviewEntry) string {
	root := &festivalPreviewTreeNode{name: rootName, isDir: true, byName: make(map[string]*festivalPreviewTreeNode)}
	for _, entry := range entries {
		parts := strings.Split(filepath.ToSlash(entry.path), "/")
		current := root
		for i, part := range parts {
			if part == "" || part == "." {
				continue
			}
			child, ok := current.byName[part]
			if !ok {
				child = &festivalPreviewTreeNode{name: part, byName: make(map[string]*festivalPreviewTreeNode)}
				current.byName[part] = child
				current.children = append(current.children, child)
			}
			if i < len(parts)-1 || entry.isDir {
				child.isDir = true
			}
			current = child
		}
	}

	var output strings.Builder
	output.WriteString(root.name)
	output.WriteString("/\n")
	renderFestivalPreviewChildren(&output, root, "")
	return strings.TrimRight(output.String(), "\n")
}

func renderFestivalPreviewChildren(output *strings.Builder, node *festivalPreviewTreeNode, prefix string) {
	sort.SliceStable(node.children, func(i, j int) bool {
		return node.children[i].name < node.children[j].name
	})
	for i, child := range node.children {
		last := i == len(node.children)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		output.WriteString(prefix)
		output.WriteString(connector)
		output.WriteString(child.name)
		if child.isDir {
			output.WriteString("/")
		}
		output.WriteString("\n")
		renderFestivalPreviewChildren(output, child, nextPrefix)
	}
}
