package guidance

import (
	"errors"
	"testing"
	"time"
)

func TestNewGuidanceContext(t *testing.T) {
	ctx := NewGuidanceContext("/path/to/festival")

	if ctx.FestivalPath != "/path/to/festival" {
		t.Errorf("FestivalPath = %v, want /path/to/festival", ctx.FestivalPath)
	}
	if ctx.FestivalName != "festival" {
		t.Errorf("FestivalName = %v, want festival", ctx.FestivalName)
	}
	if ctx.Mode != ModeExecute {
		t.Errorf("Mode = %v, want ModeExecute", ctx.Mode)
	}
	if ctx.Config == nil {
		t.Error("Config should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if !cfg.AutoDemote {
		t.Error("AutoDemote should default to true")
	}
	if !cfg.ReviewAtGates {
		t.Error("ReviewAtGates should default to true")
	}
	if cfg.MaxTasks != 0 {
		t.Errorf("MaxTasks = %d, want 0", cfg.MaxTasks)
	}
	if cfg.DryRun {
		t.Error("DryRun should default to false")
	}
	if cfg.ClaimTimeout != 5*time.Minute {
		t.Errorf("ClaimTimeout = %v, want 5 minutes", cfg.ClaimTimeout)
	}
	if cfg.OutputFormat != "text" {
		t.Errorf("OutputFormat = %v, want text", cfg.OutputFormat)
	}
}

func TestGuidanceContext_WithPhase(t *testing.T) {
	tests := []struct {
		name      string
		phaseType string
		wantMode  Mode
	}{
		{"implementation phase sets execute mode", "implementation", ModeExecute},
		{"planning phase sets plan mode", "planning", ModePlan},
		{"research phase sets research mode", "research", ModeResearch},
		{"review phase sets review mode", "review", ModeReview},
		{"deployment phase sets action mode", "deployment", ModeAction},
		{"ingest phase sets ingest mode", "ingest", ModeIngest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewGuidanceContext("/path/to/festival")
			ctx := original.WithPhase("/path/to/phase", "Phase1", tt.phaseType, 1)

			// Verify copy semantics - original unchanged
			if original.PhasePath != "" {
				t.Error("WithPhase mutated original context")
			}

			// Verify new context has correct values
			if ctx.PhasePath != "/path/to/phase" {
				t.Errorf("PhasePath = %v, want /path/to/phase", ctx.PhasePath)
			}
			if ctx.PhaseName != "Phase1" {
				t.Errorf("PhaseName = %v, want Phase1", ctx.PhaseName)
			}
			if ctx.PhaseType != tt.phaseType {
				t.Errorf("PhaseType = %v, want %v", ctx.PhaseType, tt.phaseType)
			}
			if ctx.PhaseNumber != 1 {
				t.Errorf("PhaseNumber = %v, want 1", ctx.PhaseNumber)
			}
			if ctx.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", ctx.Mode, tt.wantMode)
			}
		})
	}
}

func TestGuidanceContext_WithSequence(t *testing.T) {
	original := NewGuidanceContext("/path/to/festival")
	ctx := original.WithSequence("/path/to/sequence", "Sequence1", 2)

	// Verify copy semantics - original unchanged
	if original.SequencePath != "" {
		t.Error("WithSequence mutated original context")
	}

	// Verify new context has correct values
	if ctx.SequencePath != "/path/to/sequence" {
		t.Errorf("SequencePath = %v, want /path/to/sequence", ctx.SequencePath)
	}
	if ctx.SequenceName != "Sequence1" {
		t.Errorf("SequenceName = %v, want Sequence1", ctx.SequenceName)
	}
	if ctx.SequenceNumber != 2 {
		t.Errorf("SequenceNumber = %v, want 2", ctx.SequenceNumber)
	}
}

func TestGuidanceContext_WithConfig(t *testing.T) {
	original := NewGuidanceContext("/path/to/festival")
	newConfig := &GuidanceConfig{
		DryRun:   true,
		MaxTasks: 5,
	}
	ctx := original.WithConfig(newConfig)

	// Verify copy semantics - original unchanged
	if original.Config.DryRun {
		t.Error("WithConfig mutated original context")
	}

	// Verify new context has correct config
	if ctx.Config != newConfig {
		t.Error("Config was not set correctly")
	}
	if !ctx.Config.DryRun {
		t.Error("Config.DryRun should be true")
	}
	if ctx.Config.MaxTasks != 5 {
		t.Errorf("Config.MaxTasks = %d, want 5", ctx.Config.MaxTasks)
	}
}

