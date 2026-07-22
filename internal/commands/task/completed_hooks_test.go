package task

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

func setupHookedFestival(t *testing.T, taskFrontmatterExtra string) (string, string) {
	t.Helper()
	festDir, taskRel := setupActiveFestival(t)

	festYAML := `version: "1.0"
name: task-hooks-test
id: THK-001
metadata:
  id: THK-001
  status_history:
    - status: active
      timestamp: 2026-02-10T00:00:00Z
hooks:
  definitions:
    checker:
      command: test-checker
`
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	taskBody := "---\nfest_type: task\nfest_id: 01_task\n" + taskFrontmatterExtra + "---\n# Task body\n"
	if err := os.WriteFile(filepath.Join(festDir, taskRel), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	// Machine-layer isolation: never read the developer's real ~/.obey/fest.
	t.Setenv("HOME", t.TempDir())
	return festDir, taskRel
}

func fakeTaskRunner(t *testing.T, exitCode int, called *[]string) {
	t.Helper()
	orig := newTaskHookRunner
	t.Cleanup(func() { newTaskHookRunner = orig })
	newTaskHookRunner = func(workDir string) *hooks.Runner {
		r := hooks.NewRunner(workDir)
		r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) hooks.CommandResult {
			if called != nil {
				*called = append(*called, command)
			}
			res := hooks.CommandResult{ExitCode: exitCode}
			if exitCode != 0 {
				res.Err = context.Canceled // any non-nil error stands in for exec failure
			}
			return res
		}
		return r
	}
}

func readHookEvents(t *testing.T, festDir string) string {
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

func TestRunCompleted_PreHookBlockedRefusesCompletion(t *testing.T) {
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [checker]\n")
	var called []string
	fakeTaskRunner(t, 1, &called)
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err == nil {
		t.Fatal("expected fail-closed pre hook to block completion")
	}
	if !strings.Contains(err.Error(), "blocked by fail-closed hook") {
		t.Fatalf("err = %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("hook exec calls = %v", called)
	}

	taskID := canonicalTaskID(t, festDir, taskRel)
	if task, ok := taskStatus(t, festDir, taskID); ok && task.Status == "completed" {
		t.Fatalf("task must not be completed, got %+v", task)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"wf_hook_run"`) || !strings.Contains(events, `"hook_blocked":true`) {
		t.Fatalf("blocked wf_hook_run event missing:\n%s", events)
	}
	if !strings.Contains(events, `"hook_verb":"task_complete"`) {
		t.Fatalf("task_complete verb missing:\n%s", events)
	}
}

func TestRunCompleted_UndeclaredBindingSkipsAndCompletes(t *testing.T) {
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [ghost]\n")
	var called []string
	fakeTaskRunner(t, 0, &called)
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("undeclared binding must skip, not fail: %v", err)
	}
	if len(called) != 0 {
		t.Fatalf("undeclared hook must never exec: %v", called)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"hook_skip":"undeclared"`) {
		t.Fatalf("undeclared skip event missing:\n%s", events)
	}
}

func TestRunCompleted_HumanGateSkipsAutomationHooks(t *testing.T) {
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [checker]\napproval: human-required\n")
	var called []string
	fakeTaskRunner(t, 1, &called) // would block if it ever ran
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("human-gate step must skip automation hooks: %v", err)
	}
	if len(called) != 0 {
		t.Fatalf("automation hooks must never exec on a human gate: %v", called)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"hook_skip":"human-gate"`) {
		t.Fatalf("human-gate skip event missing:\n%s", events)
	}
}

func TestRunCompleted_PostHookRunsAfterCompletion(t *testing.T) {
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  post: [checker]\n")
	var called []string
	fakeTaskRunner(t, 0, &called)
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("runCompleted: %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("post hook exec calls = %v", called)
	}

	taskID := canonicalTaskID(t, festDir, taskRel)
	task, ok := taskStatus(t, festDir, taskID)
	if !ok || task.Status != "completed" {
		t.Fatalf("task not completed: %+v", task)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"hook_timing":"post"`) || !strings.Contains(events, `"hook_outcome":"pass"`) {
		t.Fatalf("post pass event missing:\n%s", events)
	}
}
