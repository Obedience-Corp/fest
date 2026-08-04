package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The gate templates gained a "## Before you submit any step below" section.
// StepHeaderRegexp matches "## Step", so it must not be picked up as a step:
// a phantom step would renumber every real gate and desync workflow state.
func TestShippedGateTemplatesParseToStepsOnly(t *testing.T) {
	root := "../../../methodology/festivals/.festival/templates/phases"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("templates not reachable: %v", err)
	}
	want := map[string]int{
		"implementation": 4, "research": 3, "planning": 3,
		"ingest": 3, "review": 3, "non_coding_action": 3,
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name(), "GATES.md"))
		if err != nil {
			continue
		}
		steps, err := NewParser().ParseContent(context.Background(), string(raw))
		if err != nil {
			t.Fatalf("%s: parse: %v", e.Name(), err)
		}
		if n, ok := want[e.Name()]; ok && len(steps) != n {
			t.Errorf("%s: parsed %d steps, want %d: %v", e.Name(), len(steps), n, stepNames(steps))
		}
		for _, s := range steps {
			if s.Name == "" || s.Name == "Before you submit any step below" {
				t.Errorf("%s: prose parsed as a step: %q", e.Name(), s.Name)
			}
		}
	}
}

func stepNames(steps []WorkflowStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Name)
	}
	return out
}
