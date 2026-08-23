package runloop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
)

func writeTrackedWorkflow(t *testing.T, dir, body string) {
	t.Helper()
	doc := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	store := localstore.Open(filepath.Join(dir, ".workflow"), doc)
	if err := store.Init(context.Background(), localstore.InitOptions{WorkflowID: "wf-runloop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}

func TestInspectStandaloneRunnable(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** prove routing.

**Actions:**
1. Do the thing.
`)
	snap, err := Inspect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Kind != "standalone" {
		t.Fatalf("kind = %s", snap.Kind)
	}
	if snap.Label != "ALIGN" {
		t.Fatalf("label = %s", snap.Label)
	}
	got := Classify(snap)
	if got.Outcome != OutcomeRunnable {
		t.Fatalf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
}

func TestInspectStandaloneHumanGate(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: SHIP

**Goal:** needs a person.

**Approval:** human-required
`)
	snap, err := Inspect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := Classify(snap)
	if got.Outcome != OutcomeWaitingHuman {
		t.Fatalf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
}

func TestInspectMissing(t *testing.T) {
	_, err := Inspect(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInspectCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, t.TempDir()); err == nil {
		t.Fatal("expected cancellation")
	}
}
