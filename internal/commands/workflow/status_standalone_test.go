package workflow

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
)

func snapshotRuntime(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(dir, ".workflow"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot runtime: %v", err)
	}
	return files
}

func assertRuntimeUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf(".workflow file set changed: before %d files, after %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf(".workflow file modified by status: %s", path)
		}
	}
}

func runStatusInDir(t *testing.T, dir string, jsonOutput bool) (string, error) {
	t.Helper()
	t.Chdir(dir)
	cmd := newStatusCmd()
	cmd.SetContext(context.Background())
	if jsonOutput {
		cmd.SetArgs([]string{"--json"})
	} else {
		cmd.SetArgs([]string{})
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.Execute()
	})
	return out, runErr
}

func TestWorkflowStatusStandaloneTrackedJSON(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	ctx := context.Background()
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone}); err != nil {
		t.Fatal(err)
	}
	before := snapshotRuntime(t, res.StartDir)

	out, err := runStatusInDir(t, res.StartDir, true)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.SchemaVersion != workflowStatusSchema {
		t.Errorf("schema = %q", got.SchemaVersion)
	}
	if got.Mode != "standalone-tracked" {
		t.Errorf("mode = %q", got.Mode)
	}
	if got.TotalSteps != 2 || got.Complete {
		t.Errorf("total=%d complete=%v", got.TotalSteps, got.Complete)
	}
	if got.CurrentStep == nil || *got.CurrentStep != 2 {
		t.Errorf("current_step = %v, want 2", got.CurrentStep)
	}
	if got.WorkflowStep == nil || *got.WorkflowStep != "DO" {
		t.Errorf("workflow_step = %v, want DO", got.WorkflowStep)
	}
	if got.CompletedSteps == nil || *got.CompletedSteps != 1 {
		t.Errorf("completed_steps = %v, want 1", got.CompletedSteps)
	}
	if got.RunID == "" || got.RunStatus != "active" {
		t.Errorf("run = %q (%s)", got.RunID, got.RunStatus)
	}
	if got.Blocked {
		t.Error("blocked = true, want false")
	}
	if len(got.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(got.Steps))
	}
	if got.Steps[0].Status != "completed" || got.Steps[0].IsCurrent {
		t.Errorf("step 1 = %+v", got.Steps[0])
	}
	if got.Steps[1].Status != "pending" || !got.Steps[1].IsCurrent {
		t.Errorf("step 2 = %+v", got.Steps[1])
	}
	assertRuntimeUnchanged(t, before, snapshotRuntime(t, res.StartDir))
}

func TestWorkflowStatusStandaloneTrackedText(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	if err := store.AppendEvent(context.Background(), localstore.Event{EventType: localstore.EventStepStart}); err != nil {
		t.Fatal(err)
	}
	before := snapshotRuntime(t, res.StartDir)

	out, err := runStatusInDir(t, res.StartDir, false)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	for _, want := range []string{"Workflow Status", "Mode: tracked", "Current Step: 1 of 2", "PLAN", "DO", "Progress:"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
	assertRuntimeUnchanged(t, before, snapshotRuntime(t, res.StartDir))
}

func TestWorkflowStatusStandaloneBlocked(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	ctx := context.Background()
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepBlock}); err != nil {
		t.Fatal(err)
	}
	before := snapshotRuntime(t, res.StartDir)

	out, err := runStatusInDir(t, res.StartDir, true)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if !got.Blocked {
		t.Error("blocked = false, want true")
	}
	if len(got.Steps) != 2 || got.Steps[0].Status != "blocked" || !got.Steps[0].IsCurrent {
		t.Errorf("steps = %+v", got.Steps)
	}

	text, err := runStatusInDir(t, res.StartDir, false)
	if err != nil {
		t.Fatalf("runStatus text: %v", err)
	}
	if !strings.Contains(text, "Blocked:") {
		t.Errorf("text output missing blocked state:\n%s", text)
	}
	assertRuntimeUnchanged(t, before, snapshotRuntime(t, res.StartDir))
}

func TestWorkflowStatusStandaloneComplete(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
			t.Fatal(err)
		}
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted}); err != nil {
		t.Fatal(err)
	}
	if active, err := store.LoadActive(ctx); err != nil || active != nil {
		t.Fatalf("after run completion LoadActive = %+v, %v; want no active run", active, err)
	}
	before := snapshotRuntime(t, res.StartDir)

	out, err := runStatusInDir(t, res.StartDir, true)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if !got.Complete || got.CurrentStep != nil || got.WorkflowStep != nil {
		t.Errorf("complete=%v current=%v step=%v", got.Complete, got.CurrentStep, got.WorkflowStep)
	}
	if got.RunStatus != "completed" || got.CompletedSteps == nil || *got.CompletedSteps != 2 {
		t.Errorf("run_status=%q completed_steps=%v, want completed and 2", got.RunStatus, got.CompletedSteps)
	}
	for _, s := range got.Steps {
		if s.Status != "completed" || s.IsCurrent {
			t.Errorf("step = %+v", s)
		}
	}

	text, err := runStatusInDir(t, res.StartDir, false)
	if err != nil {
		t.Fatalf("runStatus text: %v", err)
	}
	if !strings.Contains(text, "All steps complete") {
		t.Errorf("text output missing completion:\n%s", text)
	}
	assertRuntimeUnchanged(t, before, snapshotRuntime(t, res.StartDir))
}

func TestWorkflowStatusNoWorkflowStillErrors(t *testing.T) {
	out, err := runStatusInDir(t, t.TempDir(), true)
	if err == nil {
		t.Fatalf("expected error outside any workflow, got output:\n%s", out)
	}
}

func TestWorkflowStatusLinkedProjectOutsideCampUsesCampRoot(t *testing.T) {
	campRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(campRoot, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	activeDir := filepath.Join(campRoot, "festivals", "active")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	festivalPath := filepath.Join(activeDir, "test-festival")
	if err := os.Rename(setupWorkflowFestival(t), festivalPath); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	t.Setenv("CAMP_ROOT", campRoot)

	nav, err := navigation.LoadNavigation()
	if err != nil {
		t.Fatalf("load navigation: %v", err)
	}
	nav.SetLinkWithPath("test-festival", project, festivalPath)
	if err := nav.Save(); err != nil {
		t.Fatalf("save navigation: %v", err)
	}

	out, err := runStatusInDir(t, project, true)
	if err != nil {
		t.Fatalf("status from a linked project outside the camp tree: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.WorkflowDoc != "" || got.RunID != "" {
		t.Errorf("reported a standalone run, want the linked festival: %+v", got)
	}
	if got.TotalSteps == 0 {
		t.Errorf("festival workflow status has no steps:\n%s", out)
	}
}
