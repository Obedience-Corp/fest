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

	// Find highest task number to continue numbering from
	maxNum := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if entry.Name() == "SEQUENCE_GOAL.md" {
			continue
		}
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

		// Check if this configured gate ID already exists in the sequence.
		existingPath, restamped, err := findExistingGate(entries, sequencePath, gate.ID, !opts.DryRun)
		if err != nil {
			return nil, nil, errors.Wrap(err, "finding existing gate").
				WithField("gate_id", gate.ID).
				WithField("sequence_path", sequencePath)
		}
		if existingPath != "" {
			if restamped {
				warnings = append(warnings, fmt.Sprintf("Backfilled fest_gate_id for %s", existingPath))
			}
			if !opts.Force {
				results = append(results, GenerateResult{
					Type:     "skip",
					Path:     existingPath,
					Template: gate.Template,
					TaskID:   gate.ID,
					Reason:   "gate_exists",
				})
				continue
			}
			// With --force, overwrite the existing file instead of creating new
			taskPath = existingPath
		}

		// Create the task
		if !opts.DryRun {
			var festPath string
			if len(festivalPath) > 0 {
				festPath = festivalPath[0]
			}

			content, err := g.renderGateContent(ctx, gate, taskNum, festPath)
			if err != nil {
				return nil, nil, errors.Wrap(err, "rendering gate content").
					WithField("gate_id", gate.ID).
					WithField("task_path", taskPath)
			}

			// Inject frontmatter if content doesn't already have it
			if !strings.HasPrefix(strings.TrimSpace(content), "---") {
				parentSequenceID := filepath.Base(sequencePath)
				gateType := inferGateType(gate.ID)
				fm := frontmatter.NewGateFrontmatter(taskFileName, gate.Name, parentSequenceID, taskNum, gate.ID, gateType, gateTypeAutonomy(gateType))
				contentWithFM, fmErr := frontmatter.InjectString(content, fm)
				if fmErr != nil {
					warnings = append(warnings, fmt.Sprintf("Failed to inject frontmatter for %s: %v", taskPath, fmErr))
				} else {
					content = contentWithFM
				}
			}

			content = AddMarkers(content, gate.ID)

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

// findExistingGate checks if a configured gate already exists in the sequence.
// It prefers explicit fest_gate_id matching and uses an exact filename-based restamp
// only for legacy gate files that predate fest_gate_id markers.
func findExistingGate(entries []os.DirEntry, sequencePath, gateID string, persistLegacyID bool) (string, bool, error) {
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		filePath := filepath.Join(sequencePath, name)
		if GetGateID(filePath) == gateID {
			return filePath, false, nil
		}
		if matchesLegacyGateID(filePath, gateID) {
			if persistLegacyID {
				if err := stampGateID(filePath, gateID); err != nil {
					return "", false, err
				}
			}
			return filePath, persistLegacyID, nil
		}
	}
	return "", false, nil
}

func matchesLegacyGateID(filePath, gateID string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil || fm.Type != frontmatter.TypeGate || fm.GateID != "" {
		return false
	}

	return extractTaskStem(filepath.Base(filePath)) == configuredGateStem(gateID)
}

func stampGateID(filePath, gateID string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	updated := AddMarkers(string(content), gateID)
	return os.WriteFile(filePath, []byte(updated), 0644)
}

func configuredGateStem(gateID string) string {
	return extractTaskStem(tpl.FormatTaskID(1, gateID))
}

