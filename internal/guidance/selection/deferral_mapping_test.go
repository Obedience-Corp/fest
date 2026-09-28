package selection

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/deps"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestUpdateTaskStatusesFromProgressCarriesTheDeferralFlag(t *testing.T) {
	ctx := t.Context()
	festivalPath := t.TempDir()

	const (
		completedTask = "001_PHASE/01_seq/01_task.md"
		deferredTask  = "001_PHASE/01_seq/02_task.md"
		openTask      = "001_PHASE/01_seq/03_task.md"
		wipTask       = "001_PHASE/01_seq/04_task.md"
		unknownTask   = "001_PHASE/01_seq/05_task.md"
	)

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, completedTask); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, deferredTask, "provider API removed", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, openTask, "still stuck", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.MarkInProgress(ctx, wipTask); err != nil {
		t.Fatalf("MarkInProgress() error = %v", err)
	}

	store := mgr.Store()
	store.QueueEvent(&progress.ProgressEvent{
		Timestamp:      time.Now().UTC(),
		Event:          progress.EventBlockerDeferred,
		Task:           deferredTask,
		DeferralReason: "the vendor replies next week",
	})
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	graph := deps.NewGraph()
	for _, id := range []string{completedTask, deferredTask, openTask, wipTask, unknownTask} {
		graph.AddTask(&deps.Task{ID: id, Path: filepath.Join(festivalPath, id)})
	}

	// A stale flag on a reused graph must not survive the refresh.
	if task, ok := graph.GetTask(openTask); ok {
		task.BlockerDeferred = true
	}

	selector := NewSelector(festivalPath)
	if err := selector.updateTaskStatusesFromProgress(ctx, graph); err != nil {
		t.Fatalf("updateTaskStatusesFromProgress() error = %v", err)
	}

	cases := []struct {
		id           string
		wantStatus   string
		wantDeferred bool
		wantSettled  bool
	}{
		{completedTask, "complete", false, true},
		{deferredTask, "blocked", true, true},
		{openTask, "blocked", false, false},
		{wipTask, "in_progress", false, false},
		{unknownTask, "pending", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			task, ok := graph.GetTask(tc.id)
			if !ok {
				t.Fatalf("task %q missing from the graph", tc.id)
			}
			if task.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", task.Status, tc.wantStatus)
			}
			if task.BlockerDeferred != tc.wantDeferred {
				t.Errorf("BlockerDeferred = %v, want %v", task.BlockerDeferred, tc.wantDeferred)
			}
			if got := task.IsSettled(); got != tc.wantSettled {
				t.Errorf("IsSettled() = %v, want %v", got, tc.wantSettled)
			}
		})
	}
}

func TestGetReadyTasksAfterRealStatusRefresh(t *testing.T) {
	ctx := t.Context()
	festivalPath := t.TempDir()

	const (
		blockedTask   = "001_PHASE/01_seq/01_task.md"
		dependentTask = "001_PHASE/01_seq/02_task.md"
	)

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, blockedTask, "provider API removed", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	build := func() *deps.Graph {
		graph := deps.NewGraph()
		first := &deps.Task{ID: blockedTask, Number: 1, Path: filepath.Join(festivalPath, blockedTask)}
		second := &deps.Task{ID: dependentTask, Number: 2, Path: filepath.Join(festivalPath, dependentTask)}
		graph.AddTask(first)
		graph.AddTask(second)
		graph.AddDependency(first, second, deps.DepImplicit, true)
		return graph
	}

	selector := NewSelector(festivalPath)

	before := build()
	if err := selector.updateTaskStatusesFromProgress(ctx, before); err != nil {
		t.Fatalf("updateTaskStatusesFromProgress() error = %v", err)
	}
	if got := before.GetReadyTasks(); len(got) != 0 {
		t.Fatalf("ready = %v before the deferral, want none", got)
	}

	store := mgr.Store()
	store.QueueEvent(&progress.ProgressEvent{
		Timestamp:      time.Now().UTC(),
		Event:          progress.EventBlockerDeferred,
		Task:           blockedTask,
		DeferralReason: "the vendor replies next week",
	})
	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	after := build()
	if err := selector.updateTaskStatusesFromProgress(ctx, after); err != nil {
		t.Fatalf("updateTaskStatusesFromProgress() error = %v", err)
	}
	ready := after.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != dependentTask {
		ids := make([]string, 0, len(ready))
		for _, task := range ready {
			ids = append(ids, task.ID)
		}
		t.Fatalf("ready = %v after the deferral, want only %s", ids, dependentTask)
	}
}
