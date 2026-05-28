package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate WORKFLOW.md step numbering",
		Long: `Validate a standalone WORKFLOW.md file for correct step numbering.

Step headings ('## Step N:') must start at 1 and form a contiguous sequence
with no gaps or duplicates. No other checks are performed (section names,
state-tracking tables, and checkpoint syntax are deliberately not validated).

If no path is provided, defaults to ./WORKFLOW.md in the current directory.`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 {
				path = args[0]
			}
			return runValidate(cmd.Context(), path)
		},
	}
}

func runValidate(ctx context.Context, path string) error {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return festerrors.IO("getwd", err)
		}
		path = filepath.Join(cwd, "WORKFLOW.md")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return festerrors.Wrap(err, "resolving path").WithField("path", path)
	}

	info, statErr := os.Stat(abs)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return festerrors.NotFound("WORKFLOW.md").
				WithField("path", abs).
				WithHint("provide a path to an existing WORKFLOW.md or run from a directory that contains one")
		}
		return festerrors.IO("stat WORKFLOW.md", statErr).WithField("path", abs)
	}
	if info.IsDir() {
		return festerrors.Validation("path is a directory, expected a WORKFLOW.md file").
			WithField("path", abs).
			WithHint("pass the WORKFLOW.md file directly, not its parent directory")
	}

	if err := standalone.ValidateStepNumbering(ctx, abs); err != nil {
		return err
	}

	fmt.Printf("WORKFLOW.md step numbering is valid: %s\n", abs)
	return nil
}
