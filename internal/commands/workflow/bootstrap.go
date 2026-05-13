package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// BootstrapOptions controls anonymous-to-tracked conversion.
type BootstrapOptions struct {
	WorkitemID  string // optional override for the generated workitem id
	NoBootstrap bool   // refuse to create files
}

// EnsureTracked converts an anonymous standalone workflow into tracked mode by
// writing .workitem, .workflow/workflow.yaml, and starting the first run.
// Returns a standalone.Result reflecting the new tracked state.
//
// Behavior:
//   - if res.Mode is not ModeAnonymous, returns res unchanged
//   - if opts.NoBootstrap is true, returns an error
//   - if .workitem or .workflow/ already exists, refuses to overwrite
//   - otherwise creates both files and starts run 1
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

	if _, err := os.Stat(filepath.Join(dir, ".workitem")); err == nil {
		return nil, festerrors.New("refusing to bootstrap: .workitem already exists").
			WithField("path", filepath.Join(dir, ".workitem"))
	}
	if _, err := os.Stat(filepath.Join(dir, ".workflow")); err == nil {
		return nil, festerrors.New("refusing to bootstrap: .workflow/ already exists").
			WithField("path", filepath.Join(dir, ".workflow"))
	}

	workitemID := opts.WorkitemID
	if workitemID == "" {
		workitemID = "wf-" + filepath.Base(dir) + "-" + strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	workflowID := "wf-" + filepath.Base(dir)

	store := localstore.Open(filepath.Join(dir, ".workflow"), res.WorkflowDoc)
	if err := store.Init(ctx, localstore.InitOptions{
		WorkflowID: workflowID,
		WorkitemID: workitemID,
	}); err != nil {
		return nil, festerrors.Wrap(err, "bootstrap init")
	}
	if _, err := store.StartRun(ctx, ""); err != nil {
		return nil, festerrors.Wrap(err, "bootstrap start")
	}
	if err := writeMinimalWorkitem(filepath.Join(dir, ".workitem"), workitemID, filepath.Base(dir), workflowID); err != nil {
		return nil, festerrors.Wrap(err, "bootstrap .workitem")
	}

	return &standalone.Result{
		Mode:        standalone.ModeTracked,
		StartDir:    res.StartDir,
		WorkflowDoc: res.WorkflowDoc,
		RuntimeDir:  filepath.Join(dir, ".workflow"),
	}, nil
}
