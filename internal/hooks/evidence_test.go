package hooks

import (
	"os"
	"path/filepath"
	"strings"
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

func TestBuildEvidenceFiles_CapAndTruncate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("aaaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("bbbbbbbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := BuildEvidenceFiles(root, []string{"a.txt", "b.txt"}, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Truncated || files[0].Content != "aaaa" {
		t.Fatalf("a = %+v", files[0])
	}
	if !files[1].Truncated || !strings.Contains(files[1].Content, "TRUNCATED") {
		t.Fatalf("b = %+v", files[1])
	}

	// exact budget
	files, err = BuildEvidenceFiles(root, []string{"a.txt"}, 4)
	if err != nil || len(files) != 1 || files[0].Truncated {
		t.Fatalf("exact: %v %+v", err, files)
	}
}
