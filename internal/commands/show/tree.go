package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festctx "github.com/Obedience-Corp/fest/internal/context"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/taskfilter"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// DisplayNode represents a node in the festival tree hierarchy.
type DisplayNode struct {
	Name          string       // Directory/file name
	Goal          string       // Primary goal from *_GOAL.md
	Status        string       // pending/in_progress/completed/blocked
	Stats         StatusCounts // Task counts for this level
	NodeType      string       // "festival", "phase", "sequence", "step", "task"
	Children      []*DisplayNode
	IsNextPending bool // First incomplete node at this level — expanded by --inprogress
}

// TreeOptions configures how the tree is rendered.
type TreeOptions struct {
	ShowGoals  bool // Show primary goals (default: true)
	Collapsed  bool // Show only phases with counters, no children
	InProgress bool // Expand only in_progress phases/sequences, collapse rest
	Width      int  // Terminal width for alignment (0 = auto)
}

// DefaultTreeOptions returns sensible defaults for tree rendering.
func DefaultTreeOptions() TreeOptions {
	return TreeOptions{
		ShowGoals: false,
		Width:     80,
	}
}

// BuildFestivalTree constructs a hierarchical DisplayNode tree from a festival directory.
func BuildFestivalTree(ctx context.Context, festivalDir string) (*DisplayNode, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	festivalRoot := resolveFestivalRoot(festivalDir)
	var store *progress.Store
	if mgr, err := progress.NewManager(ctx, festivalRoot); err == nil {
		store = mgr.Store()
	}

	// Create festival root node
	root := &DisplayNode{
		Name:     filepath.Base(festivalDir),
		NodeType: "festival",
		Status:   "pending",
	}

	// Try to read festival goal
	root.Goal = readPrimaryGoal(festivalDir, "FESTIVAL_GOAL.md")
	if root.Goal == "" {
		root.Goal = readPrimaryGoal(festivalDir, "FESTIVAL_OVERVIEW.md")
	}

	// Find and process phases
	entries, err := os.ReadDir(festivalDir)
	if err != nil {
		return root, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		phaseDir := filepath.Join(festivalDir, entry.Name())
		if !isPhaseDir(phaseDir) && !hasNumericPrefix(entry.Name()) {
			continue
		}

		phaseNode := buildPhaseNode(ctx, phaseDir, store, festivalRoot)
		root.Children = append(root.Children, phaseNode)

		// Aggregate stats to root
		root.Stats.Total += phaseNode.Stats.Total
		root.Stats.Completed += phaseNode.Stats.Completed
		root.Stats.InProgress += phaseNode.Stats.InProgress
		root.Stats.Pending += phaseNode.Stats.Pending
		root.Stats.Blocked += phaseNode.Stats.Blocked
	}

	// Determine festival status based on children
	root.Status = determineStatus(root.Stats)

	return root, nil
}

func buildPhaseNode(ctx context.Context, phaseDir string, store *progress.Store, festivalRoot string) *DisplayNode {
	node := &DisplayNode{
		Name:     filepath.Base(phaseDir),
		NodeType: "phase",
		Goal:     readPrimaryGoal(phaseDir, "PHASE_GOAL.md"),
		Status:   "pending",
	}

	// Check if this is a workflow phase (has WORKFLOW.md)
	if shared.HasWorkflowFile(phaseDir) {
		// Load workflow steps as children
		steps, err := shared.LoadWorkflowStepsForPhase(ctx, festivalRoot, phaseDir)
		if err == nil && len(steps) > 0 {
			for _, step := range steps {
				stepNode := buildStepNode(step)
				node.Children = append(node.Children, stepNode)

				// Aggregate stats based on step status
				node.Stats.Total++
				switch step.Status {
				case wf.StepStatusCompleted:
					node.Stats.Completed++
				case wf.StepStatusInProgress:
					node.Stats.InProgress++
				case wf.StepStatusBlocked:
					node.Stats.Blocked++
				default:
					node.Stats.Pending++
				}
			}
			node.Status = determineStatus(node.Stats)
			return node
		}
	}

	// Implementation phase - load sequences
	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return node
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		seqDir := filepath.Join(phaseDir, entry.Name())
		if !isSequenceDir(seqDir) && !hasNumericPrefix(entry.Name()) {
			continue
		}

		seqNode := buildSequenceNode(seqDir, store, festivalRoot)
		node.Children = append(node.Children, seqNode)

		// Aggregate stats
		node.Stats.Total += seqNode.Stats.Total
		node.Stats.Completed += seqNode.Stats.Completed
		node.Stats.InProgress += seqNode.Stats.InProgress
		node.Stats.Pending += seqNode.Stats.Pending
		node.Stats.Blocked += seqNode.Stats.Blocked
	}

	node.Status = determineStatus(node.Stats)
	return node
}

