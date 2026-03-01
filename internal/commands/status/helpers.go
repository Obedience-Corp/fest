package status

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/workflow"
)

// dateDirPattern matches YYYY-MM-DD or YYYY-MM formatted date directory names.
var dateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}(-\d{2})?$`)

// LooksLikeDateDir checks if a directory name matches a date directory pattern (YYYY-MM-DD or YYYY-MM).
func LooksLikeDateDir(name string) bool {
	return dateDirPattern.MatchString(name)
}

// isKnownDungeonStatus checks if a name is a known dungeon substatus.
func isKnownDungeonStatus(name string) bool {
	switch name {
	case "completed", "archived", "someday":
		return true
	default:
		return false
	}
}

// resolveFestivalFromPath resolves a festival name or path from anywhere in the workspace.
// It searches: cwd (if path is relative), festivals/active/, festivals/planning/,
// festivals/completed/, and festivals/dungeon/.
// Returns the absolute path to the festival directory if found.
func resolveFestivalFromPath(cwd, pathArg string) (string, error) {
	// 1. Check if pathArg is an absolute path
	if filepath.IsAbs(pathArg) {
		if isValidFestivalDir(pathArg) {
			return pathArg, nil
		}
		return "", errors.NotFound("festival").WithField("path", pathArg)
	}

	// 2. Check if pathArg is relative to cwd
	relPath := filepath.Join(cwd, pathArg)
	if isValidFestivalDir(relPath) {
		return relPath, nil
	}

	// 3. Find festivals root and search all status directories
	festivalsRoot := findFestivalsRoot(cwd)
	if festivalsRoot == "" {
		return "", errors.NotFound("festivals directory").
			WithField("hint", "navigate to a workspace with festivals/ directory")
	}

	// Search in all status directories
	for _, status := range id.StatusDirectories {
		// Try direct path: festivals/<status>/<pathArg>
		candidatePath := filepath.Join(festivalsRoot, status, pathArg)
		if isValidFestivalDir(candidatePath) {
			return candidatePath, nil
		}

		// For dungeon statuses, also search inside date subdirectories
		if strings.HasPrefix(status, "dungeon/") {
			statusDir := filepath.Join(festivalsRoot, status)
			entries, err := os.ReadDir(statusDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() && LooksLikeDateDir(entry.Name()) {
					dateDirPath := filepath.Join(statusDir, entry.Name(), pathArg)
					if isValidFestivalDir(dateDirPath) {
						return dateDirPath, nil
					}
				}
			}
		}
	}

	// 4. Check if pathArg includes status prefix (e.g., "active/my-festival" or "dungeon/completed/my-festival")
	candidatePath := filepath.Join(festivalsRoot, pathArg)
	if isValidFestivalDir(candidatePath) {
		return candidatePath, nil
	}

	return "", errors.NotFound("festival").
		WithField("name", pathArg).
		WithField("hint", "festival not found in active, planning, completed, or dungeon/*")
}

// isValidFestivalDir checks if a directory is a valid festival root.
func isValidFestivalDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for festival markers: FESTIVAL_GOAL.md, FESTIVAL_OVERVIEW.md, or fest.yaml
	markers := []string{"FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md", "fest.yaml"}
	for _, marker := range markers {
		markerPath := filepath.Join(dir, marker)
		if _, err := os.Stat(markerPath); err == nil {
			return true
		}
	}
	return false
}

// detectEntityType determines what type of entity a path points to.
// Returns EntityFestival, EntityPhase, EntitySequence, or EntityTask.
func detectEntityType(path string) EntityType {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	// If it's a file, it's a task (markdown file)
	if !info.IsDir() {
		return EntityTask
	}

	// Check for festival markers
	if isValidFestivalDir(path) {
		return EntityFestival
	}

	// Check for phase marker (PHASE_GOAL.md)
	if _, err := os.Stat(filepath.Join(path, "PHASE_GOAL.md")); err == nil {
		return EntityPhase
	}

	// Check for sequence marker (SEQUENCE_GOAL.md)
	if _, err := os.Stat(filepath.Join(path, "SEQUENCE_GOAL.md")); err == nil {
		return EntitySequence
	}

	// Default to unknown (could be a regular directory)
	return ""
}

// resolveStatusPath resolves the target path for status commands.
// If pathArg is empty, uses current working directory.
// If pathArg is relative to a festivals/ root (e.g., "active/my-festival"),
// it resolves from the festivals root.
func resolveStatusPath(pathArg string) (string, error) {
	if pathArg == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", errors.IO("getting current directory", err)
		}
		return cwd, nil
	}

	// Try as absolute or relative path first
	absPath, err := filepath.Abs(pathArg)
	if err != nil {
		return "", errors.Wrap(err, "resolving path").WithField("path", pathArg)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}

	// Try resolving relative to festivals/ root
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.IO("getting current directory", err)
	}

	// Find festivals root and try pathArg relative to it
	festivalsRoot := findFestivalsRoot(cwd)
	if festivalsRoot != "" {
		candidatePath := filepath.Join(festivalsRoot, pathArg)
		if _, err := os.Stat(candidatePath); err == nil {
			return candidatePath, nil
		}
	}

	return "", errors.NotFound("path").WithField("path", pathArg)
}

// findFestivalsRoot walks up from startPath looking for a festivals/ directory.
func findFestivalsRoot(startPath string) string {
	current := startPath
	for {
		// Check if current is festivals/ or contains festivals/
		if filepath.Base(current) == "festivals" {
			return current
		}
		festivalsDir := filepath.Join(current, "festivals")
		if info, err := os.Stat(festivalsDir); err == nil && info.IsDir() {
			return festivalsDir
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

// isValidStatus checks if status is valid for the given entity type.
func isValidStatus(entityType EntityType, status string) bool {
	validStatuses, ok := ValidStatuses[entityType]
	if !ok {
		return false
	}
	for _, valid := range validStatuses {
		if valid == status {
			return true
		}
	}
	return false
}

// isValidFestivalStatus checks if a status is valid for festivals using the
// internal FestivalSchema. This avoids reading .workflow.yaml from disk, which
// is a camp-side interop file that doesn't understand user-facing aliases.
func isValidFestivalStatus(status string) bool {
	schema := workflow.FestivalSchema()
	if schema.HasDirectory(status) {
		return true
	}
	// Check if it's a known alias (e.g., "completed" → "dungeon/completed")
	resolved := id.ResolveStatusPath(status)
	return resolved != status && schema.HasDirectory(resolved)
}

// getValidFestivalStatuses returns valid festival statuses from the internal schema.
func getValidFestivalStatuses() []string {
	return workflow.FestivalSchema().AllDirectories()
}

// festivalsRootFromPath derives the festivals root directory from a festival's path and status.
// For simple statuses (e.g., "active"), the path is festivals/<status>/<name>.
// For nested statuses (e.g., "dungeon/completed"), the path is festivals/dungeon/completed/<name>.
// For date-organized dungeon statuses, the path is festivals/dungeon/completed/YYYY-MM-DD/<name>.
func festivalsRootFromPath(festivalPath, status string) string {
	root := festivalPath
	// Strip festival name
	root = filepath.Dir(root)
	// If parent is a date directory, strip it too
	if LooksLikeDateDir(filepath.Base(root)) {
		root = filepath.Dir(root)
	}
	// Strip each status path component
	for range strings.Split(status, "/") {
		root = filepath.Dir(root)
	}
	return root
}

// hasNumericPrefix checks if a directory name starts with digits.
func hasNumericPrefix(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= '0' && name[0] <= '9'
}

// filterPhasesByStatus filters phases to only those matching the given status.
// If status is empty, returns all phases.
func filterPhasesByStatus(phases []*PhaseInfo, status string) []*PhaseInfo {
	if status == "" {
		return phases
	}

	var filtered []*PhaseInfo
	for _, phase := range phases {
		if phase.Status == status {
			filtered = append(filtered, phase)
		}
	}
	return filtered
}

// filterSequencesByStatus filters sequences to only those matching the given status.
// If status is empty, returns all sequences.
func filterSequencesByStatus(sequences []*SequenceInfo, status string) []*SequenceInfo {
	if status == "" {
		return sequences
	}

	var filtered []*SequenceInfo
	for _, seq := range sequences {
		if seq.Status == status {
			filtered = append(filtered, seq)
		}
	}
	return filtered
}

// filterTasksByStatus filters tasks to only those matching the given status.
// If status is empty, returns all tasks.
func filterTasksByStatus(tasks []*TaskInfo, status string) []*TaskInfo {
	if status == "" {
		return tasks
	}

	var filtered []*TaskInfo
	for _, task := range tasks {
		if task.Status == status {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// detectEntityTypeForStatusPrompt determines the entity type for status prompting.
// It checks level-specific flags first, then falls back to detecting from cwd.
func detectEntityTypeForStatusPrompt(cwd string, opts *statusOptions) EntityType {
	// Check if a level-specific flag was provided
	if opts.task != "" {
		return EntityTask
	}
	if opts.sequence != "" {
		return EntitySequence
	}
	if opts.phase != "" {
		return EntityPhase
	}
	if opts.path != "" {
		return detectEntityType(opts.path)
	}

	// Try to detect from current working directory
	entityType := detectEntityType(cwd)
	if entityType != "" {
		return entityType
	}

	// Default to festival
	return EntityFestival
}
