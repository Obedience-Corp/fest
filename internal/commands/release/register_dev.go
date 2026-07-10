//go:build dev

package release

import (
	explorecmd "github.com/Obedience-Corp/fest/internal/commands/explore"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/spf13/cobra"
)

func registerDev(root *cobra.Command) {
	exploreCmd := explorecmd.NewExploreCommand()
	exploreCmd.GroupID = "navigation"
	MarkDevOnly(exploreCmd)
	root.AddCommand(exploreCmd)

	// fest tui is a create-only menu with near-zero human uptake; the public
	// create surface is bare `fest create` (same forms). Kept in the dev
	// channel as a seed for a future terminal management UI.
	if shared.NewTUICommand != nil {
		tuiCmd := shared.NewTUICommand()
		tuiCmd.GroupID = "creation"
		MarkDevOnly(tuiCmd)
		root.AddCommand(tuiCmd)
	}
}
