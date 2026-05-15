package localstore

import (
	"time"
)

// RunManifest is .workflow/runs/<run-id>/run.yaml.
type RunManifest struct {
	Version     int        `yaml:"version"`
	Kind        string     `yaml:"kind"`
	RunID       string     `yaml:"run_id"`
	WorkflowID  string     `yaml:"workflow_id"`
	Status      string     `yaml:"status"`
	StartedAt   time.Time  `yaml:"started_at"`
	CompletedAt time.Time  `yaml:"completed_at,omitempty"`
	StartedBy   string     `yaml:"started_by,omitempty"`
	Executor    string     `yaml:"executor,omitempty"`
	Source      RunSource  `yaml:"source"`
	Summary     RunSummary `yaml:"summary"`
}

// RunSource captures pointers to the authored workflow document.
type RunSource struct {
	WorkflowPath string `yaml:"workflow_path"`
	WorkflowHash string `yaml:"workflow_hash"`
	WorkitemPath string `yaml:"workitem_path"`
}

// RunSummary is the cached current state. Event replay is authoritative;
// summary is updated from replay on read.
type RunSummary struct {
	CurrentStep    int  `yaml:"current_step"`
	TotalSteps     int  `yaml:"total_steps"`
	CompletedSteps int  `yaml:"completed_steps"`
	Blocked        bool `yaml:"blocked"`
}

// RunState is the replayed view of an active or completed run.
type RunState struct {
	RunID          string
	Status         string // "active" | "completed" | "abandoned" | "blocked"
	StartedAt      time.Time
	CompletedAt    time.Time
	CurrentStep    int
	TotalSteps     int
	CompletedSteps int
	Blocked        bool
	DocHashChanged bool
}

const (
	RunVersion = 1
	RunKind    = "workflow-run"
)
