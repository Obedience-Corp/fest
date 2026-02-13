// Package status provides atomic status change operations for festivals.
package status

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/registry"
)

// AtomicStatusChange performs an atomic status change for a festival.
// For "completed" status (stored under dungeon/completed), it uses date-based directories.
// Returns the new path of the festival.
func AtomicStatusChange(ctx context.Context, festivalPath, fromStatus, toStatus string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	festivalName := filepath.Base(festivalPath)
	festivalsRoot := filepath.Dir(filepath.Dir(festivalPath))

	// Record status history before move
	if err := RecordStatusChange(ctx, festivalPath, fromStatus, toStatus, ""); err != nil {
		// Log warning but don't fail - history is optional
		_ = err
	}

	var newPath string
	if toStatus == "completed" {
		// Use date-based directory for completed festivals
		dateDir := CalculateCompletionDateDir(time.Now())
		completedDir := filepath.Join(festivalsRoot, "dungeon", "completed")
		var err error
		newPath, err = MoveToDateDirectory(festivalPath, completedDir, dateDir)
		if err != nil {
			return "", err
		}
	} else {
		newPath = filepath.Join(festivalsRoot, toStatus, festivalName)

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
