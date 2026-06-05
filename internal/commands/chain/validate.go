package chain

import (
	"context"
	"fmt"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var cross bool

	cmd := &cobra.Command{
		Use:   "validate [chain-id]",
		Short: "Validate a festival chain",
		Long: "Run all structural validation checks (S1-S10) against a chain definition.\n" +
			"The chain id is optional when it can be inferred from the current festival " +
			"or linked project, or selected interactively in a terminal.\n" +
			"Use --cross to validate across all chains.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cross {
				return runCrossValidate(cmd.Context())
			}
			chainID, _, err := resolveChainID(cmd.Context(), firstArg(args))
			if err != nil {
				return err
			}
			return runValidate(cmd.Context(), chainID)
		},
	}

	cmd.Flags().BoolVar(&cross, "cross", false, "validate across all chains (duplicate IDs, conflicts)")

	return cmd
}

func runValidate(ctx context.Context, chainID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, _, err := findChainByID(ctx, chainID)
	if err != nil {
		return err
	}

	fmt.Printf("Validating chain: %s (%s)\n\n", c.Metadata.Name, c.Metadata.ID)

	result := chainpkg.Validate(ctx, c)

	fmt.Println(ui.Label("Structural checks:"))
	if len(result.Errors) == 0 {
		fmt.Println("  All structural checks passed")
	} else {
		for _, e := range result.Errors {
			fmt.Printf("  %s %s: %s\n", ui.Accent("x"), e.Code, e.Message)
			if e.Context != "" {
				fmt.Printf("    %s\n", e.Context)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println(ui.Label("Warnings:"))
		for _, w := range result.Warnings {
			fmt.Printf("  ! %s: %s\n", w.Code, w.Message)
		}
	}

	fmt.Println()
	if result.Valid {
		fmt.Printf("Result: VALID (score: %d/100)\n", result.Score)
	} else {
		fmt.Printf("Result: INVALID (%d errors, %d warnings)\n",
			len(result.Errors), len(result.Warnings))
		return errors.Validation("validation failed").WithField("error_count", len(result.Errors))
	}

	return nil
}

func runCrossValidate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := festivalsRoot()
	if err != nil {
		return err
	}

	chains, parseErrors, err := chainpkg.DiscoverAllStrict(ctx, root)
	if err != nil {
		return err
	}

	if len(parseErrors) > 0 {
		fmt.Println(ui.Label("Parse errors:"))
		for _, pe := range parseErrors {
			fmt.Printf("  %s %s\n", ui.Accent("x"), pe.Error())
		}
		fmt.Println()
	}

	if len(chains) < 2 {
		fmt.Printf("Found %d valid chain(s) — cross-chain validation requires at least 2.\n", len(chains))
		if len(parseErrors) > 0 {
			return errors.Parse("chain files failed to parse", nil).WithField("count", len(parseErrors))
		}
		return nil
	}

	fmt.Printf("Cross-validating %d chains...\n\n", len(chains))

	result := chainpkg.ValidateCrossChain(ctx, chains)

	if len(result.Errors) > 0 {
		fmt.Println(ui.Label("Cross-chain errors:"))
		for _, e := range result.Errors {
			fmt.Printf("  %s %s: %s\n", ui.Accent("x"), e.Code, e.Message)
		}
	}

	if len(result.Warnings) > 0 {
		if len(result.Errors) > 0 {
			fmt.Println()
		}
		fmt.Println(ui.Label("Warnings:"))
		for _, w := range result.Warnings {
			fmt.Printf("  ! %s: %s\n", w.Code, w.Message)
		}
	}

	fmt.Println()
	totalErrors := len(result.Errors) + len(parseErrors)
	if result.Valid && len(parseErrors) == 0 {
		fmt.Println("Result: VALID — no cross-chain conflicts detected")
	} else {
		fmt.Printf("Result: INVALID (%d errors)\n", totalErrors)
		return errors.Validation("cross-chain validation failed").WithField("error_count", totalErrors)
	}

	return nil
}
