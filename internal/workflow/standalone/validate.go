package standalone

import (
	"context"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wfparser "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// ValidateWorkflowDoc ensures WORKFLOW.md is parseable and has at least one
// step. Callers use this BEFORE creating any localstore state so a malformed
// or empty WORKFLOW.md cannot leave the user with a half-initialized
// .workflow/ directory (D030 finding #7).
func ValidateWorkflowDoc(ctx context.Context, path string) error {
	n, err := wfparser.NewParser().StepCount(ctx, path)
	if err != nil {
		return festerrors.Wrap(err, "parsing WORKFLOW.md").WithField("path", path)
	}
	if n == 0 {
		return festerrors.Validation("WORKFLOW.md has no parseable steps").
			WithField("path", path).
			WithHint("add at least one '## Step 1:' heading")
	}
	return nil
}
