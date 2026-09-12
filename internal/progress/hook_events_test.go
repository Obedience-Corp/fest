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

func TestRecentHookRuns_FiltersByEventTypeAndPhaseKey(t *testing.T) {
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

	runs, err := store.RecentHookRuns(ctx, "gate:001_IMPLEMENT")
	if err != nil {
		t.Fatalf("RecentHookRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1 (other phases and other event types excluded)", len(runs))
	}
	if runs[0].HookName != "approval_judge" {
		t.Fatalf("hook name = %q, want approval_judge", runs[0].HookName)
	}
}

func TestRecentHookRuns_CapsAtMaxKeepingNewest(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store := NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}
	total := MaxRecentHookRuns + 5
	for i := range total {
		QueueHookRuns(store, "001_INGEST", 1, []hooks.HookRun{{
			Name:    "hook-" + strconv.Itoa(i),
			Outcome: hooks.OutcomePass,
		}})
	}
	if err := store.Save(ctx); err != nil {
		t.Fatal(err)
	}

	runs, err := store.RecentHookRuns(ctx, "001_INGEST")
	if err != nil {
		t.Fatalf("RecentHookRuns: %v", err)
	}
	if len(runs) != MaxRecentHookRuns {
		t.Fatalf("runs = %d, want %d", len(runs), MaxRecentHookRuns)
	}
	if runs[0].HookName != "hook-5" {
		t.Fatalf("oldest retained run = %q, want hook-5", runs[0].HookName)
	}
	if runs[len(runs)-1].HookName != "hook-"+strconv.Itoa(total-1) {
		t.Fatalf("newest run = %q, want the last one written", runs[len(runs)-1].HookName)
	}
}

func TestRecentHookRuns_NoLedgerIsEmptyNotAnError(t *testing.T) {
	runs, err := NewStore(t.TempDir()).RecentHookRuns(context.Background(), "001_INGEST")
	if err != nil {
		t.Fatalf("RecentHookRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs = %d, want 0", len(runs))
	}
}
