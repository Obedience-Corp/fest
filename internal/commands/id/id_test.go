package id

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// setupFestivalDir creates a temp festival directory with an optional fest.yaml.
func setupFestivalDir(t *testing.T, dirName, yamlContent string) string {
	t.Helper()
	base := t.TempDir()
	festDir := filepath.Join(base, dirName)
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if yamlContent != "" {
		if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(yamlContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return festDir
}

// captureRunID executes runID directly with the given context, capturing stdout.
func captureRunID(t *testing.T, ctx context.Context) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := runID(cmd, nil)
	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	out.ReadFrom(r)
	return out.String(), err
}

func TestID_FromMetadata(t *testing.T) {
	yaml := `version: "1.0"
metadata:
  id: SR0001
  name: submodule-ref-policy
`
	festDir := setupFestivalDir(t, "submodule-ref-policy-SR0001", yaml)

	ctx := scope.WithFestival(context.Background(), festDir)
	jsonOutput = false

	got, err := captureRunID(t, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "SR0001\n" {
		t.Errorf("expected %q, got %q", "SR0001\n", got)
	}
}

func TestID_FallbackToDirName(t *testing.T) {
	festDir := setupFestivalDir(t, "my-festival-MF0042", "")

	ctx := scope.WithFestival(context.Background(), festDir)
	jsonOutput = false

	got, err := captureRunID(t, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "MF0042\n" {
		t.Errorf("expected %q, got %q", "MF0042\n", got)
	}
}

func TestID_NoFestivalContext(t *testing.T) {
	jsonOutput = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runID(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no festival context, got nil")
	}
}

func TestID_JSONOutput(t *testing.T) {
	yaml := `version: "1.0"
metadata:
  id: GU0003
  name: guild-setup
`
	festDir := setupFestivalDir(t, "guild-setup-GU0003", yaml)

	ctx := scope.WithFestival(context.Background(), festDir)
	jsonOutput = true

	got, err := captureRunID(t, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}

	if result["id"] != "GU0003" {
		t.Errorf("expected id %q, got %q", "GU0003", result["id"])
	}
	if result["name"] != "guild-setup" {
		t.Errorf("expected name %q, got %q", "guild-setup", result["name"])
	}
	if result["path"] != festDir {
		t.Errorf("expected path %q, got %q", festDir, result["path"])
	}
}

func TestID_JSONOutput_NoName(t *testing.T) {
	festDir := setupFestivalDir(t, "something-AB1234", "")

	ctx := scope.WithFestival(context.Background(), festDir)
	jsonOutput = true

	got, err := captureRunID(t, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}

	if result["id"] != "AB1234" {
		t.Errorf("expected id %q, got %q", "AB1234", result["id"])
	}
	if _, hasName := result["name"]; hasName {
		t.Errorf("expected no name key in JSON, but got %q", result["name"])
	}
}

func TestID_NoIDAnywhere(t *testing.T) {
	festDir := setupFestivalDir(t, "no-id-here", "version: '1.0'\n")

	ctx := scope.WithFestival(context.Background(), festDir)
	jsonOutput = false

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := runID(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no ID is available, got nil")
	}
}

func TestID_CommandAnnotations(t *testing.T) {
	cmd := NewIDCommand()
	if cmd.Annotations["scope"] != string(scope.Festival) {
		t.Errorf("expected scope annotation %q, got %q", scope.Festival, cmd.Annotations["scope"])
	}
}

func TestID_HasJSONFlag(t *testing.T) {
	cmd := NewIDCommand()
	f := cmd.Flags().Lookup("json")
	if f == nil {
		t.Fatal("expected --json flag to exist")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --json default %q, got %q", "false", f.DefValue)
	}
}

func TestNewIDCommand_ReturnsCobraCommand(t *testing.T) {
	cmd := NewIDCommand()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Use != "id" {
		t.Errorf("expected Use %q, got %q", "id", cmd.Use)
	}
	if _, ok := cmd.Annotations["scope"]; !ok {
		t.Error("expected scope annotation")
	}
	var _ *cobra.Command = cmd
}
