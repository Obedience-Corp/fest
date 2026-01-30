package frontmatter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferFromPath(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create festival structure
	festivalDir := filepath.Join(tmpDir, "festivals", "active", "test-fest")
	phaseDir := filepath.Join(festivalDir, "001_Planning")
	seqDir := filepath.Join(phaseDir, "01_setup")
	taskFile := filepath.Join(seqDir, "01_first_task.md")

	// Create directories
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatalf("Failed to create test dirs: %v", err)
	}

	// Create task file
	if err := os.WriteFile(taskFile, []byte("# Task"), 0644); err != nil {
		t.Fatalf("Failed to create task file: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		expectedType Type
		expectedID   string
		wantError    bool
	}{
		{
			name:         "task file",
			path:         taskFile,
			expectedType: TypeTask,
			expectedID:   "01_first_task",
			wantError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm, err := InferFromPath(tc.path)
			if tc.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("InferFromPath() error = %v", err)
			}

			if fm.Type != tc.expectedType {
				t.Errorf("Type = %q, want %q", fm.Type, tc.expectedType)
			}
			if fm.ID != tc.expectedID {
				t.Errorf("ID = %q, want %q", fm.ID, tc.expectedID)
			}
		})
	}
}

func TestInferFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create phase directory
	phaseDir := filepath.Join(tmpDir, "001_Planning")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatalf("Failed to create phase dir: %v", err)
	}

	// Create PHASE_GOAL.md
	goalFile := filepath.Join(phaseDir, "PHASE_GOAL.md")
	if err := os.WriteFile(goalFile, []byte("# Phase Goal"), 0644); err != nil {
		t.Fatalf("Failed to create goal file: %v", err)
	}

	fm, err := InferFromDirectory(phaseDir)
	if err != nil {
		t.Fatalf("InferFromDirectory() error = %v", err)
	}

	if fm.Type != TypePhase {
		t.Errorf("Type = %q, want phase", fm.Type)
	}
	if fm.ID != "001_Planning" {
		t.Errorf("ID = %q, want 001_Planning", fm.ID)
	}
}

func TestInferFromDirectory_Sequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sequence directory
	seqDir := filepath.Join(tmpDir, "01_setup")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatalf("Failed to create seq dir: %v", err)
	}

	// Create SEQUENCE_GOAL.md
	goalFile := filepath.Join(seqDir, "SEQUENCE_GOAL.md")
	if err := os.WriteFile(goalFile, []byte("# Sequence Goal"), 0644); err != nil {
		t.Fatalf("Failed to create goal file: %v", err)
	}

	fm, err := InferFromDirectory(seqDir)
	if err != nil {
		t.Fatalf("InferFromDirectory() error = %v", err)
	}

	if fm.Type != TypeSequence {
		t.Errorf("Type = %q, want sequence", fm.Type)
	}
	if fm.ID != "01_setup" {
		t.Errorf("ID = %q, want 01_setup", fm.ID)
	}
}

func TestInferFromDirectory_Festival(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festival directory
	festDir := filepath.Join(tmpDir, "my-festival")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatalf("Failed to create fest dir: %v", err)
	}

	// Create FESTIVAL_GOAL.md
	goalFile := filepath.Join(festDir, "FESTIVAL_GOAL.md")
	if err := os.WriteFile(goalFile, []byte("# Festival Goal"), 0644); err != nil {
		t.Fatalf("Failed to create goal file: %v", err)
	}

	fm, err := InferFromDirectory(festDir)
	if err != nil {
		t.Fatalf("InferFromDirectory() error = %v", err)
	}

	if fm.Type != TypeFestival {
		t.Errorf("Type = %q, want festival", fm.Type)
	}
}

func TestInferFromPath_Gate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a gate file
	gateFile := filepath.Join(tmpDir, "quality_gate_review.md")
	if err := os.WriteFile(gateFile, []byte("# Gate"), 0644); err != nil {
		t.Fatalf("Failed to create gate file: %v", err)
	}

	fm, err := InferFromPath(gateFile)
	if err != nil {
		t.Fatalf("InferFromPath() error = %v", err)
	}

	if fm.Type != TypeGate {
		t.Errorf("Type = %q, want gate", fm.Type)
	}
}

func TestInferFromPath_Phase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create phase file
	phaseFile := filepath.Join(tmpDir, "PHASE_GOAL.md")
	if err := os.WriteFile(phaseFile, []byte("# Phase Goal"), 0644); err != nil {
		t.Fatalf("Failed to create phase file: %v", err)
	}

	fm, err := InferFromPath(phaseFile)
	if err != nil {
		t.Fatalf("InferFromPath() error = %v", err)
	}

	if fm.Type != TypePhase {
		t.Errorf("Type = %q, want phase", fm.Type)
	}
}

