package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkspaceConfig_ParsesApprovalJudgeHook(t *testing.T) {
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
	if cfg.Hooks.ApprovalJudge.Command != "ob judge" {
		t.Fatalf("approval_judge command = %q, want %q", cfg.Hooks.ApprovalJudge.Command, "ob judge")
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
	if !strings.Contains(s, "# hooks:") || !strings.Contains(s, "command: ob judge") {
		t.Fatalf("expected commented hooks placeholder, got:\n%s", s)
	}

	// The commented block must not parse into active hooks.
	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if (loaded.Hooks != HooksConfig{}) {
		t.Fatalf("commented placeholder should not parse into hooks, got %+v", loaded.Hooks)
	}
}

func TestWorkspaceConfig_ApprovalJudgeHookRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultWorkspaceConfig()
	cfg.Hooks.ApprovalJudge.Command = "my-judge --strict"
	if err := SaveWorkspaceConfig(root, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}

	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if loaded.Hooks.ApprovalJudge.Command != "my-judge --strict" {
		t.Fatalf("round-trip command = %q, want %q", loaded.Hooks.ApprovalJudge.Command, "my-judge --strict")
	}
}