func extractTaskStem(filename string) string {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(filename)), ".md")
	if len(name) >= 3 && isDigit(name[0]) && isDigit(name[1]) {
		switch name[2] {
		case '_', '-', '.', ' ':
			return name[3:]
		}
	}

	return name
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// renderGateContent renders the content for a gate task file.
// Loads directly from the festival's gates/ directory - no fallbacks.
// Returns an error if the gate file cannot be found.
func (g *TaskGenerator) renderGateContent(ctx context.Context, gate GateTask, taskNum int, festivalPath string) (string, error) {
	if festivalPath == "" {
		return "", errors.Validation("festivalPath required for gate rendering").
			WithField("gate_id", gate.ID)
	}

	// Build template context
	tmplCtx := tpl.NewContext()
	tmplCtx.SetTask(taskNum, gate.ID)
	if gate.Customizations != nil {
		for k, v := range gate.Customizations {
			tmplCtx.SetCustom(k, v)
		}
	}

	// Extract phase type and gate name from template path
	phaseType, gateName := extractPhaseAndGate(gate.Template)
	if phaseType == "" || gateName == "" {
		return "", errors.Validation("invalid gate template path").
			WithField("template", gate.Template).
			WithField("gate_id", gate.ID)
	}

	// Load from festival's gates directory - this MUST exist
	gatesPath := filepath.Join(festivalPath, "gates", phaseType, gateName+".md")
	if _, err := os.Stat(gatesPath); err != nil {
		return "", errors.NotFound("gate file").
			WithField("path", gatesPath).
			WithField("gate_id", gate.ID).
			WithField("phase_type", phaseType)
	}

	content := g.loadAndRenderTemplate(ctx, gatesPath, tmplCtx)
	if content == "" {
		return "", errors.IO("failed to load gate template", nil).
			WithField("path", gatesPath)
	}

	return content, nil
}

// extractPhaseAndGate extracts phase type and gate name from a template path.
// Handles formats: "gates/{phaseType}/{gateName}", "{phaseType}/{gateName}"
func extractPhaseAndGate(templatePath string) (phaseType, gateName string) {
	// Strip common prefixes
	path := templatePath
	for _, prefix := range []string{"gates/", "agent/gates/"} {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}

	// Split into parts: should be {phaseType}/{gateName}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[0], parts[len(parts)-1]
	}

	return "", ""
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
	return matchingExcludePattern(sequenceName, patterns) != ""
}

// matchingExcludePattern returns the first pattern that matches the sequence name,
// or empty string if no pattern matches.
func matchingExcludePattern(sequenceName string, patterns []string) string {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, sequenceName)
		if err != nil {
			continue
		}
		if matched {
			return pattern
		}
	}
	return ""
}

// SequenceInfo contains information about a sequence for generation.
type SequenceInfo struct {
	Path      string // Full path to sequence directory
	PhasePath string // Path to parent phase
	Name      string // Sequence directory name
	PhaseType string // Phase type: "implementation", "planning", "research", "review", "action"
	PhaseName string // Name of the parent phase directory
}

// SkippedSequence records a sequence that was skipped due to an exclude pattern.
type SkippedSequence struct {
	Name    string // Sequence directory name
	Pattern string // The exclude pattern that matched
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
// Returns the included sequences and any sequences skipped by exclude patterns.
func FindSequencesWithInfo(festivalRoot string, excludePatterns []string) ([]SequenceInfo, []SkippedSequence, error) {
	var sequences []SequenceInfo
	var skipped []SkippedSequence

	entries, err := os.ReadDir(festivalRoot)
	if err != nil {
		return nil, nil, errors.IO("reading festival root", err).
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

			if pattern := matchingExcludePattern(seqEntry.Name(), excludePatterns); pattern != "" {
				skipped = append(skipped, SkippedSequence{
					Name:    seqEntry.Name(),
					Pattern: pattern,
				})
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

	return sequences, skipped, nil
}

// gateTypeAutonomy returns the default autonomy level for a gate type.
func gateTypeAutonomy(gt frontmatter.GateType) frontmatter.Autonomy {
	switch gt {
	case frontmatter.GateReview:
		return frontmatter.AutonomyLow
	case frontmatter.GateCommit:
		return frontmatter.AutonomyHigh
	default:
		return frontmatter.AutonomyMedium
	}
}

// inferGateType infers the gate type from the gate ID.
func inferGateType(gateID string) frontmatter.GateType {
	gateType := CanonicalGateType(gateID)
	return gateType
}
