package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func decodeStatusJSON(t *testing.T, nav *wf.Navigator) workflowStatusJSON {
	t.Helper()
	out, err := renderWorkflowStatusJSON(nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal status json: %v\noutput: %s", err, out)
	}
	return got
}

func TestWorkflowStatusJSON_Normal(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)

	got := decodeStatusJSON(t, nav)

	if got.SchemaVersion != workflowStatusSchema {
		t.Errorf("schema_version = %q, want %q", got.SchemaVersion, workflowStatusSchema)
	}
	if got.FestivalID != "TEST-001" {
		t.Errorf("festival_id = %q, want TEST-001", got.FestivalID)
	}
	if got.FestivalPath != dir {
		t.Errorf("festival_path = %q, want %q", got.FestivalPath, dir)
	}
	if got.Phase != "001_INGEST" {
		t.Errorf("phase = %q, want 001_INGEST", got.Phase)
	}
	if got.PhasePath != phaseDir {
		t.Errorf("phase_path = %q, want %q", got.PhasePath, phaseDir)
	}
	if got.Mode != "workflow" {
		t.Errorf("mode = %q, want workflow", got.Mode)
	}
	if got.Workflow != "001_INGEST" {
		t.Errorf("workflow = %q, want 001_INGEST", got.Workflow)
	}
	if got.Complete {
		t.Error("complete = true, want false for fresh workflow")
	}
	if got.TotalSteps != 3 {
		t.Errorf("total_steps = %d, want 3", got.TotalSteps)
	}
	if got.CurrentStep == nil || *got.CurrentStep != 1 {
		t.Errorf("current_step = %v, want 1", got.CurrentStep)
	}
	if got.WorkflowStep == nil || *got.WorkflowStep != "READ" {
		t.Errorf("workflow_step = %v, want READ", got.WorkflowStep)
	}
	if len(got.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(got.Steps))
	}
	if !got.Steps[0].IsCurrent {
		t.Error("step 1 is_current = false, want true")
	}
	if got.Steps[0].Name != "READ" {
		t.Errorf("step 1 name = %q, want READ", got.Steps[0].Name)
	}
	if got.Steps[0].HasCheckpoint {
		t.Error("step 1 has_checkpoint = true, want false")
	}
	// Step 2 (ANALYZE) declares USER APPROVAL REQUIRED.
	if !got.Steps[1].HasCheckpoint {
		t.Error("step 2 has_checkpoint = false, want true")
	}
}

func TestWorkflowStatusJSON_Complete(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 1: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 2: %v", err)
	}
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve checkpoint: %v", err)
	}
	if err := nav.Advance(ctx); err != nil && err != guidance.ErrAlreadyComplete {
		if !nav.GetWorkflowState().IsComplete() {
			t.Fatalf("advance step 3: %v", err)
		}
	}
	if !nav.GetWorkflowState().IsComplete() {
		t.Fatal("workflow should be complete before asserting JSON")
	}

	got := decodeStatusJSON(t, nav)

	if !got.Complete {
		t.Error("complete = false, want true")
	}
	if got.CurrentStep != nil {
		t.Errorf("current_step = %v, want null when complete", *got.CurrentStep)
	}
	if got.WorkflowStep != nil {
		t.Errorf("workflow_step = %v, want null when complete", *got.WorkflowStep)
	}
	for i, s := range got.Steps {
		if s.IsCurrent {
			t.Errorf("step %d is_current = true, want false when complete", i+1)
		}
	}
}

func TestWorkflowStatusJSON_GateMode(t *testing.T) {
	dir := setupGateOnlyFestival(t)
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator: %v", err)
	}

	got := decodeStatusJSON(t, nav)

	if got.Mode != "gate" {
		t.Errorf("mode = %q, want gate", got.Mode)
	}
	if got.Phase != "001_IMPLEMENT" {
		t.Errorf("phase = %q, want 001_IMPLEMENT", got.Phase)
	}
	if got.TotalSteps != 2 {
		t.Errorf("total_steps = %d, want 2", got.TotalSteps)
	}
}

func TestWorkflowStatusJSON_EmptyStepsSerializesArray(t *testing.T) {
	dir := t.TempDir()
	phaseDir := filepath.Join(dir, "001_EMPTY")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}
	nav := getNavigator(t, phaseDir) // no WORKFLOW.md/GATES.md => empty steps

	out, err := renderWorkflowStatusJSON(nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	// Empty steps must serialize as [] (not null) for stable consumers.
	if !strings.Contains(out, `"steps": []`) {
		t.Errorf("expected empty steps array, got:\n%s", out)
	}

	got := decodeStatusJSON(t, nav)
	if !got.Complete {
		t.Error("complete = false, want true for empty workflow")
	}
	if got.CurrentStep != nil {
		t.Errorf("current_step = %v, want null for empty workflow", *got.CurrentStep)
	}
	if got.WorkflowStep != nil {
		t.Error("workflow_step should be null for empty workflow")
	}
	if len(got.Steps) != 0 {
		t.Errorf("steps len = %d, want 0", len(got.Steps))
	}
}

// TestRunStatus_JSONAndText exercises the real command entrypoint for both
// modes: --json emits parseable structured output and default text mode still
// renders the human-readable status without leaking JSON.
func TestRunStatus_JSONAndText(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	ctx := context.Background()

	jsonOut := captureStdout(t, func() {
		if err := runStatus(ctx, true); err != nil {
			t.Fatalf("runStatus(json): %v", err)
		}
	})
	var parsed workflowStatusJSON
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("json mode did not emit valid JSON: %v\noutput: %s", err, jsonOut)
	}
	if parsed.SchemaVersion != workflowStatusSchema {
		t.Errorf("schema_version = %q, want %q", parsed.SchemaVersion, workflowStatusSchema)
	}

	textOut := captureStdout(t, func() {
		if err := runStatus(ctx, false); err != nil {
			t.Fatalf("runStatus(text): %v", err)
		}
	})
	if !strings.Contains(textOut, "Workflow Status") {
		t.Errorf("text mode missing human header, got:\n%s", textOut)
	}
	if strings.Contains(textOut, "schema_version") {
		t.Errorf("text mode leaked JSON: %s", textOut)
	}
}
