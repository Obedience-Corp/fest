package status

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/ui"
)

// setupTaskHookFixture builds an active festival whose task binds one
// completion hook named "checker" running checkerCommand (a real command:
// true to pass, false to fail closed).
func setupTaskHookFixture(t *testing.T, checkerCommand string) (string, string) {
	t.Helper()
	dir := t.TempDir()

	festYAML := `version: "1.0"
name: task-status-hooks-test
id: TSH-001
metadata:
  id: TSH-001
  status_history:
    - status: active
      timestamp: 2026-02-10T00:00:00Z
hooks:
  definitions:
    checker:
      command: ` + checkerCommand + `
`
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	task := "---\nfest_type: task\nfest_id: 01_task\nhooks:\n  pre: [checker]\n---\n# Task body\n"
	taskRel := filepath.Join("001_PHASE", "01_seq", "01_task.md")
	if err := os.WriteFile(filepath.Join(dir, taskRel), []byte(task), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	// Machine-layer isolation: never read the developer's real ~/.obey/fest.
	t.Setenv("HOME", t.TempDir())
	return dir, taskRel
}

// TestTaskStatusSetCompletedRunsCompletionHooks is the regression test for
// the bypass this change closes: 'fest status set <task> completed' used to
// mark a task completed without ever consulting its completion hook bindings,
// because those hooks lived only in the 'fest task completed' command layer.
func TestTaskStatusSetCompletedRunsCompletionHooks(t *testing.T) {
	t.Run("fail_closed_hook_blocks_completion", func(t *testing.T) {
		festDir, taskRel := setupTaskHookFixture(t, "false")

		err := handleTaskStatusSet(context.Background(), &ui.UI{}, festDir,
			"completed", &statusOptions{task: taskRel, noCommit: true})
		if err == nil {
			t.Fatal("fail-closed completion hook must block status set completed")
		}
		if !strings.Contains(err.Error(), "blocked by fail-closed hook") {
			t.Fatalf("err = %v", err)
		}

		events := readTaskHookEvents(t, festDir)
		if !strings.Contains(events, `"hook_verb":"task_complete"`) || !strings.Contains(events, `"hook_blocked":true`) {
			t.Fatalf("blocked task_complete event missing:\n%s", events)
		}
		if strings.Contains(events, `"event":"completed"`) {
			t.Fatalf("completed event must not be recorded on a blocked completion:\n%s", events)
		}
	})

	t.Run("passing_hook_completes_with_audit_trail", func(t *testing.T) {
		festDir, taskRel := setupTaskHookFixture(t, "true")

		err := handleTaskStatusSet(context.Background(), &ui.UI{}, festDir,
			"completed", &statusOptions{task: taskRel, noCommit: true})
		if err != nil {
			t.Fatalf("handleTaskStatusSet: %v", err)
		}

		events := readTaskHookEvents(t, festDir)
		if !strings.Contains(events, `"hook_verb":"task_complete"`) || !strings.Contains(events, `"hook_outcome":"pass"`) {
			t.Fatalf("task_complete pass event missing:\n%s", events)
		}
		if !strings.Contains(events, `"event":"completed"`) {
			t.Fatalf("completed event missing:\n%s", events)
		}
	})
}

func readTaskHookEvents(t *testing.T, festDir string) string {
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
