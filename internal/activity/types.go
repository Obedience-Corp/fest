// Package activity provides the activity.jsonl event log at festival and
// campaign levels.
//
// Unlike [progress_events.jsonl] (narrow task/workflow status transitions) and
// the [campledger] package (campaign-level high-intent events only), this
// package records the wired mutating fest CLI actions listed in catalog.go
// with a versioned schema, redaction, file locking, and durable writes.
// Commands emit after a successful mutation; fail-path logging is not wired.
//
// # Two files
//
//   - Festival-level: <festival>/.fest/activity.jsonl — live events scoped
//     to that festival.
//   - Campaign-level: .campaign/fest/activity.jsonl — DestBoth lifecycle
//     events (festival.created, festival.promoted, phase.created,
//     sequence.created). Festival-only events (task.*, validate.ran,
//     next.resolved, go.navigated, workflow.skipped, commit.made) are not
//     copied here.
//
// # Semantics
//
// Append-only, O_APPEND | O_CREATE, fsync after every write, advisory file lock
// (flock) to prevent interleaved writes from concurrent fest processes.
// Rotation closes and unlocks before rename so the triggering write starts a
// fresh canonical activity.jsonl.
package activity

import "time"

// SchemaVersion is the current activity log schema version. Additive field
// additions do not bump; removed/renamed fields or changed semantics bump.
const SchemaVersion = 1

// Actor records who performed the action and with which fest version.
type Actor struct {
	User        string `json:"user"`
	Host        string `json:"host"`
	FestVersion string `json:"fest_version"`
}

// Scope records where the action happened, from campaign down to task level.
// Fields are omitempty so pure-campaign events omit festival fields and
// festival-only events omit campaign_root.
type Scope struct {
	CampaignRoot         string `json:"campaign_root,omitempty"`
	FestivalID           string `json:"festival_id,omitempty"`
	FestivalName         string `json:"festival_name,omitempty"`
	FestivalPathRelative string `json:"festival_path_relative,omitempty"`
	Phase                string `json:"phase,omitempty"`
	Sequence             string `json:"sequence,omitempty"`
	Task                 string `json:"task,omitempty"`
}

// Result records the outcome of the action.
type Result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Event is a single activity log entry. One JSON object per line, appended to
// the activity.jsonl file(s).
type Event struct {
	V         int    `json:"v"`
	Ts        string `json:"ts"`
	Event     string `json:"event"`
	Actor     Actor  `json:"actor"`
	Scope     Scope  `json:"scope"`
	SourceCmd string `json:"source_cmd"`
	Data      any    `json:"data,omitempty"`
	Result    Result `json:"result"`
}

// newEvent constructs an Event with the current timestamp, schema version,
// and actor information filled in. The caller sets Event, Scope, Data, and
// Result via the Emit method.
func newEvent(eventName string, scope Scope, sourceCmd string, data any, result Result) Event {
	actor := resolveActor()
	return Event{
		V:         SchemaVersion,
		Ts:        time.Now().UTC().Format(time.RFC3339Nano),
		Event:     eventName,
		Actor:     actor,
		Scope:     scope,
		SourceCmd: redact(sourceCmd),
		Data:      data,
		Result:    result,
	}
}
