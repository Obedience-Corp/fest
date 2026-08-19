package festival

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setupPreviewMarkerTemplates writes a template set whose markers cannot be
// auto-filled from the festival context, so a preview has something to report.
func setupPreviewMarkerTemplates(t *testing.T, festivalsDir string) {
	t.Helper()

	for _, status := range []string{"planning", "active", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0755); err != nil {
			t.Fatalf("creating status dir %s: %v", status, err)
		}
	}

	metaDir := filepath.Join(festivalsDir, ".festival")
	templatesDir := filepath.Join(metaDir, "templates")

	writeTemplate := func(rel, content string) {
		t.Helper()
		path := filepath.Join(templatesDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("creating template dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing template %s: %v", rel, err)
		}
	}

	writeTemplate("festival/OVERVIEW.md", "# Overview\n\n[REPLACE: Current state and its problems]\n")
	writeTemplate("festival/GOAL.md", "# Goal\n\n[REPLACE: Desired end state]\n")
	writeTemplate("festival/RULES.md", "# Rules\n\nNo markers here.\n")
	writeTemplate("festival/TODO.md", "# TODO\n\n[REPLACE: First action]\n")

	writeTemplate("phases/ingest/GOAL.md", "# Phase: {{.phase_name}}\n\n[REPLACE: Ingest scope]\n")
	writeTemplate("phases/planning/GOAL.md", "# Phase: {{.phase_name}}\n\n[REPLACE: Planning scope]\n")
	writeTemplate("phases/implementation/GOAL.md", "# Phase: {{.phase_name}}\n\n[REPLACE: Implementation scope]\n")
	writeTemplate("phases/implementation/gates/QUALITY_GATE_TESTING.md", "# Testing Gate\n\n[REPLACE: Testing evidence required]\n")

	festivalTypes := `version: 1
types:
  - name: standard
    description: Default festival type
    default: true
    phases:
      - name: INGEST
        type: ingest
        auto: true
        description: Ingest input materials
      - name: PLAN
        type: planning
        auto: true
        description: Plan the work
  - name: implementation
    description: Execution-only festival
    skip_ingestion: true
    phases:
      - name: IMPLEMENT
        type: implementation
        auto: true
        description: Implement pre-planned work
`
	if err := os.WriteFile(filepath.Join(metaDir, "festival_types.yaml"), []byte(festivalTypes), 0644); err != nil {
		t.Fatalf("writing festival_types.yaml: %v", err)
	}
}

// newPreviewMarkerWorkspace prepares a festivals workspace and makes it the
// working directory for the duration of the test.
func newPreviewMarkerWorkspace(t *testing.T) string {
	t.Helper()

	festivalsDir := filepath.Join(t.TempDir(), "festivals")
	setupPreviewMarkerTemplates(t, festivalsDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolving working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(festivalsDir); err != nil {
		t.Fatalf("entering festivals workspace: %v", err)
	}
	return festivalsDir
}

// capturePreviewStdout runs fn with stdout redirected and returns what it wrote.
func capturePreviewStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = writer

	runErr := fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("closing write pipe: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("closing read pipe: %v", err)
	}
	return buf.String(), runErr
}

