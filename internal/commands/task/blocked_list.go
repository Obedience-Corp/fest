package task

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// blockedEntry is the list surface's own view of one blocked task.
// progress.TaskProgress carries YAML tags only, so encoding it directly would
// publish Go field names as the JSON contract.
type blockedEntry struct {
	Task           string     `json:"task"`
	BlockerMessage string     `json:"blocker_message"`
	BlockedAt      *time.Time `json:"blocked_at,omitempty"`
	Attempts       []string   `json:"attempts,omitempty"`
	Deferred       bool       `json:"deferred"`
	DeferralReason string     `json:"deferral_reason,omitempty"`
	DeferredAt     *time.Time `json:"deferred_at,omitempty"`
	DeferredBy     string     `json:"deferred_by,omitempty"`
}

// blockedListResult is the --json shape. OpenCount and DeferredCount describe
// the entries in Blocked, so they stay consistent under --open and --deferred
// and always sum to len(Blocked).
type blockedListResult struct {
	Blocked       []blockedEntry `json:"blocked"`
	OpenCount     int            `json:"open_count"`
	DeferredCount int            `json:"deferred_count"`
}

func loadBlockedTasks(ctx context.Context, festivalPath string) (open, deferred []*progress.TaskProgress, err error) {
	mgr, err := progress.NewManagerReadOnly(ctx, festivalPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "loading progress")
	}

	festival, err := mgr.GetFestivalProgress(ctx, festivalPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reading festival progress")
	}
	if festival == nil || festival.Overall == nil {
		return nil, nil, nil
	}

	open, deferred = progress.SplitBlockers(festival.Overall.Blockers)
	return open, deferred, nil
}

func blockedEntryFor(task *progress.TaskProgress) blockedEntry {
	entry := blockedEntry{
		Task:           task.TaskID,
		BlockerMessage: task.BlockerMessage,
		BlockedAt:      task.BlockedAt,
		Attempts:       task.BlockerAttempts,
		Deferred:       task.BlockerDeferred,
	}
	if task.BlockerDeferred {
		entry.DeferralReason = task.DeferralReason
		entry.DeferredAt = task.BlockerDeferredAt
		entry.DeferredBy = task.BlockerDeferredBy
	}
	return entry
}

func blockedListJSON(open, deferred []*progress.TaskProgress) blockedListResult {
	result := blockedListResult{
		Blocked:       make([]blockedEntry, 0, len(open)+len(deferred)),
		OpenCount:     len(open),
		DeferredCount: len(deferred),
	}
	for _, task := range open {
		result.Blocked = append(result.Blocked, blockedEntryFor(task))
	}
	for _, task := range deferred {
		result.Blocked = append(result.Blocked, blockedEntryFor(task))
	}
	return result
}

// blockedAgeLine is the second line of an entry: when it was blocked and, for a
// deferred blocker, when and by whom it was deferred.
func blockedAgeLine(now time.Time, task *progress.TaskProgress) string {
	var parts []string
	if task.BlockedAt != nil {
		parts = append(parts, "blocked "+humanDurationAt(now, task.BlockedAt)+" ago")
	}
	if task.BlockerDeferred && task.BlockerDeferredAt != nil {
		deferred := "deferred " + humanDurationAt(now, task.BlockerDeferredAt) + " ago"
		if task.BlockerDeferredBy != "" {
			deferred += " by " + task.BlockerDeferredBy
		}
		parts = append(parts, deferred)
	}
	return strings.Join(parts, ", ")
}

func writeBlockedEntry(out *strings.Builder, now time.Time, task *progress.TaskProgress) {
	out.WriteString("  " + ui.StateIcon("blocked") + " " +
		ui.Value(task.TaskID, ui.TaskColor) + " " + ui.Dim(task.BlockerMessage) + "\n")

	if age := blockedAgeLine(now, task); age != "" {
		out.WriteString("      " + ui.Dim(age) + "\n")
	}
	for _, attempt := range task.BlockerAttempts {
		out.WriteString("      " + ui.Dim("tried: "+attempt) + "\n")
	}
	if task.BlockerDeferred && task.DeferralReason != "" {
		out.WriteString("      " + ui.Dim("reason: "+task.DeferralReason) + "\n")
	}
}

func renderBlockedList(now time.Time, open, deferred []*progress.TaskProgress) string {
	if len(open) == 0 && len(deferred) == 0 {
		return "No blocked tasks.\n"
	}

	var out strings.Builder
	out.WriteString(ui.H2("Blockers") + "\n")

	for _, section := range []struct {
		name  string
		tasks []*progress.TaskProgress
	}{
		{"Open", open},
		{"Deferred", deferred},
	} {
		if len(section.tasks) == 0 {
			continue
		}
		out.WriteString("\n" + ui.Label(section.name) + "\n")
		for _, task := range section.tasks {
			writeBlockedEntry(&out, now, task)
		}
	}

	return out.String()
}

// runBlockedList is the read-only reporting path of fest task blocked. It takes
// no task argument, writes nothing, and never prompts.
func runBlockedList(ctx context.Context, out io.Writer, festivalPath string, now time.Time) error {
	open, deferred, err := loadBlockedTasks(ctx, festivalPath)
	if err != nil {
		return err
	}

	switch {
	case blockedListDeferred:
		open = nil
	case blockedListOpen:
		deferred = nil
	}

	if blockedJSON {
		return shared.EncodeJSON(out, blockedListJSON(open, deferred))
	}

	if _, err := io.WriteString(out, renderBlockedList(now, open, deferred)); err != nil {
		return errors.Wrap(err, "writing the blocker list")
	}
	return nil
}
