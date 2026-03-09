package config

import "github.com/Obedience-Corp/fest/internal/version"

// SyncMode describes how the fest sync command resolves a target ref.
type SyncMode string

const (
	// SyncModeChannel syncs to the latest release in the given channel (stable/dev).
	SyncModeChannel SyncMode = "channel"

	// SyncModeBranch syncs to the tip of a named git branch.
	SyncModeBranch SyncMode = "branch"

	// SyncModeTag syncs to an exact git tag.
	SyncModeTag SyncMode = "tag"
)

// RefIntent captures the resolved synchronization intent: which mode to use and
// the concrete value (channel name, branch name, or tag string) to pass to the
// downloader.
type RefIntent struct {
	Mode  SyncMode
	Value string
}

// ResolveRefIntent determines the RefIntent to use for a sync operation.
//
// Precedence (highest to lowest):
//  1. CLI flags — if cliBranch or cliTag is non-empty it wins outright.
//     cliChannel is also a CLI flag but lower priority than branch/tag.
//  2. Repository config fields (SyncMode / Channel / Ref / Branch).
//  3. Release-profile default: DefaultChannel() from the version package.
//
// Backward compatibility: a config that has SyncMode="" but Branch set to a
// non-default value (i.e. not DefaultBranch/"main") is treated as SyncModeBranch
// so that existing configs keep working after the upgrade.
func ResolveRefIntent(cliBranch, cliTag, cliChannel string, repo Repository) RefIntent {
	// 1a. Explicit tag from CLI takes highest priority.
	if cliTag != "" {
		return RefIntent{Mode: SyncModeTag, Value: cliTag}
	}

	// 1b. Explicit branch from CLI.
	if cliBranch != "" {
		return RefIntent{Mode: SyncModeBranch, Value: cliBranch}
	}

	// 1c. Explicit channel from CLI.
	if cliChannel != "" {
		return RefIntent{Mode: SyncModeChannel, Value: cliChannel}
	}

	// 2. Config-driven resolution.
	switch SyncMode(repo.SyncMode) {
	case SyncModeTag:
		if repo.Ref != "" {
			return RefIntent{Mode: SyncModeTag, Value: repo.Ref}
		}
		// sync_mode="tag" with empty ref is a misconfiguration.
		// Fall through to release-profile default (channel mode) so the
		// user still gets a working sync rather than a hard failure.
	case SyncModeBranch:
		ref := repo.Ref
		if ref == "" {
			ref = repo.Branch
		}
		if ref != "" {
			return RefIntent{Mode: SyncModeBranch, Value: ref}
		}
	case SyncModeChannel:
		ch := repo.Channel
		if ch == "" {
			ch = version.DefaultChannel()
		}
		return RefIntent{Mode: SyncModeChannel, Value: ch}
	}

	// Backward-compat: SyncMode is empty but Branch is set to a non-default value.
	if repo.SyncMode == "" && repo.Branch != "" && repo.Branch != DefaultBranch {
		return RefIntent{Mode: SyncModeBranch, Value: repo.Branch}
	}

	// 3. Release-profile default.
	return RefIntent{Mode: SyncModeChannel, Value: version.DefaultChannel()}
}
