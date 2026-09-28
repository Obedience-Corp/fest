package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDeferralEventsRoundTrip(t *testing.T) {
	ctx := t.Context()
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	want := []ProgressEvent{
		{
			Timestamp:      at,
			Event:          EventBlockerDeferred,
			Task:           "001_PHASE/01_seq/01_task.md",
			Reason:         "provider API removed",
			DeferralReason: "shipping without it, the vendor replies next week",
			DeferredBy:     "Ada Lovelace",
			Attempts:       []string{"checked the changelog", "asked the vendor"},
			Actor:          "operator",
			TTY:            true,
			AgentMarkers:   []string{"CLAUDECODE", "CURSOR_AGENT"},
			Ancestry:       []string{"zsh", "fest"},
		},
		{
			Timestamp: at.Add(time.Minute),
			Event:     EventSweepStarted,
			Sweep:     1,
		},
		{
			Timestamp: at.Add(2 * time.Minute),
			Event:     EventBlockerRevisited,
			Task:      "001_PHASE/01_seq/01_task.md",
			Sweep:     1,
		},
		{
			Timestamp:    at.Add(3 * time.Minute),
			Event:        EventForcedComplete,
			Actor:        "operator",
			TTY:          true,
			AgentMarkers: []string{"CLAUDECODE"},
			Ancestry:     []string{"zsh", "fest"},
			DroppedTasks: []string{"001_PHASE/01_seq/01_task.md"},
		},
		{
			Timestamp: at.Add(4 * time.Minute),
			Event:     EventUnblocked,
			Task:      "001_PHASE/01_seq/01_task.md",
			Note:      "split the call into two requests and try again",
		},
	}

	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for i := range want {
		store.QueueEvent(&want[i])
	}
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ProgressDir, ProgressEventsFile))
	if err != nil {
		t.Fatalf("reading events file: %v", err)
	}
	t.Logf("written log:\n%s", raw)

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("wrote %d lines, want %d", len(lines), len(want))
	}
	const wantSweepLine = `{"ts":"2026-09-28T12:01:00Z","event":"sweep_started","sweep":1}`
	if lines[1] != wantSweepLine {
		t.Errorf("sweep line =\n%s\nwant\n%s", lines[1], wantSweepLine)
	}
	const wantRevisitLine = `{"ts":"2026-09-28T12:02:00Z","event":"blocker_revisited","task":"001_PHASE/01_seq/01_task.md","sweep":1}`
	if lines[2] != wantRevisitLine {
		t.Errorf("revisit line =\n%s\nwant\n%s", lines[2], wantRevisitLine)
	}

	got, err := NewStore(dir).ReadEvents(ctx)
	if err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("event %d round-tripped as\n%+v\nwant\n%+v", i, got[i], want[i])
		}
	}
}

func TestMinimalEventMarshalsWithoutDeferralKeys(t *testing.T) {
	raw, err := json.Marshal(ProgressEvent{
		Timestamp: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		Event:     EventBlocked,
		Task:      "01_task.md",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	const want = `{"ts":"2026-09-28T12:00:00Z","event":"blocked","task":"01_task.md"}`
	if string(raw) != want {
		t.Errorf("marshalled as\n%s\nwant\n%s", raw, want)
	}
}
