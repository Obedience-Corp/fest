package events

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCreateEvent(t *testing.T) {
	event := NewCreateEvent("CC0001", "test-festival", "planning")

	if event.Event != EventCreate {
		t.Errorf("Event = %q, want %q", event.Event, EventCreate)
	}
	if event.ID != "CC0001" {
		t.Errorf("ID = %q, want %q", event.ID, "CC0001")
	}
	if event.Name != "test-festival" {
		t.Errorf("Name = %q, want %q", event.Name, "test-festival")
	}
	if event.Status != "planning" {
		t.Errorf("Status = %q, want %q", event.Status, "planning")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewMoveEvent(t *testing.T) {
	event := NewMoveEvent("MV0001", "planning", "active")

	if event.Event != EventMove {
		t.Errorf("Event = %q, want %q", event.Event, EventMove)
	}
	if event.ID != "MV0001" {
		t.Errorf("ID = %q, want %q", event.ID, "MV0001")
	}
	if event.From != "planning" {
		t.Errorf("From = %q, want %q", event.From, "planning")
	}
	if event.To != "active" {
		t.Errorf("To = %q, want %q", event.To, "active")
	}
}

func TestNewArchiveEvent(t *testing.T) {
	event := NewArchiveEvent("AR0001")

	if event.Event != EventArchive {
		t.Errorf("Event = %q, want %q", event.Event, EventArchive)
	}
	if event.ID != "AR0001" {
		t.Errorf("ID = %q, want %q", event.ID, "AR0001")
	}
	if event.To != "dungeon" {
		t.Errorf("To = %q, want %q", event.To, "dungeon")
	}
}

func TestNewDeleteEvent(t *testing.T) {
	event := NewDeleteEvent("DEL0001")

	if event.Event != EventDelete {
		t.Errorf("Event = %q, want %q", event.Event, EventDelete)
	}
	if event.ID != "DEL0001" {
		t.Errorf("ID = %q, want %q", event.ID, "DEL0001")
	}
}

func TestAppendEvent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "test_events.jsonl")

	event := NewCreateEvent("AP0001", "append-test", "planning")
	if err := AppendEvent(ctx, eventsPath, event); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("Events file should not be empty")
	}

	// Append another event
	event2 := NewMoveEvent("AP0001", "planning", "active")
	if err := AppendEvent(ctx, eventsPath, event2); err != nil {
		t.Fatalf("AppendEvent() second call error = %v", err)
	}

	// Verify file has two lines
	data, _ = os.ReadFile(eventsPath)
	lineCount := 0
	for _, b := range data {
		if b == '\n' {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("Expected 2 lines, got %d", lineCount)
	}
}

func TestWriteEvents(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "batch_events.jsonl")

	events := []FestivalEvent{
		{Timestamp: time.Now(), Event: EventCreate, ID: "WE0001", Name: "first", Status: "planning"},
		{Timestamp: time.Now(), Event: EventCreate, ID: "WE0002", Name: "second", Status: "planning"},
		{Timestamp: time.Now(), Event: EventMove, ID: "WE0001", From: "planning", To: "active"},
	}

	if err := WriteEvents(ctx, eventsPath, events); err != nil {
		t.Fatalf("WriteEvents() error = %v", err)
	}

	// Load and verify
	loaded, err := LoadEvents(ctx, eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Errorf("Loaded %d events, want 3", len(loaded))
	}
}

func TestLoadEvents_NonExistentFile(t *testing.T) {
	ctx := context.Background()
	events, err := LoadEvents(ctx, "/nonexistent/path/events.jsonl")

	if err != nil {
		t.Errorf("LoadEvents() should return nil error for non-existent file, got %v", err)
	}
	if events != nil {
		t.Error("LoadEvents() should return nil events for non-existent file")
	}
}

