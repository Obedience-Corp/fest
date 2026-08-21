package itestenv

import (
	"os"
	"path/filepath"
	"testing"
)

func mustCreate(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%s): %v", path, err)
	}
}

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}
