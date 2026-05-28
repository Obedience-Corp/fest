package standalone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func writeWorkflow(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}
	return path
}

func TestValidateStepNumbering_Errors(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantMessage string
		wantFound   string
		wantHint    string
	}{
		{
			name: "starts at step 0",
			content: `# Workflow
## Step 0: BOOTSTRAP — Setup
**Goal:** prep
## Step 1: REVIEW — Look
**Goal:** review
## Step 2: ACT — Do
**Goal:** act
`,
			wantMessage: "step numbering must start at 1",
			wantFound:   "Step 0, Step 1, Step 2",
			wantHint:    "merge Step 0",
		},
		{
			name: "gap in numbering",
			content: `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
## Step 4: FOUR — d
**Goal:** g
`,
			wantMessage: "not contiguous",
			wantFound:   "Step 1, Step 2, Step 4",
			wantHint:    "renumber",
		},
		{
			name: "duplicate step number",
			content: `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
## Step 2: TWO_AGAIN — c
**Goal:** g
`,
			wantMessage: "duplicate step number 2",
			wantFound:   "Step 1, Step 2, Step 2",
			wantHint:    "renumber",
		},
		{
			name: "starts at step 2",
			content: `# Workflow
## Step 2: TWO — b
**Goal:** g
## Step 3: THREE — c
**Goal:** g
`,
			wantMessage: "must start at 1",
			wantFound:   "Step 2, Step 3",
			wantHint:    "renumber",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, tc.content)
			err := ValidateStepNumbering(context.Background(), path)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			ferr, ok := err.(*festerrors.Error)
			if !ok {
				t.Fatalf("expected *festerrors.Error, got %T: %v", err, err)
			}
			if ferr.Code != festerrors.ErrCodeValidation {
				t.Errorf("expected code %s, got %s", festerrors.ErrCodeValidation, ferr.Code)
			}
			if !strings.Contains(ferr.Message, tc.wantMessage) {
				t.Errorf("message %q does not contain %q", ferr.Message, tc.wantMessage)
			}
			found, _ := ferr.Fields["found"].(string)
			if found != tc.wantFound {
				t.Errorf("found field = %q, want %q", found, tc.wantFound)
			}
			if !strings.Contains(ferr.Hint, tc.wantHint) {
				t.Errorf("hint %q does not contain %q", ferr.Hint, tc.wantHint)
			}
		})
	}
}

func TestValidateStepNumbering_HappyPaths(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "single step",
			content: `# Workflow
## Step 1: ONLY — a
**Goal:** g
`,
		},
		{
			name: "contiguous 1..5",
			content: `# Workflow
## Step 1: ONE — a
**Goal:** g
## Step 2: TWO — b
**Goal:** g
## Step 3: THREE — c
**Goal:** g
## Step 4: FOUR — d
**Goal:** g
## Step 5: FIVE — e
**Goal:** g
`,
		},
		{
			name: "no steps at all",
			content: `# Workflow

Some prose without any step headings.
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, tc.content)
			if err := ValidateStepNumbering(context.Background(), path); err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestValidateStepNumbering_FileMissing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "WORKFLOW.md")
	err := ValidateStepNumbering(context.Background(), missing)
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
	ferr, ok := err.(*festerrors.Error)
	if !ok {
		t.Fatalf("expected *festerrors.Error, got %T: %v", err, err)
	}
	if got, _ := ferr.Fields["path"].(string); got != missing {
		t.Errorf("expected path field %q, got %q", missing, got)
	}
}
