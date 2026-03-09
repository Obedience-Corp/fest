package github

import (
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

// mockLsRemote builds a fake git ls-remote output string from a list of tags.
func buildLsRemoteOutput(tags []string) string {
	var sb string
	for _, tag := range tags {
		sb += "aabbccddee\trefs/tags/" + tag + "\n"
	}
	return sb
}

// parseTagsFromOutput mirrors the filtering/parsing logic in ResolveLatestTag so
// we can test it without executing real git commands.
func resolveFromOutput(output, channel string) (string, error) {
	// Inline the filtering logic identical to ResolveLatestTag
	lines := splitLines(output)
	var candidates []semver
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := splitFields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		if hasSuffix(ref, "^{}") {
			continue
		}
		tagName := trimPrefix(ref, "refs/tags/")

		switch channel {
		case "stable":
			if !stableTagRe.MatchString(tagName) {
				continue
			}
		case "dev":
			if !devTagRe.MatchString(tagName) {
				continue
			}
		default:
			return "", &parseError{msg: "unknown channel: " + channel}
		}

		sv, ok := parseSemver(tagName)
		if !ok {
			continue
		}
		candidates = append(candidates, sv)
	}

	if len(candidates) == 0 {
		return "", &notFoundError{channel: channel}
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if semverGreater(c, best) {
			best = c
		}
	}
	return best.raw, nil
}

// Small helpers to keep resolveFromOutput free of imports.

func splitLines(s string) []string {
	return splitOn(s, '\n')
}

func splitOn(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func splitFields(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

type notFoundError struct{ channel string }

func (e *notFoundError) Error() string { return "no tags found for channel " + e.channel }

// TestResolveLatestTagFiltering tests the filtering logic with mock data.
func TestResolveLatestTagFiltering(t *testing.T) {
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
		// Should be ignored by both channels:
		"v1.0.0-rc.1",
		"v1.0.0-alpha.2",
		"latest",
	}

	output := buildLsRemoteOutput(allTags)
	// Also add an annotated tag pointer that should be filtered out.
	output += "deadbeef\trefs/tags/v1.0.1^{}\n"

	t.Run("stable channel", func(t *testing.T) {
		got, err := resolveFromOutput(output, "stable")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.0.1" {
			t.Errorf("stable: got %q, want %q", got, "v1.0.1")
		}
	})

	t.Run("dev channel", func(t *testing.T) {
		got, err := resolveFromOutput(output, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.0.0-dev.10" {
			t.Errorf("dev: got %q, want %q", got, "v1.0.0-dev.10")
		}
	})

	t.Run("unknown channel", func(t *testing.T) {
		_, err := resolveFromOutput(output, "nightly")
		if err == nil {
			t.Fatal("expected error for unknown channel")
		}
	})
}

// TestResolveLatestTagNoMatches tests that a clear error is returned when no
// tags match the requested channel.
func TestResolveLatestTagNoMatches(t *testing.T) {
	output := buildLsRemoteOutput([]string{
		"v1.0.0-dev.1",
		"v1.0.0-dev.2",
	})

	_, err := resolveFromOutput(output, "stable")
	if err == nil {
		t.Fatal("expected error when no stable tags exist")
	}
}

// TestResolveLatestTagAnnotatedTagsFiltered verifies that ^{} entries are skipped.
func TestResolveLatestTagAnnotatedTagsFiltered(t *testing.T) {
	// Only annotated pointer entry — no plain ref should result in no match.
	output := "abc123\trefs/tags/v2.0.0^{}\n"

	_, err := resolveFromOutput(output, "stable")
	if err == nil {
		t.Fatal("expected not-found error when only annotated pointers are present")
	}
}

// TestResolveLatestTagEmptyOutput verifies that empty output returns an error.
func TestResolveLatestTagEmptyOutput(t *testing.T) {
	_, err := resolveFromOutput("", "stable")
	if err == nil {
		t.Fatal("expected error for empty output")
	}
}
