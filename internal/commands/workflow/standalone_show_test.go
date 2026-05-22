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

func setupTrackedStandaloneShowFixture(t *testing.T) (*standalone.Result, *localstore.Store) {
	t.Helper()
	dir := t.TempDir()
	doc := filepath.Join(dir, "WORKFLOW.md")
	body := `---
workflow_version: 1
workflow_id: wf-test
---

## Step 1: PLAN

**Goal:** Plan it.

## Step 2: DO

**Goal:** Do it.
`
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(dir, ".workflow")
	store := localstore.Open(runtimeDir, doc)
	if err := store.Init(context.Background(), localstore.InitOptions{WorkflowID: "wf-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	return &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    dir,
		WorkflowDoc: doc,
		RuntimeDir:  runtimeDir,
	}, store
}

func TestRunStandaloneShowTracksNextUnstartedStep(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	if err := store.AppendEvent(context.Background(), localstore.Event{EventType: localstore.EventStepStart}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(context.Background(), localstore.Event{EventType: localstore.EventStepDone}); err != nil {
		t.Fatal(err)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = runStandaloneShow(context.Background(), res, 0)
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(out, "Step 2 of 2: DO") {
		t.Fatalf("show should render the next unstarted step after advance:\n%s", out)
	}
	if strings.Contains(out, "Step 1 of 2: PLAN") {
		t.Fatalf("show rendered stale completed step:\n%s", out)
	}
}

func TestStandaloneShowStep(t *testing.T) {
	cases := []struct {
		name  string
		state *localstore.RunState
		total int
		want  int
	}{
		{"nil state", nil, 2, 0},
		{"new run", &localstore.RunState{Status: "active"}, 2, 1},
		{"after completed step", &localstore.RunState{Status: "active", CurrentStep: 1, CompletedSteps: 1}, 2, 2},
		{"in progress", &localstore.RunState{Status: "active", CurrentStep: 2, CompletedSteps: 1}, 2, 2},
		{"complete", &localstore.RunState{Status: "completed", CurrentStep: 2, CompletedSteps: 2}, 2, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := standaloneShowStep(c.state, c.total); got != c.want {
				t.Fatalf("standaloneShowStep() = %d, want %d", got, c.want)
			}
		})
	}
}
