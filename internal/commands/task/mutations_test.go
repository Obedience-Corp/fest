package task

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// setupActiveFestival builds an active-status festival with one implementation
// task and returns the festival dir plus the task's festival-relative path.
func setupActiveFestival(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	festYAML := "version: \"1.0\"\nname: task-mut-test\nid: TMT-001\nmetadata:\n  id: TMT-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".fest"), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	phaseGoal := "---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(dir, "001_PHASE", "PHASE_GOAL.md"), []byte(phaseGoal), 0o644); err != nil {
		t.Fatalf("write phase goal: %v", err)
	}
	seqGoal := "---\nfest_type: sequence_goal\nfest_id: 01_seq\n---\n# Sequence Goal\n"
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatalf("write sequence goal: %v", err)
	}
	taskBody := "---\nfest_type: task\nfest_id: 01_task\n---\n# Task body\n"
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	return dir, filepath.Join("001_PHASE", "01_seq", "01_task.md")
}

// resetTaskFlags clears the package-level flag variables between subtests since
// the RunE functions are invoked directly and bypass cobra flag parsing.
func resetTaskFlags() {
	completedJSON, completedYes = false, false
	blockedJSON, blockedYes, blockedReason = false, false, ""
	resetJSON, resetYes = false, false
	updateJSON = false
	unblockJSON = false
}

// captureIO redirects stdin to an EOF (non-TTY) pipe and captures stdout for the
// duration of fn, returning whatever fn wrote to stdout. A pipe is never a TTY,
// so the non-interactive refusal paths are exercised deterministically
// regardless of how the test binary is launched.
func captureIO(t *testing.T, fn func()) string {
	t.Helper()
	origIn, origOut := os.Stdin, os.Stdout

	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	_ = wIn.Close() // immediate EOF; rIn is not a terminal

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	os.Stdin, os.Stdout = rIn, wOut
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(rOut)
		done <- string(b)
	}()

	fn()

	_ = wOut.Close()
	os.Stdin, os.Stdout = origIn, origOut
	out := <-done
	_ = rIn.Close()
	return out
}

func taskCmd(t *testing.T, festDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(scope.WithFestival(context.Background(), festDir))
	return cmd
}

func canonicalTaskID(t *testing.T, festDir, taskRel string) string {
	t.Helper()
	id, _, err := resolveExplicitTask(festDir, taskRel)
	if err != nil {
		t.Fatalf("resolve task id: %v", err)
	}
	return id
}

func taskStatus(t *testing.T, festDir, taskID string) (*progress.TaskProgress, bool) {
	t.Helper()
	mgr, err := progress.NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("load manager: %v", err)
	}
	return mgr.GetTaskProgress(taskID)
}

func TestResolveConfirmation(t *testing.T) {
	tests := []struct {
		name          string
		yes           bool
		jsonOut       bool
		wantErr       bool
		wantPrompt    bool
		wantStdoutHas string
	}{
		{name: "json_without_yes_refuses", yes: false, jsonOut: true, wantErr: true, wantStdoutHas: "confirmation required"},
		{name: "non_tty_without_yes_refuses", yes: false, jsonOut: false, wantErr: true},
		{name: "yes_proceeds", yes: true, jsonOut: false, wantErr: false, wantPrompt: false},
		{name: "yes_with_json_proceeds", yes: true, jsonOut: true, wantErr: false, wantPrompt: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPrompt bool
			var gotErr error
			out := captureIO(t, func() {
				gotPrompt, gotErr = resolveConfirmation(tt.yes, tt.jsonOut, "001_PHASE/01_seq/01_task.md",
					"complete this task", "fest task completed --yes")
			})
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("resolveConfirmation() err = %v, wantErr %v", gotErr, tt.wantErr)
			}
			if gotPrompt != tt.wantPrompt {
				t.Errorf("resolveConfirmation() prompt = %v, want %v", gotPrompt, tt.wantPrompt)
			}
			if tt.wantStdoutHas != "" && !strings.Contains(out, tt.wantStdoutHas) {
				t.Errorf("stdout = %q, want it to contain %q", out, tt.wantStdoutHas)
			}
		})
	}
}

func TestRunCompleted_NonTTYRefusesWithoutYes(t *testing.T) {
	resetTaskFlags()
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	_ = captureIO(t, func() { err = runCompleted(taskCmd(t, festDir), []string{taskRel}) })
	if err == nil {
		t.Fatal("expected refusal error without --yes in non-TTY, got nil")
	}

	if task, ok := taskStatus(t, festDir, wantID); ok && task.Status == progress.StatusCompleted {
		t.Fatal("task must NOT be completed when confirmation was refused")
	}
}

func TestRunCompleted_JSONRequiresYes(t *testing.T) {
	resetTaskFlags()
	completedJSON = true
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	out := captureIO(t, func() { err = runCompleted(taskCmd(t, festDir), []string{taskRel}) })
	if err == nil {
		t.Fatal("expected error for --json without --yes, got nil")
	}
	if !strings.Contains(out, "confirmation required") {
		t.Errorf("stdout = %q, want confirmation-required JSON", out)
	}
	if task, ok := taskStatus(t, festDir, wantID); ok && task.Status == progress.StatusCompleted {
		t.Fatal("task must NOT be completed when --yes is absent")
	}
}

