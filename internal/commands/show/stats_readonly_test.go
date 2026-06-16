package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"gopkg.in/yaml.v3"
)

func TestCalculateFestivalStats_DoesNotMutate(t *testing.T) {
	festivalDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("name: stats-ro\nmetadata:\n  id: SR0001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	taskDir := filepath.Join(festivalDir, "001_PLAN", "01_seq")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "01_task.md"), []byte("# Task\n"), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	festDir := filepath.Join(festivalDir, progress.ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}
	legacyYAML := "festival: stats-ro\nupdated_at: 2026-01-15T10:00:00Z\ntasks:\n" +
		"  001_PLAN/01_seq/01_task.md:\n" +
		"    task_id: 001_PLAN/01_seq/01_task.md\n" +
		"    status: completed\n" +
		"    progress: 100\n" +
		"    started_at: 2026-01-15T09:00:00Z\n" +
		"    completed_at: 2026-01-15T09:30:00Z\n"
	legacyPath := filepath.Join(festDir, progress.ProgressFileName)
	if err := os.WriteFile(legacyPath, []byte(legacyYAML), 0o644); err != nil {
		t.Fatalf("write legacy progress: %v", err)
	}

	stats, err := CalculateFestivalStats(context.Background(), festivalDir)
	if err != nil {
		t.Fatalf("CalculateFestivalStats: %v", err)
	}
	if stats == nil {
		t.Fatal("stats is nil")
	}

	if _, err := os.Stat(legacyPath); err != nil {
		t.Errorf("legacy progress.yaml was removed by stats calculation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(festDir, progress.ProgressEventsFile)); !os.IsNotExist(err) {
		t.Error("stats calculation wrote progress_events.jsonl; it must not mutate disk")
	}
	if stats.Tasks.Total < 1 {
		t.Errorf("stats.Tasks.Total = %d, want >= 1 (read-only load must still report progress)", stats.Tasks.Total)
	}
}

func TestCalculateFestivalStats_WorkflowPhaseDoesNotMutate(t *testing.T) {
	festivalDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("name: stats-wf-ro\nmetadata:\n  id: SR0002\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	phaseDir := filepath.Join(festivalDir, "001_INGEST")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase dir: %v", err)
	}
	workflowMD := "# Ingest Workflow\n\n" +
		"## Step 1: Gather inputs\n\n**Goal:** collect specs\n\n" +
		"## Step 2: Synthesize\n\n**Goal:** produce output\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "WORKFLOW.md"), []byte(workflowMD), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	festDir := filepath.Join(festivalDir, progress.ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	started := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	completed := started.Add(30 * time.Minute)
	state := &wf.FestivalWorkflowState{
		Version:   1,
		CreatedAt: started,
		UpdatedAt: completed,
		Phases: map[string]*wf.WorkflowState{
			"001_INGEST": {
				CurrentStep: 2,
				TotalSteps:  2,
				CreatedAt:   started,
				UpdatedAt:   completed,
				Steps: map[int]*wf.StepState{
					1: {Number: 1, Status: wf.StepStatusCompleted, StartedAt: &started, CompletedAt: &completed},
					2: {Number: 2, Status: wf.StepStatusInProgress, StartedAt: &completed},
				},
			},
		},
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("marshal workflow state: %v", err)
	}
	workflowYAML := filepath.Join(festDir, wf.StateFileName)
	if err := os.WriteFile(workflowYAML, data, 0o644); err != nil {
		t.Fatalf("write workflow_state.yaml: %v", err)
	}

	if _, err := CalculateFestivalStats(context.Background(), festivalDir); err != nil {
		t.Fatalf("CalculateFestivalStats: %v", err)
	}

	if _, err := os.Stat(workflowYAML); err != nil {
		t.Errorf("workflow_state.yaml was consumed by stats calculation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(festDir, progress.ProgressEventsFile)); !os.IsNotExist(err) {
		t.Error("stats calculation wrote progress_events.jsonl for a workflow phase; it must not mutate disk")
	}
}
