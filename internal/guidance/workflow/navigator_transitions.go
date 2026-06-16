package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

// MarkComplete marks a step as complete.
func (n *Navigator) MarkComplete(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.CompleteCurrentStep()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepDoneEvents(sk, currentStep))
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			_ = n.workflowState.Advance()
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// MarkSkipped marks a step as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}
	if err := n.validateCurrentStepID(stepID); err != nil {
		return err
	}
	return n.SkipCurrentStep(ctx, StepStatusSkipped, "")
}

func (n *Navigator) validateCurrentStepID(stepID string) error {
	if strings.TrimSpace(stepID) == "" {
		return nil
	}
	expected := fmt.Sprintf("step_%d", n.workflowState.CurrentStep)
	if stepID != expected {
		return fmt.Errorf("step ID mismatch: expected %s, got %s", expected, stepID)
	}
	return nil
}

// SkipCurrentStep marks the current step as skipped/completed with an audit reason.
func (n *Navigator) SkipCurrentStep(ctx context.Context, status StepStatus, reason string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if status != StepStatusSkipped && status != StepStatusCompleted {
		return fmt.Errorf("invalid terminal status for skip: %s", status)
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.MarkCurrentStep(status, reason)

	sk := n.stateKey()
	if n.store != nil {
		if status == StepStatusSkipped {
			n.store.QueueWorkflowEvents(EmitStepSkipEvents(sk, currentStep, reason))
		} else {
			n.store.QueueWorkflowEvents(EmitStepDoneWithFeedbackEvents(sk, currentStep, reason))
		}
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			_ = n.workflowState.Advance()
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		_ = n.workflowState.Advance()
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// MarkFailed marks a step as failed.
func (n *Navigator) MarkFailed(ctx context.Context, stepID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	n.workflowState.Reject("step failed")

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepBlockEvents(sk, n.workflowState.CurrentStep, "step failed"))
		return n.store.SaveEvents(ctx)
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// Advance moves to the next step.
func (n *Navigator) Advance(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if n.workflowState.IsComplete() {
		return guidance.ErrAlreadyComplete
	}

	currentStep := n.workflowState.CurrentStep
	n.workflowState.CompleteCurrentStep()

	sk := n.stateKey()
	if n.store != nil {
		n.store.QueueWorkflowEvents(EmitStepDoneEvents(sk, currentStep))
		if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
			if err := n.workflowState.Advance(); err != nil {
				return err
			}
			n.store.QueueWorkflowEvents(EmitAdvanceEvents(sk, n.workflowState.CurrentStep))
			n.store.QueueWorkflowEvents(EmitStepStartEvents(sk, n.workflowState.CurrentStep))
		}
		return n.store.SaveEvents(ctx)
	}

	if n.workflowState.CurrentStep < n.workflowState.TotalSteps {
		if err := n.workflowState.Advance(); err != nil {
			return err
		}
	}
	return n.workflowState.Save(ctx, n.festivalPath, sk)
}

// GetProgress returns workflow progress.
func (n *Navigator) GetProgress(ctx context.Context) (*guidance.Progress, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	progress := guidance.NewProgress(n.mode)
	progress.Total = n.workflowState.TotalSteps
	progress.Completed = n.workflowState.CompletedCount()
	progress.Pending = progress.Total - progress.Completed
	progress.Calculate()

	if n.workflowState.CurrentStep >= 1 && n.workflowState.CurrentStep <= len(n.steps) {
		step := n.steps[n.workflowState.CurrentStep-1]
		progress.CurrentTask = step.Name
	}

	return progress, nil
}
