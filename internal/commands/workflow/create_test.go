package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func TestWorkflowCreateRegistered(t *testing.T) {
	create := findSubcommand(NewWorkflowCommand(), "create")
	if create == nil {
		t.Fatal("`fest workflow create` is not registered under the workflow command")
	}
	for _, f := range []string{"steps", "steps-file", "type", "json", "agent", "no-init", "path", "position", "festival"} {
		if create.Flags().Lookup(f) == nil {
			t.Errorf("create alias is missing flag --%s", f)
		}
	}
}

func TestWorkflowCreateHelpMentionsAlias(t *testing.T) {
	cmd := NewWorkflowCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"create", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	help := strings.ToLower(out.String())
	for _, want := range []string{"alias of 'fest create workflow'", "current directory", "fest next"} {
		if !strings.Contains(help, strings.ToLower(want)) {
			t.Fatalf("create help missing %q:\n%s", want, out.String())
		}
	}
}

func TestWorkflowCreateRejectsPhaseFlag(t *testing.T) {
	defer func() { phaseFlag = "" }()
	cmd := NewWorkflowCommand()
	cmd.SetArgs([]string{"create", "demo", "--phase", "001_X", "--steps", `{"title":"x","steps":[{"name":"A","goal":"g"}]}`})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --phase to be rejected")
	}
	if !strings.Contains(err.Error(), "--phase is not supported") {
		t.Errorf("error = %v, want it to mention --phase is not supported", err)
	}
}

func TestWorkflowCreateScaffoldsStandalone(t *testing.T) {
	defer func() { phaseFlag = "" }()
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewWorkflowCommand()
	cmd.SetArgs([]string{
		"create", "demo", "--json",
		"--steps", `{"title":"Demo","steps":[{"name":"SCAN","goal":"Scan items"},{"name":"DECIDE","goal":"Decide fate"}]}`,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "WORKFLOW.md")); err != nil {
		t.Errorf("WORKFLOW.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".workflow", "workflow.yaml")); err != nil {
		t.Errorf(".workflow/workflow.yaml not created: %v", err)
	}
}
