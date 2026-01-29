// Package context provides the fest context command for agent context loading.
package context

import (
	"fmt"

	ctx "github.com/Obedience-Corp/fest/internal/context"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	verbose    bool
	depth      string
	taskName   string
)

// NewContextCommand creates the context command
func NewContextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Get context for the current location or task",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Provides AI agents with context for the current location in a festival.

Context includes:
  - Festival, phase, and sequence goals
  - Relevant rules from FESTIVAL_RULES.md
  - Recent decisions from CONTEXT.md
  - Dependency outputs (what prior tasks produced)

Depth levels:
  minimal   - Immediate goals, dependencies, autonomy level
  standard  - + Rules, recent decisions (5)
  full      - + All decisions, dependency outputs

Examples:
  fest context                    # Context for current location
  fest context --depth full       # Full context with all details
  fest context --task my_task     # Context for a specific task
  fest context --json             # Output as JSON
  fest context --verbose          # Explanatory output for agents`,
		RunE: runContext,
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "output with explanatory text for agents")
	cmd.Flags().StringVar(&depth, "depth", "standard", "context depth (minimal, standard, full)")
	cmd.Flags().StringVar(&taskName, "task", "", "get context for a specific task")

	return cmd
}

func runContext(cmd *cobra.Command, args []string) error {
	// Get festival path from scope context (resolved by PersistentPreRunE)
	festivalPath, ok := scope.FestivalFrom(cmd.Context())
	if !ok || festivalPath == "" {
		return errors.NotFound("festival context").
			WithHint("The scope system should have resolved a festival path")
	}

	// Validate depth
	d := ctx.Depth(depth)
	switch d {
	case ctx.DepthMinimal, ctx.DepthStandard, ctx.DepthFull:
		// Valid
	default:
		return errors.Validation("invalid depth").
			WithField("depth", depth).
			WithField("valid", "minimal, standard, full")
	}

	// Build context
	builder := ctx.NewBuilder(festivalPath, d)

	var output *ctx.ContextOutput
	var err error
	if taskName != "" {
		output, err = builder.BuildForTask(taskName)
	} else {
		output, err = builder.Build()
	}

	if err != nil {
		return errors.Wrap(err, "building context")
	}

	// Format and output
	formatter := ctx.NewFormatter(verbose)

	if jsonOutput {
		jsonStr, err := formatter.FormatJSON(output)
		if err != nil {
			return errors.Parse("formatting JSON", err)
		}
		fmt.Println(jsonStr)
	} else if verbose {
		fmt.Print(formatter.FormatVerbose(output))
	} else {
		fmt.Print(formatter.FormatText(output))
	}

	return nil
}
