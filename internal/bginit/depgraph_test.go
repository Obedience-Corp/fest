package bginit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenDeps are packages whose init runs a terminal query (bubbletea via
// tea_init.go) or transitively links one (huh, glamour). If any enters
// bginit's import closure, bginit's inittask can no longer be guaranteed to
// run before the query fires, silently reverting the startup-stall fix.
var forbiddenDeps = []string{
	"github.com/charmbracelet/bubbletea",
	"github.com/charmbracelet/huh",
	"github.com/charmbracelet/glamour",
}

func TestDependencyClosureExcludesBubbletea(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	cmd := exec.Command("go", "list", "-deps", "./internal/bginit")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./internal/bginit: %v\n%s", err, out)
	}

	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, forbidden := range forbiddenDeps {
			if dep == forbidden || strings.HasPrefix(dep, forbidden+"/") {
				t.Errorf("bginit import closure includes %q; it must never link %q (breaks init ordering)", dep, forbidden)
			}
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test working directory")
		}
		dir = parent
	}
}
