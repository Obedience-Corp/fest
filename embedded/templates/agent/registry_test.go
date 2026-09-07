package agent

import (
	"strings"
	"testing"
)

// TestAllTemplatesRender verifies that every registered template can be
// rendered with minimal test data without producing errors.
// This is a contract test - it verifies templates work, not what they output.
func TestAllTemplatesRender(t *testing.T) {
	templates := List()
	if len(templates) == 0 {
		t.Fatal("no templates registered")
	}

	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			data := minimalTestData(name)
			output, err := Render(name, data)
			if err != nil {
				t.Fatalf("Render(%q) failed: %v", name, err)
			}

			// Contract checks - output should not contain unrendered tokens
			assertNoForbiddenTokens(t, name, output)
		})
	}
}

// TestTemplateRegistry verifies the registry is properly initialized.
func TestTemplateRegistry(t *testing.T) {
	t.Run("has templates", func(t *testing.T) {
		if len(Registry) == 0 {
			t.Fatal("Registry should not be empty")
		}
	})

	t.Run("Get returns nil for unknown", func(t *testing.T) {
		if Get("nonexistent/template") != nil {
			t.Error("Get should return nil for unknown template")
		}
	})

	t.Run("MustGet panics for unknown", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustGet should panic for unknown template")
			}
		}()
		MustGet("nonexistent/template")
	})

	t.Run("ListByPrefix filters correctly", func(t *testing.T) {
		nextTemplates := ListByPrefix("next/")
		for _, name := range nextTemplates {
			if !strings.HasPrefix(name, "next/") {
				t.Errorf("ListByPrefix returned %q which doesn't have prefix 'next/'", name)
			}
		}
	})
}

// TestRenderError verifies Render returns proper errors.
func TestRenderError(t *testing.T) {
	t.Run("unknown template", func(t *testing.T) {
		_, err := Render("nonexistent/template", nil)
		if err == nil {
			t.Error("Render should return error for unknown template")
		}
	})

	t.Run("MustRender panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustRender should panic on error")
			}
		}()
		MustRender("nonexistent/template", nil)
	})
}

// forbiddenTokens are markers that indicate template rendering problems.
var forbiddenTokens = []string{
	"{{.",        // Unrendered template variable
	"{{",         // Any unrendered template syntax
	"<no value>", // Go template's default for missing values
}

// assertNoForbiddenTokens checks that output doesn't contain problematic tokens.
func assertNoForbiddenTokens(t *testing.T, name, output string) {
	t.Helper()
	for _, token := range forbiddenTokens {
		if strings.Contains(output, token) {
			t.Errorf("template %q output contains forbidden token %q", name, token)
		}
	}
}

// minimalTestData returns the minimum data needed to render each template type.
// Data is designed to satisfy template requirements without being fragile
// to content changes.
func minimalTestData(templateName string) any {
	// Common fields used across many templates
	commonFields := map[string]any{
		"Header":            "Test Header",
		"ProgressLine":      "Progress: 50%",
		"PositionSection":   "Position: Phase 1, Sequence 1",
		"ContextSection":    "Context: test-festival",
		"ProgressCmd":       "fest task completed",
		"Message":           "Test message",
		"ActionInstruction": "Follow the instructions",
	}

	switch {
	case strings.HasPrefix(templateName, "implementation/"):
		return implementationData(templateName, commonFields)
	case strings.HasPrefix(templateName, "plan/"):
		return planData(commonFields)
	case strings.HasPrefix(templateName, "research/"):
		return researchData(commonFields)
	case strings.HasPrefix(templateName, "review/"):
		return reviewData(commonFields)
	case strings.HasPrefix(templateName, "action/"):
		return actionData(commonFields)
	case strings.HasPrefix(templateName, "ingest/"):
		return ingestData(commonFields)
	case strings.HasPrefix(templateName, "next/"):
		return nextData(templateName, commonFields)
	case strings.HasPrefix(templateName, "validate/"):
		return validateData(templateName, commonFields)
	case strings.HasPrefix(templateName, "workflow/"):
		return workflowData(templateName, commonFields)
	case strings.HasPrefix(templateName, "lifecycle/"):
		return lifecycleData()
	default:
		// Fallback: return common fields for any unknown template
		return commonFields
	}
}

