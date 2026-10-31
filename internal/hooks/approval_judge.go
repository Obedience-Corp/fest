package hooks

import (
	"context"
	"errors"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// LookupApprovalJudgeCommand uses the same layered discovery and workspace
// fallback for approval decisions and human notification routing.
func LookupApprovalJudgeCommand(ctx context.Context, festivalPath string) (string, error) {
	festivalsRoot := ""
	if ws, ok := scope.WorkspaceFrom(ctx); ok && ws != nil && strings.TrimSpace(ws.FestivalsPath) != "" {
		festivalsRoot = ws.FestivalsPath
	} else if strings.TrimSpace(festivalPath) != "" {
		ws, err := workspace.FindWorkspace(ctx, festivalPath)
		if err != nil && !errors.Is(err, workspace.ErrNoWorkspace) && !errors.Is(err, workspace.ErrNoCampaign) {
			return "", festerrors.Wrap(err, "resolving approval judge authority configuration")
		}
		if err == nil {
			festivalsRoot = ws.FestivalsPath
		}
	}
	return ResolveApprovalJudgeCommand(ctx, festivalPath, festivalsRoot)
}

// ResolveApprovalJudgeCommand preserves existing approval command discovery,
// including its workspace fallback when no festival-level command is found.
func ResolveApprovalJudgeCommand(ctx context.Context, festivalPath, festivalsRoot string) (string, error) {
	if festivalPath != "" {
		eff, err := LoadAndResolve(ctx, festivalPath)
		if err != nil {
			return "", festerrors.Wrap(err, "resolving approval judge hooks")
		}
		if cmd := ApprovalJudgeCommand(eff); cmd != "" {
			return cmd, nil
		}
	}

	if festivalsRoot == "" {
		return "", nil
	}

	var machine *config.HooksConfig
	if machineCfg, err := config.Load(ctx, ""); err != nil {
		return "", festerrors.Wrap(err, "loading machine hooks for approval judge")
	} else if machineCfg != nil {
		machine = machineCfg.Hooks
	}

	wcfg, err := config.LoadWorkspaceConfig(festivalsRoot)
	if err != nil {
		return "", festerrors.Wrap(err, "loading approval judge authority configuration")
	}
	if wcfg == nil {
		return "", festerrors.Validation("approval judge authority configuration is empty")
	}
	festivals := wcfg.Hooks
	eff, err := Resolve(machine, &festivals, nil)
	if err != nil {
		return "", festerrors.Wrap(err, "resolving approval judge hooks")
	}
	return ApprovalJudgeCommand(eff), nil
}
