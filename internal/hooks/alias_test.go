package hooks

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

func TestAlias_FlatKeyInjectsApprovalJudge(t *testing.T) {
	festivals := &config.HooksConfig{
		ApprovalJudge: config.ApprovalJudgeHookConfig{Command: "ob judge"},
	}
	// Ensure we do not mutate caller's Definitions map identity unexpectedly.
	orig := festivals

	eff, err := Resolve(nil, festivals, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if festivals.Definitions != nil {
		t.Fatal("resolver must not mutate caller's Definitions map")
	}
	_ = orig

	h, ok := eff.Hooks[ApprovalJudgeName]
	if !ok {
		t.Fatal("expected approval_judge from alias")
	}
	if h.Command != "ob judge" || h.Fail != FailClosed || h.Timeout != 0 || h.Evidence != EvidencePaths {
		t.Fatalf("resolved alias = %+v", h)
	}
	if h.Source != LayerFestivals {
		t.Fatalf("source = %s, want festivals", h.Source)
	}
	if !eff.LegacyAliasActive || eff.LegacyAliasCommand != "ob judge" {
		t.Fatalf("legacy flags = active=%v cmd=%q", eff.LegacyAliasActive, eff.LegacyAliasCommand)
	}
	if !eff.ShouldBindApprovalJudgeOnGates() {
		t.Fatal("alias should enable gate binding signal")
	}
}

func TestAlias_ExplicitDefinitionWins(t *testing.T) {
	festivals := &config.HooksConfig{
		ApprovalJudge: config.ApprovalJudgeHookConfig{Command: "flat-cmd"},
		Definitions: map[string]config.HookDefinition{
			ApprovalJudgeName: {Command: "explicit-cmd", Timeout: "5s"},
		},
	}
	eff, err := Resolve(nil, festivals, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	h := eff.Hooks[ApprovalJudgeName]
	if h.Command != "explicit-cmd" {
		t.Fatalf("command = %q, want explicit-cmd", h.Command)
	}
	if h.Timeout == 0 {
		t.Fatal("explicit timeout should apply (not alias zero)")
	}
	if eff.LegacyAliasActive {
		t.Fatal("alias should not apply when explicit definition present")
	}
}

func TestAlias_NoFlatKeyNoDefinition(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, ok := eff.Hooks[ApprovalJudgeName]; ok {
		t.Fatal("unexpected approval_judge")
	}
	if eff.LegacyAliasActive {
		t.Fatal("no alias expected")
	}
	if eff.ShouldBindApprovalJudgeOnGates() {
		t.Fatal("no gate binding without approval_judge")
	}
}

func TestAlias_WarningText(t *testing.T) {
	msg := ApprovalJudgeAliasWarning("my-judge --strict")
	if !strings.Contains(msg, "my-judge --strict") {
		t.Fatalf("missing command in warning: %s", msg)
	}
	if !strings.Contains(msg, "timeout: 0") {
		t.Fatalf("missing timeout: 0 line: %s", msg)
	}
	if !strings.Contains(msg, "definitions:") {
		t.Fatalf("missing definitions block: %s", msg)
	}
}
