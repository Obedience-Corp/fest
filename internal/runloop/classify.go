// Package runloop classifies the next fest slice and drives a leaveable run.
package runloop

import (
	"strings"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// Outcomes a leaveable run can stop with. waiting_human and completed are
// successful nights; failed is not.
const (
	OutcomeRunnable        = "runnable"
	OutcomeCompleted       = "completed"
	OutcomeWaitingHuman    = "waiting_human"
	OutcomeWaitingJudge    = "waiting_judge"
	OutcomeFailed          = "failed"
	OutcomeBudgetExhausted = "budget_exhausted"
	OutcomeStopped         = "stopped"
)

// Snapshot is a plan-agnostic view of whatever fest next would show.
type Snapshot struct {
	Kind              string // standalone | festival
	Label             string // step or task name
	Path              string
	Goal              string
	Complete          bool
	Blocked           bool
	WaitingOnJudge    bool
	HumanApproval     bool
	Checkpoint        string
	CheckpointClass   string
	TaskStatus        string
	AutonomyLevel     string
	Current           int
	Total             int
	WorkingDir        string
	Content           string
	Actions           []string
	StandaloneRuntime string
	StandaloneDoc     string
	FestivalPath      string
	RunID             string
}

// Verdict is the leaveable-run decision for a snapshot.
type Verdict struct {
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
	Label   string `json:"label,omitempty"`
}

// Classify decides whether a snapshot can be driven unattended.
func Classify(snap Snapshot) Verdict {
	label := snap.Label
	if snap.Complete {
		return Verdict{Outcome: OutcomeCompleted, Reason: "plan is complete", Label: label}
	}
	if snap.WaitingOnJudge {
		return Verdict{Outcome: OutcomeWaitingJudge, Reason: "a judge owns this checkpoint", Label: label}
	}
	if snap.Blocked {
		return Verdict{Outcome: OutcomeWaitingHuman, Reason: "plan is blocked", Label: label}
	}
	if snap.HumanApproval || isHumanApproval(snap.Checkpoint, snap.CheckpointClass) {
		return Verdict{Outcome: OutcomeWaitingHuman, Reason: "human gate", Label: label}
	}
	if strings.EqualFold(snap.TaskStatus, "blocked") {
		return Verdict{Outcome: OutcomeWaitingHuman, Reason: "task is blocked", Label: label}
	}
	if strings.EqualFold(snap.AutonomyLevel, "low") {
		return Verdict{Outcome: OutcomeWaitingHuman, Reason: "low autonomy requires a human", Label: label}
	}
	if label == "" && snap.Kind == "festival" {
		return Verdict{Outcome: OutcomeFailed, Reason: "fest next returned no task", Label: label}
	}
	return Verdict{Outcome: OutcomeRunnable, Reason: "next slice is driveable", Label: label}
}

func isHumanApproval(checkpoint, class string) bool {
	c := strings.ToLower(strings.TrimSpace(checkpoint))
	if c == string(wf.CheckpointUserApproval) || c == "user_approval" || c == "approval_required" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(class), string(wf.CheckpointClassOperatorAttestation)) {
		return true
	}
	return false
}
