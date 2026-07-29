package workflow

import (
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

const (
	decisionActorUser  = "user"
	decisionActorAgent = "agent"
)

// normalizeDecision validates the actor for a manual checkpoint decision.
// Only the user actor is accepted here: the agent actor is reserved for
// judge-mediated decisions recorded by 'fest workflow approve --auto', so an
// agent-actor entry in the audit trail always corresponds to an explicit
// operator delegation rather than an agent clearing its own gate.
func normalizeDecision(action, actor, summary string) (wf.DecisionMetadata, error) {
	actor = strings.TrimSpace(strings.ToLower(actor))
	if actor == "" {
		actor = decisionActorUser
	}
	summary = strings.TrimSpace(summary)

	switch actor {
	case decisionActorUser:
	case decisionActorAgent:
		return wf.DecisionMetadata{}, festerrors.Validation("--as agent is not allowed for manual "+action).
			WithField("actor", actor).
			WithHint(agentActorHint(action))
	default:
		return wf.DecisionMetadata{}, festerrors.Validation("invalid decision actor").
			WithField("actor", actor).
			WithHint("use --as user; agent-actor decisions are recorded only by 'fest workflow judge' with a configured approval judge")
	}

	return wf.DecisionMetadata{
		Actor:   actor,
		Summary: summary,
	}, nil
}

func agentActorHint(action string) string {
	const delegation = "agent-actor decisions enter the audit trail only through 'fest workflow judge' with a configured approval judge (hooks.definitions.approval_judge)"
	switch action {
	case "approval":
		return "blocking checkpoints are operator decisions: stop, present the work, and ask the operator to run 'fest workflow approve'; " + delegation
	case "rejection":
		return "blocking checkpoints are operator decisions: share your findings and ask the operator to run 'fest workflow reject'; " + delegation
	}
	return "blocking checkpoints are operator decisions; " + delegation
}
