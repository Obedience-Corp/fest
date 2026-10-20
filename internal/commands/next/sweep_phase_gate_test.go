package next

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// sweepGateFestival lays out FA0032's shape: phase 001 has every task settled
// (one deferred) and its gate approved; phase 002 is gates-only and unwalked.
func sweepGateFestival(t *testing.T) string {
	t.Helper()
	festDir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(festDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	gate := func(phase, step, question string) string {
		return "---\nfest_type: phase_gate\nfest_id: " + phase + "-GATE\n---\n\n# Gate\n\n## Step 1: " + step +
			" — Check\n\n**Question:** " + question + "\n\n**Actions:**\n1. check\n\n**Checkpoint:** APPROVAL REQUIRED\n"
	}

	write("fest.yaml", "name: sweep-gate\nid: SG-001\nversion: \"1.0\"\nauto_link:\n  enabled: false\nmetadata:\n  id: SG-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n")
	write("FESTIVAL_GOAL.md", "# Sweep gate\n")
	write("FESTIVAL_OVERVIEW.md", "# Sweep gate overview\n")
	write("FESTIVAL_RULES.md", "# Rules\n")
	write("001_IMPLEMENT/PHASE_GOAL.md", "---\nfest_type: phase\nfest_id: 001_IMPLEMENT\nfest_phase_type: implementation\n---\n\n# Implement\n")
	write("001_IMPLEMENT/GATES.md", gate("001_IMPLEMENT", "COMPLETENESS", "Was the implementation finished?"))
	write("001_IMPLEMENT/01_build/SEQUENCE_GOAL.md", "---\nfest_type: sequence\nfest_id: 01_build\n---\n\n# Build\n")
	write("001_IMPLEMENT/01_build/01_task.md", "# Task one\n")
	write("001_IMPLEMENT/01_build/02_task.md", "# Task two\n")
	gateTypes := map[string]string{"03_testing.md": "testing", "04_review.md": "review", "05_iterate.md": "iterate", "06_fest_commit.md": "commit"}
	qualityGates := []string{"03_testing.md", "04_review.md", "05_iterate.md", "06_fest_commit.md"}
	for _, name := range qualityGates {
		write("001_IMPLEMENT/01_build/"+name, "---\nfest_type: gate\nfest_id: "+name+"\nfest_gate_id: "+gateTypes[name]+
			"\nfest_gate_type: "+gateTypes[name]+"\n---\n\n# Gate\n")
	}
	write("002_REVIEW/PHASE_GOAL.md", "---\nfest_type: phase\nfest_id: 002_REVIEW\nfest_phase_type: review\n---\n\n# Review\n")
	write("002_REVIEW/GATES.md", gate("002_REVIEW", "COVERAGE", "Did the review cover every change?"))

	ctx := context.Background()
	mgr, err := progress.NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for _, name := range append([]string{"01_task.md"}, qualityGates...) {
		if err := mgr.MarkComplete(ctx, "001_IMPLEMENT/01_build/"+name); err != nil {
			t.Fatalf("MarkComplete(%s): %v", name, err)
		}
	}
	deferred := "001_IMPLEMENT/01_build/02_task.md"
	if err := mgr.ReportBlocker(ctx, deferred, "vendor API removed", nil); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}
	if err := mgr.DeferBlocker(ctx, deferred, "independent of the review", progress.DeferralAudit{Actor: "test"}); err != nil {
		t.Fatalf("DeferBlocker: %v", err)
	}
	completeGate(t, festDir, "gate:001_IMPLEMENT")
	return festDir
}

func completeGate(t *testing.T, festDir, key string) {
	t.Helper()
	ctx := context.Background()
	store := progress.NewStore(festDir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	store.QueueWorkflowEvents(wf.EmitInitEvents(key, 1))
	store.QueueWorkflowEvents(wf.EmitStepDoneWithDecisionEvents(key, 1, "ok", wf.DecisionMetadata{Actor: "user", Summary: "ok"}))
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
}

func runNextCommand(t *testing.T, dir string) string {
	t.Helper()
	t.Chdir(dir)
	t.Setenv("PWD", dir)
	cmd := NewNextCommand()
	cmd.SetArgs([]string{})
	cmd.SetContext(context.Background())
	out, err := captureStdoutErr(t, cmd.Execute)
	if err != nil {
		t.Fatalf("fest next: %v\n%s", err, out)
	}
	return out
}

func sweepEventCount(t *testing.T, festDir string) int {
	t.Helper()
	events, err := progress.NewStore(festDir).ReadEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	n := 0
	for _, e := range events {
		if e.Event == progress.EventSweepStarted || e.Event == progress.EventBlockerRevisited {
			n++
		}
	}
	return n
}

// Deferred blockers come back only when everything else is settled. A later
// phase's unwalked GATES.md is not settled, so fest next walks it first, and
// only then starts the sweep.
func TestNextWalksLaterPhaseGateBeforeDeferredSweep(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEST_CONFIG_DIR", t.TempDir())
	festDir := sweepGateFestival(t)

	out := runNextCommand(t, festDir)
	if !strings.Contains(out, "Did the review cover every change?") || !strings.Contains(out, "002_REVIEW") {
		t.Fatalf("fest next must surface 002_REVIEW's gate while it is unwalked:\n%s", out)
	}
	if strings.Contains(out, "revisiting a deferred blocker") {
		t.Fatalf("fest next started the sweep ahead of an unwalked phase gate:\n%s", out)
	}
	if n := sweepEventCount(t, festDir); n != 0 {
		t.Fatalf("fest next recorded %d sweep events while a phase gate remained", n)
	}

	completeGate(t, festDir, "gate:002_REVIEW")
	out = runNextCommand(t, festDir)
	if !strings.Contains(out, "revisiting a deferred blocker") || !strings.Contains(out, "02_task") {
		t.Fatalf("with every gate walked, fest next must hand back the deferred task:\n%s", out)
	}
}
