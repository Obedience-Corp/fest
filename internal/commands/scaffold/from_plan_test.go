package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	sc "github.com/Obedience-Corp/fest/internal/scaffold"
)

func TestWriteScaffoldFestYaml_ProducesRunnableMarker(t *testing.T) {
	campaignRoot := t.TempDir()
	festivalsRoot := filepath.Join(campaignRoot, "festivals", "planning")
	festivalDir := filepath.Join(festivalsRoot, "data-safety-DS0001")
	if err := os.MkdirAll(festivalDir, 0o755); err != nil {
		t.Fatalf("mkdir festival: %v", err)
	}

	plan := &sc.ParsedPlan{FestivalName: "data-safety", Goal: "fix the thing"}

	if err := writeScaffoldFestYaml(festivalDir, campaignRoot, "DS0001", "planning", plan); err != nil {
		t.Fatalf("writeScaffoldFestYaml: %v", err)
	}

	festYaml := filepath.Join(festivalDir, config.FestivalConfigFileName)
	if _, err := os.Stat(festYaml); err != nil {
		t.Fatalf("fest.yaml not written: %v", err)
	}

	cfg, err := config.LoadFestivalConfig(festivalDir, campaignRoot)
	if err != nil {
		t.Fatalf("LoadFestivalConfig: %v", err)
	}
	if cfg.Metadata.ID != "DS0001" {
		t.Fatalf("metadata.ID = %q, want DS0001", cfg.Metadata.ID)
	}
	if cfg.Metadata.Name != "data-safety" {
		t.Fatalf("metadata.Name = %q, want data-safety", cfg.Metadata.Name)
	}
	if len(cfg.Metadata.StatusHistory) == 0 || cfg.Metadata.StatusHistory[0].Status != "planning" {
		t.Fatalf("status history missing the planning entry: %+v", cfg.Metadata.StatusHistory)
	}
}
