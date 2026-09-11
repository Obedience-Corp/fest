package gif

import (
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
)

const (
	phase = "002_IMPLEMENT"
	task  = phase + "/01_build/01_task.md"
)

type eventLog struct{ events []progress.ProgressEvent }

func (l *eventLog) add(e progress.ProgressEvent) {
	e.Timestamp = time.Date(2026, 9, 10, 0, 0, len(l.events), 0, time.UTC)
	l.events = append(l.events, e)
}

func (l *eventLog) gate(ev progress.EventType, e progress.ProgressEvent) {
	e.Event, e.Phase, e.Step = ev, "gate:"+phase, 1
	l.add(e)
}

func rejectedThenApproved() []progress.ProgressEvent {
	l := &eventLog{}
	l.add(progress.ProgressEvent{Event: progress.EventCompleted, Task: task})
	l.gate(progress.EventWorkflowStepStart, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "rejected", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowStepBlock, progress.ProgressEvent{DecisionActor: "agent", Feedback: "missing evidence"})
	l.gate(progress.EventWorkflowJudgeRecheck, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "b"})
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "approved", JudgeRunID: "b"})
	l.gate(progress.EventWorkflowStepDone, progress.ProgressEvent{DecisionActor: "agent"})
	return l.events
}

func equal(t interface {
	Helper()
	Fatalf(string, ...any)
}, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got  %v\nwant %v", got, want)
		}
	}
}
