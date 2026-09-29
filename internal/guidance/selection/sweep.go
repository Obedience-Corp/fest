package selection

import (
	"context"
	"sort"
	"strconv"

	"github.com/Obedience-Corp/fest/internal/deps"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// SweepInfo describes an end-of-festival sweep hand-off. It is additive: a
// normal fest next result carries no sweep object at all.
type SweepInfo struct {
	Number         int      `json:"number"`
	DeferredTotal  int      `json:"deferred_total"`
	BlockerMessage string   `json:"blocker_message,omitempty"`
	Attempts       []string `json:"attempts,omitempty"`
	DeferralReason string   `json:"deferral_reason,omitempty"`
	CompletedSince []string `json:"completed_since,omitempty"`

	// Remaining is set only on the terminal sweep result, when every deferred
	// blocker has been revisited and nothing completed to justify another
	// sweep. It carries the data behind the remain message so the renderer
	// does not have to rebuild the sentence.
	Remaining []SweepDeferredBlocker `json:"remaining,omitempty"`
}

// SweepDeferredBlocker is one still-deferred blocker in the terminal sweep
// result.
type SweepDeferredBlocker struct {
	Task           string `json:"task"`
	BlockerMessage string `json:"blocker_message,omitempty"`
	DeferralReason string `json:"deferral_reason,omitempty"`
}

// sweepCandidates returns the deferred blockers of a fully settled festival.
// It returns nil as soon as it finds an unsettled task, because a festival with
// work left is making ordinary progress and must never enter a sweep.
func sweepCandidates(graph *deps.Graph) []*deps.Task {
	var deferred []*deps.Task
	for _, task := range graph.Tasks {
		if !task.IsSettled() {
			return nil
		}
		if task.Status == "blocked" && task.BlockerDeferred {
			deferred = append(deferred, task)
		}
	}
	return deferred
}

// sortSweepCandidates puts the deferred tasks in festival order: phase, then
// sequence, then task number, with the ID as a final tiebreak so the order is
// total and the tests are deterministic.
func sortSweepCandidates(tasks []*deps.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		switch {
		case tasks[i].PhasePath != tasks[j].PhasePath:
			return tasks[i].PhasePath < tasks[j].PhasePath
		case tasks[i].SequencePath != tasks[j].SequencePath:
			return tasks[i].SequencePath < tasks[j].SequencePath
		case tasks[i].Number != tasks[j].Number:
			return tasks[i].Number < tasks[j].Number
		default:
			return tasks[i].ID < tasks[j].ID
		}
	})
}

// findSweepTask hands a deferred blocker back to the executor when every task
// in the festival is settled and at least one is deferred. It returns nil when
// the festival is not in a sweep and when every deferred task has already been
// revisited in the current sweep; deciding whether that state starts a new
// sweep belongs to the sweep termination path.
func (s *Selector) findSweepTask(ctx context.Context, graph *deps.Graph, location *LocationInfo) (*NextTaskResult, error) {
	candidates := sweepCandidates(graph)
	if len(candidates) == 0 {
		return nil, nil
	}

	mgr, err := progress.NewManager(ctx, s.festivalPath)
	if err != nil {
		return nil, errors.Wrap(err, "loading progress for the sweep")
	}

	sortSweepCandidates(candidates)

	keys := make(map[string]string, len(candidates))
	for _, task := range candidates {
		key, keyErr := progress.NormalizeTaskID(s.festivalPath, task.Path)
		if keyErr != nil {
			return nil, keyErr
		}
		keys[task.ID] = key
	}

	current := mgr.SweepState().Current
	if current == 0 {
		current = 1
		if err := mgr.StartSweep(ctx, current); err != nil {
			return nil, err
		}
	}

	// A new sweep needs a completion in the previous one and every candidate is
	// unrevisited the moment a sweep starts, so this loop turns at most twice.
	// The bound is explicit so a regression fails instead of spinning.
	for range len(candidates) + 1 {
		revisits := mgr.SweepState().LastRevisit
		for _, task := range candidates {
			if revisits[keys[task.ID]] < current {
				return s.handBackDeferredTask(ctx, mgr, task, keys[task.ID], current, len(candidates), location)
			}
		}

		advance, advanceErr := mgr.SweepMayAdvance(ctx, current)
		if advanceErr != nil {
			return nil, advanceErr
		}
		if !advance {
			return s.sweepTerminalResult(mgr, candidates, keys, current, location), nil
		}

		current++
		if err := mgr.StartSweep(ctx, current); err != nil {
			return nil, err
		}
	}

	return nil, errors.New("sweep did not terminate within its candidate bound").
		WithField("sweep", current).
		WithField("deferred", len(candidates))
}

func (s *Selector) handBackDeferredTask(ctx context.Context, mgr *progress.Manager, task *deps.Task,
	key string, sweep, deferredTotal int, location *LocationInfo) (*NextTaskResult, error) {
	info := &SweepInfo{Number: sweep, DeferredTotal: deferredTotal}
	var notes []string
	if record, ok := progress.ResolveTaskProgress(mgr.Store(), s.festivalPath, task.Path); ok && record != nil {
		info.BlockerMessage = record.BlockerMessage
		info.Attempts = record.BlockerAttempts
		info.DeferralReason = record.DeferralReason
		notes = record.OperatorNotes
		if record.BlockerDeferredAt != nil {
			since, sinceErr := mgr.CompletedSince(ctx, *record.BlockerDeferredAt)
			if sinceErr != nil {
				return nil, sinceErr
			}
			info.CompletedSince = since
		}
	}

	if err := mgr.RecordBlockerRevisit(ctx, key, sweep); err != nil {
		return nil, err
	}

	return &NextTaskResult{
		Task:          s.taskToInfo(task),
		Reason:        "Revisiting a deferred blocker in sweep " + strconv.Itoa(sweep),
		Location:      location,
		Sweep:         info,
		OperatorNotes: notes,
	}, nil
}

// sweepTerminalResult is the executor's dead end: every deferred blocker was
// revisited in this sweep and nothing completed, so there is nothing left for
// an executor to run. The only route past it is a guarded --force.
func (s *Selector) sweepTerminalResult(mgr *progress.Manager, candidates []*deps.Task,
	keys map[string]string, sweep int, location *LocationInfo) *NextTaskResult {
	info := &SweepInfo{Number: sweep, DeferredTotal: len(candidates)}
	for _, task := range candidates {
		entry := SweepDeferredBlocker{Task: keys[task.ID]}
		if record, ok := progress.ResolveTaskProgress(mgr.Store(), s.festivalPath, task.Path); ok && record != nil {
			entry.BlockerMessage = record.BlockerMessage
			entry.DeferralReason = record.DeferralReason
		}
		info.Remaining = append(info.Remaining, entry)
	}

	return &NextTaskResult{
		Reason: strconv.Itoa(len(candidates)) + " deferred blockers remain after sweep " +
			strconv.Itoa(sweep) + ". Unblock them, or promote with --force.",
		Location: location,
		Sweep:    info,
	}
}
