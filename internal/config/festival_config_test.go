package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultFestivalConfig(t *testing.T) {
	cfg := DefaultFestivalConfig()

	if cfg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", cfg.Version)
	}

	if !cfg.QualityGates.Enabled {
		t.Error("expected quality gates to be enabled by default")
	}

	// Check implementation gates
	expectedIDs := []string{"testing", "review", "iterate", "commit"}
	if len(cfg.QualityGates.Implementation) != 4 {
		t.Errorf("expected 4 implementation gates, got %d", len(cfg.QualityGates.Implementation))
	}
	for i, gate := range cfg.QualityGates.Implementation {
		if gate.ID != expectedIDs[i] {
			t.Errorf("expected implementation gate ID %s at index %d, got %s", expectedIDs[i], i, gate.ID)
		}
		if !gate.Enabled {
			t.Errorf("expected gate %s to be enabled", gate.ID)
		}
	}

}

func TestLoadFestivalConfig_Default(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Load from non-existent file should return defaults
	cfg, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("expected default version, got %s", cfg.Version)
	}
}

func TestLoadFestivalConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test config file
	configContent := `version: "2.0"
quality_gates:
  enabled: true
  auto_append: false
  tasks:
    - id: custom_test
      template: CUSTOM_TEMPLATE
      enabled: true
excluded_patterns:

- "*_docs"
`
	configPath := filepath.Join(tmpDir, FestivalConfigFileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Version != "2.0" {
		t.Errorf("expected version 2.0, got %s", cfg.Version)
	}

	if cfg.QualityGates.AutoAppend {
		t.Error("expected auto_append to be false")
	}

	if len(cfg.QualityGates.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(cfg.QualityGates.Tasks))
	}

	if cfg.QualityGates.Tasks[0].ID != "custom_test" {
		t.Errorf("expected task ID custom_test, got %s", cfg.QualityGates.Tasks[0].ID)
	}
}

func TestSaveFestivalConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &FestivalConfig{
		Version: "1.0",
		QualityGates: QualityGatesConfig{
			Enabled: true,
			Tasks: []QualityGateTask{
				{
					ID:       "test_task",
					Template: "TEST_TEMPLATE",
					Enabled:  true,
				},
			},
		},
	}

	if err := SaveFestivalConfig(tmpDir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, FestivalConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Load and verify
	loaded, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Version != cfg.Version {
		t.Errorf("version mismatch: expected %s, got %s", cfg.Version, loaded.Version)
	}
}

