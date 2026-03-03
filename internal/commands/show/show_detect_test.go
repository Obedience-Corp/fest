package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHasNumericPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"001_PLAN", true},
		{"01_setup", true},
		{"1_task", true},
		{"001", false}, // No underscore
		{"abc_test", false},
		{"a01_test", false},
		{"", false},
		{"_001", false},
	}

	for _, tc := range tests {
		result := hasNumericPrefix(tc.input)
		if result != tc.expected {
			t.Errorf("hasNumericPrefix(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestIsGateFile(t *testing.T) {
	// Tests for gate file detection using taskfilter.IsGate
	// Only specific patterns are considered gates:
	// - *_quality_gate.md, *_testing_gate.md (contains "gate")
	// - *_testing_and_verify.md
	// - *_code_review.md
	// - *_review_results_iterate.md
	// - *_commit.md (exact match only)
	tests := []struct {
		input    string
		expected bool
	}{
		{"01_quality_gate.md", true},
		{"01_testing_gate.md", true},
		{"01_code_review.md", true},
		{"01_testing_and_verify.md", true},
		{"01_review_results_iterate.md", true},
		{"01_commit.md", true},
		{"01_verify_build.md", false},     // Not a standard gate pattern
		{"01_iterate_feedback.md", false}, // Not a standard gate pattern
		{"01_implementation.md", false},
		{"01_task.md", false},
		{"SEQUENCE_GOAL.md", false},
	}

	for _, tc := range tests {
		result := isGateFile(tc.input)
		if result != tc.expected {
			t.Errorf("isGateFile(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestIsValidFestival(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid festival with FESTIVAL_GOAL.md
	validFestival1 := filepath.Join(tmpDir, "valid1")
	if err := os.MkdirAll(validFestival1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validFestival1, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a valid festival with fest.yaml
	validFestival2 := filepath.Join(tmpDir, "valid2")
	if err := os.MkdirAll(validFestival2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validFestival2, FestivalConfigFile), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an invalid directory
	invalidDir := filepath.Join(tmpDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		dir      string
		expected bool
	}{
		{validFestival1, true},
		{validFestival2, true},
		{invalidDir, false},
		{filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tc := range tests {
		result := isValidFestival(tc.dir)
		if result != tc.expected {
			t.Errorf("isValidFestival(%q) = %v, want %v", tc.dir, result, tc.expected)
		}
	}
}

func TestDetectCurrentFestival(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival structure
	festivalDir := filepath.Join(tmpDir, "my-festival")
	phaseDir := filepath.Join(festivalDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_setup")

	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, PhaseGoalFile), []byte("# Phase"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, SequenceGoalFile), []byte("# Sequence"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		startDir string
		wantName string
		wantErr  bool
	}{
		{festivalDir, "my-festival", false},
		{phaseDir, "my-festival", false},
		{seqDir, "my-festival", false},
		{tmpDir, "", true}, // Not in a festival
	}

	for _, tc := range tests {
		result, err := DetectCurrentFestival(context.Background(), tc.startDir, "")
		if tc.wantErr {
			if err == nil {
				t.Errorf("DetectCurrentFestival(%q) expected error, got nil", tc.startDir)
			}
		} else {
			if err != nil {
				t.Errorf("DetectCurrentFestival(%q) unexpected error: %v", tc.startDir, err)
			} else if result.Name != tc.wantName {
				t.Errorf("DetectCurrentFestival(%q) name = %q, want %q", tc.startDir, result.Name, tc.wantName)
			}
		}
	}
}

func TestDetectCurrentLocation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival structure
	festivalDir := filepath.Join(tmpDir, "my-festival")
	phaseDir := filepath.Join(festivalDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_setup")

	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, PhaseGoalFile), []byte("# Phase"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, SequenceGoalFile), []byte("# Sequence"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		startDir     string
		wantType     string
		wantPhase    string
		wantSequence string
	}{
		{festivalDir, "festival", "", ""},
		{phaseDir, "phase", "001_PLAN", ""},
		{seqDir, "sequence", "001_PLAN", "01_setup"},
	}

	for _, tc := range tests {
		result, err := DetectCurrentLocation(context.Background(), tc.startDir)
		if err != nil {
			t.Errorf("DetectCurrentLocation(%q) unexpected error: %v", tc.startDir, err)
			continue
		}
		if result.Type != tc.wantType {
			t.Errorf("DetectCurrentLocation(%q) type = %q, want %q", tc.startDir, result.Type, tc.wantType)
		}
		if result.Phase != tc.wantPhase {
			t.Errorf("DetectCurrentLocation(%q) phase = %q, want %q", tc.startDir, result.Phase, tc.wantPhase)
		}
		if result.Sequence != tc.wantSequence {
			t.Errorf("DetectCurrentLocation(%q) sequence = %q, want %q", tc.startDir, result.Sequence, tc.wantSequence)
		}
	}
}

func TestCalculateFestivalStats(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival structure with phases, sequences, and tasks
	festivalDir := filepath.Join(tmpDir, "my-festival")
	phase1 := filepath.Join(festivalDir, "001_PLAN")
	seq1 := filepath.Join(phase1, "01_setup")

	if err := os.MkdirAll(seq1, 0755); err != nil {
		t.Fatal(err)
	}

	// Create goal files
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phase1, PhaseGoalFile), []byte("# Phase"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seq1, SequenceGoalFile), []byte("# Sequence"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tasks
	if err := os.WriteFile(filepath.Join(seq1, "01_task1.md"), []byte("# Task 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seq1, "02_task2.md"), []byte("# Task 2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seq1, "03_quality_gate.md"), []byte("# Gate"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	stats, err := CalculateFestivalStats(ctx, festivalDir)
	if err != nil {
		t.Fatalf("CalculateFestivalStats() unexpected error: %v", err)
	}

	if stats.Phases.Total != 1 {
		t.Errorf("Phases.Total = %d, want 1", stats.Phases.Total)
	}
	if stats.Sequences.Total != 1 {
		t.Errorf("Sequences.Total = %d, want 1", stats.Sequences.Total)
	}
	// With unified progress counting, gates are included in task totals
	// (2 regular tasks + 1 gate = 3 total)
	if stats.Tasks.Total != 3 {
		t.Errorf("Tasks.Total = %d, want 3", stats.Tasks.Total)
	}
	if stats.Gates.Total != 1 {
		t.Errorf("Gates.Total = %d, want 1", stats.Gates.Total)
	}
}

// TestParseFestivalInfo_ReadsMetadataID tests that parseFestivalInfo reads metadata from fest.yaml
func TestParseFestivalInfo_ReadsMetadataID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival with fest.yaml containing metadata
	festivalDir := filepath.Join(tmpDir, "my-project_GU0001")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create fest.yaml with metadata
	festYAML := `version: "1.0"
metadata:
  id: GU0001
  uuid: 550e8400-e29b-41d4-a716-446655440000
  name: my-project
  created_at: 2025-12-31T12:00:00Z
quality_gates:
  enabled: true
`
	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := parseFestivalInfo(context.Background(), festivalDir, "")
	if err != nil {
		t.Fatalf("parseFestivalInfo() error = %v", err)
	}

	if info.MetadataID != "GU0001" {
		t.Errorf("MetadataID = %q, want %q", info.MetadataID, "GU0001")
	}
}

// TestParseFestivalInfo_DateDirectoryStatus tests that festivals inside date directories
// have their status correctly detected.
func TestParseFestivalInfo_DateDirectoryStatus(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		relPath    string
		wantStatus string
	}{
		{
			name:       "completed in date dir YYYY-MM-DD",
			relPath:    "festivals/dungeon/completed/2026-02-28/my-fest",
			wantStatus: "dungeon/completed",
		},
		{
			name:       "archived in date dir YYYY-MM-DD",
			relPath:    "festivals/dungeon/archived/2026-01-15/my-fest",
			wantStatus: "dungeon/archived",
		},
		{
			name:       "someday in date dir YYYY-MM-DD",
			relPath:    "festivals/dungeon/someday/2025-12-01/my-fest",
			wantStatus: "dungeon/someday",
		},
		{
			name:       "completed in date dir YYYY-MM (legacy)",
			relPath:    "festivals/dungeon/completed/2025-01/my-fest",
			wantStatus: "dungeon/completed",
		},
		{
			name:       "completed without date dir",
			relPath:    "festivals/dungeon/completed/my-fest",
			wantStatus: "dungeon/completed",
		},
		{
			name:       "active without date dir",
			relPath:    "festivals/active/my-fest",
			wantStatus: "active",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			festivalDir := filepath.Join(tmpDir, tc.relPath)
			if err := os.MkdirAll(festivalDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := parseFestivalInfo(context.Background(), festivalDir, "")
			if err != nil {
				t.Fatalf("parseFestivalInfo() error = %v", err)
			}

			if info.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", info.Status, tc.wantStatus)
			}
		})
	}
}

// TestParseFestivalInfo_LegacyFestivalNoMetadata tests legacy festivals without metadata
func TestParseFestivalInfo_LegacyFestivalNoMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival without metadata in fest.yaml
	festivalDir := filepath.Join(tmpDir, "old-festival")
	if err := os.MkdirAll(festivalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create fest.yaml without metadata section
	festYAML := `version: "1.0"
quality_gates:
  enabled: true
`
	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := parseFestivalInfo(context.Background(), festivalDir, "")
	if err != nil {
		t.Fatalf("parseFestivalInfo() error = %v", err)
	}

	if info.MetadataID != "" {
		t.Errorf("MetadataID = %q, want empty string for legacy festival", info.MetadataID)
	}
}
