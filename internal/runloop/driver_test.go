package runloop

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeAgent(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-agent")
	body := "#!/bin/sh\necho ran >> \"$PWD/agent.log\"\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func initGit(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "fest@test"},
		{"git", "config", "user.name", "fest"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s (%v)", args, out, err)
		}
	}
}

func TestDriveDry(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** prove routing.
`)
	var buf bytes.Buffer
	err := Drive(context.Background(), dir, Options{Dry: true, Stdout: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "outcome: runnable") {
		t.Fatalf("output = %s", out)
	}
	snap, err := Inspect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	path, err := LedgerPath(snap)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := ReadLedger(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Outcome != OutcomeRunnable {
		t.Fatalf("ledger = %+v", recs)
	}
}

func TestDriveStatusOnlyDoesNotAppend(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** prove routing.
`)
	var buf bytes.Buffer
	if err := Drive(context.Background(), dir, Options{StatusOnly: true, Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	snap, _ := Inspect(context.Background(), dir)
	path, _ := LedgerPath(snap)
	recs, _ := ReadLedger(context.Background(), path)
	if len(recs) != 0 {
		t.Fatalf("status should not append, got %v", recs)
	}
}

func TestDriveWithoutExecDoesNotAdvance(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** first.

## Step 2: DO

**Goal:** second.
`)
	var buf bytes.Buffer
	if err := Drive(context.Background(), dir, Options{Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "outcome: runnable") {
		t.Fatalf("output = %s", buf.String())
	}
	snap, err := Inspect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Label != "ALIGN" {
		t.Fatalf("default run must not advance, label = %s", snap.Label)
	}
}

func TestDriveExecCompletes(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** first.

## Step 2: DO

**Goal:** second.
`)
	initGit(t, dir)
	agent := writeFakeAgent(t, dir)
	var buf bytes.Buffer
	err := Drive(context.Background(), dir, Options{
		Exec:       agent,
		MaxTasks:   8,
		MaxMinutes: 5,
		Stdout:     &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "outcome: completed") {
		t.Fatalf("output = %s", buf.String())
	}
	logPath := filepath.Join(dir, "agent.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "ran") != 2 {
		t.Fatalf("agent log = %s", data)
	}
}

func TestDriveDoesNotHardResetOnFailure(t *testing.T) {
	dir := t.TempDir()
	writeTrackedWorkflow(t, dir, `---
workflow_version: 1
workflow_id: wf-runloop
---

## Step 1: ALIGN

**Goal:** first.
`)
	initGit(t, dir)
	marker := filepath.Join(dir, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("stay"), 0o644); err != nil {
		t.Fatal(err)
	}
	fail := filepath.Join(dir, "fail-agent")
	if err := os.WriteFile(fail, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_ = Drive(context.Background(), dir, Options{
		Exec:       fail,
		MaxTasks:   8,
		MaxMinutes: 5,
		Stdout:     &buf,
	})
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("unlanded file was removed: %v", err)
	}
	if !strings.Contains(buf.String(), "outcome: failed") {
		t.Fatalf("output = %s", buf.String())
	}
}
