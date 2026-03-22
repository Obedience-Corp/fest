package commands

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/Obedience-Corp/fest/internal/plugins"
)

// errPluginHandled is a sentinel indicating a plugin ran successfully.
// Execute() converts this to nil so main() sees a clean exit.
var errPluginHandled = errors.New("plugin handled")

// dispatchPlugin checks whether the first non-flag argument is an unknown
// subcommand backed by a fest-<name> binary on PATH (or in a manifest).
// If so, it executes the plugin and returns errPluginHandled (success) or
// the execution error. Returns nil to fall through to Cobra's normal dispatch.
//
// Global flags appearing before the plugin name (e.g. --verbose, --config)
// are consumed by fest's own arg scanner and are NOT forwarded to the plugin.
// Only arguments after the plugin name are passed through. This matches git's
// plugin convention.
func dispatchPlugin() error {
	name, argIdx := firstSubcommand()
	if name == "" {
		return nil
	}

	// Never intercept help, completion, or Cobra internals.
	switch name {
	case "help", "completion", "__complete", "__completeNoDesc":
		return nil
	}

	if isKnownCommand(name) {
		return nil
	}

	// Initialize plugin discovery and check for a matching plugin.
	dispatcher := plugins.NewDispatcher()
	if err := dispatcher.Initialize(); err != nil {
		return nil // Discovery failure is not fatal; fall through to Cobra.
	}

	// Build the args slice starting from the plugin name onward.
	var pluginArgs []string
	if argIdx < len(os.Args) {
		pluginArgs = os.Args[argIdx:]
	}

	if !dispatcher.CanHandle(pluginArgs) {
		return nil
	}

	if err := dispatcher.Dispatch(pluginArgs); err != nil {
		return err
	}
	return errPluginHandled
}

// firstSubcommand returns the first non-flag argument from os.Args and its
// index. Returns ("", 0) if no subcommand is present.
//
// It consults the root command's persistent flags so that flags which take
// values (e.g. --config <file>) have their value skipped rather than being
// mistaken for a subcommand name.
func firstSubcommand() (string, int) {
	return findFirstPositionalArg(os.Args)
}

// findFirstPositionalArg scans args (where args[0] is the program name) and
// returns the first positional (non-flag) argument and its index.
// It correctly skips values consumed by flags like --config <file> and
// handles --flag=value, --, and boolean flags.
func findFirstPositionalArg(args []string) (string, int) {
	if len(args) < 2 {
		return "", 0
	}

	pflags := rootCmd.PersistentFlags()

	for i := 1; i < len(args); i++ {
		arg := args[i]

		// "--" terminates flag parsing; the next arg is the first positional.
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1], i + 1
			}
			return "", 0
		}

		if len(arg) == 0 || arg[0] != '-' {
			return arg, i
		}

		// It's a flag. Determine whether it consumes the next argument.
		// Flags using --flag=value syntax never consume the next arg.
		if strings.Contains(arg, "=") {
			continue
		}

		// Strip leading dashes to get the flag name.
		flagName := strings.TrimPrefix(strings.TrimPrefix(arg, "--"), "-")

		// Look up the flag in persistent flags to check if it takes a value.
		if f := pflags.Lookup(flagName); f != nil && f.NoOptDefVal == "" {
			// Flag takes a value — skip the next argument.
			i++
			continue
		}
	}
	return "", 0
}

// isKnownCommand reports whether name matches a registered Cobra subcommand
// (by name or alias).
func isKnownCommand(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}
		if slices.Contains(cmd.Aliases, name) {
			return true
		}
	}
	return false
}
