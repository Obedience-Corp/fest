// Package workspace provides workspace-aware festivals directory detection.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrNoWorkspace is returned when no workspace could be found.
	ErrNoWorkspace = errors.New("not in a fest workspace")

	// ErrNoCampaign is returned when no campaign directory was found.
	ErrNoCampaign = errors.New("not in a campaign")
)

const (
	// MarkerFile is the name of the workspace marker file inside .festival/.state/
	MarkerFile = ".workspace"
	// FestivalsDir is the expected name of the festivals directory
	FestivalsDir = "festivals"
	// DotFestival is the hidden directory inside festivals/
	DotFestival = ".festival"
	// StateDir is the subdirectory for local state files (never synced)
	StateDir = ".state"
)

// Marker represents the .workspace file content
type Marker struct {
	Workspace  string    `json:"workspace"`
	Registered time.Time `json:"registered"`
}

// MarkerPath returns the full path to the marker file for a given festivals directory
func MarkerPath(festivalsDir string) string {
	return filepath.Join(festivalsDir, DotFestival, StateDir, MarkerFile)
}

// StatePath returns the full path to the .state directory for a given festivals directory
func StatePath(festivalsDir string) string {
	return filepath.Join(festivalsDir, DotFestival, StateDir)
}

// HasMarker checks if a festivals directory has a workspace marker
func HasMarker(festivalsDir string) bool {
	markerPath := MarkerPath(festivalsDir)
	info, err := os.Stat(markerPath)
	return err == nil && !info.IsDir()
}

// ReadMarker reads and parses the workspace marker from a festivals directory
func ReadMarker(festivalsDir string) (*Marker, error) {
	markerPath := MarkerPath(festivalsDir)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return nil, err
	}

	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, err
	}

	return &marker, nil
}

// RegisterFestivals creates a .workspace marker in festivals/.festival/
// The workspace name is derived from the parent directory of festivals/
func RegisterFestivals(festivalsDir string) error {
	// Ensure the path is absolute
	absPath, err := filepath.Abs(festivalsDir)
	if err != nil {
		return err
	}

	// Derive workspace name from parent directory
	parentDir := filepath.Dir(absPath)
	workspaceName := filepath.Base(parentDir)

	// Create marker
	marker := Marker{
		Workspace:  workspaceName,
		Registered: time.Now().UTC(),
	}

	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}

	// Ensure .festival/.state directory exists
	statePath := StatePath(absPath)
	if err := os.MkdirAll(statePath, 0755); err != nil {
		return err
	}

	// Write marker file
	markerPath := MarkerPath(absPath)
	return os.WriteFile(markerPath, data, 0644)
}

// UnregisterFestivals removes the .workspace marker from a festivals directory
func UnregisterFestivals(festivalsDir string) error {
	absPath, err := filepath.Abs(festivalsDir)
	if err != nil {
		return err
	}

	markerPath := MarkerPath(absPath)
	err = os.Remove(markerPath)
	if os.IsNotExist(err) {
		return nil // Already unregistered
	}
	return err
}

// FindMarkedFestivals walks UP from startDir looking for festivals/.festival/.workspace
// Returns the path to the first festivals/ directory that has a marker, or empty string if none found
func FindMarkedFestivals(startDir string) (string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	dir := absStart
	for {
		// Check if festivals/.festival/.workspace exists at this level
		festivalsPath := filepath.Join(dir, FestivalsDir)
		if HasMarker(festivalsPath) {
			return festivalsPath, nil
		}

		// Move up to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no marker found
			return "", nil
		}
		dir = parent
	}
}

