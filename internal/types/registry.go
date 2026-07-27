package types

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// markerPattern matches REPLACE markers in templates.
var markerPattern = regexp.MustCompile(`\[REPLACE[^\]]*\]`)

// defaultPhaseType is the default phase type for create phase --type and for
// IsDefault tagging during methodology discovery. Keep in sync with
// create_phase flag default ("planning").
const defaultPhaseType = "planning"

// Registry holds all discovered template types organized by level.
type Registry struct {
	// Festival holds festival-level types.
	Festival []TypeInfo `json:"festival"`
	// Phase holds phase-level types.
	Phase []TypeInfo `json:"phase"`
	// Sequence holds sequence-level types.
	Sequence []TypeInfo `json:"sequence"`
	// Task holds task-level types.
	Task []TypeInfo `json:"task"`
}

// NewRegistry creates an empty type registry.
func NewRegistry() *Registry {
	return &Registry{
		Festival: []TypeInfo{},
		Phase:    []TypeInfo{},
		Sequence: []TypeInfo{},
		Task:     []TypeInfo{},
	}
}

// DiscoverOptions configures type discovery behavior.
type DiscoverOptions struct {
	// BuiltInDir is the path to built-in templates
	// (e.g., ~/.obey/fest/festivals/.festival/templates).
	BuiltInDir string
	// CustomDir is the path to custom templates (e.g., .festival/templates/).
	CustomDir string
	// CountMarkers enables counting REPLACE markers in templates.
	CountMarkers bool
}

