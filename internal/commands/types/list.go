package types

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		level      string
		jsonOutput bool
		showAll    bool
		customOnly bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available template types",
		Long: `List types you can pass to fest create, grouped by level.

Sources:
  - Festival workflow types from festival_types.yaml (create festival --type)
  - Phase/sequence/task scaffold packages under the methodology templates tree
    (~/.obey/fest/festivals/.festival/templates or camp festivals/.festival/templates)
  - Custom overrides in a festival's .festival/templates/

Examples:
  fest types list                      # All levels
  fest types list --level festival     # create festival --type values
  fest types list --level phase        # create phase --type values
  fest types list --json               # Machine-readable output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), level, jsonOutput, showAll, customOnly)
		},
	}

	cmd.Flags().StringVarP(&level, "level", "l", "", "Filter by level (festival, phase, sequence, task)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show additional details including marker counts")
	cmd.Flags().BoolVarP(&customOnly, "custom", "c", false, "Show only custom (user-defined) types")

	return cmd
}

func runList(ctx context.Context, levelFilter string, jsonOutput, showAll, customOnly bool) error {
	registry, err := discoverRegistry(ctx, showAll)
	if err != nil {
		return err
	}

	// Filter by level if specified
	var filteredTypes []types.TypeInfo
	if levelFilter != "" {
		level := types.Level(levelFilter)
		filteredTypes = registry.TypesForLevel(level)
	} else {
		filteredTypes = registry.AllTypes()
	}

	// Filter by custom if specified
	if customOnly {
		filteredTypes = filterCustomTypes(filteredTypes)
	}

	if jsonOutput {
		return outputJSON(filteredTypes)
	}

	return outputText(registry, levelFilter, showAll, customOnly)
}

// discoverRegistry loads scaffold types from the methodology tree and merges
// festival workflow types from festival_types.yaml.
func discoverRegistry(ctx context.Context, countMarkers bool) (*types.Registry, error) {
	registry := types.NewRegistry()
	opts := types.DiscoverOptions{
		BuiltInDir:   getBuiltInTemplatesDir(),
		CustomDir:    getCustomTemplatesDir(),
		CountMarkers: countMarkers,
	}
	if err := registry.Discover(ctx, opts); err != nil {
		return nil, err
	}
	if config, err := types.LoadFestivalTypesConfig(ctx); err == nil {
		registry.MergeFestivalWorkflowTypes(config)
	}
	return registry, nil
}

func filterCustomTypes(typeInfos []types.TypeInfo) []types.TypeInfo {
	result := []types.TypeInfo{}
	for _, t := range typeInfos {
		if t.IsCustom {
			result = append(result, t)
		}
	}
	return result
}

func outputJSON(typeInfos []types.TypeInfo) error {
	data, err := json.MarshalIndent(typeInfos, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputText(registry *types.Registry, levelFilter string, showAll, customOnly bool) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	if registry.TypeCount() == 0 {
		display.Info("No template types found.")
		builtIn := getBuiltInTemplatesDir()
		if !templatesDirExists(builtIn) {
			display.Info("Tip: Run 'fest system sync' to download built-in methodology templates.")
			display.Info("Expected templates at: %s", builtIn)
		} else {
			display.Info("Templates directory exists (%s) but no types were discovered.", builtIn)
			display.Info("Tip: Check for phases/, sequences/, tasks/ under that path, or set FEST_TEMPLATES_DIR.")
		}
		return nil
	}

	if levelFilter != "" {
		level := types.Level(levelFilter)
		typeInfos := registry.TypesForLevel(level)
		if customOnly {
			typeInfos = filterCustomTypes(typeInfos)
		}
		if len(typeInfos) == 0 {
			display.Info("No %s types found.", levelFilter)
			if customOnly {
				display.Info("Tip: Create custom templates in .festival/templates/")
			} else if levelFilter == "festival" {
				display.Info("Tip: Run 'fest types festival list' or check festivals/.festival/festival_types.yaml")
			}
			return nil
		}
		fmt.Printf("%s Types:\n\n", capitalize(levelFilter))
		printTypes(typeInfos, showAll)
	} else {
		// Print by level
		foundAny := false
		for _, level := range types.AllLevels() {
			typeInfos := registry.TypesForLevel(level)
			if customOnly {
				typeInfos = filterCustomTypes(typeInfos)
			}
			if len(typeInfos) == 0 {
				continue
			}
			foundAny = true
			fmt.Printf("%s Types:\n", capitalize(string(level)))
			printTypes(typeInfos, showAll)
			fmt.Println()
		}
		if !foundAny && customOnly {
			display.Info("No custom types found.")
			display.Info("Tip: Create custom templates in .festival/templates/")
		}
	}

	return nil
}

func printTypes(typeInfos []types.TypeInfo, showAll bool) {
	for _, t := range typeInfos {
		suffix := ""
		if t.IsDefault {
			suffix = " (default)"
		} else if t.IsCustom {
			suffix = " (custom)"
		}

		line := fmt.Sprintf("  %s%s", t.Name, suffix)
		if showAll && t.Markers > 0 {
			line = fmt.Sprintf("  %-20s %d markers%s", t.Name, t.Markers, suffix)
		}
		if showAll && t.Description != "" {
			line = fmt.Sprintf("%s — %s", line, t.Description)
		}
		fmt.Println(line)
	}
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
