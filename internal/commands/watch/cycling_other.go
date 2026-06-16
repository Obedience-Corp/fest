//go:build !unix

package watch

import (
	"context"
	"os"
)

// readCycleKey reads a cycle key on platforms without unix poll support. It
// reads in a goroutine and selects on ctx so cancellation returns promptly; the
// goroutine may remain blocked on its final read until the next keystroke. The
// leak-free poll path lives in cycling_unix.go; this shim exists only to keep
// non-unix cross-compilation building, as fest does not ship non-unix targets.
func readCycleKey(ctx context.Context, f *os.File) cycleDirection {
	keyCh := make(chan cycleDirection, 1)
	go func() {
		buf := make([]byte, 3)
		for {
			n, err := f.Read(buf)
			if err != nil || n == 0 {
				keyCh <- cycleQuit
				return
			}
			if dir, ok := classifyCycleKey(buf[:n]); ok {
				keyCh <- dir
				return
			}
		}
	}()
	select {
	case dir := <-keyCh:
		return dir
	case <-ctx.Done():
		return cycleQuit
	}
}