// Discover scans template directories and populates the registry.
func (r *Registry) Discover(ctx context.Context, opts DiscoverOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("Registry.Discover")
	}

	// Scan built-in templates
	if opts.BuiltInDir != "" {
		if err := r.scanDirectory(ctx, opts.BuiltInDir, false, opts.CountMarkers); err != nil {
			// Non-fatal: built-in dir might not exist
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Scan custom templates
	if opts.CustomDir != "" {
		if err := r.scanDirectory(ctx, opts.CustomDir, true, opts.CountMarkers); err != nil {
			// Non-fatal: custom dir might not exist
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Sort types by name within each level
	r.sortTypes()

	return nil
}

// MergeFestivalWorkflowTypes adds festival workflow blueprints (from
// festival_types.yaml) into the festival level of the registry so
// `fest types list` surfaces the same names as `fest create festival --type`.
func (r *Registry) MergeFestivalWorkflowTypes(config *FestivalTypesConfig) {
	if config == nil {
		return
	}
	for _, ft := range config.Types {
		desc := ft.Description
		r.addType(&TypeInfo{
			Name:        ft.Name,
			Level:       LevelFestival,
			Description: desc,
			IsDefault:   ft.Default,
			IsCustom:    false,
			Templates:   nil,
			Source:      "festival_types.yaml",
		})
	}
	r.sortTypes()
}

// scanDirectory scans a template directory and extracts types.
// Supports both the modern nested methodology layout
// (phases/<type>/, sequences/, tasks/, festival/) and legacy flat
// FESTIVAL_*/PHASE_*/TASK_* filenames.
func (r *Registry) scanDirectory(ctx context.Context, dir string, isCustom bool, countMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}

	if isMethodologyTemplatesRoot(dir) {
		return r.scanMethodologyRoot(ctx, dir, isCustom, countMarkers)
	}

	return r.scanLegacyFlatAndRecurse(ctx, dir, isCustom, countMarkers)
}

// isMethodologyTemplatesRoot reports whether dir uses the nested package layout
// produced by fest system sync (phases/, sequences/, tasks/, and/or festival/).
func isMethodologyTemplatesRoot(dir string) bool {
	for _, name := range []string{"phases", "sequences", "tasks", "festival"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// scanMethodologyRoot discovers types from the nested templates tree.
func (r *Registry) scanMethodologyRoot(ctx context.Context, root string, isCustom bool, countMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Phase packages: phases/<type>/{GOAL,GATES,WORKFLOW,...}.md
	phasesDir := filepath.Join(root, "phases")
	if info, err := os.Stat(phasesDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(phasesDir)
		if err != nil {
			return errors.IO("reading phases directory", err).WithField("path", phasesDir)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !entry.IsDir() {
				continue
			}
			typeName := entry.Name()
			pkgDir := filepath.Join(phasesDir, typeName)
			files, markers, err := collectMarkdownPackage(pkgDir, countMarkers)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				continue
			}
			r.addType(&TypeInfo{
				Name:      typeName,
				Level:     LevelPhase,
				IsCustom:  isCustom,
				IsDefault: typeName == defaultPhaseType,
				Templates: files,
				Source:    pkgDir,
				Markers:   markers,
			})
			// Quality gates under phases/<type>/gates/
			gatesDir := filepath.Join(pkgDir, "gates")
			gInfo, gErr := os.Stat(gatesDir)
			if gErr != nil {
				if os.IsNotExist(gErr) {
					continue
				}
				return errors.IO("statting gates directory", gErr).WithField("path", gatesDir)
			}
			if gInfo.IsDir() {
				if err := r.scanLegacyFlatAndRecurse(ctx, gatesDir, isCustom, countMarkers); err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return err
				}
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return errors.IO("statting phases directory", err).WithField("path", phasesDir)
	}

	// Sequence package: sequences/GOAL.md, GOAL_MINIMAL.md
	if err := r.scanFlatPackageDir(ctx, filepath.Join(root, "sequences"), LevelSequence, isCustom, countMarkers); err != nil {
		return err
	}

	// Task package: tasks/TASK.md
	if err := r.scanFlatPackageDir(ctx, filepath.Join(root, "tasks"), LevelTask, isCustom, countMarkers); err != nil {
		return err
	}

	// festival/ holds shared scaffold files (GOAL, OVERVIEW, …), not create --type
	// values. Workflow festival types come from festival_types.yaml via
	// MergeFestivalWorkflowTypes. Do not register "scaffold" as a festival type.

	// Any legacy flat files still sitting at the templates root.
	return r.scanLegacyFlatFilesOnly(ctx, root, isCustom, countMarkers)
}

// scanFlatPackageDir maps markdown files in a single package directory to types.
// GOAL.md / TASK.md → "default"; GOAL_MINIMAL.md → "minimal"; others lowercased stem.
func (r *Registry) scanFlatPackageDir(ctx context.Context, dir string, level Level, isCustom bool, countMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.IO("reading package directory", err).WithField("path", dir)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := entry.Name()
		stem := strings.TrimSuffix(name, ".md")
		typeName := packageFileTypeName(stem)
		markers := 0
		if countMarkers {
			markers, _ = countMarkersInFile(filepath.Join(dir, name))
		}
		r.addType(&TypeInfo{
			Name:      typeName,
			Level:     level,
			IsCustom:  isCustom,
			IsDefault: typeName == "default",
			Templates: []string{name},
			Source:    dir,
			Markers:   markers,
		})
	}
	return nil
}

func packageFileTypeName(stem string) string {
	switch strings.ToUpper(stem) {
	case "GOAL", "TASK", "OVERVIEW", "RULES", "TODO", "GATES", "WORKFLOW":
		return "default"
	case "GOAL_MINIMAL":
		return "minimal"
	default:
		return strings.ToLower(stem)
	}
}

func collectMarkdownPackage(dir string, countMarkers bool) (files []string, markers int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, errors.IO("reading package directory", err).WithField("path", dir)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		files = append(files, entry.Name())
		if countMarkers {
			n, _ := countMarkersInFile(filepath.Join(dir, entry.Name()))
			markers += n
		}
	}
	sort.Strings(files)
	return files, markers, nil
}

// scanLegacyFlatAndRecurse walks dir recursively, parsing legacy TEMPLATE filenames.
func (r *Registry) scanLegacyFlatAndRecurse(ctx context.Context, dir string, isCustom bool, countMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.IO("reading directory", err).WithField("path", dir)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			// Do not re-enter methodology package roots via recursion from a parent.
			if isMethodologyTemplatesRoot(subDir) {
				if err := r.scanMethodologyRoot(ctx, subDir, isCustom, countMarkers); err != nil {
					// Missing roots are non-fatal; permission/IO failures must surface.
					if os.IsNotExist(err) {
						continue
					}
					return err
				}
				continue
			}
			if err := r.scanLegacyFlatAndRecurse(ctx, subDir, isCustom, countMarkers); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			continue
		}
		r.maybeAddLegacyFile(entry.Name(), dir, isCustom, countMarkers)
	}
	return nil
}

