package hooks

import (
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
)

const ApprovalJudgeName = "approval_judge"

// applyApprovalJudgeAlias expands the legacy flat key into an implicit
// festivals-layer definition. R6: timeout 0 preserves today's no-deadline
// judge behavior. Mutates cfg in memory only; does not write disk.
//
// Explicit definitions.approval_judge wins over the flat key.
func applyApprovalJudgeAlias(festivals *config.HooksConfig) (aliased bool, cmd string) {
	if festivals == nil {
		return false, ""
	}
	cmd = strings.TrimSpace(festivals.ApprovalJudge.Command)
	if cmd == "" {
		return false, ""
	}
	if _, explicit := festivals.Definitions[ApprovalJudgeName]; explicit {
		return false, "" // explicit definition wins; flat key ignored
	}
	if festivals.Definitions == nil {
		festivals.Definitions = map[string]config.HookDefinition{}
	}
	festivals.Definitions[ApprovalJudgeName] = config.HookDefinition{
		Command:  cmd,
		Fail:     string(FailClosed),
		Timeout:  "0", // R6: no default 120s for the legacy alias
		Evidence: string(EvidencePaths),
	}
	return true, cmd
}

// cloneHooksConfig returns a shallow copy with a cloned Definitions map so
// alias injection does not mutate the caller's config.
func cloneHooksConfig(src *config.HooksConfig) *config.HooksConfig {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Levels != nil {
		cp.Levels = make(map[string]bool, len(src.Levels))
		for k, v := range src.Levels {
			cp.Levels[k] = v
		}
	}
	if src.Definitions != nil {
		cp.Definitions = make(map[string]config.HookDefinition, len(src.Definitions))
		for k, v := range src.Definitions {
			cp.Definitions[k] = v
		}
	}
	return &cp
}

// ApprovalJudgeAliasWarning returns the migration text for the legacy flat key.
func ApprovalJudgeAliasWarning(cmd string) string {
	return fmt.Sprintf(`hooks.approval_judge.command is deprecated; declare it explicitly:

hooks:
  definitions:
    approval_judge:
      command: %s
      timeout: 0   # preserves today's no-deadline judge behavior

The flat key still works this release and will be removed after a migration window.`, cmd)
}

// ShouldBindApprovalJudgeOnGates reports whether GATES.md blocking approval
// steps should receive an implicit post-binding of approval_judge when no
// explicit **Hooks:** marker is present (legacy compatibility heuristic).
func (e *Effective) ShouldBindApprovalJudgeOnGates() bool {
	if e == nil {
		return false
	}
	_, ok := e.Hooks[ApprovalJudgeName]
	return ok
}
