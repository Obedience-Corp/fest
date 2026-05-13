package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initForce      bool
	initWorkflowID string
	initWorkitemID string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize standalone workflow runtime",
		Long: `Create .workitem and .workflow/ next to an existing WORKFLOW.md.

Run from the directory containing WORKFLOW.md. The command refuses to run
inside a festival phase. Use --force to overwrite existing .workitem
or .workflow/workflow.yaml.`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing .workitem and .workflow/")
	cmd.Flags().StringVar(&initWorkflowID, "workflow-id", "", "workflow_id to write into manifest (defaults to wf-<basename>)")
	cmd.Flags().StringVar(&initWorkitemID, "workitem-id", "", "workitem_id to write into manifest (required)")
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

	if initWorkitemID == "" {
		return festerrors.Validation("--workitem-id is required")
	}

	workflowID := initWorkflowID
	if workflowID == "" {
		workflowID = "wf-" + filepath.Base(cwd)
	}

	// Refuse to overwrite an existing .workitem unless --force.
	workitemPath := filepath.Join(cwd, ".workitem")
	if _, statErr := os.Stat(workitemPath); statErr == nil && !initForce {
		return festerrors.New(".workitem already exists; pass --force to overwrite").
			WithField("path", workitemPath)
	}

	store := localstore.Open(filepath.Join(cwd, ".workflow"), doc)
	if err := store.Init(ctx, localstore.InitOptions{
		WorkflowID: workflowID,
		WorkitemID: initWorkitemID,
		Force:      initForce,
	}); err != nil {
		return err
	}

	if err := writeMinimalWorkitem(workitemPath, initWorkitemID, filepath.Base(cwd), workflowID); err != nil {
		return err
	}

	fmt.Printf("Initialized standalone workflow runtime at %s\n", cwd)
	fmt.Printf("  .workitem: workitem_id=%s\n", initWorkitemID)
	fmt.Printf("  .workflow/workflow.yaml: workflow_id=%s\n", workflowID)
	return nil
}

// minimalWorkitem is the typed struct yaml.Marshal serializes for a bootstrap
// .workitem file. Using a struct + yaml.Marshal escapes special characters
// (colons, quotes, leading dashes, hash signs, whitespace) in fields like
// title that flow from arbitrary user directory names. String concatenation
// would produce malformed YAML for any directory name containing those.
type minimalWorkitem struct {
	Version  int                `yaml:"version"`
	Kind     string             `yaml:"kind"`
	ID       string             `yaml:"id"`
	Type     string             `yaml:"type"`
	Title    string             `yaml:"title"`
	Workflow minimalWorkitemWf  `yaml:"workflow"`
}

type minimalWorkitemWf struct {
	DocPath    string `yaml:"doc_path"`
	RuntimeDir string `yaml:"runtime_dir"`
	WorkflowID string `yaml:"workflow_id"`
}

func writeMinimalWorkitem(path, id, basename, workflowID string) error {
	body, err := yaml.Marshal(minimalWorkitem{
		Version: 1,
		Kind:    "workitem",
		ID:      id,
		Type:    "workflow",
		Title:   basename,
		Workflow: minimalWorkitemWf{
			DocPath:    "WORKFLOW.md",
			RuntimeDir: ".workflow",
			WorkflowID: workflowID,
		},
	})
	if err != nil {
		return festerrors.Wrap(err, "marshal .workitem")
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return festerrors.IO("writing .workitem", err)
	}
	return nil
}
