package github

import (
	"strings"
	"testing"
)

// TestParseSemver verifies stable and dev tag patterns are parsed correctly.
func TestParseSemver(t *testing.T) {
	tests := []struct {
		tag       string
		wantOK    bool
		wantMajor int
		wantMinor int
		wantPatch int
		wantPre   string
	}{
		{tag: "v1.2.3", wantOK: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{tag: "v0.0.1", wantOK: true, wantMajor: 0, wantMinor: 0, wantPatch: 1},
		{tag: "v10.20.30", wantOK: true, wantMajor: 10, wantMinor: 20, wantPatch: 30},
		{tag: "v1.2.3-dev.4", wantOK: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "dev.4"},
		{tag: "v0.1.0-dev.10", wantOK: true, wantMajor: 0, wantMinor: 1, wantPatch: 0, wantPre: "dev.10"},
		// Invalid — should not parse
		{tag: "1.2.3"},
		{tag: "v1.2"},
		{tag: "v1.2.3-alpha.1"},
		{tag: "v1.2.3-rc.1"},
		{tag: ""},
	}

	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			sv, ok := parseSemver(tc.tag)
			if ok != tc.wantOK {
				t.Fatalf("parseSemver(%q) ok=%v, want %v", tc.tag, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if sv.major != tc.wantMajor || sv.minor != tc.wantMinor || sv.patch != tc.wantPatch {
				t.Errorf("parseSemver(%q) = {%d,%d,%d}, want {%d,%d,%d}",
					tc.tag, sv.major, sv.minor, sv.patch,
					tc.wantMajor, tc.wantMinor, tc.wantPatch)
			}
			if sv.pre != tc.wantPre {
				t.Errorf("parseSemver(%q).pre = %q, want %q", tc.tag, sv.pre, tc.wantPre)
			}
		})
	}
}

// TestSemverGreater verifies version comparison logic.
func TestSemverGreater(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v2.0.0", "v1.9.9", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.2.4", "v1.2.3", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v2.0.0", false},
		{"v1.2.3-dev.5", "v1.2.3-dev.4", true},
		{"v1.2.3-dev.10", "v1.2.3-dev.9", true},
		{"v1.2.3-dev.3", "v1.2.3-dev.10", false},
		// A release outranks any pre-release of the same base version.
		{"v1.2.3", "v1.2.3-dev.5", true},
		{"v1.2.3-dev.5", "v1.2.3", false},
		// A higher base beats a lower-base pre-release (and vice versa).
		{"v1.0.1", "v1.0.0-dev.10", true},
		{"v0.4.3", "v0.1.0-dev.2", true},
		{"v0.5.0-dev.1", "v0.4.3", true},
	}

	for _, tc := range tests {
		t.Run(tc.a+"_gt_"+tc.b, func(t *testing.T) {
			a, aok := parseSemver(tc.a)
			b, bok := parseSemver(tc.b)
			if !aok || !bok {
				t.Fatalf("failed to parse test inputs: %q, %q", tc.a, tc.b)
			}
			got := semverGreater(a, b)
			if got != tc.want {
				t.Errorf("semverGreater(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// buildLsRemoteOutput builds a fake git ls-remote output string from a list of tags.
func buildLsRemoteOutput(tags []string) string {
	var sb strings.Builder
	for _, tag := range tags {
		sb.WriteString("aabbccddee\trefs/tags/")
		sb.WriteString(tag)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// TestSelectLatestTag tests channel filtering and selection with mock tags.
func TestSelectLatestTag(t *testing.T) {
	allTags := []string{
		"v0.1.0",
		"v0.1.0-dev.1",
		"v0.1.0-dev.2",
		"v0.2.0",
		"v0.2.0-dev.1",
		"v1.0.0",
		"v1.0.0-dev.1",
		"v1.0.0-dev.10",
		"v1.0.0-dev.9",
		"v1.0.1",
		// Never selected by either channel:
		"v1.0.0-rc.1",
		"v1.0.0-alpha.2",
		"latest",
	}

	t.Run("stable channel", func(t *testing.T) {
		got, err := selectLatestTag(allTags, "stable")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.0.1" {
			t.Errorf("stable: got %q, want %q", got, "v1.0.1")
		}
	})

	t.Run("dev channel includes stable releases", func(t *testing.T) {
		got, err := selectLatestTag(allTags, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// v1.0.1 (stable) > v1.0.0-dev.10, so dev resolves to the stable release.
		if got != "v1.0.1" {
			t.Errorf("dev: got %q, want %q", got, "v1.0.1")
		}
	})

	t.Run("unknown channel", func(t *testing.T) {
		if _, err := selectLatestTag(allTags, "nightly"); err == nil {
			t.Fatal("expected error for unknown channel")
		}
	})
}

// TestSelectLatestTag_DevNeverBehindStable verifies the dev channel resolves to
// the newest tag overall: it falls back to stable when dev tags are stale, and
// a newer dev pre-release leads when one exists.
func TestSelectLatestTag_DevNeverBehindStable(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "falls back to stable when dev tags are stale",
			tags: []string{"v0.1.0-dev.1", "v0.1.0-dev.2", "v0.2.0", "v0.3.1", "v0.4.3"},
			want: "v0.4.3",
		},
		{
			name: "newer dev pre-release leads stable",
			tags: []string{"v0.4.3", "v0.5.0-dev.1"},
			want: "v0.5.0-dev.1",
		},
		{
			name: "release outranks its own pre-releases",
			tags: []string{"v0.5.0-dev.1", "v0.5.0-dev.3", "v0.5.0"},
			want: "v0.5.0",
		},
		{
			name: "dev tags only still resolves to the highest dev",
			tags: []string{"v0.1.0-dev.1", "v0.1.0-dev.2"},
			want: "v0.1.0-dev.2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectLatestTag(tc.tags, "dev")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSelectLatestTag_NoMatches verifies a clear error when no tags match.
func TestSelectLatestTag_NoMatches(t *testing.T) {
	if _, err := selectLatestTag([]string{"v1.0.0-dev.1", "v1.0.0-dev.2"}, "stable"); err == nil {
		t.Fatal("expected error when no stable tags exist")
	}
	if _, err := selectLatestTag(nil, "dev"); err == nil {
		t.Fatal("expected error when no tags exist")
	}
}

// TestParseLsRemoteTags verifies tag-name extraction and annotated-pointer filtering.
func TestParseLsRemoteTags(t *testing.T) {
	output := buildLsRemoteOutput([]string{"v1.0.0", "v1.0.1"})
	// Annotated tag pointer entries (ending in ^{}) must be skipped.
	output += "deadbeef\trefs/tags/v1.0.1^{}\n"

	got := parseLsRemoteTags(output)
	want := []string{"v1.0.0", "v1.0.1"}
	if len(got) != len(want) {
		t.Fatalf("parseLsRemoteTags returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if tags := parseLsRemoteTags(""); len(tags) != 0 {
		t.Errorf("parseLsRemoteTags(\"\") = %v, want empty", tags)
	}
	if tags := parseLsRemoteTags("abc123\trefs/tags/v2.0.0^{}\n"); len(tags) != 0 {
		t.Errorf("parseLsRemoteTags with only annotated pointers = %v, want empty", tags)
	}
}
