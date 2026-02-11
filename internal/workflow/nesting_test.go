package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHasParentFlow_NoParent(t *testing.T) {
	// Create a temp directory with no workflow
	root := t.TempDir()
	subdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	path, found := HasParentFlow(subdir)
	if found {
		t.Errorf("expected no parent flow, but found: %s", path)
	}
}

func TestHasParentFlow_WithParent(t *testing.T) {
	// Create a temp directory with a workflow at root
	root := t.TempDir()

	// Create a .workflow.yaml at root
	schemaPath := filepath.Join(root, SchemaFileName)
	schemaContent := `version: 1
type: status-workflow
directories:
  active:
    description: "Work in progress"
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create a nested directory
	nestedDir := filepath.Join(root, "nested", "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	// Check from nested directory - should find parent flow
	path, found := HasParentFlow(nestedDir)
	if !found {
		t.Error("expected to find parent flow, but didn't")
	}
	if path != schemaPath {
		t.Errorf("expected parent path %s, got %s", schemaPath, path)
	}
}

func TestHasParentFlow_AtFlowRoot(t *testing.T) {
	// Create a temp directory with a workflow at root
	root := t.TempDir()

	// Create a .workflow.yaml at root
	schemaPath := filepath.Join(root, SchemaFileName)
	schemaContent := `version: 1
type: status-workflow
directories:
  active:
    description: "Work in progress"
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Check from the flow root itself - should NOT find a parent (it's the current flow, not parent)
	path, found := HasParentFlow(root)
	if found {
		t.Errorf("expected no parent flow at flow root, but found: %s", path)
	}
}

func TestInit_FlowNesting_Blocked(t *testing.T) {
	// Create a temp directory with an existing workflow
	root := t.TempDir()

	// Initialize a workflow at root
	svc := NewService(root)
	ctx := context.Background()
	_, err := svc.Init(ctx, InitOptions{})
	if err != nil {
		t.Fatalf("failed to init parent workflow: %v", err)
	}

	// Try to create a nested workflow - should fail
	nestedDir := filepath.Join(root, "nested", "subflow")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	nestedSvc := NewService(nestedDir)
	_, err = nestedSvc.Init(ctx, InitOptions{})
	if err == nil {
		t.Fatal("expected error when creating nested flow, got nil")
	}

	// Verify it's the right error
	if !errors.Is(err, ErrFlowNested) {
		t.Errorf("expected ErrFlowNested, got: %v", err)
	}

	// Verify FlowNestedError contains parent path
	var nestedErr *FlowNestedError
	if !errors.As(err, &nestedErr) {
		t.Errorf("expected *FlowNestedError, got: %T", err)
	} else {
		expectedPath := filepath.Join(root, SchemaFileName)
		if nestedErr.ParentSchemaPath != expectedPath {
			t.Errorf("expected parent path %s, got %s", expectedPath, nestedErr.ParentSchemaPath)
		}
	}
}

func TestInit_FlowNesting_SiblingAllowed(t *testing.T) {
	// Create a temp directory structure
	root := t.TempDir()
	workflowA := filepath.Join(root, "workflow-a")
	workflowB := filepath.Join(root, "workflow-b")

	if err := os.MkdirAll(workflowA, 0755); err != nil {
		t.Fatalf("failed to create workflow-a: %v", err)
	}
	if err := os.MkdirAll(workflowB, 0755); err != nil {
		t.Fatalf("failed to create workflow-b: %v", err)
	}

	ctx := context.Background()

	// Initialize workflow A
	svcA := NewService(workflowA)
	_, err := svcA.Init(ctx, InitOptions{})
	if err != nil {
		t.Fatalf("failed to init workflow A: %v", err)
	}

	// Initialize workflow B - should succeed (sibling, not nested)
	svcB := NewService(workflowB)
	_, err = svcB.Init(ctx, InitOptions{})
	if err != nil {
		t.Fatalf("expected sibling workflow to succeed, got: %v", err)
	}
}

func TestFlowNestedError_Message(t *testing.T) {
	err := &FlowNestedError{ParentSchemaPath: "/path/to/parent/.workflow.yaml"}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	// Verify message contains key information
	if !contains(msg, "cannot create flow inside existing flow") {
		t.Error("message should mention nesting prohibition")
	}
	if !contains(msg, "/path/to/parent/.workflow.yaml") {
		t.Error("message should contain parent path")
	}
}

func TestFlowNestedError_Unwrap(t *testing.T) {
	err := &FlowNestedError{ParentSchemaPath: "/test/path"}

	if !errors.Is(err, ErrFlowNested) {
		t.Error("FlowNestedError should unwrap to ErrFlowNested")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
