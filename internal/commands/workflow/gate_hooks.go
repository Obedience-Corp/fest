package workflow

import (
	"context"
	"fmt"
	"os"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// newGateHookRunner is an injectable seam so tests can fake hook execution.
var newGateHookRunner = hooks.NewRunner

// gateHookRequest builds the lifecycle request for a gate step's bindings
// (spec 03: gate approve verb). A human-required step skips every automation
// hook, recorded as skipped: human-gate (spec 04).
func gateHookRequest(nav *wf.Navigator, stepNum int, step wf.WorkflowStep) progress.LifecycleHookRequest {
	return progress.LifecycleHookRequest{
		FestivalPath: nav.Ctx.FestivalPath,
		Phase:        nav.Ctx.PhaseName,
		Step:         stepNum,
		Level:        hooks.LevelGate,
		Verb:         hooks.VerbGateApprove,
		Pre:          step.Hooks.Pre,
		Post:         step.Hooks.Post,
		HumanGate:    isHumanRequired(step),
	}
}

// runGateHookStage plans and runs one timing of the step's bindings and
// persists wf_hook_run events through the navigator's state store. For the
// pre timing a fail-closed failure blocks the approve verb; for the post
// timing the verb stays applied and the failure is surfaced as a warning.
func runGateHookStage(ctx context.Context, nav *wf.Navigator, stepNum int, step wf.WorkflowStep, timing hooks.Timing) (bool, error) {
	req := gateHookRequest(nav, stepNum, step)
	planned, err := progress.PlanLifecycleHooks(ctx, req)
	if err != nil {
		return false, err
	}
	if len(planned) == 0 {
		return false, nil
	}
	runner := newGateHookRunner(nav.Ctx.FestivalPath)
	var (
		runs    []hooks.HookRun
		blocked bool
	)
	if timing == hooks.TimingPre {
		runs, blocked, err = runner.RunPre(ctx, req.Level, req.Verb, planned, nil)
	} else {
		runs, blocked, err = runner.RunPost(ctx, req.Level, req.Verb, planned, nil)
	}
	if emitErr := nav.QueueHookRunEvents(ctx, stepNum, runs); emitErr != nil && err == nil {
		err = emitErr
	}
	if err != nil {
		return blocked, festerrors.Wrap(err, "running gate hooks")
	}
	if blocked && timing == hooks.TimingPre {
		return true, progress.BlockedHookError(hooks.VerbGateApprove, runs)
	}
	if blocked {
		fmt.Fprintln(os.Stderr, "Warning: a fail-closed post hook failed after approval; see wf_hook_run in the festival audit trail")
	}
	return blocked, nil
}

// emitJudgeHookRuns persists the judge execution's own runner records so the
// audit trail includes the judge as a gate_approve hook run (D9). Best-effort:
// event persistence must never turn into a judge outcome error.
func emitJudgeHookRuns(ctx context.Context, nav *wf.Navigator, stepNum int, runs []hooks.HookRun) {
	if nav == nil {
		return
	}
	_ = nav.QueueHookRunEvents(ctx, stepNum, runs)
}
