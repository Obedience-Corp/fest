package events

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// LoadEvents reads all events from a JSONL file.
// Returns nil, nil if the file does not exist (no events yet).
func LoadEvents(ctx context.Context, eventsPath string) ([]FestivalEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	f, err := os.Open(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No events yet
		}
		return nil, errors.IO("opening events file", err).
			WithField("path", eventsPath)
	}
	defer func() { _ = f.Close() }()

	var eventsList []FestivalEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var event FestivalEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Log warning but continue - don't fail on single bad line
			// This makes the format resilient to partial writes
			continue
		}
		eventsList = append(eventsList, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.IO("reading events file", err).
			WithField("path", eventsPath)
	}

	return eventsList, nil
}

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}
