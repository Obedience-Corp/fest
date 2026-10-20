package next

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// fest next in phase-gate mode lists every blocked task in the phase, with its
// blocker and deferral, above the checkpoint question.
func TestPhaseGateModeListsBlockedTasksBeforeQuestion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEST_CONFIG_DIR", t.TempDir())
	ctx := context.Background()

	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_IMPLEMENT")
	seqDir := filepath.Join(phaseDir, "01_build")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte("name: gate-render\nid: GR-001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"),
		[]byte("---\nfest_type: phase\nfest_id: 001_IMPLEMENT\nfest_phase_type: implementation\n---\n\n# Implement\n"), 0o644); err != nil {
		t.Fatalf("write goal: %v", err)
	}
	gates := "---\nfest_type: phase_gate\nfest_id: 001_IMPLEMENT-GATE\n---\n\n# Gate\n\n" +
		"## Step 1: COMPLETENESS — Verify Work Done\n\n**Question:** Were all tasks completed?\n\n" +
		"**Actions:**\n1. Verify task completion\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}
	for _, name := range []string{"01_task.md", "02_task.md"} {
		if err := os.WriteFile(filepath.Join(seqDir, name), []byte("# Task\n"), 0o644); err != nil {
			t.Fatalf("write task: %v", err)
		}
	}

	mgr, err := progress.NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(ctx, "001_IMPLEMENT/01_build/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	blocked := "001_IMPLEMENT/01_build/02_task.md"
	if err := mgr.ReportBlocker(ctx, blocked, "vendor API removed", nil); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}
	if err := mgr.DeferBlocker(ctx, blocked, "independent of the gate", progress.DeferralAudit{Actor: "test"}); err != nil {
		t.Fatalf("DeferBlocker: %v", err)
	}

	gate, err := findFirstIncompletePhaseGate(ctx, festDir)
	if err != nil || gate != phaseDir {
		t.Fatalf("findFirstIncompletePhaseGate = %q, %v; want the phase once its only blocker is deferred", gate, err)
	}

	out, err := captureStdoutErr(t, func() error { return runPhaseGateMode(ctx, festDir, phaseDir) })
	if err != nil {
		t.Fatalf("runPhaseGateMode: %v", err)
	}
	for _, want := range []string{
		"BLOCKED TASKS IN THIS PHASE: 1 (1 deferred)",
		"01_build/02_task.md [deferred]",
		"Blocker: vendor API removed",
		"Deferred because: independent of the gate",
		"The gate decision must account for them",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("phase gate guidance lacks %q:\n%s", want, out)
		}
	}
	block, question := strings.Index(out, "BLOCKED TASKS"), strings.Index(out, "**Question:**")
	if question < 0 || block > question {
		t.Fatalf("blocked tasks must come before the checkpoint question:\n%s", out)
	}
}
