package types

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFestivalTypesConfig_Integration tests the full config loading flow.
func TestLoadFestivalTypesConfig_Integration(t *testing.T) {
	t.Run("loads from methodology directory", func(t *testing.T) {
		ctx := context.Background()

		// Save original directory
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(originalDir) }()

		// Change to methodology/festivals directory
		festRoot := filepath.Join(originalDir, "..", "..", "methodology", "festivals")
		if err := os.Chdir(festRoot); err != nil {
			t.Skipf("Skipping: methodology directory not accessible: %v", err)
		}

		// Load config
		config, err := LoadFestivalTypesConfig(ctx)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Verify config structure
		if config == nil {
			t.Fatal("expected config, got nil")
		}
		if len(config.Types) == 0 {
			t.Fatal("expected at least one type")
		}

		// Verify standard type exists
		standard, err := config.GetFestivalType("standard")
		if err != nil {
			t.Fatalf("standard type not found: %v", err)
		}
		if !standard.Default {
			t.Error("standard type should be default")
		}

		// Verify all expected types exist
		expectedTypes := []string{"standard", "implementation", "research", "ritual"}
		for _, typeName := range expectedTypes {
			if _, err := config.GetFestivalType(typeName); err != nil {
				t.Errorf("expected type %q not found: %v", typeName, err)
			}
		}
	})

	t.Run("default config is valid", func(t *testing.T) {
		config := getDefaultConfig()
		if err := validateConfig(config); err != nil {
			t.Errorf("default config validation failed: %v", err)
		}
	})

	t.Run("all types have proper phase structure", func(t *testing.T) {
		ctx := context.Background()

		// Save original directory
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(originalDir) }()

		// Try to load from methodology
		festRoot := filepath.Join(originalDir, "..", "..", "methodology", "festivals")
		if err := os.Chdir(festRoot); err != nil {
			t.Skip("Skipping: methodology directory not accessible")
		}

		config, err := LoadFestivalTypesConfig(ctx)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		for _, ft := range config.Types {
			// Ritual can have no phases
			if ft.Name == "ritual" {
				continue
			}

			if len(ft.Phases) == 0 {
				t.Errorf("type %q has no phases", ft.Name)
			}

			// Verify GetAutoPhases and GetPendingPhases partition all phases
			autoPhases := ft.GetAutoPhases()
			pendingPhases := ft.GetPendingPhases()
			total := len(autoPhases) + len(pendingPhases)

			if total != len(ft.Phases) {
				t.Errorf("type %q: auto (%d) + pending (%d) != total phases (%d)",
					ft.Name, len(autoPhases), len(pendingPhases), len(ft.Phases))
			}
		}
	})

	t.Run("standard type has expected structure", func(t *testing.T) {
		ctx := context.Background()

		// Save original directory
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(originalDir) }()

		// Try to load from methodology
		festRoot := filepath.Join(originalDir, "..", "..", "methodology", "festivals")
		if err := os.Chdir(festRoot); err != nil {
			t.Skip("Skipping: methodology directory not accessible")
		}

		config, err := LoadFestivalTypesConfig(ctx)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		standard, err := config.GetFestivalType("standard")
		if err != nil {
			t.Fatal(err)
		}

		// Standard should have INGEST and PLAN auto-scaffolded
		autoPhases := standard.GetAutoPhases()
		if len(autoPhases) < 2 {
			t.Errorf("standard type should have at least 2 auto phases, got %d", len(autoPhases))
		}

		// Standard should have IMPLEMENT and POLISH pending
		pendingPhases := standard.GetPendingPhases()
		if len(pendingPhases) < 2 {
			t.Errorf("standard type should have at least 2 pending phases, got %d", len(pendingPhases))
		}
	})
}
