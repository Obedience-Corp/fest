package status

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusPickerStatuses(t *testing.T) {
	want := []string{"active", "ready", "planning"}
	if len(statusPickerStatuses) != len(want) {
		t.Fatalf("statusPickerStatuses length = %d, want %d", len(statusPickerStatuses), len(want))
	}
	for i, s := range want {
		if statusPickerStatuses[i] != s {
			t.Errorf("statusPickerStatuses[%d] = %q, want %q", i, statusPickerStatuses[i], s)
		}
	}
}

func TestPickFestivalForDisplay_NoCampaign(t *testing.T) {
	dir := t.TempDir()
	path, err := pickFestivalForDisplay(t.Context(), dir)
	if err == nil {
		t.Fatalf("expected error when no campaign workspace, got path %q", path)
	}
}

func TestPickFestivalForDisplay_NoFestivalsMarker(t *testing.T) {
	dir := t.TempDir()
	festivals := filepath.Join(dir, "festivals")
	if err := os.MkdirAll(festivals, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := pickFestivalForDisplay(t.Context(), dir)
	if err == nil {
		t.Fatal("expected error when no festivals marker, got nil")
	}
}

func TestPickFestivalForDisplay_EmptyFestivals(t *testing.T) {
	dir := t.TempDir()
	festivals := filepath.Join(dir, "festivals")
	dotFestival := filepath.Join(festivals, ".festival")
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := pickFestivalForDisplay(t.Context(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for empty festivals dir, got %q", path)
	}
}