func TestRunCompleted_Yes(t *testing.T) {
	resetTaskFlags()
	completedYes = true
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	_ = captureIO(t, func() { err = runCompleted(taskCmd(t, festDir), []string{taskRel}) })
	if err != nil {
		t.Fatalf("runCompleted --yes: unexpected error: %v", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if !ok {
		t.Fatal("task not found after completion")
	}
	if task.Status != progress.StatusCompleted {
		t.Errorf("status = %q, want completed", task.Status)
	}
}

func TestRunCompleted_JSONYes(t *testing.T) {
	resetTaskFlags()
	completedYes = true
	completedJSON = true
	festDir, taskRel := setupActiveFestival(t)

	var err error
	out := captureIO(t, func() { err = runCompleted(taskCmd(t, festDir), []string{taskRel}) })
	if err != nil {
		t.Fatalf("runCompleted --yes --json: unexpected error: %v", err)
	}

	var payload map[string]any
	if decErr := json.Unmarshal([]byte(out), &payload); decErr != nil {
		t.Fatalf("stdout is not valid JSON (%v): %q", decErr, out)
	}
	if payload["success"] != true {
		t.Errorf("json success = %v, want true", payload["success"])
	}
	if payload["status"] != progress.StatusCompleted {
		t.Errorf("json status = %v, want completed", payload["status"])
	}
}

func TestRunBlocked_Yes(t *testing.T) {
	resetTaskFlags()
	blockedYes = true
	blockedReason = "waiting on API spec"
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	_ = captureIO(t, func() { err = runBlocked(taskCmd(t, festDir), []string{taskRel}) })
	if err != nil {
		t.Fatalf("runBlocked --yes: unexpected error: %v", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if !ok {
		t.Fatal("task not found after blocking")
	}
	if task.Status != progress.StatusBlocked {
		t.Errorf("status = %q, want blocked", task.Status)
	}
	if task.BlockerMessage != "waiting on API spec" {
		t.Errorf("blocker = %q, want the supplied reason", task.BlockerMessage)
	}
}

func TestRunReset_Yes(t *testing.T) {
	resetTaskFlags()
	completedYes = true
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	// Complete, then reset back to pending.
	_ = captureIO(t, func() { _ = runCompleted(taskCmd(t, festDir), []string{taskRel}) })

	resetTaskFlags()
	resetYes = true
	var err error
	_ = captureIO(t, func() { err = runReset(taskCmd(t, festDir), []string{taskRel}) })
	if err != nil {
		t.Fatalf("runReset --yes: unexpected error: %v", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if !ok {
		t.Fatal("task not found after reset")
	}
	if task.Status != progress.StatusPending {
		t.Errorf("status = %q, want pending", task.Status)
	}
	if task.Progress != 0 {
		t.Errorf("progress = %d, want 0", task.Progress)
	}
}

func TestRunUpdate(t *testing.T) {
	resetTaskFlags()
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	_ = captureIO(t, func() { err = runUpdate(taskCmd(t, festDir), []string{taskRel, "50%"}) })
	if err != nil {
		t.Fatalf("runUpdate: unexpected error: %v", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if !ok {
		t.Fatal("task not found after update")
	}
	if task.Progress != 50 {
		t.Errorf("progress = %d, want 50", task.Progress)
	}
	if task.Status != progress.StatusInProgress {
		t.Errorf("status = %q, want in_progress", task.Status)
	}
}

func TestRunUpdate_100RequiresGatedCompletion(t *testing.T) {
	resetTaskFlags()
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	var err error
	_ = captureIO(t, func() { err = runUpdate(taskCmd(t, festDir), []string{taskRel, "100"}) })
	if err == nil {
		t.Fatal("runUpdate at 100% unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "fest task completed") {
		t.Fatalf("error = %v, want gated completion guidance", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if ok && task.Status == progress.StatusCompleted {
		t.Fatal("task must not be completed through fest task update 100")
	}
	if ok && task.Progress == 100 {
		t.Fatal("task progress must not reach 100 through fest task update")
	}
}

func TestRunUnblock(t *testing.T) {
	resetTaskFlags()
	blockedYes = true
	blockedReason = "need a decision"
	festDir, taskRel := setupActiveFestival(t)
	wantID := canonicalTaskID(t, festDir, taskRel)

	_ = captureIO(t, func() { _ = runBlocked(taskCmd(t, festDir), []string{taskRel}) })

	resetTaskFlags()
	var err error
	_ = captureIO(t, func() { err = runUnblock(taskCmd(t, festDir), []string{taskRel}) })
	if err != nil {
		t.Fatalf("runUnblock: unexpected error: %v", err)
	}

	task, ok := taskStatus(t, festDir, wantID)
	if !ok {
		t.Fatal("task not found after unblock")
	}
	if task.BlockerMessage != "" {
		t.Errorf("blocker = %q, want empty after unblock", task.BlockerMessage)
	}
	if task.Status != progress.StatusInProgress {
		t.Errorf("status = %q, want in_progress after unblock", task.Status)
	}
}

func TestParsePercent(t *testing.T) {
	valid := []struct {
		in   string
		want int
	}{
		{"0", 0}, {"0%", 0}, {"50", 50}, {"50%", 50}, {"100", 100}, {"100%", 100}, {" 75% ", 75},
	}
	for _, tt := range valid {
		got, err := parsePercent(tt.in)
		if err != nil {
			t.Errorf("parsePercent(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePercent(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}

	invalid := []string{"-1", "101", "abc", "", "50.5"}
	for _, in := range invalid {
		if _, err := parsePercent(in); err == nil {
			t.Errorf("parsePercent(%q) expected error, got nil", in)
		}
	}
}

func TestStatusForPercent(t *testing.T) {
	tests := []struct {
		pct  int
		want string
	}{
		{0, progress.StatusPending},
		{1, progress.StatusInProgress},
		{99, progress.StatusInProgress},
		{100, progress.StatusCompleted},
	}
	for _, tt := range tests {
		if got := statusForPercent(tt.pct); got != tt.want {
			t.Errorf("statusForPercent(%d) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}
