package deps

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestDepsAndProgressSettledAgree(t *testing.T) {
	cases := []struct {
		name        string
		progStatus  string
		depsStatus  string
		deferred    bool
		wantSettled bool
	}{
		{"pending", progress.StatusPending, "pending", false, false},
		{"pending with a stale flag", progress.StatusPending, "pending", true, false},
		{"in progress", progress.StatusInProgress, "in_progress", false, false},
		{"in progress with a stale flag", progress.StatusInProgress, "in_progress", true, false},
		{"blocked open", progress.StatusBlocked, "blocked", false, false},
		{"blocked deferred", progress.StatusBlocked, "blocked", true, true},
		{"completed", progress.StatusCompleted, "complete", false, true},
		{"completed with a stale flag", progress.StatusCompleted, "complete", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			depsTask := &Task{Status: tc.depsStatus, BlockerDeferred: tc.deferred}
			progTask := &progress.TaskProgress{Status: tc.progStatus, BlockerDeferred: tc.deferred}

			if got := depsTask.IsSettled(); got != tc.wantSettled {
				t.Errorf("deps IsSettled() = %v, want %v", got, tc.wantSettled)
			}
			if got := progTask.IsSettled(); got != tc.wantSettled {
				t.Errorf("progress IsSettled() = %v, want %v", got, tc.wantSettled)
			}
		})
	}
}

func TestIsCompleteIgnoresTheDeferralFlag(t *testing.T) {
	for _, status := range []string{progress.StatusPending, progress.StatusInProgress, progress.StatusBlocked} {
		task := &Task{Status: status, BlockerDeferred: true}
		if task.IsComplete() {
			t.Errorf("IsComplete() = true for status %q with a deferred blocker", status)
		}
	}
	for _, status := range []string{progress.StatusCompleted, "complete", "skipped"} {
		task := &Task{Status: status}
		if !task.IsComplete() {
			t.Errorf("IsComplete() = false for status %q", status)
		}
	}
}

func readyIDs(tasks []*Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestSettledDependencyAdmitsDependent(t *testing.T) {
	newChain := func(deferred bool, required bool) *Graph {
		g := NewGraph()
		first := &Task{ID: "01", Name: "first", Number: 1, Status: progress.StatusCompleted}
		second := &Task{ID: "02", Name: "second", Number: 2,
			Status: progress.StatusBlocked, BlockerDeferred: deferred}
		third := &Task{ID: "03", Name: "third", Number: 3, Status: progress.StatusPending}
		g.AddTask(first)
		g.AddTask(second)
		g.AddTask(third)
		g.AddDependency(first, second, DepImplicit, true)
		g.AddDependency(second, third, DepImplicit, required)
		return g
	}

	t.Run("dependent admitted when the blocker is deferred", func(t *testing.T) {
		ids := readyIDs(newChain(true, true).GetReadyTasks())
		if !contains(ids, "03") {
			t.Errorf("ready = %v, want it to contain 03", ids)
		}
	})

	t.Run("deferred task is never selected", func(t *testing.T) {
		ids := readyIDs(newChain(true, true).GetReadyTasks())
		if contains(ids, "02") {
			t.Errorf("ready = %v, want the deferred blocker left out", ids)
		}
	})

	t.Run("dependent refused when the blocker is open", func(t *testing.T) {
		ids := readyIDs(newChain(false, true).GetReadyTasks())
		if contains(ids, "03") || contains(ids, "02") {
			t.Errorf("ready = %v, want neither 02 nor 03", ids)
		}
	})

	t.Run("soft dependency on a deferred blocker is unchanged", func(t *testing.T) {
		soft := readyIDs(newChain(true, false).GetReadyTasks())
		if !contains(soft, "03") {
			t.Errorf("ready = %v, want 03 ready through a soft dependency", soft)
		}
		open := readyIDs(newChain(false, false).GetReadyTasks())
		if !contains(open, "03") {
			t.Errorf("ready = %v, want 03 still ready through a soft dependency on an open blocker", open)
		}
	})
}

func TestGetReadyTasksParallelGroupWithOneDeferred(t *testing.T) {
	g := NewGraph()
	gate := &Task{ID: "01", Name: "gate", Number: 1, Status: progress.StatusCompleted}
	a := &Task{ID: "02a", Name: "a", Number: 2, ParallelGroup: 1, Status: progress.StatusPending}
	b := &Task{ID: "02b", Name: "b", Number: 2, ParallelGroup: 1,
		Status: progress.StatusBlocked, BlockerDeferred: true}
	c := &Task{ID: "02c", Name: "c", Number: 2, ParallelGroup: 1, Status: progress.StatusPending}
	for _, task := range []*Task{gate, a, b, c} {
		g.AddTask(task)
	}
	for _, task := range []*Task{a, b, c} {
		g.AddDependency(gate, task, DepImplicit, true)
	}

	ids := readyIDs(g.GetReadyTasks())
	for _, want := range []string{"02a", "02c"} {
		if !contains(ids, want) {
			t.Errorf("ready = %v, want it to contain %s", ids, want)
		}
	}
	if contains(ids, "02b") {
		t.Errorf("ready = %v, want the deferred sibling left out", ids)
	}
}
