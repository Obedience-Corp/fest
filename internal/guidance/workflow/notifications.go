package workflow

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

// queueEntryNotification only observes a successfully persisted transition.
// Execution belongs to the outer command, after callers release their locks.
func (n *Navigator) queueEntryNotification(ctx context.Context) {
	if n.workflowState == nil || n.workflowState.IsComplete() {
		return
	}
	step := n.workflowState.CurrentStep
	if step < 1 || step > len(n.steps) || !IsPresentationStep(n.steps[step-1]) {
		return
	}
	n.queueNotification(ctx, hooks.PresentationOpenName, hooks.VerbWorkflowPresent, step)
}

func (n *Navigator) queueNotification(ctx context.Context, name string, verb hooks.Verb, step int) {
	hooks.QueueNotification(ctx, hooks.Notification{
		Name: name, Verb: verb, Workflow: n.stateKey(),
		Coord: hooks.Coord{FestivalPath: n.festivalPath, Phase: n.phaseName, Step: step},
	})
}

// saveTransition retains the existing persistence formats. A failed save never
// queues a notification. previousStep=0 denotes a new run/reset to step one.
func (n *Navigator) saveTransition(ctx context.Context, previousStep int, approved bool) error {
	var err error
	if n.store != nil {
		err = n.store.SaveEvents(ctx)
	} else {
		err = n.workflowState.Save(ctx, n.festivalPath, n.stateKey())
	}
	if err != nil {
		return err
	}
	if approved {
		n.queueNotification(ctx, hooks.ApprovalNotifyName, hooks.VerbWorkflowApproved, previousStep)
	}
	if previousStep != n.workflowState.CurrentStep {
		n.queueEntryNotification(ctx)
	}
	return nil
}
