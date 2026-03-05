package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

func TestResetPhaseStatus(t *testing.T) {
	ctx := context.Background()
	festDir := t.TempDir()

	phaseDir := filepath.Join(festDir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create PHASE_GOAL.md marked as completed
	phaseGoal := `---
fest_type: phase
fest_id: 001_PHASE
fest_status: completed
fest_created: 2026-03-01T00:00:00Z
---

# Phase Goal
`
	goalPath := filepath.Join(phaseDir, "PHASE_GOAL.md")
	if err := os.WriteFile(goalPath, []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Reset the phase status
	if err := mgr.ResetPhaseStatus(ctx, phaseDir); err != nil {
		t.Fatalf("ResetPhaseStatus: %v", err)
	}

	// Verify frontmatter now says in_progress
	got := readStatus(t, goalPath)
	if got != frontmatter.StatusInProgress {
		t.Errorf("PHASE_GOAL.md status = %q, want %q", got, frontmatter.StatusInProgress)
	}
}

func TestResetPhaseStatus_AlreadyInProgress(t *testing.T) {
	ctx := context.Background()
	festDir := t.TempDir()

	phaseDir := filepath.Join(festDir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	phaseGoal := `---
fest_type: phase
fest_id: 001_PHASE
fest_status: in_progress
fest_created: 2026-03-01T00:00:00Z
---

# Phase Goal
`
	goalPath := filepath.Join(phaseDir, "PHASE_GOAL.md")
	if err := os.WriteFile(goalPath, []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Should be a no-op (already in_progress)
	if err := mgr.ResetPhaseStatus(ctx, phaseDir); err != nil {
		t.Fatalf("ResetPhaseStatus: %v", err)
	}

	// Status should remain in_progress
	got := readStatus(t, goalPath)
	if got != frontmatter.StatusInProgress {
		t.Errorf("PHASE_GOAL.md status = %q, want %q", got, frontmatter.StatusInProgress)
	}
}

func TestResetPhaseStatus_NoGoalFile(t *testing.T) {
	ctx := context.Background()
	festDir := t.TempDir()

	phaseDir := filepath.Join(festDir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Should not error when PHASE_GOAL.md doesn't exist
	if err := mgr.ResetPhaseStatus(ctx, phaseDir); err != nil {
		t.Errorf("ResetPhaseStatus should not error on missing goal file, got: %v", err)
	}
}

func TestResetPhaseStatus_ContextCancellation(t *testing.T) {
	ctx := context.Background()
	festDir := t.TempDir()

	mgr, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = mgr.ResetPhaseStatus(cancelledCtx, filepath.Join(festDir, "001_PHASE"))
	if err == nil {
		t.Error("ResetPhaseStatus should return error on cancelled context")
	}
}
