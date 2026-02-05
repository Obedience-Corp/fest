package config

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"gopkg.in/yaml.v3"
)

const (
	// FestivalConfigFileName is the name of the festival-level config file
	FestivalConfigFileName = "fest.yaml"
)

// FestivalConfig represents per-festival configuration
type FestivalConfig struct {
	Version          string             `yaml:"version"`
	Metadata         FestivalMetadata   `yaml:"metadata,omitempty"`
	ProjectPath      string             `yaml:"project_path,omitempty"` // Path to linked project directory
	QualityGates     QualityGatesConfig `yaml:"quality_gates"`
	ExcludedPatterns []string           `yaml:"excluded_patterns"`
	Templates        TemplatePrefs      `yaml:"templates"`
	Tracking         TrackingConfig     `yaml:"tracking"`
	Agent            AgentConfig        `yaml:"agent,omitempty"`
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

// LoadFestivalConfig loads festival configuration from fest.yaml
func LoadFestivalConfig(festivalPath string) (*FestivalConfig, error) {
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

	return &cfg, nil
}

// SaveFestivalConfig saves festival configuration to fest.yaml
func SaveFestivalConfig(festivalPath string, cfg *FestivalConfig) error {
	configPath := filepath.Join(festivalPath, FestivalConfigFileName)

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
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
				{ID: "commit", Template: "gates/implementation/QUALITY_GATE_COMMIT", Name: "Commit Changes", Enabled: true},
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

	if cfg.Version == "" {
		cfg.Version = defaults.Version
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
