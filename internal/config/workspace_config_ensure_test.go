package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EnsureWorkspaceConfig is what lets a workspace initialized before the
// config scaffold existed pick the file up from 'fest system update'. It must
// create the commented hooks placeholder once and never rewrite a file the
// operator owns.

func TestEnsureWorkspaceConfig_CreatesPlaceholderOnce(t *testing.T) {
	root := t.TempDir()

	created, err := EnsureWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if !created {
		t.Fatal("first ensure reported nothing created")
	}

	path := filepath.Join(root, DotFestivalDir, WorkspaceConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading scaffolded config: %v", err)
	}
	for _, want := range []string{"version:", "# hooks:", "#     approval_judge:", "judge-agent"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("scaffolded config lacks %q:\n%s", want, data)
		}
	}

	created, err = EnsureWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if created {
		t.Fatal("second ensure reported a file created although one existed")
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading config: %v", err)
	}
	if string(again) != string(data) {
		t.Fatal("second ensure changed the existing config")
	}
}

func TestEnsureWorkspaceConfig_LeavesOperatorFileAlone(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	own := "version: \"1.0\"\nhooks:\n  definitions:\n    approval_judge:\n      command: ob judge\n"
	path := filepath.Join(dir, WorkspaceConfigFileName)
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if created {
		t.Fatal("ensure reported a file created over an operator-owned config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != own {
		t.Fatalf("operator config was rewritten:\n%s", data)
	}
}
