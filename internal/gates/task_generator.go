// Package gates provides quality gate task generation.
package gates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"gopkg.in/yaml.v3"
)

// TaskGenerator generates quality gate task files in sequences.
type TaskGenerator struct {
	templateRoot string
	catalog      *tpl.Catalog
	manager      *tpl.Manager
}

// NewTaskGenerator creates a task generator with the given template root.
func NewTaskGenerator(ctx context.Context, templateRoot string) (*TaskGenerator, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled").
			WithOp("NewTaskGenerator")
	}

	catalog, _ := tpl.LoadCatalog(ctx, templateRoot)

	return &TaskGenerator{
		templateRoot: templateRoot,
		catalog:      catalog,
		manager:      tpl.NewManager(),
	}, nil
}

// GenerateOptions controls task file generation.
type GenerateOptions struct {
	DryRun  bool // Preview without creating files
	Force   bool // Overwrite existing files
	Verbose bool // Include verbose output
}

// GenerateResult represents the result of generating a task file.
type GenerateResult struct {
	Type     string `json:"type"`     // "create", "skip", "exists"
	Path     string `json:"path"`     // Full path to task file
	Template string `json:"template"` // Template used
	Reason   string `json:"reason"`   // Reason for skip
	TaskID   string `json:"task_id"`  // Gate task ID
}

// GenerateSummary provides statistics about generation.
type GenerateSummary struct {
	TotalSequences   int `json:"total_sequences"`
	SequencesUpdated int `json:"sequences_updated"`
	FilesCreated     int `json:"files_created"`
	FilesSkipped     int `json:"files_skipped"`
}

// GenerateForSequence generates task files for gates in a single sequence.
// festivalPath is optional - if provided, it's used to resolve gates/ prefixed templates.
func (g *TaskGenerator) GenerateForSequence(
	ctx context.Context,
	sequencePath string,
	gates []GateTask,
	opts GenerateOptions,
	festivalPath ...string,
) ([]GenerateResult, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "context cancelled").
			WithOp("TaskGenerator.GenerateForSequence")
	}

	var results []GenerateResult
	var warnings []string

	// Get existing tasks in sequence
	entries, err := os.ReadDir(sequencePath)
	if err != nil {
		return nil, nil, errors.IO("reading sequence directory", err).
			WithField("path", sequencePath)
	}

	// Find highest task number and existing task IDs
	maxNum := 0
	existingTasks := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if entry.Name() == "SEQUENCE_GOAL.md" {
			continue
		}
		existingTasks[entry.Name()] = true
		num := festival.ParseTaskNumber(entry.Name())
		if num > maxNum {
			maxNum = num
		}
	}

	// Generate each gate task
	for i, gate := range gates {
		if !gate.Enabled {
			continue
		}

		taskNum := maxNum + i + 1
		taskFileName := tpl.FormatTaskID(taskNum, gate.ID)
		taskPath := filepath.Join(sequencePath, taskFileName)

		// Check if task already exists (by ID pattern)
		taskExists := false
		for existingName := range existingTasks {
			if strings.Contains(existingName, gate.ID) {
				taskExists = true
				break
			}
		}

		if taskExists {
			results = append(results, GenerateResult{
				Type:   "exists",
				Path:   taskPath,
				TaskID: gate.ID,
				Reason: "task_already_exists",
			})
			continue
		}

		// Check if file exists (could be renamed)
		if _, err := os.Stat(taskPath); err == nil {
			if !opts.Force {
				results = append(results, GenerateResult{
					Type:   "skip",
					Path:   taskPath,
					TaskID: gate.ID,
					Reason: "file_exists",
				})
				continue
			}
		}

		// Create the task
		if !opts.DryRun {
			var festPath string
			if len(festivalPath) > 0 {
				festPath = festivalPath[0]
			}
			content := g.renderGateContent(ctx, gate, taskNum, festPath)

			// Inject frontmatter if content doesn't already have it
			if !strings.HasPrefix(strings.TrimSpace(content), "---") {
				parentSequenceID := filepath.Base(sequencePath)
				fm := frontmatter.NewGateFrontmatter(taskFileName, gate.Name, parentSequenceID, taskNum, inferGateType(gate.ID))
				contentWithFM, fmErr := frontmatter.InjectString(content, fm)
				if fmErr != nil {
					warnings = append(warnings, fmt.Sprintf("Failed to inject frontmatter for %s: %v", taskPath, fmErr))
				} else {
					content = contentWithFM
				}
			}

			if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to write %s: %v", taskPath, err))
				continue
			}
		}

		results = append(results, GenerateResult{
			Type:     "create",
			Path:     taskPath,
			Template: gate.Template,
			TaskID:   gate.ID,
		})
	}

	return results, warnings, nil
}

