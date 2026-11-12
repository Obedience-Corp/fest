package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

const checkpointWorkflowDoc = `---
workflow_version: 1
workflow_id: wf-checkpoint
---

## Step 1: WORK

**Goal:** Do the work.

**Output:** done

## Step 2: REVIEW

**Goal:** Get sign-off.

**Output:** decision

**Checkpoint:** USER APPROVAL REQUIRED

## Step 3: WRAP

**Goal:** Finish.

**Output:** done
`

const lastStepCheckpointDoc = `---
workflow_version: 1
workflow_id: wf-last
---

## Step 1: REVIEW

**Goal:** Get sign-off.

**Output:** decision

**Checkpoint:** USER APPROVAL REQUIRED
`

// newStandaloneRun writes doc into a temp directory and returns the anonymous
// result for it; the first advance bootstraps the tracked run.
func newStandaloneRun(t *testing.T, doc string) *standalone.Result {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return &standalone.Result{Mode: standalone.ModeAnonymous, StartDir: dir, WorkflowDoc: path}
}

func tracked(res *standalone.Result) *standalone.Result {
	return &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    res.StartDir,
		WorkflowDoc: res.WorkflowDoc,
		RuntimeDir:  filepath.Join(res.StartDir, ".workflow"),
	}
}

func runState(t *testing.T, res *standalone.Result) *localstore.RunState {
	t.Helper()
	store := localstore.Open(filepath.Join(res.StartDir, ".workflow"), res.WorkflowDoc)
	state, err := store.LoadLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("no run")
	}
	return state
}

// operator makes the next approve run as an interactive operator who types
// answer at the confirmation prompt.
func operator(t *testing.T, interactive bool, answer string) {
	t.Helper()
	origTTY, origRead := stdinIsInteractiveFn, readOperatorConfirm
	stdinIsInteractiveFn = func() bool { return interactive }
	readOperatorConfirm = func() (string, error) { return answer, nil }
	t.Cleanup(func() { stdinIsInteractiveFn, readOperatorConfirm = origTTY, origRead })
}

// toCheckpoint advances a fresh checkpointWorkflowDoc run to step 2's
// checkpoint and returns the tracked result.
func toCheckpoint(t *testing.T) *standalone.Result {
	t.Helper()
	ctx := context.Background()
	res := newStandaloneRun(t, checkpointWorkflowDoc)
	if err := runAdvanceBootstrap(ctx, res); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tr := tracked(res)
	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatalf("advance into checkpoint: %v", err)
	}
	state := runState(t, tr)
	if !state.AwaitingApproval() || state.CurrentStep != 2 || state.CompletedSteps != 1 {
		t.Fatalf("not at the checkpoint: %+v", state)
	}
	return tr
}

func TestStandaloneCheckpoint_AdvanceStopsAndStays(t *testing.T) {
	ctx := context.Background()
	tr := toCheckpoint(t)

	// Advancing again must not move past the checkpoint or record anything.
	for i := 0; i < 2; i++ {
		if err := runAdvanceTracked(ctx, tr); err != nil {
			t.Fatalf("advance at checkpoint: %v", err)
		}
	}
	state := runState(t, tr)
	if !state.AwaitingApproval() || state.CurrentStep != 2 || state.CompletedSteps != 1 {
		t.Fatalf("advance moved a waiting checkpoint: %+v", state)
	}
}

func TestStandaloneCheckpoint_ApproveRequiresOperator(t *testing.T) {
	ctx := context.Background()
	tr := toCheckpoint(t)

	operator(t, false, operatorApproveToken)
	err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("non-interactive approve must be refused, got %v", err)
	}

	operator(t, true, "yes")
	err = runStandaloneApprove(ctx, tr, standaloneApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("a wrong confirmation must be refused, got %v", err)
	}

	if state := runState(t, tr); !state.AwaitingApproval() || state.CompletedSteps != 1 {
		t.Fatalf("a refused approval changed the run: %+v", state)
	}
}

func TestStandaloneCheckpoint_ApproveAdvances(t *testing.T) {
	ctx := context.Background()
	tr := toCheckpoint(t)

	operator(t, true, operatorApproveToken)
	if err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{Summary: "looks right", ExpectStep: 2}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state := runState(t, tr)
	if state.Blocked || state.CompletedSteps != 2 || state.Status != "active" {
		t.Fatalf("after approve: %+v", state)
	}

	// Nothing is waiting any more.
	err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "no checkpoint is awaiting approval") {
		t.Fatalf("second approve must find nothing waiting, got %v", err)
	}

	// The run continues normally.
	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatalf("advance step 3: %v", err)
	}
	if state := runState(t, tr); state.CompletedSteps != 3 {
		t.Fatalf("after step 3: %+v", state)
	}
}

