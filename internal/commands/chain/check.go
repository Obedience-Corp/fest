package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var chainID string

	cmd := &cobra.Command{
		Use:   "check <ref-or-id>",
		Short: "Check if a festival is unblocked within its chain",
		Long:  "Quick check whether a specific festival's hard dependencies are met.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd.Context(), args[0], chainID)
		},
	}

	cmd.Flags().StringVar(&chainID, "chain", "", "specify chain ID for disambiguation")

	return cmd
}

func runCheck(ctx context.Context, refOrID, chainIDFlag string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Find the chain containing this festival ref.
	c, ref, err := findChainForFestival(ctx, refOrID, chainIDFlag)
	if err != nil {
		return err
	}

	node := c.FestivalByRef(ref)
	if node == nil {
		return fmt.Errorf("festival ref %q not found in chain %s", ref, c.Metadata.ID)
	}

	// Collect hard upstream deps.
	var hardUpstream []chainpkg.Edge
	for _, e := range c.Edges {
		if e.To == ref && e.Type == chainpkg.EdgeHard {
			hardUpstream = append(hardUpstream, e)
		}
	}

	if len(hardUpstream) == 0 {
		fmt.Printf("%s (%s) %s — no incoming hard edges\n",
			ref, node.ID, ui.Accent("UNBLOCKED"))
		return nil
	}

	// Without live festival statuses, report the dependency structure.
	fmt.Printf("%s (%s) has %d hard dependencies:\n", ref, node.ID, len(hardUpstream))
	for _, e := range hardUpstream {
		upstream := c.FestivalByRef(e.From)
		if upstream != nil {
			fmt.Printf("  %s (%s): %s\n", e.From, upstream.ID, upstream.Name)
		}
	}

	return nil
}

// findChainForFestival finds the chain containing a given ref or festival ID.
func findChainForFestival(ctx context.Context, refOrID, chainIDFlag string) (*chainpkg.Chain, string, error) {
	if chainIDFlag != "" {
		c, _, err := findChainByID(ctx, chainIDFlag)
		if err != nil {
			return nil, "", err
		}
		// Try as ref first, then as festival ID.
		if node := c.FestivalByRef(refOrID); node != nil {
			return c, refOrID, nil
		}
		for _, f := range c.Festivals {
			if f.ID == refOrID {
				return c, f.Ref, nil
			}
		}
		return nil, "", fmt.Errorf("festival %q not found in chain %s", refOrID, chainIDFlag)
	}

	// Search all chains for the ref or ID.
	chains, err := discoverAllChains(ctx)
	if err != nil {
		return nil, "", err
	}

	for _, c := range chains {
		if node := c.FestivalByRef(refOrID); node != nil {
			return c, refOrID, nil
		}
		for _, f := range c.Festivals {
			if f.ID == refOrID {
				return c, f.Ref, nil
			}
		}
	}

	return nil, "", fmt.Errorf("festival %q not found in any chain", refOrID)
}

// discoverAllChains loads chains from active and completed directories.
func discoverAllChains(ctx context.Context) ([]*chainpkg.Chain, error) {
	dirs, err := workspaceChainDirs()
	if err != nil {
		return nil, err
	}

	var all []*chainpkg.Chain
	for _, dir := range dirs {
		chains, _ := discoverChains(ctx, dir)
		all = append(all, chains...)
	}
	return all, nil
}

func workspaceChainDirs() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("finding festivals root: %w", err)
	}
	return []string{
		filepath.Join(root, "chains"),
		filepath.Join(root, "dungeon", "completed", "chains"),
	}, nil
}
