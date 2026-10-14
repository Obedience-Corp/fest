package commands

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// deferralSurface is the CLI contract this festival changed, written out by
// hand. Regenerating a contract test from the code it tests makes it a rubber
// stamp, so every entry here is a deliberate line a reviewer reads.
var deferralSurface = map[string][]string{
	"fest task defer":   {"json", "reason", "yes"},
	"fest task blocked": {"deferred", "json", "list", "open", "reason", "tried", "yes"},
	"fest task unblock": {"json", "note"},
}

func findCommandByPath(root *cobra.Command, path string) *cobra.Command {
	found := (*cobra.Command)(nil)
	var walk func(*cobra.Command, string)
	walk = func(parent *cobra.Command, prefix string) {
		for _, child := range parent.Commands() {
			current := strings.TrimSpace(prefix + " " + child.Name())
			if current == path {
				found = child
			}
			walk(child, current)
		}
	}
	walk(root, root.Name())
	return found
}

func localFlagNames(cmd *cobra.Command) []string {
	var names []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		names = append(names, f.Name)
	})
	sort.Strings(names)
	return names
}

func TestDeferralCommandSurfaceFlags(t *testing.T) {
	for path, want := range deferralSurface {
		cmd := findCommandByPath(rootCmd, path)
		if cmd == nil {
			t.Fatalf("%q is not on the command surface", path)
		}
		if got := localFlagNames(cmd); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q flags = %v, want %v", path, got, want)
		}
	}
}

// Deferral now has the same explicit non-interactive confirmation as the other
// consequential task verbs; force remains a separate operator-only action.
func TestDeferHasNoninteractiveSurface(t *testing.T) {
	cmd := findCommandByPath(rootCmd, "fest task defer")
	if cmd == nil {
		t.Fatal("fest task defer is not on the command surface")
	}
	for _, required := range []string{"yes", "json", "reason"} {
		if cmd.Flags().Lookup(required) == nil {
			t.Errorf("fest task defer must expose --%s", required)
		}
	}
	if cmd.Flags().Lookup("force") != nil {
		t.Error("fest task defer must not expose --force")
	}
}
