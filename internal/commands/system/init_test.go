package system

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

func TestResolveDisplayRoot(t *testing.T) {
	campaignRoot := "/home/user/campaigns/my-campaign"
	workspaceRoot := "/home/user/campaigns/my-campaign/festivals/active/fest-F0001/feedback"

	tests := []struct {
		name                  string
		campaignRoot          string
		campaignErr           error
		festivalWorkspaceRoot string
		want                  string
	}{
		{
			name:                  "prefers campaign root when detected",
			campaignRoot:          campaignRoot,
			campaignErr:           nil,
			festivalWorkspaceRoot: workspaceRoot,
			want:                  campaignRoot,
		},
		{
			name:                  "falls back to festival workspace root when no campaign",
			campaignRoot:          "",
			campaignErr:           workspace.ErrNoCampaign,
			festivalWorkspaceRoot: workspaceRoot,
			want:                  workspaceRoot,
		},
		{
			name:                  "falls back when campaign root empty even without error",
			campaignRoot:          "",
			campaignErr:           nil,
			festivalWorkspaceRoot: workspaceRoot,
			want:                  workspaceRoot,
		},
		{
			name:                  "standalone init target",
			campaignRoot:          "",
			campaignErr:           errors.New("no campaign"),
			festivalWorkspaceRoot: "/tmp/standalone-project",
			want:                  "/tmp/standalone-project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDisplayRoot(tc.campaignRoot, tc.campaignErr, tc.festivalWorkspaceRoot)
			if got != tc.want {
				t.Errorf("resolveDisplayRoot() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInitDisplayPaths_CampaignRelative(t *testing.T) {
	campaignRoot := "/home/lance/campaigns/lance-arch"
	absPath := filepath.Join(campaignRoot, "festivals", "active", "endeavouros-workstation-setup-EW0001", "feedback")
	festivalPath := filepath.Join(absPath, "festivals")
	checksumFile := filepath.Join(festivalPath, ".festival", ".state", ".fest-checksums.json")

	displayRoot := resolveDisplayRoot(campaignRoot, nil, absPath)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "festivals directory",
			path: festivalPath,
			want: "festivals/active/endeavouros-workstation-setup-EW0001/feedback/festivals",
		},
		{
			name: "checksum file",
			path: checksumFile,
			want: "festivals/active/endeavouros-workstation-setup-EW0001/feedback/festivals/.festival/.state/.fest-checksums.json",
		},
		{
			name: "init target for cd",
			path: absPath,
			want: "festivals/active/endeavouros-workstation-setup-EW0001/feedback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pathutil.DisplayPath(tc.path, displayRoot)
			if got != tc.want {
				t.Errorf("DisplayPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
			if filepath.IsAbs(got) {
				t.Errorf("DisplayPath(%q) still absolute: %q", tc.path, got)
			}
		})
	}
}

func TestInitDisplayPaths_StandaloneFestivalRootRelative(t *testing.T) {
	// No campaign: paths should be relative to the init target (festival workspace root).
	absPath := "/tmp/my-standalone-project"
	festivalPath := filepath.Join(absPath, "festivals")
	checksumFile := filepath.Join(festivalPath, ".festival", ".state", ".fest-checksums.json")

	displayRoot := resolveDisplayRoot("", workspace.ErrNoCampaign, absPath)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "festivals directory",
			path: festivalPath,
			want: "festivals",
		},
		{
			name: "checksum file",
			path: checksumFile,
			want: "festivals/.festival/.state/.fest-checksums.json",
		},
		{
			name: "init target for cd",
			path: absPath,
			want: ".",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pathutil.DisplayPath(tc.path, displayRoot)
			if got != tc.want {
				t.Errorf("DisplayPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
			if filepath.IsAbs(got) {
				t.Errorf("DisplayPath(%q) still absolute: %q", tc.path, got)
			}
		})
	}
}
