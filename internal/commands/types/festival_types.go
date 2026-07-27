package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newFestivalCmd() *cobra.Command {
	var (
		jsonOutput bool
		phasesOnly bool
	)

	cmd := &cobra.Command{
		Use:   "festival [type-name]",
		Short: "Discover festival types",
		Long: `Discover available festival workflow types and their phase structures.

These are the values for 'fest create festival --type'. Built-in types:
  - standard: Full planning and implementation phases (default)
  - implementation: Direct implementation without planning
  - research: Research-focused workflow
  - ritual: Custom structure defined by the festival

Examples:
  fest types festival                    # List all festival types
  fest types festival list               # Same as above
  fest types festival standard           # Show details for standard type
  fest types festival show implementation # Show details for implementation type
  fest types festival --phases standard  # Show only phases for standard type
  fest types festival --json             # Machine-readable JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args or "list" arg means list
			if len(args) == 0 {
				return runFestivalList(cmd.Context(), jsonOutput)
			}

			// "list" subcommand
			if args[0] == "list" {
				return runFestivalList(cmd.Context(), jsonOutput)
			}

			// "show" subcommand or direct type name
			typeName := args[0]
			if typeName == "show" && len(args) > 1 {
				typeName = args[1]
			}

			return runFestivalShow(cmd.Context(), typeName, jsonOutput, phasesOnly)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&phasesOnly, "phases", false, "Show only phases for the specified type")

	// Add subcommands for explicit list and show
	cmd.AddCommand(newFestivalListCmd())
	cmd.AddCommand(newFestivalShowCmd())

	return cmd
}

func newFestivalListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all festival types",
		Long: `List all available festival types with their descriptions.

Shows all festival types defined in the configuration, marking the default type.

Examples:
  fest types festival list        # List all festival types
  fest types festival list --json # JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFestivalList(cmd.Context(), jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func newFestivalShowCmd() *cobra.Command {
	var (
		jsonOutput bool
		phasesOnly bool
	)

	cmd := &cobra.Command{
		Use:   "show <type-name>",
		Short: "Show details for a festival type",
		Long: `Display detailed information about a specific festival type.

Shows the type's description, phase structure, auto-scaffolded phases,
and manually-created phases.

Examples:
  fest types festival show standard           # Show standard type details
  fest types festival show implementation     # Show implementation type
  fest types festival show standard --phases  # Show only phases
  fest types festival show research --json    # JSON output`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFestivalShow(cmd.Context(), args[0], jsonOutput, phasesOnly)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&phasesOnly, "phases", false, "Show only phases")

	return cmd
}

// runFestivalList lists all festival types.
func runFestivalList(ctx context.Context, jsonOutput bool) error {
	config, err := types.LoadFestivalTypesConfig(ctx)
	if err != nil {
		return err
	}

	if jsonOutput {
		return outputFestivalListJSON(config)
	}

	return outputFestivalListTable(config)
}

// runFestivalShow shows details for a specific festival type.
func runFestivalShow(ctx context.Context, typeName string, jsonOutput, phasesOnly bool) error {
	config, err := types.LoadFestivalTypesConfig(ctx)
	if err != nil {
		return err
	}

	festType, err := config.GetFestivalType(typeName)
	if err != nil {
		return err
	}

	if phasesOnly {
		if jsonOutput {
			return outputPhasesOnlyJSON(festType)
		}
		return outputPhasesOnlyTable(festType)
	}

	if jsonOutput {
		return outputFestivalShowJSON(festType)
	}

	return outputFestivalShowTable(festType)
}

// outputFestivalListJSON outputs the festival list as JSON.
func outputFestivalListJSON(config *types.FestivalTypesConfig) error {
	data, err := json.MarshalIndent(config.Types, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal festival types to JSON")
	}
	fmt.Println(string(data))
	return nil
}

// outputFestivalListTable outputs the festival list as a human-readable table.
func outputFestivalListTable(config *types.FestivalTypesConfig) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	if len(config.Types) == 0 {
		display.Info("No festival types configured.")
		return nil
	}

	fmt.Println("Festival Types:")
	fmt.Println()
	fmt.Printf("%-20s %-10s %-10s %s\n", "NAME", "AUTO", "PENDING", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 80))

	for _, festType := range config.Types {
		autoCount := len(festType.GetAutoPhases())
		pendingCount := len(festType.GetPendingPhases())

		name := festType.Name
		if festType.Default {
			name += " (default)"
		}

		desc := festType.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		fmt.Printf("%-20s %-10d %-10d %s\n", name, autoCount, pendingCount, desc)
	}

	fmt.Println()
	return nil
}

