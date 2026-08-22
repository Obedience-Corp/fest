//go:build unix

package itestenv

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These prove the flock contract against a real file, not a stub. They do not
// need a Docker daemon or a Colima VM: the kernel lock is the thing under
// test, and t.TempDir is enough.

const lockTestWait = 750 * time.Millisecond

func TestSuiteLockSerializesRuns(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "camp-itest.lock")
	var notices bytes.Buffer

	first, err := Acquire(context.Background(), path, LockOptions{
		Wait:  lockTestWait,
		Label: "first run",
	})
	if err != nil {
		t.Fatalf("Acquire() first = %v", err)
	}

	_, err = Acquire(context.Background(), path, LockOptions{
		Wait:  lockTestWait,
		Poll:  20 * time.Millisecond,
		Out:   &notices,
		Label: "second run",
	})
	if err == nil {
		t.Fatal("Acquire() succeeded while the lock was held")
	}
	pid := strconv.Itoa(os.Getpid())
	if !strings.Contains(err.Error(), pid) {
		t.Errorf("Acquire() error = %v, want it to name the holder pid %s", err, pid)
	}
	if notice := notices.String(); !strings.Contains(notice, "waiting for second run") {
		t.Errorf("waiting notice = %q, want a visible wait line", notice)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release() = %v", err)
	}

	second, err := Acquire(context.Background(), path, LockOptions{
		Wait:  lockTestWait,
		Label: "second run",
	})
	if err != nil {
		t.Fatalf("Acquire() after release = %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("Release() second = %v", err)
	}
}

func TestSuiteLockWaitHonoursCancellation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "camp-itest.lock")
	held, err := Acquire(context.Background(), path, LockOptions{Label: "holder"})
	if err != nil {
		t.Fatalf("Acquire() = %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := Acquire(ctx, path, LockOptions{
		Wait: time.Hour,
		Poll: 10 * time.Millisecond,
	}); err == nil {
		t.Fatal("Acquire() returned a lock that was held")
	}
	if waited := time.Since(start); waited > 10*time.Second {
		t.Errorf("Acquire() took %s to notice cancellation", waited)
	}
}

func TestSuiteLockStatusReflectsTheHolder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "camp-itest.lock")
	if held, description := LockStatus(path); held {
		t.Fatalf("LockStatus() on an untouched path = held (%s), want free", description)
	}

	lock, err := Acquire(context.Background(), path, LockOptions{Label: "a run"})
	if err != nil {
		t.Fatalf("Acquire() = %v", err)
	}
	held, description := LockStatus(path)
	if !held {
		t.Errorf("LockStatus() while held = %q, want held", description)
	}
	if pid := strconv.Itoa(os.Getpid()); !strings.Contains(description, pid) {
		t.Errorf("LockStatus() = %q, want it to name pid %s", description, pid)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() = %v", err)
	}
	if held, description := LockStatus(path); held {
		t.Fatalf("LockStatus() after release = %q, want free", description)
	}
}
