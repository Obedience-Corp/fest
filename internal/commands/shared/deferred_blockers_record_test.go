package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

func droppedState(id string, sweep int) DeferredBlockerState {
	return DeferredBlockerState{
		Sweep: sweep,
		Tasks: []*progress.TaskProgress{{
			TaskID:         id,
			Status:         progress.StatusBlocked,
			BlockerMessage: "provider gone",
			DeferralReason: "revisit after launch",
		}},
	}
}

func TestWriteDroppedBlockerRecordCreatesTheFile(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteDroppedBlockerRecord(dir, droppedState("003/01/02_cache.md", 1))
	if err != nil {
		t.Fatalf("WriteDroppedBlockerRecord() error = %v", err)
	}
	if path != filepath.Join(dir, DroppedBlockerRecordFile) {
		t.Fatalf("path = %q", path)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Dropped blockers\n\n", "## Forced at ", "after sweep 1", "### 003/01/02_cache.md", "provider gone"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("record missing %q:\n%s", want, got)
		}
	}
}

func TestWriteDroppedBlockerRecordAppendsToAnExistingRecord(t *testing.T) {
	dir := t.TempDir()

	if _, err := WriteDroppedBlockerRecord(dir, droppedState("003/01/02_cache.md", 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDroppedBlockerRecord(dir, droppedState("004/02/01_sweep.md", 3)); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, DroppedBlockerRecordFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Count(text, "# Dropped blockers\n") != 1 {
		t.Errorf("title must appear once:\n%s", text)
	}
	if strings.Count(text, "## Forced at ") != 2 {
		t.Errorf("want two forced sections:\n%s", text)
	}
	if !strings.Contains(text, "### 003/01/02_cache.md") || !strings.Contains(text, "### 004/02/01_sweep.md") {
		t.Errorf("both forced completions must be recorded:\n%s", text)
	}
	if strings.Index(text, "after sweep 1") > strings.Index(text, "after sweep 3") {
		t.Errorf("records must keep chronological order:\n%s", text)
	}
}

func TestWriteDroppedBlockerRecordPreservesAUserFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DroppedBlockerRecordFile)
	user := "# My notes\n\nkeep this line"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteDroppedBlockerRecord(dir, droppedState("003/01/02_cache.md", 1)); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.HasPrefix(text, user+"\n\n## Forced at ") {
		t.Errorf("user content must be kept verbatim and separated by a blank line:\n%s", text)
	}
	if strings.Contains(text, "# Dropped blockers\n") {
		t.Errorf("an existing file must not receive the generated title:\n%s", text)
	}
}

func TestWriteDroppedBlockerRecordWritesNothingWhenNothingIsDeferred(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteDroppedBlockerRecord(dir, DeferredBlockerState{})
	if err != nil || path != "" {
		t.Fatalf("path, err = %q, %v; want no write", path, err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, DroppedBlockerRecordFile)); !os.IsNotExist(statErr) {
		t.Fatalf("record must not exist, stat err = %v", statErr)
	}
}
