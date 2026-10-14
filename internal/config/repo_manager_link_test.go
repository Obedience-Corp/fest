//go:build container_fs

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/dirlink"
)

func TestRepoManagerConfigLinkLifecycle(t *testing.T) {
	configRoot := t.TempDir()
	sourceRepo := filepath.Join(t.TempDir(), "my-config")
	if err := os.MkdirAll(filepath.Join(sourceRepo, "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(sourceRepo, "user", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FEST_CONFIG_DIR", configRoot)
	ctx := context.Background()

	rm, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := rm.Add(ctx, "mine", sourceRepo)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(repo.LocalPath); err != nil {
		t.Fatalf("repo link missing: %v", err)
	}

	if err := rm.Use(ctx, "mine"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if rm.GetActiveName() != "mine" {
		t.Fatalf("active = %q, want mine", rm.GetActiveName())
	}

	readback := filepath.Join(ActiveConfigPath(), "user", "settings.json")
	if _, err := os.Stat(readback); err != nil {
		t.Fatalf("active readback: %v", err)
	}

	rm2, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rm2.GetActiveName() != "mine" {
		t.Fatalf("manifest readback active = %q, want mine", rm2.GetActiveName())
	}

	if err := rm2.Remove(ctx, "mine"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(sourceRepo); err != nil {
		t.Fatalf("external source repo should remain: %v", err)
	}
	if _, err := os.Stat(repo.LocalPath); !os.IsNotExist(err) {
		t.Fatalf("managed repo link should be removed, stat err=%v", err)
	}
}

func TestRepoManagerUseRecreatesMissingActiveLink(t *testing.T) {
	configRoot := t.TempDir()
	sourceRepo := filepath.Join(t.TempDir(), "cfg")
	if err := os.MkdirAll(sourceRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FEST_CONFIG_DIR", configRoot)
	ctx := context.Background()

	rm, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rm.Add(ctx, "cfg", sourceRepo); err != nil {
		t.Fatal(err)
	}
	if err := rm.Use(ctx, "cfg"); err != nil {
		t.Fatal(err)
	}

	active := ActiveConfigPath()
	if err := dirlink.Remove(active); err != nil {
		t.Fatal(err)
	}

	if err := rm.Use(ctx, "cfg"); err != nil {
		t.Fatalf("Use after manual unlink should recreate active link: %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active link should exist after Use: %v", err)
	}
}
