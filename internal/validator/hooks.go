package validator

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// ValidateHooksConfig emits warnings for legacy hook configuration shapes.
func ValidateHooksConfig(ctx context.Context, festivalPath string) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var festivalsRoot string
	ws, err := workspace.FindWorkspace(ctx, festivalPath)
	if err == nil {
		festivalsRoot = ws.FestivalsPath
	}
	if festivalsRoot == "" {
		// Fallback: parent of festival when it lives under festivals/<status>/<name>
		festivalsRoot = filepath.Dir(filepath.Dir(festivalPath))
	}

	wcfg, err := config.LoadWorkspaceConfig(festivalsRoot)
	if err != nil {
		return nil, err
	}
	if wcfg == nil {
		return nil, nil
	}

	cmd := strings.TrimSpace(wcfg.Hooks.ApprovalJudge.Command)
	if cmd == "" {
		return nil, nil
	}
	if _, explicit := wcfg.Hooks.Definitions[hooks.ApprovalJudgeName]; explicit {
		return nil, nil
	}

	return []Issue{{
		Level:   LevelWarning,
		Code:    CodeHooksLegacyAlias,
		Path:    filepath.Join(festivalsRoot, config.DotFestivalDir, config.WorkspaceConfigFileName),
		Message: hooks.ApprovalJudgeAliasWarning(cmd),
		Fix:     "Move the command under hooks.definitions.approval_judge with timeout: 0",
	}}, nil
}
