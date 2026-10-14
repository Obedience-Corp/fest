//go:build container_fs

package dirlink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateRemoveReadback(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(source, "nested", "marker.txt")
	if err := os.WriteFile(marker, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(root, "link")
	if err := Create(linkPath, source); err != nil {
		t.Fatalf("Create: %v", err)
	}

	kind, err := Kind(linkPath)
	if err != nil {
		t.Fatalf("Kind: %v", err)
	}
	if kind == KindNone {
		t.Fatal("expected directory link kind")
	}

	gotTarget, err := ReadTarget(linkPath)
	if err != nil {
		t.Fatalf("ReadTarget: %v", err)
	}
	if gotTarget != source {
		t.Fatalf("ReadTarget = %q, want %q", gotTarget, source)
	}

	readback := filepath.Join(linkPath, "nested", "marker.txt")
	data, err := os.ReadFile(readback)
	if err != nil {
		t.Fatalf("read through link: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("readback = %q, want ok", data)
	}

	if err := Remove(linkPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source tree should remain: %v", err)
	}
}

func TestRemoveRejectsPlainDirectory(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Remove(plain); err == nil {
		t.Fatal("expected error removing plain directory")
	}
}

func TestReplaceRollbackOnFinalizeError(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	active := filepath.Join(root, "active")
	if err := Create(active, first); err != nil {
		t.Fatal(err)
	}

	err := Replace(active, second, func() error {
		return os.ErrPermission
	})
	if err == nil {
		t.Fatal("expected finalize error")
	}

	target, err := ReadTarget(active)
	if err != nil {
		t.Fatalf("ReadTarget after rollback: %v", err)
	}
	if target != first {
		t.Fatalf("active link target = %q, want %q after rollback", target, first)
	}
}

func TestReplaceConfigStyleReadback(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "repo-a", "user")
	repoB := filepath.Join(root, "repo-b", "user")
	for _, dir := range []string{repoA, repoB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	active := filepath.Join(root, "active")
	if err := Replace(active, filepath.Dir(repoA), func() error { return nil }); err != nil {
		t.Fatalf("first Replace: %v", err)
	}
	if err := Replace(active, filepath.Dir(repoB), func() error { return nil }); err != nil {
		t.Fatalf("switch Replace: %v", err)
	}

	readback := filepath.Join(active, "user", "settings.json")
	if _, err := os.Stat(readback); err != nil {
		t.Fatalf("readback through active link: %v", err)
	}
}
