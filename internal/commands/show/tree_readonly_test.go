package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

func writeLegacyTreeFestival(t *testing.T, legacyTask string) (festivalDir, legacyPath string) {
	t.Helper()
	festivalDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("name: tree-ro\nmetadata:\n  id: TR0001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	taskDir := filepath.Join(festivalDir, "001_PLAN", "01_seq")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	for _, name := range []string{"01_task.md", "02_task.md"} {
		if err := os.WriteFile(filepath.Join(taskDir, name), []byte("# Task\n"), 0o644); err != nil {
			t.Fatalf("write task: %v", err)
		}
	}
	festDir := filepath.Join(festivalDir, progress.ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}
	legacyPath = filepath.Join(festDir, progress.ProgressFileName)
	if err := os.WriteFile(legacyPath, []byte("festival: tree-ro\ntasks:\n"+legacyTask), 0o644); err != nil {
		t.Fatalf("write legacy progress: %v", err)
	}
	return festivalDir, legacyPath
}

func countCompletedLeaves(node *DisplayNode) int {
	if node == nil {
		return 0
	}
	n := 0
	if len(node.Children) == 0 && node.Status == "completed" {
		n++
	}
	for _, child := range node.Children {
		n += countCompletedLeaves(child)
	}
	return n
}

func TestBuildFestivalTree_DoesNotMigrateLegacyProgress(t *testing.T) {
	legacyTask := "  001_PLAN/01_seq/01_task.md:\n" +
		"    task_id: 001_PLAN/01_seq/01_task.md\n" +
		"    status: completed\n" +
		"    progress: 100\n"
	festivalDir, legacyPath := writeLegacyTreeFestival(t, legacyTask)
	before, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("read legacy progress: %v", err)
	}

	for run := 1; run <= 2; run++ {
		tree, err := BuildFestivalTree(context.Background(), festivalDir)
		if err != nil {
			t.Fatalf("BuildFestivalTree run %d: %v", run, err)
		}
		if got := countCompletedLeaves(tree); got != 1 {
			t.Errorf("run %d: completed tasks = %d, want 1 (a timestamp-less legacy completion must survive repeated reads)", run, got)
		}
	}

	after, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("legacy progress.yaml was removed by building the tree: %v", err)
	}
	if string(after) != string(before) {
		t.Error("legacy progress.yaml was rewritten by building the tree")
	}
	if _, err := os.Stat(filepath.Join(festivalDir, progress.ProgressDir, progress.ProgressEventsFile)); !os.IsNotExist(err) {
		t.Error("building the tree wrote progress_events.jsonl; it must not mutate disk")
	}
}
