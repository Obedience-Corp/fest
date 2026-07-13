package workflow

import (
	"context"
	stderrors "errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
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
	if judgeCommand != "" {
		t.Fatalf("judge-command default = %q, want empty (resolved from hook or flag)", judgeCommand)
	}
}

func TestResolveApprovalJudgeCommand(t *testing.T) {
	festivalsRoot := t.TempDir()
	cfg := config.DefaultWorkspaceConfig()
	cfg.Hooks.ApprovalJudge.Command = "my-judge --strict"
	if err := config.SaveWorkspaceConfig(festivalsRoot, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}
	wsCtx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: festivalsRoot})

	// Flag wins over the hook (and is trimmed).
	got, err := resolveApprovalJudgeCommand(wsCtx, "  flag-judge  ")
	if err != nil {
		t.Fatalf("flag precedence: %v", err)
	}
	if got != "flag-judge" {
		t.Fatalf("flag precedence = %q, want flag-judge", got)
	}

	// Hook is used when no flag is passed.
	got, err = resolveApprovalJudgeCommand(wsCtx, "")
	if err != nil {
		t.Fatalf("hook resolution: %v", err)
	}
	if got != "my-judge --strict" {
		t.Fatalf("hook resolution = %q, want %q", got, "my-judge --strict")
	}

	// Fails closed when the workspace has no configured hook.
	emptyCtx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: t.TempDir()})
	if _, err := resolveApprovalJudgeCommand(emptyCtx, ""); err == nil {
		t.Fatal("expected fail-closed error when no judge is configured")
	}

	// Fails closed when there is no workspace in context.
	if _, err := resolveApprovalJudgeCommand(context.Background(), ""); err == nil {
		t.Fatal("expected fail-closed error when no workspace is resolvable")
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

// Judge evaluations can run long; a default deadline would kill them
// mid-inference. Zero timeout must mean no deadline on the judge context.
func TestEvaluateApprovalJudge_ZeroTimeoutMeansNoDeadline(t *testing.T) {
	var hadDeadline bool
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		_, hadDeadline = ctx.Deadline()
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"ok"}`), nil
	})

	if _, _, err := evaluateApprovalJudge(context.Background(), approvalJudgeRequest{}, approvalJudgeOptions{
		JudgeCommand: "fake judge",
	}); err != nil {
		t.Fatalf("evaluateApprovalJudge: %v", err)
	}
	if hadDeadline {
		t.Fatal("zero judge timeout must not impose a deadline")
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

func TestWorkflowStateApproveWithAuditRecordsAuditAndDecision(t *testing.T) {
	state := wf.NewWorkflowState(2)
	state.Initialize([]wf.WorkflowStep{
		{Number: 1, Name: "VERIFY"},
		{Number: 2, Name: "NEXT"},
	})

	audit := `approval auto mode: schema_version=fest.approval.judge/v1 judge_command="fake judge" decision=approve reason="ok"`
	decision := wf.DecisionMetadata{Actor: "agent", Summary: "ok"}
	if err := state.ApproveWithAudit(audit, decision); err != nil {
		t.Fatalf("ApproveWithAudit: %v", err)
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
	if step.DecisionActor != "agent" {
		t.Fatalf("decision actor = %q, want agent", step.DecisionActor)
	}
	if step.DecisionSummary != "ok" {
		t.Fatalf("decision summary = %q, want ok", step.DecisionSummary)
	}
	if step.DecisionAt == nil {
		t.Fatal("decision timestamp missing")
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

func TestRunApproveAuto_ApproveAdvancesAndRecordsAudit(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	// Advance past step 1 (no checkpoint) to the blocking checkpoint at step 2.
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()
	if !steps[1].Checkpoint.IsBlocking() {
		t.Fatalf("fixture step 2 must be a blocking checkpoint")
	}

	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"evidence complete"}`), nil
	})

	// Wait:true = in-process verdict path (mocked runner). Default --auto is async.
	out := captureStdout(t, func() {
		if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
			JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
		}); err != nil {
			t.Fatalf("runApproveAuto: %v", err)
		}
	})
	if !strings.Contains(out, "auto-approved") {
		t.Fatalf("output missing auto-approved: %q", out)
	}

	state := nav.GetWorkflowState()
	if state.CurrentStep != 3 {
		t.Fatalf("current step = %d, want 3 (advanced past approved checkpoint)", state.CurrentStep)
	}
	step2 := state.GetStepState(2)
	if step2 == nil || step2.Status != wf.StepStatusCompleted {
		t.Fatalf("step 2 status = %+v, want completed", step2)
	}
	if !strings.Contains(step2.Feedback, "decision=approve") {
		t.Fatalf("step 2 feedback missing judge audit: %q", step2.Feedback)
	}
}

func TestRunApproveAuto_RejectBlocksStepWithAudit(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"reject","reason":"missing acceptance proof"}`), nil
	})

	out := captureStdout(t, func() {
		if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
			JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
		}); err != nil {
			t.Fatalf("runApproveAuto: %v", err)
		}
	})
	if !strings.Contains(out, "auto-rejected") {
		t.Fatalf("output missing auto-rejected: %q", out)
	}

	state := nav.GetWorkflowState()
	step2 := state.GetStepState(2)
	if step2 == nil || step2.Status != wf.StepStatusBlocked {
		t.Fatalf("step 2 status = %+v, want blocked", step2)
	}
	if !strings.Contains(step2.Feedback, "decision=reject") {
		t.Fatalf("step 2 feedback missing judge audit: %q", step2.Feedback)
	}
	if state.CurrentStep != 2 {
		t.Fatalf("current step = %d, want 2 (stays on blocked step)", state.CurrentStep)
	}
}
