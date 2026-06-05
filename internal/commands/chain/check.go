package chain

import (
	"context"
	"fmt"
	"os"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var chainID string

	cmd := &cobra.Command{
		Use:   "check [ref-or-id]",
		Short: "Check if a festival is unblocked within its chain",
		Long: "Quick check whether a specific festival's hard dependencies are met. " +
			"The festival ref or id is optional when it can be inferred from the " +
			"current festival or linked project.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refOrID := firstArg(args)
			if refOrID == "" {
				festID, ok := resolveCurrentFestivalID(cmd.Context())
				if !ok {
					return errors.Validation("festival ref or id required").
						WithHint("run from inside a festival or linked project, or pass a ref/id")
				}
				refOrID = festID
			}
			return runCheck(cmd.Context(), refOrID, chainID)
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
		return errors.NotFound("festival").
			WithField("ref", ref).
			WithField("chainID", c.Metadata.ID)
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

	// Resolve live festival statuses.
	statuses, _ := resolveChainStatuses(ctx, c)

	fmt.Printf("%s (%s) has %d hard dependencies:\n", ref, node.ID, len(hardUpstream))

	pendingCount := 0
	for _, e := range hardUpstream {
		upstream := c.FestivalByRef(e.From)
		if upstream == nil {
			continue
		}
		s := statuses[e.From]
		icon := statusIcon(s)
		fmt.Printf("  %s (%s) %s  %s\n", e.From, upstream.ID, icon, upstream.Name)
		if s != chainpkg.FestivalCompleted {
			pendingCount++
		}
	}

	fmt.Println()
	if pendingCount == 0 {
		fmt.Printf("%s — all hard dependencies are completed\n", ui.Accent("UNBLOCKED"))
	} else {
		fmt.Printf("%s (%d deps pending)\n", ui.Warning("BLOCKED"), pendingCount)
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
		return nil, "", errors.NotFound("festival").
			WithField("refOrID", refOrID).
			WithField("chainID", chainIDFlag)
	}

	// Search all chains for the ref or ID.
	root, err := festivalsRoot()
	if err != nil {
		return nil, "", err
	}

	chains, err := chainpkg.DiscoverAll(ctx, root)
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

	return nil, "", errors.NotFound("festival").
		WithField("refOrID", refOrID).
		WithHint("specify a chain with --chain or ensure the festival is in a chain")
}

// festivalsRoot returns the festivals root directory.
func festivalsRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.IO("getting working directory", err)
	}
	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return "", errors.Wrap(err, "finding festivals root").WithCode(errors.ErrCodeConfig)
	}
	return root, nil
}
