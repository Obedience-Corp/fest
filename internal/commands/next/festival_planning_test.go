package next

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planningFixture describes the festival shape a test needs.
type planningFixture struct {
	status     string
	goal       string   // recorded in fest.yaml, as fest create --goal does
	filledDocs bool     // write the root documents with no markers left
	phases     []string // numbered phase directories to create
}

// writePlanningFixture creates a festival directory shaped like a fresh
// `fest create festival`: root documents carrying template markers, a fest.yaml
// recording the given status, and no phases unless asked for one.
func writePlanningFixture(t *testing.T, f planningFixture) string {
	t.Helper()
	dir := t.TempDir()

	docs := map[string]string{
		"FESTIVAL_OVERVIEW.md": "# Overview\n\n[REPLACE: what this festival delivers]\n[REPLACE: who it is for]\n",
		"FESTIVAL_GOAL.md":     "# Goal\n\n**Primary Goal:** [REPLACE: the outcome]\n",
		"TODO.md":              "# TODO\n\n[REPLACE: first thing]\n",
	}
	if f.filledDocs {
		docs = map[string]string{
			"FESTIVAL_OVERVIEW.md": "# Overview\n\nA real overview.\n",
			"FESTIVAL_GOAL.md":     "# Goal\n\n**Primary Goal:** Ship the first-run experience\n",
			"TODO.md":              "# TODO\n\nWrite the phases.\n",
		}
	}
	for name, body := range docs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if f.status != "" {
		festYAML := fmt.Sprintf(`version: "1.0"
metadata:
  id: TF0001
  name: test-fest
  goal: %q
  status_history:
    - status: %s
      timestamp: 2026-01-01T00:00:00Z
`, f.goal, f.status)
		if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, phase := range f.phases {
		phasePath := filepath.Join(dir, phase)
		if err := os.MkdirAll(phasePath, 0o755); err != nil {
			t.Fatal(err)
		}
		goal := "---\nfest_type: phase\nfest_phase_type: planning\nfest_status: pending\n---\n# Phase Goal\nPlan it.\n"
		if err := os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goal), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestBuildFestivalPlanningResult_ContextCancelled(t *testing.T) {
	dir := writePlanningFixture(t, planningFixture{status: "planning"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := buildFestivalPlanningResult(ctx, dir, "planning", 0); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestRouteUnplannedFestival_ContextCancelled(t *testing.T) {
	dir := writePlanningFixture(t, planningFixture{status: "planning"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handled, err := routeUnplannedFestival(ctx, dir, "planning", RenderOptions{})
	if err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
	if handled {
		t.Error("handled = true, want false when the context is cancelled")
	}
}

func TestRouteUnplannedFestival_NotHandled(t *testing.T) {
	tests := []struct {
		name   string
		status string
		phases []string
	}{
		{name: "ready festival is not a planning step", status: "ready"},
		{name: "active festival is not a planning step", status: "active"},
		{name: "undetermined status is not a planning step", status: ""},
		{name: "planning festival with a phase routes normally", status: "planning", phases: []string{"001_PLAN"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writePlanningFixture(t, planningFixture{status: tc.status, phases: tc.phases})
			out, err := captureStdoutErr(t, func() error {
				handled, routeErr := routeUnplannedFestival(context.Background(), dir, tc.status, RenderOptions{})
				if handled {
					t.Error("handled = true, want false")
				}
				return routeErr
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != "" {
				t.Errorf("expected no output, got: %s", out)
			}
		})
	}
}

func TestRouteUnplannedFestival_FreshFestivalText(t *testing.T) {
	dir := writePlanningFixture(t, planningFixture{status: "planning"})

	var handled bool
	out, err := captureStdoutErr(t, func() error {
		var routeErr error
		handled, routeErr = routeUnplannedFestival(context.Background(), dir, "planning", RenderOptions{})
		return routeErr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("handled = false, want true for a phase-less planning festival")
	}

	// The step has to stand on its own: what this is, what to write, and the
	// commands that build the plan.
	for _, want := range []string{
		"FESTIVAL PLANNING",
		"no phases yet",
		"FESTIVAL_GOAL.md",
		"[REPLACE: the outcome]",
		"[REPLACE: what this festival delivers]",
		"[REPLACE: first thing]",
		"fest wizard fill .",
		"fest understand planning",
		"fest understand structure",
		"fest create phase --name PHASE_NAME --type TYPE",
		"fest create sequence --name SEQUENCE_NAME",
		"fest create task --name TASK_NAME",
		"fest validate",
		"fest promote",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToUpper(out), "FESTIVAL COMPLETE") {
		t.Errorf("an unwritten festival must not read as complete:\n%s", out)
	}

	// Everything but the marker inventory stays short: the inventory is the
	// only part that grows with the festival.
	fixed := 0
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(line, "[REPLACE:") || strings.HasSuffix(trimmed, ".md") {
			continue
		}
		fixed++
	}
	if fixed > 40 {
		t.Errorf("step is %d lines outside the marker inventory, want at most 40:\n%s", fixed, out)
	}
}

func TestRouteUnplannedFestival_GoalShownOnlyWhenWritten(t *testing.T) {
	tests := []struct {
		name    string
		fixture planningFixture
		want    string
		absent  string
	}{
		{
			name:    "goal from fest create --goal",
			fixture: planningFixture{status: "planning", goal: "Ship the first-run experience"},
			want:    "Ship the first-run experience",
		},
		{
			name:    "goal from a filled FESTIVAL_GOAL.md",
			fixture: planningFixture{status: "planning", filledDocs: true},
			want:    "Ship the first-run experience",
		},
		{
			name:    "unfilled goal is not presented as the objective",
			fixture: planningFixture{status: "planning"},
			absent:  "Goal ",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writePlanningFixture(t, tc.fixture)
			out, err := captureStdoutErr(t, func() error {
				_, routeErr := routeUnplannedFestival(context.Background(), dir, "planning", RenderOptions{})
				return routeErr
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != "" && !strings.Contains(out, tc.want) {
				t.Errorf("output missing goal %q:\n%s", tc.want, out)
			}
			if tc.absent != "" && strings.Contains(out, tc.absent) {
				t.Errorf("output should not carry %q:\n%s", tc.absent, out)
			}
		})
	}
}

func TestRouteUnplannedFestival_NarrowsWhenMarkersAreFilled(t *testing.T) {
	dir := writePlanningFixture(t, planningFixture{status: "planning", filledDocs: true})

	out, err := captureStdoutErr(t, func() error {
		handled, routeErr := routeUnplannedFestival(context.Background(), dir, "planning", RenderOptions{})
		if !handled {
			t.Error("handled = false, want true")
		}
		return routeErr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "Unfilled Markers") || strings.Contains(out, "wizard fill") {
		t.Errorf("a written festival must not be told to fill markers:\n%s", out)
	}
	for _, want := range []string{"documents are written", "fest create phase --name PHASE_NAME --type TYPE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRouteUnplannedFestival_FreshFestivalJSON(t *testing.T) {
	dir := writePlanningFixture(t, planningFixture{status: "planning", goal: "Ship the first-run experience"})

	out, err := captureStdoutErr(t, func() error {
		handled, routeErr := routeUnplannedFestival(context.Background(), dir, "planning", RenderOptions{JSON: true})
		if !handled {
			t.Error("handled = false, want true")
		}
		return routeErr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload struct {
		Kind             string         `json:"kind"`
		Task             map[string]any `json:"task"`
		Reason           string         `json:"reason"`
		FestivalComplete bool           `json:"festival_complete"`
		FestivalPlanning *struct {
			Status      string `json:"status"`
			PhaseCount  int    `json:"phase_count"`
			Goal        string `json:"goal"`
			MarkerTotal int    `json:"marker_total"`
			MarkerFiles []struct {
				File    string `json:"file"`
				Count   int    `json:"count"`
				Markers []struct {
					Line int    `json:"line"`
					Hint string `json:"hint"`
				} `json:"markers"`
			} `json:"marker_files"`
			NextCommands []string `json:"next_commands"`
		} `json:"festival_planning"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	if payload.Kind != "festival_planning" {
		t.Errorf("kind = %q, want festival_planning", payload.Kind)
	}
	if payload.FestivalPlanning == nil {
		t.Fatalf("festival_planning block missing:\n%s", out)
	}
	if payload.Task != nil {
		t.Errorf("task must stay absent for a planning step, got %v", payload.Task)
	}
	if payload.FestivalComplete {
		t.Error("festival_complete = true, want false")
	}
	if payload.Reason == "" {
		t.Error("reason is empty")
	}

	p := payload.FestivalPlanning
	if p.Status != "planning" {
		t.Errorf("status = %q, want planning", p.Status)
	}
	if p.PhaseCount != 0 {
		t.Errorf("phase_count = %d, want 0", p.PhaseCount)
	}
	if p.Goal != "Ship the first-run experience" {
		t.Errorf("goal = %q, want the recorded goal", p.Goal)
	}
	if p.MarkerTotal != 4 {
		t.Errorf("marker_total = %d, want 4", p.MarkerTotal)
	}
	if len(p.MarkerFiles) != 3 {
		t.Fatalf("marker_files = %d entries, want 3", len(p.MarkerFiles))
	}
	if p.MarkerFiles[0].File != "FESTIVAL_GOAL.md" {
		t.Errorf("marker_files[0].file = %q, want FESTIVAL_GOAL.md (sorted)", p.MarkerFiles[0].File)
	}
	if len(p.MarkerFiles[0].Markers) != 1 {
		t.Fatalf("marker_files[0].markers = %d, want 1", len(p.MarkerFiles[0].Markers))
	}
	if p.MarkerFiles[0].Markers[0].Hint != "[REPLACE: the outcome]" {
		t.Errorf("marker hint = %q, want the marker text", p.MarkerFiles[0].Markers[0].Hint)
	}
	if p.MarkerFiles[0].Markers[0].Line == 0 {
		t.Error("marker line = 0, want the line the marker sits on")
	}
	if len(p.NextCommands) == 0 {
		t.Fatal("next_commands is empty")
	}
	for _, cmd := range p.NextCommands {
		if strings.Contains(cmd, "wizard fill") {
			t.Errorf("next_commands must not hand an agent an interactive command: %q", cmd)
		}
	}
}

func TestCountFestivalPhases(t *testing.T) {
	t.Run("missing directory reports an error", func(t *testing.T) {
		if _, err := countFestivalPhases(filepath.Join(t.TempDir(), "absent")); err == nil {
			t.Fatal("expected an error for a missing festival directory")
		}
	})

	t.Run("counts only numbered directories", func(t *testing.T) {
		dir := writePlanningFixture(t, planningFixture{status: "planning", phases: []string{"001_PLAN", "002_IMPL"}})
		if err := os.MkdirAll(filepath.Join(dir, "gates"), 0o755); err != nil {
			t.Fatal(err)
		}
		count, err := countFestivalPhases(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Fatalf("count = %d, want 2", count)
		}
	})
}

func TestFestivalPlanningReason(t *testing.T) {
	tests := []struct {
		name        string
		markerFiles int
		want        string
	}{
		{name: "markers pending", markerFiles: 3, want: "3 documents still hold unfilled markers"},
		{name: "documents written", markerFiles: 0, want: "has no phases yet"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := festivalPlanningReason(tc.markerFiles)
			if !strings.Contains(got, tc.want) {
				t.Errorf("reason = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
