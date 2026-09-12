package progress

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

func TestHookRunEvent_JSONShape(t *testing.T) {
	run := hooks.HookRun{
		Name: "approval_judge", Layer: hooks.LayerFestivals, Timing: hooks.TimingPost,
		Level: hooks.LevelGate, Verb: hooks.VerbGateApprove, Outcome: hooks.OutcomePass,
		ExitCode: 0, Duration: 12 * time.Millisecond, Fail: hooks.FailClosed,
	}
	ev := HookRunEvent("001_IMPLEMENT", 2, run)
	if ev.Event != EventWorkflowHookRun {
		t.Fatalf("event type = %s", ev.Event)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"ts", "event", "phase", "step", "hook_name", "hook_layer",
		"hook_timing", "hook_verb", "hook_outcome", "hook_duration_ms", "hook_fail",
	} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing json key %q in %s", key, raw)
		}
	}
	if m["event"] != "wf_hook_run" {
		t.Fatalf("event = %v", m["event"])
	}
}

func TestQueueHookRuns_WritesOneLinePerRun(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	runs := []hooks.HookRun{
		{Name: "a", Outcome: hooks.OutcomeFail, Fail: hooks.FailClosed, Blocked: true, Timing: hooks.TimingPost, Verb: hooks.VerbGateApprove},
		{Name: "b", Outcome: hooks.OutcomeSkipped, Skip: hooks.SkipShortCircuit, Timing: hooks.TimingPost, Verb: hooks.VerbGateApprove},
	}
	QueueHookRuns(store, "001_IMPLEMENT", 1, runs)
	if err := store.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".fest", "progress_events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d data=%s", len(lines), data)
	}
	for _, line := range lines {
		if !strings.Contains(line, `"wf_hook_run"`) {
			t.Fatalf("line missing wf_hook_run: %s", line)
		}
	}
}

func TestRecentHookRunsByStep_FiltersByEventTypeAndPhaseKey(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}
	QueueHookRuns(store, "gate:001_IMPLEMENT", 1, []hooks.HookRun{{Name: "approval_judge", Outcome: hooks.OutcomePass}})
	QueueHookRuns(store, "001_INGEST", 1, []hooks.HookRun{{Name: "other_phase", Outcome: hooks.OutcomePass}})
	store.QueueEvent(&ProgressEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventWorkflowJudgeReturned,
		Phase:     "gate:001_IMPLEMENT",
		Step:      1,
	})
	if err := store.Save(ctx); err != nil {
		t.Fatal(err)
	}

	byStep, err := store.RecentHookRunsByStep(ctx, "gate:001_IMPLEMENT")
	if err != nil {
		t.Fatalf("RecentHookRunsByStep: %v", err)
	}
	if len(byStep) != 1 {
		t.Fatalf("steps = %d, want 1 (other phases and other event types excluded)", len(byStep))
	}
	runs := byStep[1]
	if len(runs) != 1 || runs[0].HookName != "approval_judge" {
		t.Fatalf("step 1 runs = %+v, want one approval_judge run", runs)
	}
}

// A busy step must not evict a quiet step's only run: the cap is per step, so
// a gate that judged many times and a gate that judged once both stay visible
// in one snapshot.
func TestRecentHookRunsByStep_CapsPerStepAndKeepsAQuietStep(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}
	busy := MaxRecentHookRuns + 5
	for i := range busy {
		QueueHookRuns(store, "001_INGEST", 1, []hooks.HookRun{{
			Name:    "busy-" + strconv.Itoa(i),
			Outcome: hooks.OutcomePass,
		}})
	}
	QueueHookRuns(store, "001_INGEST", 2, []hooks.HookRun{{
		Name:    "quiet-only-run",
		Outcome: hooks.OutcomePass,
	}})
	if err := store.Save(ctx); err != nil {
		t.Fatal(err)
	}

	byStep, err := store.RecentHookRunsByStep(ctx, "001_INGEST")
	if err != nil {
		t.Fatalf("RecentHookRunsByStep: %v", err)
	}

	quiet := byStep[2]
	if len(quiet) != 1 || quiet[0].HookName != "quiet-only-run" {
		t.Fatalf("quiet step runs = %+v, want its single run to survive the busy step", quiet)
	}

	runs := byStep[1]
	if len(runs) != MaxRecentHookRuns {
		t.Fatalf("busy step runs = %d, want the per-step cap of %d", len(runs), MaxRecentHookRuns)
	}
	oldestKept := "busy-" + strconv.Itoa(busy-MaxRecentHookRuns)
	newestKept := "busy-" + strconv.Itoa(busy-1)
	if runs[0].HookName != oldestKept || runs[len(runs)-1].HookName != newestKept {
		t.Fatalf("busy step runs = %+v, want %s..%s oldest first", runs, oldestKept, newestKept)
	}
}

func TestRecentHookRunsByStep_NoLedgerIsEmptyNotAnError(t *testing.T) {
	byStep, err := NewStore(t.TempDir()).RecentHookRunsByStep(context.Background(), "001_INGEST")
	if err != nil {
		t.Fatalf("RecentHookRunsByStep: %v", err)
	}
	if len(byStep) != 0 {
		t.Fatalf("steps = %d, want 0", len(byStep))
	}
}
