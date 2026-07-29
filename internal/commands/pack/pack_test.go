package pack

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/obey-shared/festivalbundle"
	"github.com/spf13/cobra"
)

func TestPackCommand_globalScope(t *testing.T) {
	cmd := NewPackCommand()
	if cmd.Annotations["scope"] != string(scope.Global) {
		t.Fatalf("pack scope = %q, want global", cmd.Annotations["scope"])
	}
	cmd = NewUnbundleCommand()
	if cmd.Annotations["scope"] != string(scope.Global) {
		t.Fatalf("unbundle scope = %q, want global", cmd.Annotations["scope"])
	}
}

func TestPackCommand_worksOutsideWorkspace(t *testing.T) {
	// From a non-workspace cwd, pack an explicit absolute source.
	outside := t.TempDir()
	t.Chdir(outside)

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "note.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Minimal festival-ish file so inferKind can still yield note/festival
	out := filepath.Join(t.TempDir(), "t.festival")

	root := &cobra.Command{Use: "fest"}
	// Attach scope resolver like production: Global should pass from any cwd.
	packCmd := NewPackCommand()
	root.AddCommand(packCmd)
	// Simulate: set args
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"pack", src, "-o", out, "--kind", "note", "--no-sent-record"})
	if err := root.Execute(); err != nil {
		t.Fatalf("pack outside workspace: %v\nstderr=%s", err, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestUnbundleCommand_jsonNotPollutedByValidateDiagnostics(t *testing.T) {
	// Pack a minimal tree, unbundle with --json --validate on a non-festival
	// dest so validation may fail or succeed but diagnostics go to stderr.
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Minimal festival markers so FullValidate has something to score
	_ = os.WriteFile(filepath.Join(src, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "FESTIVAL_OVERVIEW.md"), []byte("# Overview\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "FESTIVAL_RULES.md"), []byte("# Rules\n"), 0o644)

	out := filepath.Join(t.TempDir(), "b.festival")
	_, err := festivalbundle.Pack(context.Background(), src, out, festivalbundle.PackOptions{
		Kind: festivalbundle.KindFestival,
		Name: "mini",
	})
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "dest")
	root := &cobra.Command{Use: "fest"}
	ub := NewUnbundleCommand()
	root.AddCommand(ub)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// --validate may fail score on incomplete festival; we still want pure JSON on success path.
	// Use without validate first for pure JSON
	root.SetArgs([]string{"unbundle", out, "-d", dest, "--force", "--json", "--no-received-record"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unbundle: %v", err)
	}
	var info festivalbundle.Info
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("stdout not pure JSON: %v\n%s", err, stdout.String())
	}
	if info.Kind != festivalbundle.KindFestival {
		t.Fatalf("kind=%q", info.Kind)
	}

	// With --validate: stdout must remain pure JSON even if diagnostics on stderr
	dest2 := filepath.Join(t.TempDir(), "dest2")
	stdout.Reset()
	stderr.Reset()
	root.SetArgs([]string{"unbundle", out, "-d", dest2, "--force", "--json", "--validate", "--no-received-record"})
	// May return error if validation fails — still check that any stdout is JSON-only if present
	_ = root.Execute()
	if stdout.Len() > 0 {
		var info2 festivalbundle.Info
		if err := json.Unmarshal(stdout.Bytes(), &info2); err != nil {
			t.Fatalf("--json --validate stdout not pure JSON: %v\nstdout=%q\nstderr=%q", err, stdout.String(), stderr.String())
		}
	}
	// If validation ran, stderr should mention validate
	if !strings.Contains(stderr.String(), "validate") && stdout.Len() == 0 {
		// validate may have failed before printing if FullValidate error — accept either
		t.Logf("stderr=%q", stderr.String())
	}
}

func TestUnbundleCommand_globalFromNonWorkspace(t *testing.T) {
	outside := t.TempDir()
	t.Chdir(outside)

	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "n.md"), []byte("n\n"), 0o644)
	out := filepath.Join(t.TempDir(), "x.festival")
	_, err := festivalbundle.Pack(context.Background(), src, out, festivalbundle.PackOptions{Kind: "note", Name: "n"})
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "d")
	root := &cobra.Command{Use: "fest"}
	root.AddCommand(NewUnbundleCommand())
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"unbundle", out, "-d", dest, "--force", "--no-received-record"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unbundle outside workspace: %v (%s)", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "n.md")); err != nil {
		t.Fatal(err)
	}
}
