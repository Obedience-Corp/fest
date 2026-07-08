package workflow

import (
	"context"
	stderrors "errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func withApprovalJudgeRunner(t *testing.T, runner approvalJudgeRunner) {
	t.Helper()
	orig := runApprovalJudgeCommand
	runApprovalJudgeCommand = runner
	t.Cleanup(func() {
		runApprovalJudgeCommand = orig
	})
}

func testApprovalJudgeRequest() approvalJudgeRequest {
	return approvalJudgeRequest{
		SchemaVersion: approvalJudgeSchemaVersion,
		FestivalPath:  "/campaign/festivals/active/example",
		PhasePath:     "/campaign/festivals/active/example/001_PLAN",
		Document:      "GATES.md",
		StepNumber:    1,
		StepName:      "VERIFY",
		Goal:          "Verify the gate criteria.",
		Actions:       []string{"Inspect evidence"},
		Output:        "Gate decision",
		Checkpoint:    "user_approval",
	}
}

func TestApproveCommandManualModeDefaultOff(t *testing.T) {
	cmd := newApproveCmd()

	auto, err := cmd.Flags().GetBool("auto")
	if err != nil {
		t.Fatalf("GetBool(auto): %v", err)
	}
	if auto {
		t.Fatal("auto approval must be default-off")
	}

	judgeCommand, err := cmd.Flags().GetString("judge-command")
	if err != nil {
		t.Fatalf("GetString(judge-command): %v", err)
	}
	if judgeCommand != "ob judge" {
		t.Fatalf("judge-command = %q, want %q", judgeCommand, "ob judge")
	}
}

func TestEvaluateApprovalJudge_ApproveDecision(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		if command != "fake judge" {
			t.Fatalf("command = %q", command)
		}
		if !strings.Contains(string(stdin), `"schema_version":"fest.approval.judge/v1"`) {
			t.Fatalf("stdin missing schema: %s", stdin)
		}
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"evidence satisfies the checklist","confidence":0.92,"followups":[]}`), nil
	})

	decision, audit, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("evaluateApprovalJudge: %v", err)
	}
	if decision.Decision != "approve" {
		t.Fatalf("decision = %q", decision.Decision)
	}
	if !strings.Contains(audit, "decision=approve") || !strings.Contains(audit, `reason="evidence satisfies the checklist"`) {
		t.Fatalf("audit missing decision/reason: %q", audit)
	}
}

func TestEvaluateApprovalJudge_RejectDecision(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"reject","reason":"missing test evidence"}`), nil
	})

	decision, audit, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("evaluateApprovalJudge: %v", err)
	}
	if decision.Decision != "reject" {
		t.Fatalf("decision = %q", decision.Decision)
	}
	if !strings.Contains(audit, "decision=reject") || !strings.Contains(audit, `reason="missing test evidence"`) {
		t.Fatalf("audit missing decision/reason: %q", audit)
	}
}

func TestEvaluateApprovalJudge_MissingCommandFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "ob judge",
		Timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("expected missing command error")
	}
	if !strings.Contains(err.Error(), "approval judge failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateApprovalJudge_TimeoutFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "slow judge",
		Timeout:      time.Nanosecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "approval judge timed out") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateApprovalJudge_MalformedJSONFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`not json`), nil
	})

	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		Timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if !strings.Contains(err.Error(), "parsing approval judge response") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateApprovalJudge_UnknownDecisionFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"maybe","reason":"uncertain"}`), nil
	})

	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		Timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("expected unknown decision error")
	}
	if !strings.Contains(err.Error(), "unsupported decision") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateApprovalJudge_EmptyReasonFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":" "}`), nil
	})

	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		Timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("expected empty reason error")
	}
	if !strings.Contains(err.Error(), "empty reason") {
		t.Fatalf("error = %v", err)
	}
}

func TestWorkflowStateApproveWithFeedbackRecordsAudit(t *testing.T) {
	state := wf.NewWorkflowState(2)
	state.Initialize([]wf.WorkflowStep{
		{Number: 1, Name: "VERIFY"},
		{Number: 2, Name: "NEXT"},
	})

	audit := `approval auto mode: schema_version=fest.approval.judge/v1 judge_command="fake judge" decision=approve reason="ok"`
	if err := state.ApproveWithFeedback(audit); err != nil {
		t.Fatalf("ApproveWithFeedback: %v", err)
	}

	step := state.GetStepState(1)
	if step == nil {
		t.Fatal("step 1 state missing")
	}
	if step.Status != wf.StepStatusCompleted {
		t.Fatalf("status = %s", step.Status)
	}
	if step.Feedback != audit {
		t.Fatalf("feedback = %q, want audit", step.Feedback)
	}
	if state.CurrentStep != 2 {
		t.Fatalf("current step = %d, want 2", state.CurrentStep)
	}
}

func TestRunApprovalJudgeCommandDefaultEmptyCommand(t *testing.T) {
	_, err := runApprovalJudgeCommandDefault(context.Background(), " ", nil)
	if err == nil {
		t.Fatal("expected empty command error")
	}
	var execErr *exec.Error
	if stderrors.As(err, &execErr) {
		t.Fatalf("empty command should be validation, got exec error: %v", err)
	}
}
