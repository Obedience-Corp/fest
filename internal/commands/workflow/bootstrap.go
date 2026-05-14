package workflow

import (
	"context"
	"os"
	"path/filepath"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// BootstrapOptions controls anonymous-to-tracked conversion.
type BootstrapOptions struct {
	NoBootstrap bool // refuse to create files
}

// EnsureTracked converts an anonymous standalone workflow into tracked mode by
// initializing .workflow/ and starting the first run. Fest does NOT write
// .workitem (D007: .workitem is owned by camp). Returns a standalone.Result
// reflecting the new tracked state.
//
// Behavior:
//   - if res.Mode is not ModeAnonymous, returns res unchanged
//   - if opts.NoBootstrap is true, returns an error
//   - if .workflow/ already exists, refuses to overwrite
//   - validates that WORKFLOW.md has at least one parseable step BEFORE
//     creating any state (D030 finding #7)
//   - otherwise initializes .workflow/ and starts run 1
//   - if StartRun fails after Init, removes the .workflow/ dir to avoid
//     leaving the user in a half-initialized state
func EnsureTracked(ctx context.Context, res *standalone.Result, opts BootstrapOptions) (*standalone.Result, error) {
	if res == nil || res.Mode != standalone.ModeAnonymous {
		return res, nil
	}
	if opts.NoBootstrap {
		return nil, festerrors.New("anonymous workflow detected and --no-bootstrap is set").
			WithField("workflow_doc", res.WorkflowDoc).
			WithHint("Run 'fest workflow init' first or omit --no-bootstrap")
	}

	dir := filepath.Dir(res.WorkflowDoc)

	workflowDir := filepath.Join(dir, ".workflow")
	if _, err := os.Stat(workflowDir); err == nil {
		return nil, festerrors.New("refusing to bootstrap: .workflow/ already exists").
			WithField("path", workflowDir)
	}

	// Validate WORKFLOW.md has parseable steps before any state is created
	// (D030 finding #7). Avoids leaving the user with a broken .workflow/
	// after a malformed WORKFLOW.md.
	if err := standalone.ValidateWorkflowDoc(ctx, res.WorkflowDoc); err != nil {
		return nil, err
	}

	workflowID := "wf-" + workflow.SanitizeBasenameAsSlug(filepath.Base(dir))
	if err := workflow.ValidateWorkflowID(workflowID); err != nil {
		return nil, err
	}

	store := localstore.Open(workflowDir, res.WorkflowDoc)
	if err := store.Init(ctx, localstore.InitOptions{
		WorkflowID: workflowID,
	}); err != nil {
		return nil, festerrors.Wrap(err, "bootstrap init")
	}
	if _, err := store.StartRun(ctx, ""); err != nil {
		// Rollback: remove .workflow/ so the user can retry cleanly.
		_ = os.RemoveAll(workflowDir)
		return nil, festerrors.Wrap(err, "bootstrap start")
	}

	return &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    res.StartDir,
		WorkflowDoc: res.WorkflowDoc,
		RuntimeDir:  workflowDir,
	}, nil
}
