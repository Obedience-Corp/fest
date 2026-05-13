package workflow

import (
	"context"
	"fmt"
	"os"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start a new run for a standalone workflow",
		Long: `Create a new .workflow/runs/<run-id>/ directory and mark it active.

Requires .workflow/workflow.yaml to exist (run fest workflow init first).`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd.Context())
		},
	}
}

func runStart(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return festerrors.IO("getwd", err)
	}
	res, err := standalone.Resolve(ctx, cwd)
	if err != nil {
		return err
	}
	if res.Mode != standalone.ModeTracked {
		return festerrors.New("fest workflow start requires a tracked standalone workflow").
			WithField("mode", string(res.Mode)).
			WithHint("Run 'fest workflow init' first to create .workflow/workflow.yaml")
	}

	store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
	runID, err := store.StartRun(ctx, "")
	if err != nil {
		return err
	}
	fmt.Printf("Started run %s\n", runID)
	return nil
}
