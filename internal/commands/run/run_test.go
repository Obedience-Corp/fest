package run

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"context"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
)

func TestRunDryJSON(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "WORKFLOW.md")
	body := `---
workflow_version: 1
workflow_id: wf-run-cmd
---

## Step 1: ALIGN

**Goal:** prove routing.
`
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	store := localstore.Open(filepath.Join(dir, ".workflow"), doc)
	ctx := context.Background()
	if err := store.Init(ctx, localstore.InitOptions{WorkflowID: "wf-run-cmd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(ctx, "test"); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cmd := NewRunCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dry", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"outcome": "runnable"`) {
		t.Fatalf("out = %s", buf.String())
	}
}

func TestRunCommandIsCobra(t *testing.T) {
	cmd := NewRunCommand()
	if cmd.Use != "run" {
		t.Fatalf("use = %s", cmd.Use)
	}
	if _, ok := cmd.Annotations["scope"]; !ok {
		t.Fatal("missing scope annotation")
	}
	if cmd.Flags().Lookup("agent") != nil {
		t.Fatal("fest run must not have --agent; fest does not own an agent")
	}
	execFlag := cmd.Flags().Lookup("exec")
	if execFlag == nil {
		t.Fatal("missing --exec")
	}
	if execFlag.DefValue != "" {
		t.Fatalf("exec default = %q, want empty", execFlag.DefValue)
	}
}
