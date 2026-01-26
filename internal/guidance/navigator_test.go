package guidance

import (
	"context"
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrMissingGuidanceContext", ErrMissingGuidanceContext, "guidance context is required"},
		{"ErrNotInitialized", ErrNotInitialized, "navigator not initialized, call Initialize() first"},
		{"ErrNoNextStep", ErrNoNextStep, "no next step available"},
		{"ErrAlreadyComplete", ErrAlreadyComplete, "all steps are complete"},
		{"ErrStepNotFound", ErrStepNotFound, "step not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("%s.Error() = %v, want %v", tt.name, tt.err.Error(), tt.want)
			}
		})
	}
}

func TestIsNoNextStep(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"ErrNoNextStep", ErrNoNextStep, true},
		{"ErrNotInitialized", ErrNotInitialized, false},
		{"other error", errors.New("some error"), false},
		{"wrapped ErrNoNextStep", &guidanceError{msg: ErrNoNextStep.msg}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNoNextStep(tt.err); got != tt.want {
				t.Errorf("IsNoNextStep(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsNotInitialized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"ErrNotInitialized", ErrNotInitialized, true},
		{"ErrNoNextStep", ErrNoNextStep, false},
		{"other error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotInitialized(tt.err); got != tt.want {
				t.Errorf("IsNotInitialized(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsAlreadyComplete(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"ErrAlreadyComplete", ErrAlreadyComplete, true},
		{"ErrNoNextStep", ErrNoNextStep, false},
		{"other error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAlreadyComplete(tt.err); got != tt.want {
				t.Errorf("IsAlreadyComplete(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsStepNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"ErrStepNotFound", ErrStepNotFound, true},
		{"ErrNoNextStep", ErrNoNextStep, false},
		{"other error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStepNotFound(tt.err); got != tt.want {
				t.Errorf("IsStepNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestNewBaseNavigator(t *testing.T) {
	tests := []struct {
		name    string
		gctx    *GuidanceContext
		mode    Mode
		wantErr bool
	}{
		{
			name:    "nil context returns error",
			gctx:    nil,
			mode:    ModeExecute,
			wantErr: true,
		},
		{
			name: "valid context creates navigator",
			gctx: &GuidanceContext{
				FestivalPath: "/path/to/festival",
			},
			mode:    ModeExecute,
			wantErr: false,
		},
		{
			name: "nil config gets defaulted",
			gctx: &GuidanceContext{
				FestivalPath: "/path/to/festival",
				Config:       nil,
			},
			mode:    ModeExecute,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav, err := NewBaseNavigator(tt.gctx, tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBaseNavigator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if nav == nil {
					t.Fatal("Expected navigator to be created")
				}
				if nav.Ctx.Config == nil {
					t.Error("Config should be defaulted if nil")
				}
				if nav.StateManager == nil {
					t.Error("StateManager should be created")
				}
			}
		})
	}
}

func TestBaseNavigator_Initialize(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: t.TempDir(),
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	// Initially not initialized
	if nav.IsInitialized() {
		t.Error("Navigator should not be initialized before calling Initialize()")
	}

	// Initialize
	if err := nav.Initialize(context.Background()); err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	// Now should be initialized
	if !nav.IsInitialized() {
		t.Error("Navigator should be initialized after calling Initialize()")
	}
}

func TestBaseNavigator_Reset(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: t.TempDir(),
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Set some state
	nav.StateManager.SetTaskStatus("task1", StatusCompleted)
	if err := nav.Save(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Reset
	if err := nav.Reset(context.Background()); err != nil {
		t.Errorf("Reset() error = %v", err)
	}

	// Should no longer be initialized
	if nav.IsInitialized() {
		t.Error("Navigator should not be initialized after Reset()")
	}
}

func TestBaseNavigator_GetState(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: t.TempDir(),
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	state := nav.GetState()
	if state == nil {
		t.Error("GetState() should not return nil")
	}
}

func TestBaseNavigator_GetConfig(t *testing.T) {
	config := &GuidanceConfig{
		DryRun:   true,
		MaxTasks: 10,
	}
	gctx := &GuidanceContext{
		FestivalPath: "/path/to/festival",
		Config:       config,
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	got := nav.GetConfig()
	if got != config {
		t.Error("GetConfig() should return the same config")
	}
	if !got.DryRun {
		t.Error("Config.DryRun should be true")
	}
}

func TestBaseNavigator_GetContext(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: "/path/to/festival",
		PhaseName:    "Implementation",
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	got := nav.GetContext()
	if got != gctx {
		t.Error("GetContext() should return the same context")
	}
	if got.PhaseName != "Implementation" {
		t.Error("Context.PhaseName should be preserved")
	}
}

func TestBaseNavigator_ShouldDemote(t *testing.T) {
	tests := []struct {
		name       string
		config     *GuidanceConfig
		step       *NextStep
		wantDemote bool
	}{
		{
			name:       "nil step returns false",
			config:     &GuidanceConfig{AutoDemote: true},
			step:       nil,
			wantDemote: false,
		},
		{
			name:       "auto demote disabled returns false",
			config:     &GuidanceConfig{AutoDemote: false},
			step:       &NextStep{AutonomyLevel: AutonomyLow},
			wantDemote: false,
		},
		{
			name:       "nil config gets defaulted with auto demote enabled",
			config:     nil, // NewBaseNavigator will set DefaultConfig which has AutoDemote: true
			step:       &NextStep{AutonomyLevel: AutonomyLow},
			wantDemote: true, // DefaultConfig has AutoDemote: true
		},
		{
			name:       "low autonomy with auto demote returns true",
			config:     &GuidanceConfig{AutoDemote: true},
			step:       &NextStep{AutonomyLevel: AutonomyLow},
			wantDemote: true,
		},
		{
			name:       "medium autonomy does not demote",
			config:     &GuidanceConfig{AutoDemote: true},
			step:       &NextStep{AutonomyLevel: AutonomyMedium},
			wantDemote: false,
		},
		{
			name:       "high autonomy does not demote",
			config:     &GuidanceConfig{AutoDemote: true},
			step:       &NextStep{AutonomyLevel: AutonomyHigh},
			wantDemote: false,
		},
		{
			name:       "gate with review at gates returns true",
			config:     &GuidanceConfig{AutoDemote: true, ReviewAtGates: true},
			step:       &NextStep{StepType: StepTypeGate},
			wantDemote: true,
		},
		{
			name:       "gate without review at gates returns false",
			config:     &GuidanceConfig{AutoDemote: true, ReviewAtGates: false},
			step:       &NextStep{StepType: StepTypeGate},
			wantDemote: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gctx := &GuidanceContext{
				FestivalPath: "/path/to/festival",
				Config:       tt.config,
			}
			nav, err := NewBaseNavigator(gctx, ModeExecute)
			if err != nil {
				t.Fatal(err)
			}

			got := nav.ShouldDemote(tt.step)
			if got != tt.wantDemote {
				t.Errorf("ShouldDemote() = %v, want %v", got, tt.wantDemote)
			}
		})
	}
}

func TestBaseNavigator_MarkTaskStatus(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: t.TempDir(),
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize first to load state
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Mark a task as complete
	if err := nav.MarkTaskStatus(context.Background(), "task1", StatusCompleted); err != nil {
		t.Errorf("MarkTaskStatus() error = %v", err)
	}

	// Verify it was set
	status := nav.StateManager.GetTaskStatus("task1")
	if status != StatusCompleted {
		t.Errorf("GetTaskStatus() = %v, want %v", status, StatusCompleted)
	}
}

func TestBaseNavigator_EnsureInitialized(t *testing.T) {
	gctx := &GuidanceContext{
		FestivalPath: t.TempDir(),
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	// Before initialize, should return error
	if err := nav.EnsureInitialized(); err != ErrNotInitialized {
		t.Errorf("EnsureInitialized() before Initialize() = %v, want ErrNotInitialized", err)
	}

	// Initialize
	if err := nav.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}

	// After initialize, should return nil
	if err := nav.EnsureInitialized(); err != nil {
		t.Errorf("EnsureInitialized() after Initialize() = %v, want nil", err)
	}
}

func TestBaseNavigator_ModeSet(t *testing.T) {
	// Test that mode is set correctly when not already set
	gctx := &GuidanceContext{
		FestivalPath: "/path/to/festival",
		Mode:         "", // empty mode
	}
	nav, err := NewBaseNavigator(gctx, ModeExecute)
	if err != nil {
		t.Fatal(err)
	}

	if nav.Ctx.Mode != ModeExecute {
		t.Errorf("Mode = %v, want %v", nav.Ctx.Mode, ModeExecute)
	}
}

func TestGuidanceError_Error(t *testing.T) {
	err := &guidanceError{msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() = %v, want 'test error'", err.Error())
	}
}
