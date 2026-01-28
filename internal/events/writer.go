package events

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const (
	// EventsFileName is the JSONL events file name.
	EventsFileName = "festival_events.jsonl"

	// DirPermissions for creating directories.
	DirPermissions os.FileMode = 0755

	// FilePermissions for creating files.
	FilePermissions os.FileMode = 0644
)

// AppendEvent appends a single event to the JSONL file.
// Uses atomic append to prevent partial writes.
func AppendEvent(ctx context.Context, eventsPath string, event *FestivalEvent) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(eventsPath), DirPermissions); err != nil {
		return errors.IO("creating events directory", err).
			WithField("path", filepath.Dir(eventsPath))
	}

	// Open for append (create if not exists)
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePermissions)
	if err != nil {
		return errors.IO("opening events file for append", err).
			WithField("path", eventsPath)
	}

	// Marshal and write with newline
	data, err := json.Marshal(event)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "marshaling festival event")
	}
	data = append(data, '\n')

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return errors.IO("appending festival event", err).
			WithField("path", eventsPath)
	}

	// Close and check for errors (some filesystems report write errors on close)
	if err := f.Close(); err != nil {
		return errors.IO("closing events file", err).
			WithField("path", eventsPath)
	}

	return nil
}

// WriteEvents writes a batch of events to the JSONL file.
// Used during migration from YAML to write all synthetic events at once.
func WriteEvents(ctx context.Context, eventsPath string, eventsList []FestivalEvent) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(eventsPath), DirPermissions); err != nil {
		return errors.IO("creating events directory", err).
			WithField("path", filepath.Dir(eventsPath))
	}

	// Create/truncate file
	f, err := os.Create(eventsPath)
	if err != nil {
		return errors.IO("creating events file", err).
			WithField("path", eventsPath)
	}

	for _, event := range eventsList {
		data, err := json.Marshal(event)
		if err != nil {
			_ = f.Close()
			return errors.Wrap(err, "marshaling festival event")
		}
		data = append(data, '\n')

		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return errors.IO("writing festival event", err).
				WithField("path", eventsPath)
		}
	}

	if err := f.Close(); err != nil {
		return errors.IO("closing events file", err).
			WithField("path", eventsPath)
	}

	return nil
}
