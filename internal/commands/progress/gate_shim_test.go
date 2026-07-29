package progress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/lifecycle"
	progresspkg "github.com/Obedience-Corp/fest/internal/progress"
)

// setupActiveFestivalForProgress builds an active-status festival with one
// implementation task and returns the festival dir plus the absolute task path,
// mirroring the task package's setupActiveFestival so the deprecated
// `fest progress` mutation shim can be exercised through handleTaskUpdate.
func setupActiveFestivalForProgress(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	festYAML := "version: \"1.0\"\nname: prog-shim-test\nid: PST-001\nmetadata:\n  id: PST-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".fest"), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	phaseGoal := "---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(dir, "001_PHASE", "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write phase goal: %v", err)
	}
	seqGoal := "---\nfest_type: sequence_goal\nfest_id: 01_seq\n---\n# Sequence Goal\n"
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatalf("write sequence goal: %v", err)
	}
	taskBody := "---\nfest_type: task\nfest_id: 01_task\n---\n# Task body\n"
	taskPath := filepath.Join(seqDir, "01_task.md")
	if err := os.WriteFile(taskPath, []byte(taskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}
	return dir, taskPath
}

// TestHandleTaskUpdate_100PercentRefusedOnDeprecatedShim locks the fest#280
// parity fix: the deprecated `fest progress --update 100%` path must refuse the
// ungated completion the same way `fest task update 100` does, pointing the
// caller at `fest task completed --yes` instead of silently completing the task
// without quality-gate evaluation during the deprecation window.
func TestHandleTaskUpdate_100PercentRefusedOnDeprecatedShim(t *testing.T) {
	festDir, taskPath := setupActiveFestivalForProgress(t)
	ctx := context.Background()

	mgr, err := progresspkg.NewManagerWithGate(ctx, festDir,
		lifecycle.NewGateWithReason(festDir, "fest progress"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	opts := &progressOptions{taskPath: taskPath, update: "100"}
	err = handleTaskUpdate(ctx, mgr, festDir, opts)
	if err == nil {
		t.Fatal("progress --update 100 unexpectedly succeeded; ungated completion side-door is open")
	}
	if !strings.Contains(err.Error(), "fest task completed") {
		t.Fatalf("error = %v, want a pointer to gated completion (fest task completed)", err)
	}

	// The task must not have been completed through the shim.
	taskID, rErr := resolveTaskID(festDir, opts)
	if rErr != nil {
		t.Fatalf("resolve task id: %v", rErr)
	}
	if task, ok := mgr.GetTaskProgress(taskID); ok && task != nil {
		if task.Status == progresspkg.StatusCompleted {
			t.Error("task must not be completed via progress --update 100")
		}
		if task.Progress == 100 {
			t.Error("task progress must not reach 100 via progress --update")
		}
	}
}

// TestHandleTaskUpdate_SubHundredStillUpdates guards against over-rejecting: a
// non-terminal percentage on the deprecated shim keeps working for the
// compatibility window.
func TestHandleTaskUpdate_SubHundredStillUpdates(t *testing.T) {
	festDir, taskPath := setupActiveFestivalForProgress(t)
	ctx := context.Background()

	mgr, err := progresspkg.NewManagerWithGate(ctx, festDir,
		lifecycle.NewGateWithReason(festDir, "fest progress"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	opts := &progressOptions{taskPath: taskPath, update: "50"}
	if err := handleTaskUpdate(ctx, mgr, festDir, opts); err != nil {
		t.Fatalf("progress --update 50 should still succeed on the shim: %v", err)
	}

	taskID, rErr := resolveTaskID(festDir, opts)
	if rErr != nil {
		t.Fatalf("resolve task id: %v", rErr)
	}
	task, ok := mgr.GetTaskProgress(taskID)
	if !ok || task == nil {
		t.Fatal("expected a progress entry after a sub-100 update")
	}
	if task.Progress != 50 {
		t.Errorf("progress = %d, want 50", task.Progress)
	}
}
