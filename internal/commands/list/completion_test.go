package list

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/spf13/cobra"
)

func TestCompleteListStatus_OffersVocabularyWithDescriptions(t *testing.T) {
	got, directive := completeListStatus(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}

	values := make(map[string]string, len(got))
	for _, entry := range got {
		name, desc, hasDesc := strings.Cut(entry, "\t")
		if !hasDesc || desc == "" {
			t.Errorf("completion %q is missing a description", entry)
		}
		values[name] = desc
	}

	for _, want := range []string{
		"active", "ready", "planning", "parked", "ritual", "completed",
		"dungeon", "dungeon/completed", "dungeon/archived", "dungeon/someday", "all",
	} {
		if _, ok := values[want]; !ok {
			t.Errorf("completion vocabulary missing %q; got %v", want, got)
		}
	}
}

func TestCompleteListStatus_OnlyFirstPositional(t *testing.T) {
	got, _ := completeListStatus(nil, []string{"active"}, "")
	if len(got) != 0 {
		t.Errorf("no completions expected past the first positional, got %v", got)
	}
}

func TestCompleteListStatus_FiltersByPrefix(t *testing.T) {
	got, _ := completeListStatus(nil, nil, "dungeon/")
	for _, entry := range got {
		if !strings.HasPrefix(entry, "dungeon/") {
			t.Errorf("prefix %q leaked non-matching completion %q", "dungeon/", entry)
		}
	}
	if len(got) == 0 {
		t.Error("expected dungeon/* completions for prefix 'dungeon/'")
	}
}

// TestListAllAliasMatchesFlag verifies `fest list all` produces byte-identical
// output to `fest list --all` (acceptance criterion 2).
func TestListAllAliasMatchesFlag(t *testing.T) {
	root := makeListCampaign(t)
	restore := chdirList(t, root)
	defer restore()

	alias := runListCommand(t, "all", "--json")
	flag := runListCommand(t, "--all", "--json")
	if alias != flag {
		t.Fatalf("`fest list all` output differs from `fest list --all`:\nall:\n%s\n--all:\n%s", alias, flag)
	}
	if !strings.Contains(alias, "\"total\"") {
		t.Errorf("expected grouped --all JSON with a total field, got:\n%s", alias)
	}
}

func makeListCampaign(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "festivals", ".festival"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("active", "alpha-AA0001"),
		filepath.Join("planning", "beta-BB0002"),
		filepath.Join("dungeon", "completed", "2026-02-10", "gamma-GG0003"),
	} {
		dir := filepath.Join(root, "festivals", rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, show.FestivalGoalFile), []byte("# Goal\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func chdirList(t *testing.T, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}
}

func runListCommand(t *testing.T, args ...string) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	cmd := NewListCommand()
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, readErr := buf.ReadFrom(r); readErr != nil {
		t.Fatal(readErr)
	}
	if runErr != nil {
		t.Fatalf("fest list %v: %v\noutput:\n%s", args, runErr, buf.String())
	}
	return buf.String()
}
