// Package events provides JSONL event logging for festival lifecycle tracking.
// Events are append-only and the current state is materialized by replaying
// events in timestamp order. This enables daemon watching and audit history.
package events

import "time"

// EventType represents the type of festival lifecycle event.
type EventType string

// Event type constants for JSONL festival events.
const (
	EventCreate  EventType = "create"
	EventMove    EventType = "move"
	EventArchive EventType = "archive"
	EventDelete  EventType = "delete"
)

// FestivalEvent represents a single festival lifecycle event in JSONL format.
// Events are append-only and the current state is materialized by
// replaying events in timestamp order.
type FestivalEvent struct {
	Timestamp time.Time `json:"ts"`
	Event     EventType `json:"event"`
	ID        string    `json:"id"`

	// Create event fields
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`

	// Move event fields
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// Festival represents the materialized state of a festival.
// This is derived from replaying events.
type Festival struct {
	ID        string
	Name      string
	Status    string // active, planning, completed, dungeon
	CreatedAt time.Time
}

// NewCreateEvent creates a new festival creation event.
func NewCreateEvent(id, name, status string) *FestivalEvent {
	return &FestivalEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventCreate,
		ID:        id,
		Name:      name,
		Status:    status,
	}
}

// NewMoveEvent creates a new festival move event.
func NewMoveEvent(id, from, to string) *FestivalEvent {
	return &FestivalEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventMove,
		ID:        id,
		From:      from,
		To:        to,
	}
}

// NewArchiveEvent creates a new festival archive event.
func NewArchiveEvent(id string) *FestivalEvent {
	return &FestivalEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventArchive,
		ID:        id,
		To:        "dungeon",
	}
}

// NewDeleteEvent creates a new festival delete event.
func NewDeleteEvent(id string) *FestivalEvent {
	return &FestivalEvent{
		Timestamp: time.Now().UTC(),
		Event:     EventDelete,
		ID:        id,
	}
}
