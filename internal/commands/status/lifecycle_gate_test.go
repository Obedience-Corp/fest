package status

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// TestPhaseStatusSetGate is the regression test for the bypass that the
// initial fix missed: an agent could mark an implementation phase
// completed via fest status set --phase in a planning festival, walking
// the phase frontmatter without ever consulting the lifecycle gate.
func TestPhaseStatusSetGate(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		phaseType   string
		expectBlock bool
	}{
		{name: "planning_blocks_impl_phase_completed", status: "planning", phaseType: "implementation", expectBlock: true},
		{name: "ready_blocks_review_phase_completed", status: "ready", phaseType: "review", expectBlock: true},
		{name: "active_allows_impl_phase_completed", status: "active", phaseType: "implementation", expectBlock: false},
		{name: "planning_allows_ingest_phase_completed", status: "planning", phaseType: "ingest", expectBlock: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			festDir := setupPhaseFixture(t, tc.status, tc.phaseType)
			if err := os.Chdir(festDir); err != nil {
				t.Fatalf("chdir: %v", err)
			}

			err := handlePhaseStatusSet(context.Background(), &ui.UI{},
				festDir, "completed", &statusOptions{phase: "001_PHASE", noCommit: true})

			if tc.expectBlock {
				if err == nil {
					t.Fatal("expected lifecycle gate to block, got nil")
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

func setupPhaseFixture(t *testing.T, status, phaseType string) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := "name: status-gate-test\nid: SGT-001\nversion: \"1.0\"\nmetadata:\n  id: SGT-001\n  status_history:\n    - status: " + status + "\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	phaseDir := filepath.Join(dir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}

	phaseGoal := "---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: " + phaseType + "\nfest_status: planning\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}

	return dir
}
