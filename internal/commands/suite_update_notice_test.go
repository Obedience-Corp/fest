package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setFlagsFor parses args onto the real command tree and restores every flag
// it changed, so later tests in the package see default flag state.
func setFlagsFor(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd, rest, err := rootCmd.Find(args)
	if err != nil {
		t.Fatalf("Find(%v): %v", args, err)
	}
	if err := cmd.ParseFlags(rest); err != nil {
		t.Fatalf("ParseFlags(%v): %v", rest, err)
	}
	t.Cleanup(func() { resetChangedFlags(cmd) })
	return cmd
}

func resetChangedFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
}

func walkCommands(root *cobra.Command, visit func(*cobra.Command)) {
	for _, child := range root.Commands() {
		visit(child)
		walkCommands(child, visit)
	}
}

func TestSuiteNoticeEligibility(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"next"}, false},
		{[]string{"next", "--no-context"}, false},
		{[]string{"help"}, false},
		{[]string{"completion", "zsh"}, false},
		{[]string{"shell-init", "zsh"}, false},
		{[]string{"version"}, false},
		{[]string{"version", "--check"}, false},
		{[]string{"gendocs"}, false},
		{[]string{"status", "--json"}, false},
		{[]string{"status", "list", "--json"}, false},
		{[]string{"go", "--json"}, false},
		{[]string{"go", "completions"}, false},
		{[]string{"show", "pick"}, false},
		{[]string{"show", "pick", "--print-path"}, false},
		{[]string{"promote", "completions"}, false},
		{[]string{"parse"}, false},
		{[]string{"chain", "graph", "--mermaid"}, false},
		{[]string{"create", "festival", "--agent"}, false},
		{[]string{"status"}, true},
		{[]string{"status", "list"}, true},
		{[]string{"go"}, true},
		{[]string{"go", "my-festival"}, true},
		{[]string{"list"}, true},
		{[]string{"show"}, true},
		{[]string{"chain", "graph"}, true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			cmd := setFlagsFor(t, tt.args...)
			if got := suiteNoticeEligible(cmd); got != tt.want {
				t.Fatalf("suiteNoticeEligible(%s) = %v, want %v", cmd.CommandPath(), got, tt.want)
			}
		})
	}
}

func TestSuiteNoticeRootAndNilAreIneligible(t *testing.T) {
	if suiteNoticeEligible(nil) {
		t.Fatal("nil command is eligible")
	}
	if suiteNoticeEligible(rootCmd) {
		t.Fatal("root command is eligible")
	}
}

// Every machine-output flag in the tree, set, makes its command ineligible.
// This keeps the suppress list true as commands gain --json or --agent.
func TestSuiteNoticeMachineFlagsSuppressAcrossTree(t *testing.T) {
	checked := 0
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		for _, name := range suiteMachineOutputFlags {
			f := cmd.Flags().Lookup(name)
			if f == nil || f.Value.Type() != "bool" {
				continue
			}
			if err := f.Value.Set("true"); err != nil {
				t.Fatalf("%s --%s: %v", cmd.CommandPath(), name, err)
			}
			f.Changed = true
			if suiteNoticeEligible(cmd) {
				t.Errorf("%s --%s is eligible for the suite notice", cmd.CommandPath(), name)
			}
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
			checked++
		}
		if f := cmd.Flags().Lookup("format"); f != nil && suiteMachineFormats[f.DefValue] && suiteNoticeEligible(cmd) {
			t.Errorf("%s defaults to --format %s and is eligible", cmd.CommandPath(), f.DefValue)
		}
		if (cmd.Name() == "completions" || cmd.Name() == "pick") && suiteNoticeEligible(cmd) {
			t.Errorf("shell helper %s is eligible", cmd.CommandPath())
		}
	})
	if checked < 50 {
		t.Fatalf("checked %d machine-output flags; the tree walk is not reaching the command surface", checked)
	}
}

// Cobra runs only the nearest PersistentPostRun. A subcommand that defined
// its own would silently skip the root notice hook for its whole subtree.
func TestSuiteNoticeHookIsNotShadowed(t *testing.T) {
	if rootCmd.PersistentPostRun == nil {
		t.Fatal("root PersistentPostRun is not wired")
	}
	if cobra.EnableTraverseRunHooks {
		t.Fatal("EnableTraverseRunHooks changes which post-run hooks fire; revisit the notice wiring")
	}
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		if cmd.PersistentPostRun != nil || cmd.PersistentPostRunE != nil {
			t.Errorf("%s defines a PersistentPostRun that shadows the suite notice hook", cmd.CommandPath())
		}
	})
	if rootCmd.PersistentPostRunE != nil {
		t.Error("root PersistentPostRunE would take precedence over the notice hook")
	}
}
