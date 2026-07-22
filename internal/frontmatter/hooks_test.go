package frontmatter

import (
	"strings"
	"testing"
)

func TestParse_HooksAndApproval(t *testing.T) {
	content := []byte(`---
fest_type: task
fest_id: t1
fest_status: pending
fest_created: 2026-07-19T00:00:00Z
hooks:
  pre: [lint-check]
  post: [approval_judge, notify]
approval: human-required
custom_meta: keep-me
---

# Body
`)
	fm, _, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(fm.Hooks.Pre) != 1 || fm.Hooks.Pre[0] != "lint-check" {
		t.Fatalf("pre = %+v", fm.Hooks.Pre)
	}
	if len(fm.Hooks.Post) != 2 || fm.Hooks.Post[0] != "approval_judge" || fm.Hooks.Post[1] != "notify" {
		t.Fatalf("post = %+v", fm.Hooks.Post)
	}
	if fm.Approval != "human-required" {
		t.Fatalf("approval = %q", fm.Approval)
	}
	if fm.Extra["custom_meta"] != "keep-me" {
		t.Fatalf("Extra should preserve custom_meta, got %+v", fm.Extra)
	}
	// hooks should not land in Extra
	if _, ok := fm.Extra["hooks"]; ok {
		t.Fatalf("hooks should not be in Extra: %+v", fm.Extra)
	}
}

func TestParse_AbsentHooksAndApproval(t *testing.T) {
	content := []byte(`---
fest_type: task
fest_id: t1
fest_status: pending
fest_created: 2026-07-19T00:00:00Z
---
`)
	fm, _, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(fm.Hooks.Pre) != 0 || len(fm.Hooks.Post) != 0 || fm.Approval != "" {
		t.Fatalf("want zero hooks/approval, got %+v %q", fm.Hooks, fm.Approval)
	}
	if b := fm.HookBindings(); len(b.Pre) != 0 || len(b.Post) != 0 {
		t.Fatalf("HookBindings = %+v", b)
	}
}

func TestParse_HooksNotInExtraRoundTripShape(t *testing.T) {
	content := []byte(`---
fest_type: task
fest_id: t1
fest_status: pending
fest_created: 2026-07-19T00:00:00Z
hooks:
  post: [x]
user_key: v
---
body
`)
	fm, body, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(string(body), "body") {
		t.Fatalf("body lost: %q", body)
	}
	if fm.Extra["user_key"] != "v" {
		t.Fatalf("user_key not preserved: %+v", fm.Extra)
	}
}
