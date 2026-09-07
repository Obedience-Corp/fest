package next

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/validator"
)

func TestHasBlockingIssues(t *testing.T) {
	tests := []struct {
		name      string
		issues    []validator.Issue
		phaseType string
		phaseName string
		status    string
		want      bool
	}{
		{
			name:   "no issues",
			issues: nil,
			want:   false,
		},
		{
			name: "naming warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeNamingConvention},
			},
			want: false,
		},
		{
			name: "info only does not block",
			issues: []validator.Issue{
				{Level: validator.LevelInfo, Code: validator.CodeNamingConvention},
			},
			want: false,
		},
		{
			name: "error level issue",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeMissingFile},
			},
			want: true,
		},
		{
			name: "unfilled template warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeUnfilledTemplate},
			},
			want: false,
		},
		{
			name: "root marker in an unpromoted festival does not block",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			want: false,
		},
		{
			name: "root marker blocks in an active implementation phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			phaseType: "implementation",
			phaseName: "001_IMPL",
			status:    "active",
			want:      true,
		},
		// --- Preparatory phase: festival-root markers skipped ---
		{
			name: "festival-root marker skipped in ingest phase while planning",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "planning",
			want:      false,
		},
		{
			name: "festival-root marker skipped in planning phase while planning",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "TODO.md"},
			},
			phaseType: "planning",
			phaseName: "002_PLAN",
			status:    "planning",
			want:      false,
		},
		{
			name: "festival-root marker skipped in research phase while planning",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			phaseType: "research",
			phaseName: "001_RESEARCH",
			status:    "planning",
			want:      false,
		},
		// --- Festival-root markers: status decides, phase does not ---
		{
			name: "festival-root marker skipped in planning with no phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			status: "planning",
			want:   false,
		},
		{
			name: "festival-root marker blocks once ready",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			status: "ready",
			want:   true,
		},
		{
			name: "festival-root marker blocks once active",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "TODO.md"},
			},
			status: "active",
			want:   true,
		},
		{
			name: "festival-root marker blocks in ready even inside an ingest phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "ready",
			want:      true,
		},
		{
			// An undetermined status relaxes here and fails closed a moment
			// later: lifecycle.EnforcePreActive stops fest next outright when
			// it cannot read the festival's status.
			name: "festival-root marker skipped when status is undetermined",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
			},
			phaseType: "planning",
			phaseName: "001_PLAN",
			status:    "",
			want:      false,
		},
		// --- Preparatory phase: markers inside the preparatory phase skipped ---
		{
			name: "marker inside ingest phase dir skipped",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "001_INGEST/01_seq/task.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "planning",
			want:      false,
		},
		{
			name: "marker inside ingest phase dir skipped while active",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "001_INGEST/01_seq/task.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "active",
			want:      false,
		},
		// --- Preparatory phase: markers from OTHER phases still block ---
		{
			name: "marker from impl phase blocks even when current is ingest",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "002_IMPL/01_seq/task.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "planning",
			want:      true,
		},
		{
			name: "marker from impl phase blocks even when current is planning",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "003_IMPL/01_seq/task.md"},
			},
			phaseType: "planning",
			phaseName: "002_PLAN",
			status:    "planning",
			want:      true,
		},
		// --- Mixed: root markers ok but impl markers block ---
		{
			name: "root marker ok but impl marker blocks in ingest phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "FESTIVAL_GOAL.md"},
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate, Path: "002_IMPL/01_seq/task.md"},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "planning",
			want:      true,
		},
		// --- Non-marker errors still block in preparatory phases ---
		{
			name: "non-marker error still blocks in ingest phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeMissingFile},
			},
			phaseType: "ingest",
			phaseName: "001_INGEST",
			status:    "planning",
			want:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &validator.Result{Issues: tc.issues}
			got := hasBlockingIssues(result, tc.phaseType, tc.phaseName, tc.status)
			if got != tc.want {
				t.Errorf("hasBlockingIssues() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEmitValidationBlock(t *testing.T) {
	festivalPath := "/tmp/test-fest"
	result := &validator.Result{
		Valid: false,
		Issues: []validator.Issue{
			{
				Level:   validator.LevelWarning,
				Code:    validator.CodeUnfilledTemplate,
				Path:    "/tmp/test-fest/001_PLAN/PHASE_GOAL.md",
				Message: "File contains unfilled template marker: [FILL:",
			},
			{
				Level:   validator.LevelWarning,
				Code:    validator.CodeNamingConvention,
				Path:    "/tmp/test-fest/002_IMPL/notes.txt",
				Message: "naming issue",
			},
		},
	}

	var err error
	output := captureStdout(t, func() {
		err = emitValidationBlock(festivalPath, result)
	})

	if err == nil {
		t.Fatal("expected error from emitValidationBlock")
	}
	if !strings.Contains(output, "STOP") {
		t.Errorf("expected STOP header, got: %s", output)
	}
	if !strings.Contains(output, "001_PLAN/PHASE_GOAL.md") {
		t.Errorf("expected relative path in output, got: %s", output)
	}
	if !strings.Contains(output, "naming issue") {
		t.Errorf("warnings should appear in output, got: %s", output)
	}
	if !strings.Contains(output, "Warnings:") {
		t.Errorf("expected Warnings: section header, got: %s", output)
	}
	if !strings.Contains(output, "fest validate") {
		t.Errorf("expected hint to run fest validate, got: %s", output)
	}
}

func TestNextBlocksOnUnfilledMarkers(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals", "active", "test-fest")
	phaseDir := filepath.Join(festDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_task_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\ntitle: Plan\n---\n# Goal\n\n[FILL: Describe the phase goal here]\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	taskContent := "---\ntitle: Task\n---\n# Task\n\n[GUIDANCE: Describe what to do]\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_do_thing.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	// With no phase type (empty string = safe default), marker errors should block
	if !hasBlockingIssues(result, "", "", "") {
		t.Fatal("expected blocking issues for festival with unfilled markers (no phase context)")
	}

	hasTemplateIssue := false
	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate {
			hasTemplateIssue = true
			break
		}
	}
	if !hasTemplateIssue {
		t.Error("expected unfilled_template issue code")
	}
}

func TestNextBlocksOnReplaceMarkerInImplementation(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "test-fest")
	phaseDir := filepath.Join(festDir, "001_IMPL")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\nfest_type: phase\nfest_phase_type: implementation\nfest_status: pending\nfest_id: P001\nfest_parent: F001\nfest_order: 1\nfest_created: 2026-01-01T00:00:00Z\n---\n# Phase Goal\nImplement the thing.\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	taskContent := "# Task: Do Thing\n\n## Objective\n[REPLACE: Describe the objective]\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_do_thing.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	hasError := false
	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate && issue.Level == validator.LevelError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected error-level unfilled_template issue for [REPLACE:] in implementation phase")
	}

	if !hasBlockingIssues(result, "implementation", "001_IMPL", "active") {
		t.Error("expected hasBlockingIssues to return true for [REPLACE:] in implementation phase")
	}
}

func TestNextDoesNotBlockOnPlanningWarnings(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "test-fest")
	phaseDir := filepath.Join(festDir, "001_PLAN")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\nfest_type: phase\nfest_phase_type: planning\nfest_status: pending\nfest_id: P001\nfest_parent: F001\nfest_order: 1\nfest_created: 2026-01-01T00:00:00Z\n---\n# Phase Goal\nPlan the thing.\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Sequence Goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	taskContent := "# Task\n\n[FILL: describe planning approach]\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_plan_task.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := validator.FullValidate(ctx, festDir)
	if err != nil {
		t.Fatalf("FullValidate error: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Code == validator.CodeUnfilledTemplate && issue.Level == validator.LevelError {
			t.Errorf("planning phase markers should be warnings, got error: %+v", issue)
		}
	}

	if hasBlockingIssues(result, "planning", "001_PLAN", "planning") {
		t.Error("expected hasBlockingIssues to return false for planning-phase warnings")
	}
}