// FindAllMarkedFestivals walks UP from startDir and collects ALL festivals/ directories with markers
// Used for `fest go --all`
func FindAllMarkedFestivals(startDir string) ([]string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	var results []string
	dir := absStart
	for {
		// Check if festivals/.festival/.workspace exists at this level
		festivalsPath := filepath.Join(dir, FestivalsDir)
		if HasMarker(festivalsPath) {
			results = append(results, festivalsPath)
		}

		// Move up to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return results, nil
}

// FindNearestFestivals walks UP from startDir looking for any festivals/ directory
// This is the fallback behavior when no markers are found
func FindNearestFestivals(startDir string) (string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	dir := absStart
	for {
		// Check if festivals/ exists at this level
		festivalsPath := filepath.Join(dir, FestivalsDir)
		info, err := os.Stat(festivalsPath)
		if err == nil && info.IsDir() {
			// Also check for .festival/ inside to confirm it's a valid festivals dir
			dotFestivalPath := filepath.Join(festivalsPath, DotFestival)
			if info, err := os.Stat(dotFestivalPath); err == nil && info.IsDir() {
				return festivalsPath, nil
			}
		}

		// Move up to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no festivals found
			return "", nil
		}
		dir = parent
	}
}

// FindFestivals finds the appropriate festivals directory, preferring marked ones
// Falls back to nearest festivals/ if no markers exist
func FindFestivals(startDir string) (string, error) {
	// First try to find a marked festivals directory
	marked, err := FindMarkedFestivals(startDir)
	if err != nil {
		return "", err
	}
	if marked != "" {
		return marked, nil
	}

	// Fall back to nearest festivals directory
	return FindNearestFestivals(startDir)
}

// CampaignDir is the name of the campaign marker directory.
const CampaignDir = ".campaign"

// EnvCampaignRoot is the environment variable that can override campaign detection.
const EnvCampaignRoot = "CAMP_ROOT"

// DetectCampaign walks up from startDir looking for a .campaign/ directory.
// Checks CAMP_ROOT env var first for explicit override.
func DetectCampaign(ctx context.Context, startDir string) (string, error) {
	// Check CAMP_ROOT env var first
	if root := os.Getenv(EnvCampaignRoot); root != "" {
		campaignDir := filepath.Join(root, CampaignDir)
		if info, err := os.Stat(campaignDir); err == nil && info.IsDir() {
			return root, nil
		}
		// If env var is set but invalid, continue with detection
	}

	// Start from given directory or cwd
	dir := startDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Resolve to absolute path
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	// Walk up directory tree
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		campaignDir := filepath.Join(current, CampaignDir)
		if info, err := os.Stat(campaignDir); err == nil && info.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached filesystem root
		}
		current = parent
	}

	return "", ErrNoCampaign
}

// FindWorkspace returns workspace information using available detection methods.
// Priority: campaign (.campaign/) → marked (.workspace) → nearest (festivals/).
func FindWorkspace(ctx context.Context, startDir string) (WorkspaceInfo, error) {
	if err := ctx.Err(); err != nil {
		return WorkspaceInfo{}, err
	}

	// Start from given directory or cwd
	dir := startDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return WorkspaceInfo{}, err
		}
	}

	// 1. Try campaign detection first
	if campaignRoot, err := DetectCampaign(ctx, dir); err == nil {
		festivalsPath := filepath.Join(campaignRoot, FestivalsDir)
		if _, err := os.Stat(festivalsPath); err == nil {
			return WorkspaceInfo{
				Root:          campaignRoot,
				FestivalsPath: festivalsPath,
				Type:          WorkspaceTypeCampaign,
			}, nil
		}
	}

	// 2. Fall back to standalone .workspace marker detection
	if festivalsPath, err := FindMarkedFestivals(dir); err == nil && festivalsPath != "" {
		return WorkspaceInfo{
			Root:          filepath.Dir(festivalsPath),
			FestivalsPath: festivalsPath,
			Type:          WorkspaceTypeStandalone,
		}, nil
	}

	// 3. Try nearest festivals/ as last resort
	if festivalsPath, err := FindNearestFestivals(dir); err == nil && festivalsPath != "" {
		return WorkspaceInfo{
			Root:          filepath.Dir(festivalsPath),
			FestivalsPath: festivalsPath,
			Type:          WorkspaceTypeStandalone,
		}, nil
	}

	return WorkspaceInfo{}, ErrNoWorkspace
}

// IsCampaignRoot checks if the given directory is a campaign root (contains .campaign/).
func IsCampaignRoot(dir string) bool {
	campaignPath := filepath.Join(dir, CampaignDir)
	info, err := os.Stat(campaignPath)
	return err == nil && info.IsDir()
}
