package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestParseSkipTerminalState(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    wf.StepStatus
		wantErr bool
	}{
		{name: "skipped", input: "skipped", want: wf.StepStatusSkipped},
		{name: "completed", input: "completed", want: wf.StepStatusCompleted},
		{name: "case-insensitive", input: " SkIpPeD ", want: wf.StepStatusSkipped},
		{name: "invalid", input: "done", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSkipTerminalState(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSkipTerminalState() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseSkipTerminalState() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRunSkipRequiresTTY(t *testing.T) {
	origTTYCheck := workflowSkipTTYCheck
	workflowSkipTTYCheck = func(fd int) bool { return false }
	t.Cleanup(func() {
		workflowSkipTTYCheck = origTTYCheck
	})

	err := runSkip(context.Background(), "external completion", wf.StepStatusSkipped)
	if err == nil {
		t.Fatal("expected TTY error, got nil")
	}
	if !strings.Contains(err.Error(), "interactive TTY") {
		t.Fatalf("expected interactive TTY error, got: %v", err)
	}
}

func TestRunSkipAppliesSkippedStateToRemainingSteps(t *testing.T) {
	origTTYCheck := workflowSkipTTYCheck
	workflowSkipTTYCheck = func(fd int) bool { return true }
	t.Cleanup(func() {
		workflowSkipTTYCheck = origTTYCheck
	})

	festDir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir phase: %v", err)
	}

	reason := "phase completed outside fest next loop"
	if err := runSkip(context.Background(), reason, wf.StepStatusSkipped); err != nil {
		t.Fatalf("runSkip() error = %v", err)
	}

	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator() error = %v", err)
	}
	state := nav.GetWorkflowState()
	if !state.IsComplete() {
		t.Fatal("workflow should be complete after skip command")
	}

	for i := 1; i <= state.TotalSteps; i++ {
		stepState := state.GetStepState(i)
		if stepState == nil {
			t.Fatalf("step %d state is nil", i)
		}
		if stepState.Status != wf.StepStatusSkipped {
			t.Fatalf("step %d status = %s, want %s", i, stepState.Status, wf.StepStatusSkipped)
		}
		if stepState.Feedback != reason {
			t.Fatalf("step %d feedback = %q, want %q", i, stepState.Feedback, reason)
		}
	}
}

func TestRunSkipAppliesCompletedStateWhenRequested(t *testing.T) {
	origTTYCheck := workflowSkipTTYCheck
	workflowSkipTTYCheck = func(fd int) bool { return true }
	t.Cleanup(func() {
		workflowSkipTTYCheck = origTTYCheck
	})

	festDir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir phase: %v", err)
	}

	reason := "legacy work already verified"
	if err := runSkip(context.Background(), reason, wf.StepStatusCompleted); err != nil {
		t.Fatalf("runSkip() error = %v", err)
	}

	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator() error = %v", err)
	}
	state := nav.GetWorkflowState()
	if !state.IsComplete() {
		t.Fatal("workflow should be complete after skip command")
	}

	for i := 1; i <= state.TotalSteps; i++ {
		stepState := state.GetStepState(i)
		if stepState == nil {
			t.Fatalf("step %d state is nil", i)
		}
		if stepState.Status != wf.StepStatusCompleted {
			t.Fatalf("step %d status = %s, want %s", i, stepState.Status, wf.StepStatusCompleted)
		}
		if stepState.Feedback != reason {
			t.Fatalf("step %d feedback = %q, want %q", i, stepState.Feedback, reason)
		}
	}
}
