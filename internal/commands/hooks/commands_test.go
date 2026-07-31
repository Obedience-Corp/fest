package hooks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

// fest hooks list --json is parsed by agents, so its key set is a contract.
// These tests pin the shape rather than only the values.

func marshalHooksListView(t *testing.T, eff *hooks.Effective) map[string]any {
	t.Helper()
	data, err := json.Marshal(buildHooksListView(eff))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func TestBuildHooksListView_JSONKeysAreTheDocumentedSet(t *testing.T) {
	eff := &hooks.Effective{
		Enabled: true,
		Levels:  map[string]bool{"phase": true, "sequence": true, "task": false},
		Hooks: map[string]hooks.ResolvedHook{
			hooks.ApprovalJudgeName: {
				Name:    hooks.ApprovalJudgeName,
				Command: "ob judge",
				Fail:    hooks.FailClosed,
				Timeout: hooks.NoTimeout,
				Enabled: true,
				Source:  hooks.LayerFestivals,
			},
		},
	}

	decoded := marshalHooksListView(t, eff)
	entries, ok := decoded["hooks"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("hooks = %#v, want one entry", decoded["hooks"])
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("entry = %#v", entries[0])
	}

	// evidence is deliberately absent: the embed contract is gone, and a key
	// that always reports "paths" would teach a knob that no longer exists.
	want := map[string]bool{
		"name": true, "source": true, "enabled": true,
		"command": true, "fail": true, "timeout": true, "shadows": true,
	}
	for key := range entry {
		if !want[key] {
			t.Errorf("unexpected key %q in hooks list JSON", key)
		}
	}
	for key := range want {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing key %q in hooks list JSON", key)
		}
	}
}

func TestBuildHooksListView_ReportsResolvedValues(t *testing.T) {
	eff := &hooks.Effective{
		Enabled: true,
		Levels:  map[string]bool{"phase": true},
		Hooks: map[string]hooks.ResolvedHook{
			"lint": {
				Name:    "lint",
				Command: "just lint",
				Fail:    hooks.FailOpen,
				Timeout: 60 * time.Second,
				Enabled: false,
				Source:  hooks.LayerFestival,
				Shadowed: []hooks.ShadowedDef{
					{Source: hooks.LayerMachine, Def: config.HookDefinition{Command: "golangci-lint run"}},
				},
			},
		},
	}

	view := buildHooksListView(eff)
	if len(view.Hooks) != 1 {
		t.Fatalf("hooks = %+v, want one entry", view.Hooks)
	}
	got := view.Hooks[0]
	if got.Name != "lint" || got.Command != "just lint" || got.Source != "festival" {
		t.Fatalf("entry = %+v", got)
	}
	if got.Fail != "open" || got.Timeout != "1m0s" || got.Enabled {
		t.Fatalf("resolved values = %+v", got)
	}
	// A local override must stay visible so it cannot silently hide an
	// updated upper-layer default.
	if len(got.Shadows) != 1 || got.Shadows[0].Command != "golangci-lint run" || !got.Shadows[0].Differs {
		t.Fatalf("shadows = %+v", got.Shadows)
	}
}

func TestBuildHooksListView_NilEffectiveIsEmptyNotNull(t *testing.T) {
	decoded := marshalHooksListView(t, nil)
	// Marshalling nil slices as null would break a consumer that ranges over
	// the value without a nil check.
	if entries, ok := decoded["hooks"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("hooks = %#v, want []", decoded["hooks"])
	}
	if levels, ok := decoded["levels"].(map[string]any); !ok || len(levels) != 0 {
		t.Fatalf("levels = %#v, want {}", decoded["levels"])
	}
}
