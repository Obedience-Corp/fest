package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"gopkg.in/yaml.v3"
)

const readonlyLegacyYAML = `festival: test-festival
updated_at: 2026-01-15T10:00:00Z
tasks:
  001_PHASE/01_seq/01_task.md:
    task_id: 001_PHASE/01_seq/01_task.md
    status: completed
    progress: 100
    started_at: 2026-01-15T09:00:00Z
    completed_at: 2026-01-15T09:30:00Z
    time_spent_minutes: 30
  001_PHASE/01_seq/02_task.md:
    task_id: 001_PHASE/01_seq/02_task.md
    status: in_progress
    progress: 40
    started_at: 2026-01-15T09:45:00Z
`

func writeReadonlyFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	festDir := filepath.Join(dir, ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(festDir, ProgressFileName), []byte(readonlyLegacyYAML), 0o644); err != nil {
		t.Fatalf("write legacy yaml: %v", err)
	}

	started := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	completed := started.Add(time.Hour)
	state := &wf.FestivalWorkflowState{
		Version:   1,
		CreatedAt: started,
		UpdatedAt: completed,
		Phases: map[string]*wf.WorkflowState{
			"001_PHASE": {
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
	if err := os.WriteFile(filepath.Join(festDir, wf.StateFileName), data, 0o644); err != nil {
		t.Fatalf("write workflow yaml: %v", err)
	}

	return dir
}

func TestLoadReadOnly_DoesNotMutateLegacyFiles(t *testing.T) {
	dir := writeReadonlyFixture(t)
	festDir := filepath.Join(dir, ProgressDir)

	store := NewStore(dir)
	if err := store.LoadReadOnly(context.Background()); err != nil {
		t.Fatalf("LoadReadOnly: %v", err)
	}

	if !fileExists(filepath.Join(festDir, ProgressFileName)) {
		t.Error("legacy progress.yaml was removed by LoadReadOnly")
	}
	if !fileExists(filepath.Join(festDir, wf.StateFileName)) {
		t.Error("workflow_state.yaml was removed by LoadReadOnly")
	}
	if fileExists(filepath.Join(festDir, ProgressEventsFile)) {
		t.Error("LoadReadOnly wrote progress_events.jsonl; it must not mutate disk")
	}

	task, ok := store.GetTask("001_PHASE/01_seq/01_task.md")
	if !ok || task.Status != StatusCompleted {
		t.Errorf("task status = %+v, want completed", task)
	}
}

func TestLoadReadOnly_MatchesMigratingLoad(t *testing.T) {
	roDir := writeReadonlyFixture(t)
	migDir := writeReadonlyFixture(t)

	roStore := NewStore(roDir)
	if err := roStore.LoadReadOnly(context.Background()); err != nil {
		t.Fatalf("LoadReadOnly: %v", err)
	}

	migStore := NewStore(migDir)
	if err := migStore.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, id := range []string{"001_PHASE/01_seq/01_task.md", "001_PHASE/01_seq/02_task.md"} {
		ro, okRO := roStore.GetTask(id)
		mig, okMig := migStore.GetTask(id)
		if okRO != okMig {
			t.Fatalf("task %s presence: read-only=%v migrating=%v", id, okRO, okMig)
		}
		if ro.Status != mig.Status || ro.Progress != mig.Progress {
			t.Errorf("task %s: read-only (%s,%d) != migrating (%s,%d)", id, ro.Status, ro.Progress, mig.Status, mig.Progress)
		}
	}

	roPhase, okRO := roStore.WorkflowPhaseState("001_PHASE")
	migPhase, okMig := migStore.WorkflowPhaseState("001_PHASE")
	if okRO != okMig {
		t.Fatalf("workflow phase presence: read-only=%v migrating=%v", okRO, okMig)
	}
	if okRO {
		if roPhase.TotalSteps != migPhase.TotalSteps {
			t.Errorf("workflow TotalSteps: read-only=%d migrating=%d", roPhase.TotalSteps, migPhase.TotalSteps)
		}
		for i := 1; i <= migPhase.TotalSteps; i++ {
			rss := roPhase.GetStepState(i)
			mss := migPhase.GetStepState(i)
			if (rss == nil) != (mss == nil) {
				t.Errorf("step %d presence: read-only=%v migrating=%v", i, rss != nil, mss != nil)
				continue
			}
			if rss != nil && rss.Status != mss.Status {
				t.Errorf("step %d status: read-only=%s migrating=%s", i, rss.Status, mss.Status)
			}
		}
	}

	if fileExists(filepath.Join(migDir, ProgressDir, wf.StateFileName)) {
		t.Error("migrating Load should have consumed workflow_state.yaml")
	}
	if !fileExists(filepath.Join(roDir, ProgressDir, wf.StateFileName)) {
		t.Error("read-only load must preserve workflow_state.yaml")
	}
}

func TestLoadReadOnly_PreservesTimestampFreeLegacyTasks(t *testing.T) {
	dir := t.TempDir()
	festDir := filepath.Join(dir, ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := "festival: legacy-no-ts\nupdated_at: 2026-01-15T10:00:00Z\ntasks:\n" +
		"  001_PHASE/01_seq/01_task.md:\n" +
		"    task_id: 001_PHASE/01_seq/01_task.md\n" +
		"    status: completed\n" +
		"    progress: 100\n" +
		"  001_PHASE/01_seq/02_task.md:\n" +
		"    task_id: 001_PHASE/01_seq/02_task.md\n" +
		"    status: in_progress\n" +
		"    progress: 50\n"
	if err := os.WriteFile(filepath.Join(festDir, ProgressFileName), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	store := NewStore(dir)
	if err := store.LoadReadOnly(context.Background()); err != nil {
		t.Fatalf("LoadReadOnly: %v", err)
	}

	completed, ok := store.GetTask("001_PHASE/01_seq/01_task.md")
	if !ok || completed.Status != StatusCompleted {
		t.Errorf("completed timestamp-free task lost: ok=%v task=%+v", ok, completed)
	}
	inProgress, ok := store.GetTask("001_PHASE/01_seq/02_task.md")
	if !ok || inProgress.Status != StatusInProgress {
		t.Errorf("in-progress timestamp-free task lost: ok=%v task=%+v", ok, inProgress)
	}
}