func implementationData(name string, common map[string]any) map[string]any {
	switch {
	case strings.HasSuffix(name, "instructions"):
		return map[string]any{
			"Header":            common["Header"],
			"ActionInstruction": common["ActionInstruction"],
			"TasksSection":      "Tasks:\n  - Task 1\n  - Task 2",
			"ProgressLine":      common["ProgressLine"],
			"PositionSection":   common["PositionSection"],
			"ContextSection":    common["ContextSection"],
			"ProgressCmd":       common["ProgressCmd"],
		}
	case strings.HasSuffix(name, "complete"):
		return map[string]any{
			"Header":  common["Header"],
			"Message": common["Message"],
		}
	case strings.HasSuffix(name, "dry_run"):
		return map[string]any{
			"Header":         common["Header"],
			"FestivalLine":   "Festival: test-fest",
			"PathLine":       "Path: /test/path",
			"SummarySection": "Summary:\n  Phases: 2\n  Tasks: 10",
			"PhasesSection":  "Phase 001_PLANNING\n  ...",
		}
	}
	return common
}

func planData(common map[string]any) map[string]any {
	return map[string]any{
		"Header":            common["Header"],
		"ObjectivesSection": "Objectives:\n  - Define scope\n  - Set goals",
		"ProgressLine":      common["ProgressLine"],
		"PositionSection":   common["PositionSection"],
		"ContextSection":    common["ContextSection"],
		"ProgressCmd":       common["ProgressCmd"],
	}
}

func researchData(common map[string]any) map[string]any {
	return map[string]any{
		"Header":          common["Header"],
		"TopicSection":    "Topic: Test research topic",
		"ProgressLine":    common["ProgressLine"],
		"PositionSection": common["PositionSection"],
		"ContextSection":  common["ContextSection"],
		"ProgressCmd":     common["ProgressCmd"],
	}
}

func reviewData(common map[string]any) map[string]any {
	return map[string]any{
		"Header":           common["Header"],
		"ChecklistSection": "Checklist:\n  - [ ] Item 1\n  - [ ] Item 2",
		"ProgressLine":     common["ProgressLine"],
		"PositionSection":  common["PositionSection"],
		"ContextSection":   common["ContextSection"],
		"ProgressCmd":      common["ProgressCmd"],
	}
}

func actionData(common map[string]any) map[string]any {
	return map[string]any{
		"Header":          common["Header"],
		"ActionSection":   "Actions:\n  1. Step one\n  2. Step two",
		"ProgressLine":    common["ProgressLine"],
		"PositionSection": common["PositionSection"],
		"ContextSection":  common["ContextSection"],
		"ProgressCmd":     common["ProgressCmd"],
	}
}

func ingestData(common map[string]any) map[string]any {
	return map[string]any{
		"Header":          common["Header"],
		"InputSection":    "Input: test data source",
		"ProgressLine":    common["ProgressLine"],
		"PositionSection": common["PositionSection"],
		"ContextSection":  common["ContextSection"],
		"ProgressCmd":     common["ProgressCmd"],
	}
}

func lifecycleData() map[string]any {
	return map[string]any{
		"RitualName": "daily-ritual-RI-DR0001",
		"Reason":     "fest next",
	}
}

