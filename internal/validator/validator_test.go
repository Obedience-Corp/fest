package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStructureValidator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setup      func(t *testing.T) string
		wantIssues int
	}{
		{
			name: "valid structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING", "01_requirements"), 0755)
				_ = os.WriteFile(filepath.Join(dir, "001_PLANNING", "01_requirements", "01_gather.md"), []byte("# Task"), 0644)
				return dir
			},
			wantIssues: 0,
		},
		{
			name: "lowercase phase",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "001_planning"), 0755)
				return dir
			},
			wantIssues: 1,
		},
		{
			name: "uppercase sequence",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Use IMPLEMENTATION phase since PLANNING is now freeform
				_ = os.MkdirAll(filepath.Join(dir, "001_IMPLEMENTATION"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "001_IMPLEMENTATION", "01_REQUIREMENTS"), 0755)
				return dir
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			v := NewStructureValidator()
			issues, err := v.Validate(ctx, dir)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if len(issues) != tt.wantIssues {
				t.Errorf("Validate() got %d issues, want %d", len(issues), tt.wantIssues)
				for _, issue := range issues {
					t.Logf("  Issue: %s - %s", issue.Code, issue.Message)
				}
			}
		})
	}
}

func TestCompletenessValidator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantMin int    // minimum issues expected
		wantHas string // code that must be present (or empty)
	}{
		{
			name: "complete festival",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Overview"), 0644)
				_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_RULES.md"), []byte("# Rules"), 0644)
				_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING"), 0755)
				_ = os.WriteFile(filepath.Join(dir, "001_PLANNING", "PHASE_GOAL.md"), []byte("# Goal"), 0644)
				return dir
			},
			wantMin: 0,
		},
		{
			name: "missing overview",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING"), 0755)
				_ = os.WriteFile(filepath.Join(dir, "001_PLANNING", "PHASE_GOAL.md"), []byte("# Goal"), 0644)
				return dir
			},
			wantMin: 1,
			wantHas: "missing_file", // FESTIVAL_OVERVIEW.md is required
		},
		{
			name: "missing phase goal",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Overview"), 0644)
				_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_RULES.md"), []byte("# Rules"), 0644)
				_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING"), 0755)
				return dir
			},
			wantMin: 1,
			wantHas: "missing_goal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			v := NewCompletenessValidator()
			issues, err := v.Validate(ctx, dir)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if len(issues) < tt.wantMin {
				t.Errorf("Validate() got %d issues, want at least %d", len(issues), tt.wantMin)
			}
			if tt.wantHas != "" {
				found := false
				for _, issue := range issues {
					if issue.Code == tt.wantHas {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue with code %q", tt.wantHas)
					for _, issue := range issues {
						t.Logf("  Got: %s - %s", issue.Code, issue.Message)
					}
				}
			}
		})
	}
}

func TestTemplateValidator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setup      func(t *testing.T) string
		wantIssues int
	}{
		{
			name: "no unfilled markers",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "task.md"), []byte("# Real Content\nThis is filled in."), 0644)
				return dir
			},
			wantIssues: 0,
		},
		{
			name: "unfilled FILL marker",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "task.md"), []byte("# Task\n[FILL: description]"), 0644)
				return dir
			},
			wantIssues: 1,
		},
		{
			name: "unfilled template variable",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "task.md"), []byte("# Task\nPhase: {{ phase_name }}"), 0644)
				return dir
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			v := NewTemplateValidator()
			issues, err := v.Validate(ctx, dir)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if len(issues) != tt.wantIssues {
				t.Errorf("Validate() got %d issues, want %d", len(issues), tt.wantIssues)
				for _, issue := range issues {
					t.Logf("  Issue: %s - %s", issue.Code, issue.Message)
				}
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "001_PLANNING"), 0755)

	v := NewStructureValidator()
	_, err := v.Validate(ctx, dir)

	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestIssue(t *testing.T) {
	issue := Issue{
		Level:       "warning",
		Code:        "TEST001",
		Path:        "/path/to/file",
		Message:     "Test message",
		Fix:         "Do something",
		AutoFixable: true,
	}

	if issue.Level != "warning" {
		t.Errorf("Level = %q, want %q", issue.Level, "warning")
	}
	if issue.Code != "TEST001" {
		t.Errorf("Code = %q, want %q", issue.Code, "TEST001")
	}
	if !issue.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
}

