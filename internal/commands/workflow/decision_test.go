package workflow

import (
	"strings"
	"testing"
)

func TestNormalizeDecisionRejectsAgentApproval(t *testing.T) {
	_, err := normalizeDecision("approval", "agent", "looks good to me")
	if err == nil {
		t.Fatal("normalizeDecision() error = nil, want agent actor rejection")
	}
	if !strings.Contains(err.Error(), "--as agent is not allowed") {
		t.Fatalf("error = %v, want --as agent rejection", err)
	}
}

func TestNormalizeDecisionRejectsAgentRejection(t *testing.T) {
	_, err := normalizeDecision("rejection", "agent", "needs revision")
	if err == nil {
		t.Fatal("normalizeDecision() error = nil, want agent actor rejection")
	}
	if !strings.Contains(err.Error(), "--as agent is not allowed") {
		t.Fatalf("error = %v, want --as agent rejection", err)
	}
}

func TestNormalizeDecisionRejectsInvalidActor(t *testing.T) {
	_, err := normalizeDecision("approval", "operator", "approved")
	if err == nil {
		t.Fatal("normalizeDecision() error = nil, want invalid actor error")
	}
}

func TestNormalizeDecisionDefaultsToUser(t *testing.T) {
	decision, err := normalizeDecision("approval", "", "  ship it  ")
	if err != nil {
		t.Fatalf("normalizeDecision() error: %v", err)
	}
	if decision.Actor != decisionActorUser {
		t.Fatalf("Actor = %q, want %q", decision.Actor, decisionActorUser)
	}
	if decision.Summary != "ship it" {
		t.Fatalf("Summary = %q, want trimmed summary", decision.Summary)
	}
}

func TestAgentActorHintNamesOperatorPath(t *testing.T) {
	for _, action := range []string{"approval", "rejection", "other"} {
		hint := agentActorHint(action)
		if !strings.Contains(hint, "approve --auto") {
			t.Fatalf("agentActorHint(%q) = %q, want delegation path mentioned", action, hint)
		}
	}
}
