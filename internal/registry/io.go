package registry

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/events"
	"gopkg.in/yaml.v3"
)

// File name constants
const (
	// EventsFileName is the JSONL events file for festival lifecycle.
	EventsFileName = "festival_events.jsonl"

	// LegacyRegistryFileName is the old YAML format (for migration).
	LegacyRegistryFileName = "id_registry.yaml"

	// DefaultRegistryFileName is kept for backward compatibility.
	// Deprecated: Use EventsFileName for new code.
	DefaultRegistryFileName = LegacyRegistryFileName

	// StateDir is the subdirectory for local state files (never synced)
	StateDir = ".state"
)

// GetEventsPath returns the full path to the JSONL events file.
func GetEventsPath(festivalsRoot string) string {
	return filepath.Join(festivalsRoot, ".festival", StateDir, EventsFileName)
}

// GetRegistryPath returns the full path to the registry file
// within the .festival/.state directory of the festivals root.
// Deprecated: Use GetEventsPath for new code.
func GetRegistryPath(festivalsRoot string) string {
	return filepath.Join(festivalsRoot, ".festival", StateDir, LegacyRegistryFileName)
}

// GetStatePath returns the full path to the .state directory.
func GetStatePath(festivalsRoot string) string {
	return filepath.Join(festivalsRoot, ".festival", StateDir)
}

// Load reads the registry using JSONL events as primary format.
// It uses convert-on-access for legacy YAML files.
//
// Load order:
//  1. JSONL exists → load from events
//  2. Legacy YAML exists → migrate to JSONL
//  3. Neither exists → return empty registry
func Load(ctx context.Context, path string) (*Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	// Derive paths from the given path (which may be YAML or events)
	dir := filepath.Dir(path)
	eventsPath := filepath.Join(dir, EventsFileName)
	legacyPath := filepath.Join(dir, LegacyRegistryFileName)

	// 1. Preferred: JSONL exists - load from events
	if fileExists(eventsPath) {
		return loadFromEvents(ctx, eventsPath)
	}

	// 2. Legacy: YAML exists - migrate to JSONL
	if fileExists(legacyPath) {
		return migrateFromLegacy(ctx, legacyPath, eventsPath)
	}

	// 3. Neither exists - return empty registry
	return New(eventsPath), nil
}

// LoadWithReconcile loads the registry and reconciles with filesystem.
// This detects manual directory moves and generates synthetic events.
func LoadWithReconcile(ctx context.Context, festivalsRoot string) (*Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	eventsPath := GetEventsPath(festivalsRoot)
	reg, err := Load(ctx, eventsPath)
	if err != nil {
		return nil, err
	}

	// Reconcile with filesystem to detect manual moves
	if err := reg.reconcileWithFilesystem(ctx, festivalsRoot); err != nil {
		return nil, err
	}

	return reg, nil
}

// loadFromEvents reads JSONL and materializes registry state.
func loadFromEvents(ctx context.Context, eventsPath string) (*Registry, error) {
	eventsList, err := events.LoadEvents(ctx, eventsPath)
	if err != nil {
		return nil, err
	}

	// Materialize state from events
	state := events.MaterializeState(eventsList)

	// Convert to registry
	reg := New(eventsPath)
	for id, f := range state {
		reg.Entries[id] = RegistryEntry{
			ID:        f.ID,
			Name:      f.Name,
			Status:    f.Status,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.CreatedAt, // Use creation time as initial update time
		}
	}

	return reg, nil
}

// migrateFromLegacy converts a legacy YAML registry to JSONL format.
// After successful migration, the YAML file is deleted.
func migrateFromLegacy(ctx context.Context, yamlPath, eventsPath string) (*Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	// 1. Load existing YAML registry
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, errors.IO("reading legacy registry file", err).
			WithField("path", yamlPath)
	}

	var legacyReg Registry
	if err := yaml.Unmarshal(data, &legacyReg); err != nil {
		return nil, errors.Parse("parsing legacy registry YAML", err).
			WithField("path", yamlPath)
	}

	if legacyReg.Entries == nil {
		legacyReg.Entries = make(map[string]RegistryEntry)
	}

	// 2. Generate synthetic "create" events for each entry
	var eventsList []events.FestivalEvent
	for _, entry := range legacyReg.Entries {
		ts := entry.CreatedAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		eventsList = append(eventsList, events.FestivalEvent{
			Timestamp: ts,
			Event:     events.EventCreate,
			ID:        entry.ID,
			Name:      entry.Name,
			Status:    entry.Status,
		})
	}

	// Sort by timestamp for consistent ordering
	sort.Slice(eventsList, func(i, j int) bool {
		return eventsList[i].Timestamp.Before(eventsList[j].Timestamp)
	})

	// 3. Write JSONL file
	if err := events.WriteEvents(ctx, eventsPath, eventsList); err != nil {
		return nil, err
	}

	// 4. Delete YAML file (cleanup, ignore errors)
	_ = os.Remove(yamlPath)

	// 5. Return registry with new events path
	reg := New(eventsPath)
	for _, entry := range legacyReg.Entries {
		reg.Entries[entry.ID] = entry
	}

	return reg, nil
}

