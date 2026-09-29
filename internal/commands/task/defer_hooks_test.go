package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allowOperatorGuard makes the guard pass so the success path of fest task
// defer can be exercised off a terminal. It replaces the three seams the guard
// reads and nothing else, so every rule still runs.
func allowOperatorGuard(t *testing.T) {
	t.Helper()
	stubOperatorGuardTerminal(t, true)
	stubOperatorGuardAncestry(t, []string{"zsh", "login"}, nil)
	clearOperatorAgentMarkers(t)
}

func deferHookedFixture(t *testing.T) (string, string) {
	t.Helper()
	festDir, taskRel := setupHookedFestival(t, "hooks:\n  pre: [checker]\n  post: [checker]\n", "true")
	resetTaskFlags()
	blockedYes = true
	blockedReason = "provider API removed"
	captureIO(t, func() {
		if err := runBlocked(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runBlocked() error = %v", err)
		}
	})
	return festDir, taskRel
}

// TestDeferMatchesSiblingHookBehaviour records the finding that design doc 02
// "Interactions" assumes a hook lifecycle the sibling verbs do not have.
// hooks.AllVerbs is task_start, task_complete, sequence_complete, phase_complete
// and gate_approve; blocked, unblock and reset run none of them. Defer matches
// its siblings rather than inventing a sixth verb.
func TestDeferMatchesSiblingHookBehaviour(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)

	if got := readHookEvents(t, festDir); strings.Contains(got, "wf_hook_run") {
		t.Fatalf("fest task blocked ran a hook; the premise of this test is wrong:\n%s", got)
	}

	allowOperatorGuard(t)
	stubOperatorPrompt(t, "01\n")
	deferReason = "the vendor replies next week"
	t.Cleanup(func() { deferReason = "" })

	captureIO(t, func() {
		if err := runDefer(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runDefer() error = %v", err)
		}
	})

	events := readHookEvents(t, festDir)
	if strings.Contains(events, "wf_hook_run") {
		t.Errorf("fest task defer ran a hook that its siblings do not run:\n%s", events)
	}
	if !strings.Contains(events, `"event":"blocker_deferred"`) {
		t.Errorf("the deferral event is missing from the log:\n%s", events)
	}

	resetTaskFlags()
	completedYes = true
	captureIO(t, func() {
		if err := runCompleted(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runCompleted() error = %v", err)
		}
	})

	if got := readHookEvents(t, festDir); !strings.Contains(got, "wf_hook_run") {
		t.Errorf("fest task completed ran no hook, so the fixture proves nothing:\n%s", got)
	}
}

func TestDeferSucceedsOnABlockedTaskAndLeavesTheFileAlone(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)

	taskPath := filepath.Join(festDir, taskRel)
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("reading task file: %v", err)
	}

	allowOperatorGuard(t)
	stubOperatorPrompt(t, "01\n")
	deferReason = "the vendor replies next week"
	t.Cleanup(func() { deferReason = "" })

	output := captureIO(t, func() {
		if err := runDefer(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runDefer() error = %v", err)
		}
	})

	if !strings.Contains(output, "Blocker deferred") {
		t.Errorf("output missing the deferral confirmation:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("output must still show the task as blocked:\n%s", output)
	}

	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("reading task file: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("task file changed; a deferral must never be mirrored into frontmatter\nbefore:\n%s\nafter:\n%s", before, after)
	}

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"deferral_reason":"the vendor replies next week"`) {
		t.Errorf("event log missing the operator reason:\n%s", events)
	}
	if !strings.Contains(events, `"actor":"operator"`) || !strings.Contains(events, `"tty":true`) {
		t.Errorf("event log missing the guard audit:\n%s", events)
	}
}

func TestDeferCancelledAtThePromptWritesNothing(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)
	before := readHookEvents(t, festDir)

	allowOperatorGuard(t)
	stubOperatorPrompt(t, "99\n")
	deferReason = "the vendor replies next week"
	t.Cleanup(func() { deferReason = "" })

	var err error
	captureIO(t, func() {
		err = runDefer(taskCmd(t, festDir), []string{taskRel})
	})
	if err == nil {
		t.Fatal("a wrong task number must cancel the deferral")
	}
	if !strings.Contains(err.Error(), "deferral cancelled") {
		t.Errorf("error = %v, want the cancellation", err)
	}

	if after := readHookEvents(t, festDir); after != before {
		t.Errorf("a cancelled deferral appended an event:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
