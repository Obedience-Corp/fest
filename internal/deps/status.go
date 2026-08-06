package deps

import (
	"context"
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// ApplyProgress overlays real task status onto a resolved graph.
//
// The resolver deliberately leaves every task "pending": it walks the
// filesystem and knows nothing about execution state. Readiness cannot be
// computed against that, so any caller that needs it must opt in here.
//
// Sources, in precedence order:
//
//  1. The progress store, which is what `fest task completed` writes and is
//     therefore authoritative.
//  2. Frontmatter `status:`, which Manager.SyncFrontmatterStatus writes when a
//     task file has parseable frontmatter, and which a human may hand-edit.
//  3. Markdown checkboxes, but only when the document actually has some.
//     ParseTaskStatus returns "pending" for a document with no checkboxes,
//     which is indistinguishable from a genuinely pending task, so it must
//     never be consulted before the two sources above.
//
// The store is opened with LoadReadOnly: a query must not convert legacy YAML
// or delete workflow_state.yaml out from under a running navigator.
func ApplyProgress(ctx context.Context, graph *Graph, festivalPath string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	if graph == nil {
		return nil
	}

	store := progress.NewStore(festivalPath)
	if err := store.LoadReadOnly(ctx); err != nil {
		return errors.Wrap(err, "loading progress state")
	}

	for _, task := range graph.Tasks {
		task.Status = resolveStatus(store, festivalPath, task.Path)
	}

	return nil
}

func resolveStatus(store *progress.Store, festivalPath, taskPath string) string {
	if status := progress.ResolveTaskStatus(store, festivalPath, taskPath); status != progress.StatusPending {
		return status
	}

	if status, ok := frontmatterStatus(taskPath); ok {
		return status
	}

	if hasCheckboxes(taskPath) {
		return progress.ParseTaskStatus(taskPath)
	}

	return progress.StatusPending
}

// frontmatterStatus reports a task's declared status, and whether the document
// declared one at all. A document with no frontmatter, or with an empty status,
// yields ok=false so the caller falls through rather than reading it as pending.
func frontmatterStatus(taskPath string) (string, bool) {
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return "", false
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil || fm.Status == "" {
		return "", false
	}

	return string(fm.Status), true
}

// hasCheckboxes reports whether a document contains any markdown checkbox, so
// checkbox-derived status is only trusted for documents that actually use them.
func hasCheckboxes(taskPath string) bool {
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return false
	}

	return progress.HasCheckboxes(content)
}
