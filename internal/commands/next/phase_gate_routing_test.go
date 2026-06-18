package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func scaffoldWorkflowGateFestival(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: wf-gate-test\nid: WGT-001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# WF Gate Test\n"), 0o644); err != nil {
		t.Fatalf("write goal: %v", err)
	}

	workflowMD := "---\nfest_type: workflow\nfest_id: %s-WF\nfest_parent: %s\n---\n\n# WF\n\n## Step 1: ONE — First\n\n**Goal:** one.\n\n**Actions:**\n1. do\n\n**Output:** done\n\n**Checkpoint:** None\n\n## Step 2: TWO — Second\n\n**Goal:** two.\n\n**Actions:**\n1. do\n\n**Output:** done\n\n**Checkpoint:** None\n"
	gatesMD := "---\nfest_type: phase_gate\nfest_id: %s-GATE\nfest_parent: %s\n---\n\n# Gate\n\n## Step 1: VERIFY — Verify\n\n**Question:** good?\n\n**Actions:**\n1. check\n\n**Checkpoint:** APPROVAL REQUIRED\n"

	ingest := filepath.Join(dir, "001_INGEST")
	if err := os.MkdirAll(ingest, 0o755); err != nil {
		t.Fatalf("mkdir ingest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ingest, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 001_INGEST\nfest_phase_type: ingest\n---\n\n# Ingest\n"), 0o644); err != nil {
		t.Fatalf("write ingest goal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ingest, "WORKFLOW.md"), fmt.Appendf(nil, workflowMD, "001_INGEST", "001_INGEST"), 0o644); err != nil {
		t.Fatalf("write ingest WORKFLOW.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ingest, "GATES.md"), fmt.Appendf(nil, gatesMD, "001_INGEST", "001_INGEST"), 0o644); err != nil {
		t.Fatalf("write ingest GATES.md: %v", err)
	}

	plan := filepath.Join(dir, "002_PLAN")
	if err := os.MkdirAll(plan, 0o755); err != nil {
		t.Fatalf("mkdir plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(plan, "PHASE_GOAL.md"), []byte("---\nfest_type: phase\nfest_id: 002_PLAN\nfest_phase_type: planning\n---\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write plan goal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(plan, "WORKFLOW.md"), fmt.Appendf(nil, workflowMD, "002_PLAN", "002_PLAN"), 0o644); err != nil {
		t.Fatalf("write plan WORKFLOW.md: %v", err)
	}

	return dir
}

func completeWorkflow(t *testing.T, festDir, key string, steps int) {
	t.Helper()
	ctx := context.Background()
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents(key, steps))
	for i := 1; i <= steps; i++ {
		store.QueueWorkflowEvents(wf.EmitStepDoneEvents(key, i))
	}
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
}

func TestResolveCompletePhaseRoute_SurfacesEarlierGateBeforeLaterWorkflow(t *testing.T) {
	festDir := scaffoldWorkflowGateFestival(t)
	ctx := context.Background()

	completeWorkflow(t, festDir, "001_INGEST", 2)

	if got, _ := findFirstIncompleteWorkflowPhase(ctx, festDir); filepath.Base(got) != "002_PLAN" {
		t.Fatalf("findFirstIncompleteWorkflowPhase = %q, want 002_PLAN", got)
	}
	if got, _ := findFirstIncompletePhaseGate(ctx, festDir); filepath.Base(got) != "001_INGEST" {
		t.Fatalf("findFirstIncompletePhaseGate = %q, want 001_INGEST", got)
	}

	route, phase, err := resolveCompletePhaseRoute(ctx, festDir)
	if err != nil {
		t.Fatalf("resolveCompletePhaseRoute: %v", err)
	}
	if route != routeGate {
		t.Fatalf("route = %v, want routeGate", route)
	}
	if filepath.Base(phase) != "001_INGEST" {
		t.Fatalf("phase = %q, want 001_INGEST", phase)
	}
}

func TestResolveCompletePhaseRoute_AdvancesToNextWorkflowAfterGateDone(t *testing.T) {
	festDir := scaffoldWorkflowGateFestival(t)
	ctx := context.Background()

	completeWorkflow(t, festDir, "001_INGEST", 2)
	completeWorkflow(t, festDir, "gate:001_INGEST", 1)

	route, phase, err := resolveCompletePhaseRoute(ctx, festDir)
	if err != nil {
		t.Fatalf("resolveCompletePhaseRoute: %v", err)
	}
	if route != routeWorkflow {
		t.Fatalf("route = %v, want routeWorkflow", route)
	}
	if filepath.Base(phase) != "002_PLAN" {
		t.Fatalf("phase = %q, want 002_PLAN", phase)
	}
}

func TestResolveCompletePhaseRoute_NoneWhenAllComplete(t *testing.T) {
	festDir := scaffoldWorkflowGateFestival(t)
	ctx := context.Background()

	completeWorkflow(t, festDir, "001_INGEST", 2)
	completeWorkflow(t, festDir, "gate:001_INGEST", 1)
	completeWorkflow(t, festDir, "002_PLAN", 2)

	route, _, err := resolveCompletePhaseRoute(ctx, festDir)
	if err != nil {
		t.Fatalf("resolveCompletePhaseRoute: %v", err)
	}
	if route != routeNone {
		t.Fatalf("route = %v, want routeNone", route)
	}
}
