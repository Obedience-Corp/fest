package bundled

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/methodology"
)

// embeddedFiles lists every file in the embedded scaffold, relative to its root.
func embeddedFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(methodology.FS, methodology.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(methodology.Root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded FS: %v", err)
	}
	sort.Strings(out)
	return out
}

// The embed directive reads the working tree, not the commit, so anything
// sitting in methodology/festivals at build time is baked into the binary —
// including untracked and gitignored files. That makes the shipped scaffold
// depend on whose machine ran the build, and it can carry local tool output to
// every user who runs 'fest init'.
//
// This is not hypothetical: the repository gitignores CLAUDE.md, and a
// developer machine can accumulate generated CLAUDE.md files under the
// methodology tree containing local session logs. A release built there would
// embed them; one built from a clean checkout would not.
//
// The binary must ship exactly what the commit contains.
func TestEmbeddedScaffoldMatchesGitTrackedFiles(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "-z", "--", "methodology/festivals")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git unavailable or not a work tree: %v", err)
	}

	const prefix = "methodology/festivals/"
	var tracked []string
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		tracked = append(tracked, strings.TrimPrefix(entry, prefix))
	}
	if len(tracked) == 0 {
		t.Skip("no tracked methodology files reported; nothing to compare")
	}
	sort.Strings(tracked)

	isTracked := make(map[string]bool, len(tracked))
	for _, f := range tracked {
		isTracked[f] = true
	}

	embedded := embeddedFiles(t)
	isEmbedded := make(map[string]bool, len(embedded))
	for _, f := range embedded {
		isEmbedded[f] = true
	}

	for _, f := range embedded {
		if !isTracked[f] {
			t.Errorf("embedded but not tracked by git: %q\n"+
				"  It would ship in the binary while a clean-checkout build omits it.\n"+
				"  Delete it from methodology/festivals, or commit it if it belongs to the methodology.", f)
		}
	}
	for _, f := range tracked {
		if !isEmbedded[f] {
			t.Errorf("tracked by git but not embedded: %q\n"+
				"  'fest init' would produce an incomplete workspace offline.", f)
		}
	}
}
