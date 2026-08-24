package activity

// Destination controls which file(s) an event is written to.
type Destination int

const (
	// DestFestivalOnly writes to <festival>/.fest/activity.jsonl only.
	DestFestivalOnly Destination = iota
	// DestBoth writes to both the festival-level and campaign-level files.
	// Used for lifecycle events (festival.*, phase.started/completed,
	// sequence.started/completed) so the campaign log is a single scrollable
	// timeline of what happened in the festival layer.
	DestBoth
)

// catalog maps event names to their destination. New events declare their
// destination here when added, per the issue spec.
var catalog = map[string]Destination{
	// Festival lifecycle — emit at both campaign and festival level.
	"festival.created":   DestBoth,
	"festival.deleted":   DestBoth,
	"festival.promoted":  DestBoth,
	"festival.linked":    DestBoth,
	"festival.unlinked":  DestBoth,
	"festival.renamed":   DestBoth,

	// Phase lifecycle — both levels.
	"phase.created":   DestBoth,
	"phase.deleted":   DestBoth,
	"phase.started":   DestBoth,
	"phase.completed": DestBoth,

	// Sequence lifecycle — both levels.
	"sequence.created":   DestBoth,
	"sequence.deleted":   DestBoth,
	"sequence.started":   DestBoth,
	"sequence.completed": DestBoth,

	// Granular scaffolding — festival only.
	"task.created":  DestFestivalOnly,
	"task.deleted":  DestFestivalOnly,
	"task.renamed":  DestFestivalOnly,
	"task.started":  DestFestivalOnly,
	"task.completed": DestFestivalOnly,
	"task.blocked":  DestFestivalOnly,
	"task.reset":    DestFestivalOnly,

	// Gates — festival only.
	"gate.applied": DestFestivalOnly,
	"gate.skipped": DestFestivalOnly,

	// Operations — festival only.
	"validate.ran":     DestFestivalOnly,
	"init.ran":         DestFestivalOnly,
	"next.resolved":    DestFestivalOnly,
	"go.navigated":     DestFestivalOnly, // festival-only per issue spec
	"workflow.skipped": DestFestivalOnly,
	"commit.made":      DestFestivalOnly,
	"tui.action":       DestFestivalOnly,
}

// destination returns the write destination for an event name. Unknown events
// default to DestFestivalOnly, the conservative choice.
func destination(eventName string) Destination {
	if dest, ok := catalog[eventName]; ok {
		return dest
	}
	return DestFestivalOnly
}
