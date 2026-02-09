// Package scaffold provides commands for generating festival structures from plan documents.
package scaffold

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewScaffoldCommand creates the scaffold command group.
func NewScaffoldCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Generate festival structures from plans",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Long: `Generate complete festival structures from plan documents.

The scaffold command reads a markdown plan document (like STRUCTURE.md) and
generates the corresponding festival directory structure with phases, sequences,
and tasks pre-populated from the plan.

Examples:
  fest scaffold from-plan --plan path/to/STRUCTURE.md --name my-festival
  fest scaffold from-plan --plan STRUCTURE.md --dry-run
  fest scaffold from-plan --plan STRUCTURE.md --name my-fest --json`,
	}

	cmd.AddCommand(newFromPlanCmd())

	return cmd
}
