package github

import (
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// semver holds parsed major/minor/patch components for sorting.
type semver struct {
	major, minor, patch int
	pre                 string // pre-release suffix (e.g. "dev.3")
	raw                 string // original tag name
}

var (
	stableTagRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
	devTagRe    = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)-(dev\.\d+)$`)
)

// parseSemver parses a tag string into semver components.
// Returns (semver, true) on success, zero value and false otherwise.
func parseSemver(tag string) (semver, bool) {
	if m := stableTagRe.FindStringSubmatch(tag); m != nil {
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		patch, _ := strconv.Atoi(m[3])
		return semver{major: major, minor: minor, patch: patch, raw: tag}, true
	}
	if m := devTagRe.FindStringSubmatch(tag); m != nil {
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		patch, _ := strconv.Atoi(m[3])
		return semver{major: major, minor: minor, patch: patch, pre: m[4], raw: tag}, true
	}
	return semver{}, false
}

// semverGreater returns true if a > b.
func semverGreater(a, b semver) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	if a.patch != b.patch {
		return a.patch > b.patch
	}
	// Same base version: a release (no pre-release) outranks any pre-release of
	// that version, e.g. v1.0.0 > v1.0.0-dev.5.
	if (a.pre == "") != (b.pre == "") {
		return a.pre == ""
	}
	// Two pre-releases of the same base: compare numerically by suffix, e.g.
	// "dev.10" > "dev.9".
	return preReleaseNum(a.pre) > preReleaseNum(b.pre)
}

// preReleaseNum extracts the numeric part from a pre-release string like "dev.3".
func preReleaseNum(pre string) int {
	parts := strings.SplitN(pre, ".", 2)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}

// ResolveLatestTag queries the remote repository for tags and returns the
// highest-precedence tag for the given channel.
//
// Supported channels:
//   - "stable": tags matching `^v\d+\.\d+\.\d+$`
//   - "dev":    dev pre-release tags (`^v\d+\.\d+\.\d+-dev\.\d+$`) AND stable
//     releases, so the dev channel never resolves to a tag older than the
//     latest stable. A newer dev pre-release still wins when one exists.
func ResolveLatestTag(repoURL, channel string) (string, error) {
	if !IsGitAvailable() {
		return "", errors.Validation("git command not found")
	}

	cmd := exec.Command("git", "ls-remote", "--tags", repoURL)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Network("listing remote tags", err, gitStderr(err)).WithField("url", repoURL)
	}

	tag, err := selectLatestTag(parseLsRemoteTags(string(output)), channel)
	if err != nil {
		return "", err
	}
	return tag, nil
}

// parseLsRemoteTags extracts plain tag names from `git ls-remote --tags` output,
// skipping annotated tag pointer entries (refs ending in "^{}").
func parseLsRemoteTags(output string) []string {
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Each line is "<sha>\trefs/tags/<name>".
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
	}
	return tags
}

// selectLatestTag returns the highest-precedence tag for the channel from the
// given tag names. See ResolveLatestTag for channel semantics.
func selectLatestTag(tags []string, channel string) (string, error) {
	matchesChannel := func(tag string) bool {
		switch channel {
		case "stable":
			return stableTagRe.MatchString(tag)
		case "dev":
			return devTagRe.MatchString(tag) || stableTagRe.MatchString(tag)
		default:
			return false
		}
	}

	switch channel {
	case "stable", "dev":
	default:
		return "", errors.Validation("unknown channel: " + channel)
	}

	var candidates []semver
	for _, tag := range tags {
		if !matchesChannel(tag) {
			continue
		}
		if sv, ok := parseSemver(tag); ok {
			candidates = append(candidates, sv)
		}
	}

	if len(candidates) == 0 {
		return "", errors.NotFound("tags matching channel " + channel)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return semverGreater(candidates[i], candidates[j])
	})
	return candidates[0].raw, nil
}
