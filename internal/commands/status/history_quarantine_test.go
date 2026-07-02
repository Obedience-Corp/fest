package status

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordStatusChange_QuarantinesCorruptHistory(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	festDir := filepath.Join(root, "planning", "corrupt-fest-CF0001")
	dotFest := filepath.Join(festDir, ".fest")
	if err := os.MkdirAll(dotFest, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	historyPath := filepath.Join(dotFest, "status_history.json")
	if err := os.WriteFile(historyPath, []byte("{ this is not valid json"), 0644); err != nil {
		t.Fatalf("setup corrupt history: %v", err)
	}

	if err := RecordStatusChange(context.Background(), festDir, "planning", "ready", ""); err != nil {
		t.Fatalf("recording should recover from a corrupt history, got: %v", err)
	}

	if _, err := os.Stat(historyPath + ".corrupt"); err != nil {
		t.Errorf("corrupt history was not quarantined: %v", err)
	}

	history, err := LoadStatusHistory(context.Background(), festDir)
	if err != nil {
		t.Fatalf("reloading history: %v", err)
	}
	if len(history) != 1 || history[0].ToStatus != "ready" {
		t.Fatalf("expected one fresh entry after quarantine, got %+v", history)
	}
}

func TestLoadStatusHistory_QuarantineDoesNotOverwriteEarlierCorrupt(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	festDir := filepath.Join(root, "planning", "corrupt-fest-CF0002")
	dotFest := filepath.Join(festDir, ".fest")
	if err := os.MkdirAll(dotFest, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	historyPath := filepath.Join(dotFest, "status_history.json")

	firstCorrupt := []byte("{ first corruption")
	if err := os.WriteFile(historyPath, firstCorrupt, 0644); err != nil {
		t.Fatalf("setup first corrupt history: %v", err)
	}
	if _, err := LoadStatusHistory(context.Background(), festDir); err != nil {
		t.Fatalf("first load should quarantine and return empty, got: %v", err)
	}

	secondCorrupt := []byte("{ second corruption")
	if err := os.WriteFile(historyPath, secondCorrupt, 0644); err != nil {
		t.Fatalf("setup second corrupt history: %v", err)
	}
	if _, err := LoadStatusHistory(context.Background(), festDir); err != nil {
		t.Fatalf("second load should quarantine and return empty, got: %v", err)
	}

	firstQuarantined, err := os.ReadFile(historyPath + ".corrupt")
	if err != nil {
		t.Fatalf("first quarantined copy missing: %v", err)
	}
	if string(firstQuarantined) != string(firstCorrupt) {
		t.Fatalf("first quarantined copy was overwritten: got %q want %q", firstQuarantined, firstCorrupt)
	}

	secondQuarantined, err := os.ReadFile(historyPath + ".corrupt.1")
	if err != nil {
		t.Fatalf("second quarantined copy missing: %v", err)
	}
	if string(secondQuarantined) != string(secondCorrupt) {
		t.Fatalf("second quarantined copy content mismatch: got %q want %q", secondQuarantined, secondCorrupt)
	}
}

func TestLoadStatusHistory_WarnsOnQuarantine(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	festDir := filepath.Join(root, "planning", "corrupt-fest-CF0003")
	dotFest := filepath.Join(festDir, ".fest")
	if err := os.MkdirAll(dotFest, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	historyPath := filepath.Join(dotFest, "status_history.json")
	if err := os.WriteFile(historyPath, []byte("{ not valid json"), 0644); err != nil {
		t.Fatalf("setup corrupt history: %v", err)
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stderr = w

	_, loadErr := LoadStatusHistory(context.Background(), festDir)

	_ = w.Close()
	os.Stderr = origStderr
	if loadErr != nil {
		t.Fatalf("load should quarantine and return empty, got: %v", loadErr)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}
	if !strings.Contains(string(out), historyPath+".corrupt") {
		t.Fatalf("expected stderr warning naming quarantine path, got: %q", out)
	}
}
