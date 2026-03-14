// Package pathutil provides centralized path conversion utilities for
// converting between absolute filesystem paths and campaign-relative paths.
//
// Campaign-relative paths are used for persistence (YAML, JSON output)
// while absolute paths are used for in-memory operations. The campaign
// root is always passed in by callers to avoid repeated directory walks.
package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToRelativePath converts an absolute path to campaign-relative if possible.
// Returns the original path if it's outside the campaign, already relative,
// or if campaignRoot is empty.
func ToRelativePath(absPath, campaignRoot string) string {
	if campaignRoot == "" || !filepath.IsAbs(absPath) {
		return absPath
	}
	rel, err := filepath.Rel(campaignRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}

// ToAbsolutePath converts a campaign-relative path to absolute.
// Returns the original path if it's already absolute or if campaignRoot is empty.
func ToAbsolutePath(path, campaignRoot string) string {
	if campaignRoot == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(campaignRoot, path)
}

// DisplayPath converts an absolute path to campaign-relative for display.
// Alias for ToRelativePath, signaling display intent.
func DisplayPath(absPath, campaignRoot string) string {
	return ToRelativePath(absPath, campaignRoot)
}

// NormalizeWorkingDir sanitizes a fest_working_dir value.
// It strips whitespace and trailing slashes, cleans the path, and rejects
// absolute paths, home-relative paths (~), and parent traversal (..).
// Returns empty string without error for empty input.
func NormalizeWorkingDir(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	if strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("fest_working_dir must be relative to campaign root, got absolute path %q", raw)
	}

	if strings.HasPrefix(s, "~") {
		return "", fmt.Errorf("fest_working_dir must be relative to campaign root, got home-relative path %q", raw)
	}

	s = strings.TrimRight(s, "/")
	if s == "" {
		return "", nil
	}

	s = filepath.Clean(s)

	if s == ".." || strings.HasPrefix(s, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fest_working_dir contains \"..\" — paths must stay within campaign root")
	}

	return s, nil
}

// ResolveProjectPathValue resolves a legacy fest.yaml project_path for use as a
// working-directory fallback. Relative paths are normalized relative to the
// campaign root; absolute paths are preserved for compatibility with existing
// festivals and converted back to campaign-relative when possible.
func ResolveProjectPathValue(raw, campaignRoot string) (relative string, absolute string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", nil
	}

	if strings.HasPrefix(s, "~/") || s == "~" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", "", homeErr
		}
		if s == "~" {
			s = home
		} else {
			s = filepath.Join(home, s[2:])
		}
	}

	if filepath.IsAbs(s) {
		absolute = filepath.Clean(s)
		if campaignRoot != "" {
			if rel := ToRelativePath(absolute, campaignRoot); rel != absolute {
				relative = rel
			}
		}
		return relative, absolute, nil
	}

	relative, err = NormalizeWorkingDir(s)
	if err != nil {
		return "", "", err
	}
	if relative == "" {
		return "", "", nil
	}
	if campaignRoot != "" {
		absolute = filepath.Join(campaignRoot, relative)
	}
	return relative, absolute, nil
}
