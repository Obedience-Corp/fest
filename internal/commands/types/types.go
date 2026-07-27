// Package types implements the fest types command for template type discovery.
package types

import (
	"github.com/spf13/cobra"
)

// NewTypesCommand creates the types command group.
func NewTypesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "types",
		Short: "Discover types for fest create",
		Long: `List festival, phase, sequence, and task types available for create.

Festival workflow types (standard, implementation, research, ritual) come from
festival_types.yaml. Phase scaffold types come from the methodology templates
tree under festivals/.festival/templates/phases/.

Examples:
  fest types                             # Same as fest types list
  fest types list --level festival       # Values for create festival --type
  fest types list --level phase          # Values for create phase --type
  fest types show standard               # Festival workflow type details
  fest types show implementation --level phase
  fest types festival                    # Festival workflow types (alias)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to list when no subcommand is given.
			return runList(cmd.Context(), "", false, false, false)
		},
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newFestivalCmd())

	return cmd
}
