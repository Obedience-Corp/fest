package progress

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/taskfilter"
)

// AggregateProgress holds aggregated progress stats
type AggregateProgress struct {
	Total        int             `json:"total"`
	Completed    int             `json:"completed"`
	InProgress   int             `json:"in_progress"`
	Blocked      int             `json:"blocked"`
	Pending      int             `json:"pending"`
	Percentage   int             `json:"percentage"`
	Blockers     []*TaskProgress `json:"blockers,omitempty"`
	TimeSpentMin int             `json:"time_spent_minutes"`
}

// PhaseProgress holds progress for a phase
type PhaseProgress struct {
	PhaseID   string             `json:"phase_id"`
	PhaseName string             `json:"phase_name"`
	Progress  *AggregateProgress `json:"progress"`
}

// SequenceProgress holds progress for a sequence
type SequenceProgress struct {
	SequenceID   string             `json:"sequence_id"`
	SequenceName string             `json:"sequence_name"`
	Progress     *AggregateProgress `json:"progress"`
}

// PhaseTypeTime holds time aggregated by phase type.
type PhaseTypeTime struct {
	PhaseType string `json:"phase_type"`
	Minutes   int    `json:"minutes"`
}

// FestivalProgress holds complete festival progress
type FestivalProgress struct {
	FestivalName      string               `json:"festival_name"`
	Overall           *AggregateProgress   `json:"overall"`
	Phases            []*PhaseProgress     `json:"phases,omitempty"`
	PhaseTypeTimes    []PhaseTypeTime      `json:"phase_type_times,omitempty"`
	TimeMetrics       *FestivalTimeMetrics `json:"time_metrics,omitempty"`
	LifecycleDuration int                  `json:"lifecycle_duration_days,omitempty"`
}

// isTask checks if a filename looks like a task file
// Uses the shared taskfilter package for unified classification
func isTask(name string) bool {
	return taskfilter.ShouldTrack(name)
}

// GetFestivalProgress calculates overall festival progress
func (m *Manager) GetFestivalProgress(ctx context.Context, festivalPath string) (*FestivalProgress, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	festivalName := filepath.Base(festivalPath)
	overall := &AggregateProgress{}
	var phases []*PhaseProgress

	// Walk the festival directory
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return nil, err
	}

	// Find and process phases
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a phase directory using shared taskfilter
		if !taskfilter.IsPhaseDir(entry.Name()) {
			continue
		}

		phasePath := filepath.Join(festivalPath, entry.Name())
		phaseProgress, err := m.GetPhaseProgress(ctx, phasePath)
		if err != nil {
			continue
		}

		phases = append(phases, phaseProgress)

		// Aggregate to overall
		overall.Total += phaseProgress.Progress.Total
		overall.Completed += phaseProgress.Progress.Completed
		overall.InProgress += phaseProgress.Progress.InProgress
		overall.Blocked += phaseProgress.Progress.Blocked
		overall.Pending += phaseProgress.Progress.Pending
		overall.TimeSpentMin += phaseProgress.Progress.TimeSpentMin
		overall.Blockers = append(overall.Blockers, phaseProgress.Progress.Blockers...)
	}

	// Calculate percentage
	if overall.Total > 0 {
		overall.Percentage = (overall.Completed * 100) / overall.Total
	}

	// Aggregate time by phase type
	phaseTypeTotals := make(map[string]int)
	for _, phase := range phases {
		pt := readPhaseType(filepath.Join(festivalPath, phase.PhaseID))
		if pt == "" {
			pt = "implementation"
		}
		phaseTypeTotals[pt] += phase.Progress.TimeSpentMin
	}
	var phaseTypeTimes []PhaseTypeTime
	for _, pt := range []string{"planning", "ingest", "research", "implementation", "review"} {
		if mins, ok := phaseTypeTotals[pt]; ok && mins > 0 {
			phaseTypeTimes = append(phaseTypeTimes, PhaseTypeTime{PhaseType: pt, Minutes: mins})
		}
	}

	// Lazy populate time data for legacy festivals
	if m.store.LazyPopulateTimeData(festivalPath) {
		// Save the inferred data for future access
		_ = m.store.Save(ctx)
	}

	// Get time metrics from store
	timeMetrics := m.store.GetTimeMetrics()
	lifecycleDays := -1
	if timeMetrics != nil {
		lifecycleDays = timeMetrics.GetLifecycleDuration()
	}

	return &FestivalProgress{
		FestivalName:      festivalName,
		Overall:           overall,
		Phases:            phases,
		PhaseTypeTimes:    phaseTypeTimes,
		TimeMetrics:       timeMetrics,
		LifecycleDuration: lifecycleDays,
	}, nil
}

