package config

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

const (
	// FestivalConfigFileName is the name of the festival-level config file
	FestivalConfigFileName = "fest.yaml"
)

// FestivalConfig represents per-festival configuration
type FestivalConfig struct {
	Version          string              `yaml:"version"`
	Metadata         FestivalMetadata    `yaml:"metadata,omitempty"`
	ProjectPath      string              `yaml:"project_path,omitempty"` // Path to linked project directory
	TypeConfig       *TypeConfigMetadata `yaml:"type_config,omitempty"`  // Type-specific configuration
	QualityGates     QualityGatesConfig  `yaml:"quality_gates"`
	ExcludedPatterns []string            `yaml:"excluded_patterns"`
	Templates        TemplatePrefs       `yaml:"templates"`
	Tracking         TrackingConfig      `yaml:"tracking"`
	Agent            AgentConfig         `yaml:"agent,omitempty"`
	AutoLink         AutoLinkConfig      `yaml:"auto_link,omitempty"`
	RitualConfig     *RitualConfig       `yaml:"ritual_config,omitempty"`
}

// RitualConfig holds configuration specific to ritual (repeatable) festivals.
type RitualConfig struct {
	Schedule string `yaml:"schedule,omitempty"`  // "weekly", "daily", "monthly", "manual", or cron expression
	LastRun  string `yaml:"last_run,omitempty"`  // ISO date of last run (e.g., "2026-02-10")
	RunCount int    `yaml:"run_count,omitempty"` // Total runs (decimal for human readability)
}

// TypeConfigMetadata holds festival type-specific configuration recorded at creation.
type TypeConfigMetadata struct {
	AutoPhases    []string       `yaml:"auto_phases,omitempty"`    // Phases auto-scaffolded at creation
	PendingPhases []PendingPhase `yaml:"pending_phases,omitempty"` // Phases to be created later
	SkipIngestion bool           `yaml:"skip_ingestion,omitempty"` // For quick type: skip ingest phase
}

// PendingPhase describes a phase that should be created later.
type PendingPhase struct {
	Name      string `yaml:"name"`                // Phase name (e.g., "IMPLEMENT")
	Type      string `yaml:"type"`                // Phase type (e.g., "implement")
	Role      string `yaml:"role,omitempty"`      // Agent role for this phase
	Trigger   string `yaml:"trigger,omitempty"`   // When to create (e.g., "manual", "auto")
	Generator string `yaml:"generator,omitempty"` // How to create (e.g., "phase_scaffold", "template")
}

// QualityGatesConfig contains quality gate settings.
// Only implementation phases have quality gates.
type QualityGatesConfig struct {
	Enabled        bool              `yaml:"enabled"`
	AutoAppend     bool              `yaml:"auto_append"`
	Tasks          []QualityGateTask `yaml:"tasks,omitempty"`          // Legacy: implementation gates only
	Implementation []QualityGateTask `yaml:"implementation,omitempty"` // Implementation phase gates
}

// QualityGateTask represents a single quality gate task configuration
type QualityGateTask struct {
	ID             string                 `yaml:"id"`
	Template       string                 `yaml:"template"`
	Name           string                 `yaml:"name,omitempty"`
	Enabled        bool                   `yaml:"enabled"`
	Customizations map[string]interface{} `yaml:"customizations,omitempty"`
}

// TemplatePrefs contains template preference settings
type TemplatePrefs struct {
	TaskDefault  string `yaml:"task_default"`
	PreferSimple bool   `yaml:"prefer_simple"`
}

// TrackingConfig contains file tracking settings
type TrackingConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ChecksumFile string `yaml:"checksum_file"`
}

// AutoLinkConfig controls auto-link validation behavior.
// When enabled, implementation sequences must declare a fest_working_dir.
type AutoLinkConfig struct {
	Enabled            bool     `yaml:"enabled"`
	RequireOnPhases    []string `yaml:"require_on_phases"`
	ValidatePathExists bool     `yaml:"validate_path_exists"`

	present               bool `yaml:"-"`
	enabledSet            bool `yaml:"-"`
	validatePathExistsSet bool `yaml:"-"`
}

// IsZero implements the yaml.IsZeroer interface so that omitempty only drops
// the auto_link section when it was never explicitly present in the source YAML.
// Without this, setting enabled: false (a zero value) causes the entire section
// to be silently dropped on re-save, reverting to defaults on next load.
func (c AutoLinkConfig) IsZero() bool {
	return !c.present
}

// DefaultAutoLinkConfig returns the default auto-link configuration.
// Auto-link is enabled by default, requiring fest_working_dir on implementation phases.
func DefaultAutoLinkConfig() AutoLinkConfig {
	return AutoLinkConfig{
		Enabled:            true,
		RequireOnPhases:    []string{"implementation"},
		ValidatePathExists: true,
	}
}

