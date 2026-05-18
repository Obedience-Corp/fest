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
	// For pre-release comparison (dev channel), compare numerically by pre suffix.
	// e.g. "dev.10" > "dev.9"
	aNum := preReleaseNum(a.pre)
	bNum := preReleaseNum(b.pre)
	return aNum > bNum
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
// highest semver tag matching the given channel.
//
// Supported channels:
//   - "stable": tags matching `^v\d+\.\d+\.\d+$`
//   - "dev":    tags matching `^v\d+\.\d+\.\d+-dev\.\d+$`
func ResolveLatestTag(repoURL, channel string) (string, error) {
	if !IsGitAvailable() {
		return "", errors.Validation("git command not found")
	}

	cmd := exec.Command("git", "ls-remote", "--tags", repoURL)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.IO("listing remote tags", err).WithField("url", repoURL)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	var candidates []semver
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Each line is "<sha>\trefs/tags/<name>"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		ref := parts[1]

		// Skip annotated tag pointer entries (end with ^{})
		if strings.HasSuffix(ref, "^{}") {
			continue
		}

		tagName := strings.TrimPrefix(ref, "refs/tags/")

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
			return "", errors.Validation("unknown channel: " + channel)
		}

		sv, ok := parseSemver(tagName)
		if !ok {
			continue
		}
		candidates = append(candidates, sv)
	}

	if len(candidates) == 0 {
		return "", errors.NotFound("tags matching channel "+channel).WithField("url", repoURL)
	}

	// Sort descending and return the first element.
	sort.Slice(candidates, func(i, j int) bool {
		return semverGreater(candidates[i], candidates[j])
	})

	return candidates[0].raw, nil
}
