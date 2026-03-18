package gates

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateForSequence_SkipsExistingCustomGates(t *testing.T) {
	t.Parallel()

	sequencePath := t.TempDir()

	files := map[string]string{
		"01_build_feature.md": "# Build feature\n",
		"02_quality_gate_testing.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Testing and Verification
`,
		"03_quality_gate_review.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Code Review
`,
		"04_quality_gate_iterate.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Review Results and Iterate
`,
		"05_quality_gate_commit.md": `---
fest_type: gate
fest_status: pending
---
# Gate: Commit Sequence Changes
`,
	}

	for name, content := range files {
		path := filepath.Join(sequencePath, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	generator := &TaskGenerator{}
	gateDefs := []GateTask{
		{ID: "testing", Enabled: true},
		{ID: "review", Enabled: true},
		{ID: "iterate", Enabled: true},
		{ID: "fest-commit", Enabled: true},
	}

	results, warnings, err := generator.GenerateForSequence(
		context.Background(),
		sequencePath,
		gateDefs,
		GenerateOptions{DryRun: true},
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("GenerateForSequence() warnings = %v, want none", warnings)
	}
	if len(results) != len(gateDefs) {
		t.Fatalf("GenerateForSequence() returned %d results, want %d", len(results), len(gateDefs))
	}

	for _, result := range results {
		if result.Type != "skip" {
			t.Fatalf("GenerateForSequence() result type = %q for %s, want skip", result.Type, result.TaskID)
		}
		if result.Reason != "gate_exists" {
			t.Fatalf("GenerateForSequence() skip reason = %q for %s, want gate_exists", result.Reason, result.TaskID)
		}
	}
}

func TestGenerateForSequence_SkipsExistingFestCommitWithMismatchedFrontmatter(t *testing.T) {
	t.Parallel()

	sequencePath := t.TempDir()

	if err := os.WriteFile(filepath.Join(sequencePath, "01_build_feature.md"), []byte("# Build feature\n"), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	// Reproduce the stale frontmatter that previously caused duplicate commit gates.
	existingCommitGate := `---
fest_type: gate
fest_gate_type: iterate
fest_status: pending
---
# Gate: Commit Sequence Changes
`
	commitPath := filepath.Join(sequencePath, "02_fest_commit.md")
	if err := os.WriteFile(commitPath, []byte(existingCommitGate), 0644); err != nil {
		t.Fatalf("write commit gate: %v", err)
	}

	generator := &TaskGenerator{}
	results, warnings, err := generator.GenerateForSequence(
		context.Background(),
		sequencePath,
		[]GateTask{{ID: "fest-commit", Enabled: true}},
		GenerateOptions{DryRun: true},
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("GenerateForSequence() warnings = %v, want none", warnings)
	}
	if len(results) != 1 {
		t.Fatalf("GenerateForSequence() returned %d results, want 1", len(results))
	}
	if results[0].Type != "skip" || results[0].Reason != "gate_exists" {
		t.Fatalf("GenerateForSequence() = %+v, want skip/gate_exists", results[0])
	}
	if results[0].Path != commitPath {
		t.Fatalf("GenerateForSequence() existing path = %q, want %q", results[0].Path, commitPath)
	}
}
