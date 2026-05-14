package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

var (
	initForce      bool
	initWorkflowID string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize standalone workflow runtime",
		Long: `Create .workflow/ next to an existing WORKFLOW.md.

Run from the directory containing WORKFLOW.md. The command refuses to run
inside a festival phase. Use --force to overwrite an existing
.workflow/workflow.yaml.

This command does not create .workitem; that file is owned by camp
(see 'camp workitem create' and 'camp workitem adopt').`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing .workflow/workflow.yaml")
	cmd.Flags().StringVar(&initWorkflowID, "workflow-id", "", "workflow_id to write into manifest (defaults to wf-<basename>)")
	return cmd
}

func runInit(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return festerrors.IO("getwd", err)
	}

	res, err := standalone.Resolve(ctx, cwd)
	if err != nil {
		return err
	}
	if res.Mode == standalone.ModeFestival {
		return festerrors.New("fest workflow init cannot run inside a festival phase").
			WithField("festival_path", res.FestivalPath).
			WithHint("This command is for standalone WORKFLOW.md directories. Use festival workflow commands inside the festival.")
	}

	doc := filepath.Join(cwd, "WORKFLOW.md")
	if _, statErr := os.Stat(doc); statErr != nil {
		return festerrors.New("no WORKFLOW.md found").
			WithField("dir", cwd).
			WithHint("Author a WORKFLOW.md before running init")
	}

	workflowID := initWorkflowID
	if workflowID == "" {
		slug := workflow.SanitizeBasenameAsSlug(filepath.Base(cwd))
		workflowID = "wf-" + slug
	}
	if err := workflow.ValidateWorkflowID(workflowID); err != nil {
		return err
	}

	store := localstore.Open(filepath.Join(cwd, ".workflow"), doc)
	if err := store.Init(ctx, localstore.InitOptions{
		WorkflowID: workflowID,
		Force:      initForce,
	}); err != nil {
		return err
	}

	fmt.Printf("Initialized standalone workflow runtime at %s\n", cwd)
	fmt.Printf("  .workflow/workflow.yaml: workflow_id=%s\n", workflowID)
	return nil
}
