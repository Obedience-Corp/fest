package release

import "github.com/spf13/cobra"

// Register attaches release-profile-specific commands to the root command.
// The actual registration is delegated to registerDev, which is a no-op in
// stable builds and registers dev-only commands in dev builds.
func Register(root *cobra.Command) {
	registerDev(root)
}
