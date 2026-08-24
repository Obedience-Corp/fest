package runloop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInvokeExecPassesPromptOnStdin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "worker")
	body := "#!/bin/sh\ncat > \"$PWD/prompt.txt\"\nexit 0\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InvokeExec(context.Background(), bin, "hello slice", dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello slice" {
		t.Fatalf("prompt = %q", got)
	}
}

func TestInvokeExecEmptyRejected(t *testing.T) {
	if err := InvokeExec(context.Background(), "", "x", t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}
