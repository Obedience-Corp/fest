package localstore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

func makeDirReadOnly(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes, so a read-only directory stays writable")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func assertDirEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s holds %v, want %v (a leftover temp file?)", dir, got, want)
	}
}

func TestWriteFileAtomic_FailedWriteKeepsOriginal(t *testing.T) {
	run := startTestRun(t)
	before, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	errDiskFull := errors.New("no space left on device")
	partial := io.MultiReader(strings.NewReader("version: 1\nkind: workflow-r"), iotest.ErrReader(errDiskFull))

	err = writeFileAtomic(run.runPath, partial)
	if !errors.Is(err, errDiskFull) {
		t.Fatalf("writeFileAtomic error = %v, want the disk-full error", err)
	}

	after, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("run.yaml changed by a failed write:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if rm := readRunManifest(t, run.runPath); rm.RunID != run.runID || rm.Kind != RunKind {
		t.Fatalf("run.yaml parses to run_id=%q kind=%q, want %q/%q", rm.RunID, rm.Kind, run.runID, RunKind)
	}
	assertDirEntries(t, filepath.Dir(run.runPath), runManifestName, eventsName)
}

func TestAppendEvent_SummaryRefreshReplacesFilesAndKeepsMode(t *testing.T) {
	run := startTestRun(t)
	ctx := t.Context()
	if err := os.Chmod(run.runPath, 0o640); err != nil {
		t.Fatal(err)
	}

	for _, evt := range []string{EventStepStart, EventStepDone} {
		if err := run.store.AppendEvent(ctx, Event{EventType: evt}); err != nil {
			t.Fatal(err)
		}
	}

	if rm := readRunManifest(t, run.runPath); rm.Summary.CurrentStep != 1 || rm.Summary.CompletedSteps != 1 {
		t.Fatalf("persisted summary = %d/%d, want 1/1", rm.Summary.CurrentStep, rm.Summary.CompletedSteps)
	}
	fi, err := os.Stat(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o640 {
		t.Fatalf("run.yaml mode = %v, want 0640 kept across the refresh", got)
	}
	assertDirEntries(t, filepath.Dir(run.runPath), runManifestName, eventsName)

	for _, evt := range []string{EventStepStart, EventStepDone, EventWorkflowRunCompleted} {
		if err := run.store.AppendEvent(ctx, Event{EventType: evt}); err != nil {
			t.Fatal(err)
		}
	}
	m, err := run.store.LoadManifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.ActiveRunID != "" || m.Runs[0].Status != "completed" {
		t.Fatalf("workflow.yaml active_run_id=%q status=%q, want finalized", m.ActiveRunID, m.Runs[0].Status)
	}
	assertDirEntries(t, filepath.Dir(run.store.ManifestPath()), manifestName, runsDir)
	if run.warnings.Len() != 0 {
		t.Fatalf("warnings = %q, want none", run.warnings.String())
	}
}

func TestAppendEvent_FailedSummaryRefreshLeavesRunYAMLReadable(t *testing.T) {
	run := startTestRun(t)
	ctx := t.Context()
	if err := run.store.AppendEvent(ctx, Event{EventType: EventStepStart}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Dir(run.runPath)
	makeDirReadOnly(t, runDir)

	if err := run.store.AppendEvent(ctx, Event{EventType: EventStepDone}); err != nil {
		t.Fatalf("AppendEvent returned %v after the event was recorded", err)
	}

	assertOneCacheWarning(t, run.warnings, EventStepDone)
	after, err := os.ReadFile(run.runPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("run.yaml changed by a failed refresh:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := countEvents(t, run.eventsPath, EventStepDone); got != 1 {
		t.Fatalf("%s events = %d, want 1", EventStepDone, got)
	}
	state, err := run.store.LoadActive(ctx)
	if err != nil {
		t.Fatalf("LoadActive after a failed refresh: %v", err)
	}
	if state == nil || state.CurrentStep != 1 || state.CompletedSteps != 1 {
		t.Fatalf("LoadActive = %+v, want 1/1", state)
	}
	assertDirEntries(t, runDir, runManifestName, eventsName)
}
