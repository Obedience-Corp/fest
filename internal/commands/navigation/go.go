package navigation

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/tui/picker"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type goOptions struct {
	showWorkspace bool
	showAll       bool
	json          bool
	printOnly     bool
}

// NewGoCommand creates the go navigation command
func NewGoCommand() *cobra.Command {
	opts := &goOptions{}

	cmd := &cobra.Command{
		Use:   "go [target]",
		Short: "Navigate to festivals/ - use 'fgo' after shell-init setup",
		Long: `Navigate to your workspace's festivals directory.

The go command finds the festivals/ directory that has been registered
as your active workspace using 'fest init --register'.

By default, outputs 'cd /path' for human-friendly display.
Use --print to output just the bare path (for scripts, tools, and agents).

SHELL INTEGRATION (recommended):
  # Add to ~/.zshrc or ~/.bashrc:
  eval "$(fest shell-init zsh)"

Then use 'fgo' to navigate:
  fgo              Navigate to festivals root
  fgo 002          Navigate to phase 002
  fgo 2/1          Navigate to phase 2, sequence 1
  fgo fest_improv  Fuzzy match to fest-improvements-*

Without shell integration, use command substitution:
  cd "$(fest go --print)"
  cd "$(fest go 002 --print)"

Fuzzy matching is supported - partial names like "impl" will match
phases containing "IMPLEMENT". Multiple words narrow the search.

If no registered festivals are found, falls back to nearest festivals/.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: CompleteGoTarget,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			return runGo(cmd.Context(), target, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.showWorkspace, "workspace", false, "show which workspace was detected")
	cmd.Flags().BoolVar(&opts.showAll, "all", false, "list all registered festivals directories")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	cmd.Flags().BoolVar(&opts.printOnly, "print", false, "print path only (for shell integration, scripts, and agents)")

	// Add subcommands for navigation shortcuts
	cmd.AddCommand(NewGoMapCommand())
	cmd.AddCommand(NewGoUnmapCommand())
	cmd.AddCommand(NewGoListCommand())
	cmd.AddCommand(NewGoShortcutCommand())
	cmd.AddCommand(NewGoProjectCommand())
	cmd.AddCommand(NewGoFestCommand())
	cmd.AddCommand(NewGoLinkCommand())
	cmd.AddCommand(NewGoCompletionsCommand())
	cmd.AddCommand(NewGoMoveCommand())

	return cmd
}

// outputPath writes a resolved path to stdout in the appropriate format.
func outputPath(path string, opts *goOptions) {
	if opts.json {
		fmt.Printf(`{"path": "%s"}`+"\n", path)
	} else if opts.printOnly {
		fmt.Println(path)
	} else {
		fmt.Printf("cd %s\n", path)
	}
}

func runGo(ctx context.Context, target string, opts *goOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Handle --all flag
	if opts.showAll {
		return runGoAll(cwd, opts)
	}

	// Find the appropriate festivals directory
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directory")
	}

	if festivalsDir == "" {
		return errors.NotFound("festivals directory")
	}

	// Handle --workspace flag
	if opts.showWorkspace {
		return runGoWorkspace(festivalsDir, opts)
	}

	// Smart navigation: no target provided
	if target == "" {
		// Try smart bidirectional navigation
		if resultPath := trySmartNavigation(ctx, cwd, festivalsDir); resultPath != "" {
			outputPath(resultPath, opts)
			return nil
		}

		// Try interactive picker if TTY available
		if pickerPath, err := launchFestivalPicker(festivalsDir); err != nil {
			return err
		} else if pickerPath != "" {
			outputPath(pickerPath, opts)
			return nil
		}

		// Fall back to festivals root (non-TTY or no festivals)
		outputPath(festivalsDir, opts)
		return nil
	}

	// Resolve target if provided
	resolved, err := resolveGoTarget(target, festivalsDir)
	if err != nil {
		return err
	}

	// Output the path
	outputPath(resolved, opts)

	return nil
}

// trySmartNavigation attempts bidirectional navigation based on current location
func trySmartNavigation(ctx context.Context, cwd, festivalsDir string) string {
	nav, err := navigation.LoadNavigation()
	if err != nil {
		return ""
	}

	// Check if we're inside a festival
	if isInsideFestival(cwd) {
		// Try to find the festival name
		loc, err := show.DetectCurrentLocation(ctx, cwd)
		if err == nil && loc != nil && loc.Festival != nil && loc.Festival.Name != "" {
			// Check if there's a linked project
			if projectPath := nav.GetLinkedProject(loc.Festival.Name); projectPath != "" {
				// Verify the path exists
				if info, err := os.Stat(projectPath); err == nil && info.IsDir() {
					return projectPath
				}
			}
		}
		return "" // No linked project, fall through to default
	}

	// Check if we're in a linked project
	if festivalName := nav.FindFestivalForPath(cwd); festivalName != "" {
		// Find the festival's path
		festPath := resolveFestivalPath(festivalsDir, festivalName)
		if festPath != "" {
			return festPath
		}
	}

	return "" // No smart navigation available
}

func runGoAll(cwd string, opts *goOptions) error {
	allFestivals, err := workspace.FindAllMarkedFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals directories")
	}

	if len(allFestivals) == 0 {
		// Fall back to showing nearest
		nearest, err := workspace.FindNearestFestivals(cwd)
		if err != nil || nearest == "" {
			return errors.NotFound("festivals directories")
		}
		if opts.json {
			fmt.Printf(`{"festivals": [{"path": "%s", "registered": false}]}%s`, nearest, "\n")
		} else {
			fmt.Printf("%s (not registered)\n", nearest)
		}
		return nil
	}

	if opts.json {
		fmt.Print(`{"festivals": [`)
		for i, f := range allFestivals {
			marker, _ := workspace.ReadMarker(f)
			ws := ""
			if marker != nil {
				ws = marker.Workspace
			}
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf(`{"workspace": "%s", "path": "%s", "registered": true}`, ws, f)
		}
		fmt.Println("]}")
	} else {
		for _, f := range allFestivals {
			marker, _ := workspace.ReadMarker(f)
			if marker != nil {
				fmt.Printf("%s → %s\n", marker.Workspace, f)
			} else {
				fmt.Println(f)
			}
		}
	}

	return nil
}

func runGoWorkspace(festivalsDir string, opts *goOptions) error {
	marker, err := workspace.ReadMarker(festivalsDir)
	ws := "(not registered)"
	if err == nil && marker != nil {
		ws = marker.Workspace
	}

	if opts.json {
		registered := marker != nil
		fmt.Printf(`{"workspace": "%s", "path": "%s", "registered": %t}%s`, ws, festivalsDir, registered, "\n")
	} else {
		fmt.Printf("%s → %s\n", ws, festivalsDir)
	}

	return nil
}

func resolveGoTarget(target, festivalsDir string) (string, error) {
	// Check if target looks like a phase shortcut (numeric)
	if isPhaseShortcut(target) {
		return resolvePhaseShortcut(target, festivalsDir)
	}

	// Check if target looks like phase/sequence shortcut
	if strings.Contains(target, "/") {
		parts := strings.SplitN(target, "/", 2)
		if isPhaseShortcut(parts[0]) {
			phaseDir, err := resolvePhaseShortcut(parts[0], festivalsDir)
			if err != nil {
				return "", err
			}
			if len(parts) > 1 && isSequenceShortcut(parts[1]) {
				return resolveSequenceShortcut(parts[1], phaseDir)
			}
			return phaseDir, nil
		}
	}

	// Resolve dungeon status aliases (completed → dungeon/completed, etc.)
	if resolved := id.ResolveStatusPath(target); resolved != target {
		fullPath := filepath.Join(festivalsDir, resolved)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			return fullPath, nil
		}
	}

	// Try to resolve as festival name (searches active/, planning/, completed/)
	if festPath := resolveFestivalByName(target, festivalsDir); festPath != "" {
		return festPath, nil
	}

	// Treat as a relative path within festivals
	fullPath := filepath.Join(festivalsDir, target)
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		return fullPath, nil
	}

	// Try fuzzy matching as fallback
	return resolveFuzzy(target, festivalsDir)
}

// resolveFuzzy attempts to match the target using fuzzy matching
func resolveFuzzy(pattern, festivalsDir string) (string, error) {
	debug := os.Getenv("FEST_DEBUG") != ""

	// Collect all possible targets
	start := time.Now()
	targets := navigation.CollectNavigationTargets(festivalsDir)
	if debug {
		log.Printf("[DEBUG] CollectNavigationTargets: %v (%d targets)", time.Since(start), len(targets))
	}
	if len(targets) == 0 {
		return "", errors.NotFound("target").WithField("pattern", pattern)
	}

	start = time.Now()
	finder := navigation.NewFuzzyFinder(targets)
	matches := finder.Find(pattern)
	if debug {
		log.Printf("[DEBUG] FuzzyFind: %v (%d matches)", time.Since(start), len(matches))
	}

	if len(matches) == 0 {
		return "", errors.NotFound("target").WithField("pattern", pattern)
	}

	// If single match or unambiguous best match, return it
	if len(matches) == 1 || navigation.IsUnambiguous(matches) {
		return matches[0].Path, nil
	}

	// Multiple ambiguous matches - show interactive picker if in TTY
	start = time.Now()
	result, err := showFuzzyPicker(pattern, matches)
	if debug {
		log.Printf("[DEBUG] showFuzzyPicker: %v", time.Since(start))
	}
	return result, err
}

// resolveFestivalByName searches for a festival by name in status directories.
// For dungeon substatuses it also descends one level into YYYY-MM-DD date
// buckets, matching the on-disk layout used when festivals are moved to the
// dungeon. Fuzzy search intentionally excludes the dungeon (see
// internal/navigation/fuzzy.go); exact-name lookup does not.
func resolveFestivalByName(name, festivalsDir string) string {
	for _, status := range id.StatusDirectories {
		statusDir := filepath.Join(festivalsDir, status)

		// Direct child: active/ready/planning/ritual, or any flat dungeon
		// entry that was moved without a date bucket.
		festPath := filepath.Join(statusDir, name)
		if info, err := os.Stat(festPath); err == nil && info.IsDir() {
			return festPath
		}

		// Dungeon substatuses: descend one level into date buckets.
		// Pick the newest bucket when the same name exists in multiple
		// buckets, matching the "newest first" semantics in sortByStatusDate.
		if !strings.HasPrefix(status, "dungeon/") {
			continue
		}
		entries, err := os.ReadDir(statusDir)
		if err != nil {
			continue
		}
		var best, bestBucket string
		for _, entry := range entries {
			if !entry.IsDir() || !show.LooksLikeDateDir(entry.Name()) {
				continue
			}
			datedPath := filepath.Join(statusDir, entry.Name(), name)
			if info, err := os.Stat(datedPath); err == nil && info.IsDir() {
				if entry.Name() > bestBucket {
					best, bestBucket = datedPath, entry.Name()
				}
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func isPhaseShortcut(s string) bool {
	// Phase shortcuts are 1-3 digit numbers
	if len(s) == 0 || len(s) > 3 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isSequenceShortcut(s string) bool {
	// Sequence shortcuts are 1-2 digit numbers
	if len(s) == 0 || len(s) > 2 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolvePhaseShortcut(shortcut, festivalsDir string) (string, error) {
	// Pad to 3 digits
	padded := fmt.Sprintf("%03s", shortcut)
	if len(shortcut) < 3 {
		// Convert "2" to "002", "02" to "002"
		n := 0
		_, _ = fmt.Sscanf(shortcut, "%d", &n)
		padded = fmt.Sprintf("%03d", n)
	}

	// Search in primary status directories for navigation
	searchDirs := []string{festivalsDir} // Start with root
	for _, status := range id.PrimaryStatusDirs {
		searchDirs = append(searchDirs, filepath.Join(festivalsDir, status))
	}

	for _, searchDir := range searchDirs {
		entries, err := os.ReadDir(searchDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), padded+"_") {
				return filepath.Join(searchDir, entry.Name()), nil
			}
		}
	}

	return "", errors.NotFound("phase").WithField("shortcut", shortcut)
}

func resolveSequenceShortcut(shortcut, phaseDir string) (string, error) {
	// Pad to 2 digits
	n := 0
	_, _ = fmt.Sscanf(shortcut, "%d", &n)
	padded := fmt.Sprintf("%02d", n)

	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return "", errors.IO("reading phase directory", err).WithField("path", phaseDir)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), padded+"_") {
			return filepath.Join(phaseDir, entry.Name()), nil
		}
	}

	return "", errors.NotFound("sequence").WithField("shortcut", shortcut).WithField("phase", filepath.Base(phaseDir))
}

// launchFestivalPicker shows an interactive fuzzy picker for all festivals.
// Returns the selected path, or empty string if cancelled or not available.
func launchFestivalPicker(festivalsDir string) (string, error) {
	stdinIsTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stderrIsTTY := term.IsTerminal(int(os.Stderr.Fd()))
	if !stdinIsTTY || !stderrIsTTY {
		return "", nil
	}

	items := collectPickerItems(festivalsDir)
	if len(items) == 0 {
		return "", nil
	}

	selected, err := picker.Run(items, navigation.Score)
	if err != nil {
		return "", errors.Wrap(err, "running festival picker")
	}
	if selected == nil {
		return "", nil
	}

	return selected.Value, nil
}

// collectPickerItems gathers all festivals and status directories as picker items.
func collectPickerItems(festivalsDir string) []picker.Item {
	var items []picker.Item

	for _, status := range id.StatusDirectories {
		statusPath := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}

		items = append(items, picker.Item{
			Name:  fmt.Sprintf("[%s]/", status),
			Value: statusPath,
		})

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			items = append(items, picker.Item{
				Name:  fmt.Sprintf("[%s] %s", status, entry.Name()),
				Value: filepath.Join(statusPath, entry.Name()),
			})
		}
	}

	return items
}

// showFuzzyPicker displays an interactive picker for ambiguous matches.
// Falls back to error message if not in a TTY.
func showFuzzyPicker(pattern string, matches []navigation.FuzzyMatch) (string, error) {
	debug := os.Getenv("FEST_DEBUG") != ""

	// Check if both stdin (for input) and stderr (for rendering) are TTYs
	// The picker needs stdin for keyboard input and stderr for TUI rendering
	start := time.Now()
	stdinIsTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stderrIsTTY := term.IsTerminal(int(os.Stderr.Fd()))
	canRunTUI := stdinIsTTY && stderrIsTTY
	if debug {
		log.Printf("[DEBUG] term.IsTerminal: %v (stdin=%v, stderr=%v, canRunTUI=%v)",
			time.Since(start), stdinIsTTY, stderrIsTTY, canRunTUI)
	}

	if !canRunTUI {
		// Not a TTY - return error with suggestions
		suggestions := navigation.FormatMatchList(matches, 5)
		msg := fmt.Sprintf("ambiguous pattern '%s' - matches: %s", pattern, strings.Join(suggestions, ", "))
		return "", errors.Validation(msg)
	}

	// Convert matches to picker items
	start = time.Now()
	items := make([]picker.Item, len(matches))
	for i, m := range matches {
		items[i] = picker.Item{
			Name:  m.Name,
			Value: m.Path,
			Score: m.Score,
		}
	}
	if debug {
		log.Printf("[DEBUG] convertItems: %v (%d items)", time.Since(start), len(items))
	}

	// Run picker
	start = time.Now()
	selected, err := picker.Run(items, navigation.Score)
	if debug {
		log.Printf("[DEBUG] picker.Run: %v", time.Since(start))
	}
	if err != nil {
		return "", errors.Wrap(err, "running picker")
	}

	if selected == nil {
		return "", errors.Validation("selection cancelled")
	}

	return selected.Value, nil
}