// renderGateContent renders the content for a gate task file.
// festivalPath is used to resolve festival-local gates/ templates.
// Template paths are resolved in this order:
// 1. Festival-local: {festivalPath}/gates/{templateName}.md
// 2. Catalog lookup by template ID
// 3. New structure: {templateRoot}/{template}.tmpl (e.g., agent/gates/implementation/testing.tmpl)
// 4. Legacy .md: {templateRoot}/{template}.md
// 5. Default generated content
func (g *TaskGenerator) renderGateContent(ctx context.Context, gate GateTask, taskNum int, festivalPath string) string {
	// Build context
	tmplCtx := tpl.NewContext()
	tmplCtx.SetTask(taskNum, gate.ID)
	if gate.Customizations != nil {
		for k, v := range gate.Customizations {
			tmplCtx.SetCustom(k, v)
		}
	}

	var content string

	// 1. Handle "gates/TEMPLATE_NAME" format - resolve from festival's gates/ directory first
	if strings.HasPrefix(gate.Template, "gates/") && festivalPath != "" {
		templateName := strings.TrimPrefix(gate.Template, "gates/")
		gatesPath := filepath.Join(festivalPath, "gates", templateName+".md")

		if _, err := os.Stat(gatesPath); err == nil {
			content = g.loadAndRenderTemplate(ctx, gatesPath, tmplCtx)
		}
	}

	// 2. Try catalog if not found in festival gates/
	if content == "" && g.catalog != nil {
		templateID := gate.Template
		// Strip common prefixes for catalog lookup
		for _, prefix := range []string{"gates/", "agent/gates/"} {
			if strings.HasPrefix(templateID, prefix) {
				templateID = strings.TrimPrefix(templateID, prefix)
				break
			}
		}
		content, _ = g.manager.RenderByID(ctx, g.catalog, templateID, tmplCtx)
	}

	// 3. Try direct file load from template root
	if content == "" && g.templateRoot != "" {
		// Build list of possible paths to try
		possiblePaths := buildTemplatePaths(g.templateRoot, gate.Template)

		for _, tpath := range possiblePaths {
			if _, err := os.Stat(tpath); err == nil {
				content = g.loadAndRenderTemplate(ctx, tpath, tmplCtx)
				if content != "" {
					break
				}
			}
		}
	}

	// 4. Use default content if template not found
	if content == "" {
		content = generateDefaultGateContent(gate)
	}

	return content
}

// loadAndRenderTemplate loads a template file and renders it with context.
func (g *TaskGenerator) loadAndRenderTemplate(ctx context.Context, path string, tmplCtx *tpl.Context) string {
	loader := tpl.NewLoader()
	t, err := loader.Load(ctx, path)
	if err != nil {
		return ""
	}

	// Only render if template has Go template syntax
	if strings.Contains(t.Content, "{{") {
		content, _ := g.manager.Render(t, tmplCtx)
		return content
	}
	return t.Content
}

// buildTemplatePaths returns a list of possible file paths for a template.
// Tries multiple extensions and directory structures.
func buildTemplatePaths(templateRoot, templatePath string) []string {
	var paths []string

	// Normalize template path - remove any leading prefixes
	normalized := templatePath
	for _, prefix := range []string{"gates/", "phases/"} {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			break
		}
	}

	// New structure with .tmpl extension (preferred)
	// e.g., agent/gates/implementation/testing -> agent/gates/implementation/testing.tmpl
	paths = append(paths, filepath.Join(templateRoot, templatePath+".tmpl"))

	// New structure without extension prefix
	// e.g., implementation/testing -> agent/gates/implementation/testing.tmpl
	if !strings.HasPrefix(templatePath, "agent/") {
		paths = append(paths, filepath.Join(templateRoot, "agent", "gates", normalized+".tmpl"))
	}

	// Legacy .md extension
	paths = append(paths, filepath.Join(templateRoot, templatePath+".md"))

	// Legacy gates/ structure
	paths = append(paths, filepath.Join(templateRoot, "gates", normalized+".md"))

	// Legacy phases/ structure
	// e.g., phases/implementation/gates/testing.md
	if strings.Contains(normalized, "/") {
		parts := strings.SplitN(normalized, "/", 2)
		if len(parts) == 2 {
			paths = append(paths, filepath.Join(templateRoot, "phases", parts[0], "gates", parts[1]+".md"))
		}
	}

	return paths
}

