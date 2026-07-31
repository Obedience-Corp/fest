package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeEvidencePath_Errors(t *testing.T) {
	if _, err := NormalizeEvidencePath("/abs"); err == nil {
		t.Fatal("absolute should fail")
	}
	if _, err := NormalizeEvidencePath("../escape"); err == nil {
		t.Fatal("escape should fail")
	}
	if _, err := NormalizeEvidencePath(""); err == nil {
		t.Fatal("empty should fail")
	}
}

func TestWithinRoot_AndResolve(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "ok.txt")
	if err := os.WriteFile(inside, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outFile := filepath.Join(outside, "out.txt")
	if err := os.WriteFile(outFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outFile, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	ok, err := WithinRoot(root, "ok.txt")
	if err != nil || !ok {
		t.Fatalf("ok.txt: %v %v", ok, err)
	}
	ok, err = WithinRoot(root, "empty.txt")
	if err != nil || ok {
		t.Fatalf("empty should not count: %v %v", ok, err)
	}
	ok, err = WithinRoot(root, "link.txt")
	if err == nil && ok {
		t.Fatal("escaping symlink should not count as present")
	}

	got := ResolvePhaseRelative(root, []string{"ok.txt", "empty.txt", "missing.txt", "ok.txt", "/abs"})
	if len(got) != 1 || got[0] != "ok.txt" {
		t.Fatalf("got = %v", got)
	}
}
