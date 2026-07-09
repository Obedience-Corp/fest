package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashZshScriptContent(t *testing.T) {
	out, err := Generate("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"fest shell integration",
		"fgo()",
		"local dest",
		"fest go",
		`cd "$dest"`,
		`-d "$dest"`,
		"return 1",
		"[[ ",
		" ]]",
		"$?",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("bash/zsh script should contain %q", want)
		}
	}
}

func TestBashZshScriptPromoteColorCompletion(t *testing.T) {
	out, err := Generate("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fest promote completions --color", "compadd -V promote"} {
		if !strings.Contains(out, want) {
			t.Errorf("bash/zsh script should contain %q for colorized promote completion", want)
		}
	}
}

func TestBashZshScriptShowPickerCompletion(t *testing.T) {
	out, err := Generate("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"fest show pick",
		"fest show completions --color",
		"compadd -V show",
		"compadd -U",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("bash/zsh script should contain %q for the fest show <tab> widget", want)
		}
	}
}

func TestBashZshScriptFlsStatusVocabulary(t *testing.T) {
	out, err := Generate("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"active ready planning ritual completed dungeon all",
		"'ready:",
		"'ritual:",
		"'all:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fls completion should advertise the refreshed status vocabulary %q", want)
		}
	}
}

func TestBashZshScriptSyntax(t *testing.T) {
	out, err := Generate("zsh")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "init.sh")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, sh := range []string{"bash", "zsh"} {
		t.Run(sh, func(t *testing.T) {
			bin, lookErr := exec.LookPath(sh)
			if lookErr != nil {
				t.Skipf("%s not installed", sh)
			}
			if b, runErr := exec.Command(bin, "-n", path).CombinedOutput(); runErr != nil {
				t.Fatalf("%s -n reported a syntax error: %v\n%s", sh, runErr, b)
			}
		})
	}
}
