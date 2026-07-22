package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

func TestLoadWorkspaceConfig_MalformedHooksReturnsParseError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := "version: \"1.0\"\nhooks: [not, a, map]\n"
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadWorkspaceConfig(root)
	if err == nil {
		t.Fatal("expected parse error for malformed hooks")
	}
	if !strings.Contains(err.Error(), "parsing workspace config") {
		t.Fatalf("error = %v, want config.Parse-wrapped message", err)
	}
}

func TestLoadWorkspaceConfig_ParsesFullHooksBlock(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := `version: "1.0"
hooks:
  enabled: true
  levels:
    phase: true
    sequence: true
    task: false
  definitions:
    approval_judge:
      command: ob judge
      fail: closed
      timeout: 120s
      evidence: paths
      enabled: false
    linter:
      command: just lint
`
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if cfg.Hooks.IsZero() {
		t.Fatal("hooks section present but IsZero()")
	}
	if cfg.Hooks.Enabled == nil || !*cfg.Hooks.Enabled {
		t.Fatalf("enabled = %v, want true", cfg.Hooks.Enabled)
	}
	if cfg.Hooks.Levels["task"] != false {
		t.Fatalf("levels.task = %v, want false", cfg.Hooks.Levels["task"])
	}
	def, ok := cfg.Hooks.Definitions["approval_judge"]
	if !ok {
		t.Fatal("missing approval_judge definition")
	}
	if def.Command != "ob judge" || def.Fail != "closed" || def.Timeout != "120s" || def.Evidence != "paths" {
		t.Fatalf("approval_judge fields = %+v", def)
	}
	if def.Enabled == nil || *def.Enabled {
		t.Fatalf("approval_judge.enabled = %v, want false", def.Enabled)
	}
	if cfg.Hooks.Definitions["linter"].Command != "just lint" {
		t.Fatalf("linter.command = %q", cfg.Hooks.Definitions["linter"].Command)
	}
}

func TestSaveWorkspaceConfig_EnabledFalseRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultWorkspaceConfig()
	cfg.Hooks = HooksConfig{
		Enabled: boolPtr(false),
		present: true,
	}
	if err := SaveWorkspaceConfig(root, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}

	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if loaded.Hooks.Enabled == nil || *loaded.Hooks.Enabled {
		t.Fatalf("enabled after round-trip = %v, want non-nil false", loaded.Hooks.Enabled)
	}
}

func TestSaveWorkspaceConfig_AbsentHooksIsZeroAndNoEmptySection(t *testing.T) {
	root := t.TempDir()
	if err := SaveWorkspaceConfig(root, DefaultWorkspaceConfig()); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, DotFestivalDir, WorkspaceConfigFileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "hooks: {}") || strings.Contains(s, "hooks:\n") && !strings.Contains(s, "# hooks:") {
		// Active hooks map must not appear; commented placeholder is fine.
		if strings.Contains(s, "\nhooks:") || strings.HasPrefix(s, "hooks:") {
			t.Fatalf("unexpected active hooks section:\n%s", s)
		}
	}

	loaded, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if !loaded.Hooks.IsZero() {
		t.Fatalf("absent hooks should IsZero, got %+v", loaded.Hooks)
	}
}

func TestLoadWorkspaceConfig_LegacyApprovalJudgeOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DotFestivalDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := "version: \"1.0\"\nhooks:\n  approval_judge:\n    command: my-judge\n"
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if cfg.Hooks.ApprovalJudge.Command != "my-judge" {
		t.Fatalf("command = %q, want my-judge", cfg.Hooks.ApprovalJudge.Command)
	}
	if cfg.Hooks.IsZero() {
		t.Fatal("legacy hooks section should set present")
	}
}

func TestSaveWorkspaceConfig_PlaceholderOnlyWhenEmpty(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultWorkspaceConfig()
	cfg.Hooks.ApprovalJudge.Command = "ob judge"
	cfg.Hooks.present = true
	if err := SaveWorkspaceConfig(root, cfg); err != nil {
		t.Fatalf("SaveWorkspaceConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, DotFestivalDir, WorkspaceConfigFileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "# hooks:") {
		t.Fatalf("placeholder should not appear when hooks configured:\n%s", data)
	}
}

func TestFestivalConfig_HooksRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultFestivalConfig()
	cfg.Hooks = HooksConfig{
		Enabled: boolPtr(false),
		Levels:  map[string]bool{"task": false},
		Definitions: map[string]HookDefinition{
			"check": {Command: "just test", Enabled: boolPtr(true)},
		},
		present: true,
	}
	if err := SaveFestivalConfig(dir, "", cfg); err != nil {
		t.Fatalf("SaveFestivalConfig: %v", err)
	}
	loaded, err := LoadFestivalConfig(dir, "")
	if err != nil {
		t.Fatalf("LoadFestivalConfig: %v", err)
	}
	if loaded.Hooks.Enabled == nil || *loaded.Hooks.Enabled {
		t.Fatalf("enabled = %v, want false", loaded.Hooks.Enabled)
	}
	if loaded.Hooks.Definitions["check"].Command != "just test" {
		t.Fatalf("definitions = %+v", loaded.Hooks.Definitions)
	}

	// Absent section drops on re-save.
	empty := DefaultFestivalConfig()
	if err := SaveFestivalConfig(dir, "", empty); err != nil {
		t.Fatalf("SaveFestivalConfig empty: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, FestivalConfigFileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "hooks:") {
		t.Fatalf("absent hooks should not be emitted:\n%s", raw)
	}
}
