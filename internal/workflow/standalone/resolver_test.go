package standalone

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func TestResolve_PropagatesNonNotFoundFestivalError(t *testing.T) {
	prev := resolveFestivalPath
	t.Cleanup(func() { resolveFestivalPath = prev })

	want := festerrors.IO("walking festivals dir", errors.New("permission denied"))
	resolveFestivalPath = func(_, _ string) (string, error) {
		return "", want
	}

	dir := t.TempDir()
	res, err := Resolve(context.Background(), dir)
	if err == nil {
		t.Fatalf("expected error, got nil (res=%+v)", res)
	}
	if res != nil {
		t.Errorf("expected nil result on propagated error, got %+v", res)
	}
	if !errors.Is(err, want) {
		t.Errorf("expected wrapped festErr, got %T: %v", err, err)
	}
}

func writeF(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_None(t *testing.T) {
	dir := t.TempDir()
	r, err := Resolve(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeNone {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeNone)
	}
}

func TestResolve_Festival(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals", "active", "test-TF0001")
	writeF(t, filepath.Join(festDir, "fest.yaml"), `version: "1.0"
metadata:
  id: TF0001
  name: test
`)

	r, err := Resolve(context.Background(), festDir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeFestival {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeFestival)
	}
}

func TestResolve_FestivalPhaseWithWorkflowDoc(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals", "active", "test-TF0002")
	writeF(t, filepath.Join(festDir, "fest.yaml"), `version: "1.0"
metadata:
  id: TF0002
  name: test
`)
	phaseDir := filepath.Join(festDir, "001_INGEST")
	writeF(t, filepath.Join(phaseDir, "PHASE_GOAL.md"), "# phase")
	writeF(t, filepath.Join(phaseDir, "WORKFLOW.md"), "## Step 1: X\n")

	r, err := Resolve(context.Background(), phaseDir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeFestival {
		t.Errorf("Mode = %q, want festival", r.Mode)
	}
	if r.PhasePath == "" {
		t.Errorf("PhasePath should be set inside a phase")
	}
	if r.WorkflowDoc == "" || filepath.Base(r.WorkflowDoc) != "WORKFLOW.md" {
		t.Errorf("WorkflowDoc = %q", r.WorkflowDoc)
	}
}

func TestResolve_Tracked(t *testing.T) {
	dir := t.TempDir()
	writeF(t, filepath.Join(dir, "WORKFLOW.md"), "## Step 1: X\n")
	writeF(t, filepath.Join(dir, ".workflow", "workflow.yaml"), `version: 1
kind: workflow-runtime
workflow_id: wf-x
`)

	r, err := Resolve(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeTracked {
		t.Errorf("Mode = %q, want tracked", r.Mode)
	}
	if r.RuntimeDir == "" {
		t.Errorf("RuntimeDir should be set")
	}
}

func TestResolve_Anonymous(t *testing.T) {
	dir := t.TempDir()
	writeF(t, filepath.Join(dir, "WORKFLOW.md"), "## Step 1: X\n")

	r, err := Resolve(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeAnonymous {
		t.Errorf("Mode = %q, want anonymous", r.Mode)
	}
	if r.RuntimeDir != "" {
		t.Errorf("RuntimeDir should be empty for anonymous, got %q", r.RuntimeDir)
	}
	if filepath.Base(r.WorkflowDoc) != "WORKFLOW.md" {
		t.Errorf("WorkflowDoc = %q", r.WorkflowDoc)
	}
}

func TestResolve_NestedAnonymous(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "wf-parent")
	subdir := filepath.Join(parent, "deeper", "nested")
	writeF(t, filepath.Join(parent, "WORKFLOW.md"), "## Step 1: X\n")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(context.Background(), subdir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeAnonymous {
		t.Errorf("Mode = %q, want anonymous (walk up to parent)", r.Mode)
	}
}

func TestResolve_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Resolve(ctx, dir)
	if err == nil {
		t.Fatal("expected context.Canceled error")
	}
}
