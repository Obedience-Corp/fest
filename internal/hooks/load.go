package hooks

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// LoadAndResolve loads machine, festivals, and festival hook configs and resolves them.
func LoadAndResolve(ctx context.Context, festivalPath string) (*Effective, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if festivalPath == "" {
		return nil, festerrors.Validation("festival path required for hook resolution")
	}

	machineCfg, err := config.Load(ctx, "")
	if err != nil {
		return nil, err
	}
	var machine *config.HooksConfig
	if machineCfg != nil {
		machine = machineCfg.Hooks
	}

	var festivals *config.HooksConfig
	ws, wsErr := workspace.FindWorkspace(ctx, festivalPath)
	if wsErr == nil && ws.FestivalsPath != "" {
		wcfg, err := config.LoadWorkspaceConfig(ws.FestivalsPath)
		if err != nil {
			return nil, err
		}
		if wcfg != nil {
			fc := wcfg.Hooks
			festivals = &fc
		}
	}

	campaignRoot := ""
	if wsErr == nil {
		campaignRoot = ws.Root
	}
	fcfg, err := config.LoadFestivalConfig(festivalPath, campaignRoot)
	if err != nil {
		return nil, err
	}
	var festival *config.HooksConfig
	if fcfg != nil {
		fc := fcfg.Hooks
		festival = &fc
	}

	return Resolve(machine, festivals, festival)
}
