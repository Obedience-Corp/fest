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

// createTestConfig creates a temporary directory with a festival types config.
func createTestConfig(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "fest-festival-types-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	festivalDir := filepath.Join(tmpDir, ".festival")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatalf("failed to create .festival dir: %v", err)
	}

	configContent := `version: 1
types:
  - name: standard
    description: Default festival type with full planning and implementation phases
    default: true
    phases:
      - name: INGEST
        type: standard
        auto: true
      - name: PLAN
        type: standard
        auto: true
      - name: IMPLEMENT
        type: implementation
        auto: false
      - name: POLISH
        type: standard
        auto: false

  - name: implementation
    description: Direct implementation without planning overhead
    default: false
    skip_ingestion: true
    phases:
      - name: IMPLEMENT
        type: implementation
        auto: true
      - name: POLISH
        type: standard
        auto: false

  - name: research
    description: Research-focused workflow
    default: false
    phases:
      - name: RESEARCH
        type: research
        auto: true
        role: researcher
      - name: ANALYSIS
        type: standard
        auto: false

  - name: quick
    description: Fast, minimal overhead workflow
    default: false
    skip_ingestion: true
    phases:
      - name: QUICK
        type: quick
        auto: true

  - name: ritual
    description: Simple repeating patterns
    default: false
    note: For simple repeating workflows
    phases: []
`

	configPath := filepath.Join(festivalDir, "festival_types.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestRunFestivalList(t *testing.T) {
	testDir, cleanup := createTestConfig(t)
	defer cleanup()

	// Change to test directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

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
				if !strings.Contains(output, "quick") {
					t.Error("output missing 'quick' type")
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
				if len(festTypes) != 5 {
					t.Errorf("expected 5 types, got %d", len(festTypes))
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

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
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
	testDir, cleanup := createTestConfig(t)
	defer cleanup()

	// Change to test directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

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
				if len(phases) != 2 {
					t.Errorf("expected 2 phases, got %d", len(phases))
				}
				if phases[0].Name != "RESEARCH" {
					t.Errorf("expected first phase to be RESEARCH, got %s", phases[0].Name)
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
				if !strings.Contains(output, "Simple repeating patterns") {
					t.Error("output missing description")
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

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
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
	testDir, cleanup := createTestConfig(t)
	defer cleanup()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

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
	testDir, cleanup := createTestConfig(t)
	defer cleanup()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

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
