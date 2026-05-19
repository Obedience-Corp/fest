package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateForSequence_SkipsExistingCustomGates(t *testing.T) {
	t.Parallel()

	sequencePath := t.TempDir()

	files := map[string]string{
		"01_build_feature.md": "# Build feature\n",
		"02_quality_gate_testing.md": `---
fest_type: gate
fest_gate_id: testing
fest_status: pending
---
# Gate: Testing and Verification
`,
		"03_quality_gate_review.md": `---
fest_type: gate
fest_gate_id: review
fest_status: pending
---
# Gate: Code Review
`,
		"04_quality_gate_iterate.md": `---
fest_type: gate
fest_gate_id: iterate
fest_status: pending
---
# Gate: Review Results and Iterate
`,
		"05_quality_gate_commit.md": `---
fest_type: gate
fest_gate_id: fest-commit
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

func TestGenerateForSequence_ResolvesTemplateWithMdSuffix(t *testing.T) {
	t.Parallel()

	festivalPath := t.TempDir()
	sequencePath := filepath.Join(festivalPath, "001_IMPLEMENTATION", "01_core")
	templateDir := filepath.Join(festivalPath, "gates", "problem-mining")
	if err := os.MkdirAll(sequencePath, 0755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}

	templatePath := filepath.Join(templateDir, "QUALITY_GATE_ARTIFACT_CHECK.md")
	templateContent := `---
fest_type: gate
fest_status: pending
---
# Gate: Artifact Check
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	generator, err := NewTaskGenerator(context.Background(), festivalPath)
	if err != nil {
		t.Fatalf("NewTaskGenerator() error = %v", err)
	}

	results, warnings, err := generator.GenerateForSequence(
		context.Background(),
		sequencePath,
		[]GateTask{{
			ID:       "artifact-check",
			Name:     "Artifact Check",
			Template: "gates/problem-mining/QUALITY_GATE_ARTIFACT_CHECK.md",
			Enabled:  true,
		}},
		GenerateOptions{DryRun: false},
		festivalPath,
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("GenerateForSequence() warnings = %v, want none", warnings)
	}
	if len(results) != 1 || results[0].Type != "create" {
		t.Fatalf("GenerateForSequence() results = %+v, want 1 create", results)
	}

	if _, err := os.Stat(filepath.Join(sequencePath, "01_artifact_check.md")); err != nil {
		t.Fatalf("expected generated gate file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sequencePath, "01_artifact_check.md.md")); err == nil {
		t.Fatalf("double .md suffix file should not be created")
	}
}

func TestExtractPhaseAndGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		template string
		phase    string
		gate     string
	}{
		{"gates/implementation/QUALITY_GATE_TESTING", "implementation", "QUALITY_GATE_TESTING"},
		{"gates/implementation/QUALITY_GATE_TESTING.md", "implementation", "QUALITY_GATE_TESTING"},
		{"gates/problem-mining/QUALITY_GATE_ARTIFACT_CHECK.md", "problem-mining", "QUALITY_GATE_ARTIFACT_CHECK"},
		{"agent/gates/implementation/testing", "implementation", "testing"},
		{"agent/gates/implementation/testing.md", "implementation", "testing"},
		{"implementation/testing", "implementation", "testing"},
		{"", "", ""},
		{"gates/", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.template, func(t *testing.T) {
			t.Parallel()
			phase, gate := ExtractPhaseAndGate(tc.template)
			if phase != tc.phase || gate != tc.gate {
				t.Fatalf("ExtractPhaseAndGate(%q) = (%q, %q), want (%q, %q)",
					tc.template, phase, gate, tc.phase, tc.gate)
			}
		})
	}
}

func TestGenerateForSequence_SkipResultPreservesTemplate(t *testing.T) {
	t.Parallel()

	sequencePath := t.TempDir()

	existing := `---
fest_type: gate
fest_gate_id: testing
fest_status: pending
---
# Gate: Testing
`
	if err := os.WriteFile(filepath.Join(sequencePath, "01_quality_gate_testing.md"), []byte(existing), 0644); err != nil {
		t.Fatalf("write existing gate: %v", err)
	}

	generator := &TaskGenerator{}
	gateTemplate := "gates/problem-mining/QUALITY_GATE_ARTIFACT_CHECK"
	results, _, err := generator.GenerateForSequence(
		context.Background(),
		sequencePath,
		[]GateTask{{ID: "testing", Template: gateTemplate, Enabled: true}},
		GenerateOptions{DryRun: true},
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("GenerateForSequence() returned %d results, want 1", len(results))
	}
	r := results[0]
	if r.Type != "skip" || r.Reason != "gate_exists" {
		t.Fatalf("GenerateForSequence() = %+v, want skip/gate_exists", r)
	}
	if r.Template != gateTemplate {
		t.Fatalf("skip result Template = %q, want %q", r.Template, gateTemplate)
	}
}

func TestGenerateForSequence_BackfillsLegacyGateIDFromFilename(t *testing.T) {
	t.Parallel()

	sequencePath := t.TempDir()

	if err := os.WriteFile(filepath.Join(sequencePath, "01_build_feature.md"), []byte("# Build feature\n"), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	// Reproduce an older generated gate file that predates fest_gate_id stamping.
	existingCommitGate := `---
fest_type: gate
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
		GenerateOptions{DryRun: false},
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("GenerateForSequence() warnings = %v, want 1 restamp warning", warnings)
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

	content, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("read restamped gate: %v", err)
	}
	if !strings.Contains(string(content), "fest_gate_id: fest-commit") {
		t.Fatalf("GenerateForSequence() did not backfill fest_gate_id in %s", commitPath)
	}
	if !strings.Contains(string(content), "fest_managed: true") {
		t.Fatalf("GenerateForSequence() did not stamp fest_managed in %s", commitPath)
	}
}

func TestGenerateForSequence_StampsGateIDIntoTemplateFrontmatter(t *testing.T) {
	t.Parallel()

	festivalPath := t.TempDir()
	sequencePath := filepath.Join(festivalPath, "001_IMPLEMENTATION", "01_core")
	templateDir := filepath.Join(festivalPath, "gates", "implementation")
	if err := os.MkdirAll(sequencePath, 0755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}

	templatePath := filepath.Join(templateDir, "QUALITY_GATE_REVIEW.md")
	templateContent := `---
fest_type: gate
fest_status: pending
custom_field: keep-me
---
# Gate: Review
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	generator, err := NewTaskGenerator(context.Background(), festivalPath)
	if err != nil {
		t.Fatalf("NewTaskGenerator() error = %v", err)
	}

	results, warnings, err := generator.GenerateForSequence(
		context.Background(),
		sequencePath,
		[]GateTask{{
			ID:       "review",
			Name:     "Review",
			Template: "gates/implementation/QUALITY_GATE_REVIEW",
			Enabled:  true,
		}},
		GenerateOptions{DryRun: false},
		festivalPath,
	)
	if err != nil {
		t.Fatalf("GenerateForSequence() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("GenerateForSequence() warnings = %v, want none", warnings)
	}
	if len(results) != 1 || results[0].Type != "create" {
		t.Fatalf("GenerateForSequence() results = %+v, want 1 create", results)
	}

	generatedPath := filepath.Join(sequencePath, "01_review.md")
	content, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated gate: %v", err)
	}
	if !strings.Contains(string(content), "fest_gate_id: review") {
		t.Fatalf("generated gate missing fest_gate_id: %s", generatedPath)
	}
	if !strings.Contains(string(content), "fest_managed: true") {
		t.Fatalf("generated gate missing fest_managed: %s", generatedPath)
	}
}
