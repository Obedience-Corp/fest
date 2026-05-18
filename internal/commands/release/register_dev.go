//go:build dev

package release

import (
	explorecmd "github.com/Obedience-Corp/fest/internal/commands/explore"
	"github.com/spf13/cobra"
)

func registerDev(root *cobra.Command) {
	exploreCmd := explorecmd.NewExploreCommand()
	exploreCmd.GroupID = "navigation"
	MarkDevOnly(exploreCmd)
	root.AddCommand(exploreCmd)
}
