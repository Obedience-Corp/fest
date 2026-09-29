package selection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// writeSweepFixture lays out a festival from festival-relative task paths of
// the form PHASE/SEQUENCE/NN_name.md and returns the festival path.
func writeSweepFixture(t *testing.T, relPaths ...string) string {
	t.Helper()

	festivalPath := t.TempDir()
	for _, rel := range relPaths {
		full := filepath.Join(festivalPath, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		body := "# " + filepath.Base(rel) + "\n\nWork.\n"
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return festivalPath
}

func writeSweepTaskWithDependency(t *testing.T, festivalPath, rel, dependency string) {
	t.Helper()
	full := filepath.Join(festivalPath, filepath.FromSlash(rel))
	body := "---\nfest_type: task\nfest_dependencies:\n  - " + dependency + "\n---\n\n# " +
		filepath.Base(rel) + "\n\nWork.\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func sweepManager(t *testing.T, festivalPath string) *progress.Manager {
	t.Helper()
	mgr, err := progress.NewManager(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return mgr
}

// seedSweepState marks the completed tasks complete and reports plus defers the
// deferred ones through the real manager and the real deferral event.
func seedSweepState(t *testing.T, festivalPath string, completed, deferred []string) {
	t.Helper()
	ctx := t.Context()
	mgr := sweepManager(t, festivalPath)

	for _, rel := range completed {
		if err := mgr.MarkComplete(ctx, rel); err != nil {
			t.Fatalf("MarkComplete(%s) error = %v", rel, err)
		}
	}
	for _, rel := range deferred {
		if err := mgr.ReportBlocker(ctx, rel, "provider API removed", []string{"checked the changelog"}); err != nil {
			t.Fatalf("ReportBlocker(%s) error = %v", rel, err)
		}
		if err := mgr.DeferBlocker(ctx, rel, "the vendor replies next week", progress.DeferralAudit{
			Actor: "operator", TTY: true, DeferredBy: "Ada Lovelace",
		}); err != nil {
			t.Fatalf("DeferBlocker(%s) error = %v", rel, err)
		}
	}
}

func readSweepEvents(t *testing.T, festivalPath string) []progress.ProgressEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festivalPath, progress.ProgressDir, progress.ProgressEventsFile))
	if err != nil {
		t.Fatalf("reading the event log: %v", err)
	}

	var events []progress.ProgressEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var event progress.ProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decoding event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func countSweepEvents(events []progress.ProgressEvent, kind progress.EventType) int {
	count := 0
	for _, event := range events {
		if event.Event == kind {
			count++
		}
	}
	return count
}

func TestSweepNotEnteredWhileWorkRemains(t *testing.T) {
	const (
		pending  = "001_PHASE/01_seq/01_pending.md"
		deferred = "001_PHASE/01_seq/02_deferred.md"
	)
	festivalPath := writeSweepFixture(t, pending, deferred)
	seedSweepState(t, festivalPath, nil, []string{deferred})

	result, err := NewSelector(festivalPath).FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if result.Sweep != nil {
		t.Fatalf("Sweep = %+v, want none while a pending task remains", result.Sweep)
	}
	if result.Task == nil || !strings.Contains(result.Task.Path, "01_pending") {
		t.Fatalf("Task = %+v, want the pending task", result.Task)
	}
	if countSweepEvents(readSweepEvents(t, festivalPath), progress.EventSweepStarted) != 0 {
		t.Error("a sweep was started although the festival still had work")
	}
}

func TestSweepEntryHandsBackTheDeferredBlocker(t *testing.T) {
	const (
		done     = "001_PHASE/01_seq/01_done.md"
		deferred = "001_PHASE/01_seq/02_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, deferred)
	seedSweepState(t, festivalPath, []string{done}, []string{deferred})

	result, err := NewSelector(festivalPath).FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if result.Sweep == nil {
		t.Fatalf("Sweep = nil, want a sweep; reason was %q", result.Reason)
	}
	if result.Sweep.Number != 1 {
		t.Errorf("Sweep.Number = %d, want 1", result.Sweep.Number)
	}
	if result.Sweep.DeferredTotal != 1 {
		t.Errorf("Sweep.DeferredTotal = %d, want 1", result.Sweep.DeferredTotal)
	}
	if result.Sweep.BlockerMessage != "provider API removed" {
		t.Errorf("Sweep.BlockerMessage = %q, want the executor blocker", result.Sweep.BlockerMessage)
	}
	if strings.Join(result.Sweep.Attempts, ",") != "checked the changelog" {
		t.Errorf("Sweep.Attempts = %v, want the recorded attempts", result.Sweep.Attempts)
	}
	if result.Sweep.DeferralReason != "the vendor replies next week" {
		t.Errorf("Sweep.DeferralReason = %q, want the operator reason", result.Sweep.DeferralReason)
	}
	if result.Task == nil || !strings.Contains(result.Task.Path, "02_deferred") {
		t.Fatalf("Task = %+v, want the deferred task", result.Task)
	}
	if result.FestivalComplete {
		t.Error("FestivalComplete = true during a sweep; a chain downstream festival would start early")
	}
}

func TestSweepStartedIsAppendedOncePerSweep(t *testing.T) {
	const (
		done           = "001_PHASE/01_seq/01_done.md"
		firstDeferred  = "001_PHASE/01_seq/02_deferred.md"
		secondDeferred = "001_PHASE/01_seq/03_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, firstDeferred, secondDeferred)
	seedSweepState(t, festivalPath, []string{done}, []string{firstDeferred, secondDeferred})

	selector := NewSelector(festivalPath)
	for range 2 {
		if _, err := selector.FindNext(t.Context(), festivalPath); err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
	}

	if got := countSweepEvents(readSweepEvents(t, festivalPath), progress.EventSweepStarted); got != 1 {
		t.Errorf("sweep_started events = %d, want 1 for two hand-offs in one sweep", got)
	}
}

func TestSweepRecordsARevisitPerHandOff(t *testing.T) {
	const (
		done           = "001_PHASE/01_seq/01_done.md"
		firstDeferred  = "001_PHASE/01_seq/02_deferred.md"
		secondDeferred = "001_PHASE/01_seq/03_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, firstDeferred, secondDeferred)
	seedSweepState(t, festivalPath, []string{done}, []string{firstDeferred, secondDeferred})

	selector := NewSelector(festivalPath)
	for range 2 {
		if _, err := selector.FindNext(t.Context(), festivalPath); err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
	}

	events := readSweepEvents(t, festivalPath)
	if got := countSweepEvents(events, progress.EventBlockerRevisited); got != 2 {
		t.Fatalf("blocker_revisited events = %d, want one per hand-off", got)
	}
	revisited := map[string]int{}
	for _, event := range events {
		if event.Event == progress.EventBlockerRevisited {
			revisited[event.Task] = event.Sweep
		}
	}
	for _, rel := range []string{firstDeferred, secondDeferred} {
		if revisited[rel] != 1 {
			t.Errorf("revisit sweep for %s = %d, want 1", rel, revisited[rel])
		}
	}
}

func TestSweepSelectsInFestivalOrder(t *testing.T) {
	const (
		done   = "001_PHASE/01_seq/01_done.md"
		first  = "001_PHASE/01_seq/02_deferred.md"
		second = "001_PHASE/02_seq/01_deferred.md"
		third  = "002_PHASE/01_seq/01_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, first, second, third)
	seedSweepState(t, festivalPath, []string{done}, []string{third, second, first})

	selector := NewSelector(festivalPath)
	var handed []string
	for range 3 {
		result, err := selector.FindNext(t.Context(), festivalPath)
		if err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
		if result.Task == nil {
			t.Fatalf("FindNext() returned no task on hand-off %d: %q", len(handed)+1, result.Reason)
		}
		handed = append(handed, result.Task.Path)
	}

	want := []string{first, second, third}
	for i, path := range handed {
		if !strings.HasSuffix(filepath.ToSlash(path), want[i]) {
			t.Errorf("hand-off %d = %q, want %q", i+1, path, want[i])
		}
	}
}

func TestSweepSkipsATaskAlreadyRevisitedInThisSweep(t *testing.T) {
	const (
		done   = "001_PHASE/01_seq/01_done.md"
		first  = "001_PHASE/01_seq/02_deferred.md"
		second = "001_PHASE/01_seq/03_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, first, second)
	seedSweepState(t, festivalPath, []string{done}, []string{first, second})

	selector := NewSelector(festivalPath)

	firstResult, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if firstResult.Task == nil || !strings.Contains(firstResult.Task.Path, "02_deferred") {
		t.Fatalf("first hand-off = %+v, want the first deferred task", firstResult.Task)
	}

	secondResult, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if secondResult.Task == nil || !strings.Contains(secondResult.Task.Path, "03_deferred") {
		t.Fatalf("second hand-off = %+v, want the task not yet revisited in this sweep", secondResult.Task)
	}

	exhausted, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if exhausted.Task != nil {
		t.Errorf("Task = %+v, want nothing an executor can run", exhausted.Task)
	}
	wantReason := "2 deferred blockers remain after sweep 1. Unblock them, or promote with --force."
	if exhausted.Reason != wantReason {
		t.Errorf("Reason = %q, want %q", exhausted.Reason, wantReason)
	}
	if exhausted.Sweep == nil || len(exhausted.Sweep.Remaining) != 2 {
		t.Fatalf("Sweep = %+v, want both deferred blockers listed", exhausted.Sweep)
	}
	for _, entry := range exhausted.Sweep.Remaining {
		if entry.BlockerMessage == "" || entry.DeferralReason == "" {
			t.Errorf("remaining entry %+v must carry the blocker and the reason", entry)
		}
	}
	if exhausted.FestivalComplete {
		t.Error("FestivalComplete = true with deferred blockers open")
	}
}

func TestSweepRevisitsADeferredDependent(t *testing.T) {
	const (
		done       = "001_PHASE/01_seq/01_done.md"
		dependency = "001_PHASE/01_seq/02_dependency.md"
		dependent  = "001_PHASE/01_seq/03_dependent.md"
	)
	festivalPath := writeSweepFixture(t, done, dependency, dependent)
	writeSweepTaskWithDependency(t, festivalPath, dependent, "02_dependency")
	seedSweepState(t, festivalPath, []string{done}, []string{dependency, dependent})

	selector := NewSelector(festivalPath)

	first, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if first.Task == nil || !strings.Contains(first.Task.Path, "02_dependency") {
		t.Fatalf("first hand-off = %+v, want the deferred dependency", first.Task)
	}

	second, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if second.Task == nil || !strings.Contains(second.Task.Path, "03_dependent") {
		t.Fatalf("second hand-off = %+v, want the deferred dependent revisited anyway", second.Task)
	}
}

func TestSweepListsWhatCompletedSinceTheDeferral(t *testing.T) {
	const (
		before   = "001_PHASE/01_seq/01_before.md"
		deferred = "001_PHASE/01_seq/02_deferred.md"
		after    = "001_PHASE/01_seq/03_after.md"
	)
	festivalPath := writeSweepFixture(t, before, deferred, after)
	ctx := t.Context()
	mgr := sweepManager(t, festivalPath)

	if err := mgr.MarkComplete(ctx, before); err != nil {
		t.Fatalf("MarkComplete(before) error = %v", err)
	}
	if err := mgr.ReportBlocker(ctx, deferred, "provider API removed", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.DeferBlocker(ctx, deferred, "the vendor replies next week", progress.DeferralAudit{Actor: "operator", TTY: true}); err != nil {
		t.Fatalf("DeferBlocker() error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := mgr.MarkComplete(ctx, after); err != nil {
		t.Fatalf("MarkComplete(after) error = %v", err)
	}

	result, err := NewSelector(festivalPath).FindNext(ctx, festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if result.Sweep == nil {
		t.Fatalf("Sweep = nil, want a sweep; reason was %q", result.Reason)
	}
	joined := strings.Join(result.Sweep.CompletedSince, ",")
	if !strings.Contains(joined, "03_after") {
		t.Errorf("CompletedSince = %v, want the completion that followed the deferral", result.Sweep.CompletedSince)
	}
	if strings.Contains(joined, "01_before") {
		t.Errorf("CompletedSince = %v, want nothing that completed before the deferral", result.Sweep.CompletedSince)
	}
}

func TestNormalResultCarriesNoSweepKey(t *testing.T) {
	const (
		done    = "001_PHASE/01_seq/01_done.md"
		pending = "001_PHASE/01_seq/02_pending.md"
	)
	festivalPath := writeSweepFixture(t, done, pending)
	seedSweepState(t, festivalPath, []string{done}, nil)

	result, err := NewSelector(festivalPath).FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshalling the result: %v", err)
	}
	if strings.Contains(string(encoded), "sweep") {
		t.Errorf("a normal fest next result must carry no sweep key:\n%s", encoded)
	}
}

// TestSweepTerminatesWithEveryTaskReblocked is design doc 05 scenario H1. The
// loop is bounded so a regression fails fast instead of hanging: an unbounded
// sweep is exactly the failure this test exists to catch.
func TestSweepTerminatesWithEveryTaskReblocked(t *testing.T) {
	const (
		done   = "001_PHASE/01_seq/01_done.md"
		first  = "001_PHASE/01_seq/02_deferred.md"
		second = "001_PHASE/01_seq/03_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, first, second)
	seedSweepState(t, festivalPath, []string{done}, []string{first, second})

	selector := NewSelector(festivalPath)
	mgr := sweepManager(t, festivalPath)

	const callBound = 8
	var terminal *NextTaskResult
	calls := 0
	for range callBound {
		calls++
		result, err := selector.FindNext(t.Context(), festivalPath)
		if err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
		if result.Task == nil {
			terminal = result
			break
		}
		// The executor tries again and reports the same blocker.
		key, err := progress.NormalizeTaskID(festivalPath, result.Task.Path)
		if err != nil {
			t.Fatalf("NormalizeTaskID() error = %v", err)
		}
		if err := mgr.ReportBlocker(t.Context(), key, "still gone", []string{"tried again"}); err != nil {
			t.Fatalf("ReportBlocker() error = %v", err)
		}
	}

	if terminal == nil {
		t.Fatalf("the sweep never terminated within %d calls", callBound)
	}
	if calls > 3 {
		t.Errorf("the sweep took %d calls for two deferred tasks, want three", calls)
	}

	wantReason := "2 deferred blockers remain after sweep 1. Unblock them, or promote with --force."
	if terminal.Reason != wantReason {
		t.Errorf("Reason = %q, want %q", terminal.Reason, wantReason)
	}
	if terminal.Sweep == nil || len(terminal.Sweep.Remaining) != 2 {
		t.Fatalf("Sweep = %+v, want both deferred blockers listed", terminal.Sweep)
	}
	if terminal.FestivalComplete {
		t.Error("FestivalComplete = true at the sweep dead end; the executor could call the festival done")
	}

	events := readSweepEvents(t, festivalPath)
	if got := countSweepEvents(events, progress.EventSweepStarted); got != 1 {
		t.Errorf("sweep_started events = %d, want 1; a sweep with no completion must not start another", got)
	}
}

// TestNewSweepStartsAfterACompletion is design doc 05 scenario H3.
func TestNewSweepStartsAfterACompletion(t *testing.T) {
	const (
		done   = "001_PHASE/01_seq/01_done.md"
		first  = "001_PHASE/01_seq/02_deferred.md"
		second = "001_PHASE/01_seq/03_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, first, second)
	seedSweepState(t, festivalPath, []string{done}, []string{first, second})

	selector := NewSelector(festivalPath)
	mgr := sweepManager(t, festivalPath)

	for range 2 {
		if _, err := selector.FindNext(t.Context(), festivalPath); err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
	}

	if err := mgr.MarkComplete(t.Context(), first); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	result, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if result.Sweep == nil {
		t.Fatalf("Sweep = nil, want a second sweep; reason was %q", result.Reason)
	}
	if result.Sweep.Number != 2 {
		t.Errorf("Sweep.Number = %d, want 2", result.Sweep.Number)
	}
	if result.Task == nil || !strings.Contains(result.Task.Path, "03_deferred") {
		t.Fatalf("Task = %+v, want the other deferred task revisited", result.Task)
	}
	if !strings.Contains(strings.Join(result.Sweep.CompletedSince, ","), "02_deferred") {
		t.Errorf("CompletedSince = %v, want the sweep completion listed", result.Sweep.CompletedSince)
	}

	if got := countSweepEvents(readSweepEvents(t, festivalPath), progress.EventSweepStarted); got != 2 {
		t.Errorf("sweep_started events = %d, want 2", got)
	}
}

// TestForcedCompletionPreservesDeferralAndReopensTheSweep is design doc 05
// scenarios D6 and H6. Forcing a festival complete records what was dropped and
// leaves the deferral in place, so a reopened festival hands the work back.
func TestForcedCompletionPreservesDeferralAndReopensTheSweep(t *testing.T) {
	const (
		done     = "001_PHASE/01_seq/01_done.md"
		deferred = "001_PHASE/01_seq/02_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, deferred)
	seedSweepState(t, festivalPath, []string{done}, []string{deferred})

	selector := NewSelector(festivalPath)
	if _, err := selector.FindNext(t.Context(), festivalPath); err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}

	terminal, err := selector.FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if terminal.Task != nil {
		t.Fatalf("Task = %+v, want the sweep dead end", terminal.Task)
	}

	mgr := sweepManager(t, festivalPath)
	if err := mgr.RecordForcedCompletion(t.Context(), progress.DeferralAudit{
		Actor: "operator", TTY: true, DeferredBy: "Ada Lovelace",
	}, []string{deferred}); err != nil {
		t.Fatalf("RecordForcedCompletion() error = %v", err)
	}

	record, ok := reloadSweepTask(t, festivalPath, deferred)
	if !ok {
		t.Fatal("the dropped task is missing from the store")
	}
	if !record.BlockerDeferred || record.Status != progress.StatusBlocked {
		t.Fatalf("dropped task = %+v, want it still blocked and still deferred", record)
	}

	reopened, err := NewSelector(festivalPath).FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	if reopened.Sweep == nil {
		t.Fatalf("Sweep = nil, want a fresh sweep after the reopen; reason was %q", reopened.Reason)
	}
	if reopened.Sweep.Number != 2 {
		t.Errorf("Sweep.Number = %d, want a second sweep", reopened.Sweep.Number)
	}
	if reopened.Task == nil || !strings.Contains(reopened.Task.Path, "02_deferred") {
		t.Fatalf("Task = %+v, want the dropped task handed back", reopened.Task)
	}
}

func reloadSweepTask(t *testing.T, festivalPath, taskID string) (*progress.TaskProgress, bool) {
	t.Helper()
	mgr, err := progress.NewManager(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return mgr.GetTaskProgress(taskID)
}
