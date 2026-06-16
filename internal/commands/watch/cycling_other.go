//go:build !unix

package watch

import (
	"context"
	"os"
)

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
