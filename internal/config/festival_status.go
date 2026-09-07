package config

import "context"

// Festival lifecycle statuses, as recorded in fest.yaml status_history.
const (
	// StatusPlanning marks a festival whose plan is still being written.
	StatusPlanning = "planning"
	// StatusReady marks a festival whose plan is written and awaiting approval.
	StatusReady = "ready"
	// StatusActive marks a festival that is being executed.
	StatusActive = "active"
)

// FestivalStatus returns the lifecycle status of the festival at festivalPath,
// taken from the last status_history entry in its fest.yaml.
//
// It returns an empty string when fest.yaml is missing, unreadable, or carries
// no status history. Callers treat an undetermined status as the strict case:
// rules that relax while a festival is in planning stay in force.
func FestivalStatus(ctx context.Context, festivalPath string) string {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return ""
		}
	}
	if festivalPath == "" {
		return ""
	}
	cfg, err := LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return ""
	}
	return cfg.Metadata.CurrentStatus()
}

// FestivalPromoted reports whether the festival has been promoted out of
// planning, meaning its status is ready or active.
//
// It is false for a festival still in planning and for one whose status cannot
// be read, so rules that relax during planning also relax when there is no
// status to consult. Callers that must fail closed on an unreadable status use
// EnforcePreActive, which does exactly that.
func FestivalPromoted(ctx context.Context, festivalPath string) bool {
	switch FestivalStatus(ctx, festivalPath) {
	case StatusReady, StatusActive:
		return true
	}
	return false
}
