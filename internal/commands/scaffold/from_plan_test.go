package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

	if err := writeScaffoldFestYaml(context.Background(), festivalDir, campaignRoot, "DS0001", "planning", plan); err != nil {
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

func TestWriteScaffoldFestYaml_SetsInitialSizeBytes(t *testing.T) {
	campaignRoot := t.TempDir()
	festivalsRoot := filepath.Join(campaignRoot, "festivals", "planning")
	festivalDir := filepath.Join(festivalsRoot, "data-safety-DS0001")
	if err := os.MkdirAll(festivalDir, 0o755); err != nil {
		t.Fatalf("mkdir festival: %v", err)
	}

	goalContent := []byte("# Festival Goal\n\nSome scaffolded content.\n")
	if err := os.WriteFile(filepath.Join(festivalDir, "FESTIVAL_GOAL.md"), goalContent, 0o644); err != nil {
		t.Fatalf("writing scaffolded content: %v", err)
	}

	plan := &sc.ParsedPlan{FestivalName: "data-safety", Goal: "fix the thing"}

	if err := writeScaffoldFestYaml(context.Background(), festivalDir, campaignRoot, "DS0001", "planning", plan); err != nil {
		t.Fatalf("writeScaffoldFestYaml: %v", err)
	}

	cfg, err := config.LoadFestivalConfig(festivalDir, campaignRoot)
	if err != nil {
		t.Fatalf("LoadFestivalConfig: %v", err)
	}
	if cfg.Metadata.InitialSizeBytes != int64(len(goalContent)) {
		t.Fatalf("InitialSizeBytes = %d, want %d", cfg.Metadata.InitialSizeBytes, len(goalContent))
	}
}

func TestRunFromPlan_DryRun_ReportsFestYamlInFilesCreated(t *testing.T) {
	campaignRoot := t.TempDir()
	festivalsRoot := filepath.Join(campaignRoot, "festivals")
	if err := os.MkdirAll(filepath.Join(festivalsRoot, ".festival"), 0o755); err != nil {
		t.Fatalf("mkdir .festival marker: %v", err)
	}

	planPath := filepath.Join(campaignRoot, "STRUCTURE.md")
	planContent := "# Festival Structure\n\n" +
		"## Festival Goal\n" +
		"Dry run coverage for fest.yaml preview.\n\n" +
		"## Hierarchy\n\n" +
		"- **Festival:** dry-run-check-DR0001\n" +
		"  - **Phase 001:** IMPLEMENT (implementation)\n" +
		"    - Sequence 01: only_sequence\n" +
		"      - Task 01: only task\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("writing plan file: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(campaignRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	opts := &fromPlanOptions{
		PlanPath:   planPath,
		Name:       "dry-run-check",
		Dest:       "planning",
		DryRun:     true,
		JSONOutput: true,
	}

	out := captureStdout(t, func() error {
		return runFromPlan(context.Background(), opts)
	})

	var result fromPlanResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshalling result: %v\noutput: %s", err, out)
	}
	if !result.OK {
		t.Fatalf("result not OK: %+v", result)
	}

	found := false
	for _, f := range result.FilesCreated {
		if filepath.Base(f) == config.FestivalConfigFileName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("FilesCreated missing %s: %+v", config.FestivalConfigFileName, result.FilesCreated)
	}

	if _, statErr := os.Stat(filepath.Join(result.FestivalDir, config.FestivalConfigFileName)); statErr == nil {
		t.Fatalf("dry-run must not write fest.yaml, but it exists at %s", result.FestivalDir)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	t.Cleanup(func() { os.Stdout = old })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	os.Stdout = old
	if runErr != nil {
		t.Fatalf("fn() returned error: %v", runErr)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copying pipe output: %v", err)
	}
	return buf.String()
}