// GetPhaseProgress calculates progress for a phase
func (m *Manager) GetPhaseProgress(ctx context.Context, phasePath string) (*PhaseProgress, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	// Check for workflow phase first
	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		return m.getWorkflowPhaseProgress(ctx, phasePath)
	}

	phaseName := filepath.Base(phasePath)
	aggregate := &AggregateProgress{}

	// Walk the phase directory
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return nil, err
	}

	// Find and process sequences
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a sequence directory using shared taskfilter
		if !taskfilter.IsSequenceDir(entry.Name()) {
			continue
		}

		seqPath := filepath.Join(phasePath, entry.Name())
		seqProgress, err := m.GetSequenceProgress(ctx, seqPath)
		if err != nil {
			continue
		}

		// Aggregate
		aggregate.Total += seqProgress.Progress.Total
		aggregate.Completed += seqProgress.Progress.Completed
		aggregate.InProgress += seqProgress.Progress.InProgress
		aggregate.Blocked += seqProgress.Progress.Blocked
		aggregate.Pending += seqProgress.Progress.Pending
		aggregate.TimeSpentMin += seqProgress.Progress.TimeSpentMin
		aggregate.Blockers = append(aggregate.Blockers, seqProgress.Progress.Blockers...)
	}

	// Calculate percentage
	if aggregate.Total > 0 {
		aggregate.Percentage = (aggregate.Completed * 100) / aggregate.Total
	}

	return &PhaseProgress{
		PhaseID:   phaseName,
		PhaseName: phaseName,
		Progress:  aggregate,
	}, nil
}

// readPhaseType reads the phase type from PHASE_GOAL.md frontmatter.
// Returns empty string if the file doesn't exist or has no phase type.
func readPhaseType(phasePath string) string {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return ""
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return ""
	}
	return string(fm.PhaseType)
}

// GetSequenceProgress calculates progress for a sequence
func (m *Manager) GetSequenceProgress(ctx context.Context, seqPath string) (*SequenceProgress, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	seqName := filepath.Base(seqPath)
	aggregate := &AggregateProgress{}

	// Walk the sequence directory
	entries, err := os.ReadDir(seqPath)
	if err != nil {
		return nil, err
	}

	// Find and count tasks
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !isTask(entry.Name()) {
			continue
		}

		aggregate.Total++
		taskPath := filepath.Join(seqPath, entry.Name())

		// Use ResolveTaskStatus which prioritizes markdown checkboxes as source of truth
		status := ResolveTaskStatus(m.store, m.store.festivalPath, taskPath)

		switch status {
		case StatusCompleted:
			aggregate.Completed++
		case StatusInProgress:
			aggregate.InProgress++
		case StatusBlocked:
			aggregate.Blocked++
			// Get task from YAML for blocker details if available
			if task, exists := ResolveTaskProgress(m.store, m.store.festivalPath, taskPath); exists && task.Status == StatusBlocked {
				aggregate.Blockers = append(aggregate.Blockers, task)
			}
		default:
			aggregate.Pending++
		}

		// Get time tracking from YAML if available (markdown doesn't track time)
		if task, exists := ResolveTaskProgress(m.store, m.store.festivalPath, taskPath); exists {
			aggregate.TimeSpentMin += task.TimeSpentMinutes
		}
	}

	// Calculate percentage
	if aggregate.Total > 0 {
		aggregate.Percentage = (aggregate.Completed * 100) / aggregate.Total
	}

	return &SequenceProgress{
		SequenceID:   seqName,
		SequenceName: seqName,
		Progress:     aggregate,
	}, nil
}

// getWorkflowPhaseProgress calculates progress for a workflow-based phase
// by parsing WORKFLOW.md and loading workflow state from the Store.
func (m *Manager) getWorkflowPhaseProgress(ctx context.Context, phasePath string) (*PhaseProgress, error) {
	phaseName := filepath.Base(phasePath)
	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")

	// Parse WORKFLOW.md to get steps
	parser := wf.NewParser()
	steps, err := parser.Parse(ctx, workflowPath)
	if err != nil {
		return nil, errors.Wrap(err, "parsing workflow")
	}

	// Load workflow state from Store (JSONL-backed)
	state, ok := m.store.WorkflowPhaseState(phaseName)
	if !ok {
		state = wf.NewWorkflowState(0)
	}

	aggregate := &AggregateProgress{
		Total: len(steps),
	}

	for _, step := range steps {
		stepState := state.GetStepState(step.Number)
		if stepState == nil {
			aggregate.Pending++
			continue
		}
		switch stepState.Status {
		case wf.StepStatusCompleted:
			aggregate.Completed++
		case wf.StepStatusSkipped:
			aggregate.Completed++
		case wf.StepStatusInProgress:
			aggregate.InProgress++
		case wf.StepStatusBlocked:
			aggregate.Blocked++
		default:
			aggregate.Pending++
		}
	}

	if aggregate.Total > 0 {
		aggregate.Percentage = (aggregate.Completed * 100) / aggregate.Total
	}

	return &PhaseProgress{
		PhaseID:   phaseName,
		PhaseName: phaseName,
		Progress:  aggregate,
	}, nil
}
