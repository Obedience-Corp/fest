package status

import (
	"context"
	"os"
	"path/filepath"
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
