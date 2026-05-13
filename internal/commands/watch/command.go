// Package watch implements the dev-gated fest watch command.
package watch

import (
	"errors"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewWatchCommand creates the watch command.
func NewWatchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watch [festival-selector]",
		Short: "Watch in-progress festival state",
		Long: `Watch the in-progress state of a festival.

With no selector, fest watch resolves the current festival context or linked
project context. From broader workspace context it opens a festival picker.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"scope":        string(scope.Workspace),
			"interactive":  "true",
			"long_running": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("fest watch is not implemented yet")
		},
	}
}
