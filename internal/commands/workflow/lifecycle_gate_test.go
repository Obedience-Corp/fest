package workflow

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
)

// TestWorkflowGate_BlocksImplPhaseInPlanningFestival is the regression test
// for the bypass that PR #162 originally missed: an agent could drive a
// WORKFLOW.md-based implementation phase to completion via fest workflow
// advance while the festival was still in planning status.
func TestWorkflowGate_BlocksImplPhaseInPlanningFestival(t *testing.T) {
	festDir := setupImplWorkflowFestival(t, "planning")
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Setenv("PWD", festDir)
	t.Chdir(festDir)

	cases := []struct {
		name string
		call func() error
	}{
		{"advance", func() error { return runAdvance(ctx) }},
		{"approve", func() error { return runApprove(ctx) }},
		{"reject", func() error { return runReject(ctx, "test") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("expected lifecycle gate to block fest workflow %s in planning, got nil", tc.name)
			}
			if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
				t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
			}
		})
	}
}

func TestWorkflowGate_AllowsImplPhaseInActiveFestival(t *testing.T) {
	festDir := setupImplWorkflowFestival(t, "active")
	ctx := scope.WithFestival(context.Background(), festDir)
	t.Chdir(festDir)

	// runAdvance against an empty workflow state should not be blocked by
	// the lifecycle gate. (It may fail later for other reasons; we only
	// care that the gate did not fire.)
	err := runAdvance(ctx)
	if stderrors.Is(err, errors.ErrAlreadyPrinted) {
		t.Errorf("active festival: lifecycle gate fired unexpectedly: %v", err)
	}
}

// setupImplWorkflowFestival builds a minimal festival with a single
// implementation-typed phase containing a one-step WORKFLOW.md.
func setupImplWorkflowFestival(t *testing.T, status string) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := "name: lifecycle-test\nid: LFC-001\nversion: \"1.0\"\nmetadata:\n  id: LFC-001\n  status_history:\n    - status: " + status + "\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	phaseDir := filepath.Join(dir, "001_IMPL")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	phaseGoal := `---
fest_type: phase_goal
fest_id: 001_IMPL
fest_phase_type: implementation
---

# Implementation Phase
`
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	workflowMD := `---
fest_type: workflow
fest_id: LFC-WF
fest_parent: 001_IMPL
---

# Implementation Workflow

## Step 1: BUILD — Build the thing

**Goal:** Build it.

**Actions:**
1. Build

**Output:** Built thing

**Checkpoint:** None
`
	if err := os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(workflowMD), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	return dir
}
