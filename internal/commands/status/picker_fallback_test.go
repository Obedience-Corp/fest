package status

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

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

func TestPickFestivalForDisplay_UsesWorkingPickerStatuses(t *testing.T) {
	if got, want := shared.WorkingFestivalPickerStatuses, []string{"active", "ready", "planning"}; len(got) != len(want) {
		t.Fatalf("WorkingFestivalPickerStatuses length = %d, want %d", len(got), len(want))
	}
}
