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
		want      bool
	}{
		{
			name:      "no issues",
			issues:    nil,
			phaseType: "",
			want:      false,
		},
		{
			name: "naming warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeNamingConvention},
			},
			phaseType: "",
			want:      false,
		},
		{
			name: "info only does not block",
			issues: []validator.Issue{
				{Level: validator.LevelInfo, Code: validator.CodeNamingConvention},
			},
			phaseType: "",
			want:      false,
		},
		{
			name: "error level issue",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeMissingFile},
			},
			phaseType: "",
			want:      true,
		},
		{
			name: "unfilled template warning does not block",
			issues: []validator.Issue{
				{Level: validator.LevelWarning, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "",
			want:      false,
		},
		{
			name: "unfilled template error blocks with no phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "",
			want:      true,
		},
		{
			name: "unfilled template error blocks in implementation phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "implementation",
			want:      true,
		},
		{
			name: "unfilled template error skipped in ingest phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "ingest",
			want:      false,
		},
		{
			name: "unfilled template error skipped in planning phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "planning",
			want:      false,
		},
		{
			name: "unfilled template error skipped in research phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeUnfilledTemplate},
			},
			phaseType: "research",
			want:      false,
		},
		{
			name: "non-marker error still blocks in ingest phase",
			issues: []validator.Issue{
				{Level: validator.LevelError, Code: validator.CodeMissingFile},
			},
			phaseType: "ingest",
			want:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &validator.Result{Issues: tc.issues}
			got := hasBlockingIssues(result, tc.phaseType)
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
	if !hasBlockingIssues(result, "") {
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

	if !hasBlockingIssues(result, "implementation") {
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

	if hasBlockingIssues(result, "planning") {
		t.Error("expected hasBlockingIssues to return false for planning-phase warnings")
	}
}

// TestNextIngestPhaseSkipsMarkerErrors verifies the core fix:
// when the current phase is ingest type, unfilled template marker errors
// (from festival-root files) do NOT block fest next.
// Uses a synthetic Result to isolate the phase-aware filtering logic.
func TestNextIngestPhaseSkipsMarkerErrors(t *testing.T) {
	// Simulate what FullValidate produces for a festival with unfilled markers
	// in festival-root files (FESTIVAL_GOAL.md, TODO.md) — these resolve to
	// implementation type → error level.
	result := &validator.Result{
		Issues: []validator.Issue{
			{
				Level:   validator.LevelError,
				Code:    validator.CodeUnfilledTemplate,
				Path:    "FESTIVAL_GOAL.md",
				Message: "File contains 1 unfilled template markers ([FILL:)",
			},
			{
				Level:   validator.LevelError,
				Code:    validator.CodeUnfilledTemplate,
				Path:    "TODO.md",
				Message: "File contains 1 unfilled template markers ([FILL:)",
			},
		},
	}

	// With ingest phase type, marker errors should NOT block
	if hasBlockingIssues(result, "ingest") {
		t.Error("expected hasBlockingIssues to return false for ingest phase despite marker errors")
	}

	// With planning phase type, marker errors should NOT block
	if hasBlockingIssues(result, "planning") {
		t.Error("expected hasBlockingIssues to return false for planning phase despite marker errors")
	}

	// With implementation phase type, same errors SHOULD block
	if !hasBlockingIssues(result, "implementation") {
		t.Error("expected hasBlockingIssues to return true for implementation phase with marker errors")
	}

	// With no phase detected (empty string), should block (safe default)
	if !hasBlockingIssues(result, "") {
		t.Error("expected hasBlockingIssues to return true when no phase detected")
	}
}
