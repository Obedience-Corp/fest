package lifecycle

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
)

func TestEnforcePreActive(t *testing.T) {
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

			err := EnforcePreActive(context.Background(), festDir, EnforceOptions{PhasePath: phaseDir})

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

func TestEnforcePreActive_MalformedConfig_FailsClosed(t *testing.T) {
	festDir := t.TempDir()
	// Write malformed YAML so config.LoadFestivalConfig returns a parse error.
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(": not yaml :\n  - [broken\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var err error
	output := captureStdout(t, func() {
		err = EnforcePreActive(context.Background(), festDir, EnforceOptions{Reason: "test"})
	})

	if err == nil {
		t.Fatal("expected fail-closed error for malformed fest.yaml, got nil")
	}
	if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
		t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
	}
	if !strings.Contains(output, "fest.yaml") {
		t.Errorf("expected fest.yaml hint in output, got: %s", output)
	}
	if !strings.Contains(output, "fest validate") {
		t.Errorf("expected 'fest validate' hint, got: %s", output)
	}
}

func TestEnforcePreActive_NoConfig_FailsClosed(t *testing.T) {
	festDir := t.TempDir()

	var err error
	output := captureStdout(t, func() {
		err = EnforcePreActive(context.Background(), festDir, EnforceOptions{Reason: "test"})
	})

	if err == nil {
		t.Fatal("expected fail-closed error when fest.yaml is missing, got nil")
	}
	if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
		t.Errorf("expected ErrAlreadyPrinted, got: %v", err)
	}
	if !strings.Contains(output, "metadata missing") && !strings.Contains(output, "fest.yaml") {
		t.Errorf("expected metadata/fest.yaml hint in output, got: %s", output)
	}
	if !strings.Contains(output, "fest validate") {
		t.Errorf("expected 'fest validate' hint in output, got: %s", output)
	}
}

func TestEnforcePreActive_NoPhasePath_AutoDetect(t *testing.T) {
	festDir := t.TempDir()
	phaseDir := filepath.Join(festDir, "001_IMPL")
	if err := os.MkdirAll(filepath.Join(phaseDir, "01_setup"), 0755); err != nil {
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

	err := EnforcePreActive(context.Background(), festDir, EnforceOptions{})
	if err == nil {
		t.Error("expected blocked error when auto-detecting implementation phase, got nil")
	}
}

func TestEnforcePreActive_TaskID_DerivesPhase(t *testing.T) {
	festDir := t.TempDir()

	implPhase := filepath.Join(festDir, "002_IMPL")
	if err := os.MkdirAll(filepath.Join(implPhase, "01_seq"), 0755); err != nil {
		t.Fatal(err)
	}
	goalImpl := "---\nfest_phase_type: implementation\n---\n# Implementation Phase\n"
	if err := os.WriteFile(filepath.Join(implPhase, "PHASE_GOAL.md"), []byte(goalImpl), 0644); err != nil {
		t.Fatal(err)
	}

	planPhase := filepath.Join(festDir, "001_PLAN")
	if err := os.MkdirAll(filepath.Join(planPhase, "01_seq"), 0755); err != nil {
		t.Fatal(err)
	}
	goalPlan := "---\nfest_phase_type: planning\n---\n# Planning Phase\n"
	if err := os.WriteFile(filepath.Join(planPhase, "PHASE_GOAL.md"), []byte(goalPlan), 0644); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: TS0001\n  status_history:\n    - status: planning\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// TaskID points at the implementation phase even though planning phase is the
	// scan-detected first incomplete. TaskID should win.
	err := EnforcePreActive(context.Background(), festDir, EnforceOptions{
		TaskID: "002_IMPL/01_seq/01_task.md",
		Reason: "fest task completed",
	})
	if err == nil {
		t.Error("expected block when TaskID resolves to implementation phase, got nil")
	}

	// TaskID points at the planning phase, should NOT block.
	err = EnforcePreActive(context.Background(), festDir, EnforceOptions{
		TaskID: "001_PLAN/01_seq/01_task.md",
	})
	if err != nil {
		t.Errorf("expected no block when TaskID resolves to planning phase, got: %v", err)
	}
}

func TestEnforcePreActive_ReadyMessage(t *testing.T) {
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
		err = EnforcePreActive(context.Background(), festDir, EnforceOptions{
			PhasePath: phaseDir,
			Reason:    "fest task completed",
		})
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
		t.Errorf("expected promote instruction, got: %s", output)
	}
	if !strings.Contains(output, "Did the user approve") {
		t.Errorf("expected user approval check, got: %s", output)
	}
	if !strings.Contains(output, "planning -> ready -> [active] -> completed") {
		t.Errorf("expected lifecycle in prompt block, got: %s", output)
	}
	if !strings.Contains(output, "fest task completed") {
		t.Errorf("expected reason in output, got: %s", output)
	}
}

func TestEnforcePreActive_MultiPhase_PlanningPhaseNotBlocked(t *testing.T) {
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

	err := EnforcePreActive(context.Background(), festDir, EnforceOptions{})
	if err != nil {
		t.Errorf("expected no block for planning phase in planning-status festival, got: %v", err)
	}
}

func TestEnforcePreActive_CwdInsideLaterImplPhase(t *testing.T) {
	festDir := t.TempDir()

	ingestPhase := filepath.Join(festDir, "001_INGEST")
	if err := os.MkdirAll(filepath.Join(ingestPhase, "01_seq"), 0755); err != nil {
		t.Fatal(err)
	}
	goalIngest := "---\nfest_phase_type: ingest\n---\n# Ingest Phase\n"
	if err := os.WriteFile(filepath.Join(ingestPhase, "PHASE_GOAL.md"), []byte(goalIngest), 0644); err != nil {
		t.Fatal(err)
	}

	implPhase := filepath.Join(festDir, "002_IMPL")
	if err := os.MkdirAll(filepath.Join(implPhase, "01_seq"), 0755); err != nil {
		t.Fatal(err)
	}
	goalImpl := "---\nfest_phase_type: implementation\n---\n# Implementation Phase\n"
	if err := os.WriteFile(filepath.Join(implPhase, "PHASE_GOAL.md"), []byte(goalImpl), 0644); err != nil {
		t.Fatal(err)
	}

	festYAML := "version: \"1.0\"\nmetadata:\n  id: CW0001\n  status_history:\n    - status: planning\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Passing the impl phase path WOULD block.
	err := EnforcePreActive(context.Background(), festDir, EnforceOptions{PhasePath: implPhase})
	if err == nil {
		t.Error("expected block when passing implementation phase path directly")
	}

	// Passing the ingest phase path should NOT block.
	err = EnforcePreActive(context.Background(), festDir, EnforceOptions{PhasePath: ingestPhase})
	if err != nil {
		t.Errorf("expected no block when passing ingest phase path, got: %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write pipe: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read pipe: %v", err)
	}

	return buf.String()
}
