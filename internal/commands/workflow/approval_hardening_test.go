package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestCheckAutoJudgeAllowed_OperatorAttestation(t *testing.T) {
	step := wf.WorkflowStep{
		Number:          3,
		Name:            "APPROVAL",
		Goal:            "Did the user validate the structured output?",
		Checkpoint:      wf.CheckpointUserApproval,
		CheckpointClass: wf.CheckpointClassOperatorAttestation,
	}
	err := checkAutoJudgeAllowed(step)
	if err == nil {
		t.Fatal("expected error for operator_attestation")
	}
	if !strings.Contains(err.Error(), "operator_attestation") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckApprovalReadiness_PresentMissingPresentation(t *testing.T) {
	phase := t.TempDir()
	step := wf.WorkflowStep{
		Number:     4,
		Name:       "PRESENT",
		Checkpoint: wf.CheckpointUserApproval,
	}
	err := checkApprovalReadiness(phase, step)
	if err == nil {
		t.Fatal("expected readiness failure")
	}
	if !strings.Contains(err.Error(), "PRESENTATION.md") && !strings.Contains(err.Error(), "presentation") {
		t.Fatalf("error = %v, want presentation mention", err)
	}
}

func TestCheckApprovalReadiness_PresentWithFiles(t *testing.T) {
	phase := t.TempDir()
	specs := filepath.Join(phase, "output_specs")
	if err := os.MkdirAll(specs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PRESENTATION.md", "purpose.md", "requirements.md", "constraints.md", "context.md"} {
		if err := os.WriteFile(filepath.Join(specs, name), []byte("# ok\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	step := wf.WorkflowStep{
		Number:     4,
		Name:       "PRESENT",
		Checkpoint: wf.CheckpointUserApproval,
	}
	if err := checkApprovalReadiness(phase, step); err != nil {
		t.Fatalf("checkApprovalReadiness: %v", err)
	}
}

func TestCheckApprovalReadiness_NonPresentSkips(t *testing.T) {
	step := wf.WorkflowStep{
		Number:     2,
		Name:       "ANALYZE",
		Checkpoint: wf.CheckpointUserApproval,
	}
	if err := checkApprovalReadiness(t.TempDir(), step); err != nil {
		t.Fatalf("non-present step should skip readiness file checks: %v", err)
	}
}

func TestResolveManualApprovalDecision_NoJudgeHookAllowsNonTTY(t *testing.T) {
	decision, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Actor: decisionActorUser, Summary: "ok"},
		false, false, nil, 1, "TEST",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Actor != decisionActorUser {
		t.Fatalf("actor = %q", decision.Actor)
	}
}

func TestResolveManualApprovalDecision_JudgeConfiguredRequiresTTY(t *testing.T) {
	orig := stdinIsInteractiveFn
	stdinIsInteractiveFn = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractiveFn = orig })

	_, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Actor: decisionActorUser},
		true, false, nil, 4, "PRESENT",
	)
	if err == nil {
		t.Fatal("expected non-TTY refusal when judge configured")
	}
	if !strings.Contains(err.Error(), "interactive operator TTY") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveManualApprovalDecision_OverrideJudge(t *testing.T) {
	orig := stdinIsInteractiveFn
	stdinIsInteractiveFn = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractiveFn = orig })

	blocked := &wf.StepState{
		Status:        wf.StepStatusBlocked,
		DecisionActor: decisionActorAgent,
		Feedback:      "approval readiness: missing presentation",
		Judge:         &wf.JudgeState{Status: wf.JudgeRejected, Detail: "thin evidence"},
	}
	decision, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Summary: "I reviewed output_specs and accept them as written"},
		true, true, blocked, 4, "PRESENT",
	)
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if decision.Actor != decisionActorUserOverride {
		t.Fatalf("actor = %q, want user_override", decision.Actor)
	}
}

func TestResolveManualApprovalDecision_OverrideRequiresSummary(t *testing.T) {
	_, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Summary: "short"},
		true, true, nil, 1, "TEST",
	)
	if err == nil {
		t.Fatal("expected short summary rejection")
	}
}

func TestResolveManualApprovalDecision_InteractiveConfirm(t *testing.T) {
	origTTY := stdinIsInteractiveFn
	origRead := readOperatorConfirm
	stdinIsInteractiveFn = func() bool { return true }
	readOperatorConfirm = func() (string, error) { return operatorApproveToken, nil }
	t.Cleanup(func() {
		stdinIsInteractiveFn = origTTY
		readOperatorConfirm = origRead
	})

	blocked := &wf.StepState{
		Status: wf.StepStatusBlocked,
		Judge:  &wf.JudgeState{Status: wf.JudgeRejected, Detail: "nope"},
	}
	decision, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{},
		true, false, blocked, 4, "PRESENT",
	)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}
	if decision.Actor != decisionActorUserOverride {
		t.Fatalf("actor = %q after prior reject, want user_override", decision.Actor)
	}
}

func TestEvaluateApprovalJudge_PassesWorkDir(t *testing.T) {
	var gotDir string
	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte, dir string) ([]byte, error) {
		gotDir = dir
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"ok"}`), nil
	})
	_, _, err := evaluateApprovalJudge(context.Background(), testApprovalJudgeRequest(), approvalJudgeOptions{
		JudgeCommand: "fake judge",
		WorkDir:      "/festival/root",
	})
	if err != nil {
		t.Fatalf("evaluateApprovalJudge: %v", err)
	}
	if gotDir != "/festival/root" {
		t.Fatalf("WorkDir = %q, want /festival/root", gotDir)
	}
}
