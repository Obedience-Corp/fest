package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const workflowStepZero = `---
fest_type: workflow
---

# Sample Workflow

## Step 0: PRECONDITION - Confirm context

**Goal:** start.

## Step 1: SCOPE - Define questions

**Goal:** scope.

## Step 2: DISCOVER - Gather

**Goal:** discover.
`

const workflowGaps = `# Gaps

## Step 1: ALPHA - first

body

## Step 2: BETA - second

body

## Step 4: DELTA - fourth

body

## Step 5: EPSILON - fifth

body
`

const workflowAlready = `# Already

## Step 1: A - one

x

## Step 2: B - two

y

## Step 3: C - three

z
`

const workflowDuplicates = `# Dupes

## Step 1: A

x

## Step 2: B

y

## Step 2: C

z

## Step 3: D

w
`

func writeTempWorkflow(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestBuildRenumberPlan(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantCount   int
		wantChanges int
		wantFirst   int
		wantLast    int
	}{
		{
			name:        "step zero shifts to one",
			content:     workflowStepZero,
			wantCount:   3,
			wantChanges: 3,
			wantFirst:   1,
			wantLast:    3,
		},
		{
			name:        "gaps compact",
			content:     workflowGaps,
			wantCount:   4,
			wantChanges: 2,
			wantFirst:   1,
			wantLast:    4,
		},
		{
			name:        "already correct is no-op",
			content:     workflowAlready,
			wantCount:   3,
			wantChanges: 0,
			wantFirst:   1,
			wantLast:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := buildRenumberPlan(tt.content)
			if err != nil {
				t.Fatalf("buildRenumberPlan: %v", err)
			}
			if len(plan.headings) != tt.wantCount {
				t.Fatalf("headings = %d, want %d", len(plan.headings), tt.wantCount)
			}
			if plan.changes != tt.wantChanges {
				t.Fatalf("changes = %d, want %d", plan.changes, tt.wantChanges)
			}
			if plan.headings[0].newNumber != tt.wantFirst {
				t.Fatalf("first new = %d, want %d", plan.headings[0].newNumber, tt.wantFirst)
			}
			if plan.headings[len(plan.headings)-1].newNumber != tt.wantLast {
				t.Fatalf("last new = %d, want %d", plan.headings[len(plan.headings)-1].newNumber, tt.wantLast)
			}
		})
	}
}

func TestBuildRenumberPlanDuplicatesError(t *testing.T) {
	_, err := buildRenumberPlan(workflowDuplicates)
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if err.Code != errors.ErrCodeValidation {
		t.Fatalf("code = %q, want %q", err.Code, errors.ErrCodeValidation)
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error does not mention duplicates: %v", err)
	}
	key := "Step 2"
	if _, ok := err.Fields[key]; !ok {
		t.Fatalf("missing field for duplicate step 2; fields = %v", err.Fields)
	}
}

func TestApplyRenumberPreservesSuffix(t *testing.T) {
	plan, err := buildRenumberPlan(workflowStepZero)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	updated := applyRenumber(workflowStepZero, plan)

	wantHeaders := []string{
		"## Step 1: PRECONDITION - Confirm context",
		"## Step 2: SCOPE - Define questions",
		"## Step 3: DISCOVER - Gather",
	}
	for _, h := range wantHeaders {
		if !strings.Contains(updated, h) {
			t.Fatalf("missing header %q in:\n%s", h, updated)
		}
	}

	if strings.Contains(updated, "## Step 0:") {
		t.Fatalf("Step 0 still present after renumber:\n%s", updated)
	}

	if !strings.Contains(updated, "**Goal:** start.") {
		t.Fatal("body content was modified")
	}
}

func TestApplyRenumberGapsCompact(t *testing.T) {
	plan, err := buildRenumberPlan(workflowGaps)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	updated := applyRenumber(workflowGaps, plan)

	if strings.Contains(updated, "## Step 5:") {
		t.Fatalf("gaps not compacted (step 5 still present):\n%s", updated)
	}
	wantHeaders := []string{
		"## Step 1: ALPHA - first",
		"## Step 2: BETA - second",
		"## Step 3: DELTA - fourth",
		"## Step 4: EPSILON - fifth",
	}
	for _, h := range wantHeaders {
		if !strings.Contains(updated, h) {
			t.Fatalf("missing header %q in:\n%s", h, updated)
		}
	}
}

