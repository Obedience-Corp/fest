package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const validateValidWorkflow = `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
`

const validateStep0Workflow = `# Workflow
## Step 0: ZERO — z
**Goal:** g
## Step 1: ONE — a
**Goal:** g
`

func TestRunValidate_MissingFile(t *testing.T) {
	dir := t.TempDir()
	err := runValidate(context.Background(), filepath.Join(dir, "WORKFLOW.md"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	var ferr *festerrors.Error
	if !errors.As(err, &ferr) {
		t.Fatalf("expected *festerrors.Error, got %T", err)
	}
	if ferr.Code != festerrors.ErrCodeNotFound {
		t.Errorf("expected code %s, got %s", festerrors.ErrCodeNotFound, ferr.Code)
	}
}

func TestRunValidate_DirectoryPath(t *testing.T) {
	dir := t.TempDir()
	err := runValidate(context.Background(), dir)
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
	var ferr *festerrors.Error
	if !errors.As(err, &ferr) {
		t.Fatalf("expected *festerrors.Error, got %T", err)
	}
	if ferr.Code != festerrors.ErrCodeValidation {
		t.Errorf("expected code %s, got %s", festerrors.ErrCodeValidation, ferr.Code)
	}
}

func TestRunValidate_Step0Fails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(validateStep0Workflow), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := runValidate(context.Background(), path)
	if err == nil {
		t.Fatalf("expected error for Step 0")
	}
	var ferr *festerrors.Error
	if !errors.As(err, &ferr) {
		t.Fatalf("expected *festerrors.Error, got %T", err)
	}
	if !strings.Contains(ferr.Message, "must start at 1") {
		t.Errorf("expected 'must start at 1' in message, got: %s", ferr.Message)
	}
}

func TestRunValidate_ValidPasses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(validateValidWorkflow), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := runValidate(context.Background(), path); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunValidate_DefaultsToCwdWorkflowMd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "WORKFLOW.md"), []byte(validateValidWorkflow), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := runValidate(context.Background(), ""); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
