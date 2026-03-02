package show

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// dateDirPattern matches YYYY-MM-DD or YYYY-MM formatted date directory names.
var dateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}(-\d{2})?$`)

// looksLikeDateDir checks if a directory name matches a date directory pattern.
func looksLikeDateDir(name string) bool {
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

const (
	// FestivalGoalFile is the primary festival marker file
	FestivalGoalFile = "FESTIVAL_GOAL.md"
	// FestivalOverviewFile is an alternative festival marker file
	FestivalOverviewFile = "FESTIVAL_OVERVIEW.md"
	// FestivalConfigFile is the festival configuration file
	FestivalConfigFile = "fest.yaml"
	// PhaseGoalFile marks a phase directory
	PhaseGoalFile = "PHASE_GOAL.md"
	// SequenceGoalFile marks a sequence directory
	SequenceGoalFile = "SEQUENCE_GOAL.md"
)

// DetectCurrentFestival walks up from the given directory to find a festival root.
// If no festival markers are found, checks navigation links for linked project directories.
// Returns the festival information if found, or an error if not in a festival.
func DetectCurrentFestival(ctx context.Context, startDir, campaignRoot string) (*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, errors.IO("getting absolute path", err)
	}

	// Step 1: Walk up from startDir looking for festival markers
	dir := absStart
	for {
		if isValidFestival(dir) {
			return parseFestivalInfo(ctx, dir, campaignRoot)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Step 2: Check navigation links (supports linked project directories)
	nav, navErr := navigation.LoadNavigation()
	if navErr == nil {
		if linkedName := nav.FindFestivalForPath(absStart); linkedName != "" {
			festivalsRoot, findErr := workspace.FindFestivals(absStart)
			if findErr == nil && festivalsRoot != "" {
				if festivalPath := findLinkedFestivalPath(festivalsRoot, linkedName); festivalPath != "" {
					return parseFestivalInfo(ctx, festivalPath, campaignRoot)
				}
			}
		}
	}

	return nil, errors.NotFound("festival").WithHint(errors.HintFestivalNotFound)
}

// findLinkedFestivalPath searches for a festival by name in all status directories.
// For dungeon statuses, also searches inside date subdirectories.
func findLinkedFestivalPath(festivalsRoot, name string) string {
	for _, status := range id.StatusDirectories {
		// Try direct path
		festivalPath := filepath.Join(festivalsRoot, status, name)
		if info, err := os.Stat(festivalPath); err == nil && info.IsDir() {
			if isValidFestival(festivalPath) {
				return festivalPath
			}
		}

		// For dungeon statuses, search inside date subdirectories
		if strings.HasPrefix(status, "dungeon/") {
			statusDir := filepath.Join(festivalsRoot, status)
			entries, err := os.ReadDir(statusDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() && looksLikeDateDir(entry.Name()) {
					datePath := filepath.Join(statusDir, entry.Name(), name)
					if info, err := os.Stat(datePath); err == nil && info.IsDir() {
						if isValidFestival(datePath) {
							return datePath
						}
					}
				}
			}
		}
	}
	return ""
}

// isValidFestival checks if a directory is a valid festival root.
func isValidFestival(dir string) bool {
	// Check for FESTIVAL_GOAL.md or FESTIVAL_OVERVIEW.md
	goalPath := filepath.Join(dir, FestivalGoalFile)
	if info, err := os.Stat(goalPath); err == nil && !info.IsDir() {
		return true
	}

	overviewPath := filepath.Join(dir, FestivalOverviewFile)
	if info, err := os.Stat(overviewPath); err == nil && !info.IsDir() {
		return true
	}

	// Also check for fest.yaml as a fallback
	configPath := filepath.Join(dir, FestivalConfigFile)
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		return true
	}

	return false
}

// FindFestivalByName searches for a festival by name in all status directories.
// For dungeon statuses, also searches inside date subdirectories.
func FindFestivalByName(ctx context.Context, festivalsDir, name, campaignRoot string) (*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, status := range id.StatusDirectories {
		statusDir := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusDir)
		if err != nil {
			continue // Skip inaccessible directories
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Check exact match or prefix match
			if entry.Name() == name || strings.HasPrefix(entry.Name(), name+"_") || strings.Contains(entry.Name(), name) {
				festivalDir := filepath.Join(statusDir, entry.Name())
				if isValidFestival(festivalDir) {
					info, err := parseFestivalInfo(ctx, festivalDir, campaignRoot)
					if err != nil {
						continue
					}
					info.Status = status
					return info, nil
				}
			}

			// If this is a date directory, search inside it
			if looksLikeDateDir(entry.Name()) {
				subEntries, subErr := os.ReadDir(filepath.Join(statusDir, entry.Name()))
				if subErr != nil {
					continue
				}
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					if sub.Name() == name || strings.HasPrefix(sub.Name(), name+"_") || strings.Contains(sub.Name(), name) {
						festivalDir := filepath.Join(statusDir, entry.Name(), sub.Name())
						if isValidFestival(festivalDir) {
							info, err := parseFestivalInfo(ctx, festivalDir, campaignRoot)
							if err != nil {
								continue
							}
							info.Status = status
							return info, nil
						}
					}
				}
			}
		}
	}

	return nil, errors.NotFound("festival").WithField("name", name).
		WithHint("Run 'fest show all' to see available festivals")
}

// ListFestivalsByStatus returns all festivals in a given status directory.
// For dungeon statuses, also recurses into date subdirectories.
func ListFestivalsByStatus(ctx context.Context, festivalsDir, status, campaignRoot string) ([]*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	statusDir := filepath.Join(festivalsDir, status)
	entries, err := os.ReadDir(statusDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*FestivalInfo{}, nil
		}
		return nil, errors.IO("reading status directory", err).WithField("status", status)
	}

	var festivals []*FestivalInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		festivalDir := filepath.Join(statusDir, entry.Name())
		if isValidFestival(festivalDir) {
			info, err := parseFestivalInfo(ctx, festivalDir, campaignRoot)
			if err != nil {
				info = &FestivalInfo{
					ID:     entry.Name(),
					Name:   entry.Name(),
					Status: status,
					Path:   festivalDir,
				}
			}
			info.Status = status
			festivals = append(festivals, info)
		} else if looksLikeDateDir(entry.Name()) {
			// Recurse into date subdirectory
			subEntries, subErr := os.ReadDir(festivalDir)
			if subErr != nil {
				continue
			}
			for _, sub := range subEntries {
				if !sub.IsDir() {
					continue
				}
				subDir := filepath.Join(festivalDir, sub.Name())
				if !isValidFestival(subDir) {
					continue
				}
				info, err := parseFestivalInfo(ctx, subDir, campaignRoot)
				if err != nil {
					info = &FestivalInfo{
						ID:     sub.Name(),
						Name:   sub.Name(),
						Status: status,
						Path:   subDir,
					}
				}
				info.Status = status
				festivals = append(festivals, info)
			}
		}
	}

	return festivals, nil
}

// ListFestivalsByStatusLight returns festivals with minimal metadata (no stats computation).
// Use this for UIs that only need name, status, path, and modtime — avoids the expensive
// recursive walk that CalculateFestivalStats performs on every task file.
// For dungeon statuses, also recurses into date subdirectories.
func ListFestivalsByStatusLight(ctx context.Context, festivalsDir, status string) ([]*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	statusDir := filepath.Join(festivalsDir, status)
	entries, err := os.ReadDir(statusDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*FestivalInfo{}, nil
		}
		return nil, errors.IO("reading status directory", err).WithField("status", status)
	}

	var festivals []*FestivalInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		festivalDir := filepath.Join(statusDir, entry.Name())
		if isValidFestival(festivalDir) {
			info := lightFestivalInfo(festivalDir, entry.Name(), status)
			festivals = append(festivals, info)
		} else if looksLikeDateDir(entry.Name()) {
			// Recurse into date subdirectory
			subEntries, subErr := os.ReadDir(festivalDir)
			if subErr != nil {
				continue
			}
			for _, sub := range subEntries {
				if !sub.IsDir() {
					continue
				}
				subDir := filepath.Join(festivalDir, sub.Name())
				if !isValidFestival(subDir) {
					continue
				}
				info := lightFestivalInfo(subDir, sub.Name(), status)
				festivals = append(festivals, info)
			}
		}
	}

	return festivals, nil
}

// lightFestivalInfo creates a FestivalInfo with minimal metadata from a festival directory.
func lightFestivalInfo(festivalDir, name, status string) *FestivalInfo {
	info := &FestivalInfo{
		ID:     name,
		Name:   name,
		Status: status,
		Path:   festivalDir,
	}

	if dirInfo, statErr := os.Stat(festivalDir); statErr == nil {
		info.ModTime = dirInfo.ModTime()
		info.UpdatedAt = dirInfo.ModTime()
	}

	// Extract timestamps from festival goal/overview frontmatter
	for _, goalFile := range []string{FestivalGoalFile, FestivalOverviewFile} {
		goalPath := filepath.Join(festivalDir, goalFile)
		data, readErr := os.ReadFile(goalPath)
		if readErr != nil {
			continue
		}
		fm, _, parseErr := frontmatter.Parse(data)
		if parseErr != nil || fm == nil {
			continue
		}
		if !fm.Created.IsZero() {
			info.CreatedAt = fm.Created
		}
		if !fm.Updated.IsZero() {
			info.UpdatedAt = fm.Updated
		}
		break
	}

	return info
}

// parseFestivalInfo parses festival information from a directory.
func parseFestivalInfo(ctx context.Context, festivalDir, campaignRoot string) (*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	info := &FestivalInfo{
		ID:   filepath.Base(festivalDir),
		Name: filepath.Base(festivalDir),
		Path: festivalDir,
	}

	// Determine status from parent directory
	parentDir := filepath.Dir(festivalDir)
	parentName := filepath.Base(parentDir)
	switch parentName {
	case "active", "ready", "planning":
		info.Status = parentName
	case "completed", "archived", "someday":
		// Could be dungeon/completed, dungeon/archived, dungeon/someday
		grandparentName := filepath.Base(filepath.Dir(parentDir))
		if grandparentName == "dungeon" {
			info.Status = "dungeon/" + parentName
		} else {
			info.Status = parentName
		}
	case "dungeon":
		info.Status = "dungeon"
	default:
		// Check if parent is a date directory (YYYY-MM-DD or YYYY-MM)
		if looksLikeDateDir(parentName) {
			// Walk up one more level to find the status name
			statusName := filepath.Base(filepath.Dir(parentDir))                // e.g., "completed"
			grandparent := filepath.Base(filepath.Dir(filepath.Dir(parentDir))) // e.g., "dungeon"
			if grandparent == "dungeon" && isKnownDungeonStatus(statusName) {
				info.Status = "dungeon/" + statusName
			} else {
				info.Status = "unknown"
			}
		} else {
			info.Status = "unknown"
		}
	}

	// Populate modification time from directory stat
	if dirInfo, statErr := os.Stat(festivalDir); statErr == nil {
		info.ModTime = dirInfo.ModTime()
		info.UpdatedAt = dirInfo.ModTime()
	}

	// Extract timestamps from festival goal/overview frontmatter
	for _, goalFile := range []string{FestivalGoalFile, FestivalOverviewFile} {
		goalPath := filepath.Join(festivalDir, goalFile)
		data, readErr := os.ReadFile(goalPath)
		if readErr != nil {
			continue
		}
		fm, _, parseErr := frontmatter.Parse(data)
		if parseErr != nil || fm == nil {
			continue
		}
		if !fm.Created.IsZero() {
			info.CreatedAt = fm.Created
		}
		if !fm.Updated.IsZero() {
			info.UpdatedAt = fm.Updated
		}
		break
	}

	// Try to load fest.yaml to get metadata ID and project path
	festConfig, err := config.LoadFestivalConfig(festivalDir, campaignRoot)
	if err == nil && festConfig != nil {
		// Extract metadata ID if present
		if festConfig.Metadata.ID != "" {
			info.MetadataID = festConfig.Metadata.ID
		}
		// Keep metadata name separate from directory name (used for linking)
		if festConfig.Metadata.Name != "" {
			info.MetadataName = festConfig.Metadata.Name
		}
		// Extract project path if present
		if festConfig.ProjectPath != "" {
			info.ProjectPath = festConfig.ProjectPath
		}
	}

	// Calculate statistics
	stats, err := CalculateFestivalStats(ctx, festivalDir)
	if err == nil {
		info.Stats = stats
	}

	return info, nil
}

// DetectCurrentLocation determines where we are in a festival hierarchy.
// Returns the current location type (festival, phase, sequence, task) and path info.
func DetectCurrentLocation(ctx context.Context, startDir string) (*LocationInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, errors.IO("getting absolute path", err)
	}

	// First find the festival root (no campaign root needed for location detection)
	festival, err := DetectCurrentFestival(ctx, absStart, "")
	if err != nil {
		return nil, err
	}

	loc := &LocationInfo{
		Festival: festival,
	}

	// Determine relative position within the festival
	relPath, err := filepath.Rel(festival.Path, absStart)
	if err != nil {
		return loc, nil // At festival root
	}

	if relPath == "." {
		loc.Type = "festival"
		return loc, nil
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) >= 1 {
		// First level is phase
		phaseDir := filepath.Join(festival.Path, parts[0])
		if isPhaseDir(phaseDir) {
			loc.Type = "phase"
			loc.Phase = parts[0]
		}
	}

	if len(parts) >= 2 && loc.Phase != "" {
		// Second level is sequence
		seqDir := filepath.Join(festival.Path, parts[0], parts[1])
		if isSequenceDir(seqDir) {
			loc.Type = "sequence"
			loc.Sequence = parts[1]
		}
	}

	if len(parts) >= 3 && loc.Sequence != "" {
		// Could be in a task file's directory
		loc.Type = "task"
		loc.Task = parts[2]
	}

	if loc.Type == "" {
		loc.Type = "festival"
	}

	return loc, nil
}

func isPhaseDir(dir string) bool {
	goalPath := filepath.Join(dir, PhaseGoalFile)
	if info, err := os.Stat(goalPath); err == nil && !info.IsDir() {
		return true
	}
	return false
}

func isSequenceDir(dir string) bool {
	goalPath := filepath.Join(dir, SequenceGoalFile)
	if info, err := os.Stat(goalPath); err == nil && !info.IsDir() {
		return true
	}
	return false
}

// LocationInfo describes the current location within a festival hierarchy.
type LocationInfo struct {
	Type     string        `json:"type"`               // festival, phase, sequence, task
	Festival *FestivalInfo `json:"festival,omitempty"` // Always present if in a festival
	Phase    string        `json:"phase,omitempty"`    // Phase directory name
	Sequence string        `json:"sequence,omitempty"` // Sequence directory name
	Task     string        `json:"task,omitempty"`     // Task file or directory
}
