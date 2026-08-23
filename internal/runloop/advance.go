package runloop

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
)

// AdvanceStandalone marks the current standalone step done, matching
// fest workflow advance tracked semantics without printing.
func AdvanceStandalone(ctx context.Context, runtimeDir, workflowDoc string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store := localstore.Open(runtimeDir, workflowDoc)
	state, err := store.LoadActive(ctx)
	if err != nil {
		return err
	}
	if state == nil {
		return errors.NotFound("no active run")
	}
	if state.Blocked {
		return errors.New("standalone workflow step is blocked")
	}
	steps, parseErr := wf.NewParser().Parse(ctx, workflowDoc)
	if parseErr != nil {
		return errors.Wrap(parseErr, "parsing WORKFLOW.md")
	}
	actionable := state.CurrentStep
	needStart := false
	if state.CurrentStep <= state.CompletedSteps {
		actionable = state.CompletedSteps + 1
		needStart = true
	}
	if actionable > len(steps) {
		return store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted})
	}
	if actionable < 1 {
		return errors.New("invalid step index")
	}
	step := steps[actionable-1]
	if step.Checkpoint.IsBlocking() {
		return errors.New("standalone workflows do not support blocking checkpoints").
			WithField("step", actionable).
			WithField("step_name", step.Name)
	}
	if needStart {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart}); err != nil {
			return err
		}
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone}); err != nil {
		return err
	}
	if actionable >= len(steps) {
		return store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted})
	}
	return nil
}
