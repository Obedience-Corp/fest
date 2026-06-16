package progress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProgressPickerStatuses(t *testing.T) {
	want := []string{"active", "ready", "planning"}
	if len(progressPickerStatuses) != len(want) {
		t.Fatalf("progressPickerStatuses length = %d, want %d", len(progressPickerStatuses), len(want))
	}
	for i, s := range want {
		if progressPickerStatuses[i] != s {
			t.Errorf("progressPickerStatuses[%d] = %q, want %q", i, progressPickerStatuses[i], s)
		}
	}
}

func TestPickFestivalForProgress_NoCampaign(t *testing.T) {
	dir := t.TempDir()
	path, err := pickFestivalForProgress(t.Context(), dir)
	if err == nil {
		t.Fatalf("expected error when no campaign workspace, got path %q", path)
	}
}

func TestPickFestivalForProgress_EmptyFestivals(t *testing.T) {
	dir := t.TempDir()
	festivals := filepath.Join(dir, "festivals")
	dotFestival := filepath.Join(festivals, ".festival")
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := pickFestivalForProgress(t.Context(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for empty festivals dir, got %q", path)
	}
}
