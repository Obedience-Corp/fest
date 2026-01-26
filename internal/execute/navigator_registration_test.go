package execute

import (
	"context"
	"slices"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

func TestNavigatorRegistration(t *testing.T) {
	// Verify that ModeImplementation is registered
	modes := guidance.GetRegisteredModes()
	if !slices.Contains(modes, guidance.ModeImplementation) {
		t.Error("ModeImplementation not registered with guidance factory")
	}
}

func TestNavigatorFactory_CreateNavigator(t *testing.T) {
	// Create a temporary festival directory
	tmpDir := t.TempDir()

	gctx := &guidance.GuidanceContext{
		FestivalPath: tmpDir,
		Mode:         guidance.ModeImplementation,
		Config:       guidance.DefaultConfig(),
	}

	// Use the factory to create a navigator
	nav, err := guidance.NewNavigator(context.Background(), gctx)
	if err != nil {
		t.Fatalf("NewNavigator() error = %v", err)
	}

	if nav == nil {
		t.Fatal("NewNavigator() returned nil")
	}

	// Verify it's the correct type
	if nav.GetContext().Mode != guidance.ModeImplementation {
		t.Errorf("Navigator mode = %v, want %v", nav.GetContext().Mode, guidance.ModeImplementation)
	}
}

func TestPlanBuilderAdapter(t *testing.T) {
	tmpDir := t.TempDir()

	adapter := newPlanBuilderAdapter(tmpDir)
	if adapter == nil {
		t.Fatal("newPlanBuilderAdapter() returned nil")
	}

	// BuildPlan should work even on empty directory (just returns empty plan)
	ctx := context.Background()
	plan, err := adapter.BuildPlan(ctx)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	if plan == nil {
		t.Fatal("BuildPlan() returned nil plan")
	}
}