// reconcileWithFilesystem compares event state against filesystem
// and generates synthetic events for any discrepancies.
func (r *Registry) reconcileWithFilesystem(ctx context.Context, festivalsRoot string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Scan filesystem for current reality
	fsEntries, err := scanFilesystemForFestivals(ctx, festivalsRoot)
	if err != nil {
		return err
	}

	eventsPath := r.path
	var newEvents []events.FestivalEvent

	// Check for moves and new festivals
	for fsID, fsPath := range fsEntries {
		fsStatus := detectStatus(fsPath)

		if entry, exists := r.Entries[fsID]; exists {
			// Festival exists in both - check if status changed (manual move)
			if entry.Status != fsStatus && fsStatus != StatusUnknown {
				newEvents = append(newEvents, events.FestivalEvent{
					Timestamp: time.Now().UTC(),
					Event:     events.EventMove,
					ID:        fsID,
					From:      entry.Status,
					To:        fsStatus,
				})
				// Update local state
				entry.Status = fsStatus
				entry.UpdatedAt = time.Now()
				r.Entries[fsID] = entry
			}
		} else {
			// Festival in filesystem but not registry - new discovery
			name := extractNameFromPath(fsPath)
			newEvents = append(newEvents, events.FestivalEvent{
				Timestamp: time.Now().UTC(),
				Event:     events.EventCreate,
				ID:        fsID,
				Name:      name,
				Status:    fsStatus,
			})
			// Add to local state
			r.Entries[fsID] = RegistryEntry{
				ID:        fsID,
				Name:      name,
				Status:    fsStatus,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}
	}

	// Check for deletions (in registry but not filesystem)
	for id := range r.Entries {
		if _, exists := fsEntries[id]; !exists {
			newEvents = append(newEvents, events.FestivalEvent{
				Timestamp: time.Now().UTC(),
				Event:     events.EventDelete,
				ID:        id,
			})
			// Remove from local state
			delete(r.Entries, id)
		}
	}

	// Append reconciliation events
	for _, event := range newEvents {
		if err := events.AppendEvent(ctx, eventsPath, &event); err != nil {
			return err
		}
	}

	return nil
}

// Save appends a create event for any new entries.
// For existing entries, it's a no-op (state is already in events).
// Note: Most operations should use AddWithEvent instead.
func (r *Registry) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Ensure the directory exists
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return errors.IO("creating registry directory", err).
			WithField("path", dir)
	}

	// For JSONL, Save is mostly a no-op since events are appended immediately.
	// We just ensure the events file exists.
	if !fileExists(r.path) {
		// Create empty events file
		if err := os.WriteFile(r.path, []byte{}, FilePermissions); err != nil {
			return errors.IO("creating events file", err).
				WithField("path", r.path)
		}
	}

	return nil
}

// AddWithEvent adds an entry and writes a create event.
func (r *Registry) AddWithEvent(ctx context.Context, entry RegistryEntry) error {
	if err := r.Add(ctx, entry); err != nil {
		return err
	}

	event := events.NewCreateEvent(entry.ID, entry.Name, entry.Status)
	return events.AppendEvent(ctx, r.path, event)
}

// UpdateStatusWithEvent updates status and writes a move event.
func (r *Registry) UpdateStatusWithEvent(ctx context.Context, id, oldStatus, newStatus string) error {
	entry, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	entry.Status = newStatus
	entry.UpdatedAt = time.Now()
	if err := r.Update(ctx, entry); err != nil {
		return err
	}

	event := events.NewMoveEvent(id, oldStatus, newStatus)
	return events.AppendEvent(ctx, r.path, event)
}

// DeleteWithEvent deletes an entry and writes a delete event.
func (r *Registry) DeleteWithEvent(ctx context.Context, id string) error {
	if err := r.Delete(ctx, id); err != nil {
		return err
	}

	event := events.NewDeleteEvent(id)
	return events.AppendEvent(ctx, r.path, event)
}

// LoadOrCreate loads an existing registry or creates a new one if it doesn't exist.
// Uses JSONL events as the primary format with convert-on-access for legacy YAML.
func LoadOrCreate(ctx context.Context, path string) (*Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	reg, err := Load(ctx, path)
	if err != nil {
		return nil, err
	}

	// Ensure the events file exists
	if err := reg.Save(ctx); err != nil {
		return nil, err
	}

	return reg, nil
}

// extractNameFromPath extracts the festival name from a directory path.
func extractNameFromPath(path string) string {
	base := filepath.Base(path)
	// Directory format: "festival-name-ID0001"
	// Remove the ID suffix to get the name
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '-' {
			return base[:i]
		}
	}
	return base
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}
