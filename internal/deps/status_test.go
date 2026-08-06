package deps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// writeFestival lays out a minimal one-sequence festival and returns its root.
func writeFestival(t *testing.T, tasks map[string]string) string {
	t.Helper()

	root := t.TempDir()
	seq := filepath.Join(root, "001_IMPLEMENT", "01_work")
	if err := os.MkdirAll(seq, 0o755); err != nil {
		t.Fatalf("creating sequence dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "fest.yaml"), []byte("name: t\n"), 0o644); err != nil {
		t.Fatalf("writing fest.yaml: %v", err)
	}

	for name, body := range tasks {
		if err := os.WriteFile(filepath.Join(seq, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing task %s: %v", name, err)
		}
	}

	return root
}

// recordCompleted appends completion events in the shape fest task completed writes.
func recordCompleted(t *testing.T, root string, taskKeys ...string) {
	t.Helper()

	dir := filepath.Join(root, progress.ProgressDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating progress dir: %v", err)
	}

	var body strings.Builder
	for _, key := range taskKeys {
		body.WriteString(`{"ts":"2026-08-05T00:00:00Z","event":"completed","task":"` + key + `"}` + "\n")
	}

	path := filepath.Join(dir, progress.ProgressEventsFile)
	if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
		t.Fatalf("writing progress events: %v", err)
	}
}

func readySet(t *testing.T, root string) map[string]bool {
	t.Helper()

	graph, err := NewResolver(root).ResolveFestival()
	if err != nil {
		t.Fatalf("resolving festival: %v", err)
	}
	if err := ApplyProgress(context.Background(), graph, root); err != nil {
		t.Fatalf("applying progress: %v", err)
	}

	ready := make(map[string]bool)
	for _, task := range graph.GetReadyTasks() {
		rel, err := filepath.Rel(root, task.Path)
		if err != nil {
			t.Fatalf("relativizing %s: %v", task.Path, err)
		}
		ready[filepath.ToSlash(rel)] = true
	}
	return ready
}

// A task with no checkboxes and no frontmatter is exactly the common case, and
// it is what the first version of this feature got wrong: checkbox-derived
// status reported every such task as pending forever, so completed work stayed
// in the ready set.
func TestApplyProgress_StoreCompletionRemovesTaskFromReadySet(t *testing.T) {
	root := writeFestival(t, map[string]string{
		"01_first.md":  "# First\n\nProse only. No checkboxes, no frontmatter.\n",
		"02_second.md": "# Second\n\nAlso prose.\n",
	})
	recordCompleted(t, root, "001_IMPLEMENT/01_work/01_first.md")

	ready := readySet(t, root)

	if ready["001_IMPLEMENT/01_work/01_first.md"] {
		t.Error("a task completed in the progress store must not be reported ready")
	}
	if !ready["001_IMPLEMENT/01_work/02_second.md"] {
		t.Error("the next task should become ready once its predecessor completes")
	}
}

func TestApplyProgress_FrontmatterStatusHonored(t *testing.T) {
	root := writeFestival(t, map[string]string{
		"01_first.md": "---\nfest_type: task\nfest_id: first\nfest_status: completed\n---\n\n# First\n\nNo checkboxes.\n",
	})

	ready := readySet(t, root)

	if ready["001_IMPLEMENT/01_work/01_first.md"] {
		t.Error("fest_status: completed must remove a task from the ready set")
	}
}

func TestApplyProgress_CheckboxFreeTaskStaysReady(t *testing.T) {
	root := writeFestival(t, map[string]string{
		"01_first.md": "# First\n\nGenuinely unstarted, and it has no checkboxes.\n",
	})

	ready := readySet(t, root)

	if !ready["001_IMPLEMENT/01_work/01_first.md"] {
		t.Error("a task with no checkboxes and no recorded progress is pending, so it must be ready")
	}
}

func TestApplyProgress_TickedCheckboxesCountWhenNoOtherSource(t *testing.T) {
	root := writeFestival(t, map[string]string{
		"01_first.md": "# First\n\n- [x] done\n- [x] also done\n",
	})

	ready := readySet(t, root)

	if ready["001_IMPLEMENT/01_work/01_first.md"] {
		t.Error("a document whose checkboxes are all ticked should not be reported ready")
	}
}

// A query must never convert legacy state or delete workflow_state.yaml out
// from under a running navigator, which is why ApplyProgress uses LoadReadOnly.
func TestApplyProgress_DoesNotMutateDisk(t *testing.T) {
	root := writeFestival(t, map[string]string{
		"01_first.md": "# First\n",
	})
	recordCompleted(t, root, "001_IMPLEMENT/01_work/01_first.md")

	statePath := filepath.Join(root, progress.ProgressDir, "workflow_state.yaml")
	if err := os.WriteFile(statePath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("writing workflow state: %v", err)
	}

	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading workflow state: %v", err)
	}

	readySet(t, root)

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("workflow_state.yaml was removed by a read-only query: %v", err)
	}
	if string(before) != string(after) {
		t.Error("workflow_state.yaml was rewritten by a read-only query")
	}
}
