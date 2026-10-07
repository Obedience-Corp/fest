package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"gopkg.in/yaml.v3"
)

func TestRunAdvanceTracked_UpdatesPersistedSummary(t *testing.T) {
	res, store := setupTrackedStandaloneShowFixture(t)
	ctx := context.Background()

	m, err := store.LoadManifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runPath := filepath.Join(res.RuntimeDir, "runs", m.ActiveRunID, "run.yaml")
	before, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	var beforeRM localstore.RunManifest
	if err := yaml.Unmarshal(before, &beforeRM); err != nil {
		t.Fatal(err)
	}
	if beforeRM.Summary.CurrentStep != 0 || beforeRM.Summary.CompletedSteps != 0 {
		t.Fatalf("precondition: summary = %d/%d, want 0/0",
			beforeRM.Summary.CurrentStep, beforeRM.Summary.CompletedSteps)
	}

	var runErr error
	_ = captureStdout(t, func() {
		runErr = runAdvanceTracked(ctx, res)
	})
	if runErr != nil {
		t.Fatal(runErr)
	}

	after, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	var afterRM localstore.RunManifest
	if err := yaml.Unmarshal(after, &afterRM); err != nil {
		t.Fatal(err)
	}
	if afterRM.Summary.CurrentStep != 1 || afterRM.Summary.CompletedSteps != 1 {
		t.Fatalf("after advance summary = %d/%d, want 1/1",
			afterRM.Summary.CurrentStep, afterRM.Summary.CompletedSteps)
	}
}
