package festival

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFestivalTemplatesWithMarkers creates templates that include auto-fillable
// markers to verify the marker replacement pipeline works end-to-end.
func setupFestivalTemplatesWithMarkers(t *testing.T, festivalsDir string) {
	t.Helper()

	festivalMetaDir := filepath.Join(festivalsDir, ".festival")
	templatesDir := filepath.Join(festivalMetaDir, "templates")

	// Create status directories
	for _, status := range []string{"planning", "active", "completed", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0755); err != nil {
			t.Fatalf("failed to create status dir: %v", err)
		}
	}

	// Create festival templates with auto-fillable markers
	festivalTemplatesDir := filepath.Join(templatesDir, "festival")
	if err := os.MkdirAll(festivalTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create festival templates dir: %v", err)
	}

	// GOAL.md with festival-level markers that should be auto-filled
	goalContent := "# Festival Goal\n\n**Name:** {{.festival_name}}\n**Goal:** {{.festival_goal}}\n"
	os.WriteFile(filepath.Join(festivalTemplatesDir, "GOAL.md"), []byte(goalContent), 0644)

	// OVERVIEW.md with festival name marker
	overviewContent := "# {{.festival_name}} Overview\n"
	os.WriteFile(filepath.Join(festivalTemplatesDir, "OVERVIEW.md"), []byte(overviewContent), 0644)

	// RULES.md and TODO.md as simple templates
	os.WriteFile(filepath.Join(festivalTemplatesDir, "RULES.md"), []byte("# Rules\n"), 0644)
	os.WriteFile(filepath.Join(festivalTemplatesDir, "TODO.md"), []byte("# TODO\n"), 0644)

	// Phase templates with type-specific "Primary Goal" markers
	phaseTemplates := map[string]string{
		"ingest":         "# Phase Goal: {{.phase_name}}\n\n**Primary Goal:** [REPLACE: What this ingest phase will transform and why]\n",
		"planning":       "# Phase Goal: {{.phase_name}}\n\n**Primary Goal:** [REPLACE: What this planning phase needs to figure out or decide]\n",
		"implementation": "# Phase Goal: {{.phase_name}}\n\n**Primary Goal:** [REPLACE: What this implementation phase must deliver]\n",
		"research":       "# Phase Goal: {{.phase_name}}\n\n**Primary Goal:** [REPLACE: What question or problem this research aims to answer]\n",
		"review":         "# Phase Goal: {{.phase_name}}\n\n**Primary Goal:** [REPLACE: What this review phase validates or approves]\n",
	}

	for phaseType, content := range phaseTemplates {
		phaseDir := filepath.Join(templatesDir, "phases", phaseType)
		if err := os.MkdirAll(phaseDir, 0755); err != nil {
			t.Fatalf("failed to create phase dir %s: %v", phaseType, err)
		}
		if err := os.WriteFile(filepath.Join(phaseDir, "GOAL.md"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create phase goal template: %v", err)
		}
	}

	// Create festival_types.yaml with descriptions
	festivalTypesYAML := `version: 1
types:
  - name: standard
    description: Default festival type with full planning and implementation phases
    default: true
    phases:
      - name: INGEST
        type: ingest
        auto: true
        description: Ingest and structure input materials into actionable specifications
      - name: PLAN
        type: planning
        auto: true
        description: Plan architecture, design decisions, and task breakdown
  - name: implementation
    description: Execution-only festival for pre-planned work
    skip_ingestion: true
    phases:
      - name: IMPLEMENT
        type: implementation
        auto: true
        description: Implement pre-planned features and functionality
  - name: research
    description: Research-oriented festival
    phases:
      - name: INGEST
        type: ingest
        auto: true
        description: Ingest research materials and context
      - name: RESEARCH
        type: research
        auto: true
        description: Investigate options and evaluate alternatives
      - name: SYNTHESIZE
        type: planning
        auto: true
        description: Synthesize research findings into recommendations
`
	if err := os.WriteFile(filepath.Join(festivalMetaDir, "festival_types.yaml"), []byte(festivalTypesYAML), 0644); err != nil {
		t.Fatalf("failed to create festival_types.yaml: %v", err)
	}
}

// markerPattern is the string that indicates an unfilled marker.
const markerPattern = "[REPLACE:"

// scanForUnfilledMarkers walks a directory tree and returns all files containing
// unfilled markers along with the marker text found.
func scanForUnfilledMarkers(t *testing.T, dir string) []string {
	t.Helper()
	var findings []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), markerPattern) {
			rel, _ := filepath.Rel(dir, path)
			// Extract the marker text for diagnostics
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.Contains(line, markerPattern) {
					findings = append(findings, rel+":"+strings.TrimSpace(line)+" (line "+itoa(i+1)+")")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning for markers: %v", err)
	}
	return findings
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func TestCreateFestivalZeroMarkers(t *testing.T) {
	tests := []struct {
		name     string
		festType string
		festName string
		festGoal string
	}{
		{"standard type", "standard", "Standard Test", "Test the standard type marker fix"},
		{"implementation type", "implementation", "Impl Test", "Test implementation type markers"},
		{"research type", "research", "Research Test", "Test research type markers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			festivalsDir := filepath.Join(tmpDir, "festivals")
			setupFestivalTemplatesWithMarkers(t, festivalsDir)

			origDir, _ := os.Getwd()
			defer os.Chdir(origDir)
			os.Chdir(festivalsDir)

			opts := &CreateFestivalOptions{
				Name:       tt.festName,
				Goal:       tt.festGoal,
				Type:       tt.festType,
				Dest:       "active",
				JSONOutput: true,
			}

			err := RunCreateFestival(context.Background(), opts)
			if err != nil {
				t.Fatalf("RunCreateFestival failed: %v", err)
			}

			// Find created festival directory
			activeDir := filepath.Join(festivalsDir, "active")
			entries, err := os.ReadDir(activeDir)
			if err != nil {
				t.Fatalf("failed to read active dir: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry in active/, got %d", len(entries))
			}
			festivalDir := filepath.Join(activeDir, entries[0].Name())

			// Scan all .md files for unfilled markers
			findings := scanForUnfilledMarkers(t, festivalDir)
			if len(findings) > 0 {
				t.Errorf("found %d unfilled markers in festival (type=%s):", len(findings), tt.festType)
				for _, f := range findings {
					t.Errorf("  %s", f)
				}
			}
		})
	}
}

func TestCreateFestivalRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := &CreateFestivalOptions{
		Name:       "context-test",
		Goal:       "Test context cancellation",
		JSONOutput: true,
	}

	err := RunCreateFestival(ctx, opts)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}
