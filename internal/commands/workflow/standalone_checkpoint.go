package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// Standalone blocking checkpoints.
//
// A standalone WORKFLOW.md has no festival around it, so it has no approval
// judge to delegate to: a blocking checkpoint there is always a human decision.
// The run records three things in its own .workflow/ event stream, using event
// types the stream already defined:
//
//	wf_step_block           the step's work is done and it waits on a decision
//	wf_checkpoint_approved  a human approved it (followed by wf_step_done)
//	wf_checkpoint_rejected  a human rejected it; the step waits on a revision
//
// 'fest workflow advance' on a rejected step re-submits it with another
// wf_step_block. Replay already treated block and rejected as "blocked" and
// approved as "not blocked", so a reader that predates these commands (camp's
// replay of the same stream included) still derives the right blocked flag.

// standaloneCheckpoint is a tracked standalone run positioned at its current
// step.
type standaloneCheckpoint struct {
	res     *standalone.Result
	store   *localstore.Store
	state   *localstore.RunState
	steps   []wf.WorkflowStep
	stepNum int
	step    wf.WorkflowStep
}

// resolveStandaloneWorkflow reports the standalone workflow that owns cwd, or
// nil when cwd belongs to a festival (or to nothing), in which case the caller
// falls through to the festival navigator and its own errors.
func resolveStandaloneWorkflow(ctx context.Context) *standalone.Result {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	res, err := standalone.Resolve(ctx, cwd)
	if err != nil || res == nil {
		// Same fall-through as 'advance'. A resolve error here is not reported:
		// the festival navigator runs next and reports what it finds, which for
		// a directory that is neither is "not in a festival".
		return nil
	}
	if res.Mode != standalone.ModeTracked && res.Mode != standalone.ModeAnonymous {
		return nil
	}
	return res
}

// loadStandaloneCheckpoint loads the run and requires it to be stopped at a
// blocking checkpoint that is waiting on a decision. verb names the command
// for error text ("approve" or "reject").
func loadStandaloneCheckpoint(ctx context.Context, res *standalone.Result, verb string) (*standaloneCheckpoint, error) {
	nothingWaiting := func() error {
		return festerrors.Validation("no checkpoint is awaiting approval").
			WithHint("run 'fest next' to see the current step; a step reaches its checkpoint when 'fest workflow advance' is run on it")
	}
	if res.Mode != standalone.ModeTracked {
		return nil, nothingWaiting()
	}
	store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
	state, err := store.LoadActive(ctx)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, festerrors.NotFound("no active run").
			WithHint("run 'fest workflow start' first")
	}
	steps, err := wf.NewParser().Parse(ctx, res.WorkflowDoc)
	if err != nil {
		return nil, festerrors.Wrap(err, "parsing WORKFLOW.md")
	}
	if !state.Blocked {
		return nil, nothingWaiting()
	}
	stepNum := state.CurrentStep
	if stepNum < 1 || stepNum > len(steps) {
		return nil, festerrors.Validation("invalid workflow state").
			WithField("current_step", stepNum).
			WithField("total_steps", len(steps))
	}
	step := steps[stepNum-1]
	if !step.Checkpoint.IsBlocking() {
		return nil, festerrors.Validation("step does not have a blocking checkpoint").
			WithField("step", stepNum).
			WithHint("the run is blocked, but not by a checkpoint, so there is nothing to " + verb)
	}
	if state.BlockReason == localstore.BlockReasonRejected {
		return nil, festerrors.Validation("this step was rejected and has not been re-submitted").
			WithField("step", stepNum).
			WithHint("revise the work, then run 'fest workflow advance' to submit the step for approval again")
	}
	return &standaloneCheckpoint{res: res, store: store, state: state, steps: steps, stepNum: stepNum, step: step}, nil
}

// checkExpectedStep enforces --expect-step. A caller that shows the user one
// checkpoint and then runs approve or reject passes the step it showed, so a
// run that has moved on is refused instead of having a different checkpoint
// decided. Zero means the caller did not pin a step.
func checkExpectedStep(expect, current int) error {
	if expect == 0 || expect == current {
		return nil
	}
	return festerrors.Validation("the checkpoint is not at the expected step").
		WithField("expected_step", expect).
		WithField("current_step", current).
		WithHint("the workflow has moved since this decision was prepared; run 'fest next' to see the current checkpoint")
}

// submitStandaloneCheckpoint records that a step's work is done and that it
// now waits on a human decision.
func submitStandaloneCheckpoint(ctx context.Context, store *localstore.Store, stepNum int, step wf.WorkflowStep, needStart bool) error {
	if needStart {
		if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepStart, StepIndex: stepNum}); err != nil {
			return err
		}
	}
	if err := store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepBlock, StepIndex: stepNum}); err != nil {
		return err
	}
	printStandaloneAwaitingApproval(stepNum, step)
	return nil
}

