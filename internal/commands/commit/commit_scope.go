package commit

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
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
// staged content in the index. Content is taken from the index, not the
// working tree: `git commit --only` re-reads the worktree at commit time, so
// writes during DrainJobs under the same pathspecs would replace the snapshot
// StageFilesWithOptions just staged and guard-checked.
func commitOnlyPaths(ctx context.Context, repoPath, message string, paths []string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if len(paths) == 0 {
		return errors.New("no festival-scoped paths to commit")
	}

	tmpIndex, cleanup, err := newTempIndex(ctx, repoPath)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := seedTempIndexFromHEAD(ctx, repoPath, tmpIndex); err != nil {
		return err
	}
	if err := applyCachedDiffToTempIndex(ctx, repoPath, tmpIndex, paths); err != nil {
		return err
	}
	if err := commitTempIndex(ctx, repoPath, tmpIndex, message); err != nil {
		return err
	}
	return resetIndexToHEAD(ctx, repoPath, paths)
}

func newTempIndex(ctx context.Context, repoPath string) (string, func(), error) {
	noop := func() {}
	if err := ctx.Err(); err != nil {
		return "", noop, errors.Wrap(err, "context cancelled")
	}
	gitDir, err := absoluteGitDir(ctx, repoPath)
	if err != nil {
		return "", noop, err
	}
	f, err := os.CreateTemp(gitDir, "index.tmp.*")
	if err != nil {
		return "", noop, errors.Wrap(err, "creating temporary git index")
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", noop, errors.Wrap(err, "closing temporary git index")
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func absoluteGitDir(ctx context.Context, repoPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--absolute-git-dir")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.Wrap(ctx.Err(), "context cancelled")
		}
		return "", errors.Wrap(err, "resolving git directory")
	}
	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", errors.New("resolving git directory: empty path")
	}
	return gitDir, nil
}

func seedTempIndexFromHEAD(ctx context.Context, repoPath, tmpIndex string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	tree := "HEAD"
	if !headResolvable(ctx, repoPath) {
		tree = "--empty"
	}
	text, err := gitWithIndex(ctx, repoPath, tmpIndex, "read-tree", tree)
	if err != nil {
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "context cancelled")
		}
		return errors.Wrapf(err, "seeding temporary git index from HEAD: %s", text)
	}
	return nil
}

func headResolvable(ctx context.Context, repoPath string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "HEAD")
	return cmd.Run() == nil
}

func applyCachedDiffToTempIndex(ctx context.Context, repoPath, tmpIndex string, paths []string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	args := append([]string{"-C", repoPath, "diff", "--cached", "--binary", "--"}, paths...)
	diffCmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	diffCmd.Stderr = &stderr
	patch, err := diffCmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "context cancelled")
		}
		return errors.Wrapf(err, "reading staged festival diff: %s", strings.TrimSpace(stderr.String()))
	}
	if len(patch) == 0 {
		return nil
	}

	applyCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "apply", "--cached", "--binary")
	applyCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	applyCmd.Stdin = bytes.NewReader(patch)
	out, err := applyCmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "context cancelled")
		}
		return errors.Wrapf(err, "applying staged festival diff to temporary index: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func commitTempIndex(ctx context.Context, repoPath, tmpIndex, message string) error {
	return withIndexLockRetry(ctx, "committing festival files at campaign root", func() (string, error) {
		return gitWithIndex(ctx, repoPath, tmpIndex, "commit", "-m", message)
	})
}

func resetIndexToHEAD(ctx context.Context, repoPath string, paths []string) error {
	args := append([]string{"-C", repoPath, "reset", "-q", "HEAD", "--"}, paths...)
	return withIndexLockRetry(ctx, "resetting committed festival paths in the index", func() (string, error) {
		return gitCombined(ctx, nil, args...)
	})
}

func withIndexLockRetry(ctx context.Context, wrapMsg string, op func() (string, error)) error {
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
		text, err := op()
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "context cancelled")
		}
		if attempt == 0 && strings.Contains(text, "index.lock") {
			continue
		}
		return errors.Wrapf(err, "%s: %s", wrapMsg, text)
	}
	return errors.Wrap(lastErr, wrapMsg)
}

func gitWithIndex(ctx context.Context, repoPath, tmpIndex string, gitArgs ...string) (string, error) {
	args := append([]string{"-C", repoPath}, gitArgs...)
	return gitCombined(ctx, append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex), args...)
}

func gitCombined(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