// generateDefaultGateContent creates default content for a gate task.
func generateDefaultGateContent(gate GateTask) string {
	name := gate.Name
	if name == "" {
		name = strings.ReplaceAll(gate.ID, "_", " ")
		name = strings.Title(name)
	}

	return fmt.Sprintf(`# Task: %s

## Objective

%s

## Requirements

- [ ] Complete this quality gate task
- [ ] Verify all requirements are met
- [ ] Document any findings

## Definition of Done

- [ ] Task completed successfully
- [ ] All checks pass
- [ ] Ready to proceed

## Notes

[Add notes here]
`, name, name)
}

// FindFestivalRoot finds the festival root directory from a starting path.
func FindFestivalRoot(startPath string) (string, error) {
	path := startPath
	for {
		// Check for festival markers
		if _, err := os.Stat(filepath.Join(path, "FESTIVAL_OVERVIEW.md")); err == nil {
			return path, nil
		}
		if _, err := os.Stat(filepath.Join(path, "fest.yaml")); err == nil {
			return path, nil
		}
		if _, err := os.Stat(filepath.Join(path, "FESTIVAL_GOAL.md")); err == nil {
			return path, nil
		}

		// Move up
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return "", errors.NotFound("festival root").
		WithField("start_path", startPath)
}

// FindImplementationSequences finds all implementation sequences in a festival.
func FindImplementationSequences(festivalRoot string, excludePatterns []string) ([]string, error) {
	var sequences []string

	entries, err := os.ReadDir(festivalRoot)
	if err != nil {
		return nil, errors.IO("reading festival root", err).
			WithField("path", festivalRoot)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a phase (starts with number)
		if !festival.IsPhase(entry.Name()) {
			continue
		}

		phasePath := filepath.Join(festivalRoot, entry.Name())

		// Walk through sequences in phase
		seqEntries, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}

		for _, seqEntry := range seqEntries {
			if !seqEntry.IsDir() {
				continue
			}

			// Check if it's a sequence (starts with number)
			if !festival.IsSequence(seqEntry.Name()) {
				continue
			}

			// Check excluded patterns
			if isSequenceExcluded(seqEntry.Name(), excludePatterns) {
				continue
			}

			sequences = append(sequences, filepath.Join(phasePath, seqEntry.Name()))
		}
	}

	return sequences, nil
}

// isSequenceExcluded checks if a sequence matches any excluded pattern.
func isSequenceExcluded(sequenceName string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, sequenceName)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// SequenceInfo contains information about a sequence for generation.
type SequenceInfo struct {
	Path      string // Full path to sequence directory
	PhasePath string // Path to parent phase
	Name      string // Sequence directory name
	PhaseType string // Phase type: "implementation", "planning", "research", "review", "action"
	PhaseName string // Name of the parent phase directory
}

// DetectPhaseType determines the phase type from the phase directory.
// Priority order:
// 1. Read from PHASE_GOAL.md frontmatter (fest_phase_type field)
// 2. Fall back to inferring from directory name
// Returns: "planning", "implementation", "research", "review", "non_coding_action"
// Returns empty string if type cannot be determined (error case).
func DetectPhaseType(phasePath string) string {
	// First try to read from PHASE_GOAL.md frontmatter
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	if content, err := os.ReadFile(goalPath); err == nil {
		if fm, _, err := frontmatter.Parse(content); err == nil && fm != nil {
			if fm.PhaseType != "" {
				// Map frontmatter.PhaseType to our internal naming
				return mapPhaseType(string(fm.PhaseType))
			}
		}
	}

	// Fall back to directory name inference
	phaseName := filepath.Base(phasePath)
	return inferPhaseTypeFromName(phaseName)
}

// mapPhaseType normalizes phase type values to internal naming.
func mapPhaseType(phaseType string) string {
	switch strings.ToLower(phaseType) {
	case "planning", "plan":
		return "planning"
	case "implementation", "implement", "build":
		return "implementation"
	case "research", "discovery":
		return "research"
	case "review", "qa":
		return "review"
	case "deployment", "deploy", "action", "non_coding_action":
		return "non_coding_action"
	default:
		return phaseType
	}
}

