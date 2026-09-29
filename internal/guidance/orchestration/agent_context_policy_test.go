package orchestration

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

// TestAgentContextIncludesPolicy asserts placement, not wording, so it uses the
// constant rather than repeating the sentence.
func TestAgentContextIncludesPolicy(t *testing.T) {
	out := buildAgentContextSection("/f", "/f/001_PHASE", "/f/001_PHASE/01_seq")

	if !strings.Contains(out, guidance.ExecutorBlockerPolicy) {
		t.Errorf("agent context section is missing the executor blocker policy:\n%s", out)
	}
	if strings.Index(out, guidance.ExecutorBlockerPolicy) < strings.Index(out, "SEQUENCE_GOAL.md") {
		t.Errorf("the policy must follow the context files:\n%s", out)
	}
	if strings.ContainsRune(out, '—') {
		t.Errorf("agent context section contains an em dash:\n%s", out)
	}
}
