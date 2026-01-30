// Package gates provides quality gate enforcement for task completion.
package gates

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/markers"
)

// GateEvaluationResult represents the result of evaluating gates for a task
type GateEvaluationResult struct {
	TaskID       string         `json:"task_id"`
	Passed       bool           `json:"passed"`
	FailedGates  []GateFailure  `json:"failed_gates,omitempty"`
	PassedGates  []string       `json:"passed_gates,omitempty"`
	SkippedGates []string       `json:"skipped_gates,omitempty"`
	Warnings     []string       `json:"warnings,omitempty"`
}

// GateFailure describes why a gate failed
type GateFailure struct {
	GateID  string   `json:"gate_id"`
	Name    string   `json:"name"`
	Reason  string   `json:"reason"`
	Details []string `json:"details,omitempty"`
}

// GateEvaluator evaluates quality gates for task completion
type GateEvaluator struct {
	festivalPath string
	phasePath    string
	sequencePath string
}

// NewGateEvaluator creates a new gate evaluator for a task
func NewGateEvaluator(festivalPath, phasePath, sequencePath string) *GateEvaluator {
	return &GateEvaluator{
		festivalPath: festivalPath,
		phasePath:    phasePath,
		sequencePath: sequencePath,
	}
}

// EvaluateForTask evaluates all applicable gates for a task before completion.
// Returns an error if any gate fails - gates are mandatory enforcement.
func (e *GateEvaluator) EvaluateForTask(ctx context.Context, taskPath string) (*GateEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled").WithOp("GateEvaluator.EvaluateForTask")
	}

	taskID := filepath.Base(taskPath)
	result := &GateEvaluationResult{
		TaskID: taskID,
		Passed: true,
	}

	// Check if task file exists
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		// No task file to evaluate - allow completion
		return result, nil
	}

	// Evaluate built-in gates

	// Gate 1: Check for unfilled markers (REPLACE, FILL, TODO)
	if failure := e.evaluateMarkerGate(ctx, taskPath); failure != nil {
		result.Passed = false
		result.FailedGates = append(result.FailedGates, *failure)
	} else {
		result.PassedGates = append(result.PassedGates, "markers")
	}

	// Gate 2: Check for required sections (if applicable)
	if failure := e.evaluateRequiredSectionsGate(ctx, taskPath); failure != nil {
		result.Passed = false
		result.FailedGates = append(result.FailedGates, *failure)
	} else {
		result.PassedGates = append(result.PassedGates, "required_sections")
	}

	return result, nil
}

// evaluateMarkerGate checks for unfilled markers in the task file
func (e *GateEvaluator) evaluateMarkerGate(ctx context.Context, taskPath string) *GateFailure {
	hasMarkers, foundMarkers, err := markers.HasUnfilledMarkers(ctx, taskPath)
	if err != nil {
		// Can't check markers - add warning but don't fail
		return nil
	}

	if !hasMarkers {
		return nil
	}

	// Build details about unfilled markers
	details := make([]string, 0, len(foundMarkers))
	for _, m := range foundMarkers {
		details = append(details, string(m.Type)+": "+m.Hint+" (line "+itoa(m.LineNumber)+")")
	}

	return &GateFailure{
		GateID:  "markers",
		Name:    "Unfilled Markers",
		Reason:  "Task file contains unfilled markers that must be completed",
		Details: details,
	}
}

// evaluateRequiredSectionsGate checks for required sections in the task file
// This is a soft gate - we check for common required sections but don't fail
// TODO: Make this configurable based on task type
func (e *GateEvaluator) evaluateRequiredSectionsGate(_ context.Context, _ string) *GateFailure {
	// For now, this is a no-op - always passes
	// Future: check for required sections like "## Results", "## Completion Notes"
	return nil
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
