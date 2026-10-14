//go:build container_fs

package config

import (
	"context"
	"errors"
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

func TestRepoManagerRemoveSaveFailureRestoresActiveLinks(t *testing.T) {
	configRoot := t.TempDir()
	sourceRepo := filepath.Join(t.TempDir(), "source config")
	if err := os.Mkdir(sourceRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(sourceRepo, "keep.txt")
	if err := os.WriteFile(marker, []byte("external data"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FEST_CONFIG_DIR", configRoot)
	ctx := context.Background()

	rm, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := rm.Add(ctx, "my config", sourceRepo)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Use(ctx, repo.Name); err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(RepoManifestPath())
	if err != nil {
		t.Fatal(err)
	}

	rm.saveManifest = func(context.Context, *ConfigRepoManifest) error {
		return os.ErrPermission
	}
	if err := rm.Remove(ctx, repo.Name); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Remove error = %v, want save permission error", err)
	}

	manifestAfter, err := os.ReadFile(RepoManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestAfter) != string(manifestBefore) {
		t.Fatal("persisted manifest changed after failed save")
	}
	if rm.GetActiveName() != repo.Name || len(rm.List()) != 1 {
		t.Fatalf("in-memory manifest changed after failed save: active=%q repos=%d", rm.GetActiveName(), len(rm.List()))
	}
	for _, path := range []string{repo.LocalPath, ActiveConfigPath()} {
		kind, err := dirlink.Kind(path)
		if err != nil || kind == dirlink.KindNone {
			t.Fatalf("managed link %s was not restored: kind=%v err=%v", path, kind, err)
		}
		data, err := os.ReadFile(filepath.Join(path, "keep.txt"))
		if err != nil || string(data) != "external data" {
			t.Fatalf("readback through restored link %s: %q, %v", path, data, err)
		}
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "external data" {
		t.Fatalf("external source changed: %q, %v", data, err)
	}
	rm2, err := NewRepoManager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rm2.GetActiveName() != repo.Name || len(rm2.List()) != 1 {
		t.Fatalf("fresh manager lost active repo: active=%q repos=%d", rm2.GetActiveName(), len(rm2.List()))
	}
	for _, parent := range []string{ConfigDir(), ConfigReposPath()} {
		matches, err := filepath.Glob(filepath.Join(parent, ".fest-remove-*"))
		if err != nil || len(matches) != 0 {
			t.Fatalf("staging artifacts in %s: %v, %v", parent, matches, err)
		}
	}
}