// inferPhaseTypeFromName infers phase type from directory name.
// Returns empty string if type cannot be determined.
func inferPhaseTypeFromName(phaseName string) string {
	lower := strings.ToLower(phaseName)

	switch {
	case strings.Contains(lower, "planning") || strings.Contains(lower, "plan"):
		return "planning"
	case strings.Contains(lower, "research") || strings.Contains(lower, "discovery"):
		return "research"
	case strings.Contains(lower, "design"):
		return "research" // Design phases use research-like structure
	case strings.Contains(lower, "review") || strings.Contains(lower, "qa") || strings.Contains(lower, "uat"):
		return "review"
	// Action phases: deployment, configuration, publishing, migrations, operational tasks
	case strings.Contains(lower, "deployment") || strings.Contains(lower, "deploy") ||
		strings.Contains(lower, "release") || strings.Contains(lower, "action") ||
		strings.Contains(lower, "operation") || strings.Contains(lower, "config") ||
		strings.Contains(lower, "publish") || strings.Contains(lower, "migrat"):
		return "non_coding_action"
	case strings.Contains(lower, "implementation") || strings.Contains(lower, "implement") ||
		strings.Contains(lower, "develop") || strings.Contains(lower, "build") ||
		strings.Contains(lower, "foundation") || strings.Contains(lower, "critical"):
		return "implementation"
	default:
		return "" // No default - require explicit type
	}
}

// FindSequencesWithInfo finds sequences and returns detailed info.
func FindSequencesWithInfo(festivalRoot string, excludePatterns []string) ([]SequenceInfo, error) {
	var sequences []SequenceInfo

	entries, err := os.ReadDir(festivalRoot)
	if err != nil {
		return nil, errors.IO("reading festival root", err).
			WithField("path", festivalRoot)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !festival.IsPhase(entry.Name()) {
			continue
		}

		phasePath := filepath.Join(festivalRoot, entry.Name())

		seqEntries, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}

		// Detect phase type from phase path (checks frontmatter first)
		phaseName := entry.Name()
		phaseType := DetectPhaseType(phasePath)

		for _, seqEntry := range seqEntries {
			if !seqEntry.IsDir() {
				continue
			}

			if !festival.IsSequence(seqEntry.Name()) {
				continue
			}

			if isSequenceExcluded(seqEntry.Name(), excludePatterns) {
				continue
			}

			sequences = append(sequences, SequenceInfo{
				Path:      filepath.Join(phasePath, seqEntry.Name()),
				PhasePath: phasePath,
				Name:      seqEntry.Name(),
				PhaseType: phaseType,
				PhaseName: phaseName,
			})
		}
	}

	return sequences, nil
}

// DiscoverGatesForPhaseType discovers gates for a phase type.
// It tries sources in this order:
// 1. gates.yaml at templateRoot/agent/gates/{phaseType}/gates.yaml (preferred - defines order)
// 2. Directory listing of templateRoot/agent/gates/{phaseType}/*.tmpl (legacy fallback)
// 3. Directory listing of templateRoot/phases/{phaseType}/gates/*.md (old legacy fallback)
// Returns an error if no gates can be found.
func DiscoverGatesForPhaseType(templateRoot, phaseType string) ([]GateTask, error) {
	// Try gates.yaml first (preferred - defines explicit ordering)
	if gates, err := LoadGatesYaml(templateRoot, phaseType); err == nil {
		return gates, nil
	}

	// Try new template structure: agent/gates/{phaseType}/*.tmpl
	newGatesDir := filepath.Join(templateRoot, "agent", "gates", phaseType)
	if _, err := os.Stat(newGatesDir); err == nil {
		gates, err := discoverGatesFromDirectory(newGatesDir, phaseType, ".tmpl")
		if err == nil && len(gates) > 0 {
			return gates, nil
		}
	}

	// Legacy fallback: phases/{phaseType}/gates/*.md
	legacyGatesDir := filepath.Join(templateRoot, "phases", phaseType, "gates")
	if _, err := os.Stat(legacyGatesDir); err == nil {
		gates, err := discoverGatesFromDirectory(legacyGatesDir, phaseType, ".md")
		if err == nil && len(gates) > 0 {
			return gates, nil
		}
	}

	return nil, errors.NotFound("gates for phase type").
		WithField("phase_type", phaseType).
		WithField("tried_paths", []string{
			filepath.Join(templateRoot, "agent", "gates", phaseType, "gates.yaml"),
			newGatesDir,
			legacyGatesDir,
		}).
		WithField("hint", fmt.Sprintf("Create gates.yaml at templates/agent/gates/%s/gates.yaml", phaseType))
}

