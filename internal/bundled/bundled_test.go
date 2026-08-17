package bundled

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Obedience-Corp/fest/methodology"
)

// sourceTree is the on-disk scaffold the embed directive reads at build time.
const sourceTree = "../../methodology/festivals"

// relFiles lists every file under root as slash-separated relative paths.
func relFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(out)
	return out
}

// The bundled scaffold must be the whole scaffold. go:embed silently skips
// dot-prefixed entries without the all: prefix, which would drop .festival —
// the directory holding every template and agent guide — and leave init
// producing a workspace that looks complete and is unusable.
//
// Comparing against the on-disk tree rather than a hardcoded list means adding
// a file to the methodology cannot quietly fail to ship.
func TestSeedMaterializesTheEntireScaffold(t *testing.T) {
	if _, err := os.Stat(sourceTree); err != nil {
		t.Skipf("methodology source tree not available: %v", err)
	}

	dest := t.TempDir()
	if err := Seed(context.Background(), dest); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	want := relFiles(t, sourceTree)
	got := relFiles(t, dest)

	if len(want) == 0 {
		t.Fatal("the methodology source tree is empty; the comparison would be vacuous")
	}
	if len(got) != len(want) {
		t.Errorf("seeded %d files, source tree has %d", len(got), len(want))
	}

	inGot := make(map[string]bool, len(got))
	for _, f := range got {
		inGot[f] = true
	}
	for _, f := range want {
		if !inGot[f] {
			t.Errorf("bundled scaffold is missing %q", f)
		}
	}
}

// The specific casualty of a missing all: prefix, named so a failure says why.
func TestSeedIncludesDotDirectories(t *testing.T) {
	dest := t.TempDir()
	if err := Seed(context.Background(), dest); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	for _, required := range []string{".festival", "active", "planning", "ready", "ritual"} {
		info, err := os.Stat(filepath.Join(dest, required))
		if err != nil {
			t.Errorf("bundled scaffold is missing %q: %v", required, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", required)
		}
	}
}

// Contents must survive the round trip, not just the names.
func TestSeedPreservesFileContents(t *testing.T) {
	dest := t.TempDir()
	if err := Seed(context.Background(), dest); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var checked int
	err := fs.WalkDir(methodology.FS, methodology.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || checked >= 5 {
			return walkErr
		}
		rel, relErr := filepath.Rel(methodology.Root, path)
		if relErr != nil {
			return relErr
		}
		want, readErr := methodology.FS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		got, readErr := os.ReadFile(filepath.Join(dest, rel))
		if readErr != nil {
			t.Errorf("read seeded %q: %v", rel, readErr)
			return nil
		}
		if string(got) != string(want) {
			t.Errorf("seeded %q does not match the embedded copy", rel)
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded FS: %v", err)
	}
	if checked == 0 {
		t.Fatal("no files compared; the embedded FS appears empty")
	}
}

// Seeding writes a few hundred files, so a cancelled context must stop it
// rather than run to completion on a machine the operator is trying to quit.
func TestSeedHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Seed(ctx, t.TempDir()); err == nil {
		t.Error("Seed with a cancelled context returned nil, want an error")
	}
}

func TestSeedRejectsEmptyTarget(t *testing.T) {
	if err := Seed(context.Background(), ""); err == nil {
		t.Error("Seed with no target directory returned nil, want an error")
	}
}