// buildStepNode creates a DisplayNode for a workflow step.
func buildStepNode(step shared.WorkflowStepView) *DisplayNode {
	status := "pending"
	switch step.Status {
	case wf.StepStatusCompleted:
		status = "completed"
	case wf.StepStatusInProgress:
		status = "in_progress"
	case wf.StepStatusBlocked:
		status = "blocked"
	}

	return &DisplayNode{
		Name:     fmt.Sprintf("Step %d: %s", step.Number, step.Name),
		Goal:     step.Goal,
		Status:   status,
		NodeType: "step",
		Stats: StatusCounts{
			Total:     1,
			Completed: boolToInt(step.Status == wf.StepStatusCompleted),
		},
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func buildSequenceNode(seqDir string, store *progress.Store, festivalRoot string) *DisplayNode {
	node := &DisplayNode{
		Name:     filepath.Base(seqDir),
		NodeType: "sequence",
		Goal:     readPrimaryGoal(seqDir, "SEQUENCE_GOAL.md"),
		Status:   "pending",
	}

	entries, err := os.ReadDir(seqDir)
	if err != nil {
		return node
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		// Use taskfilter for unified file classification
		if !taskfilter.ShouldTrack(name) {
			continue
		}

		taskPath := filepath.Join(seqDir, name)
		if !progress.IsTracked(taskPath) {
			continue
		}

		node.Stats.Total++
		status := progress.ResolveTaskStatus(store, festivalRoot, taskPath)

		displayStatus := "pending"
		switch status {
		case progress.StatusCompleted:
			node.Stats.Completed++
			displayStatus = "completed"
		case progress.StatusInProgress:
			node.Stats.InProgress++
			displayStatus = "in_progress"
		case progress.StatusBlocked:
			node.Stats.Blocked++
			displayStatus = "blocked"
		default:
			node.Stats.Pending++
		}

		// Create task child node
		taskNode := &DisplayNode{
			Name:     name,
			NodeType: "task",
			Status:   displayStatus,
			Stats: StatusCounts{
				Total:     1,
				Completed: boolToInt(status == progress.StatusCompleted),
			},
		}
		node.Children = append(node.Children, taskNode)
	}

	node.Status = determineStatus(node.Stats)
	return node
}

// markNextPending walks the tree top-down and marks the first non-completed
// child at each level as IsNextPending, recursing into it. Only one path
// through the tree gets marked, giving --inprogress a "next up" expansion.
func markNextPending(node *DisplayNode) {
	for _, child := range node.Children {
		if child.Status != "completed" {
			child.IsNextPending = true
			markNextPending(child)
			return
		}
	}
}

func readPrimaryGoal(dir, filename string) string {
	goalPath := filepath.Join(dir, filename)
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return ""
	}
	return festctx.ExtractPrimaryGoal(content)
}

func determineStatus(stats StatusCounts) string {
	if stats.Total == 0 {
		return "pending"
	}
	if stats.Completed == stats.Total {
		return "completed"
	}
	if stats.InProgress > 0 || stats.Completed > 0 {
		return "in_progress"
	}
	return "pending"
}

// RenderTree renders a DisplayNode tree to a string.
func RenderTree(node *DisplayNode, opts TreeOptions) string {
	var sb strings.Builder

	// Festival header
	progress := float64(0)
	if node.Stats.Total > 0 {
		progress = float64(node.Stats.Completed) / float64(node.Stats.Total) * 100
	}

	// Festival name with progress
	festivalStyle := lipgloss.NewStyle().Foreground(ui.FestivalColor).Bold(true)
	sb.WriteString(festivalStyle.Render(node.Name))
	sb.WriteString("  ")
	sb.WriteString(ui.Dim(formatPercent(progress)))
	sb.WriteString("\n")

	// Festival goal if present
	if opts.ShowGoals && node.Goal != "" {
		sb.WriteString(ui.Dim("Goal: " + truncateGoal(node.Goal, opts.Width-6)))
		sb.WriteString("\n")
	}

	// Mark the next pending path so --inprogress expands it
	if opts.InProgress {
		markNextPending(node)
	}

	// Render children (phases)
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		renderPhaseNode(&sb, child, "", isLast, opts)
	}

	return sb.String()
}

