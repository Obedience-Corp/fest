package navigation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	festErrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const linkForceHint = "use 'fest link --force' to relink the project to this festival, or run 'fest unlink' from the linked festival first"

func linkStyles() (lipgloss.Style, lipgloss.Style, lipgloss.Style) {
	p := ui.Current()
	return lipgloss.NewStyle().Foreground(p.Active).Bold(true),
		lipgloss.NewStyle().Foreground(p.Planning).Bold(true),
		lipgloss.NewStyle().Foreground(p.Metadata)
}

// guardProjectConflict blocks relinking a project that is already linked to a
// different festival unless force is set. A project holds exactly one festival
// link; before this guard, a second link silently evicted the first (the
// "promote dropped my link" reports were this eviction, not promote). It
// returns the existing festival's name on a permitted takeover so callers can
// report the eviction.
func guardProjectConflict(nav *navigation.Navigation, festivalName, projectPath string, force bool) (string, error) {
	existing, existingFestivalPath, hasConflict := nav.ProjectConflict(festivalName, projectPath)
	if !hasConflict {
		return "", nil
	}
	if !force {
		return "", festErrors.Validation("project is already linked to festival "+existing).
			WithField("existing_festival", existing).
			WithField("existing_festival_path", existingFestivalPath).
			WithField("project", projectPath).
			WithHint(linkForceHint)
	}
	return existing, nil
}

// reportLinkEviction prints the takeover an accepted --force link performed.
func reportLinkEviction(evicted string) {
	if evicted == "" {
		return
	}
	fmt.Printf("%s %s\n", ui.Label("Replaced link"), ui.Dim("project was linked to "+evicted))
}

// NewGoLinkCommand creates the context-aware link subcommand for fest go
func NewGoLinkCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "link [path]",
		Short: "Link current festival to a project directory (or vice versa)",
		Long: `Create a bidirectional link between a festival and a project directory.

When run inside a festival:
  - Links the festival to the specified project directory
  - If no path provided, prompts for directory input

When run inside a project directory (non-festival):
  - Shows an interactive picker to select a festival to link
  - Links the current project to the selected festival

After linking:
  - 'fgo' in the festival jumps to the project
  - 'fgo' in the project jumps to the festival

Examples:
  # Inside a festival, link to project:
  fgo link /path/to/project
  fgo link .                    # Link to current directory (if not in festival)

  # Inside a project, show festival picker:
  fgo link`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 {
				path = args[0]
			}
			return runGoLink(cmd.Context(), path, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "relink a project already linked to another festival")

	return cmd
}

func runGoLink(ctx context.Context, targetPath string, force bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return festErrors.IO("getting current directory", err)
	}

	// Detect context: are we inside a festival?
	if isInsideFestival(cwd) {
		return linkFestivalToProject(ctx, cwd, targetPath, force)
	}

	// We're in a project directory, show festival picker
	return linkProjectToFestival(ctx, cwd, force)
}

// isInsideFestival checks if the path is within a festivals/ directory
func isInsideFestival(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Walk up to find festivals/ directory
	current := absPath
	for {
		base := filepath.Base(current)
		if base == "festivals" {
			return true
		}

		// Check if parent contains festivals/ (we're inside it)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}

		// Check if we just passed through a festivals directory
		parentBase := filepath.Base(parent)
		if parentBase == "festivals" {
			return true
		}

		current = parent
	}

	return false
}

