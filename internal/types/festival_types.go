package types

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"gopkg.in/yaml.v3"
)

// FestivalTypesConfig represents the root configuration structure.
type FestivalTypesConfig struct {
	Version int            `yaml:"version"`
	Types   []FestivalType `yaml:"types"`
}

// FestivalType defines a festival type with its phase structure.
type FestivalType struct {
	Name           string      `yaml:"name"`
	Description    string      `yaml:"description"`
	Default        bool        `yaml:"default"`
	SkipIngestion  bool        `yaml:"skip_ingestion"`
	Phases         []PhaseSpec `yaml:"phases"`
	Note           string      `yaml:"note,omitempty"`
}

// PhaseSpec defines a phase within a festival type.
type PhaseSpec struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	Auto      bool   `yaml:"auto"`
	Role      string `yaml:"role,omitempty"`
	Trigger   string `yaml:"trigger,omitempty"`
	Generator string `yaml:"generator,omitempty"`
}

// GetAutoPhases returns phases that are auto-scaffolded.
func (ft *FestivalType) GetAutoPhases() []PhaseSpec {
	auto := make([]PhaseSpec, 0, len(ft.Phases))
	for _, p := range ft.Phases {
		if p.Auto {
			auto = append(auto, p)
		}
	}
	return auto
}

// GetPendingPhases returns phases that require manual creation.
func (ft *FestivalType) GetPendingPhases() []PhaseSpec {
	pending := make([]PhaseSpec, 0, len(ft.Phases))
	for _, p := range ft.Phases {
		if !p.Auto {
			pending = append(pending, p)
		}
	}
	return pending
}

// LoadFestivalTypesConfig loads festival types configuration.
// Resolution order:
//  1. Project .festival/festival_types.yaml (from CWD)
//  2. Global ~/.fest/festival_types.yaml
//  3. Hardcoded default config
func LoadFestivalTypesConfig(ctx context.Context) (*FestivalTypesConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled").
			WithOp("LoadFestivalTypesConfig")
	}

	// Try project-level config
	cwd, err := os.Getwd()
	if err == nil {
		projectPath := filepath.Join(cwd, ".festival", "festival_types.yaml")
		if config, err := loadConfigFromFile(ctx, projectPath); err == nil {
			return config, nil
		}
	}

	// Try global config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(homeDir, ".fest", "festival_types.yaml")
		if config, err := loadConfigFromFile(ctx, globalPath); err == nil {
			return config, nil
		}
	}

	// Fall back to default config
	return getDefaultConfig(), nil
}

// loadConfigFromFile reads and validates a config file.
func loadConfigFromFile(ctx context.Context, path string) (*FestivalTypesConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NotFound("config file").
				WithField("path", path)
		}
		return nil, errors.IO("reading config file", err).
			WithField("path", path)
	}

	var config FestivalTypesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Parse("invalid YAML in festival types config", err).
			WithField("path", path)
	}

	if err := validateConfig(&config); err != nil {
		return nil, errors.Wrap(err, "config validation failed").
			WithField("path", path)
	}

	return &config, nil
}

// validateConfig checks config integrity.
func validateConfig(config *FestivalTypesConfig) error {
	if len(config.Types) == 0 {
		return errors.Validation("no festival types defined")
	}

	seen := make(map[string]bool)
	defaultCount := 0

	for _, ft := range config.Types {
		if ft.Name == "" {
			return errors.Validation("festival type missing name")
		}
		if ft.Description == "" {
			return errors.Validation("festival type missing description").
				WithField("type", ft.Name)
		}

		if seen[ft.Name] {
			return errors.Validation("duplicate festival type name").
				WithField("type", ft.Name)
		}
		seen[ft.Name] = true

		if ft.Default {
			defaultCount++
		}

		// Ritual type may have no phases
		if ft.Name != "ritual" && len(ft.Phases) == 0 {
			return errors.Validation("festival type has no phases").
				WithField("type", ft.Name)
		}
	}

	if defaultCount != 1 {
		return errors.Validation("exactly one default type required").
			WithField("found", defaultCount)
	}

	return nil
}

// getDefaultConfig returns the hardcoded fallback configuration.
func getDefaultConfig() *FestivalTypesConfig {
	return &FestivalTypesConfig{
		Version: 1,
		Types: []FestivalType{
			{
				Name:        "standard",
				Description: "Default festival type with full planning and implementation phases",
				Default:     true,
				Phases: []PhaseSpec{
					{Name: "INGEST", Type: "standard", Auto: true},
					{Name: "PLAN", Type: "standard", Auto: true},
					{Name: "IMPLEMENT", Type: "implementation", Auto: false},
					{Name: "POLISH", Type: "standard", Auto: false},
				},
			},
		},
	}
}

// GetFestivalType finds a festival type by name.
func (c *FestivalTypesConfig) GetFestivalType(name string) (*FestivalType, error) {
	for i := range c.Types {
		if c.Types[i].Name == name {
			return &c.Types[i], nil
		}
	}
	return nil, errors.NotFound("festival type").
		WithField("type", name).
		WithHint("Run 'fest types list' to see available festival types")
}

// GetDefaultType returns the default festival type.
func (c *FestivalTypesConfig) GetDefaultType() *FestivalType {
	for i := range c.Types {
		if c.Types[i].Default {
			return &c.Types[i]
		}
	}
	// Should never happen after validation
	return &c.Types[0]
}