func renderPhaseNode(sb *strings.Builder, node *DisplayNode, prefix string, isLast bool, opts TreeOptions) {
	// Box drawing characters
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	// Phase line with status
	phaseStyle := lipgloss.NewStyle().Foreground(ui.PhaseColor).Bold(true)
	sb.WriteString(prefix)
	sb.WriteString(ui.Dim(connector))
	sb.WriteString(phaseStyle.Render(node.Name))
	sb.WriteString("  ")
	collapsePhase := opts.Collapsed || (opts.InProgress && node.Status != "in_progress" && !node.IsNextPending)
	if collapsePhase {
		sb.WriteString(formatCollapsedPhaseStatus(node))
	} else {
		sb.WriteString(formatNodeStatus(node))
	}
	sb.WriteString("\n")

	// Goal line
	if opts.ShowGoals && node.Goal != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.Dim("Goal: " + truncateGoal(node.Goal, opts.Width-len(prefix)-10)))
		sb.WriteString("\n")
	}

	// Render children: collapsed hides all, inprogress hides non-active
	showChildren := !opts.Collapsed
	if opts.InProgress && node.Status != "in_progress" && !node.IsNextPending {
		showChildren = false
	}
	if showChildren {
		newPrefix := prefix + childPrefix
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			if child.NodeType == "step" {
				renderStepNode(sb, child, newPrefix, childIsLast, opts)
			} else {
				renderSequenceNode(sb, child, newPrefix, childIsLast, opts)
			}
		}
	}
}

func renderSequenceNode(sb *strings.Builder, node *DisplayNode, prefix string, isLast bool, opts TreeOptions) {
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	// Sequence line with task count
	seqStyle := lipgloss.NewStyle().Foreground(ui.SequenceColor)
	sb.WriteString(prefix)
	sb.WriteString(ui.Dim(connector))
	sb.WriteString(seqStyle.Render(node.Name))
	sb.WriteString("  ")
	sb.WriteString(formatSequenceStatus(node))
	sb.WriteString("\n")

	// Goal line
	if opts.ShowGoals && node.Goal != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.Dim("Goal: " + truncateGoal(node.Goal, opts.Width-len(prefix)-10)))
		sb.WriteString("\n")
	}

	// Render task children: collapsed hides all, inprogress hides non-active
	showTasks := !opts.Collapsed
	if opts.InProgress && node.Status != "in_progress" && !node.IsNextPending {
		showTasks = false
	}
	if showTasks {
		newPrefix := prefix + childPrefix
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			renderTaskNode(sb, child, newPrefix, childIsLast, opts)
		}
	}
}

func renderStepNode(sb *strings.Builder, node *DisplayNode, prefix string, isLast bool, opts TreeOptions) {
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	// Step line with status icon
	sb.WriteString(prefix)
	sb.WriteString(ui.Dim(connector))
	sb.WriteString(formatStepStatus(node))
	sb.WriteString("\n")

	// Goal line
	if opts.ShowGoals && node.Goal != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.Dim("Goal: " + truncateGoal(node.Goal, opts.Width-len(prefix)-10)))
		sb.WriteString("\n")
	}
}

func renderTaskNode(sb *strings.Builder, node *DisplayNode, prefix string, isLast bool, _ TreeOptions) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	sb.WriteString(prefix)
	sb.WriteString(ui.Dim(connector))
	sb.WriteString(formatTaskStatus(node))
	sb.WriteString("\n")
}

func formatTaskStatus(node *DisplayNode) string {
	icon := ui.StateIcon(node.Status)
	taskStyle := lipgloss.NewStyle().Foreground(ui.TaskColor)
	return icon + " " + taskStyle.Render(node.Name)
}

func formatStepStatus(node *DisplayNode) string {
	icon := shared.WorkflowStepIcon(wf.StepStatus(node.Status))
	return icon + " " + node.Name
}

func formatCollapsedPhaseStatus(node *DisplayNode) string {
	icon := ui.StateIcon(node.Status)
	if node.Stats.Total == 0 {
		return icon + " " + ui.Dim("no items")
	}
	return icon + " " + ui.Dim(formatFraction(node.Stats.Completed, node.Stats.Total)+" items")
}

func formatNodeStatus(node *DisplayNode) string {
	icon := ui.StateIcon(node.Status)
	return icon + " " + ui.Dim(node.Status)
}

func formatSequenceStatus(node *DisplayNode) string {
	icon := ui.StateIcon(node.Status)
	if node.Stats.Total == 0 {
		return icon + " " + ui.Dim("no tasks")
	}
	return icon + " " + ui.Dim(formatFraction(node.Stats.Completed, node.Stats.Total)+" tasks")
}

func formatPercent(p float64) string {
	if p == 0 {
		return "0%"
	}
	if p == 100 {
		return "100%"
	}
	// Format with one decimal place, trim trailing zeros
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", p), "0"), ".")
	return s + "%"
}

func formatFraction(completed, total int) string {
	return ui.Value(fmt.Sprintf("%d/%d", completed, total), ui.MetadataColor)
}

func truncateGoal(goal string, _ int) string {
	// Don't truncate goals - let them display fully and wrap naturally
	return goal
}
