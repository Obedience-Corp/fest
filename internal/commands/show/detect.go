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
	"github.com/Obedience-Corp/fest/internal/resident"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// dateDirPattern matches YYYY-MM-DD or YYYY-MM formatted date directory names.
var dateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}(-\d{2})?$`)

// LooksLikeDateDir reports whether a directory name matches the dungeon
// date-bucket shape (YYYY-MM-DD, or legacy YYYY-MM).
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

// findLinkedFestivalPath searches for a festival by exact name in all status
// directories, including festivals nested under organizational buckets.
func findLinkedFestivalPath(festivalsRoot, name string) string {
	for _, status := range id.StatusDirectories {
		deep := strings.HasPrefix(status, "dungeon/")
		for _, entry := range festivalDirsUnderStatus(workspace.JoinStatus(festivalsRoot, status), deep) {
			if filepath.Base(entry.path) == name {
				return entry.path
			}
		}
	}
	return ""
}

// maxDungeonNestDepth bounds how deep festival resolution recurses under a
// dungeon status directory when festivals are filed inside organizational
// subfolders (date buckets like 2026-03-01, or free-form buckets like
// pre-2026-03-01).
const maxDungeonNestDepth = 3

// maxWorkingStatusNestDepth bounds recursion under a working status
// directory (active/ready/planning/ritual) to a single date-bucket level,
// matching the shallow lookup those statuses have always used.
const maxWorkingStatusNestDepth = 1

// festivalDir is a resolved festival root plus the outermost organizational
// bucket it was nested under below the status dir, if any.
type festivalDir struct {
	path   string
	bucket string
}

// festivalDirsUnderStatus returns every festival root at or below statusDir.
// When deep is true (dungeon statuses), it recurses through any non-festival
// subdirectory up to maxDungeonNestDepth so a festival filed under any
// organizational bucket resolves. When deep is false (working statuses), it
// only recurses one level into date-bucket-shaped subdirectories, preserving
// the prior shallow lookup for active/ready/planning/ritual.
func festivalDirsUnderStatus(statusDir string, deep bool) []festivalDir {
	maxDepth := maxWorkingStatusNestDepth
	if deep {
		maxDepth = maxDungeonNestDepth
	}

	var out []festivalDir
	var walk func(dir, bucket string, depth int)
	walk = func(dir, bucket string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			child := filepath.Join(dir, entry.Name())
			if isValidFestival(child) {
				out = append(out, festivalDir{path: child, bucket: bucket})
				continue
			}
			if depth >= maxDepth {
				continue
			}
			if !deep && !LooksLikeDateDir(entry.Name()) {
				continue
			}
			nextBucket := bucket
			if nextBucket == "" {
				nextBucket = entry.Name()
			}
			walk(child, nextBucket, depth+1)
		}
	}
	walk(statusDir, "", 0)
	return out
}

// isValidFestival checks if a directory is a valid festival root.
func isValidFestival(dir string) bool {
	return resident.IsFestivalDir(dir)
}

// FindFestivalByName searches for a festival by name in all status directories.
// For dungeon statuses, also searches inside date subdirectories.
func FindFestivalByName(ctx context.Context, festivalsDir, name, campaignRoot string) (*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var loose *festivalDir
	var looseStatus string

	for _, status := range id.StatusDirectories {
		deep := strings.HasPrefix(status, "dungeon/")
		for _, entry := range festivalDirsUnderStatus(workspace.JoinStatus(festivalsDir, status), deep) {
			base := filepath.Base(entry.path)
			switch {
			case base == name || strings.HasPrefix(base, name+"_"):
				info, err := parseFestivalInfo(ctx, entry.path, campaignRoot)
				if err != nil {
					continue
				}
				info.Status = status
				if entry.bucket != "" {
					info.StatusDate = entry.bucket
				}
				return info, nil
			case loose == nil && strings.Contains(base, name):
				match := entry
				loose = &match
				looseStatus = status
			}
		}
	}

	if loose != nil {
		info, err := parseFestivalInfo(ctx, loose.path, campaignRoot)
		if err == nil {
			info.Status = looseStatus
			if loose.bucket != "" {
				info.StatusDate = loose.bucket
			}
			return info, nil
		}
	}

	// The search above walked every status including the dungeon buckets, so
	// a plain "not found" could actually mean the festival is filed under
	// whichever dungeon spelling ResolveDungeonDir did not choose.
	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return nil, err
	}

	return nil, errors.NotFound("festival").WithField("name", name).
		WithHint("Run 'fest list --all' to see available festivals")
}

// ListFestivalsByStatus returns all festivals in a given status directory.
// For dungeon statuses, also recurses into date subdirectories.
func ListFestivalsByStatus(ctx context.Context, festivalsDir, status, campaignRoot string) ([]*FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	statusDir := workspace.JoinStatus(festivalsDir, status)
	if _, err := os.ReadDir(statusDir); err != nil {
		if os.IsNotExist(err) {
			return []*FestivalInfo{}, nil
		}
		return nil, errors.IO("reading status directory", err).WithField("status", status)
	}

	var festivals []*FestivalInfo
	for _, entry := range festivalDirsUnderStatus(statusDir, strings.HasPrefix(status, "dungeon/")) {
		info, err := parseFestivalInfo(ctx, entry.path, campaignRoot)
		if err != nil {
			info = &FestivalInfo{
				ID:     filepath.Base(entry.path),
				Name:   filepath.Base(entry.path),
				Status: status,
				Path:   entry.path,
			}
		}
		info.Status = status
		if entry.bucket != "" {
			info.StatusDate = entry.bucket
		}
		festivals = append(festivals, info)
	}

	return festivals, nil
}

// ListFestivalsByStatusLight returns festivals with minimal metadata (no stats computation).
// Use this for UIs that only need name, status, path, and modtime — avoids the expensive
// recursive walk that CalculateFestivalStats performs on every task file.
// For dungeon statuses, also recurses into date subdirectories.
func ListFestivalsByStatusLight(_ context.Context, festivalsDir, status string) ([]*FestivalInfo, error) {
	statusDir := workspace.JoinStatus(festivalsDir, status)
	if _, err := os.ReadDir(statusDir); err != nil {
		if os.IsNotExist(err) {
			return []*FestivalInfo{}, nil
		}
		return nil, errors.IO("reading status directory", err).WithField("status", status)
	}

	var festivals []*FestivalInfo
	for _, entry := range festivalDirsUnderStatus(statusDir, strings.HasPrefix(status, "dungeon/")) {
		info := lightFestivalInfo(entry.path, filepath.Base(entry.path), status)
		if entry.bucket != "" {
			info.StatusDate = entry.bucket
		}
		festivals = append(festivals, info)
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
	switch {
	case parentName == "active" || parentName == "ready" || parentName == "planning" || parentName == "ritual":
		info.Status = parentName
	case parentName == "completed" || parentName == "archived" || parentName == "someday":
		// Could be dungeon/completed, dungeon/archived, dungeon/someday
		grandparentName := filepath.Base(filepath.Dir(parentDir))
		if workspace.IsDungeonDirName(grandparentName) {
			info.Status = "dungeon/" + parentName
		} else {
			info.Status = parentName
		}
	case workspace.IsDungeonDirName(parentName):
		info.Status = "dungeon"
	default:
		// Check if parent is a date directory (YYYY-MM-DD or YYYY-MM)
		if LooksLikeDateDir(parentName) {
			// Walk up one more level to find the status name
			statusName := filepath.Base(filepath.Dir(parentDir))                // e.g., "completed"
			grandparent := filepath.Base(filepath.Dir(filepath.Dir(parentDir))) // e.g., "dungeon"
			if workspace.IsDungeonDirName(grandparent) && isKnownDungeonStatus(statusName) {
				info.Status = "dungeon/" + statusName
				info.StatusDate = parentName
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
