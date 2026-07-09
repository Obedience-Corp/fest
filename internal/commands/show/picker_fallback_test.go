package show

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

func TestBrowseFestivalPickerStatuses_Order(t *testing.T) {
	want := []string{"active", "ready", "planning", "ritual"}
	got := shared.BrowseFestivalPickerStatuses
	if len(got) != len(want) {
		t.Fatalf("BrowseFestivalPickerStatuses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("BrowseFestivalPickerStatuses[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBrowseCycleTargets_EmptyFestivalsReturnsNone(t *testing.T) {
	festivalsDir := filepath.Join(t.TempDir(), "festivals")
	if err := os.MkdirAll(filepath.Join(festivalsDir, ".festival"), 0o755); err != nil {
		t.Fatal(err)
	}
	if paths := browseCycleTargets(festivalsDir); len(paths) != 0 {
		t.Fatalf("expected no cycle targets in empty workspace, got %v", paths)
	}
}

func TestBrowseCycleTargets_ReturnsBrowseableFestival(t *testing.T) {
	festivalsDir := filepath.Join(t.TempDir(), "festivals")
	festivalDir := filepath.Join(festivalsDir, "active", "launch-readiness-LR0001")
	if err := os.MkdirAll(festivalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := browseCycleTargets(festivalsDir)
	if len(paths) != 1 {
		t.Fatalf("expected one cycle target, got %v", paths)
	}
	if filepath.Base(paths[0]) != "launch-readiness-LR0001" {
		t.Fatalf("cycle target = %q, want the active festival directory", paths[0])
	}
}
