package progress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

func setupHookPropagationFestival(t *testing.T, seqGoalFrontmatter string) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := `version: "1.0"
name: propagate-hooks-test
id: PHK-001
metadata:
  id: PHK-001
hooks:
  definitions:
    seq-check:
      command: test-seq-check
`
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".fest"), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}
	phaseGoal := "---\nfest_type: phase_goal\nfest_id: 001_PHASE\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(dir, "001_PHASE", "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write phase goal: %v", err)
	}
	seqGoal := "---\nfest_type: sequence_goal\nfest_id: 01_seq\n" + seqGoalFrontmatter + "---\n# Sequence Goal\n"
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatalf("write sequence goal: %v", err)
	}
	taskBody := "---\nfest_type: task\nfest_id: 01_task\n---\n# Task body\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	t.Setenv("HOME", t.TempDir())
	return dir
}

func fakeLifecycleRunner(t *testing.T, exitCode int, called *[]string) {
	t.Helper()
	orig := newLifecycleHookRunner
	t.Cleanup(func() { newLifecycleHookRunner = orig })
	newLifecycleHookRunner = func(workDir string) *hooks.Runner {
		r := hooks.NewRunner(workDir)
		r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) hooks.CommandResult {
			if called != nil {
				*called = append(*called, command)
			}
			res := hooks.CommandResult{ExitCode: exitCode}
			if exitCode != 0 {
				res.Err = context.Canceled
			}
			return res
		}
		return r
	}
}

func seqGoalStatus(t *testing.T, festDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festDir, "001_PHASE", "01_seq", "SEQUENCE_GOAL.md"))
	if err != nil {
		t.Fatalf("read goal: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "fest_status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "fest_status:"))
		}
	}
	return ""
}

func readEventsFile(t *testing.T, festDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festDir, ".fest", "progress_events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read events: %v", err)
	}
	return string(data)
}

func TestPropagateSequenceCompletion_BlockedPreLeavesGoalIncomplete(t *testing.T) {
	festDir := setupHookPropagationFestival(t, "hooks:\n  pre: [seq-check]\n")
	var called []string
	fakeLifecycleRunner(t, 1, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	if got := seqGoalStatus(t, festDir); got == "completed" {
		t.Fatal("blocked pre hook must leave the sequence goal incomplete")
	}
	if len(called) != 1 {
		t.Fatalf("hook exec calls = %v", called)
	}
	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_verb":"sequence_complete"`) || !strings.Contains(events, `"hook_blocked":true`) {
		t.Fatalf("blocked sequence_complete event missing:\n%s", events)
	}
}

func TestPropagateSequenceCompletion_PassingHooksCompleteGoalOnce(t *testing.T) {
	festDir := setupHookPropagationFestival(t, "hooks:\n  pre: [seq-check]\n  post: [seq-check]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	if got := seqGoalStatus(t, festDir); got != "completed" {
		t.Fatalf("sequence goal status = %q, want completed", got)
	}
	if len(called) != 2 { // one pre + one post
		t.Fatalf("hook exec calls = %v", called)
	}

	// Re-propagating an already-completed goal must not re-run hooks.
	if err := mgr.propagateSequenceCompletion(context.Background(), filepath.Join(festDir, "001_PHASE", "01_seq")); err != nil {
		t.Fatalf("re-propagate: %v", err)
	}
	if len(called) != 2 {
		t.Fatalf("completion hooks re-ran on already-completed goal: %v", called)
	}

	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_timing":"pre"`) || !strings.Contains(events, `"hook_timing":"post"`) {
		t.Fatalf("pre/post sequence events missing:\n%s", events)
	}
}
