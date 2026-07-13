package workflow

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
	"golang.org/x/term"
)

const (
	decisionActorUserOverride = "user_override"
	operatorApproveToken      = "APPROVE"
	overrideSummaryMinLen     = 20
)

// Injected for tests.
var (
	stdinIsInteractiveFn = defaultStdinIsInteractive
	readOperatorConfirm  = defaultReadOperatorConfirm
)

func defaultStdinIsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

func defaultReadOperatorConfirm() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// stepHasOpenJudgeReject reports whether the current step is blocked after a
// judge rejection (or readiness block that left agent feedback).
func stepHasOpenJudgeReject(stepState *wf.StepState) bool {
	if stepState == nil || stepState.Status != wf.StepStatusBlocked {
		return false
	}
	if stepState.Judge != nil && stepState.Judge.Status == wf.JudgeRejected {
		return true
	}
	// Readiness blocks also set agent decision + blocked without a judge run.
	if stepState.DecisionActor == decisionActorAgent {
		return true
	}
	return strings.Contains(strings.ToLower(stepState.Feedback), "approval auto mode") ||
		strings.Contains(strings.ToLower(stepState.Feedback), "approval readiness")
}

// resolveManualApprovalDecision applies authority rules when auto-judge is configured.
//
// When the workspace has an approval judge hook:
//   - Non-interactive (no TTY) manual approve is refused unless --override-judge
//     with a sufficient summary (true operator override from a scripted path).
//   - Interactive approve requires typing APPROVE.
//   - After a judge/readiness reject, the actor is user_override.
func resolveManualApprovalDecision(
	decision wf.DecisionMetadata,
	judgeConfigured bool,
	overrideJudge bool,
	stepState *wf.StepState,
	stepNum int,
	stepName string,
) (wf.DecisionMetadata, error) {
	if decision.Actor == "" {
		decision.Actor = decisionActorUser
	}

	priorReject := stepHasOpenJudgeReject(stepState)

	if !judgeConfigured {
		// Legacy path: no auto-judge hook → keep open manual approve for CI/scripts.
		if priorReject {
			decision.Actor = decisionActorUserOverride
		}
		return decision, nil
	}

	if overrideJudge {
		summary := strings.TrimSpace(decision.Summary)
		if len(summary) < overrideSummaryMinLen {
			return wf.DecisionMetadata{}, festerrors.Validation("--override-judge requires --summary with a clear operator rationale").
				WithField("min_summary_len", overrideSummaryMinLen).
				WithHint("example: fest workflow approve --override-judge --summary \"I reviewed output_specs and accept them for plan\"")
		}
		decision.Actor = decisionActorUserOverride
		return decision, nil
	}

	if !stdinIsInteractiveFn() {
		hint := "run from a terminal and type APPROVE when prompted"
		if priorReject {
			hint = "fix evidence and re-submit with 'fest workflow approve --auto', or override interactively / with --override-judge --summary \"...\""
		}
		return wf.DecisionMetadata{}, festerrors.Validation("manual approve requires an interactive operator TTY when auto-judge is configured").
			WithField("step", stepNum).
			WithField("step_name", stepName).
			WithHint(hint + "\nAgents must not clear checkpoints as the user.")
	}

	fmt.Fprintf(os.Stderr, "\n%s Step %d: %s — operator approval required\n", ui.Warning("⚠"), stepNum, stepName)
	if priorReject {
		fmt.Fprintf(os.Stderr, "%s This step was previously rejected by the approval judge or readiness check.\n", ui.Label("Note"))
		if stepState != nil && stepState.Feedback != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", ui.Label("Last feedback"), truncateForPrompt(stepState.Feedback, 240))
		}
		fmt.Fprintf(os.Stderr, "Confirming will record decision_actor=%s (operator override).\n", decisionActorUserOverride)
	}
	fmt.Fprintf(os.Stderr, "Type %s to confirm operator approval: ", operatorApproveToken)

	line, err := readOperatorConfirm()
	if err != nil {
		return wf.DecisionMetadata{}, festerrors.Wrap(err, "reading operator confirmation")
	}
	if line != operatorApproveToken {
		return wf.DecisionMetadata{}, festerrors.Validation("operator approval not confirmed").
			WithField("got", line).
			WithHint(fmt.Sprintf("type exactly %s to approve, or Ctrl-C to cancel", operatorApproveToken))
	}

	if priorReject {
		decision.Actor = decisionActorUserOverride
	} else {
		decision.Actor = decisionActorUser
	}
	return decision, nil
}

func truncateForPrompt(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
