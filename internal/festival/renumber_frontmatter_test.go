package festival

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFM(t *testing.T, path, id string, order int, extra string) {
	t.Helper()
	content := "---\nfest_type: task\nfest_id: " + id + "\nfest_order: " +
		string(rune('0'+order)) + "\n" + extra + "---\n\n# Body\n\ncontent stays\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRenumber_TaskRenameRefreshesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFM(t, filepath.Join(dir, "01_first.md"), "01_first.md", 1, "custom_key: keep-me\n")
	writeFM(t, filepath.Join(dir, "02_second.md"), "02_second.md", 2, "")

	ren := NewRenumberer(RenumberOptions{AutoApprove: true, Quiet: true})
	if err := ren.InsertTask(context.Background(), dir, 0, "zeroth"); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	shifted, err := os.ReadFile(filepath.Join(dir, "02_first.md"))
	if err != nil {
		t.Fatalf("shifted file missing: %v", err)
	}
	s := string(shifted)
	for _, want := range []string{"fest_id: 02_first.md", "fest_order: 2", "custom_key: keep-me", "content stays"} {
		if !strings.Contains(s, want) {
			t.Errorf("shifted frontmatter missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "fest_id: 01_first.md") {
		t.Error("stale fest_id survived the rename")
	}

	third, err := os.ReadFile(filepath.Join(dir, "03_second.md"))
	if err != nil {
		t.Fatalf("second shifted file missing: %v", err)
	}
	if !strings.Contains(string(third), "fest_order: 3") {
		t.Errorf("second shifted file kept stale order:\n%s", third)
	}
}

func TestRenumber_SequenceRenameRefreshesGoalAndChildren(t *testing.T) {
	phaseDir := t.TempDir()
	seqDir := filepath.Join(phaseDir, "01_alpha")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	goal := "---\nfest_type: sequence\nfest_id: 01_alpha\nfest_order: 1\nfest_parent: 003_IMPLEMENT\n---\n\n# Goal\n"
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(goal), 0644); err != nil {
		t.Fatal(err)
	}
	task := "---\nfest_type: task\nfest_id: 01_do.md\nfest_order: 1\nfest_parent: 01_alpha\n---\n\n# Task\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_do.md"), []byte(task), 0644); err != nil {
		t.Fatal(err)
	}

	ren := NewRenumberer(RenumberOptions{AutoApprove: true, Quiet: true})
	if err := ren.InsertSequence(context.Background(), phaseDir, 0, "zero"); err != nil {
		t.Fatalf("InsertSequence: %v", err)
	}

	movedGoal, err := os.ReadFile(filepath.Join(phaseDir, "02_alpha", "SEQUENCE_GOAL.md"))
	if err != nil {
		t.Fatalf("moved goal missing: %v", err)
	}
	g := string(movedGoal)
	for _, want := range []string{"fest_id: 02_alpha", "fest_order: 2", "fest_parent: 003_IMPLEMENT"} {
		if !strings.Contains(g, want) {
			t.Errorf("goal frontmatter missing %q:\n%s", want, g)
		}
	}

	movedTask, err := os.ReadFile(filepath.Join(phaseDir, "02_alpha", "01_do.md"))
	if err != nil {
		t.Fatalf("moved task missing: %v", err)
	}
	tk := string(movedTask)
	if !strings.Contains(tk, "fest_parent: 02_alpha") {
		t.Errorf("task fest_parent not refreshed to new sequence id:\n%s", tk)
	}
	if !strings.Contains(tk, "fest_id: 01_do.md") || !strings.Contains(tk, "fest_order: 1") {
		t.Errorf("task's own id/order should be untouched by a parent rename:\n%s", tk)
	}
}

func TestRewriteFrontmatterFields_NoFrontmatterUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01_plain.md")
	original := "# No frontmatter here\n\nfest_id: not-frontmatter\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := rewriteFrontmatterFields(path, map[string]string{"fest_id": "02_plain.md"}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file without frontmatter was modified:\n%s", got)
	}
}