// scanLegacyFlatFilesOnly processes only files in dir (no subdirectory descent).
func (r *Registry) scanLegacyFlatFilesOnly(ctx context.Context, dir string, isCustom bool, countMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.IO("reading directory", err).WithField("path", dir)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		r.maybeAddLegacyFile(entry.Name(), dir, isCustom, countMarkers)
	}
	return nil
}

func (r *Registry) maybeAddLegacyFile(name, dir string, isCustom bool, countMarkers bool) {
	if !strings.HasSuffix(name, ".md") {
		return
	}
	if !strings.Contains(name, "TEMPLATE") && !strings.Contains(name, "GOAL") && !strings.Contains(name, "GATE") {
		return
	}
	typeInfo := parseTemplateFilename(name, dir, isCustom)
	if typeInfo == nil {
		return
	}
	if countMarkers {
		filePath := filepath.Join(dir, name)
		markers, _ := countMarkersInFile(filePath)
		typeInfo.Markers = markers
	}
	r.addType(typeInfo)
}

// parseTemplateFilename extracts type information from a template filename.
// Examples:
//   - FESTIVAL_GOAL_TEMPLATE.md → level: festival, type: goal
//   - PHASE_GOAL_IMPLEMENTATION_TEMPLATE.md → level: phase, type: implementation
//   - TASK_TEMPLATE_SIMPLE.md → level: task, type: simple
//   - QUALITY_GATE_REVIEW.md → level: task, type: review (gate)
func parseTemplateFilename(filename, source string, isCustom bool) *TypeInfo {
	// Remove .md suffix
	name := strings.TrimSuffix(filename, ".md")

	// Determine level from prefix
	var level Level
	var typeName string

	switch {
	case strings.HasPrefix(name, "FESTIVAL_"):
		level = LevelFestival
		name = strings.TrimPrefix(name, "FESTIVAL_")
	case strings.HasPrefix(name, "PHASE_"):
		level = LevelPhase
		name = strings.TrimPrefix(name, "PHASE_")
	case strings.HasPrefix(name, "SEQUENCE_"):
		level = LevelSequence
		name = strings.TrimPrefix(name, "SEQUENCE_")
	case strings.HasPrefix(name, "TASK_"):
		level = LevelTask
		name = strings.TrimPrefix(name, "TASK_")
	case strings.HasPrefix(name, "QUALITY_GATE_"):
		level = LevelTask // Quality gates apply to tasks
		typeName = strings.ToLower(strings.TrimPrefix(name, "QUALITY_GATE_"))
		return &TypeInfo{
			Name:      "gate/" + typeName,
			Level:     level,
			IsCustom:  isCustom,
			IsDefault: false,
			Templates: []string{filename},
			Source:    source,
		}
	case strings.HasPrefix(name, "RESEARCH_"):
		level = LevelTask
		name = strings.TrimPrefix(name, "RESEARCH_")
	default:
		return nil // Unknown prefix
	}

	// Extract type name from remaining parts
	// Format: {GOAL|TEMPLATE}[_TYPE][_TEMPLATE]
	parts := strings.Split(name, "_")
	isDefault := false

	if len(parts) == 1 {
		// Just GOAL or TEMPLATE
		typeName = strings.ToLower(parts[0])
		isDefault = true
	} else if len(parts) == 2 && parts[1] == "TEMPLATE" {
		// GOAL_TEMPLATE or similar
		typeName = strings.ToLower(parts[0])
		isDefault = true
	} else {
		// Has a type suffix: GOAL_IMPLEMENTATION_TEMPLATE or TEMPLATE_SIMPLE
		// Find the type name (not GOAL, TEMPLATE, or common suffixes)
		for _, part := range parts {
			lower := strings.ToLower(part)
			if lower != "goal" && lower != "template" && lower != "research" {
				typeName = lower
				break
			}
		}
		if typeName == "" {
			typeName = strings.ToLower(parts[0])
			isDefault = true
		}
	}

	// Handle research templates specially
	if strings.Contains(filename, "RESEARCH_") {
		typeName = "research/" + typeName
		isDefault = false // Research templates are never default
	}

	return &TypeInfo{
		Name:      typeName,
		Level:     level,
		IsCustom:  isCustom,
		IsDefault: isDefault,
		Templates: []string{filename},
		Source:    source,
	}
}

