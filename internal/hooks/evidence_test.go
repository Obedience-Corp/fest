package hooks

import (
	"errors"
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

func TestResolvePhaseRelativeWithInspector(t *testing.T) {
	present := func(files ...string) func(string, string) (bool, error) {
		ok := map[string]struct{}{}
		for _, f := range files {
			ok[f] = struct{}{}
		}
		return func(_, path string) (bool, error) {
			_, found := ok[path]
			return found, nil
		}
	}

	tests := []struct {
		name     string
		paths    []string
		inspect  func(phasePath, relativePath string) (bool, error)
		want     []string
		wantNil  bool
		wantCall []string
	}{
		{
			name:    "preserves order and drops missing",
			paths:   []string{"b.md", "a.md", "missing.md"},
			inspect: present("b.md", "a.md"),
			want:    []string{"b.md", "a.md"},
		},
		{
			name:    "dedupes after normalize",
			paths:   []string{"ok.md", "./ok.md", "ok.md"},
			inspect: present("ok.md"),
			want:    []string{"ok.md"},
		},
		{
			name:     "drops absolute empty and escaping before inspect",
			paths:    []string{"/abs", "", "../escape", "ok.md"},
			inspect:  present("ok.md"),
			want:     []string{"ok.md"},
			wantCall: []string{"ok.md"},
		},
		{
			name:  "drops inspector errors",
			paths: []string{"ok.md", "bad.md"},
			inspect: func(_, path string) (bool, error) {
				if path == "bad.md" {
					return false, errors.New("containment failed")
				}
				return true, nil
			},
			want: []string{"ok.md"},
		},
		{
			name:    "empty input returns nil and does not inspect",
			paths:   nil,
			inspect: present("anything.md"),
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called []string
			inspect := func(phasePath, relativePath string) (bool, error) {
				called = append(called, relativePath)
				return tc.inspect(phasePath, relativePath)
			}
			got := ResolvePhaseRelativeWithInspector("/phase", tc.paths, inspect)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("got = %v, want nil", got)
				}
				if len(called) != 0 {
					t.Fatalf("inspected %v, want no calls", called)
				}
				return
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got = %v, want %v", got, tc.want)
			}
			if tc.wantCall != nil && strings.Join(called, "|") != strings.Join(tc.wantCall, "|") {
				t.Fatalf("inspected %v, want %v", called, tc.wantCall)
			}
		})
	}
}
