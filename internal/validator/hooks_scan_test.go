package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

func TestScanUndeclaredBindings_NilEffective(t *testing.T) {
	if got := ScanUndeclaredBindings(context.Background(), t.TempDir(), nil); got != nil {
		t.Fatalf("nil effective must yield nil, got %+v", got)
	}
}

func TestScanUndeclaredBindings_WarnsAcrossDocTypes(t *testing.T) {
	festivalPath := t.TempDir()
	phaseDir := filepath.Join(festivalPath, "001_PHASE")
	seqDir := filepath.Join(phaseDir, "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatal(err)
	}

	gates := `---
fest_type: phase_gate
---

## Step 1: GATE

**Question:** ok?

**Checkpoint:** APPROVAL REQUIRED

**Hooks:** post: [ghost-gate]
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatal(err)
	}
	task := `---
fest_type: task
fest_id: 01_task
hooks:
  pre: [ghost-task, declared]
---
# Task
`
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}

	eff, err := hooks.Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{"declared": {Command: "true"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	issues := ScanUndeclaredBindings(context.Background(), festivalPath, eff)
	if len(issues) != 2 {
		t.Fatalf("issues = %+v", issues)
	}
	names := map[string]bool{}
	for _, issue := range issues {
		if issue.Code != CodeHooksUndeclaredBinding || issue.Level != LevelWarning {
			t.Fatalf("issue shape wrong: %+v", issue)
		}
		names[issue.Message] = true
	}
	joined := ""
	for m := range names {
		joined += m + "\n"
	}
	for _, want := range []string{"ghost-gate", "ghost-task"} {
		found := false
		for m := range names {
			if len(m) > 0 && (m == "hook binding references undeclared name "+want+" (skipped)") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing warning for %s in:\n%s", want, joined)
		}
	}
}

func TestScanUndeclaredBindings_StartStage(t *testing.T) {
	festivalPath := t.TempDir()
	seqDir := filepath.Join(festivalPath, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatal(err)
	}

	task := `---
fest_type: task
fest_id: 01_task
hooks:
  start:
    pre: [ghost-start, declared]
---
# Task
`
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"), []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}
	seqGoal := `---
fest_type: sequence_goal
fest_id: 01_seq
hooks:
  start:
    pre: [declared]
---
# Goal
`
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte(seqGoal), 0o644); err != nil {
		t.Fatal(err)
	}

	eff, err := hooks.Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{"declared": {Command: "true"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	issues := ScanUndeclaredBindings(context.Background(), festivalPath, eff)
	if len(issues) != 2 {
		t.Fatalf("issues = %+v", issues)
	}
	var sawUndeclared, sawPlacement bool
	for _, issue := range issues {
		switch issue.Code {
		case CodeHooksUndeclaredBinding:
			if !strings.Contains(issue.Message, "ghost-start") {
				t.Fatalf("undeclared warning names wrong hook: %+v", issue)
			}
			sawUndeclared = true
		case CodeHooksStartBindingPlacement:
			if filepath.Base(issue.Path) != "SEQUENCE_GOAL.md" {
				t.Fatalf("placement warning on wrong doc: %+v", issue)
			}
			sawPlacement = true
		default:
			t.Fatalf("unexpected issue: %+v", issue)
		}
	}
	if !sawUndeclared || !sawPlacement {
		t.Fatalf("missing expected warnings: %+v", issues)
	}
}
