package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// loadPhaseTasks summarizes the navigator's phase from a fresh read-only
// progress store, so guards and the judge request see durable state rather
// than the navigator's load-time snapshot. It returns nil for a phase with no
// sequences, which has no tasks to report.
func loadPhaseTasks(ctx context.Context, nav *wf.Navigator) (*wf.PhaseTasks, error) {
	if nav == nil || nav.Ctx == nil || nav.Ctx.PhasePath == "" {
		return nil, nil
	}
	if !shared.HasSequenceDirs(nav.Ctx.PhasePath) {
		return nil, nil
	}
	store := progress.NewStore(nav.Ctx.FestivalPath)
	if err := store.LoadReadOnly(ctx); err != nil {
		return nil, festerrors.Wrap(err, "loading progress to summarize phase tasks").
			WithField("phase", phaseNameOf(nav))
	}
	tasks, err := shared.CollectPhaseTasks(store, nav.Ctx.PhasePath, phaseNameOf(nav))
	if err != nil {
		return nil, festerrors.Wrap(err, "summarizing phase tasks")
	}
	return &tasks, nil
}

// refuseGateWithOpenBlockers refuses a GATES.md step while any task in the
// phase is blocked without a deferral. Deferred blockers do not stop the gate,
// which is the fest task defer contract, but they come back in the summary so
// callers can show them and record their count. WORKFLOW.md steps are never
// refused here.
func refuseGateWithOpenBlockers(ctx context.Context, nav *wf.Navigator) (*wf.PhaseTasks, error) {
	if nav == nil || !nav.IsGate() {
		return nil, nil
	}
	tasks, err := loadPhaseTasks(ctx, nav)
	if err != nil {
		return nil, err
	}
	if len(tasks.OpenBlockers()) > 0 {
		return tasks, openBlockersError(phaseNameOf(nav), tasks)
	}
	return tasks, nil
}

// AttachPhaseTasks gives a gate navigator its phase task summary so fest next
// and fest workflow status list blocked tasks with the gate. store is the
// progress store the navigator was just built from; nil loads one read-only.
// WORKFLOW.md navigators are left alone.
func AttachPhaseTasks(ctx context.Context, nav *wf.Navigator, store *progress.Store) error {
	if nav == nil || !nav.IsGate() || nav.Ctx == nil || !shared.HasSequenceDirs(nav.Ctx.PhasePath) {
		return nil
	}
	if store == nil {
		tasks, err := loadPhaseTasks(ctx, nav)
		if err != nil {
			return err
		}
		nav.SetPhaseTasks(tasks)
		return nil
	}
	tasks, err := shared.CollectPhaseTasks(store, nav.Ctx.PhasePath, phaseNameOf(nav))
	if err != nil {
		return festerrors.Wrap(err, "summarizing phase tasks")
	}
	nav.SetPhaseTasks(&tasks)
	return nil
}

// openBlockersError names every blocked task that has no deferral and tells
// the caller how to clear the way. resolveNavigationMode uses the same text so
// every workflow command reports the same reason.
func openBlockersError(phaseName string, tasks *wf.PhaseTasks) error {
	open := tasks.OpenBlockers()
	noun := "tasks are"
	if len(open) == 1 {
		noun = "task is"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "phase gate for %s cannot proceed: %d blocked %s not deferred", phaseName, len(open), noun)
	for _, b := range open {
		fmt.Fprintf(&sb, "\n  - %s", b.Task)
		if reason := strings.Join(strings.Fields(b.Reason), " "); reason != "" {
			fmt.Fprintf(&sb, ": %s", reason)
		}
	}
	if unfinished := tasks.InProgress + tasks.Pending; unfinished > 0 {
		fmt.Fprintf(&sb, "\n%d other task(s) in the phase are still pending or in progress", unfinished)
	}
	return festerrors.Validation(sb.String()).
		WithField("phase", phaseName).
		WithField("open_blockers", len(open)).
		WithHint(`run 'fest task defer <task> --reason "..."' for each blocker that can wait, or unblock them first; the gate cannot be advanced, approved, or judged while a blocker is open`)
}

// phaseNameOf returns the phase directory name, which is the prefix of every
// task key in the progress store.
func phaseNameOf(nav *wf.Navigator) string {
	if nav == nil || nav.Ctx == nil || nav.Ctx.PhasePath == "" {
		return ""
	}
	return filepath.Base(nav.Ctx.PhasePath)
}
