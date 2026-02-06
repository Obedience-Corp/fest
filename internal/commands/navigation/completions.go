package navigation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

// dateDirectoryPattern matches YYYY-MM format for date-based directory organization
var dateDirectoryPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

// NewGoCompletionsCommand creates the hidden completions subcommand for shell integration
func NewGoCompletionsCommand() *cobra.Command {
	var descriptions bool
	var color bool

	cmd := &cobra.Command{
		Use:    "completions",
		Short:  "Output completion words for shell integration",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoCompletions(descriptions, color)
		},
	}

	cmd.Flags().BoolVar(&descriptions, "descriptions", false, "include descriptions for zsh _describe")
	cmd.Flags().BoolVar(&color, "color", false, "output tab-separated value\\tcolorized_display for zsh compadd")

	return cmd
}

// ANSI 256-color codes matching the fest dark palette (ui/palette.go)
const (
	ansiReset     = "\033[0m"
	ansiDim       = "\033[2m"
	ansiActive    = "\033[38;5;42m"  // bright green
	ansiPlanned   = "\033[38;5;33m"  // blue
	ansiCompleted = "\033[38;5;205m" // magenta
	ansiDungeon   = "\033[38;5;248m" // light grey
	ansiShortcut  = "\033[38;5;214m" // orange
)

// statusANSI returns the ANSI color escape for a status directory name
func statusANSI(status string) string {
	switch status {
	case "active":
		return ansiActive
	case "planned":
		return ansiPlanned
	case "completed":
		return ansiCompleted
	case "dungeon":
		return ansiDungeon
	default:
		return ansiDim
	}
}

func runGoCompletions(descriptions, color bool) error {
	// Subcommands
	subcommands := []string{
		"list",
		"link",
		"map",
		"unmap",
		"project",
		"fest",
		"help",
	}

	// Status directories
	statuses := []string{
		"active",
		"planned",
		"completed",
		"dungeon",
	}

	statusSet := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		statusSet[s] = true
	}

	// Output subcommands
	for _, cmd := range subcommands {
		switch {
		case color:
			fmt.Printf("%s\t%s %ssubcommand%s\n", cmd, cmd, ansiDim, ansiReset)
		case descriptions:
			fmt.Printf("%s:subcommand\n", cmd)
		default:
			fmt.Println(cmd)
		}
	}

	// Output status directories
	for _, status := range statuses {
		switch {
		case color:
			c := statusANSI(status)
			fmt.Printf("%s\t%s%s%s\n", status, c, status, ansiReset)
		case descriptions:
			fmt.Printf("%s:status directory\n", status)
		default:
			fmt.Println(status)
		}
	}

	// Load navigation state for shortcuts
	nav, err := navigation.LoadNavigation()
	if err == nil {
		for name := range nav.Shortcuts {
			switch {
			case color:
				fmt.Printf("-%s\t%s-%s%s %sshortcut%s\n", name, ansiShortcut, name, ansiReset, ansiDim, ansiReset)
			case descriptions:
				fmt.Printf("-%s:shortcut\n", name)
			default:
				fmt.Printf("-%s\n", name)
			}
		}
	}

	// Collect festival names from active/ and planned/
	cwd, err := os.Getwd()
	if err != nil {
		return nil // Silently skip on error
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return nil // Silently skip if no workspace
	}

	targets := navigation.CollectNavigationTargets(festivalsDir)
	for _, t := range targets {
		// Skip status directories (already emitted above)
		if statusSet[t.Name] {
			continue
		}
		status := statusFromPath(t.Path, festivalsDir)
		switch {
		case color:
			c := statusANSI(status)
			fmt.Printf("%s\t%s %s%s%s\n", t.Name, t.Name, c, status, ansiReset)
		case descriptions:
			fmt.Printf("%s:%s festival\n", t.Name, status)
		default:
			fmt.Println(t.Name)
		}
	}

	return nil
}

// statusFromPath extracts the status directory name (active, planned, etc.) from a festival path
func statusFromPath(path, festivalsDir string) string {
	rel, err := filepath.Rel(festivalsDir, path)
	if err != nil || rel == "." {
		return "festival"
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return "festival"
}

// isDateDirectory checks if a directory name matches YYYY-MM pattern
func isDateDirectory(name string) bool {
	return dateDirectoryPattern.MatchString(name)
}

// findCompletedFestivals finds all completed festivals, including those in date subdirectories.
// It supports both legacy flat structure and new date-based organization.
func findCompletedFestivals(completedDir, prefix string) []string {
	entries, err := os.ReadDir(completedDir)
	if err != nil {
		return nil
	}

	var festivals []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		if isDateDirectory(name) {
			// Search within date directory
			datePath := filepath.Join(completedDir, name)
			subEntries, err := os.ReadDir(datePath)
			if err != nil {
				continue
			}

			for _, subEntry := range subEntries {
				if !subEntry.IsDir() {
					continue
				}
				festName := subEntry.Name()
				// Check if it's a valid festival
				festPath := filepath.Join(datePath, festName)
				if !isValidFestivalDir(festPath) {
					continue
				}
				// Check prefix filter
				if prefix == "" || len(festName) >= len(prefix) && festName[:len(prefix)] == prefix {
					festivals = append(festivals, festName)
				}
			}
		} else {
			// Legacy flat structure
			festPath := filepath.Join(completedDir, name)
			if !isValidFestivalDir(festPath) {
				continue
			}
			if prefix == "" || len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				festivals = append(festivals, name)
			}
		}
	}

	return festivals
}

// isValidFestivalDir checks if a directory contains festival markers
func isValidFestivalDir(path string) bool {
	// Check for FESTIVAL_OVERVIEW.md or FESTIVAL_GOAL.md
	if _, err := os.Stat(filepath.Join(path, "FESTIVAL_OVERVIEW.md")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "FESTIVAL_GOAL.md")); err == nil {
		return true
	}
	return false
}

// CompleteGoTarget provides fuzzy completions for the go command target argument
func CompleteGoTarget(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get festivals directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Collect targets
	targets := navigation.CollectNavigationTargets(festivalsDir)
	if len(targets) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// If no partial input, return all targets
	if toComplete == "" {
		result := make([]string, len(targets))
		for i, t := range targets {
			result[i] = t.Name
		}
		return result, cobra.ShellCompDirectiveNoFileComp
	}

	// Fuzzy filter
	finder := navigation.NewFuzzyFinder(targets)
	matches := finder.Find(toComplete)

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.Name
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}
