package chain

import (
	"context"
	"fmt"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <chain-id>",
		Short: "Validate a festival chain",
		Long:  "Run all structural validation checks (S1-S10) against a chain definition.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd.Context(), args[0])
		},
	}
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
		return fmt.Errorf("validation failed with %d errors", len(result.Errors))
	}

	return nil
}
