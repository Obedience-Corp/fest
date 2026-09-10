package workflow

import (
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

func TestApprovalRecoveryLinesTeachJudgeSetupWhenUnconfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEST_CONFIG_DIR", t.TempDir())
	step := wf.WorkflowStep{Number: 1, Name: "PHASE GOAL", CheckpointClass: wf.CheckpointClassArtifactReview}
	out := strings.Join(approvalRecoveryLinesFor(t.Context(), nil, step), "\n")

	for _, want := range []string{
		"fest workflow approve",
		hooks.JudgeInstallCommand,
		"festivals/.festival/config.yaml",
		"approval_judge:",
		"command: " + hooks.JudgeExampleCommand,
		"fest workflow judge",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unconfigured approve footer lacks %q:\n%s", want, out)
		}
	}
}
