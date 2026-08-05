package deps

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g.Tasks == nil {
		t.Error("Tasks map should not be nil")
	}
	if g.Edges == nil {
		t.Error("Edges slice should not be nil")
	}
}

func TestGraph_AddTask(t *testing.T) {
	g := NewGraph()
	task := &Task{ID: "test-task", Name: "Test Task", Number: 1}

	g.AddTask(task)

	if len(g.Tasks) != 1 {
		t.Errorf("Tasks count = %d, want 1", len(g.Tasks))
	}

	if got, ok := g.GetTask("test-task"); !ok || got.Name != "Test Task" {
		t.Error("Failed to retrieve added task")
	}
}

func TestGraph_AddDependency(t *testing.T) {
	g := NewGraph()
	task1 := &Task{ID: "task-1", Name: "Task 1", Number: 1}
	task2 := &Task{ID: "task-2", Name: "Task 2", Number: 2}

	g.AddTask(task1)
	g.AddTask(task2)
	g.AddDependency(task1, task2, DepImplicit, true)

	if len(g.Edges) != 1 {
		t.Errorf("Edges count = %d, want 1", len(g.Edges))
	}

	deps := g.GetDependencies("task-2")
	if len(deps) != 1 || deps[0].ID != "task-1" {
		t.Error("GetDependencies returned wrong result")
	}

	dependents := g.GetDependents("task-1")
	if len(dependents) != 1 || dependents[0].ID != "task-2" {
		t.Error("GetDependents returned wrong result")
	}
}

func TestGraph_TopologicalSort(t *testing.T) {
	g := NewGraph()
	task1 := &Task{ID: "task-1", Name: "Task 1", Number: 1}
	task2 := &Task{ID: "task-2", Name: "Task 2", Number: 2}
	task3 := &Task{ID: "task-3", Name: "Task 3", Number: 3}

	g.AddTask(task1)
	g.AddTask(task2)
	g.AddTask(task3)
	g.AddDependency(task1, task2, DepImplicit, true) // task2 depends on task1
	g.AddDependency(task2, task3, DepImplicit, true) // task3 depends on task2

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("Sorted length = %d, want 3", len(sorted))
	}

	// Verify order: task1 before task2 before task3
	order := make(map[string]int)
	for i, task := range sorted {
		order[task.ID] = i
	}

	if order["task-1"] >= order["task-2"] {
		t.Error("task-1 should come before task-2")
	}
	if order["task-2"] >= order["task-3"] {
		t.Error("task-2 should come before task-3")
	}
}

func TestGraph_TopologicalSort_Cycle(t *testing.T) {
	g := NewGraph()
	task1 := &Task{ID: "task-1", Name: "Task 1", Number: 1}
	task2 := &Task{ID: "task-2", Name: "Task 2", Number: 2}

	g.AddTask(task1)
	g.AddTask(task2)
	g.AddDependency(task1, task2, DepImplicit, true) // task2 depends on task1
	g.AddDependency(task2, task1, DepImplicit, true) // task1 depends on task2 (cycle!)

	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("TopologicalSort() should return error for cycle")
	}

	if _, ok := err.(*CycleError); !ok {
		t.Errorf("Expected CycleError, got %T", err)
	}
}

func TestGraph_HasCycle(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Graph)
		wantCycle bool
	}{
		{
			name: "no cycle",
			setup: func(g *Graph) {
				task1 := &Task{ID: "task-1", Number: 1}
				task2 := &Task{ID: "task-2", Number: 2}
				g.AddTask(task1)
				g.AddTask(task2)
				g.AddDependency(task1, task2, DepImplicit, true)
			},
			wantCycle: false,
		},
		{
			name: "simple cycle",
			setup: func(g *Graph) {
				task1 := &Task{ID: "task-1", Number: 1}
				task2 := &Task{ID: "task-2", Number: 2}
				g.AddTask(task1)
				g.AddTask(task2)
				g.AddDependency(task1, task2, DepImplicit, true)
				g.AddDependency(task2, task1, DepImplicit, true)
			},
			wantCycle: true,
		},
		{
			name: "three node cycle",
			setup: func(g *Graph) {
				task1 := &Task{ID: "task-1", Number: 1}
				task2 := &Task{ID: "task-2", Number: 2}
				task3 := &Task{ID: "task-3", Number: 3}
				g.AddTask(task1)
				g.AddTask(task2)
				g.AddTask(task3)
				g.AddDependency(task1, task2, DepImplicit, true)
				g.AddDependency(task2, task3, DepImplicit, true)
				g.AddDependency(task3, task1, DepImplicit, true)
			},
			wantCycle: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGraph()
			tc.setup(g)
			if got := g.HasCycle(); got != tc.wantCycle {
				t.Errorf("HasCycle() = %v, want %v", got, tc.wantCycle)
			}
		})
	}
}

func TestGraph_CriticalPath(t *testing.T) {
	g := NewGraph()
	// Create a diamond pattern:
	//     1
	//    / \
	//   2   3
	//    \ /
	//     4
	task1 := &Task{ID: "task-1", Name: "Task 1", Number: 1}
	task2 := &Task{ID: "task-2", Name: "Task 2", Number: 2}
	task3 := &Task{ID: "task-3", Name: "Task 3", Number: 3}
	task4 := &Task{ID: "task-4", Name: "Task 4", Number: 4}

	g.AddTask(task1)
	g.AddTask(task2)
	g.AddTask(task3)
	g.AddTask(task4)

	g.AddDependency(task1, task2, DepImplicit, true)
	g.AddDependency(task1, task3, DepImplicit, true)
	g.AddDependency(task2, task4, DepImplicit, true)
	g.AddDependency(task3, task4, DepImplicit, true)

	path := g.CriticalPath()
	if len(path) != 3 {
		t.Errorf("CriticalPath length = %d, want 3", len(path))
	}

	// Path should start with task1 and end with task4
	if len(path) > 0 && path[0].ID != "task-1" {
		t.Error("Critical path should start with task-1")
	}
	if len(path) > 0 && path[len(path)-1].ID != "task-4" {
		t.Error("Critical path should end with task-4")
	}
}

