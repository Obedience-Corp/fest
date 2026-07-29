package types

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

func TestGetBuiltInTemplatesDirResolutionOrder(t *testing.T) {
	// Isolate config fallback from the real user install tree.
	fakeConfig := t.TempDir()
	t.Setenv("FEST_CONFIG_DIR", fakeConfig)

	configFallback := filepath.Join(config.ConfigDir(), "festivals", ".festival", "templates")

	// Campaign tree: <root>/festivals/.festival/templates (FindFestivals needs .festival/).
	campaignRoot := t.TempDir()
	campaignTemplates := filepath.Join(campaignRoot, "festivals", ".festival", "templates")
	if err := os.MkdirAll(campaignTemplates, 0o755); err != nil {
		t.Fatal(err)
	}

	// festivals/.festival exists (FindFestivals succeeds) but templates/ is absent.
	campaignNoTemplates := t.TempDir()
	if err := os.MkdirAll(filepath.Join(campaignNoTemplates, "festivals", ".festival"), 0o755); err != nil {
		t.Fatal(err)
	}

	overrideDir := t.TempDir()
	outside := t.TempDir() // cwd outside any campaign

	tests := []struct {
		name    string
		envDir  string // FEST_TEMPLATES_DIR; empty means unset
		chdir   string
		wantDir string
	}{
		{
			name:    "FEST_TEMPLATES_DIR override wins over campaign and config",
			envDir:  overrideDir,
			chdir:   campaignRoot,
			wantDir: overrideDir,
		},
		{
			name:    "campaign festivals/.festival/templates when present",
			envDir:  "",
			chdir:   campaignRoot,
			wantDir: campaignTemplates,
		},
		{
			name:    "missing campaign templates falls through to config.ConfigDir",
			envDir:  "",
			chdir:   outside,
			wantDir: configFallback,
		},
		{
			name:    "campaign festivals without templates dir falls through",
			envDir:  "",
			chdir:   campaignNoTemplates,
			wantDir: configFallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envDir != "" {
				t.Setenv("FEST_TEMPLATES_DIR", tt.envDir)
			} else {
				// Ensure parent env cannot leak an override into "unset" cases.
				t.Setenv("FEST_TEMPLATES_DIR", "")
			}

			t.Chdir(tt.chdir)

			got := getBuiltInTemplatesDir()
			if got != tt.wantDir {
				t.Fatalf("getBuiltInTemplatesDir() = %q, want %q", got, tt.wantDir)
			}
		})
	}
}

func TestTemplatesDirExists(t *testing.T) {
	tmp := t.TempDir()
	if !templatesDirExists(tmp) {
		t.Fatalf("expected existing dir to report true")
	}
	if templatesDirExists(filepath.Join(tmp, "missing")) {
		t.Fatalf("expected missing dir to report false")
	}
	if templatesDirExists("") {
		t.Fatalf("expected empty path to report false")
	}
}
