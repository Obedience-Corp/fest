package next

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
)

func TestCheckPreActiveStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		phaseType      string
		expectBlocked  bool
		expectContains string
	}{
		{
			name:           "planning+implementation is blocked",
			status:         "planning",
			phaseType:      "implementation",
			expectBlocked:  true,
			expectContains: "festival is in planning status",
		},
		{
			name:           "planning+review is blocked",
			status:         "planning",
			phaseType:      "review",
			expectBlocked:  true,
			expectContains: "festival is in planning status",
		},
		{
			name:          "planning+ingest is allowed",
			status:        "planning",
			phaseType:     "ingest",
			expectBlocked: false,
		},
		{
			name:          "planning+research is allowed",
			status:        "planning",
			phaseType:     "research",
			expectBlocked: false,
		},
		{
			name:          "planning+planning is allowed",
			status:        "planning",
			phaseType:     "planning",
			expectBlocked: false,
		},
		{
			name:          "active+implementation is allowed",
			status:        "active",
			phaseType:     "implementation",
			expectBlocked: false,
		},
		{
			name:          "active+review is allowed",
			status:        "active",
			phaseType:     "review",
			expectBlocked: false,
		},
		{
			name:           "ready+implementation is blocked",
			status:         "ready",
			phaseType:      "implementation",
			expectBlocked:  true,
			expectContains: "festival is in ready status",
		},
		{
			name:           "ready+review is blocked",
			status:         "ready",
			phaseType:      "review",
			expectBlocked:  true,
			expectContains: "festival is in ready status",
		},
		{
			name:          "ready+ingest is allowed",
			status:        "ready",
			phaseType:     "ingest",
			expectBlocked: false,
		},
		{
			name:          "ready+research is allowed",
			status:        "ready",
			phaseType:     "research",
			expectBlocked: false,
		},
		{
			name:          "ready+planning is allowed",
			status:        "ready",
			phaseType:     "planning",
			expectBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festDir := t.TempDir()
			phaseDir := filepath.Join(festDir, "001_PHASE")
			if err := os.MkdirAll(phaseDir, 0755); err != nil {
				t.Fatal(err)
			}

			festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: " + tt.status + "\n      timestamp: 2026-02-10T00:00:00Z\n"
			if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
				t.Fatal(err)
			}

			goalContent := "---\nfest_phase_type: " + tt.phaseType + "\n---\n# Phase Goal\n"
			if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
				t.Fatal(err)
			}

			err := checkPreActiveStatus(context.Background(), festDir, phaseDir)

			if tt.expectBlocked {
				if err == nil {
					t.Error("expected blocked error, got nil")
				} else if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
					t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestCheckPreActiveStatus_NoConfig(t *testing.T) {
	festDir := t.TempDir()
	err := checkPreActiveStatus(context.Background(), festDir, "")
	if err != nil {
		t.Errorf("expected no error without config, got: %v", err)
	}
}

func TestCheckPreActiveStatus_NoPhasePath(t *testing.T) {
	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_IMPL")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: planning\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(phaseDir, "01_setup"), 0755); err != nil {
		t.Fatal(err)
	}

	err := checkPreActiveStatus(context.Background(), festDir, "")
	if err == nil {
		t.Error("expected blocked error when auto-detecting implementation phase, got nil")
	}
}

func TestCheckPreActiveStatus_ReadyMessage(t *testing.T) {
	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_PHASE")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: ready\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	goalContent := "---\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goalContent), 0644); err != nil {
		t.Fatal(err)
	}

	var err error
	output := captureStdout(t, func() {
		err = checkPreActiveStatus(context.Background(), festDir, phaseDir)
	})

	if err == nil {
		t.Fatal("expected blocked error for ready+implementation, got nil")
	}
	if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
		t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
	}
	if !strings.Contains(output, "STOP") {
		t.Errorf("expected prompt block header, got: %s", output)
	}
	if !strings.Contains(output, "fest promote") {
		t.Errorf("expected promote instruction in prompt block, got: %s", output)
	}
	if !strings.Contains(output, "Did the user approve") {
		t.Errorf("expected user approval check in prompt block, got: %s", output)
	}
	if !strings.Contains(output, "planning -> ready -> [active] -> completed") {
		t.Errorf("expected lifecycle in prompt block, got: %s", output)
	}
}

// TestCheckPreActiveStatus_MultiPhase_PlanningPhaseNotBlocked verifies the
// regression scenario: a planning-status festival with a completed implementation
// phase (001_IMPL) and an incomplete planning phase (002_PLAN). When run from the
// festival root (no explicit phasePath), findFirstIncompletePhase should find
// 002_PLAN (type=planning), NOT 001_IMPL (type=implementation), so the check
// should NOT block.
func TestCheckPreActiveStatus_MultiPhase_PlanningPhaseNotBlocked(t *testing.T) {
	festDir := t.TempDir()

	phase1 := filepath.Join(festDir, "001_IMPL")
	if err := os.MkdirAll(phase1, 0755); err != nil {
		t.Fatal(err)
	}
	goalImpl := "---\nfest_phase_type: implementation\n---\n# Implementation Phase\n"
	if err := os.WriteFile(filepath.Join(phase1, "PHASE_GOAL.md"), []byte(goalImpl), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phase1, ".phase_complete"), []byte("done"), 0644); err != nil {
		t.Fatal(err)
	}

	phase2 := filepath.Join(festDir, "002_PLAN")
	if err := os.MkdirAll(phase2, 0755); err != nil {
		t.Fatal(err)
	}
	goalPlan := "---\nfest_phase_type: planning\n---\n# Planning Phase\n"
	if err := os.WriteFile(filepath.Join(phase2, "PHASE_GOAL.md"), []byte(goalPlan), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(phase2, "01_research"), 0755); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: MP0001\n  status_history:\n    - status: planning\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	err := checkPreActiveStatus(context.Background(), festDir, "")
	if err != nil {
		t.Errorf("expected no block for planning phase in planning-status festival, got: %v", err)
	}
}
