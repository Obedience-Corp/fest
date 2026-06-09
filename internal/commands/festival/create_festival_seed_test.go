package festival

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/types"
)

func runCreateForSeed(t *testing.T, opts *CreateFestivalOptions) string {
	t.Helper()
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	setupFestivalTemplatesWithMarkers(t, festivalsDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(festivalsDir)

	if err := RunCreateFestival(context.Background(), opts); err != nil {
		t.Fatalf("RunCreateFestival failed: %v", err)
	}

	planningDir := filepath.Join(festivalsDir, "planning")
	entries, err := os.ReadDir(planningDir)
	if err != nil {
		t.Fatalf("failed to read planning dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 festival in planning/, got %d", len(entries))
	}
	return filepath.Join(planningDir, entries[0].Name())
}

func TestCreateFestivalSeed_StandardWritesIngestInputSpec(t *testing.T) {
	festivalDir := runCreateForSeed(t, &CreateFestivalOptions{
		Name:       "Seeded",
		Type:       "standard",
		Dest:       "planning",
		Seed:       "Initial context: build the widget.",
		JSONOutput: true,
	})

	seedPath := filepath.Join(festivalDir, "001_INGEST", "input_specs", "seed.md")
	content, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("expected seed file at %s: %v", seedPath, err)
	}
	if string(content) != "Initial context: build the widget." {
		t.Errorf("unexpected seed content: %q", string(content))
	}
}

func TestCreateFestivalSeed_SeedFileWritesIngestInputSpec(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "seed-input.md")
	if err := os.WriteFile(srcFile, []byte("from a file"), 0644); err != nil {
		t.Fatalf("write src seed: %v", err)
	}

	festivalDir := runCreateForSeed(t, &CreateFestivalOptions{
		Name:       "SeededFile",
		Type:       "standard",
		Dest:       "planning",
		SeedFile:   srcFile,
		JSONOutput: true,
	})

	content, err := os.ReadFile(filepath.Join(festivalDir, "001_INGEST", "input_specs", "seed.md"))
	if err != nil {
		t.Fatalf("expected seed file: %v", err)
	}
	if string(content) != "from a file" {
		t.Errorf("unexpected seed content: %q", string(content))
	}
}

func TestCreateFestivalSeed_NoSeedLeavesNoSeedFile(t *testing.T) {
	festivalDir := runCreateForSeed(t, &CreateFestivalOptions{
		Name:       "Plain",
		Type:       "standard",
		Dest:       "planning",
		JSONOutput: true,
	})

	if _, err := os.Stat(filepath.Join(festivalDir, "001_INGEST", "input_specs", "seed.md")); !os.IsNotExist(err) {
		t.Errorf("expected no seed file when --seed is absent, stat err = %v", err)
	}
}

func TestCreateFestivalSeed_CountedInInitialSizeBytes(t *testing.T) {
	// A large seed (9000 bytes) far exceeds the unseeded standard scaffold
	// (~1KB of .md), so initial_size_bytes can only reach len(seed) if the seed
	// is counted in the creation baseline. A single-festival lower bound keeps
	// this deterministic (no cross-creation byte jitter).
	seed := strings.Repeat("seed content line\n", 500)
	seededDir := runCreateForSeed(t, &CreateFestivalOptions{
		Name:       "SizeProbe",
		Type:       "standard",
		Dest:       "planning",
		Seed:       seed,
		JSONOutput: true,
	})
	seededCfg, err := config.LoadFestivalConfig(seededDir, "")
	if err != nil {
		t.Fatalf("load seeded config: %v", err)
	}

	if seededCfg.Metadata.InitialSizeBytes < int64(len(seed)) {
		t.Errorf("seed not counted in initial_size_bytes: got %d, want >= %d",
			seededCfg.Metadata.InitialSizeBytes, len(seed))
	}
}

func TestCreateFestivalSeed_ImplementationTypeRejected(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	setupFestivalTemplatesWithMarkers(t, festivalsDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(festivalsDir)

	err := RunCreateFestival(context.Background(), &CreateFestivalOptions{
		Name: "ImplSeed",
		Type: "implementation",
		Dest: "planning",
		Seed: "should be refused",
	})
	if err == nil {
		t.Fatal("expected error seeding a type with no ingest phase")
	}

	entries, _ := os.ReadDir(filepath.Join(festivalsDir, "planning"))
	if len(entries) != 0 {
		t.Errorf("fail-fast expected: no festival should be created, found %d", len(entries))
	}
}

func TestCreateFestivalSeed_MutualExclusionRejected(t *testing.T) {
	err := RunCreateFestival(context.Background(), &CreateFestivalOptions{
		Name:     "Both",
		Type:     "standard",
		Dest:     "planning",
		Seed:     "a",
		SeedFile: "/tmp/whatever",
	})
	if err == nil {
		t.Fatal("expected error when both --seed and --seed-file are set")
	}
}

func TestResolveSeedContent(t *testing.T) {
	if _, ok, _ := resolveSeedContent(&CreateFestivalOptions{}); ok {
		t.Error("expected ok=false when no seed flag is set")
	}

	content, ok, err := resolveSeedContent(&CreateFestivalOptions{Seed: "inline"})
	if err != nil || !ok || content != "inline" {
		t.Errorf("inline: got (%q, %v, %v)", content, ok, err)
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "s.md")
	_ = os.WriteFile(file, []byte("file content"), 0644)
	content, ok, err = resolveSeedContent(&CreateFestivalOptions{SeedFile: file})
	if err != nil || !ok || content != "file content" {
		t.Errorf("file: got (%q, %v, %v)", content, ok, err)
	}

	if _, _, err := resolveSeedContent(&CreateFestivalOptions{SeedFile: filepath.Join(dir, "missing.md")}); err == nil {
		t.Error("expected error for missing seed file")
	}
}

func TestIngestAutoPhaseID(t *testing.T) {
	if _, ok := ingestAutoPhaseID(nil); ok {
		t.Error("nil type should have no ingest phase")
	}

	standard := &types.FestivalType{
		Phases: []types.PhaseSpec{
			{Name: "INGEST", Type: "ingest", Auto: true},
			{Name: "PLAN", Type: "planning", Auto: true},
		},
	}
	if id, ok := ingestAutoPhaseID(standard); !ok || id != "001_INGEST" {
		t.Errorf("standard: got (%q, %v), want (001_INGEST, true)", id, ok)
	}

	impl := &types.FestivalType{
		Phases: []types.PhaseSpec{
			{Name: "IMPLEMENT", Type: "implementation", Auto: true},
		},
	}
	if _, ok := ingestAutoPhaseID(impl); ok {
		t.Error("implementation type should have no ingest phase")
	}

	nonAutoIngest := &types.FestivalType{
		Phases: []types.PhaseSpec{
			{Name: "INGEST", Type: "ingest", Auto: false},
		},
	}
	if _, ok := ingestAutoPhaseID(nonAutoIngest); ok {
		t.Error("non-auto ingest phase is not scaffolded at create time")
	}
}
