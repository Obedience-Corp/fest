package status

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/task"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
)

const statusDeferredTask = "001_PHASE/01_seq/01_task.md"

func deferredStatusFestival(t *testing.T, deferTask bool) *show.FestivalInfo {
	t.Helper()
	ctx := t.Context()

	dir := t.TempDir()
	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"),
		[]byte("---\nfest_type: task\n---\n\n# Task\n"), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	mgr, err := progress.NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if deferTask {
		if err := mgr.ReportBlocker(ctx, statusDeferredTask, "provider API removed", nil); err != nil {
			t.Fatalf("ReportBlocker() error = %v", err)
		}
		if err := mgr.DeferBlocker(ctx, statusDeferredTask, "the vendor replies next week", progress.DeferralAudit{
			Actor: "operator", TTY: true,
		}); err != nil {
			t.Fatalf("DeferBlocker() error = %v", err)
		}
	} else if err := mgr.MarkComplete(ctx, statusDeferredTask); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	return &show.FestivalInfo{Name: filepath.Base(dir), Path: dir, Status: "active"}
}

func stubStatusGuard(t *testing.T, audit *task.OperatorAudit, err error) *int {
	t.Helper()
	calls := 0
	restore := operatorGuardFn
	operatorGuardFn = func(context.Context, string) (*task.OperatorAudit, error) {
		calls++
		return audit, err
	}
	t.Cleanup(func() { operatorGuardFn = restore })
	return &calls
}

func TestStatusSetCompletedRefusesWithDeferredBlockers(t *testing.T) {
	festival := deferredStatusFestival(t, true)
	calls := stubStatusGuard(t, nil, errors.Validation("the guard must not run without --force"))

	var halt bool
	output, err := captureStatusStdout(t, func() error {
		var deferErr error
		halt, deferErr = enforceDeferredBlockers(t.Context(), festival, "dungeon/completed", &statusOptions{})
		return deferErr
	})

	if err != nil {
		t.Fatalf("enforceDeferredBlockers() error = %v, want the printed refusal", err)
	}
	if !halt {
		t.Fatal("fest status set completed must be refused while a blocker is deferred")
	}
	if *calls != 0 {
		t.Error("the operator guard ran without --force")
	}
	for _, want := range []string{
		"1 deferred blockers are still open. fest next will revisit them.",
		"Promote anyway with --force to record them as dropped.",
		"--force drops every deferred blocker. There is no per-task drop yet.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("refusal missing %q:\n%s", want, output)
		}
	}
}

func TestStatusSetForceIsGuardedOnlyWhenDeferred(t *testing.T) {
	t.Run("deferred runs the guard", func(t *testing.T) {
		festival := deferredStatusFestival(t, true)
		calls := stubStatusGuard(t, &task.OperatorAudit{Actor: "operator", TTY: true}, nil)

		var halt bool
		_, err := captureStatusStdout(t, func() error {
			var deferErr error
			halt, deferErr = enforceDeferredBlockers(t.Context(), festival, "dungeon/completed", &statusOptions{force: true})
			return deferErr
		})
		if halt || err != nil {
			t.Fatalf("enforceDeferredBlockers() = halt %v, err %v", halt, err)
		}
		if *calls != 1 {
			t.Errorf("guard calls = %d, want 1", *calls)
		}
		if _, statErr := os.Stat(filepath.Join(festival.Path, shared.DroppedBlockerRecordFile)); statErr != nil {
			t.Errorf("the dropped blocker record was not written: %v", statErr)
		}
	})

	t.Run("nothing deferred never runs the guard", func(t *testing.T) {
		festival := deferredStatusFestival(t, false)
		restore := operatorGuardFn
		operatorGuardFn = func(context.Context, string) (*task.OperatorAudit, error) {
			t.Fatal("--force must not reach the operator guard when nothing is deferred")
			return nil, nil
		}
		t.Cleanup(func() { operatorGuardFn = restore })

		for _, target := range []string{"dungeon/completed", "ready", "active"} {
			halt, err := enforceDeferredBlockers(t.Context(), festival, target, &statusOptions{force: true})
			if halt || err != nil {
				t.Errorf("status set %q was disturbed: halt %v, err %v", target, halt, err)
			}
		}
	})
}

func TestStatusSetForceRefusedOffTTYWithDeferred(t *testing.T) {
	festival := deferredStatusFestival(t, true)
	stubStatusGuard(t, nil, errors.Validation("forced completion is an operator decision; run this from your terminal"))

	halt, err := enforceDeferredBlockers(t.Context(), festival, "dungeon/completed", &statusOptions{force: true})
	if !halt {
		t.Fatal("a guard refusal must stop the status change")
	}
	if err == nil || !strings.Contains(err.Error(), "run this from your terminal") {
		t.Fatalf("error = %v, want the guard refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(festival.Path, shared.DroppedBlockerRecordFile)); statErr == nil {
		t.Error("a guard refusal wrote a dropped blocker record")
	}
}
