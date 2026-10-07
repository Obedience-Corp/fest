package localstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func makeReadOnly(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores file modes, so a read-only file stays writable")
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

func countEvents(t *testing.T, eventsPath, eventType string) int {
	t.Helper()
	f, err := os.Open(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	n := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var evt Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			t.Fatalf("decoding event line %q: %v", scanner.Text(), err)
		}
		if evt.EventType == eventType {
			n++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return n
}

type startedRun struct {
	store      *Store
	runID      string
	runPath    string
	eventsPath string
	warnings   *bytes.Buffer
}

func startTestRun(t *testing.T) startedRun {
	t.Helper()
	dir := t.TempDir()
	doc := writeStoreWorkflowDoc(t, dir)
	store := Open(filepath.Join(dir, ".workflow"), doc)
	ctx := t.Context()
	if err := store.Init(ctx, InitOptions{WorkflowID: "wf-test"}); err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartRun(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, ".workflow", runsDir, runID)
	warnings := &bytes.Buffer{}
	store.warnOut = warnings
	return startedRun{
		store:      store,
		runID:      runID,
		runPath:    filepath.Join(runDir, runManifestName),
		eventsPath: filepath.Join(runDir, eventsName),
		warnings:   warnings,
	}
}

func assertOneCacheWarning(t *testing.T, warnings *bytes.Buffer, eventType string) {
	t.Helper()
	got := warnings.String()
	if strings.Count(got, "Warning: ") != 1 {
		t.Fatalf("warnings = %q, want exactly one", got)
	}
	for _, want := range []string{eventType + " recorded", "run.yaml summary cache was not refreshed", "permission denied"} {
		if !strings.Contains(got, want) {
			t.Fatalf("warning %q does not mention %q", got, want)
		}
	}
	if strings.Contains(got, "Hint: ") {
		t.Fatalf("warning %q carries a retry hint for an operation that succeeded", got)
	}
}

func TestAppendEvent_UnwritableRunSummaryKeepsStepDoneCommitted(t *testing.T) {
	run := startTestRun(t)
	ctx := t.Context()
	if err := run.store.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	makeReadOnly(t, run.runPath)
	before, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := run.store.AppendEvent(ctx, Event{EventType: EventStepDone}); err != nil {
		t.Fatalf("AppendEvent returned %v after the event was recorded; a caller would retry and duplicate progress", err)
	}
	assertOneCacheWarning(t, run.warnings, EventStepDone)

	if got := countEvents(t, run.eventsPath, EventStepDone); got != 1 {
		t.Fatalf("%s events = %d, want 1", EventStepDone, got)
	}
	after, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("precondition: run.yaml changed although it is read-only")
	}
	state, err := run.store.LoadActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.CurrentStep != 1 || state.CompletedSteps != 1 || state.Status != "active" {
		t.Fatalf("LoadActive = %+v, want step 1 started and completed, status active", state)
	}
}

func TestAppendEvent_UnwritableRunSummaryStillFinalizesCompletedRun(t *testing.T) {
	run := startTestRun(t)
	ctx := t.Context()
	for _, evt := range []string{EventStepStart, EventStepDone, EventStepStart, EventStepDone} {
		if err := run.store.AppendEvent(ctx, Event{EventType: evt}); err != nil {
			t.Fatal(err)
		}
	}
	makeReadOnly(t, run.runPath)

	if err := run.store.AppendEvent(ctx, Event{EventType: EventWorkflowRunCompleted}); err != nil {
		t.Fatalf("AppendEvent returned %v after the completion event was recorded", err)
	}
	assertOneCacheWarning(t, run.warnings, EventWorkflowRunCompleted)

	if got := countEvents(t, run.eventsPath, EventWorkflowRunCompleted); got != 1 {
		t.Fatalf("%s events = %d, want 1", EventWorkflowRunCompleted, got)
	}
	m, err := run.store.LoadManifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.ActiveRunID != "" {
		t.Fatalf("workflow.yaml active_run_id = %q, want cleared", m.ActiveRunID)
	}
	if len(m.Runs) != 1 || m.Runs[0].RunID != run.runID || m.Runs[0].Status != "completed" || m.Runs[0].EndedAt == "" {
		t.Fatalf("workflow.yaml runs = %+v, want %s completed with ended_at", m.Runs, run.runID)
	}
	if rm := readRunManifest(t, run.runPath); rm.Status != "active" {
		t.Fatalf("precondition: read-only run.yaml status = %q, want the stale active cache", rm.Status)
	}
	state, err := replayEvents(run.eventsPath, readRunManifest(t, run.runPath))
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "completed" || state.CurrentStep != 2 || state.CompletedSteps != 2 {
		t.Fatalf("replay = %+v, want completed with 2/2 steps", state)
	}
	if active, err := run.store.LoadActive(ctx); err != nil || active != nil {
		t.Fatalf("LoadActive = %+v, %v; want no active run", active, err)
	}
	if _, err := run.store.StartRun(ctx, ""); err != nil {
		t.Fatalf("StartRun after completion: %v", err)
	}
}

func TestAppendEvent_UnwritableManifestReportsRecordedTerminalEvent(t *testing.T) {
	run := startTestRun(t)
	ctx := t.Context()
	manifestPath := run.store.ManifestPath()
	makeReadOnly(t, manifestPath)

	err := run.store.AppendEvent(ctx, Event{EventType: EventWorkflowRunCompleted})
	if err == nil {
		t.Fatal("AppendEvent returned nil although workflow.yaml could not be finalized")
	}

	if got := countEvents(t, run.eventsPath, EventWorkflowRunCompleted); got != 1 {
		t.Fatalf("%s events = %d, want 1", EventWorkflowRunCompleted, got)
	}
	msg := festerrors.Message(err)
	if !strings.Contains(msg, EventWorkflowRunCompleted+" recorded") {
		t.Fatalf("error %q does not say the event was recorded", msg)
	}
	var fe *festerrors.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error %T is not a festerrors.Error", err)
	}
	if fe.Fields["event_recorded"] != true {
		t.Fatalf("error fields = %v, want event_recorded=true", fe.Fields)
	}
	if !strings.Contains(err.Error(), "Hint: ") || !strings.Contains(err.Error(), "already recorded") {
		t.Fatalf("error %q has no hint saying the event is already recorded", err.Error())
	}
	if rm := readRunManifest(t, run.runPath); rm.Status != "completed" {
		t.Fatalf("run.yaml status = %q, want completed: the summary sync must still run when finalize fails", rm.Status)
	}
	if run.warnings.Len() != 0 {
		t.Fatalf("warnings = %q, want none when only workflow.yaml is read-only", run.warnings.String())
	}
}
