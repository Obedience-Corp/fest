package progress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func setupStartHookFestival(t *testing.T, taskFrontmatter string) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := `version: "1.0"
name: start-hooks-test
id: SHK-001
metadata:
  id: SHK-001
hooks:
  definitions:
    start-anchor:
      command: test-start-anchor
    start-notify:
      command: test-start-notify
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
	task := "---\nfest_type: task\nfest_id: 01_task\n" + taskFrontmatter + "---\n# Task body\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(task), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	t.Setenv("HOME", t.TempDir())
	return dir
}

func fakeLifecycleRunnerPerCommand(t *testing.T, exits map[string]int, called *[]string) {
	t.Helper()
	orig := newLifecycleHookRunner
	t.Cleanup(func() { newLifecycleHookRunner = orig })
	newLifecycleHookRunner = func(workDir string) *hooks.Runner {
		r := hooks.NewRunner(workDir)
		r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) hooks.CommandResult {
			if called != nil {
				*called = append(*called, command)
			}
			res := hooks.CommandResult{ExitCode: exits[command]}
			if res.ExitCode != 0 {
				res.Err = context.Canceled
			}
			return res
		}
		return r
	}
}

func TestMarkInProgress_BlockedStartPreLeavesTaskUnstarted(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 1, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	err = mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md")
	if err == nil {
		t.Fatal("blocked start pre hook must fail MarkInProgress")
	}
	if !strings.Contains(err.Error(), "blocked by fail-closed hook") {
		t.Fatalf("error = %v", err)
	}
	if task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md"); exists && task.Status == StatusInProgress {
		t.Fatal("blocked start pre hook must leave the task unstarted")
	}
	if len(called) != 1 {
		t.Fatalf("hook exec calls = %v", called)
	}
	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_verb":"task_start"`) || !strings.Contains(events, `"hook_blocked":true`) {
		t.Fatalf("blocked task_start event missing:\n%s", events)
	}
	if strings.Contains(events, `"event":"started"`) {
		t.Fatalf("started event must not be recorded on a blocked start:\n%s", events)
	}
}

func TestMarkInProgress_StartHooksRunAroundTransition(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n    post: [start-notify]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}

	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.Status != StatusInProgress {
		t.Fatalf("task not in progress: exists=%v task=%+v", exists, task)
	}
	if len(called) != 2 || called[0] != "test-start-anchor" || called[1] != "test-start-notify" {
		t.Fatalf("hook exec calls = %v", called)
	}
	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_verb":"task_start"`) ||
		!strings.Contains(events, `"hook_timing":"pre"`) ||
		!strings.Contains(events, `"hook_timing":"post"`) {
		t.Fatalf("task_start pre/post events missing:\n%s", events)
	}
	if !strings.Contains(events, `"event":"started"`) {
		t.Fatalf("started event missing:\n%s", events)
	}

	// Re-entering in_progress must not re-fire start hooks.
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("second MarkInProgress: %v", err)
	}
	if len(called) != 2 {
		t.Fatalf("start hooks re-fired on second start: %v", called)
	}
}

func TestMarkInProgress_HumanGateSkipsStartHooks(t *testing.T) {
	festDir := setupStartHookFestival(t, "approval: human-required\nhooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if len(called) != 0 {
		t.Fatalf("human-gated step must not exec hooks: %v", called)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.Status != StatusInProgress {
		t.Fatalf("task not in progress: exists=%v task=%+v", exists, task)
	}
	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_skip":"human-gate"`) {
		t.Fatalf("human-gate skip record missing:\n%s", events)
	}
}

