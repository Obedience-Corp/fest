//go:build !unix && !windows

package progress

import "context"

const progressLockSupported = false

// lockProgressFile is a no-op on platforms without flock/LockFileEx.
// Writers fall back to last-writer-wins, matching navigation's other-OS lock.
func lockProgressFile(ctx context.Context, _ string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return func() {}, nil
}
