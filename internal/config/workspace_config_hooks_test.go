package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkspaceConfig_ParsesApprovalJudgeDefinition(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := "version: \"1.0\"\nhooks:\n  definitions:\n    approval_judge:\n      command: ob judge\n"
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	def, ok := cfg.Hooks.Definitions["approval_judge"]
	if !ok {
		t.Fatal("missing approval_judge definition")
	}
	if def.Command != "ob judge" {
		t.Fatalf("approval_judge command = %q, want %q", def.Command, "ob judge")
	}
}

// The flat hooks.approval_judge key was removed before the judge hook shipped
// through a festival release. It must not silently configure a judge.
func TestLoadWorkspaceConfig_FlatApprovalJudgeKeyIsInert(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := "version: \"1.0\"\nhooks:\n  approval_judge:\n    command: ob judge\n"
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if len(cfg.Hooks.Definitions) != 0 {
		t.Fatalf("flat key must not produce definitions, got %+v", cfg.Hooks.Definitions)
	}
}

func TestSaveWorkspaceConfig_WritesCommentedHooksPlaceholder(t *testing.T) {
	root := t.TempDir()
	if err := SaveWorkspaceConfig(root, DefaultWorkspaceConfig()); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, DotFestivalDir, WorkspaceConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	s := string(data)
	for _, want := range []string{"# hooks:", "#   definitions:", "#     approval_judge:", "command: ob judge"} {
		if !strings.Contains(s, want) {
			t.Fatalf("placeholder missing %q, got:\n%s", want, s)
		}
	}

	// The commented block must not parse into active hooks.
	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if !loaded.Hooks.IsZero() || len(loaded.Hooks.Definitions) != 0 {
		t.Fatalf("commented placeholder should not parse into hooks, got %+v", loaded.Hooks)
	}
}

func TestWorkspaceConfig_ApprovalJudgeDefinitionRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultWorkspaceConfig()
	cfg.Hooks.Definitions = map[string]HookDefinition{
		"approval_judge": {Command: "my-judge --strict", Timeout: "0"},
	}
	if err := SaveWorkspaceConfig(root, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}

	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	def, ok := loaded.Hooks.Definitions["approval_judge"]
	if !ok {
		t.Fatal("missing approval_judge definition after round-trip")
	}
	if def.Command != "my-judge --strict" {
		t.Fatalf("round-trip command = %q, want %q", def.Command, "my-judge --strict")
	}
	if def.Timeout != "0" {
		t.Fatalf("round-trip timeout = %q, want %q", def.Timeout, "0")
	}
}
