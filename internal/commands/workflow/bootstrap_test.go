package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

func setupAnonymousFixture(t *testing.T) (*standalone.Result, string) {
	t.Helper()
	dir := t.TempDir()
	doc := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(doc, []byte(`---
workflow_version: 1
workflow_id: wf-test
---

## Step 1: PLAN

**Goal:** test.

**Actions:**
1. Do it.

**Output:** done
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return &standalone.Result{
		Mode:        standalone.ModeAnonymous,
		StartDir:    dir,
		WorkflowDoc: doc,
	}, dir
}

func TestEnsureTracked_HappyPath(t *testing.T) {
	res, dir := setupAnonymousFixture(t)
	out, err := EnsureTracked(context.Background(), res, BootstrapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != standalone.ModeTracked {
		t.Errorf("Mode = %q, want tracked", out.Mode)
	}

	// D007: fest must NOT write .workitem.
	if _, err := os.Stat(filepath.Join(dir, ".workitem")); err == nil {
		t.Errorf(".workitem should not exist after bootstrap (D007: fest never writes .workitem)")
	}
	if _, err := os.Stat(filepath.Join(dir, ".workflow", "workflow.yaml")); err != nil {
		t.Errorf("workflow.yaml missing: %v", err)
	}

	// Run should be active.
	store := localstore.Open(filepath.Join(dir, ".workflow"), filepath.Join(dir, "WORKFLOW.md"))
	state, err := store.LoadActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.RunID == "" {
		t.Errorf("no active run after bootstrap: %+v", state)
	}
}

func TestEnsureTracked_NoBootstrap(t *testing.T) {
	res, _ := setupAnonymousFixture(t)
	_, err := EnsureTracked(context.Background(), res, BootstrapOptions{NoBootstrap: true})
	if err == nil {
		t.Fatal("expected error with NoBootstrap=true")
	}
	if !strings.Contains(err.Error(), "no-bootstrap") {
		t.Errorf("error should mention no-bootstrap: %v", err)
	}
}

func TestEnsureTracked_RefusesExistingWorkflow(t *testing.T) {
	res, dir := setupAnonymousFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, ".workflow"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := EnsureTracked(context.Background(), res, BootstrapOptions{})
	if err == nil {
		t.Fatal("expected refusal when .workflow/ exists")
	}
	if !strings.Contains(err.Error(), ".workflow") {
		t.Errorf("error should mention .workflow: %v", err)
	}
}

func TestEnsureTracked_NonAnonymousIsNoop(t *testing.T) {
	res := &standalone.Result{Mode: standalone.ModeTracked, WorkflowDoc: "/tmp/x"}
	out, err := EnsureTracked(context.Background(), res, BootstrapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if out != res {
		t.Errorf("non-anonymous Result should pass through unchanged")
	}
}

// TestEnsureTracked_RejectsEmptyWorkflowDoc locks in D030 finding #7:
// bootstrap must validate WORKFLOW.md before creating .workflow/.
func TestEnsureTracked_RejectsEmptyWorkflowDoc(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(doc, []byte("# No steps here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &standalone.Result{Mode: standalone.ModeAnonymous, StartDir: dir, WorkflowDoc: doc}
	_, err := EnsureTracked(context.Background(), res, BootstrapOptions{})
	if err == nil {
		t.Fatal("expected validation error for WORKFLOW.md with no steps")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".workflow")); statErr == nil {
		t.Errorf(".workflow/ should not exist after validation failure")
	}
}