// discoverGatesFromDirectory reads gate templates from a directory.
// Returns gate tasks constructed from the files found.
func discoverGatesFromDirectory(gatesDir, phaseType, extension string) ([]GateTask, error) {
	entries, err := os.ReadDir(gatesDir)
	if err != nil {
		return nil, errors.IO("reading gates directory", err).
			WithField("path", gatesDir)
	}

	var gates []GateTask
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip non-matching extensions and gates.yaml
		if !strings.HasSuffix(entry.Name(), extension) || entry.Name() == "gates.yaml" {
			continue
		}

		// Extract gate ID from filename (e.g., "testing.tmpl" -> "testing")
		baseName := strings.TrimSuffix(entry.Name(), extension)
		gateID := strings.ReplaceAll(baseName, "-", "_")

		// Build template path
		templatePath := filepath.Join("agent", "gates", phaseType, baseName)

		// Generate human-readable name
		gateName := strings.Title(strings.ReplaceAll(baseName, "_", " "))

		// Try to read gate name from frontmatter if it's an .md file
		if extension == ".md" {
			filePath := filepath.Join(gatesDir, entry.Name())
			if content, err := os.ReadFile(filePath); err == nil {
				if fm, _, err := frontmatter.Parse(content); err == nil && fm != nil && fm.Name != "" {
					gateName = fm.Name
				}
			}
		}

		gates = append(gates, GateTask{
			ID:       gateID,
			Template: templatePath,
			Name:     gateName,
			Enabled:  true,
		})
	}

	if len(gates) == 0 {
		return nil, errors.Validation("no gate templates found in directory").
			WithField("phase_type", phaseType).
			WithField("path", gatesDir)
	}

	return gates, nil
}

// inferGateType infers the gate type from the gate ID.
func inferGateType(gateID string) frontmatter.GateType {
	lower := strings.ToLower(gateID)
	switch {
	case strings.Contains(lower, "testing") || strings.Contains(lower, "test") || strings.Contains(lower, "verify"):
		return frontmatter.GateTesting
	case strings.Contains(lower, "review"):
		return frontmatter.GateReview
	case strings.Contains(lower, "iterate") || strings.Contains(lower, "iteration"):
		return frontmatter.GateIterate
	case strings.Contains(lower, "security"):
		return frontmatter.GateSecurity
	case strings.Contains(lower, "performance") || strings.Contains(lower, "perf"):
		return frontmatter.GatePerformance
	default:
		return frontmatter.GateTesting
	}
}

// GatesConfig represents the structure of a gates.yaml file.
type GatesConfig struct {
	Version   int              `yaml:"version"`
	PhaseType string           `yaml:"phase_type"`
	Gates     []GatesYamlEntry `yaml:"gates"`
}

// GatesYamlEntry represents a single gate entry in gates.yaml.
type GatesYamlEntry struct {
	ID       string `yaml:"id"`
	Template string `yaml:"template"`
	Name     string `yaml:"name"`
	Enabled  bool   `yaml:"enabled"`
}

// LoadGatesYaml loads gate configuration from a gates.yaml file for a specific phase type.
// It looks for the file at templateRoot/agent/gates/{phaseType}/gates.yaml.
func LoadGatesYaml(templateRoot, phaseType string) ([]GateTask, error) {
	// Build path to gates.yaml
	gatesYamlPath := filepath.Join(templateRoot, "agent", "gates", phaseType, "gates.yaml")

	// Check if file exists
	if _, err := os.Stat(gatesYamlPath); os.IsNotExist(err) {
		return nil, errors.NotFound("gates.yaml file").
			WithField("phase_type", phaseType).
			WithField("path", gatesYamlPath)
	}

	// Read file
	data, err := os.ReadFile(gatesYamlPath)
	if err != nil {
		return nil, errors.IO("reading gates.yaml", err).
			WithField("path", gatesYamlPath)
	}

	// Parse YAML
	var config GatesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Parse("parsing gates.yaml", err).
			WithField("path", gatesYamlPath)
	}

	// Convert to GateTask slice
	var gates []GateTask
	for _, entry := range config.Gates {
		if !entry.Enabled {
			continue
		}

		// Build full template path: agent/gates/{phaseType}/{template}
		templatePath := filepath.Join("agent", "gates", phaseType, entry.Template)

		gates = append(gates, GateTask{
			ID:       entry.ID,
			Template: templatePath,
			Name:     entry.Name,
			Enabled:  entry.Enabled,
		})
	}

	if len(gates) == 0 {
		return nil, errors.Validation("no enabled gates in gates.yaml").
			WithField("phase_type", phaseType).
			WithField("path", gatesYamlPath)
	}

	return gates, nil
}
