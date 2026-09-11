package progress

import "context"

// ReadEvents returns the events LoadReadOnly materializes, in the order it
// applies them: the JSONL log as written, then any workflow_state.yaml layer.
// It returns nil when the festival has no JSONL log. It never writes to disk.
func (s *Store) ReadEvents(ctx context.Context) ([]ProgressEvent, error) {
	if !fileExists(s.eventsFilePath()) {
		return nil, nil
	}
	events, err := s.parseEventsFile(ctx)
	if err != nil {
		return nil, err
	}
	if fileExists(s.workflowYAMLPath()) {
		workflowEvents, err := s.readonlyWorkflowEvents(ctx)
		if err != nil {
			return nil, err
		}
		events = append(events, workflowEvents...)
	}
	return events, nil
}

// Replay returns a store materialized from events alone: the state fest shows
// after exactly those events. It never reads or writes disk.
func Replay(festivalPath string, events []ProgressEvent) *Store {
	s := NewStore(festivalPath)
	s.materializeFrom(events)
	return s
}
