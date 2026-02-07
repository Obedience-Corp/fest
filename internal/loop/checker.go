package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckResult describes the outcome of a loop condition check.
type CheckResult struct {
	NextStep     int
	RetryContext string
	Escalated    bool
}

// CheckLoopCondition checks if a loop condition exists for the completed step
// and evaluates it. Returns a CheckResult indicating the next step and any retry context.
func CheckLoopCondition(ctx context.Context, phasePath string, completedStep int) (*CheckResult, error) {
	loopPath := filepath.Join(phasePath, "LOOP.yaml")
	if _, err := os.Stat(loopPath); os.IsNotExist(err) {
		return &CheckResult{NextStep: completedStep + 1}, nil
	}

	config, err := ParseLoopConfig(loopPath)
	if err != nil {
		return nil, fmt.Errorf("parse loop config: %w", err)
	}

	loopDef := config.FindLoopForStep(completedStep)
	if loopDef == nil {
		return &CheckResult{NextStep: completedStep + 1}, nil
	}

	loopIndex := -1
	for i := range config.Loops {
		if config.Loops[i].AfterStep == completedStep {
			loopIndex = i
			break
		}
	}

	evaluator, err := NewEvaluator(loopDef.Condition)
	if err != nil {
		return nil, fmt.Errorf("create evaluator: %w", err)
	}

	result, err := evaluator.Evaluate(ctx, phasePath, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("evaluate condition: %w", err)
	}

	store := NewStateStore(phasePath)

	checkEvent := StateEvent{
		Type:          "loop_check",
		Step:          completedStep,
		LoopIndex:     loopIndex,
		MaxIterations: loopDef.MaxIterations,
		Passed:        result.Pass,
	}
	if !result.Pass {
		checkEvent.FailureContext = result.FailureContext
	}
	if err := store.AppendEvent(checkEvent); err != nil {
		return nil, fmt.Errorf("record check event: %w", err)
	}

	if result.Pass {
		return &CheckResult{NextStep: completedStep + 1}, nil
	}

	currentIter, err := store.CurrentIterations(loopIndex, completedStep)
	if err != nil {
		return nil, fmt.Errorf("get iterations: %w", err)
	}

	if currentIter >= loopDef.MaxIterations {
		retryEvent := StateEvent{
			Type:           "loop_retry",
			Step:           completedStep,
			LoopIndex:      loopIndex,
			Iteration:      currentIter,
			MaxIterations:  loopDef.MaxIterations,
			Passed:         false,
			FailureContext: result.FailureContext,
			Action:         "escalate",
		}
		if err := store.AppendEvent(retryEvent); err != nil {
			return nil, fmt.Errorf("record escalation: %w", err)
		}

		escalationMsg := fmt.Sprintf("LOOP LIMIT REACHED: %s condition failed %d times. Human review required.\n\n%s",
			loopDef.Condition, loopDef.MaxIterations, result.FailureContext)

		return &CheckResult{
			NextStep:     completedStep,
			RetryContext: escalationMsg,
			Escalated:    true,
		}, nil
	}

	var targetStep int
	var action string

	if loopDef.OnFailure.Action == "retry_step" {
		targetStep = completedStep
		action = "retry_step"
	} else {
		targetStep = *loopDef.OnFailure.Step
		action = "return_to_step"
	}

	retryCtx := loopDef.OnFailure.Context
	retryCtx = strings.ReplaceAll(retryCtx, "{validation_output}", result.FailureContext)
	retryCtx = strings.ReplaceAll(retryCtx, "{marker_list}", result.FailureContext)

	retryMsg := fmt.Sprintf("RETRY %d/%d: %s\n\n%s",
		currentIter+1, loopDef.MaxIterations, loopDef.Condition, retryCtx)

	retryEvent := StateEvent{
		Type:           "loop_retry",
		Step:           completedStep,
		LoopIndex:      loopIndex,
		Iteration:      currentIter + 1,
		MaxIterations:  loopDef.MaxIterations,
		Passed:         false,
		FailureContext: result.FailureContext,
		Action:         action,
	}
	if action == "return_to_step" {
		retryEvent.ReturnToStep = loopDef.OnFailure.Step
	}
	if err := store.AppendEvent(retryEvent); err != nil {
		return nil, fmt.Errorf("record retry: %w", err)
	}

	return &CheckResult{
		NextStep:     targetStep,
		RetryContext: retryMsg,
	}, nil
}
