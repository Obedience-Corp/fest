// Package execution provides the ExecutionNavigator and plan types
// for navigating implementation phases in festivals.
package execution

// ExecutionPlan represents the complete plan for executing a festival.
// It contains all phases, sequences, and tasks organized in execution order.
type ExecutionPlan struct {
	FestivalPath string            `json:"festival_path"`
	Phases       []*PhaseExecution `json:"phases"`
	Summary      *ExecutionSummary `json:"summary"`
}

// PhaseExecution represents execution plan for a phase.
// It contains all sequences within the phase and optional quality gate.
type PhaseExecution struct {
	Name        string               `json:"name"`
	Path        string               `json:"path"`
	Number      int                  `json:"number"`
	Sequences   []*SequenceExecution `json:"sequences"`
	QualityGate *QualityGateInfo     `json:"quality_gate,omitempty"`
	TotalTasks  int                  `json:"total_tasks"`
	Status      string               `json:"status"`
}

// SequenceExecution represents execution plan for a sequence.
// It contains step groups which may be parallel or sequential.
type SequenceExecution struct {
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Number     int          `json:"number"`
	Steps      []*StepGroup `json:"steps"`
	TotalTasks int          `json:"total_tasks"`
	Status     string       `json:"status"`
}

// StepGroup represents a group of tasks to execute together.
// Tasks within a parallel group can be executed concurrently.
type StepGroup struct {
	Number   int         `json:"number"`
	Type     string      `json:"type"` // "parallel" or "sequential"
	Tasks    []*TaskInfo `json:"tasks"`
	Parallel bool        `json:"parallel"`
}

// TaskInfo represents a task in the execution plan.
// Renamed from PlanTask for consistency with navigator design.
type TaskInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Number        int      `json:"number"`
	AutonomyLevel string   `json:"autonomy_level"`
	Dependencies  []string `json:"dependencies,omitempty"`
	Status        string   `json:"status"`
	IsGate        bool     `json:"is_gate"`
}

// QualityGateInfo describes a quality gate.
// Quality gates are checkpoints that require verification before proceeding.
type QualityGateInfo struct {
	PhaseName   string   `json:"phase_name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Criteria    []string `json:"criteria,omitempty"`
	Passed      bool     `json:"passed"`
}

// ExecutionSummary provides summary statistics for the plan.
type ExecutionSummary struct {
	TotalPhases    int    `json:"total_phases"`
	TotalSequences int    `json:"total_sequences"`
	TotalTasks     int    `json:"total_tasks"`
	TotalSteps     int    `json:"total_steps"`
	ParallelGroups int    `json:"parallel_groups"`
	QualityGates   int    `json:"quality_gates"`
	EstimatedTime  string `json:"estimated_time,omitempty"`
}

// Status constants for task execution.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusSkipped    = "skipped"
	StatusFailed     = "failed"
	StatusBlocked    = "blocked"
)
