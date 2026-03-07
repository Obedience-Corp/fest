package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGendocs_RemovesStaleFiles(t *testing.T) {
	dir := t.TempDir()

	// Plant a fake stale command doc that shouldn't survive regeneration.
	staleFile := filepath.Join(dir, "fest_fakecmd.md")
	if err := os.WriteFile(staleFile, []byte("# stale"), 0644); err != nil {
		t.Fatal(err)
	}

	// Also plant a non-generated file that should be preserved.
	keepFile := filepath.Join(dir, "custom-notes.md")
	if err := os.WriteFile(keepFile, []byte("# keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run gendocs into the temp directory.
	gendocsOutput = dir
	gendocsFormat = "markdown"
	gendocsSingle = false

	if err := runGendocs(rootCmd, nil); err != nil {
		t.Fatalf("runGendocs: %v", err)
	}

	// The stale file must be gone.
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale file %s still exists after gendocs", filepath.Base(staleFile))
	}

	// The non-generated file must still exist.
	if _, err := os.Stat(keepFile); err != nil {
		t.Errorf("non-generated file %s was removed: %v", filepath.Base(keepFile), err)
	}

	// At least one real command doc should have been generated (fest.md at minimum).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var generated []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "fest") && strings.HasSuffix(e.Name(), ".md") {
			generated = append(generated, e.Name())
		}
	}
	if len(generated) == 0 {
		t.Error("expected at least one generated fest*.md file")
	}
}
