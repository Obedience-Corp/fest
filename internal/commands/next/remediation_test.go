package next

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestFindFailedRemediationGate_NoneWhenAbsent(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	gate, err := findFailedRemediationGate(context.Background(), festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate != nil {
		t.Fatalf("expected nil, got %+v", gate)
	}
}

func TestFindFailedRemediationGate_DiscoversAfterReject(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepFailRemediationEvents("gate:001_REVIEW", 1, "PR not ready", "005_FIX_PR_302"))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate == nil {
		t.Fatal("expected gate, got nil")
	}
	if gate.PhaseName != "001_REVIEW" {
		t.Errorf("PhaseName = %q, want 001_REVIEW", gate.PhaseName)
	}
	if gate.RemediationPhase != "005_FIX_PR_302" {
		t.Errorf("RemediationPhase = %q", gate.RemediationPhase)
	}
	if gate.Step != 1 {
		t.Errorf("Step = %d, want 1", gate.Step)
	}
	if gate.Reason != "PR not ready" {
		t.Errorf("Reason = %q", gate.Reason)
	}
	if gate.StepName != "READINESS" {
		t.Errorf("StepName = %q, want READINESS (parsed from GATES.md)", gate.StepName)
	}
}

func TestFindFailedRemediationGate_ClearedAfterRecheckDone(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepFailRemediationEvents("gate:001_REVIEW", 1, "fail", "005_FIX_PR_302"))
	store.QueueWorkflowEvents(wf.EmitStepRecheckEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("gate:001_REVIEW", 1))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate: %v", err)
	}
	if gate != nil {
		t.Errorf("expected nil after recheck+done, got %+v", gate)
	}
}

func TestIsRemediationPhaseComplete(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()

	complete, err := isRemediationPhaseComplete(ctx, festDir, "005_FIX_PR_302")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if complete {
		t.Error("expected incomplete remediation phase")
	}

	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("005_FIX_PR_302", 1))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	complete, err = isRemediationPhaseComplete(ctx, festDir, "005_FIX_PR_302")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !complete {
		t.Error("expected complete remediation phase after workflow done")
	}
}

func scaffoldRemediationFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: rem-next-test\nid: RNT-001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Rem Next Test\n"), 0o644); err != nil {
		t.Fatalf("write goal: %v", err)
	}

	gatePhase := filepath.Join(dir, "001_REVIEW")
	if err := os.MkdirAll(gatePhase, 0o755); err != nil {
		t.Fatalf("mkdir gate phase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gatePhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 001_REVIEW\nfest_phase_type: review\n---\n\n# Review\n"), 0o644); err != nil {
		t.Fatalf("write phase goal: %v", err)
	}
	gatesMD := "---\nfest_type: phase_gate\nfest_id: 001_REVIEW-GATE\nfest_parent: 001_REVIEW\n---\n\n# Gate\n\n## Step 1: READINESS — Verify ready\n\n**Question:** ready?\n\n**Actions:**\n1. check\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	if err := os.WriteFile(filepath.Join(gatePhase, "GATES.md"), []byte(gatesMD), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}

	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	if err := os.MkdirAll(remPhase, 0o755); err != nil {
		t.Fatalf("mkdir rem: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: planning\n---\n\n# Fix\n"), 0o644); err != nil {
		t.Fatalf("write rem phase goal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remPhase, "WORKFLOW.md"), []byte("---\nfest_type: workflow\nfest_id: REM-WF\nfest_parent: 005_FIX_PR_302\n---\n\n# WF\n\n## Step 1: ADDRESS — Address\n\n**Goal:** Fix.\n\n**Actions:**\n1. fix\n\n**Output:** done\n\n**Checkpoint:** None\n"), 0o644); err != nil {
		t.Fatalf("write rem WORKFLOW.md: %v", err)
	}
	return dir
}

// scaffoldRemediationFestivalWithTask adds a numbered sequence with one pending
// task to the remediation phase so the mixed workflow-plus-task routing case
// can be exercised.
func scaffoldRemediationFestivalWithTask(t *testing.T) string {
	t.Helper()
	dir := scaffoldRemediationFestival(t)
	remPhase := filepath.Join(dir, "005_FIX_PR_302")
	// Re-type the remediation phase as implementation so the selector surfaces
	// task files rather than planning objectives.
	if err := os.WriteFile(filepath.Join(remPhase, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 005_FIX_PR_302\nfest_phase_type: implementation\n---\n\n# Fix\n"), 0o644); err != nil {
		t.Fatalf("rewrite rem phase goal: %v", err)
	}
	seq := filepath.Join(remPhase, "001_REMEDIATE")
	if err := os.MkdirAll(seq, 0o755); err != nil {
		t.Fatalf("mkdir rem seq: %v", err)
	}
	task := "---\nfest_type: task\nfest_id: 01_fix.md\nfest_name: fix\nfest_status: pending\n---\n# Fix Task\n"
	if err := os.WriteFile(filepath.Join(seq, "01_fix.md"), []byte(task), 0o644); err != nil {
		t.Fatalf("write rem task: %v", err)
	}
	return dir
}

func seedFailedGate(t *testing.T, festDir string) {
	t.Helper()
	ctx := context.Background()
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("gate:001_REVIEW", 1))
	store.QueueWorkflowEvents(wf.EmitStepFailRemediationEvents("gate:001_REVIEW", 1, "PR not ready", "005_FIX_PR_302"))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
}

