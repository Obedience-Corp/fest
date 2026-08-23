package campledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"
)

// TestPayloadTargetIsArtifactKind is a regression test for fest#276.
// The "target" payload field must always carry the artifact kind
// ("festival" or "task"), never an action name ("blocked", "reset") or
// status value. This prevents consumers like camp-graph from misreading
// the field as a status.
func TestPayloadTargetIsArtifactKind(t *testing.T) {
	camp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(camp, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-id\nname: test\n"
	if err := os.WriteFile(filepath.Join(camp, ".campaign", "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	fest := filepath.Join(camp, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(fest, 0o755); err != nil {
		t.Fatal(err)
	}

	e := NewFromFestival(context.Background(), fest, func(error) {})
	if e.disabled {
		t.Skip("writer id resolution disabled emitter in this environment")
	}

	// Mirror the payloads emitted by the real call sites:
	//   - internal/commands/status/atomic.go: festival transition
	//   - internal/progress/manager.go:      task completed
	//   - internal/progress/manager.go:      task blocked
	//   - internal/progress/manager.go:      task reset
	payloads := []map[string]any{
		// festival transition (atomic.go)
		{"from": "active", "to": "completed", "target": "festival"},
		// task completed (manager.go)
		{"target": "task", "to": "completed"},
		// task blocked (manager.go)
		{"from": "pending", "to": "blocked", "target": "task", "action": "block"},
		// task reset (manager.go)
		{"to": "pending", "target": "task", "action": "reset"},
	}

	for i, p := range payloads {
		e.Emit(context.Background(), ledgerkit.KindTransitioned,
			FestivalScope(fest, "001_PHASE/01_seq/01_task.md"),
			WithPayload(p))
		_ = i
	}

	// Collect all events from the shard(s).
	eventsRoot := filepath.Join(camp, ".campaign", "events")
	var events []ledgerkit.Event
	err := filepath.Walk(eventsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var ev ledgerkit.Event
			if json.Unmarshal([]byte(line), &ev) == nil {
				events = append(events, ev)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected ledger events to be written")
	}

	validTargets := map[string]bool{"festival": true, "task": true}
	for _, ev := range events {
		if ev.Payload == nil {
			continue
		}
		target, ok := ev.Payload["target"]
		if !ok {
			t.Errorf("event %s missing target field", ev.ID)
			continue
		}
		s, ok := target.(string)
		if !ok {
			t.Errorf("event %s target is not a string: %T", ev.ID, target)
			continue
		}
		if !validTargets[s] {
			t.Errorf("event %s target=%q must be an artifact kind (festival|task), "+
				"not an action or status", ev.ID, s)
		}
	}
}

// TestPayloadToCarriesDestinationStatus verifies the "to" field always
// carries the real destination status, so consumers never need to read
// "target" for status information.
func TestPayloadToCarriesDestinationStatus(t *testing.T) {
	camp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(camp, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-id\nname: test\n"
	if err := os.WriteFile(filepath.Join(camp, ".campaign", "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	fest := filepath.Join(camp, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(fest, 0o755); err != nil {
		t.Fatal(err)
	}

	e := NewFromFestival(context.Background(), fest, func(error) {})
	if e.disabled {
		t.Skip("writer id resolution disabled emitter in this environment")
	}

	// Blocked-task payload: "to" must be "blocked", not "task".
	e.Emit(context.Background(), ledgerkit.KindTransitioned,
		FestivalScope(fest, "001_PHASE/01_seq/01_task.md"),
		WithPayload(map[string]any{
			"from":   "pending",
			"to":     "blocked",
			"target": "task",
			"action": "block",
		}))

	eventsRoot := filepath.Join(camp, ".campaign", "events")
	var found bool
	_ = filepath.Walk(eventsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(data), `"to":"blocked"`) {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal(`expected payload "to":"blocked" in ledger, not found`)
	}
}
