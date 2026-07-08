package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

func TestPickFestivalForShow_NoCampaignReturnsUnavailable(t *testing.T) {
	dir := t.TempDir()
	path, outcome, err := pickFestivalForShow(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != shared.FestivalPickUnavailable {
		t.Fatalf("outcome = %v, want unavailable", outcome)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}

func TestPickFestivalForShow_NoFestivalsMarkerReturnsUnavailable(t *testing.T) {
	dir := t.TempDir()
	festivals := filepath.Join(dir, "festivals")
	if err := os.MkdirAll(festivals, 0o755); err != nil {
		t.Fatal(err)
	}

	path, outcome, err := pickFestivalForShow(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != shared.FestivalPickUnavailable {
		t.Fatalf("outcome = %v, want unavailable", outcome)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}

func TestPickFestivalForShow_UsesBrowsePickerStatuses(t *testing.T) {
	if got, want := shared.BrowseFestivalPickerStatuses, []string{"active", "ready", "planning", "ritual"}; len(got) != len(want) {
		t.Fatalf("BrowseFestivalPickerStatuses length = %d, want %d", len(got), len(want))
	}
}

func TestPickFestivalForShow_EmptyFestivalsReturnsUnavailable(t *testing.T) {
	dir := t.TempDir()
	dotFestival := filepath.Join(dir, "festivals", ".festival")
	if err := os.MkdirAll(dotFestival, 0o755); err != nil {
		t.Fatal(err)
	}

	path, outcome, err := pickFestivalForShow(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != shared.FestivalPickUnavailable {
		t.Fatalf("outcome = %v, want unavailable", outcome)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}