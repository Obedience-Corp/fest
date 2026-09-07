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
// the status of the last status_history entry in its fest.yaml.
//
// It returns an empty string when fest.yaml is missing, unreadable, or carries
// no status history. Rules that relax while a festival is in planning also
// relax on an empty status; callers that must fail closed on an undetermined
// status use lifecycle.EnforcePreActive, which does exactly that.
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
