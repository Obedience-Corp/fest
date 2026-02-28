package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all festival chains",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), statusFilter)
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (planning|active|completed)")

	return cmd
}

func runList(ctx context.Context, statusFilter string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return fmt.Errorf("finding festivals root: %w", err)
	}

	chains, err := discoverChains(ctx, filepath.Join(root, "chains"))
	if err != nil {
		return err
	}

	// Also check dungeon for completed chains.
	dungeonChains, _ := discoverChains(ctx, filepath.Join(root, "dungeon", "completed", "chains"))
	chains = append(chains, dungeonChains...)

	if len(chains) == 0 {
		fmt.Println("No chains found. Create one with 'fest chain create --name <name>'.")
		return nil
	}

	// Filter if requested.
	if statusFilter != "" {
		var filtered []*chainpkg.Chain
		for _, c := range chains {
			if string(c.Metadata.Status) == statusFilter {
				filtered = append(filtered, c)
			}
		}
		chains = filtered
	}

	// Group by status.
	active := filterByStatus(chains, chainpkg.StatusActive, chainpkg.StatusPlanning)
	completed := filterByStatus(chains, chainpkg.StatusCompleted)

	if len(active) > 0 {
		fmt.Printf("%s (%d active)\n", ui.Label("CHAINS"), len(active))
		fmt.Printf("%-8s %-24s %-12s %-10s %s\n", "ID", "Name", "Status", "Festivals", "Refs")
		for _, c := range active {
			refs := make([]string, len(c.Festivals))
			for i, f := range c.Festivals {
				refs[i] = f.Ref
			}
			fmt.Printf("%-8s %-24s %-12s %-10s %s\n",
				c.Metadata.ID, c.Metadata.Name, c.Metadata.Status,
				fmt.Sprintf("%d", len(c.Festivals)),
				strings.Join(refs, " "))
		}
	}

	if len(completed) > 0 {
		if len(active) > 0 {
			fmt.Println()
		}
		fmt.Println(ui.Label("COMPLETED"))
		for _, c := range completed {
			fmt.Printf("%-8s %-24s completed  %d/%d\n",
				c.Metadata.ID, c.Metadata.Name,
				len(c.Festivals), len(c.Festivals))
		}
	}

	return nil
}

// discoverChains reads all .yaml files in a directory and parses them as chains.
func discoverChains(ctx context.Context, dir string) ([]*chainpkg.Chain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading chains directory: %w", err)
	}

	var chains []*chainpkg.Chain
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		c, err := chainpkg.Parse(ctx, filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip unparseable files
		}
		chains = append(chains, c)
	}

	return chains, nil
}

func filterByStatus(chains []*chainpkg.Chain, statuses ...chainpkg.ChainStatus) []*chainpkg.Chain {
	statusSet := make(map[chainpkg.ChainStatus]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}
	var result []*chainpkg.Chain
	for _, c := range chains {
		if statusSet[c.Metadata.Status] {
			result = append(result, c)
		}
	}
	return result
}
