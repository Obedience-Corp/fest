package show

import (
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
)

func TestBuildStandaloneStepViewsAnonymousMarksFirstStepCurrent(t *testing.T) {
	steps := []wf.WorkflowStep{
		{Number: 1, Name: "First", Goal: "Start"},
		{Number: 2, Name: "Second"},
	}

	views, current, completed, renderMode := buildStandaloneStepViews(steps, nil)

	if current != 1 {
		t.Fatalf("current = %d, want 1", current)
	}
	if completed != 0 {
		t.Fatalf("completed = %d, want 0", completed)
	}
	if renderMode != "next_up" {
		t.Fatalf("renderMode = %q, want next_up", renderMode)
	}
	if !views[0].IsCurrent {
		t.Fatal("first step should be marked current")
	}
	if views[0].Status != wf.StepStatusPending {
		t.Fatalf("first status = %q, want pending", views[0].Status)
	}
	if views[1].Status != wf.StepStatusPending {
		t.Fatalf("second status = %q, want pending", views[1].Status)
	}
}

func TestBuildStandaloneStepViewsTrackedRunProgress(t *testing.T) {
	steps := []wf.WorkflowStep{
		{Number: 1, Name: "First"},
		{Number: 2, Name: "Second"},
		{Number: 3, Name: "Third"},
	}
	state := &localstore.RunState{
		Status:         "active",
		CurrentStep:    2,
		CompletedSteps: 1,
	}

	views, current, completed, renderMode := buildStandaloneStepViews(steps, state)

	if current != 2 {
		t.Fatalf("current = %d, want 2", current)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, want 1", completed)
	}
	if renderMode != "in_progress" {
		t.Fatalf("renderMode = %q, want in_progress", renderMode)
	}
	if views[0].Status != wf.StepStatusCompleted {
		t.Fatalf("step 1 status = %q, want completed", views[0].Status)
	}
	if views[1].Status != wf.StepStatusInProgress || !views[1].IsCurrent {
		t.Fatalf("step 2 = status %q current %v, want in_progress/current", views[1].Status, views[1].IsCurrent)
	}
	if views[2].Status != wf.StepStatusPending {
		t.Fatalf("step 3 status = %q, want pending", views[2].Status)
	}
}

func TestFormatStandaloneWorkflowProgressIncludesStepsAndProgress(t *testing.T) {
	steps := []wf.WorkflowStep{
		{Number: 1, Name: "First", Goal: "Start"},
		{Number: 2, Name: "Second"},
	}
	views, current, completed, renderMode := buildStandaloneStepViews(steps, nil)
	info := &StandaloneWorkflowInfo{
		Mode:           "standalone-anonymous",
		WorkflowDoc:    "/tmp/demo/WORKFLOW.md",
		RunStatus:      "not-started",
		CurrentStep:    current,
		TotalSteps:     len(steps),
		CompletedSteps: completed,
		RenderMode:     renderMode,
		Steps:          views,
	}

	got := FormatStandaloneWorkflowProgress(info)
	for _, want := range []string{
		"Workflow Progress",
		"Progress: 0/2 (0%) steps",
		"Step 1: First",
		"Step 2: Second",
		"Current: Step 1 - First",
		"fest workflow advance",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted output missing %q:\n%s", want, got)
		}
	}
}
