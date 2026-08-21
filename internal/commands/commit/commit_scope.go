package commit

import (
	"context"
	stderrors "errors"
	"os/exec"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const indexLockRetryWait = 2 * time.Second

// hasStagedPathChanges reports whether the index at repoPath differs from HEAD
// under paths. The whole-index check is the wrong question for a festival-scoped
// root commit: unrelated already-staged campaign files must not count as "this
// festival has something to commit".
func hasStagedPathChanges(ctx context.Context, repoPath string, paths []string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errors.Wrap(err, "context cancelled")
	}
	if len(paths) == 0 {
		return false, nil
	}

	args := append([]string{"-C", repoPath, "diff", "--cached", "--quiet", "--"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if ctx.Err() != nil {
		return false, errors.Wrap(ctx.Err(), "context cancelled")
	}
	var exitErr *exec.ExitError
	if stderrors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, errors.Wrap(err, "checking staged festival-scoped changes")
}

// commitOnlyPaths records a commit that contains only paths, leaving unrelated
// staged content in the index. commitkit.Commit records the whole index, so a
// festival-scoped stage followed by that commit would sweep in concurrent
// campaign work that happened to already be staged.
func commitOnlyPaths(ctx context.Context, repoPath, message string, paths []string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if len(paths) == 0 {
		return errors.New("no festival-scoped paths to commit")
	}

	args := append([]string{"-C", repoPath, "commit", "--only", "-m", message, "--"}, paths...)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
		if attempt > 0 {
			timer := time.NewTimer(indexLockRetryWait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Wrap(ctx.Err(), "context cancelled")
			case <-timer.C:
			}
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = err
		text := strings.TrimSpace(string(output))
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "context cancelled")
		}
		if attempt == 0 && strings.Contains(text, "index.lock") {
			continue
		}
		return errors.Wrapf(err, "committing festival files at campaign root: %s", text)
	}
	return errors.Wrap(lastErr, "committing festival files at campaign root")
}
