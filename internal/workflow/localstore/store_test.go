package localstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleWorkflow = `---
workflow_version: 1
workflow_id: wf-sample
workitem_id: smoke-001
---

## Step 1: ALIGN PROBLEM

**Goal:** Sample.

**Actions:**
1. Read step.

**Output:** A read step.
`

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	docPath := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(docPath, []byte(sampleWorkflow), 0o644); err != nil {
		t.Fatal(err)
	}
	return Open(filepath.Join(dir, ".workflow"), docPath), dir
}

func TestStore_InitCreatesManifest(t *testing.T) {
	s, _ := newTestStore(t)
	err := s.Init(context.Background(), InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.LoadManifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != ManifestVersion || m.Kind != ManifestKind || m.WorkflowID != "wf-x" {
		t.Errorf("unexpected manifest: %+v", m)
	}
	if m.DocHash == "" {
		t.Errorf("DocHash should be populated when workflow doc exists")
	}
}

func TestStore_InitRefusesOverwrite(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	err := s.Init(ctx, InitOptions{WorkflowID: "wf-y", WorkitemID: "wi-y"})
	if err == nil {
		t.Fatal("expected refusal without Force")
	}
}

func TestStore_InitForceOverwrites(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-y", WorkitemID: "wi-y", Force: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := s.LoadManifest(ctx)
	if m.WorkflowID != "wf-y" {
		t.Errorf("expected overwrite to wf-y, got %q", m.WorkflowID)
	}
}

func TestStore_StartRunCreatesRunDir(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	runID, err := s.StartRun(ctx, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(runID, "run-") {
		t.Errorf("RunID = %q, want run- prefix", runID)
	}

	m, _ := s.LoadManifest(ctx)
	if m.ActiveRunID != runID {
		t.Errorf("active_run_id not updated, got %q", m.ActiveRunID)
	}
	if len(m.Runs) != 1 {
		t.Errorf("expected 1 run record, got %d", len(m.Runs))
	}

	runDir := filepath.Join(s.root, "runs", runID)
	if _, err := os.Stat(filepath.Join(runDir, "run.yaml")); err != nil {
		t.Errorf("run.yaml missing: %v", err)
	}

	eventsPath := filepath.Join(runDir, "progress_events.jsonl")
	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("events file: %v", err)
	}
	if !strings.Contains(string(raw), EventWorkflowRunStarted) {
		t.Errorf("missing workflow_run_started event: %s", raw)
	}
}

func TestStore_SecondStartCoexistsWithFirst(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	first, _ := s.StartRun(ctx, "")
	// Move clock forward slightly so the run-id timestamp differs.
	time.Sleep(1100 * time.Millisecond)
	second, err := s.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("expected distinct run ids, got %q and %q", first, second)
	}
	if _, err := os.Stat(filepath.Join(s.root, "runs", first, "run.yaml")); err != nil {
		t.Errorf("first run dir missing after second start: %v", err)
	}
	runs, _ := s.ListRuns(ctx)
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestStore_StartRunUniqueWithinSameSecond(t *testing.T) {
	// Pre-create the .workflow/runs/<base-id>/ directory so the next
	// StartRun in the same second must allocate a -2 suffix instead of
	// overwriting it.
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	first, err := s.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	firstRunYAMLBefore, err := os.ReadFile(filepath.Join(s.root, "runs", first, "run.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// Second start within the same second.
	second, err := s.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("expected distinct run ids on same-second starts, got %q and %q", first, second)
	}

	// First run's directory and run.yaml must be untouched.
	firstRunYAMLAfter, err := os.ReadFile(filepath.Join(s.root, "runs", first, "run.yaml"))
	if err != nil {
		t.Fatalf("first run.yaml missing after second start: %v", err)
	}
	if string(firstRunYAMLBefore) != string(firstRunYAMLAfter) {
		t.Errorf("first run.yaml was modified by second start")
	}
	// Second run directory must exist.
	if _, err := os.Stat(filepath.Join(s.root, "runs", second, "run.yaml")); err != nil {
		t.Errorf("second run.yaml missing: %v", err)
	}
}

func TestStore_AppendEventAndReplay(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartRun(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: EventStepDone}); err != nil {
		t.Fatal(err)
	}

	state, err := s.LoadActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.CurrentStep != 1 || state.CompletedSteps != 1 {
		t.Errorf("CurrentStep=%d CompletedSteps=%d", state.CurrentStep, state.CompletedSteps)
	}
}

func TestStore_ReplayOverridesStaleSummary(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	runID, _ := s.StartRun(ctx, "")
	if err := s.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: EventStepDone}); err != nil {
		t.Fatal(err)
	}

	// Corrupt the summary on disk.
	runPath := filepath.Join(s.root, "runs", runID, "run.yaml")
	raw, _ := os.ReadFile(runPath)
	bad := strings.Replace(string(raw), "current_step: 0", "current_step: 999", 1)
	_ = os.WriteFile(runPath, []byte(bad), 0o644)

	state, _ := s.LoadActive(ctx)
	if state.CurrentStep != 2 || state.CompletedSteps != 1 {
		t.Errorf("replay should override stale summary, got CurrentStep=%d CompletedSteps=%d", state.CurrentStep, state.CompletedSteps)
	}

	// Repair should have rewritten the summary.
	raw2, _ := os.ReadFile(runPath)
	if strings.Contains(string(raw2), "current_step: 999") {
		t.Errorf("stale summary not repaired")
	}
}

func TestStore_DocHashChange(t *testing.T) {
	s, root := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "WORKFLOW.md"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, cur, stored, err := s.CheckDocHash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Errorf("expected changed=true, cur=%s stored=%s", cur, stored)
	}
}

func TestStore_EventsAreAppendOnly(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartRun(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, Event{EventType: "B"}); err != nil {
		t.Fatal(err)
	}

	m, _ := s.LoadManifest(ctx)
	eventsPath := filepath.Join(s.root, "runs", m.ActiveRunID, "progress_events.jsonl")
	raw, _ := os.ReadFile(eventsPath)
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// expect 3 lines: workflow_run_started, A, B
	if len(lines) != 3 {
		t.Fatalf("expected 3 event lines, got %d: %s", len(lines), raw)
	}

	var first, second, third struct {
		EventType string `json:"event_type"`
	}
	_ = json.Unmarshal([]byte(lines[0]), &first)
	_ = json.Unmarshal([]byte(lines[1]), &second)
	_ = json.Unmarshal([]byte(lines[2]), &third)
	if first.EventType != EventWorkflowRunStarted {
		t.Errorf("line 1 = %q", first.EventType)
	}
	if second.EventType != "A" || third.EventType != "B" {
		t.Errorf("append order broken: A=%q B=%q", second.EventType, third.EventType)
	}
}

func TestStore_ContextCancel(t *testing.T) {
	s, _ := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Init(ctx, InitOptions{WorkflowID: "wf-x", WorkitemID: "wi-x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
