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
workitem_id: test-001
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

	if _, err := os.Stat(filepath.Join(dir, ".workitem")); err != nil {
		t.Errorf(".workitem missing: %v", err)
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

func TestEnsureTracked_RefusesExistingWorkitem(t *testing.T) {
	res, dir := setupAnonymousFixture(t)
	if err := os.WriteFile(filepath.Join(dir, ".workitem"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := EnsureTracked(context.Background(), res, BootstrapOptions{})
	if err == nil {
		t.Fatal("expected refusal when .workitem exists")
	}
	if !strings.Contains(err.Error(), ".workitem") {
		t.Errorf("error should mention .workitem: %v", err)
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

func TestEnsureTracked_CustomWorkitemID(t *testing.T) {
	res, dir := setupAnonymousFixture(t)
	_, err := EnsureTracked(context.Background(), res, BootstrapOptions{WorkitemID: "custom-id-123"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, ".workitem"))
	if !strings.Contains(string(raw), "id: custom-id-123") {
		t.Errorf(".workitem missing custom id: %s", raw)
	}
}
