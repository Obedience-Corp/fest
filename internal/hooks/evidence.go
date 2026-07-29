package hooks

import (
	"os"
	"path/filepath"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

// EvidenceEmbedCapBytes is the total budget for evidence: embed mode (256KB).
const EvidenceEmbedCapBytes = 256 * 1024

// EvidenceFile is an optional embedded evidence payload entry (additive).
type EvidenceFile struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// NormalizeEvidencePath cleans a phase-relative evidence path and rejects
// absolute or escaping paths (read-by-approver contract).
func NormalizeEvidencePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", festerrors.Validation("evidence path is empty")
	}
	if filepath.IsAbs(path) {
		return "", festerrors.Validation("absolute evidence paths are not allowed")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", festerrors.Validation("evidence path escapes the phase directory")
	}
	return clean, nil
}

// WithinRoot reports whether a phase-relative path resolves under phasePath
// after symlink evaluation and is a non-empty regular file.
func WithinRoot(phasePath, relativePath string) (bool, error) {
	phaseRoot, err := filepath.Abs(phasePath)
	if err != nil {
		return false, festerrors.Wrap(err, "resolving phase root")
	}
	resolvedRoot, err := filepath.EvalSymlinks(phaseRoot)
	if err != nil {
		return false, festerrors.Wrap(err, "resolving phase root symlinks")
	}

	candidate := filepath.Join(phaseRoot, relativePath)
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, festerrors.Wrap(err, "resolving evidence symlinks")
	}
	containedPath, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return false, festerrors.Wrap(err, "checking resolved evidence containment")
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return false, festerrors.Validation("resolved evidence path escapes the phase directory")
	}

	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular() && info.Size() > 0, nil
}

// ResolvePhaseRelative returns the subset of relative paths that exist as
// non-empty regular files under phasePath, preserving order and dropping
// duplicates, absolute/escaping paths, and unreadable/missing files.
func ResolvePhaseRelative(phasePath string, paths []string) []string {
	seen := map[string]struct{}{}
	var present []string
	for _, p := range paths {
		rel, err := NormalizeEvidencePath(p)
		if err != nil {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		ok, err := WithinRoot(phasePath, rel)
		if err != nil || !ok {
			continue
		}
		seen[rel] = struct{}{}
		present = append(present, rel)
	}
	return present
}

// BuildEvidenceFiles packs file contents under a total byte budget.
// Files that fail the within-root contract or cannot be read are dropped.
// When the budget is exhausted mid-file, content is truncated with a marker.
// Further files after budget exhaustion are omitted (paths remain on evidence list).
func BuildEvidenceFiles(phasePath string, relPaths []string, capBytes int) ([]EvidenceFile, error) {
	if capBytes <= 0 {
		capBytes = EvidenceEmbedCapBytes
	}
	var files []EvidenceFile
	remaining := capBytes
	const truncMarker = "\n[TRUNCATED: evidence exceeded the 256KB embed budget]"
	for _, rel := range relPaths {
		if remaining <= 0 {
			break
		}
		ok, err := WithinRoot(phasePath, rel)
		if err != nil || !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(phasePath, rel))
		if err != nil {
			continue
		}
		ef := EvidenceFile{Path: rel}
		if len(data) > remaining {
			chunk := data[:remaining]
			ef.Truncated = true
			ef.Content = string(chunk) + truncMarker
			remaining = 0
		} else {
			ef.Content = string(data)
			remaining -= len(data)
		}
		files = append(files, ef)
	}
	return files, nil
}
