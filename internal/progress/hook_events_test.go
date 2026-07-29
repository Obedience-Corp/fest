package progress

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
