package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFishScriptContent(t *testing.T) {
	out, err := Generate("fish")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"fest shell integration",
		"function fgo",
		"set -l dest",
		"fest go",
		"cd $dest",
		`-d "$dest"`,
		"return 1",
		"$status",
		"end",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fish script should contain %q", want)
		}
	}
}

func TestFishScriptSyntax(t *testing.T) {
	bin, lookErr := exec.LookPath("fish")
	if lookErr != nil {
		t.Skip("fish not installed")
	}
	out, err := Generate("fish")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "init.fish")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, runErr := exec.Command(bin, "-n", path).CombinedOutput(); runErr != nil {
		t.Fatalf("fish -n reported a syntax error: %v\n%s", runErr, b)
	}
}
