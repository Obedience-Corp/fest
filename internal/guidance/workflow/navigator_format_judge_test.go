package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/scope"
)

func TestFormatCheckpoint_NoJudgeConfigured(t *testing.T) {
	nav := &Navigator{}
	step := WorkflowStep{Number: 2, Name: "REVIEW", Goal: "Is the work acceptable?"}

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

func TestFormatCheckpoint_JudgeConfigured(t *testing.T) {
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
	step := WorkflowStep{Number: 2, Name: "REVIEW"}

	out, err := nav.formatCheckpoint(ctx, step)
	if err != nil {
		t.Fatalf("formatCheckpoint() error = %v", err)
	}

	for _, want := range []string{
		"This checkpoint is delegated",
		"Run: fest workflow approve --auto",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("checkpoint output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Present your work to the user") {
		t.Errorf("delegated checkpoint should not tell the agent to wait for the user:\n%s", out)
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
