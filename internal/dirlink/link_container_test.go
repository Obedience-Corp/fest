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

func TestUnicodeAndSpaceLinkSafeRemoval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Config With Spaces 日本語")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(source, "keep.txt")
	if err := os.WriteFile(payload, []byte("external data"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "Active Config 日本語")
	if err := Create(link, source); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(link, "keep.txt")); err != nil || string(data) != "external data" {
		t.Fatalf("read through Unicode/space link: %q, %v", data, err)
	}
	if err := Remove(link); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(payload); err != nil || string(data) != "external data" {
		t.Fatalf("source changed after unlink: %q, %v", data, err)
	}
}

func TestBrokenAndCyclicLinkSafeRemoval(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "vanished")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken")
	if err := Create(broken, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := Remove(broken); err != nil {
		t.Fatalf("remove broken link: %v", err)
	}
	if _, err := os.Lstat(broken); !os.IsNotExist(err) {
		t.Fatalf("broken link still exists: %v", err)
	}

	cycle := filepath.Join(root, "cycle")
	if err := Create(cycle, cycle); err != nil {
		t.Fatal(err)
	}
	if err := Remove(cycle); err != nil {
		t.Fatalf("remove cyclic link: %v", err)
	}
	if _, err := os.Lstat(cycle); !os.IsNotExist(err) {
		t.Fatalf("cyclic link still exists: %v", err)
	}
}
