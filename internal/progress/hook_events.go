package progress

import (
	"time"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

// HookRunEvent maps a hooks.HookRun into a festival-local wf_hook_run event.
// Callers queue via QueueEvent and Save. Never writes the campaign ledger.
func HookRunEvent(phase string, step int, run hooks.HookRun) *ProgressEvent {
	return &ProgressEvent{
		Timestamp:    time.Now().UTC(),
		Event:        EventWorkflowHookRun,
		Phase:        phase,
		Step:         step,
		HookName:     run.Name,
		HookLayer:    string(run.Layer),
		HookTiming:   string(run.Timing),
		HookVerb:     string(run.Verb),
		HookOutcome:  string(run.Outcome),
		HookSkip:     string(run.Skip),
		HookExitCode: run.ExitCode,
		HookMillis:   run.Duration.Milliseconds(),
		HookFail:     string(run.Fail),
		HookBlocked:  run.Blocked,
	}
}

// QueueHookRuns appends one wf_hook_run event per run on the store.
func QueueHookRuns(store *Store, phase string, step int, runs []hooks.HookRun) {
	if store == nil {
		return
	}
	for _, run := range runs {
		store.QueueEvent(HookRunEvent(phase, step, run))
	}
}
