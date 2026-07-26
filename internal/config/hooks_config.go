package config

import "gopkg.in/yaml.v3"

// HookDefinition is a single declared hook. Redeclaring a name at a more
// specific layer replaces this whole definition (no field-level merge).
type HookDefinition struct {
	Command  string `yaml:"command" json:"command"`                       // required
	Fail     string `yaml:"fail,omitempty" json:"fail,omitempty"`         // "closed" (default) | "open"
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // e.g. "120s"; parsed in internal/hooks
	Evidence string `yaml:"evidence,omitempty" json:"evidence,omitempty"` // "paths" (default) | "embed"
	Enabled  *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`   // nil = enabled; per-hook switch (D1)
}

// HooksConfig holds optional hooks configuration shared by machine (JSON),
// workspace, and festival YAML layers.
type HooksConfig struct {
	Enabled     *bool                     `yaml:"enabled,omitempty" json:"enabled,omitempty"` // nil = unset (inherit)
	Levels      map[string]bool           `yaml:"levels,omitempty" json:"levels,omitempty"`   // keys: phase, sequence, task
	Definitions map[string]HookDefinition `yaml:"definitions,omitempty" json:"definitions,omitempty"`

	present bool `yaml:"-" json:"-"` // set by UnmarshalYAML; true when the section appeared in source
}

// IsZero drops the hooks section on re-save only when it never appeared in
// source and has no programmatically set content.
func (h HooksConfig) IsZero() bool {
	if h.present {
		return false
	}
	return h.Enabled == nil &&
		len(h.Levels) == 0 &&
		len(h.Definitions) == 0
}

// UnmarshalYAML marks the hooks section as present and preserves tri-state fields.
func (h *HooksConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawHooks struct {
		Enabled     *bool                     `yaml:"enabled"`
		Levels      map[string]bool           `yaml:"levels"`
		Definitions map[string]HookDefinition `yaml:"definitions"`
	}
	var raw rawHooks
	if err := value.Decode(&raw); err != nil {
		return err
	}
	h.present = true
	h.Enabled = raw.Enabled
	h.Levels = raw.Levels
	h.Definitions = raw.Definitions
	return nil
}
