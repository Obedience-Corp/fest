package workflow

import (
	"encoding/json"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// workflowStatusSchema identifies the structured workflow status contract.
// Consumers should treat any object carrying this schema_version as stable.
const workflowStatusSchema = "fest.workflow.status/v1"

// workflowStatusJSON is the machine-readable snapshot emitted by
// `fest workflow status --json`. It is derived from structured navigator state,
// never from the human-readable rendering.
type workflowStatusJSON struct {
	SchemaVersion string                   `json:"schema_version"`
	FestivalID    string                   `json:"festival_id"`
	FestivalName  string                   `json:"festival_name"`
	FestivalPath  string                   `json:"festival_path"`
	Phase         string                   `json:"phase"`
	PhasePath     string                   `json:"phase_path"`
	Mode          string                   `json:"mode"`
	Workflow      string                   `json:"workflow"`
	WorkflowStep  *string                  `json:"workflow_step"`
	CurrentStep   *int                     `json:"current_step"`
	TotalSteps    int                      `json:"total_steps"`
	Complete      bool                     `json:"complete"`
	Steps         []workflowStatusStepJSON `json:"steps"`
}

// workflowStatusStepJSON is one step entry in the structured snapshot.
type workflowStatusStepJSON struct {
	Number           int    `json:"number"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	IsCurrent        bool   `json:"is_current"`
	HasCheckpoint    bool   `json:"has_checkpoint"`
	Goal             string `json:"goal"`
	Feedback         string `json:"feedback,omitempty"`
	RemediationPhase string `json:"remediation_phase,omitempty"`
	// WaitingOnJudge is true when a detached approval judge is still running.
	WaitingOnJudge bool   `json:"waiting_on_judge,omitempty"`
	JudgeStatus    string `json:"judge_status,omitempty"`
}

// collectWorkflowStatus builds the structured snapshot from navigator state.
// An empty workflow (no steps) is reported as complete with a null current step
// and an empty steps slice, matching the documented no-active-workflow contract.
func collectWorkflowStatus(nav *wf.Navigator) workflowStatusJSON {
	steps := nav.GetSteps()
	state := nav.GetWorkflowState()

	out := workflowStatusJSON{
		SchemaVersion: workflowStatusSchema,
		FestivalPath:  nav.Ctx.FestivalPath,
		Phase:         nav.Ctx.PhaseName,
		PhasePath:     nav.Ctx.PhasePath,
		Mode:          "workflow",
		Workflow:      nav.Ctx.PhaseName,
		Steps:         make([]workflowStatusStepJSON, 0, len(steps)),
	}
	if nav.IsGate() {
		out.Mode = "gate"
	}
	if cfg, err := config.LoadFestivalConfig(nav.Ctx.FestivalPath, ""); err == nil {
		out.FestivalID = cfg.Metadata.ID
		out.FestivalName = cfg.Metadata.Name
	}

	if len(steps) == 0 {
		out.Complete = true
		return out
	}

	out.TotalSteps = state.TotalSteps
	out.Complete = state.IsComplete()
	if !out.Complete {
		current := state.CurrentStep
		out.CurrentStep = &current
	}

	for _, step := range steps {
		status := wf.StepStatusPending
		feedback := ""
		remediation := ""
		entry := workflowStatusStepJSON{
			Number:        step.Number,
			Name:          step.Name,
			HasCheckpoint: step.HasCheckpoint(),
			Goal:          step.Goal,
		}
		if stepState := state.GetStepState(step.Number); stepState != nil {
			status = stepState.Status
			feedback = wf.DisplayFeedback(stepState.Feedback)
			remediation = stepState.RemediationPhase
			if stepState.Judge != nil {
				entry.JudgeStatus = stepState.Judge.Status
				entry.WaitingOnJudge = stepState.Judge.Status == wf.JudgeRunning
			}
		}
		isCurrent := step.Number == state.CurrentStep && !out.Complete
		entry.Status = string(status)
		entry.IsCurrent = isCurrent
		entry.Feedback = feedback
		entry.RemediationPhase = remediation

		out.Steps = append(out.Steps, entry)

		if isCurrent {
			name := step.Name
			out.WorkflowStep = &name
		}
	}

	return out
}

// renderWorkflowStatusJSON encodes the structured snapshot as indented JSON.
func renderWorkflowStatusJSON(nav *wf.Navigator) (string, error) {
	data, err := json.MarshalIndent(collectWorkflowStatus(nav), "", "  ")
	if err != nil {
		return "", errors.Parse("formatting workflow status JSON", err)
	}
	return string(data), nil
}
