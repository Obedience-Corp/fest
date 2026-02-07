package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateStore_AppendEvent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	event := StateEvent{
		Type:          "loop_check",
		Step:          1,
		LoopIndex:     0,
		MaxIterations: 3,
		Passed:        true,
	}

	if err := store.AppendEvent(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	statePath := filepath.Join(tmpDir, ".fest", "loop_state.jsonl")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var parsed StateEvent
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if parsed.Type != "loop_check" {
		t.Errorf("expected type=loop_check, got %s", parsed.Type)
	}
	if parsed.Step != 1 {
		t.Errorf("expected step=1, got %d", parsed.Step)
	}
	if !parsed.Passed {
		t.Error("expected passed=true")
	}
	if parsed.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestStateStore_AppendMultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	for i := 0; i < 3; i++ {
		event := StateEvent{
			Type:      "loop_retry",
			Step:      1,
			LoopIndex: 0,
			Iteration: i + 1,
		}
		if err := store.AppendEvent(event); err != nil {
			t.Fatalf("event %d: unexpected error: %v", i, err)
		}
	}

	events, err := store.Events()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestStateStore_CurrentIterations(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	// No events yet
	count, err := store.CurrentIterations(0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 iterations, got %d", count)
	}

	// Add some retry events
	for i := 0; i < 3; i++ {
		event := StateEvent{
			Type:      "loop_retry",
			Step:      1,
			LoopIndex: 0,
			Iteration: i + 1,
		}
		if err := store.AppendEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	// Add a check event (should not count)
	if err := store.AppendEvent(StateEvent{
		Type:      "loop_check",
		Step:      1,
		LoopIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	// Add retry for different loop (should not count)
	if err := store.AppendEvent(StateEvent{
		Type:      "loop_retry",
		Step:      1,
		LoopIndex: 1,
		Iteration: 1,
	}); err != nil {
		t.Fatal(err)
	}

	count, err = store.CurrentIterations(0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 iterations for loop 0 step 1, got %d", count)
	}

	// Different loop
	count, err = store.CurrentIterations(1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 iteration for loop 1 step 1, got %d", count)
	}
}

func TestStateStore_Events(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	// No events yet
	events, err := store.Events()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}

	// Add events
	now := time.Now()
	for i := 0; i < 2; i++ {
		event := StateEvent{
			Type:      "loop_check",
			Timestamp: now,
			Step:      i + 1,
		}
		if err := store.AppendEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	events, err = store.Events()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestStateStore_PreservesTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	fixedTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	event := StateEvent{
		Type:      "loop_check",
		Timestamp: fixedTime,
		Step:      1,
	}

	if err := store.AppendEvent(event); err != nil {
		t.Fatal(err)
	}

	events, err := store.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if !events[0].Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, events[0].Timestamp)
	}
}
