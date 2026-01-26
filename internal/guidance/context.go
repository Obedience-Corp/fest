package guidance

import (
	"path/filepath"
	"time"
)

// GuidanceContext holds all context needed for guidance operations.
// It is passed to navigator constructors and used throughout guidance.
type GuidanceContext struct {
	// Festival information
	FestivalPath string `json:"festival_path"`
	FestivalName string `json:"festival_name"`
	FestivalID   string `json:"festival_id"`

	// Current position in hierarchy
	PhasePath      string `json:"phase_path,omitempty"`
	PhaseName      string `json:"phase_name,omitempty"`
	PhaseType      string `json:"phase_type,omitempty"`
	PhaseNumber    int    `json:"phase_number,omitempty"`
	SequencePath   string `json:"sequence_path,omitempty"`
	SequenceName   string `json:"sequence_name,omitempty"`
	SequenceNumber int    `json:"sequence_number,omitempty"`
	TaskPath       string `json:"task_path,omitempty"`
	TaskName       string `json:"task_name,omitempty"`

	// Mode and configuration
	Mode   Mode            `json:"mode"`
	Config *GuidanceConfig `json:"config,omitempty"`
}

// GuidanceConfig holds configuration for guidance behavior.
type GuidanceConfig struct {
	// AutoDemote switches to human-guided mode on low autonomy tasks.
	AutoDemote bool `json:"auto_demote" yaml:"auto_demote"`

	// ReviewAtGates pauses execution at quality gates for human review.
	ReviewAtGates bool `json:"review_at_gates" yaml:"review_at_gates"`

	// MaxTasks limits tasks per loop iteration (0 = unlimited).
	MaxTasks int `json:"max_tasks" yaml:"max_tasks"`

	// DryRun enables preview mode without making changes.
	DryRun bool `json:"dry_run" yaml:"dry_run"`

	// ClaimTimeout is the duration for task claiming in orchestrated mode.
	ClaimTimeout time.Duration `json:"claim_timeout" yaml:"claim_timeout"`

	// OutputFormat controls output formatting (text, json, quiet).
	OutputFormat string `json:"output_format" yaml:"output_format"`
}

// DefaultConfig returns sensible defaults for guidance configuration.
func DefaultConfig() *GuidanceConfig {
	return &GuidanceConfig{
		AutoDemote:    true,
		ReviewAtGates: true,
		MaxTasks:      0,
		DryRun:        false,
		ClaimTimeout:  5 * time.Minute,
		OutputFormat:  "text",
	}
}

// NewGuidanceContext creates a new GuidanceContext with defaults.
func NewGuidanceContext(festivalPath string) *GuidanceContext {
	return &GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: filepath.Base(festivalPath),
		Mode:         ModeImplementation, // Default mode
		Config:       DefaultConfig(),
	}
}

// WithPhase returns a copy with phase information set.
func (c *GuidanceContext) WithPhase(phasePath, phaseName, phaseType string, phaseNum int) *GuidanceContext {
	copy := *c
	copy.PhasePath = phasePath
	copy.PhaseName = phaseName
	copy.PhaseType = phaseType
	copy.PhaseNumber = phaseNum
	copy.Mode = ModeFromPhaseType(phaseType)
	return &copy
}

// WithSequence returns a copy with sequence information set.
func (c *GuidanceContext) WithSequence(seqPath, seqName string, seqNum int) *GuidanceContext {
	copy := *c
	copy.SequencePath = seqPath
	copy.SequenceName = seqName
	copy.SequenceNumber = seqNum
	return &copy
}

// WithConfig returns a copy with the given config.
func (c *GuidanceContext) WithConfig(config *GuidanceConfig) *GuidanceContext {
	copy := *c
	copy.Config = config
	return &copy
}

// WithMode returns a copy with the given mode.
func (c *GuidanceContext) WithMode(mode Mode) *GuidanceContext {
	copy := *c
	copy.Mode = mode
	return &copy
}

// CurrentLocation returns a human-readable location string.
func (c *GuidanceContext) CurrentLocation() string {
	if c.TaskPath != "" {
		return c.TaskPath
	}
	if c.SequencePath != "" {
		return c.SequencePath
	}
	if c.PhasePath != "" {
		return c.PhasePath
	}
	return c.FestivalPath
}

// Validate checks that required fields are set.
func (c *GuidanceContext) Validate() error {
	if c.FestivalPath == "" {
		return ErrMissingFestivalPath
	}
	if c.Config == nil {
		c.Config = DefaultConfig()
	}
	return nil
}

// IsValid returns true if the context is valid.
func (c *GuidanceContext) IsValid() bool {
	return c.Validate() == nil
}

// Sentinel errors for context validation
var (
	ErrMissingFestivalPath = &validationError{msg: "festival path is required"}
)

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}
