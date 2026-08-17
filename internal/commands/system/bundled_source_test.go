package system

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

// The bundled scaffold is a copy of the default repository. Seeding it for an
// operator who pointed fest at their own methodology would replace their
// templates with ours, and init would still report success — so the substitution
// is only legitimate when the configured source is the one we bundled.
func TestBundledMethodologyApplies(t *testing.T) {
	tests := []struct {
		name string
		repo config.Repository
		want bool
	}{
		{
			name: "unconfigured falls back",
			repo: config.Repository{},
			want: true,
		},
		{
			name: "explicit default falls back",
			repo: config.Repository{URL: config.DefaultRepositoryURL, Path: config.DefaultRepoPath},
			want: true,
		},
		{
			name: "default url with unset path falls back",
			repo: config.Repository{URL: config.DefaultRepositoryURL},
			want: true,
		},
		{
			name: "a pinned channel still uses the same repository",
			repo: config.Repository{URL: config.DefaultRepositoryURL, Channel: "stable"},
			want: true,
		},
		{
			name: "someone else's repository must not be substituted",
			repo: config.Repository{URL: "https://github.com/some-org/our-own-methodology"},
			want: false,
		},
		{
			name: "default repository at a different path must not be substituted",
			repo: config.Repository{URL: config.DefaultRepositoryURL, Path: "some/other/scaffold"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bundledMethodologyApplies(tt.repo); got != tt.want {
				t.Errorf("bundledMethodologyApplies(%+v) = %v, want %v", tt.repo, got, tt.want)
			}
		})
	}
}
