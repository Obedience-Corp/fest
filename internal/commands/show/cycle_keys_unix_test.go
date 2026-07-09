//go:build unix

package show

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

func TestReadCycleAction_RightArrow(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x1b, '[', 'C'}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if action, _ := readCycleAction(context.Background(), r, nil); action != CycleNext {
		t.Errorf("readCycleAction = %v, want CycleNext", action)
	}
}

func TestReadCycleAction_LeftArrow(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x1b, '[', 'D'}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if action, _ := readCycleAction(context.Background(), r, nil); action != CyclePrev {
		t.Errorf("readCycleAction = %v, want CyclePrev", action)
	}
}

func TestReadCycleAction_CtrlC(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if _, err := w.Write([]byte{0x03}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if action, _ := readCycleAction(context.Background(), r, nil); action != CycleQuit {
		t.Errorf("readCycleAction = %v, want CycleQuit", action)
	}
}

func TestReadCycleAction_ExtraKeyDispatch(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	extra := map[byte]ExtraKeyHandler{'q': noopExtraKey}
	if _, err := w.Write([]byte{'q'}); err != nil {
		t.Fatalf("write: %v", err)
	}
	action, key := readCycleAction(context.Background(), r, extra)
	if action != CycleExtra || key != 'q' {
		t.Errorf("readCycleAction = (%v, %q), want (CycleExtra, 'q')", action, key)
	}
}

func TestReadCycleAction_EOFReturnsQuit(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()

	_ = w.Close()
	if action, _ := readCycleAction(context.Background(), r, nil); action != CycleQuit {
		t.Errorf("readCycleAction on EOF = %v, want CycleQuit", action)
	}
}

func TestReadCycleAction_ContextAlreadyCancelled(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if action, _ := readCycleAction(ctx, r, nil); action != CycleQuit {
		t.Errorf("readCycleAction on cancelled ctx = %v, want CycleQuit", action)
	}
}

func TestReadCycleAction_CancelReturnsQuitNoLeak(t *testing.T) {
	r, w := newCyclePipe(t)
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	time.Sleep(20 * time.Millisecond)
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan CycleAction, 1)
	go func() {
		action, _ := readCycleAction(ctx, r, nil)
		done <- action
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case action := <-done:
		if action != CycleQuit {
			t.Errorf("readCycleAction on cancel = %v, want CycleQuit", action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readCycleAction did not return after cancellation; reader is blocked")
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
