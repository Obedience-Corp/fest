package chain

import (
	"context"
	"fmt"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newGraphCmd() *cobra.Command {
	var (
		mermaid bool
		live    bool
	)

	cmd := &cobra.Command{
		Use:   "graph <chain-id>",
		Short: "Visualize chain dependency graph",
		Long:  "Render the chain's dependency graph as ASCII waves or Mermaid diagram syntax.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraph(cmd.Context(), args[0], mermaid, live)
		},
	}

	cmd.Flags().BoolVar(&mermaid, "mermaid", false, "output Mermaid diagram syntax")
	cmd.Flags().BoolVar(&live, "live", false, "annotate nodes with live festival statuses")

	return cmd
}

func runGraph(ctx context.Context, chainID string, mermaid, live bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, _, err := findChainByID(ctx, chainID)
	if err != nil {
		return err
	}

	// Optionally resolve live statuses.
	var statuses map[string]chainpkg.FestivalStatus
	if live {
		statuses, _ = resolveChainStatuses(ctx, c)
	}

	if mermaid {
		return renderMermaid(c, statuses)
	}
	return renderASCII(c, statuses)
}

// renderASCII outputs a wave-grouped ASCII graph with an explicit edge list.
func renderASCII(c *chainpkg.Chain, statuses map[string]chainpkg.FestivalStatus) error {
	fmt.Printf("%s %s (%s)\n\n", ui.Label("Chain:"), c.Metadata.Name, c.Metadata.ID)

	if len(c.Waves) > 0 {
		for _, w := range c.Waves {
			fmt.Printf("Wave %d: %s\n", w.ID, w.Name)
			for _, ref := range w.Festivals {
				node := c.FestivalByRef(ref)
				if node == nil {
					continue
				}
				line := fmt.Sprintf("  [%s] %s %s", node.Ref, node.ID, node.Name)
				if statuses != nil {
					line += "  " + statusIcon(statuses[ref])
				}
				fmt.Println(line)
			}
			fmt.Println()
		}
	} else {
		// No waves — list all festivals in topological order.
		dag, err := chainpkg.BuildDAG(ctx(), c)
		if err == nil {
			sorted, sortErr := dag.TopologicalSort(ctx())
			if sortErr == nil {
				fmt.Println("Festivals (topological order):")
				for _, ref := range sorted {
					node := c.FestivalByRef(ref)
					if node == nil {
						continue
					}
					line := fmt.Sprintf("  [%s] %s %s", node.Ref, node.ID, node.Name)
					if statuses != nil {
						line += "  " + statusIcon(statuses[ref])
					}
					fmt.Println(line)
				}
				fmt.Println()
			}
		}
	}

	// Edge list.
	if len(c.Edges) > 0 {
		fmt.Println("Dependencies:")
		for _, e := range c.Edges {
			arrow := "──hard──>"
			if e.Type == chainpkg.EdgeSoft {
				arrow = "··soft··>"
			}
			line := fmt.Sprintf("  %s %s %s", e.From, arrow, e.To)
			if e.Note != "" {
				line += fmt.Sprintf("  (%s)", e.Note)
			}
			fmt.Println(line)
		}
	}

	return nil
}

// ctx returns a background context for DAG operations in render functions.
func ctx() context.Context {
	return context.Background()
}

// renderMermaid outputs Mermaid graph TD syntax.
func renderMermaid(c *chainpkg.Chain, statuses map[string]chainpkg.FestivalStatus) error {
	fmt.Println("graph TD")

	// Define nodes with labels.
	for _, f := range c.Festivals {
		label := fmt.Sprintf("%s %s", f.ID, f.Name)
		if statuses != nil {
			label += " " + statusIcon(statuses[f.Ref])
		}
		fmt.Printf("    %s[\"%s\"]\n", sanitizeMermaidID(f.Ref), label)
	}

	fmt.Println()

	// Define edges.
	for _, e := range c.Edges {
		from := sanitizeMermaidID(e.From)
		to := sanitizeMermaidID(e.To)
		if e.Type == chainpkg.EdgeHard {
			fmt.Printf("    %s --> %s\n", from, to)
		} else {
			fmt.Printf("    %s -.-> %s\n", from, to)
		}
	}

	// Style waves with subgraphs if present.
	if len(c.Waves) > 0 {
		fmt.Println()
		for _, w := range c.Waves {
			fmt.Printf("    subgraph Wave_%d[\"%s\"]\n", w.ID, w.Name)
			for _, ref := range w.Festivals {
				fmt.Printf("        %s\n", sanitizeMermaidID(ref))
			}
			fmt.Println("    end")
		}
	}

	return nil
}

// sanitizeMermaidID replaces characters that aren't valid in Mermaid node IDs.
func sanitizeMermaidID(s string) string {
	return strings.NewReplacer("-", "_", " ", "_").Replace(s)
}