func printStandaloneAwaitingApproval(stepNum int, step wf.WorkflowStep) {
	fmt.Printf("%s Step %d: %s — work complete\n", ui.Success("✓"), stepNum, step.Name)
	fmt.Printf("%s CHECKPOINT: Awaiting approval\n\n", ui.Warning("⚠"))
	fmt.Println("This checkpoint is a human decision. Stop here, present the work, and ask the operator to decide.")
	fmt.Println("To approve (operator, in a terminal): " + ui.Accent("fest workflow approve"))
	fmt.Println("To reject with feedback: " + ui.Accent("fest workflow reject --reason \"...\""))
}

// standaloneApproveOptions carries the approve flags that apply to a
// standalone run.
type standaloneApproveOptions struct {
	Summary    string
	ExpectStep int
}

// runStandaloneApprove approves the checkpoint a standalone run is waiting at.
//
// It always requires an interactive terminal and a typed confirmation. A
// festival can delegate a checkpoint to a configured approval judge; a
// standalone workflow has no such configuration, so the only authority is the
// operator, and a flag is not proof of an operator: anything a script can pass,
// an agent can pass too.
func runStandaloneApprove(ctx context.Context, res *standalone.Result, opts standaloneApproveOptions) error {
	cp, err := loadStandaloneCheckpoint(ctx, res, "approve")
	if err != nil {
		return err
	}
	if err := checkExpectedStep(opts.ExpectStep, cp.stepNum); err != nil {
		return err
	}
	if !stdinIsInteractiveFn() {
		return festerrors.Validation("approving a standalone checkpoint requires an interactive terminal").
			WithField("step", cp.stepNum).
			WithHint("this checkpoint is a human decision; ask the operator to run 'fest workflow approve' in a terminal")
	}

	fmt.Fprintf(os.Stderr, "%s Step %d of %d: %s\n", ui.Label("Checkpoint"), cp.stepNum, len(cp.steps), cp.step.Name)
	if goal := strings.TrimSpace(cp.step.Goal); goal != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", ui.Label("Goal"), truncateForPrompt(goal, 240))
	}
	fmt.Fprintf(os.Stderr, "Type %s to confirm operator approval: ", operatorApproveToken)
	line, err := readOperatorConfirm()
	if err != nil {
		return festerrors.Wrap(err, "reading operator confirmation")
	}
	if line != operatorApproveToken {
		return festerrors.Validation("operator approval not confirmed").
			WithField("got", line).
			WithHint(fmt.Sprintf("type exactly %s to approve, or Ctrl-C to cancel", operatorApproveToken))
	}

	// The approval is two appends (three on the last step), not one. If the
	// process dies after the first, replay reads the run as not blocked with
	// the step not done, and the next 'advance' submits the checkpoint again
	// instead of skipping it. That asks the operator to approve twice, which is
	// odd but safe: no step is ever completed without a recorded approval.
	if err := cp.store.AppendEvent(ctx, localstore.Event{
		EventType: localstore.EventCheckpointApproved,
		StepIndex: cp.stepNum,
		Feedback:  strings.TrimSpace(opts.Summary),
	}); err != nil {
		return err
	}
	if err := cp.store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventStepDone, StepIndex: cp.stepNum}); err != nil {
		return err
	}
	fmt.Printf("%s Step %d: %s approved\n", ui.Success("✓"), cp.stepNum, cp.step.Name)

	if cp.stepNum >= len(cp.steps) {
		// Approving the last step finishes the run. Leaving it for a further
		// 'advance' would ask for one more command that has nothing to do.
		if err := cp.store.AppendEvent(ctx, localstore.Event{EventType: localstore.EventWorkflowRunCompleted}); err != nil {
			return err
		}
		fmt.Println(ui.Success("🎉 Workflow complete!"))
		return nil
	}
	fmt.Println("next: " + ui.Accent("fest next"))
	return nil
}

// runStandaloneReject rejects the checkpoint a standalone run is waiting at
// and records the feedback. The step stays blocked until it is re-submitted
// with 'fest workflow advance'.
func runStandaloneReject(ctx context.Context, res *standalone.Result, reason string, expectStep int) error {
	cp, err := loadStandaloneCheckpoint(ctx, res, "reject")
	if err != nil {
		return err
	}
	if err := checkExpectedStep(expectStep, cp.stepNum); err != nil {
		return err
	}
	if err := cp.store.AppendEvent(ctx, localstore.Event{
		EventType: localstore.EventCheckpointRejected,
		StepIndex: cp.stepNum,
		Feedback:  strings.TrimSpace(reason),
	}); err != nil {
		return err
	}
	fmt.Printf("%s Step %d: %s rejected\n", ui.Warning("✗"), cp.stepNum, cp.step.Name)
	fmt.Printf("%s %s\n\n", ui.Label("Feedback:"), strings.TrimSpace(reason))
	fmt.Println("Revise the work, then re-submit with: " + ui.Accent("fest workflow advance"))
	return nil
}
