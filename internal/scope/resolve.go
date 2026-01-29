package scope

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/navigation"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

// WorkspaceType indicates the type of workspace detected.
type WorkspaceType string

const (
	// WorkspaceTypeCampaign indicates a .campaign/ directory was found.
	WorkspaceTypeCampaign WorkspaceType = "campaign"

	// WorkspaceTypeStandalone indicates a standalone festivals/ workspace.
	WorkspaceTypeStandalone WorkspaceType = "standalone"
)

// WorkspaceInfo holds resolved workspace context.
// This will be expanded in sequence 02_workspace_detection.
type WorkspaceInfo struct {
	// Root is the workspace root (parent of festivals/).
	// For campaigns, this is the .campaign/ parent.
	Root string

	// FestivalsPath is the absolute path to the festivals/ directory.
	FestivalsPath string

	// Type indicates whether this is a campaign workspace or standalone.
	Type WorkspaceType
}

// Resolve reads the command's scope annotation and resolves the required
// filesystem context. It should be called from rootCmd.PersistentPreRunE.
//
// Commands declare scope via Annotations: map[string]string{"scope": "festival"}
// Commands without annotation default to Workspace scope.
func Resolve(cmd *cobra.Command) error {
	// Handle built-in cobra commands that don't have annotations.
	// These should work globally without any workspace context.
	if cmd.Name() == "help" || cmd.Name() == "__complete" || cmd.Name() == "__completeNoDesc" {
		return nil
	}

	// Handle root command (showing help/version).
	// Only match the actual "fest" root command, not test commands without parents.
	if cmd.Use == "fest" && cmd.Parent() == nil {
		return nil
	}

	scopeStr, ok := cmd.Annotations["scope"]
	if !ok {
		scopeStr = string(Workspace) // default
	}

	switch CommandScope(scopeStr) {
	case Global:
		return nil

	case Workspace:
		return resolveWorkspaceScope(cmd)

	case Festival:
		return resolveFestivalScope(cmd)

	default:
		return fmt.Errorf("unknown scope: %s", scopeStr)
	}
}

// resolveWorkspaceScope resolves workspace context and injects it into the command context.
func resolveWorkspaceScope(cmd *cobra.Command) error {
	ctx := cmd.Context()

	ws, err := resolveWorkspace()
	if err != nil {
		return workspaceScopeError()
	}

	ctx = WithWorkspace(ctx, ws)
	cmd.SetContext(ctx)
	return nil
}

// resolveWorkspace finds the festivals/ directory using the existing detection chain.
// This will be replaced with workspace.FindWorkspace() in sequence 02_workspace_detection.
func resolveWorkspace() (*WorkspaceInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Use existing workspace detection (marked workspace or nearest)
	festivalsPath, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsPath == "" {
		return nil, fmt.Errorf("no festivals directory found")
	}

	return &WorkspaceInfo{
		Root:          filepath.Dir(festivalsPath),
		FestivalsPath: festivalsPath,
		Type:          WorkspaceTypeStandalone, // Will be updated in sequence 02
	}, nil
}

// workspaceScopeError returns an actionable error for workspace scope failures.
func workspaceScopeError() error {
	return fmt.Errorf(`not in a fest workspace

  Could not find a festivals/ directory.

  To fix this, either:
    - Run from inside a campaign (contains .campaign/)
    - Run from inside or above a festivals/ directory
    - Set CAMP_ROOT to your campaign root
    - Run 'fest init' to create a new workspace`)
}

// resolveFestivalScope resolves both workspace and festival context.
func resolveFestivalScope(cmd *cobra.Command) error {
	ctx := cmd.Context()

	// First resolve workspace
	ws, err := resolveWorkspace()
	if err != nil {
		return workspaceScopeError()
	}
	ctx = WithWorkspace(ctx, ws)

	// Then resolve festival
	festivalPath, err := resolveFestival(ws)
	if err == nil {
		ctx = WithFestival(ctx, festivalPath)
		cmd.SetContext(ctx)
		return nil
	}

	// Festival not found from cwd or link.
	// Check if command has --festival flag with a value — let it handle resolution.
	if f := cmd.Flags().Lookup("festival"); f != nil && f.Value.String() != "" {
		cmd.SetContext(ctx) // workspace resolved, festival deferred to command
		return nil
	}

	return festivalScopeError()
}

// resolveFestival attempts to find the current festival from cwd or navigation links.
func resolveFestival(ws *WorkspaceInfo) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Step 1: Walk up from cwd looking for fest.yaml (canonical festival marker)
	festivalPath, err := tpl.FindFestivalRoot(cwd)
	if err == nil {
		return festivalPath, nil
	}

	// Step 2: Check navigation links
	nav, err := navigation.LoadNavigation()
	if err == nil {
		if linkedName := nav.FindFestivalForPath(cwd); linkedName != "" {
			// Found a linked festival name, search for its path
			festivalPath, err := findFestivalByName(ws.FestivalsPath, linkedName)
			if err == nil {
				return festivalPath, nil
			}
		}
	}

	return "", fmt.Errorf("no festival found")
}

// findFestivalByName searches for a festival by name in all status directories.
func findFestivalByName(festivalsRoot, name string) (string, error) {
	statusDirs := []string{"active", "planned", "completed", "dungeon"}

	for _, status := range statusDirs {
		statusDir := filepath.Join(festivalsRoot, status)
		festivalPath := filepath.Join(statusDir, name)

		// Check if this festival directory exists
		if info, err := os.Stat(festivalPath); err == nil && info.IsDir() {
			// Verify it's a valid festival by checking for festival markers
			if isValidFestival(festivalPath) {
				return festivalPath, nil
			}
		}
	}

	return "", fmt.Errorf("festival %q not found", name)
}

// isValidFestival checks if a directory is a valid festival root.
func isValidFestival(dir string) bool {
	// Check for FESTIVAL_GOAL.md
	goalPath := filepath.Join(dir, "FESTIVAL_GOAL.md")
	if info, err := os.Stat(goalPath); err == nil && !info.IsDir() {
		return true
	}

	// Check for FESTIVAL_OVERVIEW.md
	overviewPath := filepath.Join(dir, "FESTIVAL_OVERVIEW.md")
	if info, err := os.Stat(overviewPath); err == nil && !info.IsDir() {
		return true
	}

	// Check for fest.yaml as fallback
	configPath := filepath.Join(dir, "fest.yaml")
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		return true
	}

	return false
}

// festivalScopeError returns an actionable error for festival scope failures.
func festivalScopeError() error {
	return fmt.Errorf(`not inside a festival

  This command operates on a specific festival.

  To fix this, either:
    - Run from inside a festival directory
    - Run from a linked project directory (see 'fest link')
    - Use --festival <path> to specify one`)
}
