package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/scope"
)

func TestFormatCheckpoint_NoJudge_OperatorAttestation(t *testing.T) {
	// Ambiguous legacy steps fail closed as operator_attestation: agents must
	// hand off to a human and must not be told to configure/run --auto.
	nav := &Navigator{}
	step := WorkflowStep{Number: 2, Name: "REVIEW", Goal: "Is the work acceptable?"}

	out, err := nav.formatCheckpoint(context.Background(), step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}

	for _, want := range []string{
		"Wait for the user's response",
		"cannot be delegated to a judge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "fest workflow approve --auto") {
		t.Errorf("operator_attestation checkpoint must not advertise --auto:\n%s", out)
	}
	if strings.Contains(out, "Delegate this decision") {
		t.Errorf("operator_attestation checkpoint must not suggest configuring a judge:\n%s", out)
	}
}

func TestFormatCheckpoint_NoJudge_ArtifactReviewOffersDelegate(t *testing.T) {
	// Artifact-review checkpoints without a configured judge still stop for a
	// human, but operators may opt in to auto-judging via hooks.
	nav := &Navigator{}
	step := WorkflowStep{
		Number:          2,
		Name:            "PRESENT",
		Goal:            "Present the plan summary",
		CheckpointClass: CheckpointClassArtifactReview,
	}

	out, err := nav.formatCheckpoint(context.Background(), step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}

	for _, want := range []string{
		"Wait for the user's response",
		"Delegate this decision",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "1. Run: fest workflow approve --auto") {
		t.Errorf("checkpoint output must not instruct --auto without a configured judge:\n%s", out)
	}
}

func TestFormatCheckpoint_JudgeConfigured_ArtifactReviewDelegates(t *testing.T) {
	festivalsRoot := t.TempDir()
	dotFestival := filepath.Join(festivalsRoot, config.DotFestivalDir)
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgYAML := "version: \"1.0\"\nhooks:\n  approval_judge:\n    command: ob judge\n"
	if err := os.WriteFile(filepath.Join(dotFestival, config.WorkspaceConfigFileName), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: festivalsRoot})

	nav := &Navigator{}
	step := WorkflowStep{
		Number:          2,
		Name:            "PRESENT",
		CheckpointClass: CheckpointClassArtifactReview,
	}

	out, err := nav.formatCheckpoint(ctx, step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}

	for _, want := range []string{
		"This checkpoint is delegated",
		"fest next` auto-invokes the judge",
		"fest workflow judge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Present your work to the user") {
		t.Errorf("delegated artifact_review checkpoint should not tell the agent to wait for the user:\n%s", out)
	}
}

func TestFormatCheckpoint_JudgeConfigured_OperatorAttestationStaysManual(t *testing.T) {
	// A configured judge must not rewrite operator_attestation gates into the
	// auto path — those ask whether a human already validated.
	festivalsRoot := t.TempDir()
	dotFestival := filepath.Join(festivalsRoot, config.DotFestivalDir)
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgYAML := "version: \"1.0\"\nhooks:\n  approval_judge:\n    command: ob judge\n"
	if err := os.WriteFile(filepath.Join(dotFestival, config.WorkspaceConfigFileName), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: festivalsRoot})

	nav := &Navigator{}
	step := WorkflowStep{
		Number:          2,
		Name:            "VALIDATE",
		Goal:            "Did the user explicitly approve the plan?",
		CheckpointClass: CheckpointClassOperatorAttestation,
	}

	out, err := nav.formatCheckpoint(ctx, step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}

	for _, want := range []string{
		"Wait for the user's response",
		"cannot be delegated to a judge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "This checkpoint is delegated") {
		t.Errorf("operator_attestation must not use the delegated judge path:\n%s", out)
	}
	if strings.Contains(out, "fest workflow approve --auto") {
		t.Errorf("operator_attestation must not advertise --auto even when a judge is configured:\n%s", out)
	}
}

func TestApprovalJudgeConfigured_NilAndMissing(t *testing.T) {
	nav := &Navigator{}
	//nolint:staticcheck // formatCheckpoint tolerates a nil context by design; verify the guard.
	if nav.approvalJudgeConfigured(nil) {
		t.Error("nil context should report no judge")
	}
	if nav.approvalJudgeConfigured(context.Background()) {
		t.Error("context without workspace should report no judge")
	}
	ctx := scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: t.TempDir()})
	if nav.approvalJudgeConfigured(ctx) {
		t.Error("workspace without a configured judge should report no judge")
	}
}

func TestApprovalJudgeConfigured_FromFestivalPathWithoutWorkspaceCtx(t *testing.T) {
	// Mirrors fest next: scope.Global so WorkspaceFrom is empty, but the
	// navigator knows the festival path under festivals/.
	root := t.TempDir()
	festivalsRoot := filepath.Join(root, "festivals")
	festivalPath := filepath.Join(festivalsRoot, "active", "example-fest")
	if err := os.MkdirAll(festivalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	dotFestival := filepath.Join(festivalsRoot, config.DotFestivalDir)
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "version: \"1.0\"\nhooks:\n  approval_judge:\n    command: ob judge\n"
	if err := os.WriteFile(filepath.Join(dotFestival, config.WorkspaceConfigFileName), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	gctx := &guidance.GuidanceContext{FestivalPath: festivalPath}
	nav, err := NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}

	if !nav.approvalJudgeConfigured(context.Background()) {
		t.Fatal("judge should be detected from festival path without WorkspaceFrom")
	}
	if got := nav.ApprovalJudgeCommand(context.Background()); got != "ob judge" {
		t.Fatalf("ApprovalJudgeCommand = %q, want %q", got, "ob judge")
	}
}
