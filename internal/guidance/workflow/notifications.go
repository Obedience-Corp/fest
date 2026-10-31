package workflow

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

// queueEntryNotification only observes a successfully persisted transition.
// Execution belongs to the outer command, after callers release their locks.
func (n *Navigator) queueEntryNotification(ctx context.Context) {
	if n.workflowState == nil || n.workflowState.IsComplete() {
		return
	}
	step := n.workflowState.CurrentStep
	if step < 1 || step > len(n.steps) {
		return
	}
	current := n.steps[step-1]
	if IsPresentationStep(current) {
		n.queueNotification(ctx, hooks.PresentationOpenName, hooks.VerbWorkflowPresent, step)
	}
	if n.needsHumanApproval(ctx, current) {
		n.queueNotification(ctx, hooks.ApprovalNotifyName, hooks.VerbWorkflowApprovalNeeded, step)
	}
}

func (n *Navigator) queueNotification(ctx context.Context, name string, verb hooks.Verb, step int) {
	hooks.QueueNotification(ctx, hooks.Notification{
		Name: name, Verb: verb, Workflow: n.stateKey(),
		Coord: hooks.Coord{FestivalPath: n.festivalPath, Phase: n.phaseName, Step: step},
	})
}

// saveTransition retains the existing persistence formats. A failed save never
// queues a notification. previousStep=0 denotes a new run/reset to step one.
func (n *Navigator) saveTransition(ctx context.Context, previousStep int) error {
	var err error
	if n.store != nil {
		err = n.store.SaveEvents(ctx)
	} else {
		err = n.workflowState.Save(ctx, n.festivalPath, n.stateKey())
	}
	if err != nil {
		return err
	}
	// Initialization may have queued an approval request earlier in this same
	// command. Only invalidate it after persistence succeeds, including when
	// completing the final step leaves CurrentStep unchanged.
	hooks.DiscardApprovalNotifications(ctx, n.festivalPath, n.stateKey())
	if previousStep != n.workflowState.CurrentStep {
		n.queueEntryNotification(ctx)
	}
	return nil
}

// needsHumanApproval mirrors workflow approval routing. An invalid judge config
// needs configuration repair, not a notification that grants an alternate route.
func (n *Navigator) needsHumanApproval(ctx context.Context, step WorkflowStep) bool {
	if !step.Checkpoint.IsBlocking() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(step.Approval), "human-required") || ClassifyCheckpoint(step) == CheckpointClassOperatorAttestation {
		return true
	}
	load := n.notificationHooks
	if load == nil {
		load = hooks.LookupApprovalJudgeCommand
	}
	command, err := load(ctx, n.festivalPath)
	return err == nil && command == ""
}
