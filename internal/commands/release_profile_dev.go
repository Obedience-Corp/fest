//go:build dev

package commands

import explorecmd "github.com/Obedience-Corp/fest/internal/commands/explore"

func registerReleaseProfileCommands() {
	exploreCmd := explorecmd.NewExploreCommand()
	exploreCmd.GroupID = "navigation"
	markDevOnlyCommand(exploreCmd)
	rootCmd.AddCommand(exploreCmd)
}
