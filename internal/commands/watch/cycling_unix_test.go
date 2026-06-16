//go:build unix

package watch

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func newCyclePipe(t *testing.T) (r, w *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	return r, w
}

func TestReadCycleKey_RightArrow(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x1b, '[', 'C'}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dir := readCycleKey(context.Background(), r); dir != cycleNext {
		t.Errorf("readCycleKey = %v, want cycleNext", dir)
	}
}

func TestReadCycleKey_LeftArrow(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x1b, '[', 'D'}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dir := readCycleKey(context.Background(), r); dir != cyclePrev {
		t.Errorf("readCycleKey = %v, want cyclePrev", dir)
	}
}

func TestReadCycleKey_CtrlC(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x03}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dir := readCycleKey(context.Background(), r); dir != cycleQuit {
		t.Errorf("readCycleKey = %v, want cycleQuit", dir)
	}
}

func TestReadCycleKey_EOFReturnsQuit(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()

	_ = w.Close() // closing the write end signals EOF/hangup on the read end
	if dir := readCycleKey(context.Background(), r); dir != cycleQuit {
		t.Errorf("readCycleKey on EOF = %v, want cycleQuit", dir)
	}
}

func TestReadCycleKey_ContextAlreadyCancelled(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if dir := readCycleKey(ctx, r); dir != cycleQuit {
		t.Errorf("readCycleKey on cancelled ctx = %v, want cycleQuit", dir)
	}
}

// TestReadCycleKey_CancelReturnsQuitNoLeak is the regression test for the
// goroutine leak: while blocked waiting for a key, a cancelled context must
// make readCycleKey return cycleQuit promptly AND leave no goroutine parked on
// the terminal. The earlier SetReadDeadline approach no-oped on macOS TTYs and
// fell back to a goroutine that stayed blocked on Read until the next keystroke.
func TestReadCycleKey_CancelReturnsQuitNoLeak(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	time.Sleep(20 * time.Millisecond)
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan cycleDirection, 1)
	go func() { done <- readCycleKey(ctx, r) }()

	time.Sleep(50 * time.Millisecond) // let readCycleKey enter poll
	cancel()

	select {
	case dir := <-done:
		if dir != cycleQuit {
			t.Errorf("readCycleKey on cancel = %v, want cycleQuit", dir)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readCycleKey did not return after cancellation; reader is blocked")
	}

	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutine leak: NumGoroutine=%d > baseline=%d after cancellation",
				runtime.NumGoroutine(), before)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
