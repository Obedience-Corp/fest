//go:build windows && container_fs

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoManagerRejectsCaseOnlyNameCollision(t *testing.T) {
	configRoot := t.TempDir()
	sources := t.TempDir()
	first := filepath.Join(sources, "first")
	second := filepath.Join(sources, "second")
	for _, source := range []string{first, second} {
		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(first, "keep.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FEST_CONFIG_DIR", configRoot)
	ctx := context.Background()
	rm, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstRepo, err := rm.Add(ctx, "Config", first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rm.Add(ctx, "config", second); err == nil {
		t.Fatal("case-only name must not replace an occupied Windows directory link")
	}
	if len(rm.List()) != 1 || rm.List()[0].Name != firstRepo.Name {
		t.Fatalf("case-only add changed in-memory manifest: %+v", rm.List())
	}
	fresh, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.List()) != 1 || fresh.List()[0].Name != firstRepo.Name {
		t.Fatalf("case-only add changed persisted manifest: %+v", fresh.List())
	}
	if data, err := os.ReadFile(filepath.Join(firstRepo.LocalPath, "keep.txt")); err != nil || string(data) != "original" {
		t.Fatalf("original directory link was replaced: %q, %v", data, err)
	}
	if err := fresh.Remove(ctx, firstRepo.Name); err != nil {
		t.Fatalf("remove original after collision: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(first, "keep.txt")); err != nil || string(data) != "original" {
		t.Fatalf("external source changed after unlink: %q, %v", data, err)
	}
}