func TestRunWorkflowRenumberDryRunDoesNotWrite(t *testing.T) {
	path := writeTempWorkflow(t, workflowStepZero)
	original, _ := os.ReadFile(path)

	var out bytes.Buffer
	if err := runWorkflowRenumber(&out, path, true, false); err != nil {
		t.Fatalf("runWorkflowRenumber: %v", err)
	}

	current, _ := os.ReadFile(path)
	if string(current) != string(original) {
		t.Fatal("dry-run wrote changes to disk")
	}
	if !strings.Contains(out.String(), "Dry-run plan") {
		t.Fatalf("missing dry-run banner in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Step 0 -> Step 1") {
		t.Fatalf("missing rename line in output:\n%s", out.String())
	}
}

func TestRunWorkflowRenumberWrites(t *testing.T) {
	path := writeTempWorkflow(t, workflowStepZero)

	var out bytes.Buffer
	if err := runWorkflowRenumber(&out, path, false, false); err != nil {
		t.Fatalf("runWorkflowRenumber: %v", err)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if !strings.Contains(string(current), "## Step 1: PRECONDITION - Confirm context") {
		t.Fatalf("step 0 was not renumbered to 1:\n%s", string(current))
	}
	if strings.Contains(string(current), "## Step 0:") {
		t.Fatal("step 0 still present")
	}
}

func TestRunWorkflowRenumberBackup(t *testing.T) {
	path := writeTempWorkflow(t, workflowStepZero)

	var out bytes.Buffer
	if err := runWorkflowRenumber(&out, path, false, true); err != nil {
		t.Fatalf("runWorkflowRenumber: %v", err)
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected backup file at %s.bak: %v", path, err)
	}
	backup, _ := os.ReadFile(path + ".bak")
	if !strings.Contains(string(backup), "## Step 0:") {
		t.Fatal("backup does not contain original content")
	}
}

func TestRunWorkflowRenumberAlreadyCorrectIsNoop(t *testing.T) {
	path := writeTempWorkflow(t, workflowAlready)
	original, _ := os.ReadFile(path)

	var out bytes.Buffer
	if err := runWorkflowRenumber(&out, path, false, false); err != nil {
		t.Fatalf("runWorkflowRenumber: %v", err)
	}

	current, _ := os.ReadFile(path)
	if string(current) != string(original) {
		t.Fatal("no-op case wrote changes")
	}
	if !strings.Contains(out.String(), "nothing to renumber") {
		t.Fatalf("expected nothing-to-renumber message in output:\n%s", out.String())
	}
}

func TestRunWorkflowRenumberDuplicatesError(t *testing.T) {
	path := writeTempWorkflow(t, workflowDuplicates)
	original, _ := os.ReadFile(path)

	var out bytes.Buffer
	err := runWorkflowRenumber(&out, path, false, false)
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if !errors.Is(err, errors.ErrCodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	current, _ := os.ReadFile(path)
	if string(current) != string(original) {
		t.Fatal("duplicates error case still wrote changes")
	}
}

func TestRunWorkflowRenumberEmptyFileError(t *testing.T) {
	path := writeTempWorkflow(t, "# Just a heading\n\nNo steps here.\n")

	var out bytes.Buffer
	err := runWorkflowRenumber(&out, path, false, false)
	if err == nil {
		t.Fatal("expected error for file with no steps")
	}
	if !errors.Is(err, errors.ErrCodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRunWorkflowRenumberFileNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist", "WORKFLOW.md")

	var out bytes.Buffer
	err := runWorkflowRenumber(&out, missing, true, false)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, errors.ErrCodeNotFound) {
		t.Fatalf("expected NOT_FOUND error, got %v", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error should include path %q: %v", missing, err)
	}
}