// UnmarshalYAML preserves whether the auto_link section was present and which
// individual fields were explicitly set, so defaults do not overwrite an
// intentional `enabled: false`.
func (c *AutoLinkConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawAutoLink struct {
		Enabled            *bool    `yaml:"enabled"`
		RequireOnPhases    []string `yaml:"require_on_phases"`
		ValidatePathExists *bool    `yaml:"validate_path_exists"`
	}

	var raw rawAutoLink
	if err := value.Decode(&raw); err != nil {
		return err
	}

	c.present = true
	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
		c.enabledSet = true
	}
	c.RequireOnPhases = raw.RequireOnPhases
	if raw.ValidatePathExists != nil {
		c.ValidatePathExists = *raw.ValidatePathExists
		c.validatePathExistsSet = true
	}

	return nil
}

// LoadFestivalConfig loads festival configuration from fest.yaml.
// When campaignRoot is non-empty, campaign-relative paths in the config
// are resolved to absolute for in-memory usage.
func LoadFestivalConfig(festivalPath, campaignRoot string) (*FestivalConfig, error) {
	configPath := filepath.Join(festivalPath, FestivalConfigFileName)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultFestivalConfig(), nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.IO("reading festival config", err).WithField("path", configPath)
	}

	// Parse YAML
	var cfg FestivalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Parse("parsing festival config", err).WithField("path", configPath)
	}

	// Apply defaults for missing values
	applyFestivalDefaults(&cfg)

	// Resolve campaign-relative paths to absolute for in-memory usage
	if campaignRoot != "" {
		if cfg.ProjectPath != "" {
			cfg.ProjectPath = pathutil.ToAbsolutePath(cfg.ProjectPath, campaignRoot)
		}
		for i := range cfg.Metadata.StatusHistory {
			if cfg.Metadata.StatusHistory[i].Path != "" {
				cfg.Metadata.StatusHistory[i].Path = pathutil.ToAbsolutePath(cfg.Metadata.StatusHistory[i].Path, campaignRoot)
			}
		}
	}

	return &cfg, nil
}

// SaveFestivalConfig saves festival configuration to fest.yaml.
// When campaignRoot is non-empty, paths are converted to campaign-relative before persistence.
func SaveFestivalConfig(festivalPath, campaignRoot string, cfg *FestivalConfig) error {
	configPath := filepath.Join(festivalPath, FestivalConfigFileName)

	// Create a shallow copy to avoid mutating the in-memory struct
	saveCopy := *cfg

	// Convert paths to campaign-relative for persistence
	if campaignRoot != "" {
		if saveCopy.ProjectPath != "" {
			saveCopy.ProjectPath = pathutil.ToRelativePath(saveCopy.ProjectPath, campaignRoot)
		}
		if len(saveCopy.Metadata.StatusHistory) > 0 {
			history := make([]StatusChange, len(saveCopy.Metadata.StatusHistory))
			copy(history, saveCopy.Metadata.StatusHistory)
			for i := range history {
				if history[i].Path != "" {
					history[i].Path = pathutil.ToRelativePath(history[i].Path, campaignRoot)
				}
			}
			saveCopy.Metadata.StatusHistory = history
		}
	}

	// Marshal to YAML
	data, err := yamlutil.Marshal(&saveCopy)
	if err != nil {
		return errors.Wrap(err, "marshaling festival config")
	}

	// Write file
	if err := os.WriteFile(configPath, data, filePermissions); err != nil {
		return errors.IO("writing festival config", err).WithField("path", configPath)
	}

	return nil
}

// DefaultFestivalConfig returns the default festival configuration.
// Note: Template paths reference the festival's gates/ directory.
func DefaultFestivalConfig() *FestivalConfig {
	return &FestivalConfig{
		Version: "1.0",
		QualityGates: QualityGatesConfig{
			Enabled:    true,
			AutoAppend: true,
			Implementation: []QualityGateTask{
				{ID: "testing", Template: "gates/implementation/QUALITY_GATE_TESTING", Name: "Testing and Verification", Enabled: true},
				{ID: "review", Template: "gates/implementation/QUALITY_GATE_REVIEW", Name: "Code Review", Enabled: true},
				{ID: "iterate", Template: "gates/implementation/QUALITY_GATE_ITERATE", Name: "Review Results and Iterate", Enabled: true},
				{ID: "fest-commit", Template: "gates/implementation/QUALITY_GATE_FEST_COMMIT", Name: "Fest Commit Changes", Enabled: true},
			},
		},
		ExcludedPatterns: []string{
			"*_planning",
			"*_research",
			"*_requirements",
			"*_docs",
		},
		Templates: TemplatePrefs{
			TaskDefault:  "tasks/SIMPLE",
			PreferSimple: true,
		},
		Tracking: TrackingConfig{
			Enabled:      true,
			ChecksumFile: ".festival-checksums.json",
		},
	}
}

