package shared

import (
	"strings"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func judgeStepView(judge *wf.JudgeState) WorkflowStepView {
	return WorkflowStepView{
		Number:        2,
		Name:          "REVIEW",
		Status:        wf.StepStatusInProgress,
		IsCurrent:     true,
		HasCheckpoint: true,
		Judge:         judge,
	}
}

func TestRenderWorkflowStepLine_WaitingOnJudge(t *testing.T) {
	started := time.Now()
	out := RenderWorkflowStepLine(judgeStepView(&wf.JudgeState{
		Status:    wf.JudgeRunning,
		Command:   "ob judge",
		StartedAt: &started,
	}), false)

	for _, want := range []string{"[waiting on judge]", "Judge: waiting", "⚖"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderWorkflowStepLine_JudgeFailed(t *testing.T) {
	out := RenderWorkflowStepLine(judgeStepView(&wf.JudgeState{
		Status: wf.JudgeFailed,
		Detail: "judge timed out",
	}), false)

	if !strings.Contains(out, "Judge: failed (fails closed)") || !strings.Contains(out, "judge timed out") {
		t.Errorf("failed judge not surfaced:\n%s", out)
	}
	if strings.Contains(out, "[waiting on judge]") {
		t.Errorf("failed judge must not render as waiting:\n%s", out)
	}
}

func TestRenderWorkflowStepLine_JudgeOutcomeNote(t *testing.T) {
	out := RenderWorkflowStepLine(judgeStepView(&wf.JudgeState{
		Status: wf.JudgeApproved,
		Detail: "evidence complete",
	}), false)

	if !strings.Contains(out, "Judge: approved") {
		t.Errorf("judge outcome note missing:\n%s", out)
	}
	if strings.Contains(out, "evidence complete") {
		t.Errorf("judge detail should be rendered as feedback, not metadata:\n%s", out)
	}
}
