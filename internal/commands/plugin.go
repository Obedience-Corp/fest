package commands

import (
	"errors"
	"os"
	"slices"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/plugins"
)

var errPluginHandled = errors.New("plugin handled")

func dispatchPlugin() error {
	name, argIdx := firstSubcommand()
	if name == "" {
		return nil
	}

	switch name {
	case "help", "completion", "__complete", "__completeNoDesc":
		return nil
	}

	if isKnownCommand(name) {
		return nil
	}

	dispatcher := plugins.NewDispatcher()
	if err := dispatcher.Initialize(); err != nil {
		return nil
	}

	var pluginArgs []string
	if argIdx < len(os.Args) {
		pluginArgs = os.Args[argIdx:]
	}

	if err := dispatcher.Dispatch(pluginArgs); err != nil {
		if festerrors.Is(err, festerrors.ErrCodeNotFound) {
			return nil
		}
		return err
	}
	return errPluginHandled
}

func firstSubcommand() (string, int) {
	return findFirstPositionalArg(os.Args)
}

// findFirstPositionalArg returns the first positional (non-flag) arg and its
// index. It consults rootCmd's persistent flags to skip flag values like
// --config <file> that would otherwise look like subcommand names.
func findFirstPositionalArg(args []string) (string, int) {
	if len(args) < 2 {
		return "", 0
	}

	pflags := rootCmd.PersistentFlags()

	for i := 1; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1], i + 1
			}
			return "", 0
		}

		if len(arg) == 0 || arg[0] != '-' {
			return arg, i
		}

		if strings.Contains(arg, "=") {
			eqName := strings.TrimPrefix(strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--"), "-")
			if pflags.Lookup(eqName) == nil {
				return "", 0
			}
			continue
		}

		flagName := strings.TrimPrefix(strings.TrimPrefix(arg, "--"), "-")
		f := pflags.Lookup(flagName)
		if f == nil {
			// Unknown flag — bail out and let Cobra report the error.
			return "", 0
		}
		if f.NoOptDefVal == "" {
			i++
		}
	}
	return "", 0
}

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
