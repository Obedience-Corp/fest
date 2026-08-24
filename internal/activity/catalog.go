package activity

// Destination controls which file(s) an event is written to.
type Destination int

const (
	// DestFestivalOnly writes to <festival>/.fest/activity.jsonl only.
	DestFestivalOnly Destination = iota
	// DestBoth writes to both the festival-level and campaign-level files.
	// Used for the live lifecycle events (festival.created, festival.promoted,
	// phase.created, sequence.created) so the campaign log is a single
	// scrollable timeline of what happened in the festival layer.
	DestBoth
)

// catalog maps live event names to their destination. Only events that a
// command or progress.Manager actually Emit()s belong here. Reserved names
// for later wiring (festival.linked/unlinked/deleted/renamed, gate.*,
// phase.started/completed/deleted, sequence.started/completed/deleted,
// task.started/deleted/renamed, init.ran, tui.action) are documented in
// docs/activity_log.md as future and default to DestFestivalOnly until added.
var catalog = map[string]Destination{
	"festival.created":  DestBoth,
	"festival.promoted": DestBoth,
	"phase.created":     DestBoth,
	"sequence.created":  DestBoth,

	"task.created":   DestFestivalOnly,
	"task.completed": DestFestivalOnly,
	"task.blocked":   DestFestivalOnly,
	"task.reset":     DestFestivalOnly,

	"validate.ran":     DestFestivalOnly,
	"next.resolved":    DestFestivalOnly,
	"go.navigated":     DestFestivalOnly,
	"workflow.skipped": DestFestivalOnly,
}

// destination returns the write destination for an event name. Unknown events
// default to DestFestivalOnly, the conservative choice.
func destination(eventName string) Destination {
	if dest, ok := catalog[eventName]; ok {
		return dest
	}
	return DestFestivalOnly
}