// outputFestivalShowJSON outputs a single festival type as JSON.
func outputFestivalShowJSON(festType *types.FestivalType) error {
	data, err := json.MarshalIndent(festType, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal festival type to JSON")
	}
	fmt.Println(string(data))
	return nil
}

// outputFestivalShowTable outputs a single festival type as a human-readable display.
func outputFestivalShowTable(festType *types.FestivalType) error {
	// Header
	fmt.Printf("Festival Type: %s\n", festType.Name)
	if festType.Default {
		fmt.Println("Default: Yes")
	}
	if festType.SkipIngestion {
		fmt.Println("Skip Ingestion: Yes")
	}
	fmt.Printf("Description: %s\n", festType.Description)

	if festType.Note != "" {
		fmt.Printf("Note: %s\n", festType.Note)
	}

	fmt.Println()

	// Auto phases
	autoPhases := festType.GetAutoPhases()
	if len(autoPhases) > 0 {
		fmt.Println("Auto-Scaffolded Phases:")
		for _, phase := range autoPhases {
			fmt.Printf("  • %s (type: %s)\n", phase.Name, phase.Type)
			if phase.Role != "" {
				fmt.Printf("    Role: %s\n", phase.Role)
			}
			if phase.Trigger != "" {
				fmt.Printf("    Trigger: %s\n", phase.Trigger)
			}
			if phase.Generator != "" {
				fmt.Printf("    Generator: %s\n", phase.Generator)
			}
		}
		fmt.Println()
	}

	// Pending phases
	pendingPhases := festType.GetPendingPhases()
	if len(pendingPhases) > 0 {
		fmt.Println("Manual Phases (created as needed):")
		for _, phase := range pendingPhases {
			fmt.Printf("  • %s (type: %s)\n", phase.Name, phase.Type)
			if phase.Role != "" {
				fmt.Printf("    Role: %s\n", phase.Role)
			}
		}
		fmt.Println()
	}

	// Usage example
	fmt.Println("Example Usage:")
	fmt.Printf("  fest create festival --type %s <festival-name>\n\n", festType.Name)

	return nil
}

// outputPhasesOnlyJSON outputs just the phases as JSON.
func outputPhasesOnlyJSON(festType *types.FestivalType) error {
	data, err := json.MarshalIndent(festType.Phases, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal phases to JSON")
	}
	fmt.Println(string(data))
	return nil
}

// outputPhasesOnlyTable outputs just the phases as a human-readable table.
func outputPhasesOnlyTable(festType *types.FestivalType) error {
	fmt.Printf("Phases for %s:\n\n", festType.Name)

	if len(festType.Phases) == 0 {
		fmt.Println("  No phases defined")
		return nil
	}

	fmt.Printf("%-20s %-20s %-8s\n", "NAME", "TYPE", "AUTO")
	fmt.Println(strings.Repeat("-", 50))

	for _, phase := range festType.Phases {
		auto := "No"
		if phase.Auto {
			auto = "Yes"
		}
		fmt.Printf("%-20s %-20s %-8s\n", phase.Name, phase.Type, auto)
		if phase.Role != "" {
			fmt.Printf("  Role: %s\n", phase.Role)
		}
		if phase.Trigger != "" {
			fmt.Printf("  Trigger: %s\n", phase.Trigger)
		}
		if phase.Generator != "" {
			fmt.Printf("  Generator: %s\n", phase.Generator)
		}
	}

	fmt.Println()
	return nil
}
