package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
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
	// These judge fields are retained for fest.workflow.status/v1 consumers.
	// Human renderers intentionally keep this metadata out of normal output.
	JudgeCommand string `json:"judge_command,omitempty"`
	JudgePid     int    `json:"judge_pid,omitempty"`
	JudgeRunID   string `json:"judge_run_id,omitempty"`
	JudgeDetail  string `json:"judge_detail,omitempty"`
	// HumanApprovalRequired is true when approval: human-required is set on the step.
	HumanApprovalRequired bool `json:"human_approval_required,omitempty"`
	// Followups are the judge's itemized fixes for a rejected step. Additive to
	// fest.workflow.status/v1.
	Followups []string `json:"followups,omitempty"`
	// The remaining judge fields complete the recorded verdict. Additive to
	// fest.workflow.status/v1.
	JudgeFinishedAt     string   `json:"judge_finished_at,omitempty"`
	JudgeConfidence     *float64 `json:"judge_confidence,omitempty"`
	JudgeEvidenceStatus string   `json:"judge_evidence_status,omitempty"`
	// RecentHookRuns are the hook executions the ledger recorded for this step,
	// oldest first. The cap of progress.MaxRecentHookRuns applies to the phase
	// read as a whole, so a step busier than its neighbours can crowd them out
	// of one snapshot. Additive to fest.workflow.status/v1.
	RecentHookRuns []workflowStatusHookRunJSON `json:"recent_hook_runs,omitempty"`
}

// workflowStatusHookRunJSON is one recorded hook execution for a step.
type workflowStatusHookRunJSON struct {
	Name       string `json:"name"`
	Layer      string `json:"layer"`
	Timing     string `json:"timing"`
	Verb       string `json:"verb"`
	Outcome    string `json:"outcome"`
	Skip       string `json:"skip,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Fail       string `json:"fail,omitempty"`
	Blocked    bool   `json:"blocked,omitempty"`
	At         string `json:"at"`
}

func hookRunJSON(event progress.ProgressEvent) workflowStatusHookRunJSON {
	return workflowStatusHookRunJSON{
		Name:       event.HookName,
		Layer:      event.HookLayer,
		Timing:     event.HookTiming,
		Verb:       event.HookVerb,
		Outcome:    event.HookOutcome,
		Skip:       event.HookSkip,
		ExitCode:   event.HookExitCode,
		DurationMS: event.HookMillis,
		Fail:       event.HookFail,
		Blocked:    event.HookBlocked,
		At:         event.Timestamp.Format(time.RFC3339),
	}
}

// recentHookRunsByStep reads the festival ledger once and groups this
// navigator's hook runs by step. A ledger that cannot be read degrades to no
// hook runs: a status snapshot missing hook history is still useful, and one
// that fails because the ledger is mid-write is not.
func recentHookRunsByStep(ctx context.Context, nav *wf.Navigator) map[int][]workflowStatusHookRunJSON {
	byStep := map[int][]workflowStatusHookRunJSON{}
	store := progress.NewStore(nav.Ctx.FestivalPath)
	runs, err := store.RecentHookRuns(ctx, nav.StateKey())
	if err != nil {
		return byStep
	}
	for _, run := range runs {
		byStep[run.Step] = append(byStep[run.Step], hookRunJSON(run))
	}
	return byStep
}

// collectWorkflowStatus builds the structured snapshot from navigator state.
// An empty workflow (no steps) is reported as complete with a null current step
// and an empty steps slice, matching the documented no-active-workflow contract.
func collectWorkflowStatus(ctx context.Context, nav *wf.Navigator) workflowStatusJSON {
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

	hookRuns := recentHookRunsByStep(ctx, nav)

	for _, step := range steps {
		status := wf.StepStatusPending
		feedback := ""
		remediation := ""
		entry := workflowStatusStepJSON{
			Number:                step.Number,
			Name:                  step.Name,
			HasCheckpoint:         step.HasCheckpoint(),
			Goal:                  step.Goal,
			HumanApprovalRequired: isHumanRequired(step),
		}
		if stepState := state.GetStepState(step.Number); stepState != nil {
			status = stepState.Status
			feedback = wf.DisplayFeedback(stepState.Feedback)
			remediation = stepState.RemediationPhase
			entry.Followups = stepState.Followups
			if stepState.Judge != nil {
				entry.JudgeStatus = stepState.Judge.Status
				entry.WaitingOnJudge = stepState.Judge.Status == wf.JudgeRunning
				entry.JudgeCommand = stepState.Judge.Command
				entry.JudgePid = stepState.Judge.Pid
				entry.JudgeRunID = stepState.Judge.RunID
				entry.JudgeDetail = wf.DisplayFeedback(stepState.Judge.Detail)
				entry.JudgeConfidence = stepState.Judge.Confidence
				entry.JudgeEvidenceStatus = stepState.Judge.EvidenceStatus
				if stepState.Judge.FinishedAt != nil {
					entry.JudgeFinishedAt = stepState.Judge.FinishedAt.Format(time.RFC3339)
				}
			}
		}
		entry.RecentHookRuns = hookRuns[step.Number]
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
func renderWorkflowStatusJSON(ctx context.Context, nav *wf.Navigator) (string, error) {
	data, err := json.MarshalIndent(collectWorkflowStatus(ctx, nav), "", "  ")
	if err != nil {
		return "", errors.Parse("formatting workflow status JSON", err)
	}
	return string(data), nil
}