func nextData(name string, common map[string]any) map[string]any {
	switch name {
	case "next/verbose_task":
		return map[string]any{
			"Header":                 common["Header"],
			"TaskDetailsSection":     "## Task Details\nTask: test_task\nNumber: 1\nPath: /test/task.md",
			"LocationSection":        "## Location\nPhase: 001_PLANNING\nSequence: 01_setup",
			"PropertiesSection":      "",
			"DependenciesSection":    "",
			"RecommendationSection":  "## Recommendation\nNext task in sequence",
			"ParallelTasksSection":   "",
			"CurrentLocationSection": "## Current Location\nFestival: test-fest",
			"TaskContentSection":     "",
		}
	case "next/verbose_complete":
		return map[string]any{
			"Header":     common["Header"],
			"Message":    "All tasks completed.",
			"Congrats":   "Congratulations!",
			"ReasonLine": "Reason: All tasks done",
		}
	case "next/verbose_no_task":
		return map[string]any{
			"Header":          common["Header"],
			"ReasonLine":      "Reason: No tasks available",
			"LocationSection": "Location:\n  Festival: test\n  Phase: 001",
		}
	case "next/planning":
		return map[string]any{
			"Header":              common["Header"],
			"PhaseInfoSection":    "Phase: 001_PLAN\nType: planning",
			"ObjectivesSection":   "## Objectives\n  - Define scope",
			"NoObjectivesSection": "",
			"NextStepsSection":    "## Suggested Actions\n  - Explore the problem space",
		}
	case "next/festival_planning":
		return map[string]any{
			"Header":           common["Header"],
			"InfoSection":      "Status planning\nPhases 0",
			"MarkerSection":    "Unfilled Markers\n  FESTIVAL_GOAL.md (7)",
			"NextStepsSection": "Next Step\n  1. Fill the markers: fest wizard fill .",
		}
	case "next/gate":
		return map[string]any{
			"Header":          common["Header"],
			"TaskLine":        "Task: 05_testing",
			"PathLine":        "Path: 001_IMPL/01_core/05_testing.md",
			"TypeLine":        "Type: gate (testing)",
			"GateContent":     "- [ ] All tests pass\n- [ ] Coverage meets 90%",
			"FallbackMessage": "",
			"CompletionHint":  "When complete, run: fest task completed",
		}
	case "next/task":
		return map[string]any{
			"Header":             common["Header"],
			"ProgressLine":       common["ProgressLine"],
			"SequenceLine":       "Sequence: 01_setup",
			"PhaseLine":          "Phase: 001_PLANNING",
			"AutonomyLine":       "Autonomy: high",
			"TaskLine":           "Task: test_task",
			"ActionInstruction":  common["ActionInstruction"],
			"PathLine":           "Path: /test/task.md",
			"RecommendationLine": "Recommendation: Next task",
			"ParallelSection":    "",
			"ProgressCmd":        common["ProgressCmd"],
			"ContextSection":     common["ContextSection"],
		}
	case "next/blocked":
		return map[string]any{
			"Header":          common["Header"],
			"PhaseLine":       "Phase: 001_PLANNING",
			"TypeLine":        "Type: review",
			"DescriptionLine": "Description: Quality gate required",
			"CriteriaSection": "Criteria:\n  - Tests pass\n  - Code reviewed",
		}
	case "next/complete":
		return map[string]any{
			"Header":     common["Header"],
			"Message":    "Festival complete!",
			"ReasonLine": "Reason: All tasks done",
		}
	case "next/no_task":
		return map[string]any{
			"Header":          common["Header"],
			"ReasonLine":      "Reason: No tasks available",
			"LocationSection": "Location:\n  Festival: test\n  Phase: 001",
		}
	}
	return common
}

func validateData(name string, common map[string]any) map[string]any {
	switch {
	case strings.HasSuffix(name, "markers"):
		return map[string]any{
			"Header": common["Header"],
			"Markers": []map[string]string{
				{"Marker": "NN_task", "File": "task.md"},
				{"Marker": "REPLACE", "File": "doc.md"},
			},
		}
	case strings.HasSuffix(name, "reflection"):
		return map[string]any{
			"Header": common["Header"],
			"Criteria": []string{
				"All tests pass",
				"Code compiles",
				"Documentation updated",
			},
		}
	}
	return common
}

func workflowData(name string, common map[string]any) map[string]any {
	// Common workflow fields
	workflowCommon := map[string]any{
		"PhaseType":   "ingest",
		"PhaseName":   "001_INGEST",
		"StepNumber":  2,
		"StepName":    "ANALYZE",
		"Goal":        "Analyze input data and extract key patterns",
		"Actions":     []string{"Review input files", "Identify patterns"},
		"Output":      "analysis_notes.md",
		"IsBlocking":  false,
		"Status":      "in_progress",
		"Feedback":    "",
		"TotalSteps":  5,
		"CurrentStep": 2,
	}

	switch {
	case strings.HasSuffix(name, "step"):
		return workflowCommon
	case strings.HasSuffix(name, "checkpoint"):
		return map[string]any{
			"StepNumber": workflowCommon["StepNumber"],
			"StepName":   workflowCommon["StepName"],
		}
	case strings.HasSuffix(name, "complete"):
		return map[string]any{
			"PhaseType":  workflowCommon["PhaseType"],
			"TotalSteps": workflowCommon["TotalSteps"],
			"Steps": []map[string]any{
				{"Number": 1, "Name": "REVIEW"},
				{"Number": 2, "Name": "ANALYZE"},
			},
		}
	case strings.HasSuffix(name, "progress"):
		return map[string]any{
			"PhaseName":     workflowCommon["PhaseName"],
			"PhaseType":     workflowCommon["PhaseType"],
			"Completed":     1,
			"Total":         5,
			"CurrentStep":   2,
			"CurrentStatus": "in progress",
			"Steps": []map[string]any{
				{"Number": 1, "Name": "REVIEW", "Status": "completed", "IsBlocking": false},
				{"Number": 2, "Name": "ANALYZE", "Status": "in_progress", "IsBlocking": true},
			},
		}
	}
	return common
}
