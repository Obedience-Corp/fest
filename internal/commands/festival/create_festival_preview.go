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
	"github.com/Obedience-Corp/fest/internal/pathutil"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
)

type festivalPreviewEntry struct {
	path  string
	isDir bool
}

type festivalPreview struct {
	entries []festivalPreviewEntry
}

type createFestivalPreviewResult struct {
	OK           bool              `json:"ok"`
	Action       string            `json:"action"`
	DryRun       bool              `json:"dry_run"`
	Festival     map[string]string `json:"festival"`
	TargetPath   string            `json:"target_path"`
	PlannedPaths []string          `json:"planned_paths"`
	Tree         string            `json:"tree"`
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

	displayPath := func(path string) string {
		return pathutil.DisplayPath(path, cfg.campaignRoot)
	}
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
		OK:           true,
		Action:       "create_festival_preview",
		DryRun:       true,
		Festival:     festivalMapForConfig(cfg),
		TargetPath:   displayPath(cfg.destDir),
		PlannedPaths: plannedPaths,
		Tree:         tree,
	}

	if cfg.opts.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	fmt.Println(ui.H2("Dry Run — No Files Created"))
	fmt.Printf("Would create %s%c\n", result.TargetPath, filepath.Separator)
	fmt.Println(tree)
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
	var planned []festivalPreviewEntry
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

		planned = append(planned,
			festivalPreviewEntry{path: "gates", isDir: true},
			festivalPreviewEntry{path: filepath.Join("gates", phaseType), isDir: true},
		)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				planned = append(planned, festivalPreviewEntry{path: filepath.Join("gates", phaseType, entry.Name())})
			}
		}
	}
	return planned, nil
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
