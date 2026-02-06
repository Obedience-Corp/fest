package show

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
