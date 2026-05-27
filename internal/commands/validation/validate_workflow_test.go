package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const validWorkflow = `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
`

const step0Workflow = `# Workflow
## Step 0: ZERO — z
**Goal:** g
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
`

const gapWorkflow = `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
## Step 4: FOUR — d
**Goal:** g
`

func TestValidateWorkflowDocsChecks_DetectsStep0(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, "001_PHASE", "WORKFLOW.md"), step0Workflow)

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(result.Issues), result.Issues)
	}
	issue := result.Issues[0]
	if issue.Code != CodeWorkflowNumbering {
		t.Errorf("expected code %s, got %s", CodeWorkflowNumbering, issue.Code)
	}
	if issue.Level != LevelError {
		t.Errorf("expected level error, got %s", issue.Level)
	}
	if !strings.Contains(issue.Message, "must start at 1") {
		t.Errorf("expected message about starting at 1, got: %s", issue.Message)
	}
	if !strings.Contains(issue.Message, "Step 0, Step 1, Step 2") {
		t.Errorf("expected found sequence in message, got: %s", issue.Message)
	}
	if !strings.Contains(issue.Fix, "merge Step 0") {
		t.Errorf("expected remediation in fix, got: %s", issue.Fix)
	}
}

func TestValidateWorkflowDocsChecks_DetectsGap(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, "001_PHASE", "WORKFLOW.md"), gapWorkflow)

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(result.Issues), result.Issues)
	}
	if !strings.Contains(result.Issues[0].Message, "not contiguous") {
		t.Errorf("expected 'not contiguous', got: %s", result.Issues[0].Message)
	}
}

func TestValidateWorkflowDocsChecks_PassesValidDoc(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, "001_PHASE", "WORKFLOW.md"), validWorkflow)

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %+v", len(result.Issues), result.Issues)
	}
}

func TestValidateWorkflowDocsChecks_WalksMultipleDocs(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, "001_PHASE", "WORKFLOW.md"), validWorkflow)
	writeFile(t, filepath.Join(fest, "002_PHASE", "01_seq", "WORKFLOW.md"), step0Workflow)
	writeFile(t, filepath.Join(fest, "003_PHASE", "WORKFLOW.md"), gapWorkflow)

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %+v", len(result.Issues), result.Issues)
	}
}

func TestValidateWorkflowDocsChecks_SkipsRuntimeDirs(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, ".fest", "WORKFLOW.md"), step0Workflow)
	writeFile(t, filepath.Join(fest, ".workflow", "WORKFLOW.md"), step0Workflow)
	writeFile(t, filepath.Join(fest, ".git", "WORKFLOW.md"), step0Workflow)
	writeFile(t, filepath.Join(fest, "001_PHASE", "WORKFLOW.md"), validWorkflow)

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 0 {
		t.Fatalf("expected 0 issues (runtime dirs should be skipped), got %d: %+v", len(result.Issues), result.Issues)
	}
}

func TestValidateWorkflowDocsChecks_NoWorkflowFiles(t *testing.T) {
	fest := t.TempDir()
	writeFile(t, filepath.Join(fest, "FESTIVAL_OVERVIEW.md"), "# Overview\n")

	result := &ValidationResult{Issues: []ValidationIssue{}}
	validateWorkflowDocsChecks(context.Background(), fest, result)

	if len(result.Issues) != 0 {
		t.Fatalf("expected 0 issues when no WORKFLOW.md exists, got %d", len(result.Issues))
	}
}
