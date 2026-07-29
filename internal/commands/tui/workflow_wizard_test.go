package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWorkflowStepsText(t *testing.T) {
	t.Parallel()
	steps, err := parseWorkflowStepsText("Review|Check the PR\n\nShip|Tag and release\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("len=%d want 2", len(steps))
	}
	if steps[0].Name != "Review" || steps[0].Goal != "Check the PR" {
		t.Fatalf("step0=%+v", steps[0])
	}
	if steps[1].Name != "Ship" {
		t.Fatalf("step1=%+v", steps[1])
	}
}

func TestParseWorkflowStepsTextDefaultGoal(t *testing.T) {
	t.Parallel()
	steps, err := parseWorkflowStepsText("Only name")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Name != "Only name" || steps[0].Goal == "" {
		t.Fatalf("got %+v", steps)
	}
}

func TestParseWorkflowStepsTextEmptyGoalAfterPipe(t *testing.T) {
	t.Parallel()
	steps, err := parseWorkflowStepsText("Name|")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Name != "Name" || steps[0].Goal != "Describe this step." {
		t.Fatalf("got %+v", steps)
	}
	// Whitespace-only goal also defaults.
	steps, err = parseWorkflowStepsText("Name|  ")
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Goal != "Describe this step." {
		t.Fatalf("got %+v", steps[0])
	}
}

func TestParseWorkflowStepsTextEmptyName(t *testing.T) {
	t.Parallel()
	if _, err := parseWorkflowStepsText("|goal"); err == nil {
		t.Fatal("expected error for empty name after |")
	}
	if _, err := parseWorkflowStepsText("  |x"); err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
}

func TestParseWorkflowStepsTextEmpty(t *testing.T) {
	t.Parallel()
	if _, err := parseWorkflowStepsText("  \n\n"); err == nil {
		t.Fatal("expected error for empty steps")
	}
}

func TestWorkflowStepsJSON(t *testing.T) {
	t.Parallel()
	raw, err := workflowStepsJSON(&workflowDraft{
		Name:        "demo",
		Title:       "Demo flow",
		Description: "thin start",
		StepsText:   "One|Do the thing\nTwo|Finish",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"title":"Demo flow"`, `"name":"One"`, `"goal":"Do the thing"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("json missing %s: %s", want, raw)
		}
	}
}

func TestWorkflowInputFromDraftDefaultsTitle(t *testing.T) {
	t.Parallel()
	in, err := workflowInputFromDraft(&workflowDraft{
		Name:      "my-flow",
		StepsText: "Start|Go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "my-flow" {
		t.Fatalf("title=%q", in.Title)
	}
}

func TestWorkflowInputFromDraftMissingName(t *testing.T) {
	t.Parallel()
	if _, err := workflowInputFromDraft(nil); err == nil {
		t.Fatal("expected error for nil draft")
	}
	if _, err := workflowInputFromDraft(&workflowDraft{StepsText: "A|B"}); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestWorkflowConfirmSummary(t *testing.T) {
	t.Parallel()
	s := workflowConfirmSummary(&workflowDraft{
		Name:      "demo",
		StepsText: "A|B",
	})
	if !strings.Contains(s, "demo") || !strings.Contains(s, "A — B") {
		t.Fatalf("summary=%q", s)
	}
}

func TestRefuseExistingStandaloneWorkflow(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	if err := refuseExistingStandaloneWorkflow(empty); err != nil {
		t.Fatalf("empty dir should be ok: %v", err)
	}

	withDoc := t.TempDir()
	if err := os.WriteFile(filepath.Join(withDoc, "WORKFLOW.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := refuseExistingStandaloneWorkflow(withDoc)
	if err == nil {
		t.Fatal("expected WORKFLOW.md refusal")
	}
	if !strings.Contains(err.Error(), "WORKFLOW.md") {
		t.Fatalf("err=%v", err)
	}

	withRuntime := t.TempDir()
	if err := os.Mkdir(filepath.Join(withRuntime, ".workflow"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = refuseExistingStandaloneWorkflow(withRuntime)
	if err == nil {
		t.Fatal("expected .workflow/ refusal")
	}
	if !strings.Contains(err.Error(), ".workflow") {
		t.Fatalf("err=%v", err)
	}
}

func TestDefaultWorkflowStepsTextParses(t *testing.T) {
	t.Parallel()
	steps, err := parseWorkflowStepsText(defaultWorkflowStepsText)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 {
		t.Fatalf("len=%d want 3", len(steps))
	}
}
