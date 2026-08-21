package progress

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const (
	// ProgressLockFile is the sidecar flock used to serialize store writers.
	ProgressLockFile = "progress.lock"

	progressLockPoll = 25 * time.Millisecond
)

func (s *Store) lockPath() string {
	return filepath.Join(s.festivalPath, ProgressDir, ProgressLockFile)
}

// withExclusiveLock serializes this writer against other fest processes,
// then reloads disk state so the mutation is not applied to a snapshot
// taken at NewManager. Nested Save/SaveEvents calls do not take the lock
// (the caller already holds it). Lifecycle hooks that invoke fest progress
// mutations on the same festival would deadlock; they must not.
func (s *Store) withExclusiveLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	lockPath := s.lockPath()
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return errors.IO("creating progress directory", err).
			WithField("path", filepath.Dir(lockPath))
	}

	unlock, err := lockProgressFile(ctx, lockPath)
	if err != nil {
		return errors.IO("locking progress store", err).
			WithField("path", lockPath).
			WithHint("Another fest process may be writing this festival's progress; retry after it finishes")
	}
	defer unlock()

	if err := s.Load(ctx); err != nil {
		return errors.Wrap(err, "reloading progress store under lock")
	}
	s.pendingEvents = nil
	return fn()
}
