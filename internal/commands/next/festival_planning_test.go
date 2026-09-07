package next

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance/selection"
)

// writePlanningFixture creates a festival directory shaped like a fresh
// `fest create festival`: root documents carrying template markers, a fest.yaml
// recording the given status, and no phases unless asked for one.
func writePlanningFixture(t *testing.T, status string, phaseDirs ...string) string {
	t.Helper()
	dir := t.TempDir()

	docs := map[string]string{
		"FESTIVAL_OVERVIEW.md": "# Overview\n\n[REPLACE: what this festival delivers]\n[REPLACE: who it is for]\n",
		"FESTIVAL_GOAL.md":     "# Goal\n\n[REPLACE: the outcome]\n",
		"TODO.md":              "# TODO\n\n[REPLACE: first thing]\n",
	}
	for name, body := range docs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if status != "" {
		festYAML := fmt.Sprintf(`version: "1.0"
metadata:
  id: TF0001
  name: test-fest
  status_history:
    - status: %s
      timestamp: 2026-01-01T00:00:00Z
`, status)
		if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, phase := range phaseDirs {
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
	dir := writePlanningFixture(t, "planning")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := buildFestivalPlanningResult(ctx, dir, "planning", 0); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestRouteUnplannedFestival_ContextCancelled(t *testing.T) {
	dir := writePlanningFixture(t, "planning")
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
			dir := writePlanningFixture(t, tc.status, tc.phases...)
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
	dir := writePlanningFixture(t, "planning")

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

	for _, want := range []string{
		"FESTIVAL PLANNING",
		"FESTIVAL_GOAL.md",
		"FESTIVAL_OVERVIEW.md",
		"TODO.md",
		"fest wizard fill .",
		"fest create phase",
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
}

func TestRouteUnplannedFestival_FreshFestivalJSON(t *testing.T) {
	dir := writePlanningFixture(t, "planning")

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
		Task             map[string]any `json:"task"`
		Reason           string         `json:"reason"`
		FestivalComplete bool           `json:"festival_complete"`
		FestivalPlanning *struct {
			Status      string `json:"status"`
			PhaseCount  int    `json:"phase_count"`
			MarkerTotal int    `json:"marker_total"`
			MarkerFiles []struct {
				File  string `json:"file"`
				Count int    `json:"count"`
			} `json:"marker_files"`
			NextActions []string `json:"next_actions"`
		} `json:"festival_planning"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
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
	if payload.FestivalPlanning.Status != "planning" {
		t.Errorf("status = %q, want planning", payload.FestivalPlanning.Status)
	}
	if payload.FestivalPlanning.PhaseCount != 0 {
		t.Errorf("phase_count = %d, want 0", payload.FestivalPlanning.PhaseCount)
	}
	if payload.FestivalPlanning.MarkerTotal != 4 {
		t.Errorf("marker_total = %d, want 4", payload.FestivalPlanning.MarkerTotal)
	}
	if len(payload.FestivalPlanning.MarkerFiles) != 3 {
		t.Fatalf("marker_files = %d entries, want 3", len(payload.FestivalPlanning.MarkerFiles))
	}
	if payload.FestivalPlanning.MarkerFiles[0].File != "FESTIVAL_GOAL.md" {
		t.Errorf("marker_files[0].file = %q, want FESTIVAL_GOAL.md (sorted)",
			payload.FestivalPlanning.MarkerFiles[0].File)
	}
	if len(payload.FestivalPlanning.NextActions) == 0 {
		t.Error("next_actions is empty")
	}
}

func TestRouteEmptyPlanningFestival(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		result      *selection.NextTaskResult
		wantHandled bool
	}{
		{
			name:        "nil result is not handled",
			status:      "planning",
			result:      nil,
			wantHandled: false,
		},
		{
			name:        "incomplete festival is not handled",
			status:      "planning",
			result:      &selection.NextTaskResult{FestivalComplete: false},
			wantHandled: false,
		},
		{
			name:        "unknown progress is not handled",
			status:      "planning",
			result:      &selection.NextTaskResult{FestivalComplete: true},
			wantHandled: false,
		},
		{
			name:   "festival with tasks is genuinely complete",
			status: "planning",
			result: &selection.NextTaskResult{
				FestivalComplete: true,
				Progress:         &selection.ProgressInfo{TotalTasks: 3, CompletedTasks: 3},
			},
			wantHandled: false,
		},
		{
			name:   "active festival is left alone",
			status: "active",
			result: &selection.NextTaskResult{
				FestivalComplete: true,
				Progress:         &selection.ProgressInfo{TotalTasks: 0},
			},
			wantHandled: false,
		},
		{
			name:   "planning festival with no tasks gets the planning step",
			status: "planning",
			result: &selection.NextTaskResult{
				FestivalComplete: true,
				Progress:         &selection.ProgressInfo{TotalTasks: 0},
			},
			wantHandled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writePlanningFixture(t, tc.status, "001_PLAN")
			var handled bool
			out, err := captureStdoutErr(t, func() error {
				var routeErr error
				handled, routeErr = routeEmptyPlanningFestival(
					context.Background(), dir, tc.status, tc.result, RenderOptions{})
				return routeErr
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if handled != tc.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tc.wantHandled)
			}
			if tc.wantHandled && !strings.Contains(out, "FESTIVAL PLANNING") {
				t.Errorf("expected the planning step, got: %s", out)
			}
			if !tc.wantHandled && out != "" {
				t.Errorf("expected no output, got: %s", out)
			}
		})
	}
}

func TestCountFestivalPhases(t *testing.T) {
	t.Run("missing directory reports an error", func(t *testing.T) {
		if _, err := countFestivalPhases(filepath.Join(t.TempDir(), "absent")); err == nil {
			t.Fatal("expected an error for a missing festival directory")
		}
	})

	t.Run("counts only numbered directories", func(t *testing.T) {
		dir := writePlanningFixture(t, "planning", "001_PLAN", "002_IMPL")
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
		phaseCount  int
		want        string
	}{
		{name: "markers and no phases", markerFiles: 3, phaseCount: 0, want: "no phases exist yet"},
		{name: "markers with phases", markerFiles: 3, phaseCount: 2, want: "no step is ready to run"},
		{name: "no markers and no phases", markerFiles: 0, phaseCount: 0, want: "has no phases yet"},
		{name: "no markers with phases", markerFiles: 0, phaseCount: 2, want: "no task or workflow step"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := festivalPlanningReason(tc.markerFiles, tc.phaseCount)
			if !strings.Contains(got, tc.want) {
				t.Errorf("reason = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
