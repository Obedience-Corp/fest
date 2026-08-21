package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const helpEmDash = '\u2014'

func TestCobraHelpHasNoEmDash(t *testing.T) {
	hits := cobraHelpEmDashHits(rootCmd)
	if len(hits) > 0 {
		t.Fatalf("em dash (U+2014) in cobra help strings that leak into generated CLI docs:\n  %s", strings.Join(hits, "\n  "))
	}
}

func TestCobraHelpEmDashHitsDetectsShortLongExampleAndFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:     "demo",
		Short:   "short — hit",
		Long:    "long — hit",
		Example: "example — hit",
	}
	cmd.Flags().String("author", "", "flag — hit")

	hits := cobraHelpEmDashHits(cmd)
	if len(hits) != 4 {
		t.Fatalf("hits = %v, want 4", hits)
	}
}

func cobraHelpEmDashHits(root *cobra.Command) []string {
	var hits []string
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		path := cmd.CommandPath()
		for _, item := range []struct {
			kind string
			text string
		}{
			{"Short", cmd.Short},
			{"Long", cmd.Long},
			{"Example", cmd.Example},
		} {
			if strings.ContainsRune(item.text, helpEmDash) {
				hits = append(hits, fmt.Sprintf("%s %s", path, item.kind))
			}
		}
		seen := make(map[string]struct{})
		visit := func(f *pflag.Flag) {
			if _, ok := seen[f.Name]; ok {
				return
			}
			seen[f.Name] = struct{}{}
			if strings.ContainsRune(f.Usage, helpEmDash) {
				hits = append(hits, fmt.Sprintf("%s flag --%s", path, f.Name))
			}
		}
		cmd.LocalFlags().VisitAll(visit)
		cmd.PersistentFlags().VisitAll(visit)
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return hits
}
