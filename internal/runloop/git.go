package runloop

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// maybeCommit creates a git commit in workDir when it is a repo with changes.
// Returns the short SHA or empty if there was nothing to commit or no repo.
// Never runs git reset.
func maybeCommit(ctx context.Context, workDir, message string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !isGitRepo(ctx, workDir) {
		return "", nil
	}
	status, err := gitOutput(ctx, workDir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "", nil
	}
	if _, err := gitOutput(ctx, workDir, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := gitOutput(ctx, workDir, "commit", "-m", message); err != nil {
		return "", err
	}
	sha, err := gitOutput(ctx, workDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func isGitRepo(ctx context.Context, dir string) bool {
	_, err := gitOutput(ctx, dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