func TestResult(t *testing.T) {
	result := &Result{
		OK:       true,
		Action:   "validate",
		Festival: "test-festival",
		Valid:    true,
		Score:    85,
		Issues:   []Issue{},
	}

	if !result.OK {
		t.Error("Expected OK to be true")
	}
	if result.Score != 85 {
		t.Errorf("Score = %d, want %d", result.Score, 85)
	}
}

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name      string
		issues    []Issue
		wantScore int
	}{
		{
			name:      "perfect score",
			issues:    []Issue{},
			wantScore: 100,
		},
		{
			name: "with warnings only",
			issues: []Issue{
				{Level: LevelWarning, Code: "W001"},
				{Level: LevelWarning, Code: "W002"},
			},
			wantScore: 90, // 100 - 2*5
		},
		{
			name: "with errors",
			issues: []Issue{
				{Level: LevelError, Code: "E001"},
				{Level: LevelError, Code: "E002"},
			},
			wantScore: 70, // 100 - 2*15
		},
		{
			name: "mixed issues",
			issues: []Issue{
				{Level: LevelError, Code: "E001"},
				{Level: LevelWarning, Code: "W001"},
			},
			wantScore: 80, // 100 - 15 - 5
		},
		{
			name: "many unfilled markers cap at one pending penalty",
			issues: []Issue{
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
			},
			wantScore: 100 - MarkerPendingPenalty,
		},
		{
			name: "markers pending plus structural error",
			issues: []Issue{
				{Level: LevelError, Code: CodeMissingFile},
				{Level: LevelError, Code: CodeUnfilledTemplate},
				{Level: LevelError, Code: CodeUnfilledTemplate},
			},
			wantScore: 100 - 15 - MarkerPendingPenalty,
		},
		{
			name: "planning-phase marker warnings do not stack",
			issues: []Issue{
				{Level: LevelWarning, Code: CodeUnfilledTemplate},
				{Level: LevelWarning, Code: CodeUnfilledTemplate},
				{Level: LevelWarning, Code: CodeUnfilledTemplate},
			},
			wantScore: 100 - MarkerPendingPenalty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{Issues: tt.issues}
			score := CalculateScore(result)
			if score != tt.wantScore {
				t.Errorf("CalculateScore() = %d, want %d", score, tt.wantScore)
			}
		})
	}
}

func TestHasStructuralErrorsIgnoresPendingMarkers(t *testing.T) {
	markersOnly := &Result{Issues: []Issue{
		{Level: LevelError, Code: CodeUnfilledTemplate, Path: "a.md"},
		{Level: LevelError, Code: CodeUnfilledTemplate, Path: "b.md"},
	}}
	if !markersOnly.HasErrors() {
		t.Fatal("HasErrors() should still be true so fest next can block on implementation markers")
	}
	if markersOnly.HasStructuralErrors() {
		t.Fatal("HasStructuralErrors() should be false when only markers are pending")
	}
	if !markersOnly.HasPendingMarkers() {
		t.Fatal("HasPendingMarkers() = false, want true")
	}

	broken := &Result{Issues: []Issue{
		{Level: LevelError, Code: CodeMissingFile, Path: "FESTIVAL_OVERVIEW.md"},
		{Level: LevelError, Code: CodeUnfilledTemplate, Path: "a.md"},
	}}
	if !broken.HasStructuralErrors() {
		t.Fatal("HasStructuralErrors() should be true when a file is missing")
	}
}

func TestQuickValidate_MarkersPendingIsStructurallyValid(t *testing.T) {
	dir := setupTemplateTestFestival(t, "implementation", "# Task\n\n[REPLACE: placeholder text]\n")
	_ = os.WriteFile(filepath.Join(dir, "001_PHASE", "01_seq", "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\nDo the thing.\n"), 0644)

	result, err := QuickValidate(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasPendingMarkers() {
		t.Fatal("expected pending markers")
	}
	if result.HasStructuralErrors() {
		t.Fatalf("unexpected structural errors: %+v", result.Issues)
	}
	if !result.Valid {
		t.Fatalf("Valid = false, want true for markers-only festival; issues=%+v", result.Issues)
	}
	if result.OK {
		t.Fatal("OK should stay false while marker errors remain so create --agent still blocks")
	}
	if result.Score != 100-MarkerPendingPenalty {
		t.Fatalf("Score = %d, want %d", result.Score, 100-MarkerPendingPenalty)
	}
}
