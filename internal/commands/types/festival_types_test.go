package types

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/types"
)

// useMethodologyConfig changes to the methodology/festivals/ directory so that
// LoadFestivalTypesConfig picks up the real festival_types.yaml from the template
// library. Returns the original directory for restoration via defer.
func useMethodologyConfig(t *testing.T) string {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	// Navigate from internal/commands/types/ to methodology/festivals/
	festRoot := filepath.Join(originalDir, "..", "..", "..", "methodology", "festivals")
	configPath := filepath.Join(festRoot, ".festival", "festival_types.yaml")

	if _, err := os.Stat(configPath); err != nil {
		t.Skipf("Skipping: methodology config not accessible: %v", err)
	}

	if err := os.Chdir(festRoot); err != nil {
		t.Skipf("Skipping: cannot chdir to methodology/festivals: %v", err)
	}

	return originalDir
}

func TestRunFestivalList(t *testing.T) {
	originalDir := useMethodologyConfig(t)
	defer os.Chdir(originalDir)

	tests := []struct {
		name       string
		jsonOutput bool
		wantErr    bool
		validate   func(t *testing.T, output string)
	}{
		{
			name:       "list text output",
			jsonOutput: false,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				// Check for all types
				if !strings.Contains(output, "standard") {
					t.Error("output missing 'standard' type")
				}
				if !strings.Contains(output, "implementation") {
					t.Error("output missing 'implementation' type")
				}
				if !strings.Contains(output, "research") {
					t.Error("output missing 'research' type")
				}
				if !strings.Contains(output, "ritual") {
					t.Error("output missing 'ritual' type")
				}
				// Check for default marker
				if !strings.Contains(output, "(default)") {
					t.Error("output missing default marker")
				}
				// Check for column headers
				if !strings.Contains(output, "NAME") {
					t.Error("output missing NAME header")
				}
				if !strings.Contains(output, "AUTO") {
					t.Error("output missing AUTO header")
				}
				if !strings.Contains(output, "PENDING") {
					t.Error("output missing PENDING header")
				}
			},
		},
		{
			name:       "list json output",
			jsonOutput: true,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				var festTypes []types.FestivalType
				if err := json.Unmarshal([]byte(output), &festTypes); err != nil {
					t.Errorf("failed to parse JSON: %v", err)
				}
				if len(festTypes) != 4 {
					t.Errorf("expected 4 types, got %d", len(festTypes))
				}
				// Check for standard type
				found := false
				for _, ft := range festTypes {
					if ft.Name == "standard" && ft.Default {
						found = true
						break
					}
				}
				if !found {
					t.Error("standard type not found or not default")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := runFestivalList(context.Background(), tt.jsonOutput)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			if (err != nil) != tt.wantErr {
				t.Errorf("runFestivalList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestRunFestivalShow(t *testing.T) {
	originalDir := useMethodologyConfig(t)
	defer os.Chdir(originalDir)

	tests := []struct {
		name       string
		typeName   string
		jsonOutput bool
		phasesOnly bool
		wantErr    bool
		validate   func(t *testing.T, output string)
	}{
		{
			name:       "show standard type text",
			typeName:   "standard",
			jsonOutput: false,
			phasesOnly: false,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Festival Type: standard") {
					t.Error("output missing festival type name")
				}
				if !strings.Contains(output, "Default: Yes") {
					t.Error("output missing default marker")
				}
				if !strings.Contains(output, "Auto-Scaffolded Phases") {
					t.Error("output missing auto phases section")
				}
				if !strings.Contains(output, "INGEST") {
					t.Error("output missing INGEST phase")
				}
				if !strings.Contains(output, "PLAN") {
					t.Error("output missing PLAN phase")
				}
			},
		},
		{
			name:       "show implementation type json",
			typeName:   "implementation",
			jsonOutput: true,
			phasesOnly: false,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				var festType types.FestivalType
				if err := json.Unmarshal([]byte(output), &festType); err != nil {
					t.Errorf("failed to parse JSON: %v", err)
				}
				if festType.Name != "implementation" {
					t.Errorf("expected name 'implementation', got %s", festType.Name)
				}
				if !festType.SkipIngestion {
					t.Error("expected skip_ingestion to be true")
				}
			},
		},
		{
			name:       "show phases only text",
			typeName:   "standard",
			jsonOutput: false,
			phasesOnly: true,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Phases for standard") {
					t.Error("output missing phases header")
				}
				if !strings.Contains(output, "INGEST") {
					t.Error("output missing INGEST phase")
				}
				if !strings.Contains(output, "TYPE") {
					t.Error("output missing TYPE column")
				}
				if !strings.Contains(output, "AUTO") {
					t.Error("output missing AUTO column")
				}
			},
		},
		{
			name:       "show phases only json",
			typeName:   "research",
			jsonOutput: true,
			phasesOnly: true,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				var phases []types.PhaseSpec
				if err := json.Unmarshal([]byte(output), &phases); err != nil {
					t.Errorf("failed to parse JSON: %v", err)
				}
				// Real research config has 3 phases: INGEST, RESEARCH, SYNTHESIZE
				if len(phases) != 3 {
					t.Errorf("expected 3 phases, got %d", len(phases))
				}
				if len(phases) > 0 && phases[0].Name != "INGEST" {
					t.Errorf("expected first phase to be INGEST, got %s", phases[0].Name)
				}
				if len(phases) > 1 && phases[1].Name != "RESEARCH" {
					t.Errorf("expected second phase to be RESEARCH, got %s", phases[1].Name)
				}
			},
		},
		{
			name:       "show nonexistent type",
			typeName:   "nonexistent",
			jsonOutput: false,
			phasesOnly: false,
			wantErr:    true,
			validate:   nil,
		},
		{
			name:       "show ritual type (no phases)",
			typeName:   "ritual",
			jsonOutput: false,
			phasesOnly: false,
			wantErr:    false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Festival Type: ritual") {
					t.Error("output missing festival type name")
				}
				if !strings.Contains(output, "ritual") {
					t.Error("output missing ritual type reference")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := runFestivalShow(context.Background(), tt.typeName, tt.jsonOutput, tt.phasesOnly)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			if (err != nil) != tt.wantErr {
				t.Errorf("runFestivalShow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestRunFestivalList_NoConfig(t *testing.T) {
	// Create a temp dir without config to test default behavior
	tmpDir, err := os.MkdirTemp("", "fest-no-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Should fall back to default config
	err = runFestivalList(context.Background(), false)
	if err != nil {
		t.Errorf("expected no error with default config, got: %v", err)
	}
}

func TestFestivalType_GetAutoPhases(t *testing.T) {
	originalDir := useMethodologyConfig(t)
	defer os.Chdir(originalDir)

	config, err := types.LoadFestivalTypesConfig(context.Background())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	standardType, err := config.GetFestivalType("standard")
	if err != nil {
		t.Fatalf("failed to get standard type: %v", err)
	}

	autoPhases := standardType.GetAutoPhases()
	if len(autoPhases) != 2 {
		t.Errorf("expected 2 auto phases, got %d", len(autoPhases))
	}

	if autoPhases[0].Name != "INGEST" {
		t.Errorf("expected first auto phase to be INGEST, got %s", autoPhases[0].Name)
	}
	if autoPhases[1].Name != "PLAN" {
		t.Errorf("expected second auto phase to be PLAN, got %s", autoPhases[1].Name)
	}
}

func TestFestivalType_GetPendingPhases(t *testing.T) {
	originalDir := useMethodologyConfig(t)
	defer os.Chdir(originalDir)

	config, err := types.LoadFestivalTypesConfig(context.Background())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	standardType, err := config.GetFestivalType("standard")
	if err != nil {
		t.Fatalf("failed to get standard type: %v", err)
	}

	pendingPhases := standardType.GetPendingPhases()
	if len(pendingPhases) != 2 {
		t.Errorf("expected 2 pending phases, got %d", len(pendingPhases))
	}

	if pendingPhases[0].Name != "IMPLEMENT" {
		t.Errorf("expected first pending phase to be IMPLEMENT, got %s", pendingPhases[0].Name)
	}
	if pendingPhases[1].Name != "POLISH" {
		t.Errorf("expected second pending phase to be POLISH, got %s", pendingPhases[1].Name)
	}
}
