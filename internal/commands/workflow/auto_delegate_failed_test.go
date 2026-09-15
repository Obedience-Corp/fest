package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
)

// withConfiguredJudge points the judge lookup at a festivals root whose
// .festival/config.yaml delegates checkpoints to a judge command. Nothing is
// launched: the tests below stop before any process spawn.
func withConfiguredJudge(t *testing.T) context.Context {
	t.Helper()
	festivalsRoot := t.TempDir()
	dotFestival := filepath.Join(festivalsRoot, config.DotFestivalDir)
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: \"1.0\"\nhooks:\n  definitions:\n    approval_judge:\n      command: fake judge\n"
	if err := os.WriteFile(filepath.Join(dotFestival, config.WorkspaceConfigFileName), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return scope.WithWorkspace(context.Background(), &scope.WorkspaceInfo{FestivalsPath: festivalsRoot})
}

func TestAutoDelegateBlockingCheckpoints_FailedJudgeIsNotRelaunched(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := withConfiguredJudge(t)

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	// First run fails closed, the way a dead provider or bad command does.
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return nil, festerrors.Validation("daemon refused: model does not fit at long")
	}))
	if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
		JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
	}); err == nil {
		t.Fatal("runApproveAuto should fail when the judge fails")
	}
	before := *nav.GetWorkflowState().GetStepState(2).Judge
	if before.Status != wf.JudgeFailed {
		t.Fatalf("precondition: judge status = %q, want failed", before.Status)
	}

	// A runner that must never be reached: fest next must not relaunch.
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		t.Fatal("fest next relaunched a failed judge")
		return nil, nil
	}))

	out := captureStdout(t, func() {
		if err := AutoDelegateBlockingCheckpoints(ctx, nav); err != nil {
			t.Fatalf("AutoDelegateBlockingCheckpoints: %v", err)
		}
	})

	for _, want := range []string{
		"Approval judge failed (fails closed)",
		"daemon refused: model does not fit at long",
		"does not relaunch a failed judge",
		"fest workflow judge",
		"fest workflow approve",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fest next output missing %q:\n%s", want, out)
		}
	}
	for _, reject := range []string{"Judge launched", "already running", "Checkpoint delegated to approval judge"} {
		if strings.Contains(out, reject) {
			t.Errorf("fest next output must not claim a relaunch (%q):\n%s", reject, out)
		}
	}

	after := nav.GetWorkflowState().GetStepState(2).Judge
	if after == nil || after.Status != wf.JudgeFailed || after.RunID != before.RunID {
		t.Fatalf("judge record changed: before %+v, after %+v", before, after)
	}
	if nav.GetWorkflowState().CurrentStep != 2 {
		t.Fatalf("current step = %d, want 2 (checkpoint unchanged)", nav.GetWorkflowState().CurrentStep)
	}
}
