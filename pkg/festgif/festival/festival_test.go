package festival

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/pkg/festgif"
)

const (
	root  = "/camp/festivals/active/demo-DM0001"
	phase = "002_IMPLEMENT"
	task  = phase + "/01_build/01_task.md"
)

func fixtureTree(gateStatus, gateJudge string) *show.DisplayNode {
	return &show.DisplayNode{NodeType: "festival", Children: []*show.DisplayNode{{
		Name: phase, NodeType: "phase", Children: []*show.DisplayNode{
			{Name: "01_build", NodeType: "sequence", Children: []*show.DisplayNode{
				{Name: "01_task.md", NodeType: "task", Status: "completed", TaskPath: filepath.Join(root, task)},
			}},
			{Name: "Gate 1: PHASE GOAL", NodeType: "step", Status: gateStatus, JudgeStatus: gateJudge,
				StateKey: "gate:" + phase, StepNumber: 1},
		},
	}}}
}

type eventLog struct{ events []progress.ProgressEvent }

func (l *eventLog) add(e progress.ProgressEvent) {
	e.Timestamp = time.Date(2026, 9, 10, 0, 0, len(l.events), 0, time.UTC)
	l.events = append(l.events, e)
}

func (l *eventLog) gate(ev progress.EventType, e progress.ProgressEvent) {
	e.Event, e.Phase, e.Step = ev, "gate:"+phase, 1
	l.add(e)
}

func (l *eventLog) judgeHook(ms int64) {
	l.gate(progress.EventWorkflowHookRun, progress.ProgressEvent{
		HookName: "approval_judge", HookLayer: "festivals", HookTiming: "post",
		HookVerb: "gate_approve", HookOutcome: "pass", HookMillis: ms,
	})
}

func rejectedThenApproved() []progress.ProgressEvent {
	l := &eventLog{}
	l.add(progress.ProgressEvent{Event: progress.EventCompleted, Task: task})
	l.gate(progress.EventWorkflowStepStart, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "a"})
	l.judgeHook(11200)
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "rejected", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowStepBlock, progress.ProgressEvent{DecisionActor: "agent", Feedback: "missing evidence"})
	l.gate(progress.EventWorkflowJudgeRecheck, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "b"})
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "approved", JudgeRunID: "stale"})
	l.judgeHook(9000)
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "approved", JudgeRunID: "b"})
	l.gate(progress.EventWorkflowStepDone, progress.ProgressEvent{DecisionActor: "agent"})
	return l.events
}

// gateStates lists every state the gate shows, in order.
func gateStates(in festgif.Input) []string {
	var out []string
	for _, b := range in.Beats {
		for _, c := range b.Changes {
			if c.Key == stepKey("gate:"+phase, 1) {
				judge := c.State.Judge
				if judge == "" {
					judge = "-"
				}
				out = append(out, c.State.Status+"/"+judge)
			}
		}
	}
	return out
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestBuildInputReplaysAJudgeRejectionLoop(t *testing.T) {
	in := build("demo", root, fixtureTree("completed", "approved"), rejectedThenApproved())
	equal(t, gateStates(in), []string{
		"in_progress/-",
		"in_progress/running",
		"in_progress/rejected",
		"blocked/rejected",
		"in_progress/-",
		"in_progress/running",
		"in_progress/approved",
		"completed/approved",
	})
}

func TestBuildInputIgnoresAStaleVerdict(t *testing.T) {
	in := build("demo", root, fixtureTree("completed", "approved"), rejectedThenApproved())
	var order []string
	for _, b := range in.Beats {
		if b.Hook != nil {
			order = append(order, fmt.Sprintf("hook %dms", b.Hook.Millis))
		}
		for _, c := range b.Changes {
			if c.State.Judge == "approved" && c.State.Status == "in_progress" {
				order = append(order, "approved")
			}
		}
	}
	equal(t, order, []string{"hook 11200ms", "hook 9000ms", "approved"})
}

func TestBuildInputKeepsAHumanRejectionBlocked(t *testing.T) {
	l := &eventLog{}
	l.gate(progress.EventWorkflowStepStart, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "rejected", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowStepBlock, progress.ProgressEvent{DecisionActor: "human", Feedback: "not yet"})
	l.gate(progress.EventWorkflowJudgeRecheck, progress.ProgressEvent{})
	in := build("demo", root, fixtureTree("blocked", "rejected"), l.events)
	equal(t, gateStates(in), []string{
		"in_progress/-",
		"in_progress/running",
		"in_progress/rejected",
		"blocked/rejected",
	})
}

func TestBuildInputHoldsJudgeMoments(t *testing.T) {
	in := build("demo", root, fixtureTree("completed", "approved"), rejectedThenApproved())
	var holds []festgif.Hold
	for _, b := range in.Beats {
		if b.Hold != festgif.HoldNone {
			holds = append(holds, b.Hold)
		}
	}
	want := []festgif.Hold{festgif.HoldJudgeWait, festgif.HoldRejection, festgif.HoldBlocked, festgif.HoldJudgeWait, festgif.HoldVerdict}
	equal(t, fmtHolds(holds), fmtHolds(want))
}

func fmtHolds(h []festgif.Hold) []string {
	out := make([]string, len(h))
	for i, v := range h {
		out[i] = fmt.Sprint(int(v))
	}
	return out
}

func TestBuildInputPinsHooksToTheirRows(t *testing.T) {
	l := &eventLog{}
	l.add(progress.ProgressEvent{Event: progress.EventWorkflowHookRun, Phase: phase, Task: task,
		HookName: "lint", HookTiming: "post", HookVerb: "task_complete", HookOutcome: "fail", HookBlocked: true})
	l.add(progress.ProgressEvent{Event: progress.EventWorkflowHookRun, Phase: phase, Task: phase + "/01_build",
		HookName: "notify", HookTiming: "post", HookVerb: "sequence_complete", HookOutcome: "pass"})
	l.add(progress.ProgressEvent{Event: progress.EventWorkflowHookRun, Phase: phase,
		HookName: "report", HookTiming: "post", HookVerb: "phase_complete", HookOutcome: "pass"})
	l.judgeHook(500)
	in := build("demo", root, fixtureTree("completed", "approved"), l.events)
	var keys []string
	for _, b := range in.Beats {
		if b.Hook != nil {
			keys = append(keys, b.Hook.Key)
		}
	}
	equal(t, keys, []string{"task:" + task, "seq:" + phase + "/01_build", "phase:" + phase, stepKey("gate:"+phase, 1)})
	if in.Beats[0].Hold != festgif.HoldHookFail {
		t.Errorf("failed hook hold = %v, want HoldHookFail", in.Beats[0].Hold)
	}
}

func TestBuildInputTakesFinalStateFromTheTree(t *testing.T) {
	in := build("demo", root, fixtureTree("completed", "approved"), nil)
	if len(in.Beats) != 0 {
		t.Fatalf("no events should mean no beats, got %d", len(in.Beats))
	}
	want := festgif.State{Status: "completed", Judge: "approved"}
	if got := in.Final[stepKey("gate:"+phase, 1)]; got != want {
		t.Fatalf("final gate = %+v, want %+v", got, want)
	}
	if got := in.Final["task:"+task]; got.Status != "completed" {
		t.Fatalf("final task = %+v", got)
	}
}