func TestStandaloneCheckpoint_ExpectStepPinsTheDecision(t *testing.T) {
	ctx := context.Background()
	tr := toCheckpoint(t)
	operator(t, true, operatorApproveToken)

	err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{ExpectStep: 3})
	if err == nil || !strings.Contains(err.Error(), "not at the expected step") {
		t.Fatalf("approve pinned to another step must be refused, got %v", err)
	}
	err = runStandaloneReject(ctx, tr, "no", 1)
	if err == nil || !strings.Contains(err.Error(), "not at the expected step") {
		t.Fatalf("reject pinned to another step must be refused, got %v", err)
	}
	if state := runState(t, tr); !state.AwaitingApproval() {
		t.Fatalf("a refused pinned decision changed the run: %+v", state)
	}
}

func TestStandaloneCheckpoint_RejectThenResubmit(t *testing.T) {
	ctx := context.Background()
	tr := toCheckpoint(t)

	if err := runStandaloneReject(ctx, tr, "missing evidence", 0); err != nil {
		t.Fatalf("reject: %v", err)
	}
	state := runState(t, tr)
	if !state.Blocked || state.AwaitingApproval() || state.Feedback != "missing evidence" {
		t.Fatalf("after reject: %+v", state)
	}

	// A rejected step is not waiting on a decision until it is re-submitted.
	operator(t, true, operatorApproveToken)
	err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "has not been re-submitted") {
		t.Fatalf("approve after reject must be refused, got %v", err)
	}
	if err := runStandaloneReject(ctx, tr, "again", 0); err == nil {
		t.Fatal("rejecting an already rejected step must be refused")
	}

	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatalf("re-submit: %v", err)
	}
	if state := runState(t, tr); !state.AwaitingApproval() || state.Feedback != "" {
		t.Fatalf("after re-submit: %+v", state)
	}
	if err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{}); err != nil {
		t.Fatalf("approve after re-submit: %v", err)
	}
}

func TestStandaloneCheckpoint_FirstStepAndLastStep(t *testing.T) {
	ctx := context.Background()
	res := newStandaloneRun(t, lastStepCheckpointDoc)

	// Bootstrap into a step-1 checkpoint creates the run and waits there
	// rather than auto-approving it.
	if err := runAdvanceBootstrap(ctx, res); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tr := tracked(res)
	state := runState(t, tr)
	if !state.AwaitingApproval() || state.CurrentStep != 1 || state.CompletedSteps != 0 {
		t.Fatalf("bootstrap did not stop at the step 1 checkpoint: %+v", state)
	}

	// Approving the last step completes the run.
	operator(t, true, operatorApproveToken)
	if err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if state := runState(t, tr); state.Status != "completed" || state.CompletedSteps != 1 {
		t.Fatalf("after approving the last step: %+v", state)
	}
}

func TestStandaloneCheckpoint_NothingWaiting(t *testing.T) {
	ctx := context.Background()
	res := newStandaloneRun(t, checkpointWorkflowDoc)
	operator(t, true, operatorApproveToken)

	// No run yet.
	if err := runStandaloneApprove(ctx, res, standaloneApproveOptions{}); err == nil {
		t.Fatal("approve with no run must fail")
	}
	if err := runAdvanceBootstrap(ctx, res); err != nil {
		t.Fatal(err)
	}
	// A run that is between steps has nothing waiting.
	tr := tracked(res)
	if err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{}); err == nil {
		t.Fatal("approve with nothing waiting must fail")
	}
	if err := runStandaloneReject(ctx, tr, "x", 0); err == nil {
		t.Fatal("reject with nothing waiting must fail")
	}
}

func TestCheckExpectedStep(t *testing.T) {
	if err := checkExpectedStep(0, 4); err != nil {
		t.Fatalf("unpinned must pass: %v", err)
	}
	if err := checkExpectedStep(4, 4); err != nil {
		t.Fatalf("matching pin must pass: %v", err)
	}
	if err := checkExpectedStep(3, 4); err == nil {
		t.Fatal("mismatched pin must fail")
	}
}