func TestIsSequenceExcluded(t *testing.T) {
	cfg := &FestivalConfig{
		ExcludedPatterns: []string{
			"*_planning",
			"*_requirements",
			"01_research",
		},
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{"01_planning", true},
		{"02_requirements", true},
		{"01_research", true},
		{"01_implementation", false},
		{"02_api", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.IsSequenceExcluded(tt.name)
			if result != tt.expected {
				t.Errorf("IsSequenceExcluded(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestGetEnabledTasks(t *testing.T) {
	cfg := &FestivalConfig{
		QualityGates: QualityGatesConfig{
			Tasks: []QualityGateTask{
				{ID: "task1", Enabled: true},
				{ID: "task2", Enabled: false},
				{ID: "task3", Enabled: true},
			},
		},
	}

	enabled := cfg.GetEnabledTasks()

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled tasks, got %d", len(enabled))
	}

	if enabled[0].ID != "task1" || enabled[1].ID != "task3" {
		t.Error("unexpected enabled tasks returned")
	}
}

func TestFestivalConfigExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not exist initially
	if FestivalConfigExists(tmpDir) {
		t.Error("expected config to not exist")
	}

	// Create config file
	configPath := filepath.Join(tmpDir, FestivalConfigFileName)
	if err := os.WriteFile(configPath, []byte("version: 1.0"), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Should exist now
	if !FestivalConfigExists(tmpDir) {
		t.Error("expected config to exist")
	}
}

// --- Type Config Extension Tests ---

func TestTypeConfigMetadataRoundTrip(t *testing.T) {
	original := &TypeConfigMetadata{
		AutoPhases: []string{"INGEST", "PLAN"},
		PendingPhases: []PendingPhase{
			{
				Name:      "IMPLEMENT",
				Type:      "implement",
				Role:      "developer",
				Trigger:   "manual",
				Generator: "phase_scaffold",
			},
			{
				Name:      "POLISH",
				Type:      "polish",
				Role:      "reviewer",
				Trigger:   "manual",
				Generator: "phase_scaffold",
			},
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored TypeConfigMetadata
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(restored.AutoPhases) != 2 {
		t.Errorf("expected 2 auto phases, got %d", len(restored.AutoPhases))
	}
	if restored.AutoPhases[0] != "INGEST" || restored.AutoPhases[1] != "PLAN" {
		t.Errorf("auto phases mismatch: %v", restored.AutoPhases)
	}
	if len(restored.PendingPhases) != 2 {
		t.Errorf("expected 2 pending phases, got %d", len(restored.PendingPhases))
	}
	if restored.PendingPhases[0].Name != "IMPLEMENT" {
		t.Errorf("expected first pending phase IMPLEMENT, got %s", restored.PendingPhases[0].Name)
	}
	if restored.PendingPhases[0].Role != "developer" {
		t.Errorf("expected role developer, got %s", restored.PendingPhases[0].Role)
	}
}

func TestFestivalConfigWithTypeConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &FestivalConfig{
		Version: "1.0",
		Metadata: FestivalMetadata{
			ID:           "TE0001",
			Name:         "test-festival",
			FestivalType: "standard",
		},
		TypeConfig: &TypeConfigMetadata{
			AutoPhases: []string{"INGEST", "PLAN"},
			PendingPhases: []PendingPhase{
				{Name: "IMPLEMENT", Type: "implement", Role: "developer", Trigger: "manual", Generator: "phase_scaffold"},
			},
		},
		QualityGates: QualityGatesConfig{Enabled: true},
		Templates:    TemplatePrefs{TaskDefault: "tasks/SIMPLE", PreferSimple: true},
		Tracking:     TrackingConfig{Enabled: true, ChecksumFile: ".festival-checksums.json"},
	}

	if err := SaveFestivalConfig(tmpDir, cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify type_config fields present in YAML output
	data, err := os.ReadFile(filepath.Join(tmpDir, FestivalConfigFileName))
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	content := string(data)
	for _, field := range []string{"type_config:", "auto_phases:", "pending_phases:"} {
		if !strings.Contains(content, field) {
			t.Errorf("expected %s in YAML output", field)
		}
	}

	// Read back and verify round-trip
	restored, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if restored.Metadata.FestivalType != "standard" {
		t.Errorf("expected type standard, got %s", restored.Metadata.FestivalType)
	}
	if restored.TypeConfig == nil {
		t.Fatal("expected TypeConfig to be non-nil")
	}
	if len(restored.TypeConfig.AutoPhases) != 2 {
		t.Errorf("expected 2 auto phases, got %d", len(restored.TypeConfig.AutoPhases))
	}
	if len(restored.TypeConfig.PendingPhases) != 1 {
		t.Errorf("expected 1 pending phase, got %d", len(restored.TypeConfig.PendingPhases))
	}
	pp := restored.TypeConfig.PendingPhases[0]
	if pp.Name != "IMPLEMENT" || pp.Generator != "phase_scaffold" {
		t.Errorf("pending phase mismatch: %+v", pp)
	}
}

func TestBackwardCompatibility_NoTypeFields(t *testing.T) {
	tmpDir := t.TempDir()
	festPath := filepath.Join(tmpDir, FestivalConfigFileName)

	oldStyle := `version: "1.0"
metadata:
  id: OL0001
  name: old-festival
quality_gates:
  enabled: true
templates:
  task_default: tasks/SIMPLE
  prefer_simple: true
tracking:
  enabled: true
  checksum_file: .festival-checksums.json
`
	if err := os.WriteFile(festPath, []byte(oldStyle), 0644); err != nil {
		t.Fatalf("write old-style fest.yaml: %v", err)
	}

	cfg, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Metadata.FestivalType != "standard" {
		t.Errorf("expected default type 'standard', got %s", cfg.Metadata.FestivalType)
	}
	if cfg.TypeConfig != nil {
		t.Error("expected TypeConfig to be nil for legacy festival")
	}
}

func TestBackwardCompatibility_NoMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	festPath := filepath.Join(tmpDir, FestivalConfigFileName)

	minimal := `version: "1.0"
quality_gates:
  enabled: true
`
	if err := os.WriteFile(festPath, []byte(minimal), 0644); err != nil {
		t.Fatalf("write minimal fest.yaml: %v", err)
	}

	cfg, err := LoadFestivalConfig(tmpDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Metadata.FestivalType != "standard" {
		t.Errorf("expected default type 'standard', got %s", cfg.Metadata.FestivalType)
	}
}

func TestHasPendingPhases(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *FestivalConfig
		expected bool
	}{
		{"nil type config", &FestivalConfig{}, false},
		{"empty pending phases", &FestivalConfig{TypeConfig: &TypeConfigMetadata{}}, false},
		{
			"has pending phases",
			&FestivalConfig{TypeConfig: &TypeConfigMetadata{
				PendingPhases: []PendingPhase{{Name: "IMPLEMENT", Type: "implement"}},
			}},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.HasPendingPhases(); got != tc.expected {
				t.Errorf("HasPendingPhases() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestGetPendingPhaseByName(t *testing.T) {
	cfg := &FestivalConfig{
		TypeConfig: &TypeConfigMetadata{
			PendingPhases: []PendingPhase{
				{Name: "IMPLEMENT", Type: "implement", Role: "developer"},
				{Name: "POLISH", Type: "polish", Role: "reviewer"},
			},
		},
	}

	t.Run("found", func(t *testing.T) {
		phase, found := cfg.GetPendingPhaseByName("IMPLEMENT")
		if !found {
			t.Fatal("expected to find IMPLEMENT")
		}
		if phase.Type != "implement" {
			t.Errorf("expected type implement, got %s", phase.Type)
		}
		if phase.Role != "developer" {
			t.Errorf("expected role developer, got %s", phase.Role)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found := cfg.GetPendingPhaseByName("NONEXISTENT")
		if found {
			t.Error("expected not to find NONEXISTENT")
		}
	})

	t.Run("nil type config", func(t *testing.T) {
		empty := &FestivalConfig{}
		_, found := empty.GetPendingPhaseByName("IMPLEMENT")
		if found {
			t.Error("expected not to find with nil TypeConfig")
		}
	})
}

func TestIsAutoPhase(t *testing.T) {
	cfg := &FestivalConfig{
		TypeConfig: &TypeConfigMetadata{
			AutoPhases: []string{"INGEST", "PLAN"},
		},
	}

	tests := []struct {
		name     string
		phase    string
		expected bool
	}{
		{"auto phase INGEST", "INGEST", true},
		{"auto phase PLAN", "PLAN", true},
		{"not auto phase", "IMPLEMENT", false},
		{"empty string", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.IsAutoPhase(tc.phase); got != tc.expected {
				t.Errorf("IsAutoPhase(%q) = %v, want %v", tc.phase, got, tc.expected)
			}
		})
	}

	t.Run("nil type config", func(t *testing.T) {
		empty := &FestivalConfig{}
		if empty.IsAutoPhase("INGEST") {
			t.Error("expected false with nil TypeConfig")
		}
	})
}

func TestRemovePendingPhase(t *testing.T) {
	t.Run("removes existing phase", func(t *testing.T) {
		cfg := &FestivalConfig{
			TypeConfig: &TypeConfigMetadata{
				PendingPhases: []PendingPhase{
					{Name: "IMPLEMENT", Type: "implement"},
					{Name: "POLISH", Type: "polish"},
				},
			},
		}

		cfg.RemovePendingPhase("IMPLEMENT")

		if len(cfg.TypeConfig.PendingPhases) != 1 {
			t.Fatalf("expected 1 pending phase, got %d", len(cfg.TypeConfig.PendingPhases))
		}
		if cfg.TypeConfig.PendingPhases[0].Name != "POLISH" {
			t.Errorf("expected POLISH to remain, got %s", cfg.TypeConfig.PendingPhases[0].Name)
		}
	})

	t.Run("no-op for nonexistent phase", func(t *testing.T) {
		cfg := &FestivalConfig{
			TypeConfig: &TypeConfigMetadata{
				PendingPhases: []PendingPhase{
					{Name: "IMPLEMENT", Type: "implement"},
				},
			},
		}

		cfg.RemovePendingPhase("NONEXISTENT")

		if len(cfg.TypeConfig.PendingPhases) != 1 {
			t.Errorf("expected 1 pending phase unchanged, got %d", len(cfg.TypeConfig.PendingPhases))
		}
	})

	t.Run("no-op for nil type config", func(t *testing.T) {
		cfg := &FestivalConfig{}
		cfg.RemovePendingPhase("IMPLEMENT") // Should not panic
	})
}

func TestPendingPhaseAllFields(t *testing.T) {
	phase := PendingPhase{
		Name:      "IMPLEMENT",
		Type:      "implement",
		Role:      "developer",
		Trigger:   "manual",
		Generator: "phase_scaffold",
	}

	data, err := yaml.Marshal(&phase)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	yamlStr := string(data)
	for _, field := range []string{"name:", "type:", "role:", "trigger:", "generator:"} {
		if !strings.Contains(yamlStr, field) {
			t.Errorf("expected YAML to contain %s, got:\n%s", field, yamlStr)
		}
	}

	var restored PendingPhase
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.Name != phase.Name {
		t.Errorf("name mismatch: %s vs %s", restored.Name, phase.Name)
	}
	if restored.Role != phase.Role {
		t.Errorf("role mismatch: %s vs %s", restored.Role, phase.Role)
	}
	if restored.Trigger != phase.Trigger {
		t.Errorf("trigger mismatch: %s vs %s", restored.Trigger, phase.Trigger)
	}
	if restored.Generator != phase.Generator {
		t.Errorf("generator mismatch: %s vs %s", restored.Generator, phase.Generator)
	}
}

func TestSkipIngestionField(t *testing.T) {
	cfg := &TypeConfigMetadata{
		SkipIngestion: true,
		AutoPhases:    []string{"DO"},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if !strings.Contains(string(data), "skip_ingestion: true") {
		t.Error("expected skip_ingestion: true in YAML output")
	}

	var restored TypeConfigMetadata
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !restored.SkipIngestion {
		t.Error("expected SkipIngestion to be true")
	}
}
