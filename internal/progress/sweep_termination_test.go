package progress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReblockKeepsDeferral pins the asymmetry the design asks for: unblock,
// complete and reset end the blocked state and clear the deferral, but a
// re-block re-enters a state the operator already ruled on, so the deferral
// survives.
func TestReblockKeepsDeferral(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	if err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit()); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, deferTestTaskID, "still gone on the second attempt", []string{"tried the v2 endpoint"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}

	task, ok := reloadManager(t, festivalPath).GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if task.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", task.Status, StatusBlocked)
	}
	if !task.BlockerDeferred {
		t.Error("BlockerDeferred = false; a re-block during a sweep must not undo the operator's decision")
	}
	if task.DeferralReason != "the vendor replies next week" {
		t.Errorf("DeferralReason = %q, want the operator reason preserved", task.DeferralReason)
	}
	if task.BlockerMessage != "still gone on the second attempt" {
		t.Errorf("BlockerMessage = %q, want the new blocker", task.BlockerMessage)
	}
	if len(task.BlockerAttempts) != 1 || task.BlockerAttempts[0] != "tried the v2 endpoint" {
		t.Errorf("BlockerAttempts = %v, want the new attempt", task.BlockerAttempts)
	}
}

func TestCompletionsInSweepCountsOnlyThatSweep(t *testing.T) {
	events := []ProgressEvent{
		{Event: EventCompleted, Task: "a"},
		{Event: EventSweepStarted, Sweep: 1},
		{Event: EventCompleted, Task: "b"},
		{Event: EventSweepStarted, Sweep: 2},
		{Event: EventCompleted, Task: "c"},
		{Event: EventCompleted, Task: "d"},
	}

	if got := completionsInSweep(events, 1); got != 1 {
		t.Errorf("completionsInSweep(1) = %d, want 1", got)
	}
	if got := completionsInSweep(events, 2); got != 2 {
		t.Errorf("completionsInSweep(2) = %d, want 2", got)
	}
	if got := completionsInSweep(events, 3); got != 0 {
		t.Errorf("completionsInSweep(3) = %d, want 0", got)
	}
}

// TestSweepCompletionReopensAGatePassedWhileDeferred is design doc 05 scenario
// H4: a gate that passed while a task was deferred is returned to pending when
// that task completes in a sweep, so fest next offers it again and its own
// evaluator runs against the changed inputs.
func TestSweepCompletionReopensAGatePassedWhileDeferred(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	const gateID = "001_PHASE/01_sequence/04_testing.md"
	gateBody := "---\nfest_type: gate\nfest_gate_type: testing\nfest_status: pending\n---\n\n# Gate: Testing\n"
	if err := os.WriteFile(filepath.Join(festivalPath, filepath.FromSlash(gateID)), []byte(gateBody), 0o644); err != nil {
		t.Fatalf("writing the gate task: %v", err)
	}

	if err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit()); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, "001_PHASE/01_sequence/02_task.md"); err != nil {
		t.Fatalf("MarkComplete(sibling) error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, gateID); err != nil {
		t.Fatalf("MarkComplete(gate) error = %v", err)
	}
	if err := mgr.StartSweep(ctx, 1); err != nil {
		t.Fatalf("StartSweep() error = %v", err)
	}

	if err := mgr.MarkComplete(ctx, deferTestTaskID); err != nil {
		t.Fatalf("MarkComplete(deferred) error = %v", err)
	}

	reloaded := reloadManager(t, festivalPath)

	gate, ok := reloaded.GetTaskProgress(gateID)
	if !ok {
		t.Fatal("gate missing from the store")
	}
	if gate.Status != StatusPending {
		t.Errorf("gate status = %q, want %q so the gate is evaluated again", gate.Status, StatusPending)
	}
	if gate.CompletedAt != nil {
		t.Errorf("gate CompletedAt = %v, want nil after reopening", gate.CompletedAt)
	}

	sibling, ok := reloaded.GetTaskProgress("001_PHASE/01_sequence/02_task.md")
	if !ok {
		t.Fatal("sibling missing from the store")
	}
	if sibling.Status != StatusCompleted {
		t.Errorf("sibling status = %q, want ordinary completed work left alone", sibling.Status)
	}

	deferred, ok := reloaded.GetTaskProgress(deferTestTaskID)
	if !ok {
		t.Fatal("the completed task is missing from the store")
	}
	if deferred.Status != StatusCompleted || deferred.BlockerDeferred {
		t.Errorf("completed task = %+v, want completed with the deferral cleared", deferred)
	}

	onDisk, err := os.ReadFile(filepath.Join(festivalPath, filepath.FromSlash(gateID)))
	if err != nil {
		t.Fatalf("reading the gate task: %v", err)
	}
	if !strings.Contains(string(onDisk), "fest_status: pending") {
		t.Errorf("gate frontmatter was not synced back to pending:\n%s", onDisk)
	}
}

func TestSweepCompletionLeavesGatesAloneOutsideASweep(t *testing.T) {
	ctx := t.Context()
	festivalPath, mgr := blockedDeferFixture(t)

	const gateID = "001_PHASE/01_sequence/04_testing.md"
	gateBody := "---\nfest_type: gate\nfest_gate_type: testing\nfest_status: pending\n---\n\n# Gate: Testing\n"
	if err := os.WriteFile(filepath.Join(festivalPath, filepath.FromSlash(gateID)), []byte(gateBody), 0o644); err != nil {
		t.Fatalf("writing the gate task: %v", err)
	}

	if err := mgr.DeferBlocker(ctx, deferTestTaskID, "the vendor replies next week", deferTestAudit()); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, gateID); err != nil {
		t.Fatalf("MarkComplete(gate) error = %v", err)
	}
	if err := mgr.MarkComplete(ctx, deferTestTaskID); err != nil {
		t.Fatalf("MarkComplete(deferred) error = %v", err)
	}

	gate, ok := reloadManager(t, festivalPath).GetTaskProgress(gateID)
	if !ok {
		t.Fatal("gate missing from the store")
	}
	if gate.Status != StatusCompleted {
		t.Errorf("gate status = %q, want %q when no sweep is running", gate.Status, StatusCompleted)
	}
}