func TestMarkInProgress_FailedClosedStartPostKeepsTaskStarted(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    post: [start-notify]\n")
	var called []string
	fakeLifecycleRunnerPerCommand(t, map[string]int{"test-start-notify": 1}, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("failed post hook must not fail MarkInProgress: %v", err)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.Status != StatusInProgress {
		t.Fatalf("task must stay started: exists=%v task=%+v", exists, task)
	}
	events := readEventsFile(t, festDir)
	if !strings.Contains(events, `"hook_outcome":"fail"`) {
		t.Fatalf("failed post hook audit record missing:\n%s", events)
	}
}

func TestMarkInProgress_NoStartBindingsRunsNoHooks(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if len(called) != 0 {
		t.Fatalf("completion-stage bindings must not run at start: %v", called)
	}
}

func TestUpdateProgress_FirstProgressFiresStartHooks(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n    post: [start-notify]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.UpdateProgress(context.Background(), "001_PHASE/01_seq/01_task.md", 10); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	if len(called) != 2 || called[0] != "test-start-anchor" || called[1] != "test-start-notify" {
		t.Fatalf("hook exec calls = %v", called)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.Status != StatusInProgress || task.StartedAt == nil {
		t.Fatalf("task not started: exists=%v task=%+v", exists, task)
	}

	// Neither a later progress update nor an explicit in-progress re-fires.
	if err := mgr.UpdateProgress(context.Background(), "001_PHASE/01_seq/01_task.md", 20); err != nil {
		t.Fatalf("second UpdateProgress: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if len(called) != 2 {
		t.Fatalf("start hooks re-fired after first progress: %v", called)
	}
}

func TestUpdateProgress_CompleteReplayDoesNotRefireStartHooks(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.UpdateProgress(context.Background(), "001_PHASE/01_seq/01_task.md", 100); err != nil {
		t.Fatalf("UpdateProgress(100): %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("start hook calls after completion = %v", called)
	}

	// A lone completed event must preserve the first-start predicate across
	// reload, so a later mutation cannot re-run task_start.
	mgr, err = NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager reload: %v", err)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.StartedAt == nil {
		t.Fatalf("completion replay lost StartedAt: exists=%v task=%+v", exists, task)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress after reload: %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("start hook re-fired after completion replay: %v", called)
	}
}

func TestReportBlockerBeforeStartKeepsStartHooksArmed(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.ReportBlocker(context.Background(), "001_PHASE/01_seq/01_task.md", "waiting"); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.StartedAt != nil {
		t.Fatalf("blocker-first must leave task_start armed: exists=%v task=%+v", exists, task)
	}
	if len(called) != 0 {
		t.Fatalf("reporting a pre-start blocker ran start hooks: %v", called)
	}

	if err := mgr.ClearBlocker(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("ClearBlocker: %v", err)
	}
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("start hook did not fire after blocker-first path: %v", called)
	}
}

func TestMarkComplete_FirstWorkFiresStartHooks(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n    post: [start-notify]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.MarkComplete(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if len(called) != 2 || called[0] != "test-start-anchor" || called[1] != "test-start-notify" {
		t.Fatalf("hook exec calls = %v", called)
	}
	task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md")
	if !exists || task.Status != StatusCompleted || task.StartedAt == nil {
		t.Fatalf("task not completed after start hooks: exists=%v task=%+v", exists, task)
	}
}

func TestMarkComplete_BlockedStartPreLeavesTaskUncompleted(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 1, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	err = mgr.MarkComplete(context.Background(), "001_PHASE/01_seq/01_task.md")
	if err == nil || !strings.Contains(err.Error(), "blocked by fail-closed hook") {
		t.Fatalf("MarkComplete error = %v, want blocked start hook", err)
	}
	if task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md"); exists &&
		(task.Status == StatusCompleted || task.Progress == 100 || task.StartedAt != nil) {
		t.Fatalf("blocked start pre must leave the task uncompleted: %+v", task)
	}
	events := readEventsFile(t, festDir)
	if strings.Contains(events, `"event":"completed"`) {
		t.Fatalf("completion event must not be recorded on a blocked start:\n%s", events)
	}
}

func TestUpdateProgress_BlockedStartPreLeavesTaskUntouched(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 1, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	err = mgr.UpdateProgress(context.Background(), "001_PHASE/01_seq/01_task.md", 10)
	if err == nil {
		t.Fatal("blocked start pre hook must fail UpdateProgress")
	}
	if !strings.Contains(err.Error(), "blocked by fail-closed hook") {
		t.Fatalf("error = %v", err)
	}
	if task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md"); exists &&
		(task.Status == StatusInProgress || task.Progress != 0 || task.StartedAt != nil) {
		t.Fatalf("blocked start pre must leave the task untouched: %+v", task)
	}
	events := readEventsFile(t, festDir)
	if strings.Contains(events, `"event":"progress"`) {
		t.Fatalf("progress event must not be recorded on a blocked start:\n%s", events)
	}
	if !strings.Contains(events, `"hook_verb":"task_start"`) || !strings.Contains(events, `"hook_blocked":true`) {
		t.Fatalf("blocked task_start event missing:\n%s", events)
	}
}

func TestUpdateProgress_ZeroProgressKeepsStartAnchorArmed(t *testing.T) {
	festDir := setupStartHookFestival(t, "hooks:\n  start:\n    pre: [start-anchor]\n")
	var called []string
	fakeLifecycleRunner(t, 0, &called)

	mgr, err := NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.UpdateProgress(context.Background(), "001_PHASE/01_seq/01_task.md", 0); err != nil {
		t.Fatalf("UpdateProgress(0): %v", err)
	}
	if len(called) != 0 {
		t.Fatalf("zero progress must not fire start hooks: %v", called)
	}
	if task, exists := mgr.GetTaskProgress("001_PHASE/01_seq/01_task.md"); exists && task.StartedAt != nil {
		t.Fatalf("zero progress must not record a start: %+v", task)
	}

	// The anchor still fires when work actually begins.
	if err := mgr.MarkInProgress(context.Background(), "001_PHASE/01_seq/01_task.md"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if len(called) != 1 {
		t.Fatalf("start anchor lost after zero progress update: %v", called)
	}
}

func TestMaterializeState_ProgressEventSetsStartedAt(t *testing.T) {
	ts := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	tasks := materializeState([]ProgressEvent{
		{Timestamp: ts, Event: EventProgress, Task: "t1", Percent: 10},
	})
	task := tasks["t1"]
	if task == nil || task.StartedAt == nil || !task.StartedAt.Equal(ts) {
		t.Fatalf("progress replay must set StartedAt: %+v", task)
	}
	// Zero progress replay records no start, matching the live path.
	tasks = materializeState([]ProgressEvent{
		{Timestamp: ts, Event: EventProgress, Task: "t1", Percent: 0},
	})
	if task := tasks["t1"]; task == nil || task.StartedAt != nil {
		t.Fatalf("zero progress replay must not set StartedAt: %+v", task)
	}
}
