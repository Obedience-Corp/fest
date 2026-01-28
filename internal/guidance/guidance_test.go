package guidance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterNavigator(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	// Clear registry for test
	ClearNavigatorRegistry()

	// Register a factory
	RegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{mode: ModeImplementation}, nil
	})

	// Verify it was registered
	modes := GetRegisteredModes()
	if len(modes) != 1 {
		t.Errorf("Expected 1 mode registered, got %d", len(modes))
	}
	if modes[0] != ModeImplementation {
		t.Errorf("Expected ModeImplementation, got %v", modes[0])
	}
}

func TestMustRegisterNavigator(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	ClearNavigatorRegistry()

	// Test nil factory panics
	t.Run("nil factory panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil factory")
			}
		}()
		MustRegisterNavigator(ModeImplementation, nil)
	})

	ClearNavigatorRegistry()

	// Test duplicate registration panics
	t.Run("duplicate registration panics", func(t *testing.T) {
		MustRegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
			return &mockNavigator{}, nil
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for duplicate registration")
			}
		}()
		MustRegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
			return &mockNavigator{}, nil
		})
	})
}

func TestNewNavigator(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	ClearNavigatorRegistry()
	RegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{mode: ModeImplementation, ctx: gctx}, nil
	})

	tests := []struct {
		name    string
		gctx    *GuidanceContext
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil context returns error",
			gctx:    nil,
			wantErr: true,
			errMsg:  "guidance context required",
		},
		{
			name: "missing festival path returns error",
			gctx: &GuidanceContext{
				FestivalPath: "",
			},
			wantErr: true,
			errMsg:  "invalid guidance context",
		},
		{
			name: "valid context creates navigator",
			gctx: &GuidanceContext{
				FestivalPath: "/path/to/festival",
				PhaseType:    "implementation",
			},
			wantErr: false,
		},
		{
			name: "derives mode from phase type",
			gctx: &GuidanceContext{
				FestivalPath: "/path/to/festival",
				PhaseType:    "implementation",
				Mode:         "", // empty mode should be derived
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav, err := NewNavigator(context.Background(), tt.gctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNavigator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message = %v, want containing %v", err.Error(), tt.errMsg)
				}
			}
			if !tt.wantErr && nav == nil {
				t.Error("Expected navigator to be created")
			}
		})
	}
}

func TestNewNavigator_UnknownMode(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	ClearNavigatorRegistry()
	// Don't register any navigators

	gctx := &GuidanceContext{
		FestivalPath: "/path/to/festival",
		Mode:         ModeImplementation, // Mode is set but no factory registered
	}

	_, err := NewNavigator(context.Background(), gctx)
	if err == nil {
		t.Error("Expected error for unknown mode")
	}
}

func TestNewNavigatorForPath(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	ClearNavigatorRegistry()
	RegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{mode: ModeImplementation, ctx: gctx}, nil
	})

	// Create a temp festival directory
	festivalPath := t.TempDir()
	phasePath := filepath.Join(festivalPath, "001_Implementation")
	if err := os.MkdirAll(phasePath, 0755); err != nil {
		t.Fatal(err)
	}

	nav, err := NewNavigatorForPath(context.Background(), festivalPath, phasePath, nil)
	if err != nil {
		t.Errorf("NewNavigatorForPath() error = %v", err)
		return
	}

	if nav == nil {
		t.Error("Expected navigator to be created")
	}

	// Verify the context was set up correctly
	ctx := nav.GetContext()
	if ctx.FestivalPath != festivalPath {
		t.Errorf("FestivalPath = %v, want %v", ctx.FestivalPath, festivalPath)
	}
	if ctx.PhasePath != phasePath {
		t.Errorf("PhasePath = %v, want %v", ctx.PhasePath, phasePath)
	}
}

func TestDetectPhaseType(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantType string
	}{
		{
			name: "no PHASE_GOAL.md defaults to implementation",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantType: "implementation",
		},
		{
			name: "PHASE_GOAL.md with no frontmatter defaults to implementation",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				content := "# Phase Goal\n\nSome content without frontmatter"
				if err := os.WriteFile(filepath.Join(dir, "PHASE_GOAL.md"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantType: "implementation",
		},
		{
			name: "PHASE_GOAL.md with phase type",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				content := `---
fest_type: phase
fest_id: 001_Planning
fest_phase_type: planning
fest_status: active
---

# Phase Goal

Planning phase`
				if err := os.WriteFile(filepath.Join(dir, "PHASE_GOAL.md"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantType: "planning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phasePath := tt.setup(t)
			got := DetectPhaseType(phasePath)
			if got != tt.wantType {
				t.Errorf("DetectPhaseType() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

func TestGetRegisteredModes(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	ClearNavigatorRegistry()

	// Initially empty
	modes := GetRegisteredModes()
	if len(modes) != 0 {
		t.Errorf("Expected 0 modes, got %d", len(modes))
	}

	// Register some modes
	RegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{}, nil
	})
	RegisterNavigator(ModePlan, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{}, nil
	})

	modes = GetRegisteredModes()
	if len(modes) != 2 {
		t.Errorf("Expected 2 modes, got %d", len(modes))
	}
}

func TestClearNavigatorRegistry(t *testing.T) {
	// Save and restore original state
	original := navigatorFactories
	defer func() { navigatorFactories = original }()

	// Register some modes
	RegisterNavigator(ModeImplementation, func(gctx *GuidanceContext) (Navigator, error) {
		return &mockNavigator{}, nil
	})

	// Clear
	ClearNavigatorRegistry()

	modes := GetRegisteredModes()
	if len(modes) != 0 {
		t.Errorf("Expected 0 modes after clear, got %d", len(modes))
	}
}

// mockNavigator implements Navigator for testing
type mockNavigator struct {
	mode        Mode
	ctx         *GuidanceContext
	initialized bool
}

func (m *mockNavigator) GetNext(ctx context.Context) (*NextStep, error) {
	return nil, nil
}

func (m *mockNavigator) MarkComplete(ctx context.Context, stepID string) error {
	return nil
}

func (m *mockNavigator) MarkSkipped(ctx context.Context, stepID string) error {
	return nil
}

func (m *mockNavigator) MarkFailed(ctx context.Context, stepID string) error {
	return nil
}

func (m *mockNavigator) GetProgress(ctx context.Context) (*Progress, error) {
	return NewProgress(m.mode), nil
}

func (m *mockNavigator) FormatInstructions(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockNavigator) GetContextFiles(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockNavigator) Initialize(ctx context.Context) error {
	m.initialized = true
	return nil
}

func (m *mockNavigator) Advance(ctx context.Context) error {
	return nil
}

func (m *mockNavigator) Reset(ctx context.Context) error {
	m.initialized = false
	return nil
}

func (m *mockNavigator) GetState() *GuidanceState {
	return &GuidanceState{Mode: m.mode}
}

func (m *mockNavigator) GetConfig() *GuidanceConfig {
	if m.ctx != nil {
		return m.ctx.Config
	}
	return DefaultConfig()
}

func (m *mockNavigator) GetContext() *GuidanceContext {
	if m.ctx != nil {
		return m.ctx
	}
	return &GuidanceContext{Mode: m.mode}
}

func (m *mockNavigator) ShouldDemote(step *NextStep) bool {
	return false
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
