// Package status provides status history tracking for festivals.
package status

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
)

// StatusHistoryEntry represents a single status change record.
type StatusHistoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Note       string    `json:"note,omitempty"`
}

// RecordStatusChange appends a new entry to the festival's status history.
// It creates the .fest directory if it doesn't exist.
func RecordStatusChange(ctx context.Context, festivalDir, fromStatus, toStatus, note string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Don't record if status hasn't changed
	if fromStatus == toStatus {
		return nil
	}

	festDir := filepath.Join(festivalDir, ".fest")
	historyPath := filepath.Join(festDir, "status_history.json")

	// Load existing history
	history, err := LoadStatusHistory(ctx, festivalDir)
	if err != nil {
		return errors.Wrap(err, "loading status history")
	}

	// Append new entry
	entry := StatusHistoryEntry{
		Timestamp:  time.Now(),
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Note:       note,
	}
	history = append(history, entry)

	// Ensure .fest directory exists
	if err := os.MkdirAll(festDir, 0755); err != nil {
		return errors.IO("creating .fest directory", err)
	}

	// Write updated history
	var buffer bytes.Buffer
	if err := shared.EncodeJSON(&buffer, history); err != nil {
		return errors.Wrap(err, "encoding status history JSON")
	}

	data := bytes.TrimRight(buffer.Bytes(), "\n")
	if err := writeStatusHistoryAtomic(historyPath, data); err != nil {
		return err
	}

	return nil
}

// writeStatusHistoryAtomic writes the history file via a temp file and rename so
// a crash mid-write cannot truncate or corrupt the existing history.
func writeStatusHistoryAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".status-history-*.tmp")
	if err != nil {
		return errors.IO("creating temp status history", err).WithField("dir", dir)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return errors.IO("writing temp status history", err).WithField("path", tmpPath)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return errors.IO("syncing temp status history", err).WithField("path", tmpPath)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return errors.IO("closing temp status history", err).WithField("path", tmpPath)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return errors.IO("setting status history mode", err).WithField("path", tmpPath)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return errors.IO("replacing status history", err).WithField("path", path)
	}
	return nil
}

// LoadStatusHistory loads the status history from a festival's .fest directory.
// Returns an empty slice if no history file exists.
func LoadStatusHistory(ctx context.Context, festivalDir string) ([]StatusHistoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	historyPath := filepath.Join(festivalDir, ".fest", "status_history.json")

	// Check if file exists
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		return []StatusHistoryEntry{}, nil
	}

	// Read and parse history
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, errors.IO("reading status history", err)
	}

	var history []StatusHistoryEntry
	if err := json.Unmarshal(data, &history); err != nil {
		quarantinePath := historyPath + ".corrupt"
		if renameErr := os.Rename(historyPath, quarantinePath); renameErr != nil {
			return nil, errors.Wrap(err, "parsing status history")
		}
		return []StatusHistoryEntry{}, nil
	}

	return history, nil
}

// GetLastStatusChange returns the most recent status change entry, if any.
func GetLastStatusChange(ctx context.Context, festivalDir string) (*StatusHistoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	history, err := LoadStatusHistory(ctx, festivalDir)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return nil, nil
	}

	return &history[len(history)-1], nil
}
