package festival

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/festival"
)

func writeTaskFile(t *testing.T, dir string, number int, name string) {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("%02d_%s.md", number, name))
	if err := os.WriteFile(path, []byte("# Task: "+name+"\n"), 0644); err != nil {
		t.Fatalf("writing task file %s: %v", path, err)
	}
}

func seedTasks(t *testing.T, dir string, names ...string) {
	t.Helper()
	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
	after := 0
	for _, name := range names {
		if err := ren.InsertTask(context.Background(), dir, after, name); err != nil {
			t.Fatalf("seeding task %s: %v", name, err)
		}
		writeTaskFile(t, dir, after+1, name)
		after++
	}
}

func TestResolveTaskInsertAfter_AppendsAtEnd(t *testing.T) {
	dir := t.TempDir()
	seedTasks(t, dir, "first", "second", "third")

	got := resolveTaskInsertAfter(context.Background(), dir, -1)
	if got != 3 {
		t.Fatalf("resolveTaskInsertAfter(-1) = %d, want 3 (append after last)", got)
	}
}

func TestResolveTaskInsertAfter_EmptyDirInsertsAtBeginning(t *testing.T) {
	dir := t.TempDir()
	if got := resolveTaskInsertAfter(context.Background(), dir, -1); got != 0 {
		t.Fatalf("resolveTaskInsertAfter(-1) on empty dir = %d, want 0", got)
	}
}

func TestResolveTaskInsertAfter_ExplicitValuesPassThrough(t *testing.T) {
	dir := t.TempDir()
	seedTasks(t, dir, "first", "second")

	for _, after := range []int{0, 1, 2, 7} {
		if got := resolveTaskInsertAfter(context.Background(), dir, after); got != after {
			t.Fatalf("resolveTaskInsertAfter(%d) = %d, want passthrough", after, got)
		}
	}
}

func TestCreateTask_DictationOrderYieldsSequentialNumbers(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})

	for _, name := range []string{"alpha", "beta", "gamma"} {
		after := resolveTaskInsertAfter(ctx, dir, -1)
		if err := ren.InsertTask(ctx, dir, after, name); err != nil {
			t.Fatalf("InsertTask %s: %v", name, err)
		}
		writeTaskFile(t, dir, after+1, name)
	}

	for _, want := range []string{"01_alpha.md", "02_beta.md", "03_gamma.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

func TestRenumberer_ChangesReportsRenames(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	seedTasks(t, dir, "first", "second")

	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
	if err := ren.InsertTask(ctx, dir, 0, "zeroth"); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}
	writeTaskFile(t, dir, 1, "zeroth")

	renames := 0
	for _, ch := range ren.Changes() {
		if ch.Type == festival.ChangeRename {
			renames++
		}
	}
	if renames != 2 {
		t.Fatalf("Changes() reported %d renames, want 2", renames)
	}

	for _, want := range []string{"01_zeroth.md", "02_first.md", "03_second.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}
