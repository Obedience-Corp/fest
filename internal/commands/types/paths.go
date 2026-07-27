package types

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// getBuiltInTemplatesDir returns the methodology templates tree used by create/sync.
// Resolution order:
//  1. FEST_TEMPLATES_DIR (explicit override)
//  2. Campaign festivals/.festival/templates (when cwd is inside a campaign)
//  3. System-synced ~/.obey/fest/festivals/.festival/templates
func getBuiltInTemplatesDir() string {
	if dir := os.Getenv("FEST_TEMPLATES_DIR"); dir != "" {
		return dir
	}

	if cwd, err := os.Getwd(); err == nil {
		if festivalsRoot, err := workspace.FindFestivals(cwd); err == nil && festivalsRoot != "" {
			campaignTemplates := filepath.Join(festivalsRoot, ".festival", "templates")
			if info, err := os.Stat(campaignTemplates); err == nil && info.IsDir() {
				return campaignTemplates
			}
		}
	}

	return filepath.Join(config.ConfigDir(), "festivals", ".festival", "templates")
}

// getCustomTemplatesDir looks for festival-local custom templates.
func getCustomTemplatesDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return ""
	}

	return filepath.Join(festivalPath, ".festival", "templates")
}

// templatesDirExists reports whether the built-in templates path is present.
func templatesDirExists(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
