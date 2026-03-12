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
		{
			name:         "empty campaignRoot returns path unchanged",
			absPath:      "/some/absolute/path",
			campaignRoot: "",
			want:         "/some/absolute/path",
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
		{
			name:         "empty campaignRoot returns path unchanged",
			path:         "relative/path",
			campaignRoot: "",
			want:         "relative/path",
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

func TestNormalizeWorkingDir(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Valid cases
		{name: "simple relative path", input: "projects/fest", want: "projects/fest"},
		{name: "nested path", input: "projects/obey-platform-monorepo/obey", want: "projects/obey-platform-monorepo/obey"},
		{name: "single dir", input: "projects", want: "projects"},
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "trailing slash stripped", input: "projects/fest/", want: "projects/fest"},
		{name: "leading whitespace stripped", input: "  projects/fest", want: "projects/fest"},
		{name: "trailing whitespace stripped", input: "projects/fest  ", want: "projects/fest"},
		{name: "dot component cleaned", input: "projects/./fest", want: "projects/fest"},
		{name: "double slash cleaned", input: "projects//fest", want: "projects/fest"},

		// Invalid cases
		{name: "absolute path", input: "/Users/lance/projects/fest", wantErr: true},
		{name: "home relative", input: "~/projects/fest", wantErr: true},
		{name: "parent traversal start", input: "../other-repo", wantErr: true},
		{name: "parent traversal middle", input: "projects/../../../etc", wantErr: true},
		{name: "just dotdot", input: "..", wantErr: true},
		{name: "tilde alone", input: "~", wantErr: true},
		{name: "slash alone", input: "/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWorkingDir(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizeWorkingDir(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("NormalizeWorkingDir(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeWorkingDir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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
