package shared

import (
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/resident"
	"github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// ResolveFestivalPath resolves the festival path from multiple sources in priority order:
//  1. Explicit path (--festival flag, highest priority)
//  2. FindFestivalRoot from current directory (standing inside a festival)
//  3. Navigation link for current directory (fest link)
//
// Standing inside a festival must outrank a project link. FindFestivalForPath
// walks up the directory tree, so a festival linked to a directory that
// contains other festivals claims every path beneath it. The campaign root is
// the case that bites: a sequence with fest_working_dir "." links its festival
// to the root, and the root contains festivals/, so every other festival in the
// campaign resolves to the linked one. That sent `fest next` into a different
// festival than the one the caller was standing in.
//
// Links still work from anywhere they are supposed to: a linked project or
// worktree is not itself a festival, so FindFestivalRoot fails there and
// resolution falls through to the link.
func ResolveFestivalPath(cwd, explicitPath string) (string, error) {
	// 1. Explicit flag takes precedence
	if explicitPath != "" {
		return explicitPath, nil
	}

	// 2. Standing inside a festival wins over any link covering this path.
	festivalPath, rootErr := template.FindFestivalRoot(cwd)
	if rootErr == nil && festivalPath != "" {
		return festivalPath, nil
	}

	// 3. Check for linked festival (running from a linked project or worktree)
	nav, err := navigation.LoadNavigation()
	if err == nil {
		if linkedFestivalName := nav.FindFestivalForPath(cwd); linkedFestivalName != "" {
			// Found a linked festival name, now search for its actual path
			festivalsRoot, err := workspace.FindFestivals(cwd)
			if err == nil && festivalsRoot != "" {
				linkedPath, err := findFestivalByName(festivalsRoot, linkedFestivalName)
				if err == nil {
					return linkedPath, nil
				}
			}
		}
	}

	// 4. No festival at cwd and no usable link: report the original detection error.
	return festivalPath, rootErr
}

// findFestivalByName searches for a festival by name in all status directories
func findFestivalByName(festivalsRoot, name string) (string, error) {
	for _, status := range id.StatusDirectories {
		statusDir := workspace.JoinStatus(festivalsRoot, status)
		festivalPath := filepath.Join(statusDir, name)

		// Check if this festival directory exists
		if info, err := os.Stat(festivalPath); err == nil && info.IsDir() {
			// Verify it's a valid festival by checking for festival markers
			if isValidFestival(festivalPath) {
				return festivalPath, nil
			}
		}
	}

	// Festival name not found in any status directory
	return template.FindFestivalRoot(festivalsRoot)
}

// isValidFestival checks if a directory is a valid festival root
func isValidFestival(dir string) bool {
	return resident.IsFestivalDir(dir)
}

// ResolvePhasePath detects the current phase directory from the given path.
// It walks up from cwd looking for a PHASE_GOAL.md file, stopping at the festival root.
// Returns the phase path if found, empty string if not within a phase.
func ResolvePhasePath(cwd, festivalPath string) string {
	// Start from cwd and walk up to festival root
	dir := cwd
	for {
		// Stop at festival root
		if dir == festivalPath || dir == "" || dir == "/" || dir == "." {
			return ""
		}

		// Check for PHASE_GOAL.md
		phaseGoalPath := filepath.Join(dir, "PHASE_GOAL.md")
		if _, err := os.Stat(phaseGoalPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
