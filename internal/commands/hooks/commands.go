package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewHooksCommand creates the hooks inspection command group.
func NewHooksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Inspect resolved lifecycle hooks",
		Long: `Inspect the hooks resolved from the machine, festivals, and festival layers.

Available Commands:
  list   Show the effective resolved hook set with source layers and shadow diffs`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
	}
	cmd.AddCommand(newHooksListCmd())
	return cmd
}

func newHooksListCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the effective resolved hook set",
		Example: `  fest hooks list
  fest hooks list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHooksList(cmd.Context(), jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	return cmd
}

type hooksListJSON struct {
	Enabled bool                    `json:"enabled"`
	Levels  map[string]bool         `json:"levels"`
	Hooks   []hooksListHookJSON     `json:"hooks"`
	Legacy  *hooksListLegacyJSON    `json:"legacy_alias,omitempty"`
}

type hooksListHookJSON struct {
	Name     string                   `json:"name"`
	Source   string                   `json:"source"`
	Enabled  bool                     `json:"enabled"`
	Command  string                   `json:"command"`
	Fail     string                   `json:"fail"`
	Timeout  string                   `json:"timeout"`
	Evidence string                   `json:"evidence"`
	Shadows  []hooksListShadowJSON    `json:"shadows"`
}

type hooksListShadowJSON struct {
	Source  string `json:"source"`
	Command string `json:"command"`
	Differs bool   `json:"differs"`
}

type hooksListLegacyJSON struct {
	Active  bool   `json:"active"`
	Command string `json:"command,omitempty"`
}

func runHooksList(ctx context.Context, jsonOutput bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	festivalPath, err := resolveFestivalPath(ctx)
	if err != nil {
		return err
	}
	eff, err := hooks.LoadAndResolve(ctx, festivalPath)
	if err != nil {
		return festerrors.Wrap(err, "resolving hooks")
	}
	view := buildHooksListView(eff)
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(view)
	}
	printHooksListText(view)
	return nil
}

func resolveFestivalPath(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", festerrors.IO("getting working directory", err)
	}
	path, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return "", festerrors.Wrap(err, "resolving festival path").
			WithHint("run from a festival directory or a linked project")
	}
	return path, nil
}

func buildHooksListView(eff *hooks.Effective) hooksListJSON {
	view := hooksListJSON{
		Enabled: true,
		Levels:  map[string]bool{},
		Hooks:   []hooksListHookJSON{},
	}
	if eff == nil {
		return view
	}
	view.Enabled = eff.Enabled
	for k, v := range eff.Levels {
		view.Levels[k] = v
	}
	if eff.LegacyAliasActive {
		view.Legacy = &hooksListLegacyJSON{Active: true, Command: eff.LegacyAliasCommand}
	}
	names := make([]string, 0, len(eff.Hooks))
	for name := range eff.Hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h := eff.Hooks[name]
		entry := hooksListHookJSON{
			Name:     name,
			Source:   string(h.Source),
			Enabled:  h.Enabled,
			Command:  h.Command,
			Fail:     string(h.Fail),
			Timeout:  h.Timeout.String(),
			Evidence: string(h.Evidence),
			Shadows:  []hooksListShadowJSON{},
		}
		for _, s := range h.Shadowed {
			entry.Shadows = append(entry.Shadows, hooksListShadowJSON{
				Source:  string(s.Source),
				Command: s.Def.Command,
				Differs: true,
			})
		}
		view.Hooks = append(view.Hooks, entry)
	}
	return view
}

func printHooksListText(view hooksListJSON) {
	fmt.Printf("hooks.enabled: %v\n", view.Enabled)
	if len(view.Levels) > 0 {
		keys := make([]string, 0, len(view.Levels))
		for k := range view.Levels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, view.Levels[k]))
		}
		fmt.Printf("hooks.levels: %s\n", strings.Join(parts, " "))
	}
	if view.Legacy != nil && view.Legacy.Active {
		fmt.Printf("legacy alias: approval_judge.command=%q (timeout=0)\n", view.Legacy.Command)
	}
	if len(view.Hooks) == 0 {
		fmt.Println("No hooks configured. Declare hooks under hooks.definitions or run `fest hooks list` after configuring.")
		return
	}
	fmt.Println()
	for _, h := range view.Hooks {
		enabled := "enabled"
		if !h.Enabled {
			enabled = "disabled"
		}
		fmt.Printf("%s   [%s]   %s   fail=%s  timeout=%s   evidence=%s\n",
			h.Name, h.Source, enabled, h.Fail, h.Timeout, h.Evidence)
		fmt.Printf("  command: %s\n", h.Command)
		if len(h.Shadows) > 0 {
			fmt.Println("  shadows:")
			for _, s := range h.Shadows {
				fmt.Printf("    [%s]  command=%q           (differs)\n", s.Source, s.Command)
			}
		}
	}
}
