package validator

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// ValidateDeferrals checks the progress store for deferral state the verbs
// cannot produce. fest task defer requires a blocked task and a reason, and
// unblock, complete and reset all clear the flag, so either shape here means
// the store was hand edited or an event was appended directly. That is design
// doc 05 scenario A7, the cheat the campaign ledger cross-check exists to
// catch, and this is where an operator sees it.
func ValidateDeferrals(ctx context.Context, festivalPath string) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	store := progress.NewStore(festivalPath)
	if err := store.LoadReadOnly(ctx); err != nil {
		// A festival with no progress store has no deferral to be wrong about.
		return nil, nil
	}

	var deferred []*progress.TaskProgress
	for _, task := range store.AllTasks() {
		if task != nil && task.BlockerDeferred {
			deferred = append(deferred, task)
		}
	}
	sort.Slice(deferred, func(i, j int) bool { return deferred[i].TaskID < deferred[j].TaskID })

	var issues []Issue
	for _, task := range deferred {

		if task.Status != progress.StatusBlocked {
			issues = append(issues, Issue{
				Level: LevelError,
				Code:  CodeDeferralNotBlocked,
				Path:  filepath.FromSlash(task.TaskID),
				Message: "deferral flag set on a task whose status is " + task.Status +
					"; only a blocked task can carry one",
				Fix: "fest task reset " + task.TaskID + " clears the deferral, " +
					"or restore the progress store from git",
			})
		}

		if task.DeferralReason == "" {
			issues = append(issues, Issue{
				Level:   LevelError,
				Code:    CodeDeferralNoReason,
				Path:    filepath.FromSlash(task.TaskID),
				Message: "deferred blocker has no reason; fest task defer requires one, so this deferral did not come from the verb",
				Fix: "fest task unblock " + task.TaskID + " clears it, " +
					"or restore the progress store from git",
			})
		}
	}

	return issues, nil
}
