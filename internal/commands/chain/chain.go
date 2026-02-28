// Package chain implements the `fest chain` command group for inter-festival
// dependency management.
package chain

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewChainCmd returns the parent `fest chain` command.
func NewChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chain",
		Short: "Manage festival chains (inter-festival dependencies)",
		Long:  "Create, validate, and track chains of dependent festivals.",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
	}

	cmd.AddCommand(
		newCreateCmd(),
		newListCmd(),
		newStatusCmd(),
		newValidateCmd(),
		newCheckCmd(),
	)

	return cmd
}
