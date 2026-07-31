package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
)

func bp(v bool) *bool { return &v }

func TestResolve_InvalidFail(t *testing.T) {
	_, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			"x": {Command: "true", Fail: "maybe"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid hook fail policy") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolve_InvalidTimeout(t *testing.T) {
	_, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			"x": {Command: "true", Timeout: "soon"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid hook timeout") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolve_ByNameWholeDefinitionOverride(t *testing.T) {
	festivals := &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			"lint": {Command: "ws-lint", Fail: "open", Timeout: "30s"},
		},
	}
	festival := &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			"lint": {Command: "fest-lint"},
		},
	}
	eff, err := Resolve(nil, festivals, festival)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	h := eff.Hooks["lint"]
	if h.Command != "fest-lint" {
		t.Fatalf("command = %q, want fest-lint (whole replace)", h.Command)
	}
	if h.Fail != FailClosed || h.Timeout != DefaultTimeout {
		t.Fatalf("want defaults after replace, got fail=%s timeout=%v", h.Fail, h.Timeout)
	}
	if h.Source != LayerFestival {
		t.Fatalf("source = %s, want festival", h.Source)
	}
}

// approval_judge shells out to an LLM, and a timeout fails closed. Defaulting it
// to a wall-clock deadline would block gates on slow judges rather than judge
// them, so an omitted timeout means no deadline for that name only.
func TestResolve_ApprovalJudgeDefaultsToNoTimeout(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			ApprovalJudgeName: {Command: "ob judge"},
			"lint":            {Command: "just lint"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := eff.Hooks[ApprovalJudgeName].Timeout; got != NoTimeout {
		t.Fatalf("approval_judge timeout = %v, want no deadline", got)
	}
	if got := eff.Hooks["lint"].Timeout; got != DefaultTimeout {
		t.Fatalf("lint timeout = %v, want %v", got, DefaultTimeout)
	}
}

func TestResolve_ApprovalJudgeExplicitTimeoutWins(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			ApprovalJudgeName: {Command: "ob judge", Timeout: "45s"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := eff.Hooks[ApprovalJudgeName].Timeout; got != 45*time.Second {
		t.Fatalf("approval_judge timeout = %v, want 45s", got)
	}
}

func TestResolve_ShadowedDifferingOnly(t *testing.T) {
	same := config.HookDefinition{Command: "same"}
	festivals := &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{"a": {Command: "old"}},
	}
	festival := &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{"a": {Command: "new"}},
	}
	eff, err := Resolve(nil, festivals, festival)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(eff.Hooks["a"].Shadowed) != 1 || eff.Hooks["a"].Shadowed[0].Def.Command != "old" {
		t.Fatalf("shadowed = %+v", eff.Hooks["a"].Shadowed)
	}

	festivals.Definitions["b"] = same
	festival.Definitions["b"] = same
	eff, err = Resolve(nil, festivals, festival)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(eff.Hooks["b"].Shadowed) != 0 {
		t.Fatalf("identical defs should not shadow, got %+v", eff.Hooks["b"].Shadowed)
	}
}

func TestResolve_SwitchesMostSpecificWins(t *testing.T) {
	machine := &config.HooksConfig{Enabled: bp(true)}
	festival := &config.HooksConfig{Enabled: bp(false)}
	eff, err := Resolve(machine, nil, festival)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if eff.Enabled {
		t.Fatal("festival enabled:false should win")
	}

	eff, err = Resolve(machine, nil, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !eff.Enabled {
		t.Fatal("machine true should stick when festival unset")
	}
	eff.Hooks["h"] = ResolvedHook{Name: "h", Enabled: true}
	if !eff.Runnable("h", "unknown-level") {
		t.Fatal("unknown level should default true")
	}
}

func TestResolve_PerLevelAndPerHookDisable(t *testing.T) {
	cfg := &config.HooksConfig{
		Levels: map[string]bool{"task": false},
		Definitions: map[string]config.HookDefinition{
			"a": {Command: "true"},
			"b": {Command: "true", Enabled: bp(false)},
		},
	}
	eff, err := Resolve(nil, cfg, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if eff.Runnable("a", "task") {
		t.Fatal("levels.task=false should disable task runs")
	}
	if !eff.Runnable("a", "phase") {
		t.Fatal("phase should still run")
	}
	if eff.Runnable("b", "phase") {
		t.Fatal("per-hook enabled:false should disable")
	}
}

func TestResolve_DefaultsApplied(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{"only": {Command: "echo hi"}},
	}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	h := eff.Hooks["only"]
	if h.Fail != FailClosed || h.Timeout != 120*time.Second || !h.Enabled {
		t.Fatalf("defaults wrong: %+v", h)
	}
}

func TestResolve_EmptyAllLayers(t *testing.T) {
	eff, err := Resolve(nil, nil, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(eff.Hooks) != 0 || !eff.Enabled {
		t.Fatalf("empty resolve = %+v", eff)
	}
}

func TestMachineJSON_HooksRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.ConfigFileName)
	raw := map[string]any{
		"version": "1.0",
		"hooks": map[string]any{
			"enabled": true,
			"definitions": map[string]any{
				"lint": map[string]any{"command": "just lint", "timeout": "5s"},
			},
		},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hooks == nil || cfg.Hooks.Definitions["lint"].Command != "just lint" {
		t.Fatalf("hooks = %+v", cfg.Hooks)
	}

	path2 := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path2, []byte(`{"version":"1.0"}`), 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	cfg2, err := config.Load(context.Background(), path2)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if cfg2.Hooks != nil {
		t.Fatalf("absent hooks should be nil, got %+v", cfg2.Hooks)
	}
}
