package workflow

import (
	"context"
	"testing"
)

func TestClassifyCheckpoint_OperatorAttestationHeuristics(t *testing.T) {
	step := WorkflowStep{
		Name:           "APPROVAL",
		Goal:           "Did the user validate the structured output?",
		Checkpoint:     CheckpointUserApproval,
		CheckpointText: "APPROVAL REQUIRED — Confirm user validated output",
	}
	if got := ClassifyCheckpoint(step); got != CheckpointClassOperatorAttestation {
		t.Fatalf("ClassifyCheckpoint() = %q, want operator_attestation", got)
	}
}

func TestClassifyCheckpoint_PresentIsArtifactReview(t *testing.T) {
	step := WorkflowStep{
		Name:           "PRESENT",
		Goal:           "Verify the structured output captures the user's intent.",
		Checkpoint:     CheckpointUserApproval,
		CheckpointText: "APPROVAL REQUIRED — Wait for user response",
		Output:         "Summary presented to user",
	}
	if got := ClassifyCheckpoint(step); got != CheckpointClassArtifactReview {
		t.Fatalf("ClassifyCheckpoint() = %q, want artifact_review", got)
	}
	if !IsPresentationStep(step) {
		t.Fatal("IsPresentationStep() = false, want true")
	}
}

func TestClassifyCheckpoint_ExplicitAnnotationWins(t *testing.T) {
	step := WorkflowStep{
		Name:            "APPROVAL",
		Goal:            "Did the user validate?",
		CheckpointClass: CheckpointClassArtifactReview,
	}
	if got := ClassifyCheckpoint(step); got != CheckpointClassArtifactReview {
		t.Fatalf("explicit class ignored: got %q", got)
	}
}

func TestParser_CheckpointClassAndEvidence(t *testing.T) {
	content := `## Step 4: PRESENT — Get User Approval

**Goal:** Verify the structured output captures the user's intent.

**Checkpoint class:** artifact_review

**Evidence:**
- output_specs/PRESENTATION.md
- output_specs/purpose.md

**Actions:**
1. Summarize what you've produced

**Output:** Summary presented to user

**Checkpoint:** APPROVAL REQUIRED — Wait for user response
`
	steps, err := NewParser().ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	s := steps[0]
	if s.CheckpointClass != CheckpointClassArtifactReview {
		t.Fatalf("CheckpointClass = %q", s.CheckpointClass)
	}
	if len(s.EvidencePaths) != 2 {
		t.Fatalf("EvidencePaths = %v, want 2", s.EvidencePaths)
	}
	if s.EvidencePaths[0] != "output_specs/PRESENTATION.md" {
		t.Fatalf("EvidencePaths[0] = %q", s.EvidencePaths[0])
	}
	if s.CheckpointText == "" {
		t.Fatal("CheckpointText empty")
	}
}

func TestParser_OperatorAttestationClass(t *testing.T) {
	content := `## Step 3: APPROVAL — Verify User Validated Output

**Question:** Did the user validate the structured output?

**Checkpoint class:** operator_attestation

**Actions:**
1. Confirm the user reviewed and approved the output specifications

**Checkpoint:** APPROVAL REQUIRED — Confirm user validated output
`
	steps, err := NewParser().ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	if steps[0].CheckpointClass != CheckpointClassOperatorAttestation {
		t.Fatalf("CheckpointClass = %q", steps[0].CheckpointClass)
	}
	if ClassifyCheckpoint(steps[0]) != CheckpointClassOperatorAttestation {
		t.Fatal("ClassifyCheckpoint should keep explicit operator_attestation")
	}
}