func TestGuidanceContext_WithMode(t *testing.T) {
	original := NewGuidanceContext("/path/to/festival")
	ctx := original.WithMode(ModePlan)

	// Verify copy semantics - original unchanged
	if original.Mode != ModeExecute {
		t.Error("WithMode mutated original context")
	}

	// Verify new context has correct mode
	if ctx.Mode != ModePlan {
		t.Errorf("Mode = %v, want ModePlan", ctx.Mode)
	}
}

func TestGuidanceContext_CurrentLocation(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *GuidanceContext
		wantPath string
	}{
		{
			name: "returns task path when set",
			setup: func() *GuidanceContext {
				ctx := NewGuidanceContext("/path/to/festival")
				ctx.TaskPath = "/path/to/task"
				ctx.SequencePath = "/path/to/sequence"
				ctx.PhasePath = "/path/to/phase"
				return ctx
			},
			wantPath: "/path/to/task",
		},
		{
			name: "returns sequence path when no task",
			setup: func() *GuidanceContext {
				ctx := NewGuidanceContext("/path/to/festival")
				ctx.SequencePath = "/path/to/sequence"
				ctx.PhasePath = "/path/to/phase"
				return ctx
			},
			wantPath: "/path/to/sequence",
		},
		{
			name: "returns phase path when no sequence",
			setup: func() *GuidanceContext {
				ctx := NewGuidanceContext("/path/to/festival")
				ctx.PhasePath = "/path/to/phase"
				return ctx
			},
			wantPath: "/path/to/phase",
		},
		{
			name: "returns festival path when nothing else set",
			setup: func() *GuidanceContext {
				return NewGuidanceContext("/path/to/festival")
			},
			wantPath: "/path/to/festival",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			if got := ctx.CurrentLocation(); got != tt.wantPath {
				t.Errorf("CurrentLocation() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestGuidanceContext_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *GuidanceContext
		wantErr bool
		errIs   error
	}{
		{
			name:    "valid context",
			ctx:     NewGuidanceContext("/path/to/festival"),
			wantErr: false,
		},
		{
			name:    "missing festival path",
			ctx:     &GuidanceContext{},
			wantErr: true,
			errIs:   ErrMissingFestivalPath,
		},
		{
			name: "nil config gets defaulted",
			ctx: &GuidanceContext{
				FestivalPath: "/path/to/festival",
				Config:       nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("Validate() error = %v, want %v", err, tt.errIs)
			}
		})
	}
}

func TestGuidanceContext_Validate_SetsDefaultConfig(t *testing.T) {
	ctx := &GuidanceContext{
		FestivalPath: "/path/to/festival",
		Config:       nil,
	}

	err := ctx.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if ctx.Config == nil {
		t.Error("Validate() should set default config when nil")
	}
}

func TestGuidanceContext_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ctx  *GuidanceContext
		want bool
	}{
		{
			name: "valid context",
			ctx:  NewGuidanceContext("/path/to/festival"),
			want: true,
		},
		{
			name: "invalid context - missing festival path",
			ctx:  &GuidanceContext{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuidanceContext_Chaining(t *testing.T) {
	ctx := NewGuidanceContext("/path/to/festival").
		WithPhase("/path/to/phase", "Phase1", "planning", 1).
		WithSequence("/path/to/sequence", "Sequence1", 1).
		WithConfig(&GuidanceConfig{DryRun: true})

	if ctx.FestivalPath != "/path/to/festival" {
		t.Errorf("FestivalPath = %v, want /path/to/festival", ctx.FestivalPath)
	}
	if ctx.PhasePath != "/path/to/phase" {
		t.Errorf("PhasePath = %v, want /path/to/phase", ctx.PhasePath)
	}
	if ctx.SequencePath != "/path/to/sequence" {
		t.Errorf("SequencePath = %v, want /path/to/sequence", ctx.SequencePath)
	}
	if ctx.Mode != ModePlan {
		t.Errorf("Mode = %v, want ModePlan", ctx.Mode)
	}
	if !ctx.Config.DryRun {
		t.Error("Config.DryRun should be true")
	}
}

func TestValidationError(t *testing.T) {
	if ErrMissingFestivalPath.Error() != "festival path is required" {
		t.Errorf("ErrMissingFestivalPath.Error() = %v, want 'festival path is required'", ErrMissingFestivalPath.Error())
	}
}
