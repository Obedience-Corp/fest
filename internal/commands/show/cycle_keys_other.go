//go:build !unix

package show

import (
	"context"
	"os"
)

func readCycleAction(ctx context.Context, f *os.File, extraKeys map[byte]ExtraKeyHandler) (CycleAction, byte) {
	type keyResult struct {
		action CycleAction
		key    byte
	}
	keyCh := make(chan keyResult, 1)
	go func() {
		buf := make([]byte, 3)
		for {
			n, err := f.Read(buf)
			if err != nil || n == 0 {
				keyCh <- keyResult{CycleQuit, 0}
				return
			}
			if action, key, ok := classifyCycleKey(buf[:n], extraKeys); ok {
				keyCh <- keyResult{action, key}
				return
			}
		}
	}()
	select {
	case r := <-keyCh:
		return r.action, r.key
	case <-ctx.Done():
		return CycleQuit, 0
	}
}
