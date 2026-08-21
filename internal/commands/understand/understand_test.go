package understand

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	understanddocs "github.com/Obedience-Corp/fest/embedded/docs/understand"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintStructure_ContainsCurrentLifecycleDirs(t *testing.T) {
	output := captureOutput(func() {
		printStructure("")
	})

	requiredDirs := []string{
		"planning/",
		"ready/",
		"active/",
		"ritual/",
		"dungeon/",
	}

	for _, dir := range requiredDirs {
		if !strings.Contains(output, dir) {
			t.Errorf("printStructure output missing lifecycle directory: %s", dir)
		}
	}
}

func TestPrintStructure_ContainsAllPhaseTypes(t *testing.T) {
	output := captureOutput(func() {
		printStructure("")
	})

	phaseTypes := []string{
		"planning",
		"research",
		"ingest",
		"implementation",
		"review",
		"non_coding_action",
	}

	for _, pt := range phaseTypes {
		if !strings.Contains(output, pt) {
			t.Errorf("printStructure output missing phase type: %s", pt)
		}
	}
}

func TestPrintChecklist_ContainsWorkflowPhaseChecks(t *testing.T) {
	output := captureOutput(func() {
		printChecklist()
	})

	requiredChecks := []string{
		"WORKFLOW.md",
		"Workflow Phases",
		"TASK FILES",
		"Quality Gates",
	}

	for _, check := range requiredChecks {
		if !strings.Contains(output, check) {
			t.Errorf("printChecklist output missing required check: %s", check)
		}
	}
}

var deprecatedTerms = []string{
	"commission",
	"artisan",
	"Workshop Board",
	"Guild Hall",
}

func TestUnderstandOutput_NoDeprecatedTerms(t *testing.T) {
	outputs := map[string]func(){
		"structure": func() { printStructure("") },
		"checklist": func() { printChecklist() },
		"context":   func() { printContext() },
		"nodeids":   func() { printNodeIDs() },
		"templates": func() { printTemplates() },
		"loop":      func() { printLoop() },
		"lifecycle": func() { printLifecycle() },
		"roles":     func() { printRoles() },
		"planning":  func() { printPlanning() },
		"chains":    func() { printChains() },
		"rituals":   func() { printRituals() },
	}

	for name, fn := range outputs {
		t.Run(name, func(t *testing.T) {
			output := captureOutput(fn)
			outputLower := strings.ToLower(output)
			for _, term := range deprecatedTerms {
				if strings.Contains(outputLower, strings.ToLower(term)) {
					t.Errorf("%s output contains deprecated term: %q", name, term)
				}
			}
		})
	}
}

func TestEmbeddedDocs_LoadSuccessfully(t *testing.T) {
	docNames := []string{
		"overview.txt",
		"methodology.txt",
		"workflow.txt",
		"rules.txt",
		"tasks.txt",
		"templates.txt",
		"context.txt",
		"nodeids.txt",
		"loop.txt",
		"lifecycle.txt",
		"roles.txt",
		"planning.txt",
		"chains.txt",
		"rituals.txt",
		"hooks.txt",
	}

	for _, name := range docNames {
		t.Run(name, func(t *testing.T) {
			content := understanddocs.Load(name)
			if content == "" {
				t.Errorf("embedded doc %s loaded empty", name)
			}
			if len(content) < 50 {
				t.Errorf("embedded doc %s suspiciously short (%d chars)", name, len(content))
			}
		})
	}
}

func TestConsistency_PhaseTypesMatchAcrossSubcommands(t *testing.T) {
	structureOutput := captureOutput(func() { printStructure("") })
	rulesOutput := captureOutput(func() { printRules("") })

	phaseTypes := []string{"implementation", "planning", "research", "ingest"}
	for _, pt := range phaseTypes {
		if strings.Contains(structureOutput, pt) && !strings.Contains(rulesOutput, pt) {
			t.Errorf("phase type %q in structure output but missing from rules output", pt)
		}
	}
}

func TestStructure_EmbeddedDocContainsLifecycleDirs(t *testing.T) {
	content := understanddocs.Load("structure.txt")

	requiredDirs := []string{"ready/", "ritual/", "dungeon/"}
	for _, dir := range requiredDirs {
		if !strings.Contains(content, dir) {
			t.Errorf("embedded structure.txt missing lifecycle directory: %s", dir)
		}
	}
}

func TestRules_EmbeddedDocContainsStatusDirectories(t *testing.T) {
	content := understanddocs.Load("rules.txt")

	if !strings.Contains(content, "STATUS DIRECTORIES") {
		t.Error("rules.txt missing STATUS DIRECTORIES section")
	}
	if !strings.Contains(content, "ready/") {
		t.Error("rules.txt missing ready/ in status directories")
	}
}

func TestHooks_EmbeddedDocListsEveryV1Verb(t *testing.T) {
	content := understanddocs.Load("hooks.txt")
	var verbsLine string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "v1 verbs:") {
			verbsLine = line
			break
		}
	}
	if verbsLine == "" {
		t.Fatal("hooks.txt missing v1 verbs line")
	}
	for _, verb := range hooks.V1Verbs() {
		if !strings.Contains(verbsLine, string(verb)) {
			t.Errorf("v1 verbs line omits %q: %s", verb, verbsLine)
		}
	}
}
