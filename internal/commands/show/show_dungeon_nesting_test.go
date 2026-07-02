package show

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func writeFestival(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FestivalGoalFile), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindFestivalByName_ResolvesNonDateDungeonBucket(t *testing.T) {
	festivals := t.TempDir()
	name := "obey-search-shared-module-OS0001"
	writeFestival(t, filepath.Join(festivals, "dungeon/someday/pre-2026-03-01", name))

	info, err := FindFestivalByName(context.Background(), festivals, name, "")
	if err != nil {
		t.Fatalf("expected festival to resolve under pre-2026-03-01 bucket, got: %v", err)
	}
	if info.Status != "dungeon/someday" {
		t.Fatalf("status = %q, want dungeon/someday", info.Status)
	}
	if info.StatusDate != "pre-2026-03-01" {
		t.Fatalf("bucket = %q, want pre-2026-03-01", info.StatusDate)
	}
}

func TestListFestivalsByStatus_IncludesNonDateDungeonBucket(t *testing.T) {
	festivals := t.TempDir()
	name := "nested-beta-NB0001"
	writeFestival(t, filepath.Join(festivals, "dungeon/someday/pre-2026-03-01", name))

	got, err := ListFestivalsByStatus(context.Background(), festivals, "dungeon/someday", "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range got {
		if f.Name == name {
			found = true
			if f.StatusDate != "pre-2026-03-01" {
				t.Fatalf("bucket = %q, want pre-2026-03-01", f.StatusDate)
			}
		}
	}
	if !found {
		t.Fatalf("festival %q not listed under dungeon/someday", name)
	}
}

func TestEmitShowErrorJSON_ReturnsAlreadyPrintedSentinel(t *testing.T) {
	err := emitShowErrorJSON("festival not found")
	if !errors.Is(err, festerrors.ErrAlreadyPrinted) {
		t.Fatalf("emitShowErrorJSON returned %v, want ErrAlreadyPrinted so the process exits non-zero", err)
	}
}
