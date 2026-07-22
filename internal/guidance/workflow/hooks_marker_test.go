package workflow

import (
	"context"
	"testing"
)

func TestParseContent_HooksAndApprovalMarkers(t *testing.T) {
	content := `## Step 1: REVIEW — Check

**Goal:** Review work

**Actions:**
1. Look

**Output:** Notes

**Checkpoint:** User approval required before continuing

**Hooks:** post: [approval_judge], pre: [lint]

**Approval:** human-required
`
	steps, err := NewParser().ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d", len(steps))
	}
	s := steps[0]
	if len(s.Hooks.Pre) != 1 || s.Hooks.Pre[0] != "lint" {
		t.Fatalf("pre = %+v", s.Hooks.Pre)
	}
	if len(s.Hooks.Post) != 1 || s.Hooks.Post[0] != "approval_judge" {
		t.Fatalf("post = %+v", s.Hooks.Post)
	}
	if s.Approval != "human-required" {
		t.Fatalf("approval = %q", s.Approval)
	}
}

func TestParseContent_AbsentHooksAndApproval(t *testing.T) {
	content := `## Step 1: READ

**Goal:** Read docs

**Actions:**
1. Open file

**Output:** Notes
`
	steps, err := NewParser().ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	s := steps[0]
	if len(s.Hooks.Pre) != 0 || len(s.Hooks.Post) != 0 || s.Approval != "" {
		t.Fatalf("want empty hooks/approval, got %+v %q", s.Hooks, s.Approval)
	}
}

func TestParseContent_MalformedHooksMarker(t *testing.T) {
	content := `## Step 1: X

**Goal:** g

**Actions:**
1. a

**Output:** o

**Hooks:** not-a-valid-list {{{
`
	steps, err := NewParser().ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	s := steps[0]
	if len(s.Hooks.Pre) != 0 || len(s.Hooks.Post) != 0 {
		t.Fatalf("malformed should yield empty, got %+v", s.Hooks)
	}
}
