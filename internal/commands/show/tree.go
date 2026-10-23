package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festctx "github.com/Obedience-Corp/fest/internal/context"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/taskfilter"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// DisplayNode represents a node in the festival tree hierarchy.
type DisplayNode struct {
	Name           string         `json:"name"`                      // Directory/file name
	Goal           string         `json:"goal,omitempty"`            // Primary goal from *_GOAL.md
	Status         string         `json:"status"`                    // pending/in_progress/completed/blocked
	Stats          StatusCounts   `json:"stats"`                     // Task counts for this level
	NodeType       string         `json:"node_type"`                 // "festival", "phase", "sequence", "step", "task"
	Children       []*DisplayNode `json:"children,omitempty"`        // Child phases, sequences, steps, or tasks
	IsNextPending  bool           `json:"is_next_pending,omitempty"` // First node at this level with workable items — expanded by --inprogress
	Feedback       string         `json:"feedback,omitempty"`        // Concise actionable feedback for blocked tasks or steps
	JudgeStatus    string         `json:"judge_status,omitempty"`    // waiting/approved/rejected/failed
	WaitingOnJudge bool           `json:"waiting_on_judge,omitempty"`

	// BlockerDeferred marks a blocked task whose blocker was deferred with
	// fest task defer. The task still displays as blocked.
	BlockerDeferred bool `json:"blocker_deferred,omitempty"`

	// DeferredBlockers is how many deferred blockers the phase had when this
	// gate step was approved. Absent for older approvals and when none.
	DeferredBlockers int `json:"deferred_blockers,omitempty"`

	// Replay identity for callers that re-materialize progress (fest gif).
	// Not part of the JSON contract.
	TaskPath   string `json:"-"` // task file path (task nodes)
	StateKey   string `json:"-"` // workflow state key, "gate:<phase>" for gates (step nodes)
	StepNumber int    `json:"-"` // workflow step number (step nodes)
}

// TreeOptions configures how the tree is rendered.
type TreeOptions struct {
	ShowGoals    bool // Show primary goals (default: true)
	ShowFeedback bool // Show blocked-task and step feedback
	Collapsed    bool // Show only phases with counters, no children
	InProgress   bool // Expand only in_progress phases/sequences, collapse rest
	Width        int  // Terminal width for alignment (0 = auto)
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
	// The tree is display-only, like stats: building it must never migrate or
	// rewrite .fest/ files as a side effect.
	if mgr, err := progress.NewManagerReadOnly(ctx, festivalRoot); err == nil {
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

	// Load workflow steps if WORKFLOW.md exists
	var stepNodes []*DisplayNode
	var stepStats StatusCounts
	if shared.HasWorkflowFile(phaseDir) {
		steps, err := shared.LoadWorkflowStepsForPhase(ctx, festivalRoot, phaseDir)
		if err == nil && len(steps) > 0 {
			for _, step := range steps {
				stepNode := buildStepNode(step)
				stepNode.StateKey = filepath.Base(phaseDir)
				stepNodes = append(stepNodes, stepNode)
				stepStats.Total++
				switch step.Status {
				case wf.StepStatusCompleted, wf.StepStatusSkipped:
					stepStats.Completed++
				case wf.StepStatusInProgress:
					stepStats.InProgress++
				case wf.StepStatusBlocked:
					stepStats.Blocked++
				default:
					stepStats.Pending++
				}
			}
		}
	}

	// Load sequence nodes
	var seqNodes []*DisplayNode
	var seqStats StatusCounts
	entries, err := os.ReadDir(phaseDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			seqDir := filepath.Join(phaseDir, entry.Name())
			if !isSequenceDir(seqDir) && !hasNumericPrefix(entry.Name()) {
				continue
			}
			seqNode := buildSequenceNode(seqDir, store, festivalRoot)
			seqNodes = append(seqNodes, seqNode)
			seqStats.Total += seqNode.Stats.Total
			seqStats.Completed += seqNode.Stats.Completed
			seqStats.InProgress += seqNode.Stats.InProgress
			seqStats.Pending += seqNode.Stats.Pending
			seqStats.Blocked += seqNode.Stats.Blocked
		}
	}

	// Load gate steps if GATES.md exists
	var gateNodes []*DisplayNode
	var gateStats StatusCounts
	if shared.HasGateFile(phaseDir) {
		gateSteps, err := shared.LoadGateStepsForPhase(ctx, festivalRoot, phaseDir)
		if err == nil && len(gateSteps) > 0 {
			for _, step := range gateSteps {
				gateNode := buildStepNode(step)
				gateNode.Name = "Gate " + gateNode.Name[len("Step "):]
				gateNode.StateKey = "gate:" + filepath.Base(phaseDir)
				gateNodes = append(gateNodes, gateNode)
				gateStats.Total++
				switch step.Status {
				case wf.StepStatusCompleted, wf.StepStatusSkipped:
					gateStats.Completed++
				case wf.StepStatusInProgress:
					gateStats.InProgress++
				case wf.StepStatusBlocked:
					gateStats.Blocked++
				default:
					gateStats.Pending++
				}
			}
		}
	}

	// Order children based on workflow_position (default: after sequences)
	// Gates always come last since they run after all other phase work
	position := shared.WorkflowPositionForPhase(phaseDir)
	if position == frontmatter.WorkflowPositionBefore {
		node.Children = append(node.Children, stepNodes...)
		node.Children = append(node.Children, seqNodes...)
	} else {
		node.Children = append(node.Children, seqNodes...)
		node.Children = append(node.Children, stepNodes...)
	}
	node.Children = append(node.Children, gateNodes...)

	// Aggregate all stats
	node.Stats.Total = stepStats.Total + seqStats.Total + gateStats.Total
	node.Stats.Completed = stepStats.Completed + seqStats.Completed + gateStats.Completed
	node.Stats.InProgress = stepStats.InProgress + seqStats.InProgress + gateStats.InProgress
	node.Stats.Pending = stepStats.Pending + seqStats.Pending + gateStats.Pending
	node.Stats.Blocked = stepStats.Blocked + seqStats.Blocked + gateStats.Blocked

	node.Status = determineStatus(node.Stats)
	return node
}

