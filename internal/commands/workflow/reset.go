package workflow

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newResetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset workflow to step 1",
		Long: `Reset the workflow state back to step 1.

This clears all step progress and starts the workflow from the beginning.
Use with caution as this cannot be undone.`,
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReset(cmd.Context(), force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func runReset(ctx context.Context, force bool) error {
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	completed := state.CompletedCount()

	// Confirm unless force flag is set
	if !force && completed > 0 {
		fmt.Printf("%s This will reset all progress (%d completed steps). Continue? [y/N]: ",
			ui.Warning("⚠"), completed)

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := nav.Reset(ctx); err != nil {
		return fmt.Errorf("resetting workflow: %w", err)
	}

	fmt.Println(ui.Success("✓ Workflow reset to Step 1"))
	fmt.Println("All step states cleared.")

	return nil
}
