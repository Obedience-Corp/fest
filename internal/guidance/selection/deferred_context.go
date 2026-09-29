package selection

import (
	"time"

	"github.com/Obedience-Corp/fest/internal/deps"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// TaskRef names one deferred blocker the selected task depends on. DeferredAt
// is RFC 3339, the format the progress store writes and every other fest
// timestamp uses; the payload carries no other timestamp to copy.
type TaskRef struct {
	Task           string `json:"task"`
	BlockerMessage string `json:"blocker_message"`
	DeferralReason string `json:"deferral_reason,omitempty"`
	DeferredAt     string `json:"deferred_at,omitempty"`
}

// attachDeferredContext fills the selected task's deferred dependencies and its
// operator notes from the manager the status update already loaded. The
// deferred set is the task's own hard dependencies, never the festival's, so a
// deferral in an unrelated sequence puts no line under an unrelated task. Both
// fields stay empty when there is nothing to say, which is what keeps the JSON
// additive.
func (s *Selector) attachDeferredContext(mgr *progress.Manager, graph *deps.Graph,
	primary *deps.Task, result *NextTaskResult) {
	if mgr == nil || graph == nil || primary == nil || result == nil {
		return
	}

	for _, dep := range deferredDependencies(graph, primary) {
		result.DeferredBlockers = append(result.DeferredBlockers, s.taskRefFor(mgr, dep))
	}

	if record, ok := progress.ResolveTaskProgress(mgr.Store(), s.festivalPath, primary.Path); ok && record != nil {
		result.OperatorNotes = record.OperatorNotes
	}
}

// deferredDependencies returns the task's hard dependencies whose blockers an
// operator deferred, in festival order. Soft dependencies are excluded because
// they never gated the task in the first place.
func deferredDependencies(graph *deps.Graph, primary *deps.Task) []*deps.Task {
	var deferred []*deps.Task
	// A task can carry the same dependency twice, by number and by name, and
	// the graph keeps both edges. One line per blocker, not one per edge.
	seen := make(map[string]bool)
	for _, dep := range graph.GetRequiredDependencies(primary.ID) {
		if dep == nil || dep.Status != "blocked" || !dep.BlockerDeferred || seen[dep.ID] {
			continue
		}
		seen[dep.ID] = true
		deferred = append(deferred, dep)
	}
	sortSweepCandidates(deferred)
	return deferred
}

// taskRefFor builds the reference from the store record. A task whose key
// cannot be normalised, or that has no record, still produces a reference with
// whatever is known rather than dropping the blocker from the list.
func (s *Selector) taskRefFor(mgr *progress.Manager, task *deps.Task) TaskRef {
	ref := TaskRef{Task: task.ID}
	if key, err := progress.NormalizeTaskID(s.festivalPath, task.Path); err == nil {
		ref.Task = key
	}

	record, ok := progress.ResolveTaskProgress(mgr.Store(), s.festivalPath, task.Path)
	if !ok || record == nil {
		return ref
	}

	ref.BlockerMessage = record.BlockerMessage
	ref.DeferralReason = record.DeferralReason
	if record.BlockerDeferredAt != nil {
		ref.DeferredAt = record.BlockerDeferredAt.UTC().Format(time.RFC3339)
	}
	return ref
}

// openBlockers names the blocked tasks no operator has deferred, in festival
// order. A deferred blocker is left out because it is no longer what the
// executor is waiting on; the sweep brings it back on its own.
func (s *Selector) openBlockers(mgr *progress.Manager, graph *deps.Graph) []TaskRef {
	if mgr == nil || graph == nil {
		return nil
	}

	var blocked []*deps.Task
	for _, task := range graph.Tasks {
		if task == nil || task.Status != "blocked" || task.BlockerDeferred {
			continue
		}
		blocked = append(blocked, task)
	}
	if len(blocked) == 0 {
		return nil
	}
	sortSweepCandidates(blocked)

	refs := make([]TaskRef, 0, len(blocked))
	for _, task := range blocked {
		refs = append(refs, s.taskRefFor(mgr, task))
	}
	return refs
}