// runPreview executes a dry-run create and decodes its JSON preview.
func runPreview(t *testing.T, opts *CreateFestivalOptions) createFestivalPreviewResult {
	t.Helper()

	opts.DryRun = true
	opts.JSONOutput = true
	output, err := capturePreviewStdout(t, func() error {
		return RunCreateFestival(context.Background(), opts)
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v (output: %s)", err, output)
	}

	var preview createFestivalPreviewResult
	if err := json.Unmarshal([]byte(output), &preview); err != nil {
		t.Fatalf("decoding preview JSON: %v (output: %s)", err, output)
	}
	if !preview.OK {
		t.Fatalf("preview reported failure: %s", output)
	}
	return preview
}

func previewHints(preview createFestivalPreviewResult) []string {
	hints := make([]string, 0, len(preview.Markers))
	for _, marker := range preview.Markers {
		hints = append(hints, marker.Hint)
	}
	return hints
}

// previewFilledHints returns the hints the preview reports as filled.
func previewFilledHints(preview createFestivalPreviewResult) []string {
	var hints []string
	for _, marker := range preview.Markers {
		if marker.Filled {
			hints = append(hints, marker.Hint)
		}
	}
	return hints
}

// countFilledPreviewMarkers counts the markers flagged filled.
func countFilledPreviewMarkers(preview createFestivalPreviewResult) int {
	return len(previewFilledHints(preview))
}

func TestPreviewMarkersRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, opts *CreateFestivalOptions)
		ctx     func() (context.Context, context.CancelFunc)
		wantErr string
	}{
		{
			name:   "cancelled context",
			mutate: func(t *testing.T, opts *CreateFestivalOptions) {},
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			wantErr: "context cancelled",
		},
		{
			name: "malformed --markers JSON",
			mutate: func(t *testing.T, opts *CreateFestivalOptions) {
				opts.Markers = "{not json"
			},
			wantErr: "parsing --markers JSON",
		},
		{
			name: "missing --markers-file",
			mutate: func(t *testing.T, opts *CreateFestivalOptions) {
				opts.MarkersFile = filepath.Join(t.TempDir(), "absent.json")
			},
			wantErr: "reading markers file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPreviewMarkerWorkspace(t)

			ctx := context.Background()
			if tt.ctx != nil {
				var cancel context.CancelFunc
				ctx, cancel = tt.ctx()
				defer cancel()
			}

			opts := &CreateFestivalOptions{Name: "bad-input", Type: "standard", Dest: "planning", DryRun: true}
			tt.mutate(t, opts)

			err := RunCreateFestival(ctx, opts)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPreviewReportsMarkersWithoutWriting(t *testing.T) {
	tests := []struct {
		name         string
		festivalType string
		wantHints    []string
		wantTotal    int
	}{
		{
			name:         "standard type",
			festivalType: "standard",
			wantHints: []string{
				"Current state and its problems",
				"Desired end state",
				"First action",
				"Testing evidence required",
				"Ingest scope",
				"Planning scope",
			},
			wantTotal: 6,
		},
		{
			name:         "implementation type",
			festivalType: "implementation",
			wantHints: []string{
				"Current state and its problems",
				"Desired end state",
				"First action",
				"Testing evidence required",
				"Implementation scope",
			},
			wantTotal: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalsDir := newPreviewMarkerWorkspace(t)

			preview := runPreview(t, &CreateFestivalOptions{
				Name: "preview-markers",
				Goal: "Prove the preview reports markers",
				Type: tt.festivalType,
				Dest: "planning",
			})

			if got := previewHints(preview); !equalStringSlices(got, tt.wantHints) {
				t.Fatalf("preview hints = %v, want %v", got, tt.wantHints)
			}
			if preview.MarkersTotal != tt.wantTotal {
				t.Fatalf("markers_total = %d, want %d", preview.MarkersTotal, tt.wantTotal)
			}
			if preview.MarkersUnfilled != tt.wantTotal {
				t.Fatalf("markers_unfilled = %d, want %d", preview.MarkersUnfilled, tt.wantTotal)
			}
			if preview.MarkersFilled != 0 {
				t.Fatalf("markers_filled = %d, want 0", preview.MarkersFilled)
			}
			for _, marker := range preview.Markers {
				if marker.File == "" {
					t.Fatalf("marker %q reported no file", marker.Hint)
				}
				if marker.Line <= 0 {
					t.Fatalf("marker %q reported line %d", marker.Hint, marker.Line)
				}
				if marker.Filled {
					t.Fatalf("marker %q reported filled with no marker input", marker.Hint)
				}
			}

			planning := filepath.Join(festivalsDir, "planning")
			entries, err := os.ReadDir(planning)
			if err != nil {
				t.Fatalf("reading planning dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("dry-run wrote %d entries into planning/", len(entries))
			}

			second := runPreview(t, &CreateFestivalOptions{
				Name: "preview-markers",
				Goal: "Prove the preview reports markers",
				Type: tt.festivalType,
				Dest: "planning",
			})
			if second.Festival["id"] != preview.Festival["id"] {
				t.Fatalf("dry-run consumed the festival ID: %q then %q",
					preview.Festival["id"], second.Festival["id"])
			}
		})
	}
}

func TestPreviewAppliesSuppliedMarkerValues(t *testing.T) {
	tests := []struct {
		name            string
		useFile         bool
		values          map[string]string
		wantFilled      int
		wantUnfilled    int
		wantFilledHints []string
	}{
		{
			name:            "complete markers file fills everything",
			useFile:         true,
			values:          completeStandardMarkerValues(),
			wantFilled:      6,
			wantUnfilled:    0,
			wantFilledHints: nil,
		},
		{
			name:            "partial markers file yields mixed filled flags",
			useFile:         true,
			values:          map[string]string{"Desired end state": "A working preview", "Ingest scope": "The regression report"},
			wantFilled:      2,
			wantUnfilled:    4,
			wantFilledHints: []string{"Desired end state", "Ingest scope"},
		},
		{
			name:            "partial inline markers leave the rest",
			values:          map[string]string{"Desired end state": "A working preview"},
			wantFilled:      1,
			wantUnfilled:    5,
			wantFilledHints: []string{"Desired end state"},
		},
		{
			name:            "unrelated hints fill nothing",
			values:          map[string]string{"Not a real hint": "ignored"},
			wantFilled:      0,
			wantUnfilled:    6,
			wantFilledHints: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPreviewMarkerWorkspace(t)

			opts := &CreateFestivalOptions{
				Name: "filled-preview",
				Goal: "Prove markers input applies to the preview",
				Type: "standard",
				Dest: "planning",
			}
			encoded, err := json.Marshal(tt.values)
			if err != nil {
				t.Fatalf("encoding marker values: %v", err)
			}
			if tt.useFile {
				path := filepath.Join(t.TempDir(), "markers.json")
				if err := os.WriteFile(path, encoded, 0644); err != nil {
					t.Fatalf("writing markers file: %v", err)
				}
				opts.MarkersFile = path
			} else {
				opts.Markers = string(encoded)
			}

			preview := runPreview(t, opts)
			if preview.MarkersTotal != 6 {
				t.Fatalf("markers_total = %d, want 6", preview.MarkersTotal)
			}
			if preview.MarkersFilled != tt.wantFilled {
				t.Fatalf("markers_filled = %d, want %d", preview.MarkersFilled, tt.wantFilled)
			}
			if preview.MarkersUnfilled != tt.wantUnfilled {
				t.Fatalf("markers_unfilled = %d, want %d", preview.MarkersUnfilled, tt.wantUnfilled)
			}
			if len(preview.Markers) != 6 {
				t.Fatalf("markers list has %d entries, want every marker (6)", len(preview.Markers))
			}
			if got := countFilledPreviewMarkers(preview); got != tt.wantFilled {
				t.Fatalf("%d markers flagged filled, want %d (markers_filled reported %d)",
					got, tt.wantFilled, preview.MarkersFilled)
			}
			if tt.wantFilledHints != nil {
				got := previewFilledHints(preview)
				sort.Strings(got)
				want := append([]string(nil), tt.wantFilledHints...)
				sort.Strings(want)
				if !equalStringSlices(got, want) {
					t.Fatalf("filled hints = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestPreviewHumanOutputListsMarkers(t *testing.T) {
	newPreviewMarkerWorkspace(t)

	output, err := capturePreviewStdout(t, func() error {
		return RunCreateFestival(context.Background(), &CreateFestivalOptions{
			Name:   "human-preview",
			Goal:   "Prove the human preview lists markers",
			Type:   "standard",
			Dest:   "planning",
			DryRun: true,
		})
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v (output: %s)", err, output)
	}

	for _, want := range []string{
		"Replace Markers in Template",
		"FESTIVAL_OVERVIEW.md",
		"Current state and its problems",
		"001_INGEST/PHASE_GOAL.md",
		"Ingest scope",
		"6 markers, 6 unfilled",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("human dry-run output is missing %q:\n%s", want, output)
		}
	}
}

func TestPreviewHumanOutputSummarizesMarkerState(t *testing.T) {
	tests := []struct {
		name         string
		values       map[string]string
		wantSummary  string
		wantListed   bool
		wantHintText string
	}{
		{
			name:        "complete plan lists nothing to fill",
			values:      completeStandardMarkerValues(),
			wantSummary: "6 markers, 6 filled",
		},
		{
			name:         "partial plan reports both counts and lists the rest",
			values:       map[string]string{"Desired end state": "A working preview", "Ingest scope": "The regression report"},
			wantSummary:  "6 markers, 2 filled, 4 unfilled",
			wantListed:   true,
			wantHintText: "Current state and its problems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPreviewMarkerWorkspace(t)

			encoded, err := json.Marshal(tt.values)
			if err != nil {
				t.Fatalf("encoding marker values: %v", err)
			}

			output, err := capturePreviewStdout(t, func() error {
				return RunCreateFestival(context.Background(), &CreateFestivalOptions{
					Name:    "summary-human-preview",
					Goal:    "Prove the human summary reports marker state",
					Type:    "standard",
					Dest:    "planning",
					Markers: string(encoded),
					DryRun:  true,
				})
			})
			if err != nil {
				t.Fatalf("dry-run failed: %v (output: %s)", err, output)
			}

			if !strings.Contains(output, tt.wantSummary) {
				t.Fatalf("human dry-run output is missing summary %q:\n%s", tt.wantSummary, output)
			}
			if listed := strings.Contains(output, "[line "); listed != tt.wantListed {
				t.Fatalf("human dry-run listed markers = %v, want %v:\n%s", listed, tt.wantListed, output)
			}
			if tt.wantHintText != "" && !strings.Contains(output, tt.wantHintText) {
				t.Fatalf("human dry-run output is missing unfilled hint %q:\n%s", tt.wantHintText, output)
			}
			for hint := range tt.values {
				if strings.Contains(output, hint) {
					t.Fatalf("human dry-run listed the already filled hint %q:\n%s", hint, output)
				}
			}
		})
	}
}

func TestGroupPreviewMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		markers   []festivalPreviewMarker
		wantFiles []string
		wantCount []int
	}{
		{
			name:      "no markers",
			markers:   nil,
			wantFiles: nil,
			wantCount: nil,
		},
		{
			name: "groups by file in first-seen order",
			markers: []festivalPreviewMarker{
				{Hint: "a", rel: "TODO.md"},
				{Hint: "b", rel: "FESTIVAL_GOAL.md"},
				{Hint: "c", rel: "TODO.md"},
			},
			wantFiles: []string{"TODO.md", "FESTIVAL_GOAL.md"},
			wantCount: []int{2, 1},
		},
		{
			name:      "falls back to the display path",
			markers:   []festivalPreviewMarker{{Hint: "a", File: "planning/example/TODO.md"}},
			wantFiles: []string{"planning/example/TODO.md"},
			wantCount: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := groupPreviewMarkers(tt.markers)
			if len(groups) != len(tt.wantFiles) {
				t.Fatalf("got %d groups, want %d", len(groups), len(tt.wantFiles))
			}
			for i, group := range groups {
				if group.file != tt.wantFiles[i] {
					t.Fatalf("group %d file = %q, want %q", i, group.file, tt.wantFiles[i])
				}
				if len(group.markers) != tt.wantCount[i] {
					t.Fatalf("group %d has %d markers, want %d", i, len(group.markers), tt.wantCount[i])
				}
			}
		})
	}
}

func completeStandardMarkerValues() map[string]string {
	return map[string]string{
		"Current state and its problems": "Dry run stopped reporting markers",
		"Desired end state":              "Dry run reports markers again",
		"First action":                   "Restore the marker report",
		"Testing evidence required":      "Unit tests plus the beginner path smoke",
		"Ingest scope":                   "The regression report",
		"Planning scope":                 "The restore plan",
	}
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
