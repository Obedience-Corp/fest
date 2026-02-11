// Package flow provides commands for managing festival status workflows.
package flow

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewFlowCommand creates the flow command group.
func NewFlowCommand() *cobra.Command {
	// Subcommands are added below
	cmd := &cobra.Command{
		Use:   "flow",
		Short: "Manage festival status workflow",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Long: `Manage the festival status workflow system.

The flow command provides tools for initializing, upgrading, and inspecting
the status workflow that organizes festivals through their lifecycle:

  planned → ready → active → dungeon/{completed,archived,someday}

Subcommands:
  upgrade    Migrate existing festivals/ to the extended workflow structure
  init       Initialize a new workflow in the current directory
  status     Show workflow status and item counts`,
	}

	cmd.AddCommand(newUpgradeCommand())

	return cmd
}
