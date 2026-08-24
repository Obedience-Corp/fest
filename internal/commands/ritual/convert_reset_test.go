package ritual

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResetMarkdownCompletion(t *testing.T) {
	input := `---
fest_type: task
fest_id: 01_audit
fest_status: completed
custom: keep
---

# Audit

- [x] Review access logs
- [X] Check firewall
* [x] Star item
  - [x] Indented
See [x] in prose
- [✅] Emoji done
- [🚧] Emoji wip
- [❌] Emoji blocked
- [ ] Already open
`

	got := string(resetMarkdownCompletion([]byte(input)))

	wants := []string{
		"fest_status: pending",
		"custom: keep",
		"- [ ] Review access logs",
		"- [ ] Check firewall",
		"* [ ] Star item",
		"  - [ ] Indented",
		"See [x] in prose",
		"- [ ] Emoji done",
		"- [ ] Emoji wip",
		"- [ ] Emoji blocked",
		"- [ ] Already open",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "fest_status: completed") {
		t.Errorf("completed status survived:\n%s", got)
	}
	if strings.Contains(got, "[x] Review") || strings.Contains(got, "[X] Check") {
		t.Errorf("checked boxes survived:\n%s", got)
	}
}

func TestResetMarkdownCompletion_FestivalStatus(t *testing.T) {
	input := "---\nfest_type: festival\nfest_status: completed\n---\n\n# Goal\n"
	got := string(resetMarkdownCompletion([]byte(input)))
	if !strings.Contains(got, "fest_status: planning") {
		t.Fatalf("festival status = %q, want planning", got)
	}
}

func TestResetProgressArtifacts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	festDir := filepath.Join(root, ".fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "progress_events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "status_history.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	wfDir := filepath.Join(root, ".workflow", "runs", "r1")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "progress_events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	taskDir := filepath.Join(root, "001_REVIEW")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	taskPath := filepath.Join(taskDir, "01_audit.md")
	task := `---
fest_type: task
fest_status: completed
---

- [x] Done item
`
	if err := os.WriteFile(taskPath, []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := resetProgressArtifacts(ctx, root); err != nil {
		t.Fatalf("resetProgressArtifacts: %v", err)
	}

	if _, err := os.Stat(festDir); !os.IsNotExist(err) {
		t.Fatalf(".fest still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".workflow")); !os.IsNotExist(err) {
		t.Fatalf(".workflow still present: %v", err)
	}

	got, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "fest_status: pending") {
		t.Errorf("task status not pending:\n%s", body)
	}
	if !strings.Contains(body, "- [ ] Done item") {
		t.Errorf("checkbox not reset:\n%s", body)
	}
}

func TestResetProgressArtifacts_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := resetProgressArtifacts(ctx, t.TempDir())
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
