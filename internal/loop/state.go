package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StateEvent represents a loop state event in JSONL.
type StateEvent struct {
	Type           string    `json:"type"`                      // "loop_check" or "loop_retry"
	Timestamp      time.Time `json:"timestamp"`
	Step           int       `json:"step"`
	LoopIndex      int       `json:"loop_index"`
	Iteration      int       `json:"iteration"`
	MaxIterations  int       `json:"max_iterations"`
	Passed         bool      `json:"passed"`
	FailureContext string    `json:"failure_context,omitempty"`
	Action         string    `json:"action,omitempty"` // "advance", "retry_step", "return_to_step", "escalate"
	ReturnToStep   *int      `json:"return_to_step,omitempty"`
}

// StateStore manages loop state persistence.
type StateStore struct {
	phasePath string
}

// NewStateStore creates a state store for a phase.
func NewStateStore(phasePath string) *StateStore {
	return &StateStore{phasePath: phasePath}
}

// AppendEvent appends a state event to the JSONL file.
func (s *StateStore) AppendEvent(event StateEvent) error {
	stateDir := filepath.Join(s.phasePath, ".fest")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	statePath := filepath.Join(stateDir, "loop_state.jsonl")
	f, err := os.OpenFile(statePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open state file: %w", err)
	}
	defer f.Close()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	return nil
}

// CurrentIterations returns the current iteration count for a loop at a given step.
func (s *StateStore) CurrentIterations(loopIndex int, step int) (int, error) {
	statePath := filepath.Join(s.phasePath, ".fest", "loop_state.jsonl")

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read state file: %w", err)
	}

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event StateEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.LoopIndex == loopIndex && event.Step == step && event.Type == "loop_retry" {
			count++
		}
	}

	return count, nil
}

// Events returns all loop state events.
func (s *StateStore) Events() ([]StateEvent, error) {
	statePath := filepath.Join(s.phasePath, ".fest", "loop_state.jsonl")

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var events []StateEvent
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event StateEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}