// applyFestivalDefaults applies default values to missing configuration fields
func applyFestivalDefaults(cfg *FestivalConfig) {
	defaults := DefaultFestivalConfig()
	autoLinkDefaults := DefaultAutoLinkConfig()

	if cfg.Version == "" {
		cfg.Version = defaults.Version
	}

	// Default festival type to "standard" if metadata has no type set
	if cfg.Metadata.FestivalType == "" {
		cfg.Metadata.FestivalType = "standard"
	}

	// Apply defaults for phase-specific gates if not defined
	// Note: We don't fill in defaults for phase gates - if not defined in fest.yaml,
	// they simply won't have gates applied. This is intentional.

	// If no excluded patterns, use defaults
	if len(cfg.ExcludedPatterns) == 0 {
		cfg.ExcludedPatterns = defaults.ExcludedPatterns
	}

	if cfg.Templates.TaskDefault == "" {
		cfg.Templates.TaskDefault = defaults.Templates.TaskDefault
	}

	if cfg.Tracking.ChecksumFile == "" {
		cfg.Tracking.ChecksumFile = defaults.Tracking.ChecksumFile
	}

	// Apply auto-link defaults when the section is absent from fest.yaml.
	if !cfg.AutoLink.present {
		cfg.AutoLink = autoLinkDefaults
		return
	}

	if !cfg.AutoLink.enabledSet {
		cfg.AutoLink.Enabled = autoLinkDefaults.Enabled
	}
	if cfg.AutoLink.RequireOnPhases == nil && cfg.AutoLink.Enabled {
		cfg.AutoLink.RequireOnPhases = autoLinkDefaults.RequireOnPhases
	}
	if !cfg.AutoLink.validatePathExistsSet && cfg.AutoLink.Enabled {
		cfg.AutoLink.ValidatePathExists = autoLinkDefaults.ValidatePathExists
	}
}

// IsSequenceExcluded checks if a sequence name matches any excluded pattern
func (cfg *FestivalConfig) IsSequenceExcluded(sequenceName string) bool {
	for _, pattern := range cfg.ExcludedPatterns {
		matched, err := filepath.Match(pattern, sequenceName)
		if err != nil {
			continue // Skip invalid patterns
		}
		if matched {
			return true
		}
	}
	return false
}

// GetEnabledTasks returns only enabled quality gate tasks
func (cfg *FestivalConfig) GetEnabledTasks() []QualityGateTask {
	var enabled []QualityGateTask
	for _, task := range cfg.QualityGates.Tasks {
		if task.Enabled {
			enabled = append(enabled, task)
		}
	}
	return enabled
}

// GetGatesForPhaseType returns configured gates for implementation phases.
// Only implementation phases have quality gates. Returns nil for all other phase types.
// Falls back to Tasks field for backwards compatibility.
func (cfg *FestivalConfig) GetGatesForPhaseType(phaseType string) []QualityGateTask {
	if phaseType != "implementation" {
		return nil
	}

	gates := cfg.QualityGates.Implementation
	// Fallback to legacy Tasks field for backwards compatibility
	if len(gates) == 0 && len(cfg.QualityGates.Tasks) > 0 {
		gates = cfg.QualityGates.Tasks
	}

	// Filter to enabled only
	var enabled []QualityGateTask
	for _, gate := range gates {
		if gate.Enabled {
			enabled = append(enabled, gate)
		}
	}
	return enabled
}

// FestivalConfigExists checks if a fest.yaml file exists in the given path
func FestivalConfigExists(festivalPath string) bool {
	configPath := filepath.Join(festivalPath, FestivalConfigFileName)
	_, err := os.Stat(configPath)
	return err == nil
}

// HasPendingPhases returns true if the festival has phases pending creation.
func (cfg *FestivalConfig) HasPendingPhases() bool {
	return cfg.TypeConfig != nil && len(cfg.TypeConfig.PendingPhases) > 0
}

// GetPendingPhaseByName finds a pending phase by name.
func (cfg *FestivalConfig) GetPendingPhaseByName(name string) (*PendingPhase, bool) {
	if cfg.TypeConfig == nil {
		return nil, false
	}
	for i := range cfg.TypeConfig.PendingPhases {
		if cfg.TypeConfig.PendingPhases[i].Name == name {
			return &cfg.TypeConfig.PendingPhases[i], true
		}
	}
	return nil, false
}

// IsAutoPhase checks if a phase name was auto-scaffolded at creation.
func (cfg *FestivalConfig) IsAutoPhase(phaseName string) bool {
	if cfg.TypeConfig == nil {
		return false
	}
	for _, name := range cfg.TypeConfig.AutoPhases {
		if name == phaseName {
			return true
		}
	}
	return false
}

// RemovePendingPhase removes a phase from the pending list (e.g., after creation).
func (cfg *FestivalConfig) RemovePendingPhase(name string) {
	if cfg.TypeConfig == nil {
		return
	}
	filtered := make([]PendingPhase, 0, len(cfg.TypeConfig.PendingPhases))
	for _, phase := range cfg.TypeConfig.PendingPhases {
		if phase.Name != name {
			filtered = append(filtered, phase)
		}
	}
	cfg.TypeConfig.PendingPhases = filtered
}
