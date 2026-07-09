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

func normalizeDecision(action, actor, summary, fallbackSummary string) (wf.DecisionMetadata, error) {
	actor = strings.TrimSpace(strings.ToLower(actor))
	if actor == "" {
		actor = decisionActorUser
	}
	summary = strings.TrimSpace(summary)

	switch actor {
	case decisionActorUser, decisionActorAgent:
	default:
		return wf.DecisionMetadata{}, festerrors.Validation("invalid decision actor").
			WithField("actor", actor).
			WithHint("use --as user or --as agent")
	}

	if actor == decisionActorAgent && summary == "" {
		if fallbackSummary != "" {
			summary = strings.TrimSpace(fallbackSummary)
		} else {
			return wf.DecisionMetadata{}, festerrors.Validation("--summary is required for agent " + action).
				WithHint("record the approval basis with --summary \"...\"")
		}
	}

	return wf.DecisionMetadata{
		Actor:   actor,
		Summary: summary,
	}, nil
}
