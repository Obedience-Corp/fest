package workflow

import (
	"fmt"
	"strings"
)

// PhaseTasks summarizes the task states of the phase a checkpoint belongs to.
// Its JSON form is the phase_tasks field of the fest.approval.judge/v1
// request, so the field names are a contract with judge consumers.
//
// Blocked holds every blocked task, deferred or not. Blocked tasks are not
// counted anywhere else, so Total equals the four counts plus len(Blocked).
type PhaseTasks struct {
	Total      int           `json:"total"`
	Completed  int           `json:"completed"`
	Skipped    int           `json:"skipped"`
	InProgress int           `json:"in_progress"`
	Pending    int           `json:"pending"`
	Blocked    []BlockedTask `json:"blocked"`
}

// BlockedTask is one blocked task in a phase. Task is relative to the phase
// directory, for example 01_build/02_wire_cli.md.
type BlockedTask struct {
	Task           string `json:"task"`
	Reason         string `json:"reason"`
	Deferred       bool   `json:"deferred"`
	DeferredReason string `json:"deferred_reason"`
}

// DeferredBlockers counts the blocked tasks whose blocker was deferred.
func (p *PhaseTasks) DeferredBlockers() int {
	if p == nil {
		return 0
	}
	n := 0
	for _, b := range p.Blocked {
		if b.Deferred {
			n++
		}
	}
	return n
}

// OpenBlockers returns the blocked tasks whose blocker was not deferred. A
// phase gate cannot be advanced, approved, or judged while any remain.
func (p *PhaseTasks) OpenBlockers() []BlockedTask {
	if p == nil {
		return nil
	}
	var open []BlockedTask
	for _, b := range p.Blocked {
		if !b.Deferred {
			open = append(open, b)
		}
	}
	return open
}

// FormatPhaseBlockers renders the labeled block that gate guidance prints
// before the checkpoint question, so whoever decides the gate sees every
// blocked task in the phase. It returns "" when nothing is blocked.
func FormatPhaseBlockers(blocked []BlockedTask) string {
	if len(blocked) == 0 {
		return ""
	}
	deferred := 0
	for _, b := range blocked {
		if b.Deferred {
			deferred++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "BLOCKED TASKS IN THIS PHASE: %d (%d deferred)\n", len(blocked), deferred)
	for _, b := range blocked {
		state := "deferred"
		if !b.Deferred {
			state = "NOT deferred"
		}
		fmt.Fprintf(&sb, "  - %s [%s]\n", b.Task, state)
		if reason := oneLine(b.Reason); reason != "" {
			fmt.Fprintf(&sb, "    Blocker: %s\n", reason)
		}
		if reason := oneLine(b.DeferredReason); reason != "" {
			fmt.Fprintf(&sb, "    Deferred because: %s\n", reason)
		}
	}
	sb.WriteString("These tasks are not done. The gate decision must account for them:\n")
	sb.WriteString("approve only if the gate holds with this work still blocked, and say so.\n")
	return sb.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
