package localstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestValidateManifest(t *testing.T) {
	valid := Manifest{
		Version:    ManifestVersion,
		Kind:       ManifestKind,
		WorkflowID: "wf-x",
	}
	if err := validateManifest(&valid, "/virtual/.workflow/workflow.yaml"); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := []struct {
		name string
		m    Manifest
		want string
	}{
		{
			name: "missing version",
			m: Manifest{
				Kind:       ManifestKind,
				WorkflowID: "wf-x",
			},
			want: "missing version",
		},
		{
			name: "unsupported version",
			m: Manifest{
				Version:    99,
				Kind:       ManifestKind,
				WorkflowID: "wf-x",
			},
			want: "unsupported workflow manifest version",
		},
		{
			name: "wrong kind",
			m: Manifest{
				Version:    ManifestVersion,
				Kind:       "not-workflow",
				WorkflowID: "wf-x",
			},
			want: "kind mismatch",
		},
		{
			name: "missing workflow_id",
			m: Manifest{
				Version: ManifestVersion,
				Kind:    ManifestKind,
			},
			want: "missing workflow_id",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateManifest(&c.m, "/virtual/.workflow/workflow.yaml")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}

func TestReplayEventStream_DerivesProgress(t *testing.T) {
	events := strings.Join([]string{
		`{"event_type":"workflow_run_started"}`,
		`{"event_type":"wf_step_start"}`,
		`{"event_type":"wf_step_block"}`,
		`{"event_type":"wf_step_skip"}`,
		`{"event_type":"wf_step_start"}`,
		`{"event_type":"wf_step_done"}`,
		`{"event_type":"unknown_event"}`,
		`not-json`,
	}, "\n")

	state, err := replayEventStream(strings.NewReader(events))
	if err != nil {
		t.Fatal(err)
	}
	if state.CurrentStep != 2 || state.CompletedSteps != 2 {
		t.Errorf("CurrentStep=%d CompletedSteps=%d, want 2/2", state.CurrentStep, state.CompletedSteps)
	}
	if state.Blocked {
		t.Error("Blocked = true, want false after skip/done")
	}
	if state.Status != "active" {
		t.Errorf("Status = %q, want active", state.Status)
	}
}

func TestReplayEventStream_StatusTransitions(t *testing.T) {
	cases := []struct {
		name          string
		events        string
		wantStatus    string
		wantBlocked   bool
		wantCompleted int
	}{
		{
			name:        "block marks blocked",
			events:      `{"event_type":"wf_step_start"}` + "\n" + `{"event_type":"wf_step_block"}`,
			wantStatus:  "blocked",
			wantBlocked: true,
		},
		{
			name:          "completion marks completed",
			events:        `{"event_type":"wf_step_start"}` + "\n" + `{"event_type":"wf_step_done"}` + "\n" + `{"event_type":"workflow_run_completed"}`,
			wantStatus:    "completed",
			wantCompleted: 1,
		},
		{
			name:       "abandon marks abandoned",
			events:     `{"event_type":"workflow_run_abandoned"}`,
			wantStatus: "abandoned",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, err := replayEventStream(strings.NewReader(c.events))
			if err != nil {
				t.Fatal(err)
			}
			if state.Status != c.wantStatus {
				t.Errorf("Status = %q, want %q", state.Status, c.wantStatus)
			}
			if state.Blocked != c.wantBlocked {
				t.Errorf("Blocked = %v, want %v", state.Blocked, c.wantBlocked)
			}
			if state.CompletedSteps != c.wantCompleted {
				t.Errorf("CompletedSteps = %d, want %d", state.CompletedSteps, c.wantCompleted)
			}
		})
	}
}

func TestNewRunID(t *testing.T) {
	ts := time.Date(2026, 5, 18, 12, 34, 56, 0, time.UTC)
	if got := newRunID(ts); got != "run-20260518T123456Z" {
		t.Fatalf("newRunID() = %q", got)
	}
}

func TestStore_InitReturnsCanceledContextBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Open("/virtual/.workflow", "/virtual/WORKFLOW.md").Init(ctx, InitOptions{WorkflowID: "wf-x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func writeStoreWorkflowDoc(t *testing.T, dir string) string {
	t.Helper()
	doc := filepath.Join(dir, "WORKFLOW.md")
	body := `---
workflow_version: 1
workflow_id: wf-test
---

## Step 1: ALIGN

**Goal:** prove routing.

## Step 2: EXECUTE

**Goal:** do the work.
`
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestLoadActive_StaleSummaryDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	doc := writeStoreWorkflowDoc(t, dir)
	store := Open(filepath.Join(dir, ".workflow"), doc)
	ctx := context.Background()
	if err := store.Init(ctx, InitOptions{WorkflowID: "wf-test"}); err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, ".workflow", runsDir, runID)
	if err := store.appendEvent(ctx, runDir, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := store.appendEvent(ctx, runDir, Event{EventType: EventStepDone}); err != nil {
		t.Fatal(err)
	}

	runPath := filepath.Join(runDir, runManifestName)
	before, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	var beforeRM RunManifest
	if err := yaml.Unmarshal(before, &beforeRM); err != nil {
		t.Fatal(err)
	}
	if beforeRM.Summary.CurrentStep != 0 || beforeRM.Summary.CompletedSteps != 0 {
		t.Fatalf("precondition: expected stale summary 0/0, got %d/%d",
			beforeRM.Summary.CurrentStep, beforeRM.Summary.CompletedSteps)
	}

	state, err := store.LoadActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("LoadActive returned nil state")
	}
	if state.CurrentStep != 1 || state.CompletedSteps != 1 {
		t.Fatalf("LoadActive state = %d/%d, want 1/1", state.CurrentStep, state.CompletedSteps)
	}

	after, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("LoadActive rewrote run.yaml; before:\n%s\nafter:\n%s", before, after)
	}
}

func TestAppendEvent_UpdatesPersistedSummary(t *testing.T) {
	dir := t.TempDir()
	doc := writeStoreWorkflowDoc(t, dir)
	store := Open(filepath.Join(dir, ".workflow"), doc)
	ctx := context.Background()
	if err := store.Init(ctx, InitOptions{WorkflowID: "wf-test"}); err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, Event{EventType: EventStepDone}); err != nil {
		t.Fatal(err)
	}

	runPath := filepath.Join(dir, ".workflow", runsDir, runID, runManifestName)
	raw, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	var rm RunManifest
	if err := yaml.Unmarshal(raw, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Summary.CurrentStep != 1 || rm.Summary.CompletedSteps != 1 {
		t.Fatalf("persisted summary = %d/%d, want 1/1", rm.Summary.CurrentStep, rm.Summary.CompletedSteps)
	}
	if rm.Summary.Blocked {
		t.Fatal("persisted summary.blocked = true, want false")
	}
}
