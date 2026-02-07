package loop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultFestivalLoopConfig(t *testing.T) {
	cfg := DefaultFestivalLoopConfig()
	if cfg.MaxTotalIterations != 10 {
		t.Errorf("expected default max_total_iterations=10, got %d", cfg.MaxTotalIterations)
	}
}

func TestLoadFestivalLoopConfig_NoFestYAML(t *testing.T) {
	cfg, err := LoadFestivalLoopConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTotalIterations != 10 {
		t.Errorf("expected default 10, got %d", cfg.MaxTotalIterations)
	}
}

func TestLoadFestivalLoopConfig_NoLoopSection(t *testing.T) {
	tmpDir := t.TempDir()

	festYAML := "version: \"1.0\"\nmetadata:\n  id: test\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFestivalLoopConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTotalIterations != 10 {
		t.Errorf("expected default 10, got %d", cfg.MaxTotalIterations)
	}
}

func TestLoadFestivalLoopConfig_CustomCap(t *testing.T) {
	tmpDir := t.TempDir()

	festYAML := "loop:\n  max_total_iterations: 5\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFestivalLoopConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTotalIterations != 5 {
		t.Errorf("expected 5, got %d", cfg.MaxTotalIterations)
	}
}

func TestLoadFestivalLoopConfig_ZeroCap_DefaultsToTen(t *testing.T) {
	tmpDir := t.TempDir()

	festYAML := "loop:\n  max_total_iterations: 0\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFestivalLoopConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTotalIterations != 10 {
		t.Errorf("expected default 10 for zero cap, got %d", cfg.MaxTotalIterations)
	}
}

func TestTotalIterations_NoEvents(t *testing.T) {
	total, err := TotalIterations(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0, got %d", total)
	}
}

func TestTotalIterations_MultiplePhases(t *testing.T) {
	festDir := t.TempDir()

	// Create two phases with loop state
	for _, phase := range []string{"001_PHASE", "002_PHASE"} {
		stateDir := filepath.Join(festDir, phase, ".fest")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Each phase has 2 retry events
		events := `{"type":"loop_check","step":1,"passed":false}
{"type":"loop_retry","step":1,"iteration":1}
{"type":"loop_check","step":1,"passed":false}
{"type":"loop_retry","step":1,"iteration":2}
`
		if err := os.WriteFile(filepath.Join(stateDir, "loop_state.jsonl"), []byte(events), 0644); err != nil {
			t.Fatal(err)
		}
	}

	total, err := TotalIterations(festDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 4 {
		t.Errorf("expected 4 total retries, got %d", total)
	}
}