func completeRemediationWorkflow(t *testing.T, festDir string) {
	t.Helper()
	ctx := context.Background()
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepStartEvents("005_FIX_PR_302", 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneEvents("005_FIX_PR_302", 1))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
}

func loadStoreForTest(t *testing.T, ctx context.Context, festDir string) *progress.Store {
	t.Helper()
	store, err := loadProgressStore(ctx, festDir)
	if err != nil {
		t.Fatalf("loadProgressStore: %v", err)
	}
	return store
}

func TestShouldRouteToRemediationWorkflow_IncompleteWorkflowOnly(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	remPath := filepath.Join(festDir, "005_FIX_PR_302")
	if !shouldRouteToRemediationWorkflow(context.Background(), festDir, remPath, "005_FIX_PR_302") {
		t.Fatal("incomplete workflow-only remediation phase must route to the workflow")
	}
}

func TestShouldRouteToRemediationWorkflow_CompletedWorkflowRoutesToTask(t *testing.T) {
	festDir := scaffoldRemediationFestivalWithTask(t)
	completeRemediationWorkflow(t, festDir)
	remPath := filepath.Join(festDir, "005_FIX_PR_302")
	if shouldRouteToRemediationWorkflow(context.Background(), festDir, remPath, "005_FIX_PR_302") {
		t.Fatal("completed remediation workflow with an incomplete task must route to the task, not re-show WORKFLOW COMPLETE")
	}
}

func TestShouldRouteToRemediationWorkflow_NoWorkflow(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	gatePhase := filepath.Join(festDir, "001_REVIEW")
	if shouldRouteToRemediationWorkflow(context.Background(), festDir, gatePhase, "001_REVIEW") {
		t.Fatal("a phase without WORKFLOW.md must not route to workflow mode")
	}
}

func TestRouteFailedRemediationGate_CompletedWorkflowSurfacesTask(t *testing.T) {
	festDir := scaffoldRemediationFestivalWithTask(t)
	ctx := context.Background()
	seedFailedGate(t, festDir)
	completeRemediationWorkflow(t, festDir)

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil || gate == nil {
		t.Fatalf("findFailedRemediationGate: err=%v gate=%+v", err, gate)
	}

	out := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{}); rErr != nil {
			t.Fatalf("route: %v", rErr)
		}
	})
	if strings.Contains(out, "WORKFLOW COMPLETE") {
		t.Fatalf("remediation route dead-ended on WORKFLOW COMPLETE instead of surfacing the task:\n%s", out)
	}
	if !strings.Contains(out, "001_REMEDIATE") {
		t.Fatalf("expected the remaining remediation task in output:\n%s", out)
	}
}

func TestRouteFailedRemediationGate_MachineOutputSuppressesBanner(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()
	seedFailedGate(t, festDir)

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil || gate == nil {
		t.Fatalf("findFailedRemediationGate: err=%v gate=%+v", err, gate)
	}

	wantDoc := filepath.Join("005_FIX_PR_302", "WORKFLOW.md")

	pathOut := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{Path: true}); rErr != nil {
			t.Fatalf("route --path: %v", rErr)
		}
	})
	if strings.TrimSpace(pathOut) != wantDoc {
		t.Fatalf("--path = %q, want %q", strings.TrimSpace(pathOut), wantDoc)
	}
	if strings.Contains(pathOut, "FAILED GATE") {
		t.Fatalf("--path leaked the human banner: %q", pathOut)
	}

	cdOut := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{CD: true}); rErr != nil {
			t.Fatalf("route --cd: %v", rErr)
		}
	})
	if strings.TrimSpace(cdOut) != filepath.Join(festDir, "005_FIX_PR_302") {
		t.Fatalf("--cd = %q", strings.TrimSpace(cdOut))
	}

	shortOut := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{Short: true}); rErr != nil {
			t.Fatalf("route --short: %v", rErr)
		}
	})
	if !strings.Contains(shortOut, "remediation active") || strings.Contains(shortOut, "FAILED GATE") {
		t.Fatalf("--short = %q", shortOut)
	}

	jsonOut := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{JSON: true}); rErr != nil {
			t.Fatalf("route --json: %v", rErr)
		}
	})
	if strings.Contains(jsonOut, "FAILED GATE") {
		t.Fatalf("--json leaked the human banner:\n%s", jsonOut)
	}
	var v remediationView
	if err := json.Unmarshal([]byte(jsonOut), &v); err != nil {
		t.Fatalf("unmarshal --json: %v\n%s", err, jsonOut)
	}
	if v.Route != "remediation-workflow" {
		t.Errorf("Route = %q, want remediation-workflow", v.Route)
	}
	if v.TargetDoc != wantDoc {
		t.Errorf("TargetDoc = %q, want %q", v.TargetDoc, wantDoc)
	}
	if v.Complete {
		t.Error("Complete = true, want false while remediation is active")
	}
}

