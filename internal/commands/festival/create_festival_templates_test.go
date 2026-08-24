package festival

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func setupFestivalsRoot(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	for _, status := range []string{"planning", "active", "dungeon"} {
		if err := os.MkdirAll(filepath.Join(festivalsRoot, status), 0755); err != nil {
			t.Fatalf("create status dir: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Join(festivalsRoot, "dungeon", "completed"), 0755); err != nil {
		t.Fatalf("create dungeon/completed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(festivalsRoot, ".festival"), 0755); err != nil {
		t.Fatalf("create .festival: %v", err)
	}
	return festivalsRoot
}

func writeNamedCoreTemplates(t *testing.T, festivalsRoot string, names ...string) {
	t.Helper()
	dir := filepath.Join(festivalsRoot, ".festival", "templates", "festival")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create templates dir: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func planningEntries(t *testing.T, festivalsRoot string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(festivalsRoot, "planning"))
	if err != nil {
		t.Fatalf("read planning: %v", err)
	}
	return entries
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

// TestRenderFestivalTemplates_DoesNotWriteWhenCoreTemplateMissing is the
// half-written-scaffold regression for fest#139: a partial core template set
// used to write every present file, then return Template. Dest must stay empty.
func TestRenderFestivalTemplates_DoesNotWriteWhenCoreTemplateMissing(t *testing.T) {
	festivalsRoot := setupFestivalsRoot(t)
	writeNamedCoreTemplates(t, festivalsRoot, "OVERVIEW.md", "GOAL.md", "RULES.md")

	destDir := filepath.Join(festivalsRoot, "planning", "partial-PA0001")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	cfg := &createConfig{
		opts:     &CreateFestivalOptions{Name: "partial", JSONOutput: true},
		tmplRoot: filepath.Join(festivalsRoot, ".festival", "templates"),
		destDir:  destDir,
		tmplCtx:  tpl.NewContext(),
		display:  ui.New(true, false),
	}

	created, gates, err := renderFestivalTemplates(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when a core template is missing")
	}
	if !errors.Is(err, errors.ErrCodeTemplate) {
		t.Fatalf("want TEMPLATE error, got %v", err)
	}
	if !strings.Contains(err.Error(), "festival/TODO.md") {
		t.Fatalf("error should name missing template, got: %v", err)
	}
	if len(created) != 0 || len(gates) != 0 {
		t.Fatalf("must not report created files on missing-template error, created=%v gates=%v", created, gates)
	}

	entries, readErr := os.ReadDir(destDir)
	if readErr != nil {
		t.Fatalf("read dest: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("dest dir must stay empty after missing-template error; wrote %v", entryNames(entries))
	}
}

func TestRenderFestivalTemplates_DirectoryTemplateCountsAsMissing(t *testing.T) {
	festivalsRoot := setupFestivalsRoot(t)
	writeNamedCoreTemplates(t, festivalsRoot, "OVERVIEW.md", "GOAL.md", "RULES.md")
	todoDir := filepath.Join(festivalsRoot, ".festival", "templates", "festival", "TODO.md")
	if err := os.MkdirAll(todoDir, 0755); err != nil {
		t.Fatalf("mkdir TODO.md as directory: %v", err)
	}

	destDir := filepath.Join(festivalsRoot, "planning", "dir-tmpl-DT0001")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	cfg := &createConfig{
		opts:     &CreateFestivalOptions{Name: "dir-tmpl", JSONOutput: true},
		tmplRoot: filepath.Join(festivalsRoot, ".festival", "templates"),
		destDir:  destDir,
		tmplCtx:  tpl.NewContext(),
		display:  ui.New(true, false),
	}

	_, _, err := renderFestivalTemplates(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when a core template path is a directory")
	}
	if !strings.Contains(err.Error(), "festival/TODO.md") {
		t.Fatalf("error should name directory template, got: %v", err)
	}
	entries, readErr := os.ReadDir(destDir)
	if readErr != nil {
		t.Fatalf("read dest: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("dest dir must stay empty; wrote %v", entryNames(entries))
	}
}

// TestCreateFestival_MissingCoreTemplatesLeavesNoDestDir asserts that a PLAIN
// (non-agent) create failure does not leave planning/<slug>-<id>/ behind.
func TestCreateFestival_MissingCoreTemplatesLeavesNoDestDir(t *testing.T) {
	cases := []struct {
		name      string
		templates []string
		dirName   string
		missing   string
	}{
		{name: "all missing", missing: "festival/OVERVIEW.md"},
		{
			name:      "partial set",
			templates: []string{"OVERVIEW.md", "GOAL.md", "RULES.md"},
			missing:   "festival/TODO.md",
		},
		{
			name:      "template is directory",
			templates: []string{"OVERVIEW.md", "GOAL.md", "RULES.md"},
			dirName:   "TODO.md",
			missing:   "festival/TODO.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			festivalsRoot := setupFestivalsRoot(t)
			if len(tc.templates) > 0 {
				writeNamedCoreTemplates(t, festivalsRoot, tc.templates...)
			}
			if tc.dirName != "" {
				dirPath := filepath.Join(festivalsRoot, ".festival", "templates", "festival", tc.dirName)
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					t.Fatalf("mkdir %s as directory: %v", tc.dirName, err)
				}
			}

			chdir(t, festivalsRoot)

			err := RunCreateFestival(context.Background(), &CreateFestivalOptions{
				Name:        "no leftovers",
				Dest:        "planning",
				SkipMarkers: true,
			})
			if err == nil {
				t.Fatal("expected missing-template error")
			}
			if !strings.Contains(err.Error(), "missing required core festival templates") {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("error should name %s, got: %v", tc.missing, err)
			}
			if tc.name == "all missing" {
				for _, name := range []string{
					"festival/OVERVIEW.md",
					"festival/GOAL.md",
					"festival/RULES.md",
					"festival/TODO.md",
				} {
					if !strings.Contains(err.Error(), name) {
						t.Errorf("error should name %s, got: %v", name, err)
					}
				}
			}

			entries := planningEntries(t, festivalsRoot)
			if len(entries) != 0 {
				t.Fatalf("planning/ must be empty after plain create failure, found %v", entryNames(entries))
			}

			if tc.dirName != "" {
				dirPath := filepath.Join(festivalsRoot, ".festival", "templates", "festival", tc.dirName)
				if err := os.RemoveAll(dirPath); err != nil {
					t.Fatalf("remove directory template %s: %v", tc.dirName, err)
				}
			}
			writeMinimalCoreTemplates(t, festivalsRoot)
			if err := RunCreateFestival(context.Background(), &CreateFestivalOptions{
				Name:        "no leftovers",
				Dest:        "planning",
				SkipMarkers: true,
				JSONOutput:  true,
			}); err != nil {
				t.Fatalf("create after templates restored: %v", err)
			}
			entries = planningEntries(t, festivalsRoot)
			if len(entries) != 1 {
				t.Fatalf("expected 1 festival after recovery, got %d (%v)", len(entries), entryNames(entries))
			}
			if !strings.HasSuffix(entries[0].Name(), "0001") {
				t.Fatalf("failed missing-template create burned an ID; got %q, want suffix 0001", entries[0].Name())
			}
		})
	}
}

func TestCreateFestival_MissingCoreTemplatesJSONLeavesNoDestDir(t *testing.T) {
	festivalsRoot := setupFestivalsRoot(t)
	writeNamedCoreTemplates(t, festivalsRoot, "OVERVIEW.md", "GOAL.md", "RULES.md")
	chdir(t, festivalsRoot)

	err := RunCreateFestival(context.Background(), &CreateFestivalOptions{
		Name:        "json leftovers",
		Dest:        "planning",
		SkipMarkers: true,
		JSONOutput:  true,
	})
	if err != nil {
		t.Fatalf("non-agent --json should return nil after emitting the error payload, got %v", err)
	}
	entries := planningEntries(t, festivalsRoot)
	if len(entries) != 0 {
		t.Fatalf("planning/ must be empty after JSON create failure, found %v", entryNames(entries))
	}
}

func TestMissingCoreTemplates_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := missingCoreTemplates(ctx, t.TempDir())
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
