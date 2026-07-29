package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [chain-id]",
		Short: "Show chain status and progress",
		Long: "Show chain status and progress. The chain id is optional when it can " +
			"be inferred from the current festival or linked project, or selected " +
			"interactively in a terminal.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID, _, err := resolveChainID(cmd.Context(), firstArg(args))
			if err != nil {
				return err
			}
			return runStatus(cmd.Context(), chainID)
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

	// Resolve live statuses.
	statuses, resolveErr := resolveChainStatuses(ctx, c)
	if resolveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve all statuses: %s\n", resolveErr)
	}

	// Compute progress if we have statuses.
	var progress *chainpkg.ChainProgress
	if statuses != nil {
		progress, _ = chainpkg.ComputeProgress(ctx, c, statuses)
	}

	// Waves with live status annotations.
	for _, w := range c.Waves {
		waveLabel := fmt.Sprintf("Wave %d: %s", w.ID, w.Name)
		if progress != nil {
			if wp, ok := progress.WaveStatus[w.ID]; ok {
				waveLabel += fmt.Sprintf(" [%s]", wp.State)
			}
		}
		fmt.Println(waveLabel)
		for _, ref := range w.Festivals {
			node := c.FestivalByRef(ref)
			if node == nil {
				continue
			}
			icon := statusIcon(statuses[ref])
			fmt.Printf("  %-4s %-8s %-10s %s\n", node.Ref, node.ID, icon, node.Name)
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

	// Progress bar.
	if progress != nil {
		pct := int(progress.Percentage)
		bar := progressBar(progress.Completed, progress.Total, 30)
		fmt.Printf("%s %s [%d/%d] %d%%\n", ui.Label("Progress:"), bar, progress.Completed, progress.Total, pct)

		if len(progress.Unblocked) > 0 {
			fmt.Println()
			fmt.Println(ui.Label("Unblocked:"))
			for _, ref := range progress.Unblocked {
				node := c.FestivalByRef(ref)
				if node != nil {
					fmt.Printf("  %s (%s) %s\n", ref, node.ID, node.Name)
				}
			}
		}
	} else {
		fmt.Printf("Festivals: %d total\n", len(c.Festivals))
	}

	return nil
}

// progressBar renders a simple ASCII progress bar.
func progressBar(completed, total, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := width * completed / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// findChainByID searches for a chain file by its ID across active and completed dirs.
func findChainByID(ctx context.Context, chainID string) (*chainpkg.Chain, string, error) {
	root, err := festivalsRoot()
	if err != nil {
		return nil, "", err
	}

	if err := workspace.CheckDungeonConflict(root); err != nil {
		return nil, "", err
	}

	searchDirs := []string{
		filepath.Join(root, "chains"),
		workspace.JoinDungeon(root, "completed", "chains"),
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

	return nil, "", errors.NotFound("chain").WithField("chainID", chainID)
}
