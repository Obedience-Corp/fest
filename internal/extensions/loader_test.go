package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

func writeExtension(t *testing.T, dir, name string) {
	t.Helper()
	extDir := filepath.Join(dir, name)
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", extDir, err)
	}
	manifest := "name: " + name + "\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(extDir, ExtensionManifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestLoadAll_MarketplaceFlagOn(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", cfg)
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "1")
	writeExtension(t, MarketplaceExtensionsDir(), "demo")

	l := NewExtensionLoader()
	if err := l.LoadAll(""); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	ext := l.Get("demo")
	if ext == nil {
		t.Fatal("expected demo to load from the marketplace path")
	}
	if ext.Source != "marketplace" {
		t.Fatalf("source = %q, want marketplace", ext.Source)
	}
}

func TestLoadAll_MarketplaceDefaultOff(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", cfg)
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "")
	writeExtension(t, MarketplaceExtensionsDir(), "demo")

	l := NewExtensionLoader()
	if err := l.LoadAll(""); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if l.Get("demo") != nil {
		t.Fatal("expected the marketplace path to be ignored by default")
	}
}

func TestLoadAll_MarketplaceFlagOff(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", cfg)
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "0")
	writeExtension(t, MarketplaceExtensionsDir(), "demo")

	l := NewExtensionLoader()
	if err := l.LoadAll(""); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if l.Get("demo") != nil {
		t.Fatal("expected the marketplace path to be ignored when the flag is off")
	}
}

func TestLoadAll_UserOverridesMarketplace(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", cfg)
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "1")
	writeExtension(t, MarketplaceExtensionsDir(), "demo")

	userFest := filepath.Join(config.ActiveConfigPath(), "festivals", ".festival", ExtensionsDirName)
	writeExtension(t, userFest, "demo")

	l := NewExtensionLoader()
	if err := l.LoadAll(""); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	ext := l.Get("demo")
	if ext == nil || ext.Source != "user" {
		t.Fatalf("expected user override of marketplace, got %+v", ext)
	}
}

func TestLoadAll_MissingMarketplaceDirBenign(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", cfg)
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "1")

	l := NewExtensionLoader()
	if err := l.LoadAll(""); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(l.List()) != 0 {
		t.Fatalf("expected no extensions, got %d", len(l.List()))
	}
}
