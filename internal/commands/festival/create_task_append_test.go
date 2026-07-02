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

	got, err := resolveTaskInsertAfter(context.Background(), dir, -1)
	if err != nil {
		t.Fatalf("resolveTaskInsertAfter: %v", err)
	}
	if got != 3 {
		t.Fatalf("resolveTaskInsertAfter(-1) = %d, want 3 (append after last)", got)
	}
}

func TestResolveTaskInsertAfter_EmptyDirInsertsAtBeginning(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveTaskInsertAfter(context.Background(), dir, -1)
	if err != nil {
		t.Fatalf("resolveTaskInsertAfter: %v", err)
	}
	if got != 0 {
		t.Fatalf("resolveTaskInsertAfter(-1) on empty dir = %d, want 0", got)
	}
}

func TestResolveTaskInsertAfter_ParseFailureIsAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := resolveTaskInsertAfter(context.Background(), missing, -1); err == nil {
		t.Fatal("expected an error for an unreadable sequence dir, got prepend fallback")
	}
}

func TestResolveTaskInsertAfter_ExplicitValuesPassThrough(t *testing.T) {
	dir := t.TempDir()
	seedTasks(t, dir, "first", "second")

	for _, after := range []int{0, 1, 2, 7} {
		got, err := resolveTaskInsertAfter(context.Background(), dir, after)
		if err != nil {
			t.Fatalf("resolveTaskInsertAfter(%d): %v", after, err)
		}
		if got != after {
			t.Fatalf("resolveTaskInsertAfter(%d) = %d, want passthrough", after, got)
		}
	}
}

func TestNetRenames_CollapsesChains(t *testing.T) {
	tests := []struct {
		name  string
		steps [][2]string
		want  []string
	}{
		{"empty", nil, nil},
		{"single", [][2]string{{"01_a.md", "02_a.md"}}, []string{"01_a.md -> 02_a.md"}},
		{"chain collapses", [][2]string{{"01_a.md", "02_a.md"}, {"02_a.md", "03_a.md"}}, []string{"01_a.md -> 03_a.md"}},
		{"independent sorted by origin", [][2]string{{"02_b.md", "03_b.md"}, {"01_a.md", "02_a.md"}}, []string{"01_a.md -> 02_a.md", "02_b.md -> 03_b.md"}},
		{"batch shape", [][2]string{{"01_x.md", "02_x.md"}, {"02_x.md", "03_x.md"}, {"03_y.md", "04_y.md"}}, []string{"01_x.md -> 03_x.md", "03_y.md -> 04_y.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := netRenames(tt.steps)
			if len(got) != len(tt.want) {
				t.Fatalf("netRenames = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("netRenames = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCreateTask_BatchInsertReportsNetRenames(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	seedTasks(t, dir, "first", "second")

	var steps [][2]string
	after := 0
	for _, name := range []string{"new_a", "new_b"} {
		ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})
		if err := ren.InsertTask(ctx, dir, after, name); err != nil {
			t.Fatalf("InsertTask %s: %v", name, err)
		}
		writeTaskFile(t, dir, after+1, name)
		for _, ch := range ren.Changes() {
			if ch.Type == festival.ChangeRename {
				steps = append(steps, [2]string{filepath.Base(ch.OldPath), filepath.Base(ch.NewPath)})
			}
		}
		after++
	}

	got := netRenames(steps)
	want := []string{"01_first.md -> 03_first.md", "02_second.md -> 04_second.md"}
	if len(got) != len(want) {
		t.Fatalf("net renames = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("net renames = %v, want %v", got, want)
		}
	}
}

func TestCreateTask_DictationOrderYieldsSequentialNumbers(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	ren := festival.NewRenumberer(festival.RenumberOptions{AutoApprove: true, Quiet: true})

	for _, name := range []string{"alpha", "beta", "gamma"} {
		after, err := resolveTaskInsertAfter(ctx, dir, -1)
		if err != nil {
			t.Fatalf("resolveTaskInsertAfter: %v", err)
		}
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
