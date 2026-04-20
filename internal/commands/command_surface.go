package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	commandSurfaceCmd := &cobra.Command{
		Use:    "__commands",
		Short:  "Output visible CLI command paths",
		Hidden: true,
		Annotations: map[string]string{
			"scope": "global",
		},
		RunE: runCommandSurface,
	}
	rootCmd.AddCommand(commandSurfaceCmd)
}

func runCommandSurface(cmd *cobra.Command, args []string) error {
	for _, path := range collectVisibleCommandPaths(cmd.Root()) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), path); err != nil {
			return err
		}
	}
	return nil
}

func collectVisibleCommandPaths(root *cobra.Command) []string {
	paths := make([]string, 0, len(root.Commands()))
	walkVisibleCommandPaths(root, root.Name(), &paths)
	sort.Strings(paths)
	return paths
}

func walkVisibleCommandPaths(parent *cobra.Command, prefix string, paths *[]string) {
	for _, child := range parent.Commands() {
		if shouldSkipCommandSurfaceEntry(child) {
			continue
		}

		path := strings.TrimSpace(prefix + " " + child.Name())
		*paths = append(*paths, path)
		walkVisibleCommandPaths(child, path, paths)
	}
}

func shouldSkipCommandSurfaceEntry(cmd *cobra.Command) bool {
	if cmd.Hidden {
		return true
	}

	return cmd.Name() == "help"
}