func TestRouteFailedRemediationGate_RecheckMachineOutputDoesNotMutateState(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()
	seedFailedGate(t, festDir)
	completeRemediationWorkflow(t, festDir)

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil || gate == nil {
		t.Fatalf("findFailedRemediationGate: err=%v gate=%+v", err, gate)
	}

	jsonOut := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{JSON: true}); rErr != nil {
			t.Fatalf("route --json: %v", rErr)
		}
	})
	var v remediationView
	if err := json.Unmarshal([]byte(jsonOut), &v); err != nil {
		t.Fatalf("unmarshal --json: %v\n%s", err, jsonOut)
	}
	if v.Route != "recheck-gate" {
		t.Errorf("Route = %q, want recheck-gate", v.Route)
	}
	if !v.Complete {
		t.Error("Complete = false, want true once the remediation phase is done")
	}
	if v.TargetDoc != filepath.Join("001_REVIEW", "GATES.md") {
		t.Errorf("TargetDoc = %q", v.TargetDoc)
	}
	if strings.Contains(jsonOut, "RECHECK GATE") {
		t.Fatalf("--json leaked the human banner:\n%s", jsonOut)
	}

	after, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate after recheck: %v", err)
	}
	if after == nil {
		t.Fatal("machine-output recheck view must not clear failed-remediation state")
	}
}

func TestRouteFailedRemediationGate_RecheckHumanOutputRecordsSideEffect(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()
	seedFailedGate(t, festDir)
	completeRemediationWorkflow(t, festDir)

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil || gate == nil {
		t.Fatalf("findFailedRemediationGate: err=%v gate=%+v", err, gate)
	}

	out := captureStdout(t, func() {
		if _, rErr := routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{}); rErr != nil {
			t.Fatalf("route: %v", rErr)
		}
	})
	if !strings.Contains(out, "RECHECK GATE") {
		t.Fatalf("human output should include recheck banner:\n%s", out)
	}

	after, err := findFailedRemediationGate(ctx, festDir)
	if err != nil {
		t.Fatalf("findFailedRemediationGate after recheck: %v", err)
	}
	if after != nil {
		t.Fatalf("human recheck should have cleared the failed gate, still got %+v", after)
	}
}

func TestRouteFailedRemediationGate_StaleRemediationPhasePointerIsActionable(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	ctx := context.Background()
	seedFailedGate(t, festDir)

	gate, err := findFailedRemediationGate(ctx, festDir)
	if err != nil || gate == nil {
		t.Fatalf("findFailedRemediationGate: err=%v gate=%+v", err, gate)
	}
	if err := os.RemoveAll(filepath.Join(festDir, "005_FIX_PR_302")); err != nil {
		t.Fatalf("remove remediation phase: %v", err)
	}

	_, err = routeFailedRemediationGate(ctx, festDir, loadStoreForTest(t, ctx, festDir), gate, RenderOptions{})
	if err == nil {
		t.Fatal("expected stale remediation phase error, got nil")
	}
	if !strings.Contains(err.Error(), "no longer exists") || !strings.Contains(err.Error(), "fest workflow reject --phase 001_REVIEW") {
		t.Fatalf("error is not actionable: %v", err)
	}
}

func TestFindFailedRemediationGate_FailsClosedOnUnreadableStore(t *testing.T) {
	festDir := scaffoldRemediationFestival(t)
	progressDir := filepath.Join(festDir, progress.ProgressDir)
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatalf("mkdir progress: %v", err)
	}
	// A single line longer than bufio.Scanner's max token size with no newline
	// makes the event-log read fail, standing in for a corrupt/unreadable store.
	oversized := strings.Repeat("x", 128*1024)
	if err := os.WriteFile(filepath.Join(progressDir, progress.ProgressEventsFile), []byte(oversized), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	if _, err := findFailedRemediationGate(context.Background(), festDir); err == nil {
		t.Fatal("expected an error from an unreadable progress store so fest next can fail closed")
	}
}
