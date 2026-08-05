// Package deps provides the fest deps command for dependency visualization.
package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/deps"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	jsonOutput   bool
	showAll      bool
	criticalPath bool
	readyOnly    bool
)

// NewDepsCommand creates the deps command
func NewDepsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps [task]",
		Short: "Show task dependencies",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Display dependency information for tasks in the festival.

Without arguments, shows the dependency graph for the current sequence.
With a task name, shows dependencies for that specific task.

Examples:
  fest deps                    # Show all deps in current sequence
  fest deps 02_implement       # Show deps for specific task
  fest deps --all              # Show all deps in festival
  fest deps --json             # Output as JSON
  fest deps --critical-path    # Show critical path through the DAG
  fest deps --ready            # Show every task that is unblocked right now
  fest deps --ready --all --json   # The whole festival's ready set, for orchestrators

The --ready set is the execution front: tasks whose hard dependencies are all
complete and which are not themselves complete or blocked. Unlike 'fest next',
which returns a single step, --ready returns every task that could be started
now, so an orchestrator can fan them out concurrently.`,
		RunE: runDeps,
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&showAll, "all", false, "show all dependencies in festival")
	cmd.Flags().BoolVar(&criticalPath, "critical-path", false, "show the critical path")
	cmd.Flags().BoolVar(&readyOnly, "ready", false, "show only tasks that are unblocked right now")

	return cmd
}

func runDeps(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Get festival path from scope context (resolved by PersistentPreRunE)
	festivalPath, ok := scope.FestivalFrom(cmd.Context())
	if !ok || festivalPath == "" {
		return errors.NotFound("festival context").
			WithHint("The scope system should have resolved a festival path")
	}

	resolver := deps.NewResolver(festivalPath)

	var graph *deps.Graph
	if showAll {
		graph, err = resolver.ResolveFestival()
	} else {
		// Try to resolve just the current sequence
		seqPath := findSequencePath(cwd, festivalPath)
		if seqPath != "" {
			graph, err = resolver.ResolveSequence(seqPath)
		} else {
			graph, err = resolver.ResolveFestival()
		}
	}

	if err != nil {
		return errors.Wrap(err, "resolving dependencies")
	}

	// If a specific task was requested
	if len(args) > 0 {
		taskName := args[0]
		return showTaskDeps(graph, taskName)
	}

	// Show the ready set if requested. Readiness is the only view that depends
	// on execution state, so it is the only one that pays for loading it.
	if readyOnly {
		if err := deps.ApplyProgress(cmd.Context(), graph, festivalPath); err != nil {
			return err
		}
		return showReady(graph, festivalPath)
	}

	// Show critical path if requested
	if criticalPath {
		return showCriticalPath(graph)
	}

	// Show full graph
	return showGraph(graph)
}

func showGraph(graph *deps.Graph) error {
	if jsonOutput {
		if err := shared.EncodeJSON(os.Stdout, graph); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Text output
	sorted, err := graph.TopologicalSort()
	if err != nil {
		fmt.Println(ui.Warning(fmt.Sprintf("Warning: %v", err)))
		fmt.Println()
	}

	fmt.Println(ui.H1("Dependency Graph"))

	// Show parallel groups
	groups := graph.GetParallelGroups()
	for i, group := range groups {
		fmt.Printf("\n%s %s\n", ui.H2(fmt.Sprintf("Level %d", i)), ui.Dim("(can run in parallel)"))
		for _, task := range group {
			deps := graph.GetDependencies(task.ID)
			depNames := make([]string, len(deps))
			for j, d := range deps {
				depNames[j] = d.Name
			}
			if len(depNames) > 0 {
				fmt.Printf("  - %s %s %s\n",
					ui.Value(task.Name, ui.TaskColor),
					ui.Dim("<-"),
					ui.Dim(strings.Join(depNames, ", ")),
				)
			} else {
				fmt.Printf("  - %s\n", ui.Value(task.Name, ui.TaskColor))
			}
		}
	}

	if sorted != nil {
		fmt.Println()
		fmt.Println(ui.H2("Execution Order"))
		for i, task := range sorted {
			fmt.Printf("  %s %s\n", ui.Value(fmt.Sprintf("%d.", i+1)), ui.Value(task.Name, ui.TaskColor))
		}
	}

	return nil
}

func showTaskDeps(graph *deps.Graph, taskName string) error {
	// Find the task
	var task *deps.Task
	for _, t := range graph.Tasks {
		if t.Name == taskName || strings.TrimSuffix(t.Name, ".md") == taskName ||
			filepath.Base(t.Path) == taskName || filepath.Base(t.Path) == taskName+".md" {
			task = t
			break
		}
	}

	if task == nil {
		return errors.NotFound("task not found").
			WithField("task", taskName)
	}

	if jsonOutput {
		output := struct {
			Task       *deps.Task   `json:"task"`
			DependsOn  []*deps.Task `json:"depends_on"`
			DependedBy []*deps.Task `json:"depended_by"`
		}{
			Task:       task,
			DependsOn:  graph.GetDependencies(task.ID),
			DependedBy: graph.GetDependents(task.ID),
		}
		if err := shared.EncodeJSON(os.Stdout, output); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	// Text output
	fmt.Println(ui.H1("Dependencies"))
	fmt.Printf("%s %s\n", ui.Label("Task"), ui.Value(task.Name, ui.TaskColor))
	printTaskInfo(task)

	deps := graph.GetDependencies(task.ID)
	printTaskList("Depends On", deps, "No dependencies (can start immediately)")

	dependents := graph.GetDependents(task.ID)
	printTaskList("Depended By", dependents, "No dependents (nothing waiting on this task)")

	fmt.Println(ui.H2("Dependency Tree"))
	printDepTree(graph, task, "  ", make(map[string]bool))

	return nil
}

func printTaskInfo(task *deps.Task) {
	fmt.Println(ui.H2("Task Info"))
	fmt.Printf("%s %s\n", ui.Label("Number"), ui.Value(fmt.Sprintf("%d", task.Number)))
	fmt.Printf("%s %s\n", ui.Label("Parallel Group"), ui.Value(fmt.Sprintf("%d", task.ParallelGroup)))
	if task.AutonomyLevel != "" {
		fmt.Printf("%s %s\n", ui.Label("Autonomy"), ui.Value(task.AutonomyLevel))
	}
	fmt.Println()
}

func printTaskList(title string, tasks []*deps.Task, emptyMessage string) {
	fmt.Println(ui.H2(title))
	if len(tasks) == 0 {
		fmt.Println(ui.Info(emptyMessage))
		fmt.Println()
		return
	}
	for _, task := range tasks {
		fmt.Printf("  - %s\n", ui.Value(task.Name, ui.TaskColor))
	}
	fmt.Println()
}

func printDepTree(graph *deps.Graph, task *deps.Task, indent string, visited map[string]bool) {
	if visited[task.ID] {
		fmt.Printf("%s└─ %s %s\n", indent, ui.Value(task.Name, ui.TaskColor), ui.Dim("(cycle)"))
		return
	}
	visited[task.ID] = true

	deps := graph.GetDependencies(task.ID)
	if len(deps) == 0 {
		fmt.Printf("%s└─ %s %s\n", indent, ui.Value(task.Name, ui.TaskColor), ui.Dim("(root)"))
		return
	}

	fmt.Printf("%s└─ %s\n", indent, ui.Value(task.Name, ui.TaskColor))
	for _, dep := range deps {
		printDepTree(graph, dep, indent+"  ", visited)
	}
}

// ReadyOutput is the machine-readable shape of the ready set. It is a stable
// contract: orchestrators fan out over tasks and need the count to decide
// whether there is anything to do at all.
//
// GatesEvaluated is always false today and is carried explicitly so a consumer
// can tell "no gate blocks this" from "gates were never checked". A phase can
// be held by an approval checkpoint that only the navigator resolves.
type ReadyOutput struct {
	Count          int          `json:"count"`
	GatesEvaluated bool         `json:"gates_evaluated"`
	Tasks          []*deps.Task `json:"tasks"`
}

func showReady(graph *deps.Graph, festivalPath string) error {
	ready := graph.GetExecutionFront()

	if jsonOutput {
		output := ReadyOutput{Count: len(ready), GatesEvaluated: false, Tasks: ready}
		if output.Tasks == nil {
			output.Tasks = []*deps.Task{}
		}
		if err := shared.EncodeJSON(os.Stdout, output); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	fmt.Println(ui.H1("Ready Tasks"))

	if len(ready) == 0 {
		fmt.Println(ui.Info("Nothing is unblocked right now."))
		fmt.Println(ui.Info("Every task is complete, blocked, or waiting on a dependency."))
		return nil
	}

	fmt.Println(ui.Info("These tasks have all dependencies satisfied and can start now."))
	fmt.Println()

	for _, task := range ready {
		fmt.Printf("  - %s %s\n",
			ui.Value(task.Name, ui.TaskColor),
			ui.Dim(relativeToFestival(festivalPath, task.Path)),
		)
	}

	fmt.Println()
	fmt.Printf("%s %s\n", ui.Label("Ready"), ui.Value(fmt.Sprintf("%d tasks", len(ready))))
	fmt.Println(ui.Dim("Phase quality gates are not evaluated here. Check 'fest next' before dispatching."))

	return nil
}

func showCriticalPath(graph *deps.Graph) error {
	path := graph.CriticalPath()

	if jsonOutput {
		if err := shared.EncodeJSON(os.Stdout, path); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	fmt.Println(ui.H1("Critical Path"))
	fmt.Println(ui.Info("The critical path is the longest chain of dependencies."))
	fmt.Println(ui.Info("Optimizing these tasks reduces overall completion time."))

	if len(path) == 0 {
		fmt.Println(ui.Info("No critical path (no dependencies or cycle detected)."))
		return nil
	}

	for i, task := range path {
		if i < len(path)-1 {
			fmt.Printf("  %s %s\n  %s\n",
				ui.Value(fmt.Sprintf("%d.", i+1)),
				ui.Value(task.Name, ui.TaskColor),
				ui.Dim("↓"),
			)
		} else {
			fmt.Printf("  %s %s\n", ui.Value(fmt.Sprintf("%d.", i+1)), ui.Value(task.Name, ui.TaskColor))
		}
	}

	fmt.Println()
	fmt.Printf("%s %s\n", ui.Label("Critical path length"), ui.Value(fmt.Sprintf("%d tasks", len(path))))

	return nil
}

// relativeToFestival trims the festival root off a task path for display.
// JSON output keeps the absolute path so orchestrators can act on it directly.
func relativeToFestival(festivalPath, taskPath string) string {
	rel, err := filepath.Rel(festivalPath, taskPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return taskPath
	}
	return rel
}

func findSequencePath(cwd, festivalPath string) string {
	rel, err := filepath.Rel(festivalPath, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 {
		// We're in a sequence directory
		return filepath.Join(festivalPath, parts[0], parts[1])
	}

	return ""
}