func TestInferFromPath_Sequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sequence file
	seqFile := filepath.Join(tmpDir, "SEQUENCE_GOAL.md")
	if err := os.WriteFile(seqFile, []byte("# Sequence Goal"), 0644); err != nil {
		t.Fatalf("Failed to create seq file: %v", err)
	}

	fm, err := InferFromPath(seqFile)
	if err != nil {
		t.Fatalf("InferFromPath() error = %v", err)
	}

	if fm.Type != TypeSequence {
		t.Errorf("Type = %q, want sequence", fm.Type)
	}
}

func TestMergeInto(t *testing.T) {
	original := &Frontmatter{
		Type:   TypeTask,
		ID:     "01_test",
		Status: StatusPending,
		// Name is empty
	}

	inferred := &Frontmatter{
		Type:   TypeTask,
		ID:     "01_test",
		Name:   "Test Task",
		Parent: "01_seq",
		Order:  1,
		Status: StatusCompleted, // Should overwrite because it's non-empty
	}

	merged := MergeInto(original, inferred)

	if merged.Name != "Test Task" {
		t.Errorf("Name should be filled from inferred, got %q", merged.Name)
	}
	if merged.Parent != "01_seq" {
		t.Errorf("Parent should be filled from inferred, got %q", merged.Parent)
	}
	// MergeInto updates non-zero values, so status gets overwritten
	if merged.Status != StatusCompleted {
		t.Errorf("Status should be overwritten, got %q", merged.Status)
	}
}

func TestInferCreatedTime(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := InferCreatedTime(testFile)
	if result.IsZero() {
		t.Error("InferCreatedTime should return non-zero time for existing file")
	}
}

func TestInferCreatedTime_NonExistent(t *testing.T) {
	result := InferCreatedTime("/nonexistent/path/file.md")
	if result.IsZero() {
		t.Error("InferCreatedTime should return time.Now() for non-existent file")
	}
}

func TestInferWorkType(t *testing.T) {
	tests := []struct {
		filename string
		expected WorkType
	}{
		// Verification/testing
		{"01_testing_and_verify.md", WorkTypeVerify},
		{"02_test_coverage.md", WorkTypeVerify},
		{"qa_validation.md", WorkTypeVerify},

		// Review
		{"code_review.md", WorkTypeReview},
		{"review_results_iterate.md", WorkTypeReview},
		{"approval_gate.md", WorkTypeReview},

		// Analysis/research
		{"01_analysis.md", WorkTypeAnalysis},
		{"research_options.md", WorkTypeAnalysis},
		{"investigate_issue.md", WorkTypeAnalysis},
		{"discovery_phase.md", WorkTypeAnalysis},

		// Documentation
		{"update_docs.md", WorkTypeDocs},
		{"readme_updates.md", WorkTypeDocs},
		{"changelog_entry.md", WorkTypeDocs},

		// Planning
		{"design_system.md", WorkTypePlan},
		{"architect_solution.md", WorkTypePlan},
		{"scope_definition.md", WorkTypePlan},

		// Configuration
		{"config_setup.md", WorkTypeConfig},
		{"environment_settings.md", WorkTypeConfig},

		// Action
		{"deploy_production.md", WorkTypeAction},
		{"publish_release.md", WorkTypeAction},
		{"migrate_data.md", WorkTypeAction},
		{"commit_changes.md", WorkTypeAction},

		// Default implementation
		{"01_implement_feature.md", WorkTypeImplementation},
		{"build_module.md", WorkTypeImplementation},
		{"create_api.md", WorkTypeImplementation},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := InferWorkType(tc.filename)
			if got != tc.expected {
				t.Errorf("InferWorkType(%q) = %q, want %q", tc.filename, got, tc.expected)
			}
		})
	}
}

func TestWorkTypeLabel(t *testing.T) {
	tests := []struct {
		workType WorkType
		expected string
	}{
		{WorkTypeImplementation, "[impl]"},
		{WorkTypeAnalysis, "[analysis]"},
		{WorkTypeReview, "[review]"},
		{WorkTypeVerify, "[verify]"},
		{WorkTypeConfig, "[config]"},
		{WorkTypeDocs, "[docs]"},
		{WorkTypePlan, "[plan]"},
		{WorkTypeAction, "[action]"},
		{WorkType("unknown"), ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.workType), func(t *testing.T) {
			got := tc.workType.Label()
			if got != tc.expected {
				t.Errorf("WorkType(%q).Label() = %q, want %q", tc.workType, got, tc.expected)
			}
		})
	}
}
