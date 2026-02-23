// Package pathutil provides centralized path conversion utilities for
// converting between absolute filesystem paths and campaign-relative paths.
//
// Campaign-relative paths are used for persistence (YAML, JSON output)
// while absolute paths are used for in-memory operations. The campaign
// root is always passed in by callers to avoid repeated directory walks.
package pathutil

import (
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
