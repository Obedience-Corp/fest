package workflow

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
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
	step := wf.WorkflowStep{
		Number:     4,
		Name:       "PRESENT",
		Checkpoint: wf.CheckpointUserApproval,
	}
	err := checkApprovalReadinessWithInspector("/phase", step, func(_, _ string) (bool, error) {
		return false, nil
	})
	if err == nil {
		t.Fatal("expected readiness failure")
	}
	if !strings.Contains(err.Error(), "PRESENTATION.md") && !strings.Contains(err.Error(), "presentation") {
		t.Fatalf("error = %v, want presentation mention", err)
	}
}

func TestCheckApprovalReadiness_PresentWithFiles(t *testing.T) {
	step := wf.WorkflowStep{
		Number:     4,
		Name:       "PRESENT",
		Checkpoint: wf.CheckpointUserApproval,
	}
	inspect := func(_, path string) (bool, error) {
		return path == "output_specs/PRESENTATION.md", nil
	}
	if err := checkApprovalReadinessWithInspector("/phase", step, inspect); err != nil {
		t.Fatalf("checkApprovalReadiness: %v", err)
	}
}

func TestCheckApprovalReadiness_PresentAcceptsExplicitEvidencePack(t *testing.T) {
	step := wf.WorkflowStep{
		Number:        4,
		Name:          "PRESENT",
		Checkpoint:    wf.CheckpointUserApproval,
		EvidencePaths: []string{"output_specs/review.md", "output_specs/demo.txt"},
	}
	if err := checkApprovalReadinessWithInspector("/phase", step, func(_, _ string) (bool, error) {
		return true, nil
	}); err != nil {
		t.Fatalf("explicit evidence pack should satisfy readiness: %v", err)
	}
}

func TestCheckApprovalReadiness_ExplicitEvidenceRequiresEveryFile(t *testing.T) {
	step := wf.WorkflowStep{
		Number:        4,
		Name:          "PRESENT",
		Checkpoint:    wf.CheckpointUserApproval,
		EvidencePaths: []string{"output_specs/review.md", "output_specs/missing.md"},
	}
	err := checkApprovalReadinessWithInspector("/phase", step, func(_, path string) (bool, error) {
		return path != "output_specs/missing.md", nil
	})
	if err == nil || !strings.Contains(err.Error(), "listed evidence") {
		t.Fatalf("missing explicit evidence error = %v", err)
	}
}

func TestCheckApprovalReadiness_InspectorErrorFailsClosed(t *testing.T) {
	step := wf.WorkflowStep{
		Number:          2,
		Name:            "REVIEW",
		Checkpoint:      wf.CheckpointUserApproval,
		CheckpointClass: wf.CheckpointClassArtifactReview,
		EvidencePaths:   []string{"output_specs/review.md"},
	}
	err := checkApprovalReadinessWithInspector("/phase", step, func(_, _ string) (bool, error) {
		return false, errors.New("containment failed")
	})
	if err == nil || !strings.Contains(err.Error(), "containment failed") {
		t.Fatalf("inspector error = %v", err)
	}
}

func TestCheckApprovalReadiness_InvalidEvidencePathsFailClosed(t *testing.T) {
	for _, path := range []string{"../outside.md", "/absolute/outside.md", "."} {
		t.Run(path, func(t *testing.T) {
			step := wf.WorkflowStep{
				Number:          2,
				Name:            "REVIEW",
				Checkpoint:      wf.CheckpointUserApproval,
				CheckpointClass: wf.CheckpointClassArtifactReview,
				EvidencePaths:   []string{path},
			}
			err := checkApprovalReadinessWithInspector("/phase", step, func(_, _ string) (bool, error) {
				t.Fatal("invalid path must fail before inspection")
				return false, nil
			})
			if err == nil || !strings.Contains(err.Error(), "invalid evidence path") {
				t.Fatalf("invalid path %q error = %v", path, err)
			}
		})
	}
}

func TestCheckApprovalReadiness_NonPresentSkips(t *testing.T) {
	step := wf.WorkflowStep{
		Number:          2,
		Name:            "ANALYZE",
		Checkpoint:      wf.CheckpointUserApproval,
		CheckpointClass: wf.CheckpointClassArtifactReview,
	}
	if err := checkApprovalReadinessWithInspector("/phase", step, func(_, _ string) (bool, error) {
		t.Fatal("non-presentation step without explicit evidence must not inspect files")
		return false, nil
	}); err != nil {
		t.Fatalf("non-present step should skip readiness file checks: %v", err)
	}
}

