package deps

import (
	"sort"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// TopologicalSort returns tasks in dependency order using Kahn's algorithm.
// Returns an error if a cycle is detected.
func (g *Graph) TopologicalSort() ([]*Task, error) {
	// Copy in-degree map since we'll modify it
	inDegree := make(map[string]int)
	for id, deg := range g.Incoming {
		inDegree[id] = deg
	}

	// Find all tasks with no dependencies
	var queue []*Task
	for id, task := range g.Tasks {
		if inDegree[id] == 0 {
			queue = append(queue, task)
		}
	}

	// Sort queue by task number for deterministic output
	sort.Slice(queue, func(i, j int) bool {
		if queue[i].Number == queue[j].Number {
			return queue[i].ID < queue[j].ID
		}
		return queue[i].Number < queue[j].Number
	})

	var result []*Task
	visited := make(map[string]bool)

	for len(queue) > 0 {
		// Take first task from queue
		task := queue[0]
		queue = queue[1:]

		if visited[task.ID] {
			continue
		}
		visited[task.ID] = true
		result = append(result, task)

		// Reduce in-degree of all dependents
		for _, dependent := range g.Outgoing[task.ID] {
			inDegree[dependent.ID]--
			if inDegree[dependent.ID] == 0 {
				queue = append(queue, dependent)
			}
		}

		// Re-sort queue for deterministic output
		sort.Slice(queue, func(i, j int) bool {
			if queue[i].Number == queue[j].Number {
				return queue[i].ID < queue[j].ID
			}
			return queue[i].Number < queue[j].Number
		})
	}

	// If not all tasks are in result, there's a cycle
	if len(result) != len(g.Tasks) {
		cycle := g.findCycle()
		return nil, &CycleError{Cycle: cycle}
	}

	return result, nil
}

// HasCycle returns true if the graph contains a cycle
func (g *Graph) HasCycle() bool {
	_, err := g.TopologicalSort()
	return err != nil
}

// findCycle finds and returns the cycle in the graph using DFS
func (g *Graph) findCycle() []string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)

	var cycle []string
	var dfs func(taskID string) bool
	dfs = func(taskID string) bool {
		visited[taskID] = true
		recStack[taskID] = true

		for _, dep := range g.Outgoing[taskID] {
			if !visited[dep.ID] {
				parent[dep.ID] = taskID
				if dfs(dep.ID) {
					return true
				}
			} else if recStack[dep.ID] {
				// Found cycle - reconstruct it
				cycle = append(cycle, dep.ID)
				curr := taskID
				for curr != dep.ID {
					cycle = append(cycle, curr)
					curr = parent[curr]
				}
				cycle = append(cycle, dep.ID)
				// Reverse to show cycle in correct order
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return true
			}
		}

		recStack[taskID] = false
		return false
	}

	for id := range g.Tasks {
		if !visited[id] {
			if dfs(id) {
				return cycle
			}
		}
	}

	return nil
}

// CriticalPath returns the longest path through the DAG
// This represents the minimum time to complete all tasks if executed optimally
func (g *Graph) CriticalPath() []*Task {
	sorted, err := g.TopologicalSort()
	if err != nil {
		return nil // Can't compute critical path if there's a cycle
	}

	if len(sorted) == 0 {
		return nil
	}

	// Compute longest path to each node
	dist := make(map[string]int)
	prev := make(map[string]string)

	for _, task := range g.Tasks {
		dist[task.ID] = 0
	}

	for _, task := range sorted {
		for _, dep := range g.GetDependencies(task.ID) {
			if dist[dep.ID]+1 > dist[task.ID] {
				dist[task.ID] = dist[dep.ID] + 1
				prev[task.ID] = dep.ID
			}
		}
	}

	// Find task with maximum distance (end of critical path)
	maxDist := 0
	var endTask string
	for id, d := range dist {
		if d >= maxDist {
			maxDist = d
			endTask = id
		}
	}

	// Reconstruct path
	var path []*Task
	curr := endTask
	for curr != "" {
		if task, ok := g.Tasks[curr]; ok {
			path = append([]*Task{task}, path...)
		}
		curr = prev[curr]
	}

	return path
}

// GetParallelGroups returns groups of tasks that can be executed in parallel
func (g *Graph) GetParallelGroups() [][]*Task {
	sorted, err := g.TopologicalSort()
	if err != nil {
		return nil
	}

	if len(sorted) == 0 {
		return nil
	}

	// Compute the level of each task (max level of dependencies + 1)
	level := make(map[string]int)
	for _, task := range g.Tasks {
		level[task.ID] = 0
	}

	for _, task := range sorted {
		for _, dep := range g.GetDependencies(task.ID) {
			if level[dep.ID]+1 > level[task.ID] {
				level[task.ID] = level[dep.ID] + 1
			}
		}
	}

	// Group tasks by level
	maxLevel := 0
	for _, l := range level {
		if l > maxLevel {
			maxLevel = l
		}
	}

	groups := make([][]*Task, maxLevel+1)
	for _, task := range sorted {
		l := level[task.ID]
		groups[l] = append(groups[l], task)
	}

	return groups
}

// GetExecutionFront returns the tasks that may be started right now, respecting
// phase order.
//
// GetReadyTasks answers a narrower question: whose dependencies are satisfied.
// It has no notion of phases, because the resolver only creates edges within a
// sequence plus whatever the frontmatter declares. On a real festival that means
// tasks from phases 004, 005 and 006 can all report ready at once while 004 is
// still open, which is not an execution front, it is every unblocked leaf in the
// festival.
//
// This keeps only the earliest phase that still has ready work. It is
// deliberately a separate function: GetReadyTasks has another caller in the
// guidance selector, and narrowing it there would change task selection.
//
// This does not evaluate phase quality gates. A phase can be blocked on an
// approval checkpoint that only the navigator knows about, so callers that
// dispatch work must still honour `fest next`.
func (g *Graph) GetExecutionFront() []*Task {
	ready := g.GetReadyTasks()
	if len(ready) == 0 {
		return ready
	}

	earliest := ""
	for _, task := range ready {
		if earliest == "" || task.PhasePath < earliest {
			earliest = task.PhasePath
		}
	}

	front := make([]*Task, 0, len(ready))
	for _, task := range ready {
		if task.PhasePath == earliest {
			front = append(front, task)
		}
	}

	return front
}

// IsComplete reports whether a task's status means no further work is expected.
// It accepts both the progress package's "completed" and the shorter "complete"
// that older festival state and hand-edited frontmatter may still carry.
func (t *Task) IsComplete() bool {
	return t.Status == progress.StatusCompleted || t.Status == "complete" || t.Status == "skipped"
}

// GetReadyTasks returns tasks that are ready to execute right now: not already
// complete, not blocked, and with every hard dependency complete. Soft
// dependencies do not gate readiness.
func (g *Graph) GetReadyTasks() []*Task {
	var ready []*Task

	for _, task := range g.Tasks {
		if task.IsComplete() || task.Status == progress.StatusBlocked {
			continue
		}

		deps := g.GetRequiredDependencies(task.ID)
		allComplete := true
		for _, dep := range deps {
			if !dep.IsComplete() {
				allComplete = false
				break
			}
		}

		if allComplete {
			ready = append(ready, task)
		}
	}

	// Sort by number for deterministic order
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].Number == ready[j].Number {
			return ready[i].ID < ready[j].ID
		}
		return ready[i].Number < ready[j].Number
	})

	return ready
}
