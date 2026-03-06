package chain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
	"github.com/Obedience-Corp/fest/internal/yamlutil"
)

func newCompleteCmd() *cobra.Command {
	var (
		force bool
		notes string
	)

	cmd := &cobra.Command{
		Use:   "complete <chain-id>",
		Short: "Complete and archive a chain",
		Long: `Mark a chain as completed and move it to festivals/dungeon/completed/chains/.

All festivals in the chain must be completed unless --force is used.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runComplete(cmd.Context(), args[0], force, notes)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "complete even if not all festivals are done")
	cmd.Flags().StringVar(&notes, "notes", "", "completion notes for the status history")

	return cmd
}

func runComplete(ctx context.Context, chainID string, force bool, notes string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, chainPath, err := findChainByID(ctx, chainID)
	if err != nil {
		return err
	}

	// Resolve live statuses and compute progress.
	statuses, _ := resolveChainStatuses(ctx, c)
	progress, _ := chainpkg.ComputeProgress(ctx, c, statuses)

	if progress != nil && progress.Completed < progress.Total && !force {
		fmt.Printf("%s Cannot complete chain — %d of %d festivals incomplete.\n",
			ui.Warning("BLOCKED"), progress.Total-progress.Completed, progress.Total)
		for _, f := range c.Festivals {
			if statuses[f.Ref] != chainpkg.FestivalCompleted {
				fmt.Printf("  %s (%s) %s  %s\n", f.Ref, f.ID, statusIcon(statuses[f.Ref]), f.Name)
			}
		}
		fmt.Println()
		fmt.Println("Use --force to complete anyway.")
		return nil
	}

	// Transition chain to completed.
	if notes == "" {
		notes = "Chain completed"
	}
	if err := chainpkg.Transition(ctx, c, chainpkg.StatusCompleted, notes); err != nil {
		return errors.Wrap(err, "transitioning chain").WithCode(errors.ErrCodeValidation)
	}

	// Marshal updated chain.
	data, err := yamlutil.Marshal(c)
	if err != nil {
		return errors.Wrap(err, "marshaling chain").WithCode(errors.ErrCodeInternal)
	}

	// Determine destination path.
	root, err := festivalsRoot()
	if err != nil {
		return err
	}

	destDir := filepath.Join(root, "dungeon", "completed", "chains")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return errors.IO("creating completed chains directory", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(chainPath))

	// Write updated chain to destination.
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return errors.IO("writing completed chain", err)
	}

	// Remove original if it moved to a new location.
	if destPath != chainPath {
		if err := os.Remove(chainPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove original chain file: %s\n", err)
		}
	}

	// Summary.
	fmt.Println(ui.Label("CHAIN COMPLETED"))
	fmt.Printf("  Chain: %s (%s)\n", c.Metadata.Name, c.Metadata.ID)
	if progress != nil {
		fmt.Printf("  Festivals: %d/%d completed\n", progress.Completed, progress.Total)
	}
	fmt.Printf("  Archived to: %s\n", destPath)

	return nil
}