// linkFestivalToProject links the current festival to a project directory
func linkFestivalToProject(ctx context.Context, cwd, targetPath string, force bool) error {
	// Detect current festival
	loc, err := show.DetectCurrentLocation(ctx, cwd)
	if err != nil || loc == nil || loc.Festival == nil || loc.Festival.Name == "" {
		return festErrors.Validation("not inside a recognized festival")
	}
	festivalName := loc.Festival.Name

	var projectPath string

	if targetPath != "" {
		// Use provided path
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return festErrors.Wrap(err, "resolving path").WithField("path", targetPath)
		}

		// Validate path exists and is a directory
		info, err := os.Stat(absPath)
		if err != nil {
			return festErrors.NotFound("directory").WithField("path", absPath)
		}
		if !info.IsDir() {
			return festErrors.Validation("path is not a directory").WithField("path", absPath)
		}

		projectPath = absPath
	} else {
		// Show interactive directory picker
		selectedPath, err := selectProjectDirectory(ctx, loc.Festival.Path, festivalName)
		if err != nil {
			// Silent exit on user cancel (Ctrl-C or Esc)
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}

		projectPath = selectedPath
	}

	// Get the festival path for reverse navigation
	festivalPath := loc.Festival.Path

	var evicted string
	if err := navigation.Update(ctx, func(nav *navigation.Navigation) error {
		if _, err := guardProjectConflict(nav, festivalName, projectPath, force); err != nil {
			return err
		}
		evicted = nav.SetLinkWithPath(festivalName, projectPath, festivalPath)
		return nil
	}); err != nil {
		return err
	}

	fmt.Println(ui.H1("Link Created"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(festivalName, ui.FestivalColor))
	fmt.Printf("%s %s\n", ui.Label("Project"), ui.Dim(projectPath))
	reportLinkEviction(evicted)
	fmt.Println()
	fmt.Println(ui.Dim("Use 'fgo' to navigate between them."))

	return nil
}

// linkProjectToFestival shows a picker to select a festival to link
func linkProjectToFestival(ctx context.Context, cwd string, force bool) error {
	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return festErrors.Wrap(err, "resolving current directory")
	}

	// Find festivals directory
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return festErrors.NotFound("festivals directory").WithHint("run 'fest init' first")
	}

	// Collect all festivals from active/ and planning/
	festivals, err := collectFestivals(festivalsDir)
	if err != nil {
		return festErrors.Wrap(err, "collecting festivals")
	}

	if len(festivals) == 0 {
		return festErrors.NotFound("festivals").WithHint("create a festival with 'fest create festival'")
	}

	// Build options for picker with color-coded status
	options := make([]huh.Option[string], 0, len(festivals))
	activeStyle, planningStyle, pathStyle := linkStyles()

	for _, f := range festivals {
		var label string
		if f.status == "active" {
			label = activeStyle.Render("● "+f.name) + " " + pathStyle.Render("(active)")
		} else {
			label = planningStyle.Render("○ "+f.name) + " " + pathStyle.Render("(planning)")
		}
		options = append(options, huh.NewOption(label, f.name))
	}

	// Show picker
	var selectedFestival string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a festival to link to this project").
				Description(pathStyle.Render(absPath)).
				Options(options...).
				Value(&selectedFestival),
		),
	).WithTheme(theme.GetThemeFromConfig(ctx))

	if err := form.Run(); err != nil {
		// Silent exit on user cancel (Ctrl-C or Esc)
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	if selectedFestival == "" {
		// User cancelled without selecting
		return nil
	}

	// Find the festival path from the selected festival
	var festivalPath string
	for _, f := range festivals {
		if f.name == selectedFestival {
			festivalPath = f.path
			break
		}
	}

	var evicted string
	if err := navigation.Update(ctx, func(nav *navigation.Navigation) error {
		if _, err := guardProjectConflict(nav, selectedFestival, absPath, force); err != nil {
			return err
		}
		evicted = nav.SetLinkWithPath(selectedFestival, absPath, festivalPath)
		return nil
	}); err != nil {
		return err
	}

	fmt.Println(ui.H1("Link Created"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(selectedFestival, ui.FestivalColor))
	fmt.Printf("%s %s\n", ui.Label("Project"), ui.Dim(absPath))
	reportLinkEviction(evicted)
	fmt.Println()
	fmt.Println(ui.Dim("Use 'fgo' to navigate between them."))

	return nil
}

type festivalInfo struct {
	name        string
	displayName string
	status      string
	path        string
}

// projectEntry represents a project directory that can be linked to a festival.
type projectEntry struct {
	path  string // Absolute path to the project directory
	label string // Display label (e.g., "camp" or "obey-platform-monorepo@obey")
}

// collectFestivals finds all festivals in primary status directories
func collectFestivals(festivalsDir string) ([]festivalInfo, error) {
	var festivals []festivalInfo

	for _, status := range id.PrimaryStatusDirs {
		statusPath := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue // Skip if directory doesn't exist
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				continue
			}

			festivals = append(festivals, festivalInfo{
				name:        name,
				displayName: fmt.Sprintf("%s (%s)", name, status),
				status:      status,
				path:        filepath.Join(statusPath, name),
			})
		}
	}

	return festivals, nil
}

