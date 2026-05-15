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
	err := s.Init(context.Background(), InitOptions{WorkflowID: "wf-x"})
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
		t.Fatal(err)
	}
	err := s.Init(ctx, InitOptions{WorkflowID: "wf-y"})
	if err == nil {
		t.Fatal("expected refusal without Force")
	}
}

func TestStore_InitForceOverwrites(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-y", Force: true}); err != nil {
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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

// TestStore_TerminalEventFinalizesManifest locks in D030 finding #3:
// completing/abandoning a run clears active_run_id and stamps the runs[]
// entry. Without this, `fest workflow runs` reports terminated runs as
// active.
func TestStore_TerminalEventFinalizesManifest(t *testing.T) {
	for _, ev := range []string{EventWorkflowRunCompleted, EventWorkflowRunAbandoned} {
		t.Run(ev, func(t *testing.T) {
			s, _ := newTestStore(t)
			ctx := context.Background()
			if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
				t.Fatal(err)
			}
			runID, err := s.StartRun(ctx, "")
			if err != nil {
				t.Fatal(err)
			}

			if err := s.AppendEvent(ctx, Event{EventType: ev}); err != nil {
				t.Fatal(err)
			}

			m, err := s.LoadManifest(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if m.ActiveRunID != "" {
				t.Errorf("ActiveRunID = %q, want empty after terminal event", m.ActiveRunID)
			}
			if len(m.Runs) != 1 {
				t.Fatalf("Runs = %d, want 1", len(m.Runs))
			}
			wantStatus := "completed"
			if ev == EventWorkflowRunAbandoned {
				wantStatus = "abandoned"
			}
			if m.Runs[0].Status != wantStatus {
				t.Errorf("Runs[0].Status = %q, want %q", m.Runs[0].Status, wantStatus)
			}
			if m.Runs[0].RunID != runID {
				t.Errorf("Runs[0].RunID = %q, want %q", m.Runs[0].RunID, runID)
			}
			if m.Runs[0].EndedAt == "" {
				t.Error("Runs[0].EndedAt empty, want a timestamp")
			}

			// Idempotent at the finalize layer: calling finalize again for an
			// already-terminal run does not corrupt state. AppendEvent itself
			// refuses with "no active run" once ActiveRunID is cleared, which
			// is the expected guard for callers.
			if err := s.finalizeRunInManifest(ctx, Event{EventType: ev, RunID: runID, Timestamp: time.Now().UTC()}); err != nil {
				t.Fatalf("re-finalize should be idempotent, got %v", err)
			}
			m2, _ := s.LoadManifest(ctx)
			if m2.Runs[0].Status != wantStatus {
				t.Errorf("after re-finalize Runs[0].Status = %q, want %q", m2.Runs[0].Status, wantStatus)
			}
		})
	}
}

// TestLoadManifest_RejectsMalformed locks in D030 finding #2: LoadManifest
// must validate version/kind/workflow_id before returning so callers do not
// silently propagate corrupt state.
func TestLoadManifest_RejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "missing version",
			yaml: "kind: workflow-runtime\nworkflow_id: wf-x\n",
			want: "missing version",
		},
		{
			name: "unsupported version",
			yaml: "version: 99\nkind: workflow-runtime\nworkflow_id: wf-x\n",
			want: "unsupported workflow manifest version",
		},
		{
			name: "wrong kind",
			yaml: "version: 1\nkind: not-workflow\nworkflow_id: wf-x\n",
			want: "kind mismatch",
		},
		{
			name: "missing workflow_id",
			yaml: "version: 1\nkind: workflow-runtime\n",
			want: "missing workflow_id",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			s := &Store{root: dir}
			_, err := s.LoadManifest(context.Background())
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}

// TestStore_ReplayHandlesStepSkip locks in D030 finding #1: skip clears
// Blocked and counts as forward progress, matching camp's localrun.go
// semantics so the same event stream produces the same state in both repos.
func TestStore_ReplayHandlesStepSkip(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartRun(ctx, ""); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []string{EventStepStart, EventStepBlock, EventStepSkip} {
		if err := s.AppendEvent(ctx, Event{EventType: ev}); err != nil {
			t.Fatal(err)
		}
	}

	state, err := s.LoadActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.Blocked {
		t.Errorf("Blocked = true after skip, want false")
	}
	if state.CompletedSteps != 1 {
		t.Errorf("CompletedSteps = %d, want 1 (skip counts as progress)", state.CompletedSteps)
	}
	if state.Status != "active" {
		t.Errorf("Status = %q, want active (skip clears blocked status)", state.Status)
	}
}

func TestStore_ReplayOverridesStaleSummary(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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

func TestStore_ReplayEventsScannerErrorSurfaces(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
		t.Fatal(err)
	}
	runID, _ := s.StartRun(ctx, "")
	eventsPath := filepath.Join(s.root, "runs", runID, "progress_events.jsonl")

	huge := make([]byte, 4*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	huge = append(huge, '\n')
	if err := os.WriteFile(eventsPath, huge, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.LoadActive(ctx)
	if err == nil {
		t.Fatal("expected scanner error to surface from LoadActive")
	}
	if !strings.Contains(err.Error(), "scanning event stream") {
		t.Errorf("expected wrapped scanner error, got: %v", err)
	}
}

func TestStore_ReplayEventsMissingFileDegradesToCache(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
		t.Fatal(err)
	}
	runID, _ := s.StartRun(ctx, "")
	eventsPath := filepath.Join(s.root, "runs", runID, "progress_events.jsonl")
	if err := os.Remove(eventsPath); err != nil {
		t.Fatal(err)
	}

	state, err := s.LoadActive(ctx)
	if err != nil {
		t.Fatalf("missing event stream should not error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state from cached summary")
	}
}

func TestStore_DocHashChange(t *testing.T) {
	s, root := newTestStore(t)
	ctx := context.Background()
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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
	if err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"}); err != nil {
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
	err := s.Init(ctx, InitOptions{WorkflowID: "wf-x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