// countMarkersInFile counts REPLACE markers in a template file.
func countMarkersInFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	matches := markerPattern.FindAllString(string(content), -1)
	return len(matches), nil
}

// addType adds a type to the appropriate level in the registry.
// If a type already exists, it merges the template files.
func (r *Registry) addType(info *TypeInfo) {
	var types *[]TypeInfo
	switch info.Level {
	case LevelFestival:
		types = &r.Festival
	case LevelPhase:
		types = &r.Phase
	case LevelSequence:
		types = &r.Sequence
	case LevelTask:
		types = &r.Task
	default:
		return
	}

	// Check if type already exists
	for i := range *types {
		if (*types)[i].Name == info.Name {
			// Merge template files
			(*types)[i].Templates = append((*types)[i].Templates, info.Templates...)
			// Custom types override built-in
			if info.IsCustom {
				(*types)[i].IsCustom = true
			}
			// Update marker count if higher
			if info.Markers > (*types)[i].Markers {
				(*types)[i].Markers = info.Markers
			}
			if info.Description != "" && (*types)[i].Description == "" {
				(*types)[i].Description = info.Description
			}
			if info.IsDefault {
				(*types)[i].IsDefault = true
			}
			if info.Source != "" && (*types)[i].Source == "" {
				(*types)[i].Source = info.Source
			}
			return
		}
	}

	// Add new type
	*types = append(*types, *info)
}

// sortTypes sorts types within each level by name.
func (r *Registry) sortTypes() {
	sort.Slice(r.Festival, func(i, j int) bool {
		return r.Festival[i].Name < r.Festival[j].Name
	})
	sort.Slice(r.Phase, func(i, j int) bool {
		return r.Phase[i].Name < r.Phase[j].Name
	})
	sort.Slice(r.Sequence, func(i, j int) bool {
		return r.Sequence[i].Name < r.Sequence[j].Name
	})
	sort.Slice(r.Task, func(i, j int) bool {
		return r.Task[i].Name < r.Task[j].Name
	})
}

// TypesForLevel returns all types for a given level.
func (r *Registry) TypesForLevel(level Level) []TypeInfo {
	switch level {
	case LevelFestival:
		return r.Festival
	case LevelPhase:
		return r.Phase
	case LevelSequence:
		return r.Sequence
	case LevelTask:
		return r.Task
	default:
		return nil
	}
}

// FindType finds a type by name and level.
func (r *Registry) FindType(level Level, name string) *TypeInfo {
	types := r.TypesForLevel(level)
	for i := range types {
		if types[i].Name == name {
			return &types[i]
		}
	}
	return nil
}

// AllTypes returns all types across all levels.
func (r *Registry) AllTypes() []TypeInfo {
	all := []TypeInfo{}
	all = append(all, r.Festival...)
	all = append(all, r.Phase...)
	all = append(all, r.Sequence...)
	all = append(all, r.Task...)
	return all
}

// TypeCount returns the total count of discovered types.
func (r *Registry) TypeCount() int {
	return len(r.Festival) + len(r.Phase) + len(r.Sequence) + len(r.Task)
}
