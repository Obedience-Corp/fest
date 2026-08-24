// Package status provides atomic status change operations for festivals.
package status

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/activity"
	"github.com/Obedience-Corp/fest/internal/campledger"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/registry"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// AtomicStatusChange performs an atomic status change for a festival.
// For dungeon statuses (completed, archived, someday), it uses date-based directories.
// Returns the new path of the festival.
func AtomicStatusChange(ctx context.Context, festivalPath, fromStatus, toStatus string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	festivalName := filepath.Base(festivalPath)
	festivalsRoot := festivalsRootFromPath(festivalPath, fromStatus)

	// Record status history before move
	if err := RecordStatusChange(ctx, festivalPath, fromStatus, toStatus, ""); err != nil {
		// Log warning but don't fail - history is optional
		_ = err
	}

	// Resolve aliases like "completed" -> "dungeon/completed"
	resolvedStatus := id.ResolveStatusPath(toStatus)

	var newPath string
	if strings.HasPrefix(resolvedStatus, "dungeon/") {
		if err := workspace.CheckDungeonConflict(festivalsRoot); err != nil {
			return "", err
		}

		// All dungeon statuses use date-based directories
		dateDir := CalculateDateDir(time.Now())
		statusDir := workspace.JoinStatus(festivalsRoot, resolvedStatus)
		var err error
		newPath, err = MoveToDateDirectory(festivalPath, statusDir, dateDir)
		if err != nil {
			return "", err
		}
	} else {
		newPath = filepath.Join(festivalsRoot, resolvedStatus, festivalName)

		// Check if destination exists
		if _, err := os.Stat(newPath); err == nil {
			return "", os.ErrExist
		}

		// Create parent and move
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			return "", err
		}
		if err := os.Rename(festivalPath, newPath); err != nil {
			return "", err
		}
	}

	// Update registry with new path/status
	updateRegistry(ctx, festivalsRoot, festivalName, toStatus, newPath)

	// Campaign ledger: festival lifecycle transition at the atomic core
	// success boundary (D003/D006). status_history + dir move already done.
	// Payload: target = artifact kind ("festival"), from/to = status.
	emit := campledger.NewFromFestival(ctx, newPath, campledger.WarnToStderr())
	emit.Emit(ctx, ledgerkit.KindTransitioned, campledger.FestivalScope(newPath, ""),
		campledger.WithPayload(map[string]any{
			"from":   fromStatus,
			"to":     toStatus,
			"target": "festival",
		}),
	)

	// Activity log: festival.promoted at both campaign and festival level.
	actEmitter := activity.NewFromFestival(ctx, newPath, func(error) {})
	actEmitter.Emit(ctx, "festival.promoted", activity.Scope{},
		"fest promote --to "+toStatus,
		activity.WithData(map[string]any{
			"from": fromStatus,
			"to":   toStatus,
		}),
	)

	return newPath, nil
}

// updateRegistry updates the ID registry with the new festival status and path.
// This is best-effort - registry updates are non-blocking.
// Writes a move event to the JSONL events log.
func updateRegistry(ctx context.Context, festivalsRoot, festivalName, newStatus, newPath string) {
	if ctx == nil {
		ctx = context.Background()
	}
	regPath := registry.GetEventsPath(festivalsRoot)
	reg, err := registry.Load(ctx, regPath)
	if err != nil {
		return
	}

	// Try to extract ID from directory name
	festivalID, err := id.ExtractIDFromDirName(festivalName)
	if err != nil {
		return
	}

	// Get existing entry to determine old status
	entry, err := reg.Get(ctx, festivalID)
	if err != nil {
		// Entry doesn't exist - this shouldn't happen for proper festivals
		return
	}

	oldStatus := entry.Status

	// Update with event logging
	_ = reg.UpdateStatusWithEvent(ctx, festivalID, oldStatus, newStatus)
}