func TestLoadEvents_EmptyFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "empty.jsonl")

	if err := os.WriteFile(eventsPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadEvents(ctx, eventsPath)
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestLoadEvents_SkipsMalformedLines(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "mixed.jsonl")

	// Mix of valid and invalid JSON lines
	content := `{"ts":"2026-01-01T00:00:00Z","event":"create","id":"OK0001","name":"valid","status":"planning"}
not valid json at all
{"ts":"2026-01-02T00:00:00Z","event":"create","id":"OK0002","name":"also-valid","status":"active"}
`
	if err := os.WriteFile(eventsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadEvents(ctx, eventsPath)
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	// Should have 2 valid events, skipping the malformed line
	if len(events) != 2 {
		t.Errorf("Expected 2 valid events, got %d", len(events))
	}
}

func TestMaterializeState_Create(t *testing.T) {
	events := []FestivalEvent{
		{Timestamp: time.Now(), Event: EventCreate, ID: "MS0001", Name: "test", Status: "planning"},
	}

	state := MaterializeState(events)

	if len(state) != 1 {
		t.Errorf("State has %d entries, want 1", len(state))
	}
	if state["MS0001"] == nil {
		t.Fatal("Expected MS0001 in state")
	}
	if state["MS0001"].Status != "planning" {
		t.Errorf("Status = %q, want %q", state["MS0001"].Status, "planning")
	}
}

func TestMaterializeState_Move(t *testing.T) {
	events := []FestivalEvent{
		{Timestamp: time.Now(), Event: EventCreate, ID: "MS0001", Name: "test", Status: "planning"},
		{Timestamp: time.Now().Add(time.Hour), Event: EventMove, ID: "MS0001", From: "planning", To: "active"},
	}

	state := MaterializeState(events)

	if state["MS0001"].Status != "active" {
		t.Errorf("Status = %q, want %q", state["MS0001"].Status, "active")
	}
}

func TestMaterializeState_Archive(t *testing.T) {
	events := []FestivalEvent{
		{Timestamp: time.Now(), Event: EventCreate, ID: "MS0001", Name: "test", Status: "active"},
		{Timestamp: time.Now().Add(time.Hour), Event: EventArchive, ID: "MS0001", To: "dungeon"},
	}

	state := MaterializeState(events)

	if state["MS0001"].Status != "dungeon" {
		t.Errorf("Status = %q, want %q", state["MS0001"].Status, "dungeon")
	}
}

func TestMaterializeState_Delete(t *testing.T) {
	events := []FestivalEvent{
		{Timestamp: time.Now(), Event: EventCreate, ID: "MS0001", Name: "test", Status: "planning"},
		{Timestamp: time.Now().Add(time.Hour), Event: EventDelete, ID: "MS0001"},
	}

	state := MaterializeState(events)

	if _, exists := state["MS0001"]; exists {
		t.Error("Deleted festival should not exist in state")
	}
}

func TestMaterializeState_ComplexLifecycle(t *testing.T) {
	now := time.Now()
	events := []FestivalEvent{
		{Timestamp: now, Event: EventCreate, ID: "F001", Name: "first", Status: "planning"},
		{Timestamp: now.Add(1 * time.Hour), Event: EventCreate, ID: "F002", Name: "second", Status: "planning"},
		{Timestamp: now.Add(2 * time.Hour), Event: EventMove, ID: "F001", From: "planning", To: "active"},
		{Timestamp: now.Add(3 * time.Hour), Event: EventMove, ID: "F002", From: "planning", To: "active"},
		{Timestamp: now.Add(4 * time.Hour), Event: EventMove, ID: "F001", From: "active", To: "completed"},
		{Timestamp: now.Add(5 * time.Hour), Event: EventArchive, ID: "F002", To: "dungeon"},
		{Timestamp: now.Add(6 * time.Hour), Event: EventCreate, ID: "F003", Name: "third", Status: "planning"},
	}

	state := MaterializeState(events)

	// F001 should be completed
	if state["F001"].Status != "completed" {
		t.Errorf("F001 Status = %q, want %q", state["F001"].Status, "completed")
	}
	// F002 should be in dungeon
	if state["F002"].Status != "dungeon" {
		t.Errorf("F002 Status = %q, want %q", state["F002"].Status, "dungeon")
	}
	// F003 should be planning
	if state["F003"].Status != "planning" {
		t.Errorf("F003 Status = %q, want %q", state["F003"].Status, "planning")
	}
}

func TestFilterByStatus(t *testing.T) {
	state := map[string]*Festival{
		"F001": {ID: "F001", Status: "active"},
		"F002": {ID: "F002", Status: "planning"},
		"F003": {ID: "F003", Status: "active"},
		"F004": {ID: "F004", Status: "completed"},
	}

	active := FilterByStatus(state, "active")
	if len(active) != 2 {
		t.Errorf("Expected 2 active, got %d", len(active))
	}

	planning := FilterByStatus(state, "planning")
	if len(planning) != 1 {
		t.Errorf("Expected 1 planning, got %d", len(planning))
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with existing file
	existingPath := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !FileExists(existingPath) {
		t.Error("FileExists should return true for existing file")
	}

	// Test with non-existent file
	if FileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("FileExists should return false for non-existent file")
	}

	// Test with directory
	if FileExists(tmpDir) {
		t.Error("FileExists should return false for directory")
	}
}

func TestAppendEvent_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := NewCreateEvent("CC0001", "test", "planning")
	err := AppendEvent(ctx, "/any/path", event)
	if err == nil {
		t.Error("AppendEvent should return error with cancelled context")
	}
}

func TestLoadEvents_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := LoadEvents(ctx, "/any/path")
	if err == nil {
		t.Error("LoadEvents should return error with cancelled context")
	}
}
