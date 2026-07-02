package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/plugins"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
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

func newPluginsCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List discovered fest plugins",
		Long: `List fest plugins discovered from the active config repo manifest and PATH.

Any executable named fest-<name> on PATH is a fest plugin and runs as
"fest <name> [args...]". Plugins declared in the active user config repo
manifest (plugins/manifest.yml) carry richer metadata such as summaries.`,
		Example: `  fest plugins
  fest plugins --json`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsList(cmd, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func runPluginsList(cmd *cobra.Command, jsonOutput bool) error {
	discovery := plugins.NewPluginDiscovery()
	if err := discovery.DiscoverAll(); err != nil {
		return err
	}

	found := slices.Clone(discovery.Plugins())
	sort.Slice(found, func(i, j int) bool { return found[i].Command < found[j].Command })

	out := cmd.OutOrStdout()

	if jsonOutput {
		payload := map[string]any{
			"plugins": found,
			"count":   len(found),
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	if len(found) == 0 {
		_, _ = fmt.Fprintln(out, "No fest plugins found.")
		_, _ = fmt.Fprintln(out, "Install an executable named fest-<name> on PATH to extend fest.")
		return nil
	}

	_, _ = fmt.Fprintln(out, ui.Category("Installed Plugins:"))
	for _, p := range found {
		location := p.Path
		if location == "" {
			location = ui.Dim("(exec not found: " + p.Exec + ")")
		}
		_, _ = fmt.Fprintf(out, "  %-20s %s\n", ui.Accent(p.Command), location)
		if p.Summary != "" && p.Summary != "Plugin: "+p.Exec {
			_, _ = fmt.Fprintf(out, "  %-20s %s\n", "", ui.Dim(p.Summary))
		}
	}
	return nil
}
