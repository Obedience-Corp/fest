package task

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// TestTaskShowGate is the regression for the explicit-arg bypass that
// obey-agent flagged in the second review. Reading an implementation
// task body in a planning festival is the bypass we're closing: an
// agent that knows or guesses the task ID could read what to do
// without ever hitting fest promote. The phase-type check inside
// EnforcePreActive must still let planning/research/ingest tasks
// through.
func TestTaskShowGate(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		phaseType   string
		expectBlock bool
	}{
		{name: "planning_blocks_explicit_impl_task", status: "planning", phaseType: "implementation", expectBlock: true},
		{name: "planning_blocks_explicit_review_task", status: "planning", phaseType: "review", expectBlock: true},
		{name: "ready_blocks_explicit_impl_task", status: "ready", phaseType: "implementation", expectBlock: true},
		{name: "active_allows_explicit_impl_task", status: "active", phaseType: "implementation", expectBlock: false},
		{name: "planning_allows_explicit_planning_task", status: "planning", phaseType: "planning", expectBlock: false},
		{name: "planning_allows_explicit_research_task", status: "planning", phaseType: "research", expectBlock: false},
		{name: "planning_allows_explicit_ingest_task", status: "planning", phaseType: "ingest", expectBlock: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			festDir, taskRel := setupTaskFixture(t, tc.status, tc.phaseType)
			if err := os.Chdir(festDir); err != nil {
				t.Fatalf("chdir: %v", err)
			}

			cmd := &cobra.Command{}
			ctx := scope.WithFestival(context.Background(), festDir)
			cmd.SetContext(ctx)

			// Suppress stdout so the captured block message does not pollute
			// test output. The error value is what we assert on.
			devnull, _ := os.Open(os.DevNull)
			origOut := os.Stdout
			os.Stdout = devnull
			err := runShow(cmd, []string{taskRel})
			os.Stdout = origOut
			_ = devnull.Close()

			if tc.expectBlock {
				if err == nil {
					t.Fatalf("expected lifecycle gate to block fest task show %s, got nil", taskRel)
				}
				if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
					t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
				}
			} else if stderrors.Is(err, errors.ErrAlreadyPrinted) {
				t.Errorf("did not expect lifecycle block, got: %v", err)
			}
		})
	}
}

// setupTaskFixture creates a festival with one phase containing one
// sequence containing one task file. Returns the festival dir and the
// festival-relative task path used as the explicit arg for fest task show.
func setupTaskFixture(t *testing.T, status, phaseType string) (string, string) {
	t.Helper()
	dir := t.TempDir()

	festYAML := "version: \"1.0\"\nname: task-gate-test\nid: TGT-001\nmetadata:\n  id: TGT-001\n  status_history:\n    - status: " + status + "\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	phaseDir := filepath.Join(dir, "001_PHASE")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}

	phaseGoal := "---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: " + phaseType + "\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	seqGoal := "---\nfest_type: sequence_goal\nfest_id: 01_seq\n---\n# Sequence Goal\n"
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatalf("write SEQUENCE_GOAL.md: %v", err)
	}

	taskName := "01_task.md"
	taskBody := "---\nfest_type: task\nfest_id: 01_task\n---\n# Task body\n"
	if err := os.WriteFile(filepath.Join(seqDir, taskName), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	return dir, filepath.Join("001_PHASE", "01_seq", taskName)
}
