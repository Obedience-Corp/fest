package promote

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/task"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// operatorGuardFn is a seam for tests. It is the one guard from
// internal/commands/task, never a second copy of its rules.
var operatorGuardFn = task.OperatorGuard

// isCompletionStatus reports whether a promotion target means "this festival is
// finished". Both the lifecycle target and the dungeon alias resolve here.
func isCompletionStatus(status string) bool {
	return status == "completed" || status == "dungeon/completed"
}

// enforceDeferredBlockers refuses to call a festival complete while blockers
// are deferred, and puts the --force override behind the same operator guard as
// the deferral itself. It returns halt=true when the caller must stop without
// promoting. With nothing deferred it does nothing at all, so every existing
// --force use is unchanged (D009).
func enforceDeferredBlockers(ctx context.Context, festival *show.FestivalInfo, nextStatus string, opts *promoteOptions) (halt bool, err error) {
	if !isCompletionStatus(nextStatus) {
		return false, nil
	}

	state, err := shared.LoadDeferredBlockers(ctx, festival.Path)
	if err != nil {
		return false, err
	}
	if !state.Open() {
		return false, nil
	}

	if !opts.force {
		if opts.json {
			if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
				"success":           false,
				"error":             "deferred blockers are still open",
				"deferred_blockers": state.IDs(),
				"hint":              "use --force to record them as dropped",
			}); encErr != nil {
				return true, encErr
			}
			return true, errors.ErrAlreadyPrinted
		}
		fmt.Println(shared.DeferredBlockerRefusal(state))
		return true, nil
	}

	audit, err := operatorGuardFn(ctx, "forced completion")
	if err != nil {
		return true, err
	}

	mgr, err := progress.NewManager(ctx, festival.Path)
	if err != nil {
		return true, errors.Wrap(err, "loading progress")
	}
	if err := mgr.RecordForcedCompletion(ctx, audit.Progress(), state.IDs()); err != nil {
		return true, err
	}

	recordPath, err := shared.WriteDroppedBlockerRecord(festival.Path, state)
	if err != nil {
		return true, err
	}
	if !opts.json && recordPath != "" {
		fmt.Printf("%s %s\n", ui.Warning("Dropped deferred blockers:"), ui.Dim(shared.DroppedBlockerRecordFile))
	}

	return false, nil
}
