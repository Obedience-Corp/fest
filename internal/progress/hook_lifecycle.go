package progress

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

// newLifecycleHookRunner is an injectable seam so tests can fake hook execution.
var newLifecycleHookRunner = hooks.NewRunner

// LifecycleHookRequest describes one lifecycle verb whose step bindings run
// through the hook runner (spec 03, D4).
type LifecycleHookRequest struct {
	FestivalPath string
	Phase        string // audit-event coordinate
	Step         int    // audit-event coordinate (0 outside workflow steps)
	Task         string // audit-event coordinate (task or sequence id, when applicable)
	Level        hooks.Level
	Verb         hooks.Verb
	Pre          []string
	Post         []string
	HumanGate    bool // approval: human-required on the step (spec 04)
}

// PlanLifecycleHooks resolves the effective hook set and plans the request's
// bindings. On a human-required step every automation hook is skipped and
// recorded as skipped: human-gate, without requiring hook resolution.
func PlanLifecycleHooks(ctx context.Context, req LifecycleHookRequest) ([]hooks.PlannedHook, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled before hook planning")
	}
	if len(req.Pre) == 0 && len(req.Post) == 0 {
		return nil, nil
	}
	if req.HumanGate {
		var eff *hooks.Effective
		return hooks.ApplyHumanGate(eff.PlanBindings(req.Level, req.Pre, req.Post)), nil
	}
	eff, err := hooks.LoadAndResolve(ctx, req.FestivalPath)
	if err != nil {
		return nil, errors.Wrap(err, "resolving hooks")
	}
	return eff.PlanBindings(req.Level, req.Pre, req.Post), nil
}

// RunLifecycleStage executes one timing of the planned bindings and queues one
// wf_hook_run event per record on the store. The caller owns event flushing
// (Store.SaveEvents / Store.Save). blocked reports a fail-closed failure: for
// pre the verb must not apply; for post the verb stays applied but the caller
// should surface the failure.
func RunLifecycleStage(ctx context.Context, store *Store, runner *hooks.Runner, req LifecycleHookRequest, planned []hooks.PlannedHook, timing hooks.Timing) ([]hooks.HookRun, bool, error) {
	if len(planned) == 0 {
		return nil, false, nil
	}
	var (
		runs    []hooks.HookRun
		blocked bool
		err     error
	)
	if timing == hooks.TimingPre {
		runs, blocked, err = runner.RunPre(ctx, req.Level, req.Verb, planned, nil)
	} else {
		runs, blocked, err = runner.RunPost(ctx, req.Level, req.Verb, planned, nil)
	}
	queueLifecycleRuns(store, req, runs)
	return runs, blocked, err
}

func queueLifecycleRuns(store *Store, req LifecycleHookRequest, runs []hooks.HookRun) {
	if store == nil {
		return
	}
	for _, run := range runs {
		ev := HookRunEvent(req.Phase, req.Step, run)
		ev.Task = req.Task
		store.QueueEvent(ev)
	}
}

// StepHookBindingsFromFrontmatter extracts the hook bindings and human-gate
// flag from a task or goal document's frontmatter. Unparseable frontmatter
// yields no bindings.
func StepHookBindingsFromFrontmatter(content []byte) (pre, post []string, humanGate bool) {
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return nil, nil, false
	}
	return fm.Hooks.Pre, fm.Hooks.Post, strings.EqualFold(strings.TrimSpace(fm.Approval), "human-required")
}

// BlockedHookError names the fail-closed hook that blocked a lifecycle verb.
func BlockedHookError(verb hooks.Verb, runs []hooks.HookRun) error {
	for _, run := range runs {
		if !run.Blocked {
			continue
		}
		e := errors.Validation("blocked by fail-closed hook").
			WithField("hook", run.Name).
			WithField("verb", string(verb)).
			WithField("outcome", string(run.Outcome)).
			WithHint("fix the hook failure, or set fail: open / enabled: false for the hook, then retry")
		if run.Err != nil {
			e = e.WithField("error", run.Err.Error())
		}
		return e
	}
	return errors.Validation("blocked by fail-closed hook").WithField("verb", string(verb))
}
