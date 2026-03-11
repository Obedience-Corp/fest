package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTemplateTestFestival(t *testing.T, phaseType string, content string) string {
	t.Helper()
	dir := t.TempDir()

	// Create FESTIVAL_OVERVIEW.md
	_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Test Festival\n"), 0644)

	phasePath := filepath.Join(dir, "001_PHASE")
	_ = os.MkdirAll(phasePath, 0755)

	// Write PHASE_GOAL.md with frontmatter
	goalContent := "---\nfest_type: phase\nfest_phase_type: " + phaseType + "\nfest_status: pending\nfest_id: P001\nfest_parent: F001\nfest_order: 1\nfest_created: 2026-01-01T00:00:00Z\n---\n# Phase Goal\n"
	_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte(goalContent), 0644)

	// Write a task file with the given content
	seqPath := filepath.Join(phasePath, "01_seq")
	_ = os.MkdirAll(seqPath, 0755)
	_ = os.WriteFile(filepath.Join(seqPath, "01_task.md"), []byte(content), 0644)

	return dir
}

func TestValidateTemplateMarkers_DetectsReplace(t *testing.T) {
	dir := setupTemplateTestFestival(t, "implementation", "# Task\n\n[REPLACE: placeholder text]\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			found = true
		}
	}
	if !found {
		t.Error("expected [REPLACE:] marker to be detected")
	}
}

func TestValidateTemplateMarkers_ImplementationPhaseReturnsError(t *testing.T) {
	dir := setupTemplateTestFestival(t, "implementation", "# Task\n\n[FILL: description]\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			if issue.Level != LevelError {
				t.Errorf("implementation phase markers should be errors, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for the task file")
}

func TestValidateTemplateMarkers_PlanningPhaseReturnsWarning(t *testing.T) {
	dir := setupTemplateTestFestival(t, "planning", "# Task\n\n[FILL: description]\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			if issue.Level != LevelWarning {
				t.Errorf("planning phase markers should be warnings, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for the task file")
}

func TestValidateTemplateMarkers_ResearchPhaseReturnsWarning(t *testing.T) {
	dir := setupTemplateTestFestival(t, "research", "# Task\n\n[GUIDANCE: do something]\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			if issue.Level != LevelWarning {
				t.Errorf("research phase markers should be warnings, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for the task file")
}

func TestValidateTemplateMarkers_ReviewPhaseReturnsError(t *testing.T) {
	dir := setupTemplateTestFestival(t, "review", "# Task\n\n[REPLACE: fill this in]\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			if issue.Level != LevelError {
				t.Errorf("review phase markers should be errors, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for the task file")
}

func TestValidateTemplateMarkers_NonCodingActionPhaseReturnsError(t *testing.T) {
	dir := setupTemplateTestFestival(t, "non_coding_action", "# Task\n\n{{ variable }}\n")

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			if issue.Level != LevelError {
				t.Errorf("non_coding_action phase markers should be errors, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for the task file")
}

func TestValidateTemplateMarkers_SkipsCodeBlocks(t *testing.T) {
	content := "# Task\n\n```\n[FILL: inside code block]\n```\n\nReal content here.\n"
	dir := setupTemplateTestFestival(t, "implementation", content)

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			t.Error("markers inside code blocks should be skipped")
		}
	}
}

func TestValidateTemplateMarkers_SkipsInlineCode(t *testing.T) {
	content := "# Task\n\nUse `[FILL: example]` in your templates.\n\nReal content here.\n"
	dir := setupTemplateTestFestival(t, "implementation", content)

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == filepath.Join("001_PHASE", "01_seq", "01_task.md") {
			t.Error("markers inside inline code should be skipped")
		}
	}
}

func TestValidateTemplateMarkers_SkipsGatesDirectory(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644)

	gatesPath := filepath.Join(dir, "gates")
	_ = os.MkdirAll(gatesPath, 0755)
	_ = os.WriteFile(filepath.Join(gatesPath, "gate.md"), []byte("[FILL: intentional template]\n"), 0644)

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate {
			t.Errorf("markers in gates/ directory should be skipped, got issue: %+v", issue)
		}
	}
}

func TestValidateTemplateMarkers_DefaultsToImplementation(t *testing.T) {
	// Phase with no frontmatter → defaults to implementation → error level
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte("# Test\n"), 0644)

	phasePath := filepath.Join(dir, "001_PHASE")
	_ = os.MkdirAll(phasePath, 0755)
	// PHASE_GOAL.md without frontmatter
	_ = os.WriteFile(filepath.Join(phasePath, "PHASE_GOAL.md"), []byte("# Phase Goal\nNo frontmatter here.\n"), 0644)

	seqPath := filepath.Join(phasePath, "01_seq")
	_ = os.MkdirAll(seqPath, 0755)
	_ = os.WriteFile(filepath.Join(seqPath, "01_task.md"), []byte("[FILL: missing]\n"), 0644)

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate {
			if issue.Level != LevelError {
				t.Errorf("missing frontmatter should default to implementation (error), got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue")
}

func TestResolvePhaseType_FestivalRootFile(t *testing.T) {
	dir := t.TempDir()
	pt := resolvePhaseType(dir, "FESTIVAL_GOAL.md")
	if pt != "planning" {
		t.Errorf("festival-root file should resolve to planning, got %s", pt)
	}
}

func TestResolvePhaseType_FestivalOverviewFile(t *testing.T) {
	dir := t.TempDir()
	pt := resolvePhaseType(dir, "FESTIVAL_OVERVIEW.md")
	if pt != "planning" {
		t.Errorf("festival-root file should resolve to planning, got %s", pt)
	}
}

func TestResolvePhaseType_TodoFile(t *testing.T) {
	dir := t.TempDir()
	pt := resolvePhaseType(dir, "TODO.md")
	if pt != "planning" {
		t.Errorf("festival-root TODO.md should resolve to planning, got %s", pt)
	}
}

func TestValidateTemplateMarkers_FestivalRootFileReturnsWarning(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n\n[FILL: festival goal]\n"), 0644)

	issues, err := ValidateTemplateMarkers(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, issue := range issues {
		if issue.Code == CodeUnfilledTemplate && issue.Path == "FESTIVAL_GOAL.md" {
			if issue.Level != LevelWarning {
				t.Errorf("festival-root file markers should be warnings, got %s", issue.Level)
			}
			return
		}
	}
	t.Error("expected an unfilled_template issue for FESTIVAL_GOAL.md")
}

func TestCheckTemplatesFilled_DetectsReplace(t *testing.T) {
	dir := setupTemplateTestFestival(t, "implementation", "# Task\n\n[REPLACE: placeholder]\n")

	if CheckTemplatesFilled(dir) {
		t.Error("CheckTemplatesFilled should return false when [REPLACE:] markers exist")
	}
}

func TestCheckTemplatesFilled_PassesClean(t *testing.T) {
	dir := setupTemplateTestFestival(t, "implementation", "# Task\n\nClean content with no markers.\n")

	if !CheckTemplatesFilled(dir) {
		t.Error("CheckTemplatesFilled should return true when no markers exist")
	}
}