// StepDisplayStatus maps a workflow step status to the status the tree shows.
// Anything else, including failed_with_remediation, displays as pending.
func StepDisplayStatus(status wf.StepStatus) string {
	switch status {
	case wf.StepStatusCompleted:
		return "completed"
	case wf.StepStatusInProgress:
		return "in_progress"
	case wf.StepStatusBlocked:
		return "blocked"
	case wf.StepStatusSkipped:
		return "skipped"
	default:
		return "pending"
	}
}

// TaskDisplayStatus maps a progress task status to the status the tree shows.
func TaskDisplayStatus(status string) string {
	switch status {
	case progress.StatusCompleted:
		return "completed"
	case progress.StatusInProgress:
		return "in_progress"
	case progress.StatusBlocked:
		return "blocked"
	default:
		return "pending"
	}
}

// buildStepNode creates a DisplayNode for a workflow step.
func buildStepNode(step shared.WorkflowStepView) *DisplayNode {
	return &DisplayNode{
		Name:       fmt.Sprintf("Step %d: %s", step.Number, step.Name),
		Goal:       step.Goal,
		Status:     StepDisplayStatus(step.Status),
		NodeType:   "step",
		StepNumber: step.Number,
		Feedback:   wf.DisplayFeedback(step.Feedback),
		JudgeStatus: func() string {
			if step.Judge == nil {
				return ""
			}
			return step.Judge.Status
		}(),
		WaitingOnJudge:   step.JudgeWaiting(),
		DeferredBlockers: step.DeferredBlockers,
		Stats: StatusCounts{
			Total:     1,
			Completed: boolToInt(step.Status == wf.StepStatusCompleted || step.Status == wf.StepStatusSkipped),
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
		status := progress.StatusPending
		deferred := false
		feedback := ""
		if record, ok := progress.ResolveTaskProgress(store, festivalRoot, taskPath); ok && record != nil {
			status = record.Status
			deferred = record.Status == progress.StatusBlocked && record.BlockerDeferred
			if record.Status == progress.StatusBlocked {
				feedback = wf.DisplayFeedback(record.BlockerMessage)
			}
		}

		displayStatus := TaskDisplayStatus(status)
		switch displayStatus {
		case "completed":
			node.Stats.Completed++
		case "in_progress":
			node.Stats.InProgress++
		case "blocked":
			node.Stats.Blocked++
		default:
			node.Stats.Pending++
		}

		// Create task child node
		taskNode := &DisplayNode{
			Name:            name,
			NodeType:        "task",
			Status:          displayStatus,
			BlockerDeferred: deferred,
			Feedback:        feedback,
			TaskPath:        taskPath,
			Stats: StatusCounts{
				Total:      1,
				Completed:  boolToInt(status == progress.StatusCompleted),
				InProgress: boolToInt(status == progress.StatusInProgress),
				Pending:    boolToInt(status == progress.StatusPending),
				Blocked:    boolToInt(status == progress.StatusBlocked),
			},
		}
		node.Children = append(node.Children, taskNode)
	}

	node.Status = determineStatus(node.Stats)
	return node
}

// markNextPending walks the tree top-down and marks the first child at each
// level that still has workable items as IsNextPending, recursing into it.
// Children whose only open work is deferred-blocked tasks are passed over;
// when every open child is like that, the first open one is marked instead.
// Only one path through the tree gets marked, giving --inprogress a "next up"
// expansion.
func markNextPending(node *DisplayNode) {
	next := firstChild(node, func(child *DisplayNode) bool { return !isSettled(child) })
	if next == nil {
		next = firstChild(node, func(child *DisplayNode) bool { return !isTerminal(child) })
	}
	if next == nil {
		return
	}
	next.IsNextPending = true
	// Highlight an actual pending leaf as the next place to work, but do
	// not mask an existing in_progress or blocked status.
	if (next.NodeType == "task" || next.NodeType == "step") && next.Status == "pending" {
		next.Status = "in_progress"
	}
	markNextPending(next)
}

func firstChild(node *DisplayNode, match func(*DisplayNode) bool) *DisplayNode {
	for _, child := range node.Children {
		if match(child) {
			return child
		}
	}
	return nil
}

func isTerminal(node *DisplayNode) bool {
	return node.Status == "completed" || node.Status == "skipped"
}

// isSettled reports whether a node has no workable items left: it is
// terminal, it is a deferred-blocked task, or it is blocked and every child
// beneath it is settled.
func isSettled(node *DisplayNode) bool {
	if isTerminal(node) {
		return true
	}
	if node.Status != "blocked" {
		return false
	}
	if node.NodeType == "task" {
		return node.BlockerDeferred
	}
	if len(node.Children) == 0 {
		return false
	}
	for _, child := range node.Children {
		if !isSettled(child) {
			return false
		}
	}
	return true
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
	if stats.Blocked > 0 {
		return "blocked"
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
	blockedGate := hasBlockedGate(node)
	collapsePhase := opts.Collapsed || (opts.InProgress && !expandInProgress(node) && !blockedGate)
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
	showChildren := !collapsePhase
	if showChildren {
		newPrefix := prefix + childPrefix
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			if child.NodeType == "step" {
				renderStepNode(sb, child, newPrefix, childIsLast, opts)
			} else {
				sequenceOpts := opts
				// A rejected gate can still depend on deferred tasks. Keep
				// those tasks visible so the gate does not hide its blockers.
				if blockedGate && child.Stats.Blocked > 0 {
					sequenceOpts.InProgress = false
				}
				renderSequenceNode(sb, child, newPrefix, childIsLast, sequenceOpts)
			}
		}
	}
}

// The next-up path is not the only active work: an earlier blocked phase must
// not hide a task that is already running elsewhere in the festival.
func expandInProgress(node *DisplayNode) bool {
	return node.IsNextPending || node.Stats.InProgress > 0
}

func hasBlockedGate(node *DisplayNode) bool {
	for _, child := range node.Children {
		if strings.HasPrefix(child.StateKey, "gate:") && child.Status == "blocked" {
			return true
		}
	}
	return false
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
	if opts.InProgress && !expandInProgress(node) {
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
	if opts.ShowFeedback && node.Feedback != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.Error("Feedback: ") + node.Feedback)
		sb.WriteString("\n")
	}
	if node.JudgeStatus != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		label := shared.JudgeVerdictLabel(node.JudgeStatus, node.DeferredBlockers)
		if node.WaitingOnJudge {
			label = "Judge: waiting"
		}
		sb.WriteString(ui.ColoredText(label, ui.JudgeColor))
		sb.WriteString("\n")
	} else if node.DeferredBlockers > 0 && node.Status == "completed" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.ColoredText("Approved "+shared.DeferredBlockersSuffix(node.DeferredBlockers), ui.JudgeColor))
		sb.WriteString("\n")
	}
}

func renderTaskNode(sb *strings.Builder, node *DisplayNode, prefix string, isLast bool, opts TreeOptions) {
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	sb.WriteString(prefix)
	sb.WriteString(ui.Dim(connector))
	sb.WriteString(formatTaskStatus(node))
	sb.WriteString("\n")
	if opts.ShowFeedback && node.Status == "blocked" && node.Feedback != "" {
		sb.WriteString(prefix)
		sb.WriteString(ui.Dim(childPrefix))
		sb.WriteString(ui.Error("Feedback: ") + node.Feedback)
		sb.WriteString("\n")
	}
}

func formatTaskStatus(node *DisplayNode) string {
	icon := ui.StateIcon(node.Status)
	taskStyle := lipgloss.NewStyle().Foreground(ui.TaskColor)
	label := icon + " " + taskStyle.Render(node.Name)
	if node.Status == "blocked" && node.BlockerDeferred {
		label += ui.Dim(" (deferred)")
	}
	return label
}

func formatStepStatus(node *DisplayNode) string {
	icon := shared.WorkflowStepIcon(wf.StepStatus(node.Status))
	if node.WaitingOnJudge {
		icon = ui.ColoredText("⚖", ui.JudgeColor)
	}
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
