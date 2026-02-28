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

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <chain-id>",
		Short: "Show chain status and progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context(), args[0])
		},
	}
}

func runStatus(ctx context.Context, chainID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, _, err := findChainByID(ctx, chainID)
	if err != nil {
		return err
	}

	// Header
	fmt.Printf("%s %s (%s)\n", ui.Label("Chain:"), c.Metadata.Name, c.Metadata.ID)
	if c.Metadata.Goal != "" {
		fmt.Printf("Goal:   %s\n", c.Metadata.Goal)
	}
	fmt.Printf("Status: %s\n", c.Metadata.Status)
	fmt.Printf("Created: %s\n", c.Metadata.CreatedAt.Format("2006-01-02"))
	fmt.Println()

	// Waves
	for _, w := range c.Waves {
		fmt.Printf("Wave %d: %s\n", w.ID, w.Name)
		for _, ref := range w.Festivals {
			node := c.FestivalByRef(ref)
			if node == nil {
				continue
			}
			fmt.Printf("  %-4s %-8s %s\n", node.Ref, node.ID, node.Name)
		}
		fmt.Println()
	}

	// Edges
	if len(c.Edges) > 0 {
		fmt.Println(ui.Label("Edges:"))
		for _, e := range c.Edges {
			edgeStyle := "hard"
			if e.Type == chainpkg.EdgeSoft {
				edgeStyle = "soft"
			}
			fmt.Printf("  %s --%s--> %s", e.From, edgeStyle, e.To)
			if e.Note != "" {
				fmt.Printf("  (%s)", e.Note)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Printf("Festivals: %d total\n", len(c.Festivals))

	return nil
}

// findChainByID searches for a chain file by its ID across active and completed dirs.
func findChainByID(ctx context.Context, chainID string) (*chainpkg.Chain, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getting working directory: %w", err)
	}

	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return nil, "", fmt.Errorf("finding festivals root: %w", err)
	}

	searchDirs := []string{
		filepath.Join(root, "chains"),
		filepath.Join(root, "dungeon", "completed", "chains"),
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			if strings.Contains(e.Name(), chainID) {
				path := filepath.Join(dir, e.Name())
				c, err := chainpkg.Parse(ctx, path)
				if err != nil {
					continue
				}
				if c.Metadata.ID == chainID {
					return c, path, nil
				}
			}
		}
	}

	return nil, "", fmt.Errorf("chain %s not found", chainID)
}
