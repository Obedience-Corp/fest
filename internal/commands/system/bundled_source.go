package system

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
)

// bundledMethodologyApplies reports whether the scaffold embedded in this
// binary is a legitimate stand-in for what a sync would have fetched.
//
// The bundle is a copy of the default repository's methodology. When an
// operator has pointed fest at their own repository, ours is not a substitute
// for theirs: seeding it would hand them a methodology they never chose, and
// the swap would be easy to miss because init otherwise looks successful.
// Those configurations keep failing when the network is down, which is the
// honest outcome — fest cannot produce templates it has never seen.
func bundledMethodologyApplies(repo config.Repository) bool {
	if repo.URL != "" && repo.URL != config.DefaultRepositoryURL {
		return false
	}
	if repo.Path != "" && repo.Path != config.DefaultRepoPath {
		return false
	}
	return true
}

// usesBundledMethodology answers the same question from the on-disk config.
// A machine with no config at all is the first-run case the bundle exists for,
// so an unreadable or absent config falls back rather than failing.
func usesBundledMethodology(ctx context.Context) bool {
	cfg, err := config.Load(ctx, shared.GetConfigFile())
	if err != nil || cfg == nil {
		return true
	}
	return bundledMethodologyApplies(cfg.Repository)
}