// resolveFestivalPath finds the full path of a festival by name
func resolveFestivalPath(festivalsDir, festivalName string) string {
	for _, status := range id.StatusDirectories {
		statusPath := filepath.Join(festivalsDir, status)
		festPath := filepath.Join(statusPath, festivalName)
		if info, err := os.Stat(festPath); err == nil && info.IsDir() {
			return festPath
		}
	}

	return ""
}

// selectProjectDirectory shows an interactive picker for selecting a project directory.
// It lists directories inside campaignRoot/projects/, expanding monorepos into sub-entries.
func selectProjectDirectory(ctx context.Context, festivalPath, festivalName string) (string, error) {
	// Find campaign root by walking up from festival path
	// festivalPath is like: /path/to/campaign/festivals/active/festival-name
	// We want campaign root: /path/to/campaign
	campaignRoot := campaignRootFromFestival(festivalPath)

	entries, err := collectProjectDirectories(campaignRoot)
	if err != nil {
		return "", festErrors.Wrap(err, "collecting project directories")
	}

	if len(entries) == 0 {
		return "", festErrors.NotFound("project directories").
			WithHint("no directories found in projects/")
	}

	// Build options — show project label (with monorepo@sub notation)
	options := make([]huh.Option[string], 0, len(entries))
	_, _, pathStyle := linkStyles()
	for _, entry := range entries {
		label := pathStyle.Render(entry.label)
		options = append(options, huh.NewOption(label, entry.path))
	}

	var selectedDir string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Select project to link to '%s'", festivalName)).
				Description(pathStyle.Render("Projects: " + filepath.Join(campaignRoot, "projects"))).
				Options(options...).
				Value(&selectedDir),
		),
	).WithTheme(theme.GetThemeFromConfig(ctx))

	if err := form.Run(); err != nil {
		return "", err
	}

	if selectedDir == "" {
		return "", festErrors.Validation("no directory selected")
	}

	return selectedDir, nil
}

// campaignRootFromFestival derives the campaign root from a festival path.
// festivalPath is like: /path/to/campaign/festivals/active/festival-name
// Returns: /path/to/campaign
func campaignRootFromFestival(festivalPath string) string {
	festivalsDir := filepath.Dir(filepath.Dir(festivalPath)) // → festivals/
	return filepath.Dir(festivalsDir)                        // → campaign root
}

// collectProjectDirectories returns project directories inside campaignRoot/projects/,
// expanding monorepos (detected via .gitmodules) into sub-project entries using the
// name@submodule convention (matching camp CLI's project listing).
func collectProjectDirectories(campaignRoot string) ([]projectEntry, error) {
	projectsDir := filepath.Join(campaignRoot, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	var results []projectEntry
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirName := entry.Name()
		dirPath := filepath.Join(projectsDir, dirName)

		results = append(results, projectEntry{path: dirPath, label: dirName})

		// Expand monorepo sub-projects
		for _, sub := range listSubmodulePaths(dirPath) {
			results = append(results, projectEntry{
				path:  filepath.Join(dirPath, sub),
				label: dirName + "@" + sub,
			})
		}
	}
	return results, nil
}

// listSubmodulePaths parses .gitmodules to discover submodule paths.
// Returns nil if no .gitmodules exists or parsing fails.
func listSubmodulePaths(projectDir string) []string {
	cmd := exec.Command("git", "-C", projectDir, "config", "-f", ".gitmodules", "--list")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, ".path=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			paths = append(paths, parts[1])
		}
	}
	return paths
}
