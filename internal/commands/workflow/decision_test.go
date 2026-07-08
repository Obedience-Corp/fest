package workflow

import "testing"

func TestNormalizeDecisionRequiresAgentApprovalSummary(t *testing.T) {
	_, err := normalizeDecision("approval", "agent", "", "")
	if err == nil {
		t.Fatal("normalizeDecision() error = nil, want summary validation error")
	}
}

func TestNormalizeDecisionUsesFallbackForAgentRejection(t *testing.T) {
	decision, err := normalizeDecision("rejection", "agent", "", "needs revision")
	if err != nil {
		t.Fatalf("normalizeDecision() error: %v", err)
	}
	if decision.Actor != decisionActorAgent {
		t.Fatalf("Actor = %q, want %q", decision.Actor, decisionActorAgent)
	}
	if decision.Summary != "needs revision" {
		t.Fatalf("Summary = %q, want reason fallback", decision.Summary)
	}
}

func TestNormalizeDecisionRejectsInvalidActor(t *testing.T) {
	_, err := normalizeDecision("approval", "operator", "approved", "")
	if err == nil {
		t.Fatal("normalizeDecision() error = nil, want invalid actor error")
	}
}