func TestGraph_GetParallelGroups(t *testing.T) {
	g := NewGraph()
	task1 := &Task{ID: "task-1", Number: 1}
	task2a := &Task{ID: "task-2a", Number: 2}
	task2b := &Task{ID: "task-2b", Number: 2}
	task3 := &Task{ID: "task-3", Number: 3}

	g.AddTask(task1)
	g.AddTask(task2a)
	g.AddTask(task2b)
	g.AddTask(task3)

	g.AddDependency(task1, task2a, DepImplicit, true)
	g.AddDependency(task1, task2b, DepImplicit, true)
	g.AddDependency(task2a, task3, DepImplicit, true)
	g.AddDependency(task2b, task3, DepImplicit, true)

	groups := g.GetParallelGroups()
	if len(groups) != 3 {
		t.Errorf("GetParallelGroups() = %d groups, want 3", len(groups))
	}

	// First group should have task1
	if len(groups) > 0 && len(groups[0]) != 1 {
		t.Errorf("First group size = %d, want 1", len(groups[0]))
	}

	// Second group should have task2a and task2b (parallel)
	if len(groups) > 1 && len(groups[1]) != 2 {
		t.Errorf("Second group size = %d, want 2", len(groups[1]))
	}
}

func TestGraph_GetReadyTasks(t *testing.T) {
	g := NewGraph()
	task1 := &Task{ID: "task-1", Number: 1, Status: "pending"}
	task2 := &Task{ID: "task-2", Number: 2, Status: "pending"}

	g.AddTask(task1)
	g.AddTask(task2)
	g.AddDependency(task1, task2, DepImplicit, true)

	// Initially only task1 should be ready
	ready := g.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "task-1" {
		t.Errorf("GetReadyTasks() = %v, want [task-1]", ready)
	}

	// After completing task1, task2 should be ready
	task1.Status = "complete"
	ready = g.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "task-2" {
		t.Errorf("GetReadyTasks() after completion = %v, want [task-2]", ready)
	}
}

func TestTask_IsComplete(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"progress package completed", "completed", true},
		{"legacy complete", "complete", true},
		{"skipped counts as done", "skipped", true},
		{"pending", "pending", false},
		{"in_progress", "in_progress", false},
		{"blocked", "blocked", false},
		{"empty status", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{ID: "t", Status: tt.status}
			if got := task.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() with status %q = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestGraph_GetReadyTasks_StatusVariants(t *testing.T) {
	tests := []struct {
		name          string
		depStatus     string
		wantDependent bool
	}{
		{"dependency completed unblocks dependent", "completed", true},
		{"legacy complete unblocks dependent", "complete", true},
		{"pending dependency blocks dependent", "pending", false},
		{"in_progress dependency blocks dependent", "in_progress", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraph()
			dep := &Task{ID: "dep", Number: 1, Status: tt.depStatus}
			dependent := &Task{ID: "dependent", Number: 2, Status: "pending"}
			g.AddTask(dep)
			g.AddTask(dependent)
			g.AddDependency(dep, dependent, DepImplicit, true)

			ready := g.GetReadyTasks()

			var sawDependent bool
			for _, task := range ready {
				if task.ID == "dependent" {
					sawDependent = true
				}
			}

			if sawDependent != tt.wantDependent {
				t.Errorf("dependent ready = %v, want %v (dependency status %q)",
					sawDependent, tt.wantDependent, tt.depStatus)
			}
		})
	}
}

func TestGraph_GetReadyTasks_ExcludesBlocked(t *testing.T) {
	g := NewGraph()
	g.AddTask(&Task{ID: "task-1", Number: 1, Status: "blocked"})
	g.AddTask(&Task{ID: "task-2", Number: 2, Status: "pending"})

	ready := g.GetReadyTasks()

	if len(ready) != 1 || ready[0].ID != "task-2" {
		t.Errorf("GetReadyTasks() = %v, want only task-2 (blocked task must be excluded)", ready)
	}
}

func TestGraph_GetReadyTasks_SoftDepsDoNotBlock(t *testing.T) {
	g := NewGraph()
	soft := &Task{ID: "soft", Number: 1, Status: "pending"}
	hard := &Task{ID: "hard", Number: 2, Status: "pending"}
	target := &Task{ID: "target", Number: 3, Status: "pending"}
	g.AddTask(soft)
	g.AddTask(hard)
	g.AddTask(target)

	// A soft edge expresses preferred order only.
	g.AddDependency(soft, target, DepExplicit, false)

	ready := g.GetReadyTasks()
	if len(ready) != 3 {
		t.Fatalf("GetReadyTasks() = %d tasks, want 3 (a soft dependency must not block)", len(ready))
	}

	// A hard edge on the same target must still block it.
	g.AddDependency(hard, target, DepExplicit, true)

	ready = g.GetReadyTasks()
	for _, task := range ready {
		if task.ID == "target" {
			t.Error("target should be blocked by its hard dependency")
		}
	}
}
