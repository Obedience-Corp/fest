package workflow

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestRunApproveAuto_RecordsJudgeLifecycle(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"evidence complete"}`), nil
	})

	// Wait:true exercises the in-process path with a mocked judge runner.
	// Default async launch must never run under go test (see launchJudgeProcessDefault).
	_ = captureStdout(t, func() {
		if err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
			JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
		}); err != nil {
			t.Fatalf("runApproveAuto: %v", err)
		}
	})

	judge := nav.GetWorkflowState().GetStepState(2).Judge
	if judge == nil {
		t.Fatal("step 2 judge state not recorded")
	}
	if judge.Status != wf.JudgeApproved {
		t.Fatalf("judge status = %q, want %q", judge.Status, wf.JudgeApproved)
	}
	if judge.Command != "fake judge" {
		t.Fatalf("judge command = %q, want fake judge", judge.Command)
	}
	if judge.StartedAt == nil || judge.FinishedAt == nil {
		t.Fatalf("judge timestamps missing: %+v", judge)
	}

	// The lifecycle must survive a reload from the event stream so watchers
	// in other processes see it.
	reloaded := getNavigator(t, phaseDir)
	replayed := reloaded.GetWorkflowState().GetStepState(2).Judge
	if replayed == nil || replayed.Status != wf.JudgeApproved || replayed.Detail != "evidence complete" {
		t.Fatalf("replayed judge state = %+v, want approved with reason", replayed)
	}
}

func TestRunApproveAuto_JudgeFailureLeavesDurableTrace(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	steps := nav.GetSteps()

	withApprovalJudgeRunner(t, func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return nil, festerrors.Validation("judge exploded")
	})

	err := runApproveAuto(ctx, nav, 2, steps[1], approvalJudgeOptions{
		JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
	})
	if err == nil {
		t.Fatal("runApproveAuto should fail when the judge fails")
	}

	// Checkpoint unchanged: still awaiting a decision.
	state := nav.GetWorkflowState()
	if state.CurrentStep != 2 {
		t.Fatalf("current step = %d, want 2 (checkpoint unchanged)", state.CurrentStep)
	}

	// The failed run is durably recorded, including across a reload.
	reloaded := getNavigator(t, phaseDir)
	judge := reloaded.GetWorkflowState().GetStepState(2).Judge
	if judge == nil {
		t.Fatal("failed judge run left no durable trace")
	}
	if judge.Status != wf.JudgeFailed {
		t.Fatalf("judge status = %q, want %q", judge.Status, wf.JudgeFailed)
	}
	if !strings.Contains(judge.Detail, "judge exploded") {
		t.Fatalf("judge detail = %q, want failure text", judge.Detail)
	}
	if judge.StartedAt == nil || judge.FinishedAt == nil {
		t.Fatalf("judge timestamps missing: %+v", judge)
	}
}
