package shared

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// DroppedBlockerRecordFile is the human readable record a forced completion
// leaves in the festival, so the festival carries the answer to "what did we
// ship without?" into the dungeon with it.
const DroppedBlockerRecordFile = "DROPPED_BLOCKERS.md"

// DeferredBlockerState is what a completion path needs to know about the
// festival's open deferrals.
type DeferredBlockerState struct {
	Tasks []*progress.TaskProgress
	Sweep int
}

// Open reports whether anything is deferred.
func (s DeferredBlockerState) Open() bool { return len(s.Tasks) > 0 }

// IDs lists the deferred task IDs in festival order.
func (s DeferredBlockerState) IDs() []string {
	ids := make([]string, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
}

// LoadDeferredBlockers reads the festival's deferred blockers and the sweep it
// reached. Both completion paths call this rather than looking the deferrals up
// themselves, so they cannot disagree about what is deferred.
func LoadDeferredBlockers(ctx context.Context, festivalPath string) (DeferredBlockerState, error) {
	mgr, err := progress.NewManagerReadOnly(ctx, festivalPath)
	if err != nil {
		return DeferredBlockerState{}, errors.Wrap(err, "loading progress")
	}
	return DeferredBlockerState{Tasks: mgr.DeferredTasks(), Sweep: mgr.SweepState().Current}, nil
}

// DeferredBlockerRefusal is the message both completion paths print when a
// festival still has deferred blockers and no --force. The wording is design
// doc 02's, and the last line names the limit design doc 05 H11 records.
func DeferredBlockerRefusal(state DeferredBlockerState) string {
	var out strings.Builder
	out.WriteString(strconv.Itoa(len(state.Tasks)) +
		" deferred blockers are still open. fest next will revisit them.\n")
	out.WriteString("Promote anyway with --force to record them as dropped.\n\n")

	for _, task := range state.Tasks {
		out.WriteString(ui.StateIcon("blocked") + " " + ui.Value(task.TaskID, ui.TaskColor) + " " +
			ui.Dim(task.BlockerMessage) + "\n")
		if task.DeferralReason != "" {
			out.WriteString("    " + ui.Dim("deferred: "+task.DeferralReason) + "\n")
		}
	}

	out.WriteString("\n" + ui.Dim("--force drops every deferred blocker. There is no per-task drop yet."))
	return out.String()
}

// WriteDroppedBlockerRecord writes the record of what a forced completion
// dropped into the festival, so it travels with the festival into the dungeon.
func WriteDroppedBlockerRecord(festivalPath string, state DeferredBlockerState) (string, error) {
	if !state.Open() {
		return "", nil
	}

	var out strings.Builder
	out.WriteString("# Dropped blockers\n\n")
	out.WriteString("This festival was completed with `--force` while " +
		strconv.Itoa(len(state.Tasks)) + " blocker(s) were still deferred.\n")
	out.WriteString("Forced at " + time.Now().UTC().Format(time.RFC3339) + " after sweep " +
		strconv.Itoa(state.Sweep) + ".\n\n")

	for _, task := range state.Tasks {
		out.WriteString("## " + task.TaskID + "\n\n")
		out.WriteString("- **Blocker:** " + fallback(task.BlockerMessage, "not recorded") + "\n")
		out.WriteString("- **Deferral reason:** " + fallback(task.DeferralReason, "not recorded") + "\n")
		out.WriteString("- **Deferred by:** " + fallback(task.BlockerDeferredBy, "not recorded") + "\n")
		if task.BlockerDeferredAt != nil {
			out.WriteString("- **Deferred at:** " + task.BlockerDeferredAt.Format(time.RFC3339) + "\n")
		}
		out.WriteString("- **Sweeps run:** " + strconv.Itoa(state.Sweep) + "\n")

		if len(task.BlockerAttempts) == 0 {
			out.WriteString("- **Attempts:** none recorded\n")
		} else {
			out.WriteString("- **Attempts:**\n")
			for _, attempt := range task.BlockerAttempts {
				out.WriteString("  - " + attempt + "\n")
			}
		}
		out.WriteString("\n")
	}

	recordPath := filepath.Join(festivalPath, DroppedBlockerRecordFile)
	if err := os.WriteFile(recordPath, []byte(out.String()), 0o644); err != nil {
		return "", errors.IO("writing the dropped blocker record", err).WithField("path", recordPath)
	}
	return recordPath, nil
}

func fallback(value, absent string) string {
	if strings.TrimSpace(value) == "" {
		return absent
	}
	return value
}
