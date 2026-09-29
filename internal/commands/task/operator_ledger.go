package task

import (
	"context"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"
	"github.com/Obedience-Corp/fest/internal/campledger"
	"github.com/Obedience-Corp/fest/internal/progress"
)

var deferralLedgerEmit = emitDeferralLedger

// deferrer is the manager surface applyDeferral needs. Keeping it narrow lets
// the success-only ordering below be tested without a festival on disk.
type deferrer interface {
	DeferBlocker(ctx context.Context, taskID, reason string, audit progress.DeferralAudit) error
}

// applyDeferral writes the deferral and, only once the write succeeded, emits
// the camp ledger entry. The ledger entry is the cross-check for design doc 04
// guard 7: a deferral event in the JSONL with no matching ledger entry was
// hand-written, so a failed deferral must leave no entry to contradict.
func applyDeferral(ctx context.Context, mgr deferrer, festivalPath, taskID, reason string, audit *OperatorAudit) error {
	if err := mgr.DeferBlocker(ctx, taskID, reason, audit.Progress()); err != nil {
		return err
	}
	deferralLedgerEmit(ctx, festivalPath, taskID, reason, audit.DeferredBy)
	return nil
}

func emitDeferralLedger(ctx context.Context, festivalPath, taskID, reason, deferredBy string) {
	if festivalPath == "" {
		return
	}

	payload := map[string]any{
		"title":  "blocker deferred",
		"target": "task",
		"from":   progress.StatusBlocked,
		"to":     progress.StatusBlocked,
		"action": "defer",
	}
	if deferredBy != "" {
		payload["deferred_by"] = deferredBy
	}

	emit := campledger.NewFromFestival(ctx, festivalPath, campledger.WarnToStderr())
	emit.Emit(ctx, ledgerkit.KindDecided, campledger.FestivalScope(festivalPath, taskID),
		campledger.WithWhy(reason),
		campledger.WithPayload(payload),
	)
}
