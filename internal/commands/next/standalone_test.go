package next

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

func writeWFDoc(t *testing.T, dir string) string {
	t.Helper()
	doc := filepath.Join(dir, "WORKFLOW.md")
	body := `---
workflow_version: 1
workflow_id: wf-test
workitem_id: test-001
---

## Step 1: ALIGN

**Goal:** prove routing.

**Actions:**
1. Step one.

**Output:** done
`
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return doc
}

func captureStdoutErr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	err = fn()
	_ = w.Close()
	<-done
	return buf.String(), err
}

func TestRunStandaloneNext_Tracked(t *testing.T) {
	dir := t.TempDir()
	doc := writeWFDoc(t, dir)
	store := localstore.Open(filepath.Join(dir, ".workflow"), doc)
	if err := store.Init(context.Background(), localstore.InitOptions{
		WorkflowID: "wf-test", WorkitemID: "test-001",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(context.Background(), ""); err != nil {
		t.Fatal(err)
	}

	res := &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    dir,
		WorkflowDoc: doc,
		RuntimeDir:  filepath.Join(dir, ".workflow"),
	}

	out, err := captureStdoutErr(t, func() error {
		return runStandaloneNext(context.Background(), res, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "STANDALONE WORKFLOW") {
		t.Errorf("output missing header: %s", out)
	}
	if !strings.Contains(out, "ALIGN") {
		t.Errorf("output missing step name: %s", out)
	}
}

func TestRunStandaloneNext_TrackedJSON(t *testing.T) {
	dir := t.TempDir()
	doc := writeWFDoc(t, dir)
	store := localstore.Open(filepath.Join(dir, ".workflow"), doc)
	_ = store.Init(context.Background(), localstore.InitOptions{
		WorkflowID: "wf-test", WorkitemID: "test-001",
	})
	_, _ = store.StartRun(context.Background(), "")

	res := &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    dir,
		WorkflowDoc: doc,
		RuntimeDir:  filepath.Join(dir, ".workflow"),
	}

	// Set the JSON flag for this test.
	prev := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = prev }()

	out, err := captureStdoutErr(t, func() error {
		return runStandaloneNext(context.Background(), res, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"mode": "standalone-tracked"`) {
		t.Errorf("JSON mode field missing: %s", out)
	}
	if !strings.Contains(out, `"current_step": 1`) && !strings.Contains(out, `"current_step":1`) {
		t.Errorf("JSON current_step missing: %s", out)
	}
}

func TestRunAnonymousNext_NoFilesCreated(t *testing.T) {
	dir := t.TempDir()
	doc := writeWFDoc(t, dir)

	res := &standalone.Result{
		Mode:        standalone.ModeAnonymous,
		StartDir:    dir,
		WorkflowDoc: doc,
	}

	out, err := captureStdoutErr(t, func() error {
		return runAnonymousNext(context.Background(), res, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "anonymous") {
		t.Errorf("output should identify anonymous mode: %s", out)
	}

	// Critical: no files created.
	if _, err := os.Stat(filepath.Join(dir, ".workitem")); err == nil {
		t.Errorf(".workitem should not exist after read-only fest next")
	}
	if _, err := os.Stat(filepath.Join(dir, ".workflow")); err == nil {
		t.Errorf(".workflow/ should not exist after read-only fest next")
	}
}

func TestRunAnonymousNext_JSONMode(t *testing.T) {
	dir := t.TempDir()
	doc := writeWFDoc(t, dir)
	res := &standalone.Result{
		Mode:        standalone.ModeAnonymous,
		StartDir:    dir,
		WorkflowDoc: doc,
	}

	prev := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = prev }()

	out, err := captureStdoutErr(t, func() error {
		return runAnonymousNext(context.Background(), res, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"mode": "standalone-anonymous"`) {
		t.Errorf("JSON mode field missing: %s", out)
	}
	if !strings.Contains(out, `"run_status": "not-started"`) {
		t.Errorf("JSON run_status missing: %s", out)
	}
}
