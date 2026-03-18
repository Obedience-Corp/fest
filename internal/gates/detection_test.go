package gates

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

func TestDetectGateType_KeepsExplicitReviewDespiteSecurityChecklistText(t *testing.T) {
	t.Parallel()

	content := []byte(`---
fest_type: gate
fest_gate_type: review
fest_status: pending
---
# Gate: Code Review

### Error Handling & Security

- [ ] No obvious security issues
`)

	gateType, ok := detectGateTypeFromContent("03_quality_gate_review.md", content)
	if !ok {
		t.Fatal("detectGateTypeFromContent() = not detected, want review")
	}
	if gateType != frontmatter.GateReview {
		t.Fatalf("detectGateTypeFromContent() = %q, want %q", gateType, frontmatter.GateReview)
	}
}

func TestDetectGateType_OverridesLegacyIterateCommitMismatch(t *testing.T) {
	t.Parallel()

	content := []byte(`---
fest_type: gate
fest_gate_type: iterate
fest_status: pending
---
# Gate: Commit Sequence Changes
`)

	gateType, ok := detectGateTypeFromContent("02_fest_commit.md", content)
	if !ok {
		t.Fatal("detectGateTypeFromContent() = not detected, want commit")
	}
	if gateType != frontmatter.GateCommit {
		t.Fatalf("detectGateTypeFromContent() = %q, want %q", gateType, frontmatter.GateCommit)
	}
}
