package config

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/version"
)

// setVersion temporarily overrides the package-level version.Version variable
// and restores it when the test finishes.
func setVersion(t *testing.T, v string) {
	t.Helper()
	original := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = original })
}

// profileChannel returns the channel a release-profile fallback resolves to for
// the build profile these tests were compiled with, so the expectations hold
// under both `go test ./...` and `go test -tags dev ./...`.
func profileChannel() string {
	if version.Profile == "dev" {
		return "dev"
	}
	return "stable"
}

func TestResolveRefIntent_CLIFlags(t *testing.T) {
	setVersion(t, "v0.2.0") // stable build

	repo := Repository{
		URL:      DefaultRepositoryURL,
		Branch:   DefaultBranch,
		SyncMode: "channel",
	}

	t.Run("cli tag wins over everything", func(t *testing.T) {
		got := ResolveRefIntent("", "v0.1.0", "", repo)
		if got.Mode != SyncModeTag || got.Value != "v0.1.0" {
			t.Errorf("got %+v, want {SyncModeTag, v0.1.0}", got)
		}
	})

	t.Run("cli branch wins over channel", func(t *testing.T) {
		got := ResolveRefIntent("feature/foo", "", "", repo)
		if got.Mode != SyncModeBranch || got.Value != "feature/foo" {
			t.Errorf("got %+v, want {SyncModeBranch, feature/foo}", got)
		}
	})

	t.Run("cli tag wins over cli branch", func(t *testing.T) {
		got := ResolveRefIntent("feature/foo", "v0.1.0", "", repo)
		if got.Mode != SyncModeTag || got.Value != "v0.1.0" {
			t.Errorf("got %+v, want {SyncModeTag, v0.1.0}", got)
		}
	})

	t.Run("cli channel used when no tag or branch", func(t *testing.T) {
		got := ResolveRefIntent("", "", "dev", repo)
		if got.Mode != SyncModeChannel || got.Value != "dev" {
			t.Errorf("got %+v, want {SyncModeChannel, dev}", got)
		}
	})
}

func TestResolveRefIntent_ConfigDriven(t *testing.T) {
	t.Run("config channel mode uses repo.Channel", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		repo := Repository{SyncMode: "channel", Channel: "dev"}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeChannel || got.Value != "dev" {
			t.Errorf("got %+v, want {SyncModeChannel, dev}", got)
		}
	})

	t.Run("config channel mode falls back to release profile when Channel empty", func(t *testing.T) {
		setVersion(t, "v0.2.0") // stable version string
		repo := Repository{SyncMode: "channel", Channel: ""}
		got := ResolveRefIntent("", "", "", repo)
		want := profileChannel()
		if got.Mode != SyncModeChannel || got.Value != want {
			t.Errorf("got %+v, want {SyncModeChannel, %s}", got, want)
		}
	})

	t.Run("config channel mode uses dev profile for dev build", func(t *testing.T) {
		setVersion(t, "dev")
		repo := Repository{SyncMode: "channel", Channel: ""}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeChannel || got.Value != "dev" {
			t.Errorf("got %+v, want {SyncModeChannel, dev}", got)
		}
	})

	t.Run("config branch mode uses repo.Ref", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		repo := Repository{SyncMode: "branch", Ref: "release/v0.2"}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeBranch || got.Value != "release/v0.2" {
			t.Errorf("got %+v, want {SyncModeBranch, release/v0.2}", got)
		}
	})

	t.Run("config branch mode falls back to repo.Branch when Ref empty", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		repo := Repository{SyncMode: "branch", Branch: "develop"}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeBranch || got.Value != "develop" {
			t.Errorf("got %+v, want {SyncModeBranch, develop}", got)
		}
	})

	t.Run("config tag mode uses repo.Ref", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		repo := Repository{SyncMode: "tag", Ref: "v0.1.5"}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeTag || got.Value != "v0.1.5" {
			t.Errorf("got %+v, want {SyncModeTag, v0.1.5}", got)
		}
	})
}

func TestResolveRefIntent_BackwardCompat(t *testing.T) {
	t.Run("empty SyncMode with non-default branch uses branch mode", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		// Old config: SyncMode was never set, user had branch=develop
		repo := Repository{Branch: "develop", SyncMode: ""}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeBranch || got.Value != "develop" {
			t.Errorf("got %+v, want {SyncModeBranch, develop}", got)
		}
	})

	t.Run("empty SyncMode with default branch falls through to release profile", func(t *testing.T) {
		setVersion(t, "v0.2.0") // stable version string
		repo := Repository{Branch: DefaultBranch, SyncMode: ""}
		got := ResolveRefIntent("", "", "", repo)
		want := profileChannel()
		if got.Mode != SyncModeChannel || got.Value != want {
			t.Errorf("got %+v, want {SyncModeChannel, %s}", got, want)
		}
	})

	t.Run("empty SyncMode and empty Branch falls through to release profile", func(t *testing.T) {
		setVersion(t, "dev")
		repo := Repository{Branch: "", SyncMode: ""}
		got := ResolveRefIntent("", "", "", repo)
		if got.Mode != SyncModeChannel || got.Value != "dev" {
			t.Errorf("got %+v, want {SyncModeChannel, dev}", got)
		}
	})
}

func TestResolveRefIntent_DefaultConfig(t *testing.T) {
	t.Run("default config resolves to the release-profile channel", func(t *testing.T) {
		setVersion(t, "v0.2.0")
		cfg := DefaultConfig()
		got := ResolveRefIntent("", "", "", cfg.Repository)
		want := profileChannel()
		if got.Mode != SyncModeChannel || got.Value != want {
			t.Errorf("got %+v, want {SyncModeChannel, %s}", got, want)
		}
	})

	t.Run("default config with dev build resolves to dev channel", func(t *testing.T) {
		setVersion(t, "dev")
		cfg := DefaultConfig()
		got := ResolveRefIntent("", "", "", cfg.Repository)
		if got.Mode != SyncModeChannel || got.Value != "dev" {
			t.Errorf("got %+v, want {SyncModeChannel, dev}", got)
		}
	})
}
