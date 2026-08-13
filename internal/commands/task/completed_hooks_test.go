package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupHookedFestival builds an active festival whose fest.yaml declares one
// hook definition named "checker" running checkerCommand. Completion hooks now
// execute inside progress.Manager, so these tests drive the real runner with
// real commands (true/false) instead of faking an exec seam.
func setupHookedFestival(t *testing.T, taskFrontmatterExtra, checkerCommand string) (string, string) {
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
      command: ` + checkerCommand + `
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
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [checker]\n", "false")
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
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [ghost]\n", "false")
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("undeclared binding must skip, not fail: %v", err)
	}

	taskID := canonicalTaskID(t, festDir, taskRel)
	task, ok := taskStatus(t, festDir, taskID)
	if !ok || task.Status != "completed" {
		t.Fatalf("task not completed: %+v", task)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"hook_skip":"undeclared"`) {
		t.Fatalf("undeclared skip event missing:\n%s", events)
	}
}

func TestRunCompleted_HumanGateSkipsAutomationHooks(t *testing.T) {
	// checker would block if it ever ran; the human gate must skip it.
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [checker]\napproval: human-required\n", "false")
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("human-gate step must skip automation hooks: %v", err)
	}

	taskID := canonicalTaskID(t, festDir, taskRel)
	task, ok := taskStatus(t, festDir, taskID)
	if !ok || task.Status != "completed" {
		t.Fatalf("task not completed: %+v", task)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"hook_skip":"human-gate"`) {
		t.Fatalf("human-gate skip event missing:\n%s", events)
	}
}

func TestRunCompleted_PostHookRunsAfterCompletion(t *testing.T) {
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  post: [checker]\n", "true")
	resetTaskFlags()
	completedYes = true

	var err error
	captureIO(t, func() {
		err = runCompleted(taskCmd(t, festDir), []string{taskRel})
	})
	if err != nil {
		t.Fatalf("runCompleted: %v", err)
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
