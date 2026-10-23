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

func TestEnsureWorkspaceConfig_DoesNotFollowDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.yaml")
	link := filepath.Join(dir, WorkspaceConfigFileName)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	created, err := EnsureWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if created {
		t.Fatal("ensure reported a file created over a dangling symlink")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("ensure followed the link and wrote its target: stat err=%v", err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the operator's symlink was replaced: err=%v", err)
	}
	if !WorkspaceConfigExists(root) {
		t.Fatal("WorkspaceConfigExists reported a dangling symlink as absent")
	}
}

func TestEnsureWorkspaceConfig_LeavesSymlinkedConfigAlone(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "shared-config.yaml")
	own := "version: \"1.0\"\nhooks:\n  definitions:\n    approval_judge:\n      command: ob judge\n"
	if err := os.WriteFile(target, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, WorkspaceConfigFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	created, err := EnsureWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if created {
		t.Fatal("ensure reported a file created over a symlinked config")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != own {
		t.Fatalf("symlink target was rewritten:\n%s", data)
	}
}
