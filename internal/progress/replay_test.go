package progress

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func writeEventLog(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ProgressDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, ProgressDir, ProgressEventsFile), []byte(body), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	return dir
}

func TestReadEventsKeepsFileOrder(t *testing.T) {
	// Go trims trailing zeros, so .655661 sorts before .65566 as text even
	// though it was written later. Fest applies events in file order.
	dir := writeEventLog(t,
		`{"ts":"2026-09-09T08:48:55.65566Z","event":"completed","task":"001_P/01_s/01_t.md"}`,
		`{"ts":"2026-09-09T08:48:55.655661Z","event":"reset","task":"001_P/01_s/01_t.md"}`,
	)
	events, err := NewStore(dir).ReadEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 || events[0].Event != EventCompleted || events[1].Event != EventReset {
		t.Fatalf("events = %+v", events)
	}
	task, ok := Replay(dir, events).GetTask("001_P/01_s/01_t.md")
	if !ok || task.Status != StatusPending {
		t.Fatalf("after completed then reset, task = %+v", task)
	}
}

func TestReadEventsWithoutALogReturnsNothing(t *testing.T) {
	events, err := NewStore(t.TempDir()).ReadEvents(context.Background())
	if err != nil || events != nil {
		t.Fatalf("events = %v, err = %v", events, err)
	}
}

func TestReplayMatchesLoadReadOnly(t *testing.T) {
	started := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	line := func(e ProgressEvent) string {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	dir := writeEventLog(t,
		line(ProgressEvent{Timestamp: started, Event: EventCompleted, Task: "001_P/01_s/01_t.md"}),
		line(ProgressEvent{Timestamp: started, Event: EventWorkflowStepStart, Phase: "gate:001_P", Step: 1}),
		line(ProgressEvent{Timestamp: started, Event: EventWorkflowJudgeStarted, Phase: "gate:001_P", Step: 1, JudgeStatus: wf.JudgeRunning, JudgeRunID: "a"}),
	)
	ctx := context.Background()
	loaded := NewStore(dir)
	if err := loaded.LoadReadOnly(ctx); err != nil {
		t.Fatalf("LoadReadOnly: %v", err)
	}
	events, err := loaded.ReadEvents(ctx)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	replayed := Replay(dir, events)

	want, _ := loaded.GatePhaseState("001_P")
	got, ok := replayed.GatePhaseState("001_P")
	if !ok || got.GetStepState(1).Judge.Status != want.GetStepState(1).Judge.Status {
		t.Fatalf("replayed gate state differs: got %+v want %+v", got.GetStepState(1), want.GetStepState(1))
	}
	if task, _ := replayed.GetTask("001_P/01_s/01_t.md"); task.Status != StatusCompleted {
		t.Fatalf("replayed task = %+v", task)
	}
}
