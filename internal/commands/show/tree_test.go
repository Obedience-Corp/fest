package show

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func TestBuildFestivalTree(t *testing.T) {
	// Create a temporary festival structure
	tmpDir := t.TempDir()

	// Create festival root with goal file
	festivalGoal := `# Festival Goal

**Primary Goal:** Test the tree builder functionality.

## Objective

Build and test the festival tree.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "FESTIVAL_GOAL.md"), []byte(festivalGoal), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a phase directory
	phaseDir := filepath.Join(tmpDir, "001_TEST_PHASE")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	phaseGoal := `# Phase Goal

**Primary Goal:** Test phase goal extraction.
`
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a sequence directory
	seqDir := filepath.Join(phaseDir, "01_test_sequence")
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}

	seqGoal := `# Sequence Goal

**Primary Goal:** Test sequence goal extraction.
`
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0644); err != nil {
		t.Fatal(err)
	}

	// Create task files
	task1 := `---
fest_type: task
fest_tracking: true
---
# Task 1
`
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(task1), 0644); err != nil {
		t.Fatal(err)
	}

	task2 := `---
fest_type: task
fest_tracking: true
---
# Task 2
`
	if err := os.WriteFile(filepath.Join(seqDir, "02_task.md"), []byte(task2), 0644); err != nil {
		t.Fatal(err)
	}

	// Build the tree
	tree, err := BuildFestivalTree(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree failed: %v", err)
	}

	// Verify festival node
	if tree.NodeType != "festival" {
		t.Errorf("expected NodeType 'festival', got '%s'", tree.NodeType)
	}
	if tree.Goal != "Test the tree builder functionality." {
		t.Errorf("expected festival goal, got '%s'", tree.Goal)
	}

	// Verify phase
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(tree.Children))
	}
	phase := tree.Children[0]
	if phase.NodeType != "phase" {
		t.Errorf("expected NodeType 'phase', got '%s'", phase.NodeType)
	}
	if phase.Goal != "Test phase goal extraction." {
		t.Errorf("expected phase goal, got '%s'", phase.Goal)
	}

	// Verify sequence
	if len(phase.Children) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(phase.Children))
	}
	seq := phase.Children[0]
	if seq.NodeType != "sequence" {
		t.Errorf("expected NodeType 'sequence', got '%s'", seq.NodeType)
	}
	if seq.Goal != "Test sequence goal extraction." {
		t.Errorf("expected sequence goal, got '%s'", seq.Goal)
	}

	// Verify task count
	if seq.Stats.Total != 2 {
		t.Errorf("expected 2 tasks, got %d", seq.Stats.Total)
	}

	// Verify task children are created
	if len(seq.Children) != 2 {
		t.Fatalf("expected 2 task children, got %d", len(seq.Children))
	}
	for _, taskNode := range seq.Children {
		if taskNode.NodeType != "task" {
			t.Errorf("expected NodeType 'task', got '%s'", taskNode.NodeType)
		}
	}
	if seq.Children[0].Name != "01_task.md" {
		t.Errorf("expected task name '01_task.md', got '%s'", seq.Children[0].Name)
	}
	if seq.Children[1].Name != "02_task.md" {
		t.Errorf("expected task name '02_task.md', got '%s'", seq.Children[1].Name)
	}
}

func TestRenderTreeStepShowsConciseJudgeFeedback(t *testing.T) {
	step := buildStepNode(shared.WorkflowStepView{
		Number:   2,
		Name:     "REVIEW",
		Status:   wf.StepStatusBlocked,
		Feedback: `approval auto mode: schema_version=fest.approval.judge/v1 judge_command="ob judge" decision=reject reason="missing acceptance proof"`,
		Judge:    &wf.JudgeState{Status: wf.JudgeRejected},
	})
	tree := &DisplayNode{
		Name:     "festival",
		NodeType: "festival",
		Children: []*DisplayNode{{
			Name:     "001_PHASE",
			NodeType: "phase",
			Status:   "blocked",
			Stats:    StatusCounts{Total: 1, Blocked: 1},
			Children: []*DisplayNode{step},
		}},
	}
	defaultOutput := RenderTree(tree, DefaultTreeOptions())
	if strings.Contains(defaultOutput, "Feedback:") {
		t.Fatalf("tree should hide feedback by default:\n%s", defaultOutput)
	}
	opts := DefaultTreeOptions()
	opts.ShowFeedback = true
	output := RenderTree(tree, opts)

	if !strings.Contains(output, "Feedback: missing acceptance proof") {
		t.Fatalf("tree missing concise feedback:\n%s", output)
	}
	if !strings.Contains(output, "Judge: rejected") {
		t.Fatalf("tree missing judge status:\n%s", output)
	}
	if strings.Contains(output, "schema_version") || strings.Contains(output, "judge_command") {
		t.Fatalf("tree leaked judge metadata:\n%s", output)
	}
}

func TestRenderTreeStepShowsWaitingJudgeIcon(t *testing.T) {
	step := buildStepNode(shared.WorkflowStepView{
		Number: 2,
		Name:   "REVIEW",
		Status: wf.StepStatusBlocked,
		Judge:  &wf.JudgeState{Status: wf.JudgeRunning},
	})
	output := RenderTree(&DisplayNode{
		Name:     "festival",
		NodeType: "festival",
		Children: []*DisplayNode{{
			Name:     "001_PHASE",
			NodeType: "phase",
			Status:   "blocked",
			Stats:    StatusCounts{Total: 1, Blocked: 1},
			Children: []*DisplayNode{step},
		}},
	}, DefaultTreeOptions())

	if !strings.Contains(output, "⚖ Step 2: REVIEW") {
		t.Fatalf("tree missing purple waiting judge icon:\n%s", output)
	}
	if !strings.Contains(output, "Judge: waiting") {
		t.Fatalf("tree missing waiting judge state:\n%s", output)
	}
}

func TestRenderTree(t *testing.T) {
	// Create a simple tree structure
	tree := &DisplayNode{
		Name:     "test-festival",
		NodeType: "festival",
		Goal:     "Test festival goal",
		Status:   "pending",
		Stats:    StatusCounts{Total: 5, Completed: 2, Pending: 3},
		Children: []*DisplayNode{
			{
				Name:     "001_PHASE",
				NodeType: "phase",
				Goal:     "Test phase goal",
				Status:   "in_progress",
				Stats:    StatusCounts{Total: 5, Completed: 2, Pending: 3},
				Children: []*DisplayNode{
					{
						Name:     "01_sequence",
						NodeType: "sequence",
						Goal:     "Test sequence goal",
						Status:   "in_progress",
						Stats:    StatusCounts{Total: 5, Completed: 2, Pending: 3},
					},
				},
			},
		},
	}

	opts := DefaultTreeOptions()
	opts.ShowGoals = true
	output := RenderTree(tree, opts)

	// Verify output contains expected elements
	if !strings.Contains(output, "test-festival") {
		t.Error("output should contain festival name")
	}
	if !strings.Contains(output, "001_PHASE") {
		t.Error("output should contain phase name")
	}
	if !strings.Contains(output, "01_sequence") {
		t.Error("output should contain sequence name")
	}
	if !strings.Contains(output, "Goal:") {
		t.Error("output should contain goals")
	}
	if !strings.Contains(output, "2/5") {
		t.Error("output should contain task fraction")
	}
}

func TestRenderTree_NoGoals(t *testing.T) {
	tree := &DisplayNode{
		Name:     "test-festival",
		NodeType: "festival",
		Status:   "pending",
		Children: []*DisplayNode{
			{
				Name:     "001_PHASE",
				NodeType: "phase",
				Status:   "pending",
				Children: []*DisplayNode{
					{
						Name:     "01_sequence",
						NodeType: "sequence",
						Status:   "pending",
						Stats:    StatusCounts{Total: 3, Pending: 3},
					},
				},
			},
		},
	}

	opts := DefaultTreeOptions()
	opts.ShowGoals = false
	output := RenderTree(tree, opts)

	// Verify goals are not shown
	if strings.Contains(output, "Goal:") {
		t.Error("output should not contain goals when ShowGoals is false")
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0%"},
		{100, "100%"},
		{50, "50%"},
		{33.3, "33.3%"},
		{66.7, "66.7%"},
		{12.5, "12.5%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatPercent(tt.input)
			if result != tt.expected {
				t.Errorf("formatPercent(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateGoal(t *testing.T) {
	// truncateGoal no longer truncates - it returns goals as-is to allow natural wrapping
	tests := []struct {
		name     string
		goal     string
		maxLen   int
		expected string
	}{
		{"short goal", "Short goal", 20, "Short goal"},
		{"long goal returned as-is", "This is a very long goal that needs truncation", 20, "This is a very long goal that needs truncation"},
		{"exact length goal", "Exact length goal", 17, "Exact length goal"},
		{"empty goal", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateGoal(tt.goal, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateGoal(%q, %d) = %q, want %q", tt.goal, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestRenderTree_WithTasks(t *testing.T) {
	tree := &DisplayNode{
		Name:     "test-festival",
		NodeType: "festival",
		Status:   "in_progress",
		Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
		Children: []*DisplayNode{
			{
				Name:     "001_PHASE",
				NodeType: "phase",
				Status:   "in_progress",
				Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
				Children: []*DisplayNode{
					{
						Name:     "01_sequence",
						NodeType: "sequence",
						Status:   "in_progress",
						Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
						Children: []*DisplayNode{
							{Name: "01_analyze.md", NodeType: "task", Status: "completed",
								Stats: StatusCounts{Total: 1, Completed: 1}},
							{Name: "02_implement.md", NodeType: "task", Status: "pending",
								Stats: StatusCounts{Total: 1, Pending: 1}},
							{Name: "03_review.md", NodeType: "task", Status: "pending",
								Stats: StatusCounts{Total: 1, Pending: 1}},
						},
					},
				},
			},
		},
	}

	opts := DefaultTreeOptions()
	output := RenderTree(tree, opts)

	// Tasks should appear individually in expanded mode
	if !strings.Contains(output, "01_analyze.md") {
		t.Error("output should contain task name 01_analyze.md")
	}
	if !strings.Contains(output, "02_implement.md") {
		t.Error("output should contain task name 02_implement.md")
	}
	if !strings.Contains(output, "03_review.md") {
		t.Error("output should contain task name 03_review.md")
	}
}

func TestRenderTree_Collapsed(t *testing.T) {
	tree := &DisplayNode{
		Name:     "test-festival",
		NodeType: "festival",
		Status:   "in_progress",
		Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
		Children: []*DisplayNode{
			{
				Name:     "001_PHASE",
				NodeType: "phase",
				Status:   "in_progress",
				Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
				Children: []*DisplayNode{
					{
						Name:     "01_sequence",
						NodeType: "sequence",
						Status:   "in_progress",
						Stats:    StatusCounts{Total: 3, Completed: 1, Pending: 2},
						Children: []*DisplayNode{
							{Name: "01_task.md", NodeType: "task", Status: "completed"},
						},
					},
				},
			},
		},
	}

	opts := DefaultTreeOptions()
	opts.Collapsed = true
	output := RenderTree(tree, opts)

	// Phase should appear
	if !strings.Contains(output, "001_PHASE") {
		t.Error("output should contain phase name in collapsed mode")
	}
	// Sequence should NOT appear (children are hidden)
	if strings.Contains(output, "01_sequence") {
		t.Error("output should NOT contain sequence name in collapsed mode")
	}
	// Task should NOT appear
	if strings.Contains(output, "01_task.md") {
		t.Error("output should NOT contain task name in collapsed mode")
	}
	// Counter should appear
	if !strings.Contains(output, "1/3") {
		t.Error("output should contain aggregate counter 1/3 in collapsed mode")
	}
}

func TestRenderTree_InProgressPreservesBlockedTaskStatus(t *testing.T) {
	tree := &DisplayNode{
		Name:   "test-festival",
		Status: "blocked",
		Stats:  StatusCounts{Total: 1, Blocked: 1},
		Children: []*DisplayNode{{
			Name:   "001_PHASE",
			Status: "blocked",
			Stats:  StatusCounts{Total: 1, Blocked: 1},
			Children: []*DisplayNode{{
				Name:   "01_sequence",
				Status: "blocked",
				Stats:  StatusCounts{Total: 1, Blocked: 1},
				Children: []*DisplayNode{{
					Name:     "01_blocked.md",
					NodeType: "task",
					Status:   "blocked",
					Stats:    StatusCounts{Total: 1, Blocked: 1},
				}},
			}},
		}},
	}

	opts := DefaultTreeOptions()
	opts.InProgress = true
	output := RenderTree(tree, opts)

	if tree.Children[0].Children[0].Children[0].Status != "blocked" {
		t.Fatalf("in-progress rendering changed blocked task status to %q", tree.Children[0].Children[0].Children[0].Status)
	}
	if !strings.Contains(output, "■") {
		t.Errorf("blocked task should render the blocked icon, got %q", output)
	}
	if strings.Contains(output, "●") {
		t.Errorf("blocked task should not render as in_progress, got %q", output)
	}
}

func TestRenderTree_InProgressHighlightsPendingTask(t *testing.T) {
	task := &DisplayNode{
		Name:     "01_pending.md",
		NodeType: "task",
		Status:   "pending",
		Stats:    StatusCounts{Total: 1, Pending: 1},
	}
	tree := &DisplayNode{
		Name:   "test-festival",
		Status: "pending",
		Stats:  StatusCounts{Total: 1, Pending: 1},
		Children: []*DisplayNode{{
			Name:   "001_PHASE",
			Status: "pending",
			Stats:  StatusCounts{Total: 1, Pending: 1},
			Children: []*DisplayNode{{
				Name:     "01_sequence",
				NodeType: "sequence",
				Status:   "pending",
				Stats:    StatusCounts{Total: 1, Pending: 1},
				Children: []*DisplayNode{task},
			}},
		}},
	}

	opts := DefaultTreeOptions()
	opts.InProgress = true
	RenderTree(tree, opts)

	if task.Status != "in_progress" {
		t.Errorf("next pending task status = %q, want in_progress", task.Status)
	}
}

func inProgressTask(name, status string, deferred bool) *DisplayNode {
	return &DisplayNode{
		Name:            name,
		NodeType:        "task",
		Status:          status,
		BlockerDeferred: deferred,
		Stats: StatusCounts{
			Total:      1,
			Completed:  boolToInt(status == "completed"),
			InProgress: boolToInt(status == "in_progress"),
			Pending:    boolToInt(status == "pending"),
			Blocked:    boolToInt(status == "blocked"),
		},
	}
}

func inProgressParent(name, nodeType string, children ...*DisplayNode) *DisplayNode {
	node := &DisplayNode{Name: name, NodeType: nodeType, Children: children}
	for _, child := range children {
		node.Stats.Total += child.Stats.Total
		node.Stats.Completed += child.Stats.Completed
		node.Stats.InProgress += child.Stats.InProgress
		node.Stats.Pending += child.Stats.Pending
		node.Stats.Blocked += child.Stats.Blocked
	}
	node.Status = determineStatus(node.Stats)
	return node
}

func TestRenderTree_InProgressNextUpSkipsDeferredOnlyWork(t *testing.T) {
	tests := []struct {
		name         string
		children     func() []*DisplayNode
		wantNext     []string
		wantShown    []string
		wantHidden   []string
		wantBlocked  string
		wantPhaseRow string
	}{
		{
			name: "deferred blocker moves next-up to the following phase",
			children: func() []*DisplayNode {
				return []*DisplayNode{
					inProgressParent("001_PACKAGE", "phase", inProgressParent("01_package_seq", "sequence",
						inProgressTask("01_done.md", "completed", false),
						inProgressTask("02_deferred.md", "blocked", true))),
					inProgressParent("002_RELEASE", "phase", inProgressParent("01_release_seq", "sequence",
						inProgressTask("01_ready.md", "pending", false),
						inProgressTask("02_ready.md", "pending", false))),
				}
			},
			wantNext:     []string{"002_RELEASE", "01_release_seq", "01_ready.md"},
			wantShown:    []string{"01_release_seq", "01_ready.md", "02_ready.md"},
			wantHidden:   []string{"01_package_seq", "02_deferred.md"},
			wantBlocked:  "02_deferred.md",
			wantPhaseRow: "001_PACKAGE  ■ 1/2 items",
		},
		{
			name: "undeferred blocker keeps the phase next-up",
			children: func() []*DisplayNode {
				return []*DisplayNode{
					inProgressParent("001_PACKAGE", "phase", inProgressParent("01_package_seq", "sequence",
						inProgressTask("01_done.md", "completed", false),
						inProgressTask("02_blocked.md", "blocked", false))),
					inProgressParent("002_RELEASE", "phase", inProgressParent("01_release_seq", "sequence",
						inProgressTask("01_ready.md", "pending", false),
						inProgressTask("02_ready.md", "pending", false))),
				}
			},
			wantNext:     []string{"001_PACKAGE", "01_package_seq", "02_blocked.md"},
			wantShown:    []string{"01_package_seq", "02_blocked.md"},
			wantHidden:   []string{"01_release_seq", "01_ready.md"},
			wantBlocked:  "02_blocked.md",
			wantPhaseRow: "001_PACKAGE  ■ blocked",
		},
		{
			name: "deferred-only sequence yields to a ready sibling sequence",
			children: func() []*DisplayNode {
				return []*DisplayNode{
					inProgressParent("001_PACKAGE", "phase",
						inProgressParent("01_package_seq", "sequence",
							inProgressTask("01_done.md", "completed", false),
							inProgressTask("02_deferred.md", "blocked", true)),
						inProgressParent("02_package_seq", "sequence",
							inProgressTask("03_ready.md", "pending", false))),
				}
			},
			wantNext:     []string{"001_PACKAGE", "02_package_seq", "03_ready.md"},
			wantShown:    []string{"01_package_seq", "02_package_seq", "03_ready.md"},
			wantHidden:   []string{"02_deferred.md"},
			wantBlocked:  "02_deferred.md",
			wantPhaseRow: "001_PACKAGE  ■ blocked",
		},
		{
			name: "all remaining work deferred falls back to the first blocked phase",
			children: func() []*DisplayNode {
				return []*DisplayNode{
					inProgressParent("001_PACKAGE", "phase", inProgressParent("01_package_seq", "sequence",
						inProgressTask("01_done.md", "completed", false),
						inProgressTask("02_deferred.md", "blocked", true))),
					inProgressParent("002_RELEASE", "phase", inProgressParent("01_release_seq", "sequence",
						inProgressTask("01_also_deferred.md", "blocked", true))),
				}
			},
			wantNext:     []string{"001_PACKAGE", "01_package_seq", "02_deferred.md"},
			wantShown:    []string{"01_package_seq", "02_deferred.md"},
			wantHidden:   []string{"01_release_seq", "01_also_deferred.md"},
			wantBlocked:  "02_deferred.md",
			wantPhaseRow: "001_PACKAGE  ■ blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := inProgressParent("test-festival", "festival", tt.children()...)
			opts := DefaultTreeOptions()
			opts.InProgress = true
			output := RenderTree(tree, opts)

			var next []string
			var blocked *DisplayNode
			var walk func(*DisplayNode)
			walk = func(node *DisplayNode) {
				if node.IsNextPending {
					next = append(next, node.Name)
				}
				if node.Name == tt.wantBlocked {
					blocked = node
				}
				for _, child := range node.Children {
					walk(child)
				}
			}
			walk(tree)

			if strings.Join(next, " > ") != strings.Join(tt.wantNext, " > ") {
				t.Errorf("next-up path = %v, want %v", next, tt.wantNext)
			}
			for _, name := range tt.wantShown {
				if !strings.Contains(output, name) {
					t.Errorf("output should show %q", name)
				}
			}
			for _, name := range tt.wantHidden {
				if strings.Contains(output, name) {
					t.Errorf("output should hide %q", name)
				}
			}
			if !strings.Contains(output, tt.wantPhaseRow) {
				t.Errorf("output should contain phase row %q", tt.wantPhaseRow)
			}
			if blocked == nil || blocked.Status != "blocked" {
				t.Errorf("task %q must keep blocked status, got %+v", tt.wantBlocked, blocked)
			}
			if got := tree.Children[0].Status; got != "blocked" {
				t.Errorf("phase %s status = %q, want blocked", tree.Children[0].Name, got)
			}
			if t.Failed() {
				t.Logf("rendered tree:\n%s", output)
			}
		})
	}
}

func TestBuildFestivalTree_CarriesBlockerDeferredFromProgressEvents(t *testing.T) {
	festivalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), []byte("name: tree-deferred\nmetadata:\n  id: TD0001\n"), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	seqDir := filepath.Join(festivalDir, "001_PLAN", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	for _, name := range []string{"01_deferred.md", "02_blocked.md"} {
		if err := os.WriteFile(filepath.Join(seqDir, name), []byte("# Task\n"), 0o644); err != nil {
			t.Fatalf("write task: %v", err)
		}
	}
	festDir := filepath.Join(festivalDir, progress.ProgressDir)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}
	events := `{"ts":"2026-09-20T10:00:00Z","event":"blocked","task":"001_PLAN/01_seq/01_deferred.md","reason":"vendor API removed"}
{"ts":"2026-09-20T10:01:00Z","event":"blocked","task":"001_PLAN/01_seq/02_blocked.md","reason":"waiting on review"}
{"ts":"2026-09-21T11:00:00Z","event":"blocker_deferred","task":"001_PLAN/01_seq/01_deferred.md","reason":"vendor API removed","deferral_reason":"revisit after the rest lands","deferred_by":"Ada Lovelace","actor":"operator"}
`
	if err := os.WriteFile(filepath.Join(festDir, progress.ProgressEventsFile), []byte(events), 0o644); err != nil {
		t.Fatalf("write progress events: %v", err)
	}

	tree, err := BuildFestivalTree(context.Background(), festivalDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}
	tasks := tree.Children[0].Children[0].Children
	if len(tasks) != 2 {
		t.Fatalf("task nodes = %d, want 2", len(tasks))
	}
	want := map[string]bool{"01_deferred.md": true, "02_blocked.md": false}
	for _, task := range tasks {
		if task.Status != "blocked" {
			t.Errorf("%s status = %q, want blocked", task.Name, task.Status)
		}
		if task.BlockerDeferred != want[task.Name] {
			t.Errorf("%s BlockerDeferred = %v, want %v", task.Name, task.BlockerDeferred, want[task.Name])
		}
		wantFeedback := "waiting on review"
		if task.Name == "01_deferred.md" {
			wantFeedback = "vendor API removed"
		}
		if task.Feedback != wantFeedback {
			t.Errorf("%s Feedback = %q, want %q", task.Name, task.Feedback, wantFeedback)
		}
	}
}

func TestRenderTree_InProgressRejectedGateShowsDeferredTasks(t *testing.T) {
	first := inProgressTask("01_native_artifacts.md", "blocked", true)
	first.Feedback = "Windows native execution is still required"
	second := inProgressTask("02_catalog_entry.md", "blocked", true)
	second.Feedback = "Catalog registration needs published producer bytes"
	gate := buildStepNode(shared.WorkflowStepView{
		Number: 1, Name: "OUTCOME", Status: wf.StepStatusBlocked,
		Feedback: "The required public artifacts and catalog entry are missing",
		Judge:    &wf.JudgeState{Status: wf.JudgeRejected},
	})
	gate.Name = "Gate 1: OUTCOME"
	gate.StateKey = "gate:006_DISTRIBUTION"
	gate.Stats.Blocked = 1
	tree := inProgressParent("release-festival", "festival",
		inProgressParent("006_DISTRIBUTION", "phase",
			inProgressParent("01_producer", "sequence", first),
			inProgressParent("02_catalog", "sequence", second), gate))
	originalStats := tree.Stats
	opts := DefaultTreeOptions()
	opts.InProgress = true
	out := RenderTree(tree, opts)
	for _, want := range []string{first.Name + " (deferred)", second.Name + " (deferred)", "Judge: rejected"} {
		if !strings.Contains(out, want) {
			t.Errorf("watch must expose rejected gate's deferred tasks; missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Feedback:") {
		t.Fatalf("feedback must remain opt-in:\n%s", out)
	}
	if !gate.IsNextPending || first.IsNextPending || second.IsNextPending {
		t.Fatal("exposing deferred tasks must not change the next-up path")
	}
	opts.ShowFeedback = true
	out = RenderTree(tree, opts)
	for _, want := range []string{first.Feedback, second.Feedback, gate.Feedback} {
		if !strings.Contains(out, "Feedback: "+want) {
			t.Errorf("--show-feedback must explain both task blockers and rejection; missing %q:\n%s", want, out)
		}
	}
	if tree.Stats != originalStats || first.Status != "blocked" || second.Status != "blocked" || gate.Status != "blocked" {
		t.Fatal("displaying remediation must preserve counts and blocked states")
	}
	opts.Collapsed = true
	out = RenderTree(tree, opts)
	if strings.Contains(out, first.Name) || strings.Contains(out, "Gate 1:") {
		t.Fatalf("explicit --collapsed must still hide children:\n%s", out)
	}
}

func TestRenderTree_InProgressShowsRunningWorkBeyondNextPath(t *testing.T) {
	for _, laterPhase := range []bool{false, true} {
		name := "sibling sequence"
		if laterPhase {
			name = "later phase"
		}
		t.Run(name, func(t *testing.T) {
			blocked := inProgressParent("01_blocked", "sequence",
				inProgressTask("01_waiting.md", "blocked", false))
			active := inProgressParent("02_active", "sequence",
				inProgressTask("01_running.md", "in_progress", false),
				inProgressTask("02_external.md", "blocked", true))
			phase := inProgressParent("001_IMPLEMENT", "phase", blocked)
			var tree *DisplayNode
			if laterPhase {
				tree = inProgressParent("release-festival", "festival", phase,
					inProgressParent("002_RELEASE", "phase", active))
			} else {
				tree = inProgressParent("release-festival", "festival",
					inProgressParent("001_IMPLEMENT", "phase", blocked, active))
			}
			opts := DefaultTreeOptions()
			opts.InProgress = true
			out := RenderTree(tree, opts)
			if !strings.Contains(out, "01_running.md") || !strings.Contains(out, "01_waiting.md") {
				t.Fatalf("watch must show running work alongside the blocking next-up path:\n%s", out)
			}
			if active.Status != "blocked" || active.Children[0].Status != "in_progress" {
				t.Fatal("mixed blocked/running sequence must preserve actual task states")
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name     string
		stats    StatusCounts
		expected string
	}{
		{"empty", StatusCounts{}, "pending"},
		{"all pending", StatusCounts{Total: 5, Pending: 5}, "pending"},
		{"all completed", StatusCounts{Total: 5, Completed: 5}, "completed"},
		{"in progress", StatusCounts{Total: 5, Completed: 2, InProgress: 1, Pending: 2}, "in_progress"},
		{"some completed no in progress", StatusCounts{Total: 5, Completed: 2, Pending: 3}, "in_progress"},
		{"blocked", StatusCounts{Total: 5, Blocked: 1, Pending: 4}, "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatus(tt.stats)
			if result != tt.expected {
				t.Errorf("determineStatus(%+v) = %s, want %s", tt.stats, result, tt.expected)
			}
		})
	}
}
