package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

var runsJSON bool

func newRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List runs of a standalone workflow",
		Long: `List runs recorded in .workflow/workflow.yaml.

Each row shows run id, status, started time, and completed time.`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuns(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(&runsJSON, "json", false, "emit JSON")
	return cmd
}

func runRuns(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return festerrors.IO("getwd", err)
	}
	res, err := standalone.Resolve(ctx, cwd)
	if err != nil {
		return err
	}
	if res.Mode != standalone.ModeTracked {
		return festerrors.New("fest workflow runs requires a tracked standalone workflow").
			WithField("mode", string(res.Mode))
	}
	store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
	runs, err := store.ListRuns(ctx)
	if err != nil {
		return err
	}
	if runsJSON {
		data, err := json.MarshalIndent(runs, "", "  ")
		if err != nil {
			return festerrors.Parse("encoding runs json", err)
		}
		fmt.Println(string(data))
		return nil
	}
	if len(runs) == 0 {
		fmt.Println("No runs.")
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-26s  %-10s  %-25s  %s\n", "RUN_ID", "STATUS", "STARTED", "COMPLETED")
	for _, r := range runs {
		fmt.Fprintf(&b, "%-26s  %-10s  %-25s  %s\n", r.RunID, r.Status, r.StartedAt, r.CompletedAt)
	}
	fmt.Print(b.String())
	return nil
}