func TestResolveManualApprovalDecision_NoJudgeHookAllowsNonTTY(t *testing.T) {
	decision, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Actor: decisionActorUser, Summary: "ok"},
		false, false, false, nil, 1, "TEST",
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
		true, false, false, nil, 4, "PRESENT",
	)
	if err == nil {
		t.Fatal("expected non-TTY refusal when judge configured")
	}
	if !strings.Contains(err.Error(), "interactive operator TTY") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveManualApprovalDecision_DurableJudgeRejectRequiresTTYWithoutHook(t *testing.T) {
	orig := stdinIsInteractiveFn
	stdinIsInteractiveFn = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractiveFn = orig })

	tests := []struct {
		name    string
		command string
	}{
		{name: "one-off judge command", command: "custom-judge --strict"},
		{name: "workspace hook removed after rejection", command: "ob judge"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked := &wf.StepState{
				Status:        wf.StepStatusBlocked,
				DecisionActor: decisionActorAgent,
				Judge: &wf.JudgeState{
					Status:  wf.JudgeRejected,
					Command: tt.command,
				},
			}
			_, err := resolveManualApprovalDecision(
				wf.DecisionMetadata{Actor: decisionActorUser},
				false, false, false, blocked, 4, "PRESENT",
			)
			if err == nil || !strings.Contains(err.Error(), "interactive operator TTY") {
				t.Fatalf("durable reject without current hook must remain protected: %v", err)
			}
		})
	}
}

func TestResolveManualApprovalDecision_OperatorAttestationRequiresTTYWithoutHook(t *testing.T) {
	orig := stdinIsInteractiveFn
	stdinIsInteractiveFn = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractiveFn = orig })

	_, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{Actor: decisionActorUser},
		false, true, false, nil, 3, "SIGN-OFF",
	)
	if err == nil || !strings.Contains(err.Error(), "interactive operator TTY") {
		t.Fatalf("operator attestation without hook must require a human TTY: %v", err)
	}
}

func TestApprovalJudgeConfigured_ConfigLoadFailureFailsClosed(t *testing.T) {
	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: "/festivals"})
	configured, err := approvalJudgeConfiguredWithLoader(ctx, func(string) (*config.WorkspaceConfig, error) {
		return nil, errors.New("malformed yaml")
	})
	if err == nil || !strings.Contains(err.Error(), "malformed yaml") {
		t.Fatalf("config load error = %v", err)
	}
	if configured {
		t.Fatal("configuration load failure must not report a configured judge")
	}
}

func TestApprovalRecoveryLines_OnlySuggestValidRoutesWithoutJudge(t *testing.T) {
	operatorLines := approvalRecoveryLines(context.Background(), wf.WorkflowStep{
		CheckpointClass: wf.CheckpointClassOperatorAttestation,
	})
	operatorText := strings.Join(operatorLines, "\n")
	if strings.Contains(operatorText, "--auto") || !strings.Contains(operatorText, "Human attestation") {
		t.Fatalf("operator guidance = %q", operatorText)
	}

	artifactLines := approvalRecoveryLines(context.Background(), wf.WorkflowStep{
		CheckpointClass: wf.CheckpointClassArtifactReview,
	})
	artifactText := strings.Join(artifactLines, "\n")
	if strings.Contains(artifactText, "approve --auto") || !strings.Contains(artifactText, "configure") {
		t.Fatalf("unconfigured artifact guidance = %q", artifactText)
	}
}

func TestApprovalRecoveryLines_OrdinaryOperatorRejectionOmitsJudgeRetry(t *testing.T) {
	dir := setupWorkflowFestival(t)
	cfg := config.DefaultWorkspaceConfig()
	cfg.Hooks.ApprovalJudge.Command = "configured-judge"
	if err := config.SaveWorkspaceConfig(dir, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}
	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: dir})
	nav := getNavigator(t, filepath.Join(dir, "001_INGEST"))
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if err := nav.RejectWithDecision(ctx, "operator review required", wf.DecisionMetadata{Actor: decisionActorUser}); err != nil {
		t.Fatalf("RejectWithDecision: %v", err)
	}

	text := strings.Join(approvalRecoveryLinesFor(ctx, nav, nav.GetSteps()[1]), "\n")
	if strings.Contains(text, "fest workflow judge") {
		t.Fatalf("ordinary operator rejection should not suggest judge retry: %q", text)
	}
	if !strings.Contains(text, "fest workflow approve") {
		t.Fatalf("operator recovery missing approve route: %q", text)
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
		true, false, true, blocked, 4, "PRESENT",
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
		true, false, true, nil, 1, "TEST",
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
		Status:        wf.StepStatusBlocked,
		DecisionActor: decisionActorAgent,
		Judge:         &wf.JudgeState{Status: wf.JudgeRejected, Detail: "nope"},
	}
	decision, err := resolveManualApprovalDecision(
		wf.DecisionMetadata{},
		true, false, false, blocked, 4, "PRESENT",
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
