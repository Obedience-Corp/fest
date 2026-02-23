package pathutil

import (
	"path/filepath"
	"testing"
)

func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestToRelativePath(t *testing.T) {
	campaignRoot := "/home/user/campaigns/my-campaign"

	tests := []struct {
		name         string
		absPath      string
		campaignRoot string
		want         string
	}{
		{
			name:         "path inside campaign",
			absPath:      "/home/user/campaigns/my-campaign/festivals/active/my-fest",
			campaignRoot: campaignRoot,
			want:         "festivals/active/my-fest",
		},
		{
			name:         "path at campaign root",
			absPath:      "/home/user/campaigns/my-campaign",
			campaignRoot: campaignRoot,
			want:         ".",
		},
		{
			name:         "path outside campaign",
			absPath:      "/some/other/path",
			campaignRoot: campaignRoot,
			want:         "/some/other/path",
		},
		{
			name:         "already relative",
			absPath:      "festivals/active/my-fest",
			campaignRoot: campaignRoot,
			want:         "festivals/active/my-fest",
		},
		{
			name:         "empty path",
			absPath:      "",
			campaignRoot: campaignRoot,
			want:         "",
		},
		{
			name:         "project submodule path",
			absPath:      "/home/user/campaigns/my-campaign/projects/fest",
			campaignRoot: campaignRoot,
			want:         "projects/fest",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ToRelativePath(tc.absPath, tc.campaignRoot)
			if got != tc.want {
				t.Errorf("ToRelativePath(%q, %q) = %q, want %q", tc.absPath, tc.campaignRoot, got, tc.want)
			}
		})
	}
}

func TestToAbsolutePath(t *testing.T) {
	campaignRoot := "/home/user/campaigns/my-campaign"

	tests := []struct {
		name         string
		path         string
		campaignRoot string
		want         string
	}{
		{
			name:         "relative path",
			path:         "festivals/active/my-fest",
			campaignRoot: campaignRoot,
			want:         "/home/user/campaigns/my-campaign/festivals/active/my-fest",
		},
		{
			name:         "already absolute",
			path:         "/some/absolute/path",
			campaignRoot: campaignRoot,
			want:         "/some/absolute/path",
		},
		{
			name:         "dot path",
			path:         ".",
			campaignRoot: campaignRoot,
			want:         "/home/user/campaigns/my-campaign",
		},
		{
			name:         "empty path becomes campaign root",
			path:         "",
			campaignRoot: campaignRoot,
			want:         "/home/user/campaigns/my-campaign",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ToAbsolutePath(tc.path, tc.campaignRoot)
			if got != tc.want {
				t.Errorf("ToAbsolutePath(%q, %q) = %q, want %q", tc.path, tc.campaignRoot, got, tc.want)
			}
		})
	}
}

func TestDisplayPath(t *testing.T) {
	campaignRoot := "/home/user/campaigns/my-campaign"
	absPath := "/home/user/campaigns/my-campaign/festivals/active/my-fest"

	got := DisplayPath(absPath, campaignRoot)
	want := "festivals/active/my-fest"

	if got != want {
		t.Errorf("DisplayPath(%q, %q) = %q, want %q", absPath, campaignRoot, got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	campaignRoot := "/home/user/campaigns/my-campaign"
	original := "/home/user/campaigns/my-campaign/festivals/active/my-fest"

	relative := ToRelativePath(original, campaignRoot)
	absolute := ToAbsolutePath(relative, campaignRoot)

	if absolute != original {
		t.Errorf("Round trip failed: %q → %q → %q", original, relative, absolute)
	}
}

func TestToRelativePath_RealTempDir(t *testing.T) {
	tmpDir := resolvePath(t, t.TempDir())

	absPath := filepath.Join(tmpDir, "festivals", "active", "my-fest")
	got := ToRelativePath(absPath, tmpDir)
	want := filepath.Join("festivals", "active", "my-fest")

	if got != want {
		t.Errorf("ToRelativePath(%q, %q) = %q, want %q", absPath, tmpDir, got, want)
	}
}
