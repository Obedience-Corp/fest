package progress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

func TestMarkComplete_ReloadsStaleSnapshotBeforeMutating(t *testing.T) {
	ctx := context.Background()
	festDir, task1, task2 := setupPropagationFestival(t)

	mgr1, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager mgr1: %v", err)
	}
	mgr2, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager mgr2: %v", err)
	}

	if err := mgr1.MarkComplete(ctx, task1); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task1, err)
	}
	if err := mgr2.MarkComplete(ctx, task2); err != nil {
		t.Fatalf("MarkComplete(%s): %v", task2, err)
	}

	seqGoalPath := filepath.Join(festDir, "001_PHASE", "01_sequence", "SEQUENCE_GOAL.md")
	if got := readStatus(t, seqGoalPath); got != frontmatter.StatusCompleted {
		t.Errorf("SEQUENCE_GOAL.md status = %q, want %q (stale mgr2 must reload under lock)", got, frontmatter.StatusCompleted)
	}

	phaseGoalPath := filepath.Join(festDir, "001_PHASE", "PHASE_GOAL.md")
	if got := readStatus(t, phaseGoalPath); got != frontmatter.StatusCompleted {
		t.Errorf("PHASE_GOAL.md status = %q, want %q", got, frontmatter.StatusCompleted)
	}
}

func TestMarkComplete_ConcurrentSiblingTasksPropagate(t *testing.T) {
	ctx := context.Background()
	festDir, task1, task2 := setupPropagationFestival(t)

	mgr1, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager mgr1: %v", err)
	}
	mgr2, err := NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager mgr2: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- mgr1.MarkComplete(ctx, task1)
	}()
	go func() {
		defer wg.Done()
		errCh <- mgr2.MarkComplete(ctx, task2)
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("MarkComplete: %v", err)
		}
	}

	verify := NewStore(festDir)
	if err := verify.Load(ctx); err != nil {
		t.Fatalf("Load after concurrent complete: %v", err)
	}
	for _, id := range []string{task1, task2} {
		task, ok := verify.GetTask(id)
		if !ok {
			t.Errorf("task %s missing from store after concurrent complete", id)
			continue
		}
		if task.Status != StatusCompleted {
			t.Errorf("task %s status = %q, want %q", id, task.Status, StatusCompleted)
		}
	}

	seqGoalPath := filepath.Join(festDir, "001_PHASE", "01_sequence", "SEQUENCE_GOAL.md")
	if got := readStatus(t, seqGoalPath); got != frontmatter.StatusCompleted {
		t.Errorf("SEQUENCE_GOAL.md status = %q, want %q", got, frontmatter.StatusCompleted)
	}
}

func TestWithExclusiveLock_CreatesLockFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.withExclusiveLock(ctx, func() error { return nil }); err != nil {
		t.Fatalf("withExclusiveLock: %v", err)
	}

	if _, err := os.Stat(store.lockPath()); err != nil {
		t.Fatalf("lock file: %v", err)
	}
}

func TestWithExclusiveLock_ContextCancelledWhileWaiting(t *testing.T) {
	if !progressLockSupported {
		t.Skip("progress store lock is a no-op on this platform")
	}

	ctx := context.Background()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ProgressDir, ProgressLockFile)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	held, err := lockProgressFile(ctx, lockPath)
	if err != nil {
		t.Fatalf("holding lock: %v", err)
	}
	defer held()

	waitCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	store := NewStore(dir)
	err = store.withExclusiveLock(waitCtx, func() error {
		t.Error("callback must not run while the lock is held")
		return nil
	})
	if err == nil {
		t.Fatal("withExclusiveLock should fail when the wait context is cancelled")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("withExclusiveLock error = %v, want context deadline/cancel", err)
	}
}
