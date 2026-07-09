package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser() returned nil")
	}
}

func TestParser_ParseContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantSteps int
		wantErr   bool
	}{
		{
			name: "valid workflow with 3 steps",
			content: `# Workflow

## Step 1: REVIEW — Understand the inputs

**Goal:** Review all input specifications.

**Actions:**
1. Read the documentation
2. Identify key requirements

**Output:** Understanding of requirements

**Checkpoint:** None — proceed to Step 2

---

## Step 2: ANALYZE — Process data

**Goal:** Analyze the collected data.

**Actions:**
1. Process data
2. Validate results

**Output:** Analyzed data

**Checkpoint:** APPROVAL REQUIRED — Wait for user

---

## Step 3: COMPLETE — Finish work

**Goal:** Complete the workflow.

**Actions:**
1. Wrap up work
2. Document results

**Output:** Completed work

**Checkpoint:** None — phase ends`,
			wantSteps: 3,
			wantErr:   false,
		},
		{
			name:      "empty content",
			content:   "",
			wantSteps: 0,
			wantErr:   false,
		},
		{
			name:      "no step headers",
			content:   "# Just a title\n\nSome content without steps.",
			wantSteps: 0,
			wantErr:   false,
		},
		{
			name: "single step",
			content: `## Step 1: READ — Understand

**Goal:** Build understanding.

**Actions:**
1. Read docs

**Output:** Understanding

**Checkpoint:** None`,
			wantSteps: 1,
			wantErr:   false,
		},
		{
			name: "step with no actions",
			content: `## Step 1: REVIEW — Review

**Goal:** Review inputs.

**Output:** Understanding

**Checkpoint:** None`,
			wantSteps: 1,
			wantErr:   false,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps, err := parser.ParseContent(context.Background(), tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(steps) != tt.wantSteps {
				t.Errorf("ParseContent() got %d steps, want %d", len(steps), tt.wantSteps)
			}
		})
	}
}

func TestParser_ParseContent_StepDetails(t *testing.T) {
	content := `## Step 2: ANALYZE — Process the data

**Goal:** Analyze and validate all data points.

**Actions:**
1. Parse input files
2. Validate data structure
3. Generate report

**Output:** Validated data report

**Checkpoint:** APPROVAL REQUIRED — Wait for user response`

	parser := NewParser()
	steps, err := parser.ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("ParseContent() got %d steps, want 1", len(steps))
	}

	step := steps[0]

	if step.Number != 2 {
		t.Errorf("Step.Number = %d, want 2", step.Number)
	}

	if step.Name != "ANALYZE" {
		t.Errorf("Step.Name = %q, want ANALYZE", step.Name)
	}

	if step.Goal != "Analyze and validate all data points." {
		t.Errorf("Step.Goal = %q", step.Goal)
	}

	if len(step.Actions) != 3 {
		t.Errorf("Step.Actions has %d items, want 3", len(step.Actions))
	}

	expectedActions := []string{
		"Parse input files",
		"Validate data structure",
		"Generate report",
	}
	for i, expected := range expectedActions {
		if i < len(step.Actions) && step.Actions[i] != expected {
			t.Errorf("Step.Actions[%d] = %q, want %q", i, step.Actions[i], expected)
		}
	}

	if step.Output != "Validated data report" {
		t.Errorf("Step.Output = %q", step.Output)
	}

	if step.Checkpoint != CheckpointUserApproval {
		t.Errorf("Step.Checkpoint = %v, want CheckpointUserApproval", step.Checkpoint)
	}
}

func TestParser_ParseContent_NestedActions(t *testing.T) {
	content := `## Step 1: DECOMPOSE — Break Down Goals

**Goal:** Transform requirements into structure.

**Actions:**
1. Identify the Festival Goal
2. Break into Phase Goals:
   - What planning phases are needed?
   - What implementation phases are needed?
3. Document the hierarchy

**Output:** Structure

**Checkpoint:** None`

	parser := NewParser()
	steps, err := parser.ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if len(steps[0].Actions) != 3 {
		t.Fatalf("got %d actions, want 3: %#v", len(steps[0].Actions), steps[0].Actions)
	}
	action := steps[0].Actions[1]
	if !strings.Contains(action, "Break into Phase Goals:") ||
		!strings.Contains(action, "- What planning phases are needed?") ||
		!strings.Contains(action, "- What implementation phases are needed?") {
		t.Fatalf("nested action detail was not preserved:\n%s", action)
	}
}

func TestParser_CheckpointDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantType CheckpointType
	}{
		{
			name: "user approval",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test

**Checkpoint:** APPROVAL REQUIRED — Wait for user`,
			wantType: CheckpointUserApproval,
		},
		{
			name: "user approval variant",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test

**Checkpoint:** USER APPROVAL needed`,
			wantType: CheckpointUserApproval,
		},
		{
			name: "documentation checkpoint",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test

**Checkpoint:** DOCUMENTATION required`,
			wantType: CheckpointDocumentation,
		},
		{
			name: "verification checkpoint",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test

**Checkpoint:** VERIFICATION needed`,
			wantType: CheckpointVerification,
		},
		{
			name: "none checkpoint",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test

**Checkpoint:** None — proceed`,
			wantType: CheckpointNone,
		},
		{
			name: "no checkpoint section",
			content: `## Step 1: TEST — Test

**Goal:** Test

**Actions:**
1. Test

**Output:** Test`,
			wantType: CheckpointNone,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps, err := parser.ParseContent(context.Background(), tt.content)
			if err != nil {
				t.Fatalf("ParseContent() error = %v", err)
			}
			if len(steps) != 1 {
				t.Fatalf("ParseContent() got %d steps, want 1", len(steps))
			}
			if steps[0].Checkpoint != tt.wantType {
				t.Errorf("Checkpoint = %v, want %v", steps[0].Checkpoint, tt.wantType)
			}
		})
	}
}

func TestParser_Parse_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "WORKFLOW.md")

	content := `# Test Workflow

## Step 1: READ — Read input

**Goal:** Read all input files.

**Actions:**
1. Open files
2. Read content

**Output:** File contents

**Checkpoint:** None — proceed
`

	if err := os.WriteFile(workflowPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	parser := NewParser()
	steps, err := parser.Parse(context.Background(), workflowPath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Errorf("Parse() got %d steps, want 1", len(steps))
	}
}

func TestParser_Parse_FileNotFound(t *testing.T) {
	parser := NewParser()
	_, err := parser.Parse(context.Background(), "/nonexistent/path/WORKFLOW.md")
	if err == nil {
		t.Error("Parse() should fail for non-existent file")
	}
}

func TestParser_StepCount(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "WORKFLOW.md")

	content := `# Test

## Step 1: ONE — First

**Goal:** First

## Step 2: TWO — Second

**Goal:** Second

## Step 3: THREE — Third

**Goal:** Third
`

	if err := os.WriteFile(workflowPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	parser := NewParser()
	count, err := parser.StepCount(context.Background(), workflowPath)
	if err != nil {
		t.Fatalf("StepCount() error = %v", err)
	}

	if count != 3 {
		t.Errorf("StepCount() = %d, want 3", count)
	}
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := parser.ParseContent(ctx, "any content")
	if err == nil {
		t.Error("ParseContent() should fail with cancelled context")
	}

	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "WORKFLOW.md")
	if err := os.WriteFile(workflowPath, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = parser.Parse(ctx, workflowPath)
	if err == nil {
		t.Error("Parse() should fail with cancelled context")
	}

	_, err = parser.StepCount(ctx, workflowPath)
	if err == nil {
		t.Error("StepCount() should fail with cancelled context")
	}
}

func TestParser_RealWorkflowTemplate(t *testing.T) {
	content := `---
fest_type: workflow
---

# Ingest Phase Workflow

## Step 1: GATHER — Copy All Requirement Artifacts into input_specs/

**Goal:** Ensure every piece of requirement material the user already has is present in input_specs/ before any reading or analysis begins.

**Actions:**
1. Ask the user what requirement artifacts exist: intents, notes, structured specs, prior planning text, docs, or any other relevant material
2. Copy or transcribe each artifact into input_specs/
3. If the user seeded content at festival creation time, confirm it is present in input_specs/
4. List the files now in input_specs/ and confirm with the user that nothing is missing

**Output:** All available requirement artifacts present in input_specs/

**Checkpoint:** None — proceed to Step 2

---

## Step 2: READ — Understand All Input

**Goal:** Build comprehensive understanding of what the user has provided.

**Actions:**
1. List all files in input_specs/
2. Read each file completely — do not skim
3. Identify: What is the user trying to accomplish?
4. Note any questions or ambiguities

**Output:** Mental model of the user's intent (no document yet)

**Checkpoint:** None — proceed to Step 3

---

## Step 3: EXTRACT — Identify Key Elements

**Goal:** Pull out the essential information that needs to be structured.

**Actions:**
1. Extract festival purpose
2. Extract requirements
3. Extract constraints
4. Extract context

**Output:** Notes on each element (can be rough)

**Checkpoint:** None — proceed to Step 4

---

## Step 4: STRUCTURE — Produce Output Specs

**Goal:** Transform extracted elements into structured documents.

**Actions:**
1. Create output_specs/purpose.md
2. Create output_specs/requirements.md
3. Create output_specs/constraints.md
4. Create output_specs/context.md

**Output:** Four documents in output_specs/

**Checkpoint:** None — proceed to Step 5

---

## Step 5: PRESENT — Get User Approval

**Goal:** Verify the structured output captures the user's intent.

**Actions:**
1. Summarize what you've produced
2. Highlight any interpretations you made
3. Ask: "Do these specs accurately capture what you want to accomplish?"

**Output:** Summary presented to user

**Checkpoint:** APPROVAL REQUIRED — Wait for user response

---

## Step 6: ITERATE or COMPLETE

**Goal:** Handle user feedback or finalize the phase.

**Actions:**
1. If user rejects: Note feedback, return to Step 4, update specs
2. If user approves: Mark phase complete

**Output:** Phase completion or iteration

**Checkpoint:** None — phase ends
`

	parser := NewParser()
	steps, err := parser.ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}

	if len(steps) != 6 {
		t.Errorf("ParseContent() got %d steps, want 6", len(steps))
	}

	expectedNames := []string{"GATHER", "READ", "EXTRACT", "STRUCTURE", "PRESENT", "ITERATE or COMPLETE"}
	for i, name := range expectedNames {
		if i < len(steps) && steps[i].Name != name {
			t.Errorf("Step %d name = %q, want %q", i+1, steps[i].Name, name)
		}
	}

	if steps[4].Checkpoint != CheckpointUserApproval {
		t.Errorf("Step 5 checkpoint = %v, want CheckpointUserApproval", steps[4].Checkpoint)
	}

	if len(steps[0].Actions) != 4 {
		t.Errorf("Step 1 has %d actions, want 4", len(steps[0].Actions))
	}
}

func TestParser_PhaseGateDocument(t *testing.T) {
	// Test parsing a GATES.md document that uses **Question:** instead of **Goal:**
	content := `---
fest_type: phase_gate
fest_id: 001_PLAN-GATE
fest_parent: 001_PLAN
---

# Planning Phase Gate

## Step 1: METHODOLOGY — Verify Workflow Compliance

**Question:** Were all workflow steps completed without skipping?

**Actions:**
1. Verify all workflow steps show completed status
2. Confirm no steps were skipped without documented justification

**Checkpoint:** APPROVAL REQUIRED — Confirm methodology compliance

---

## Step 2: APPROVAL — Verify User Sign-off

**Question:** Did the user explicitly approve the planning outputs?

**Actions:**
1. Check that the user reviewed and approved deliverables
2. Confirm approval is documented

**Checkpoint:** APPROVAL REQUIRED — Confirm user approval obtained

---

## Step 3: STRUCTURE — Verify Festival Integrity

**Question:** Is the festival structure valid and complete?

**Actions:**
1. Run fest validate
2. Verify all markers are filled
3. Confirm phase dependencies are satisfied

**Checkpoint:** APPROVAL REQUIRED — Confirm structure validated
`

	parser := NewParser()
	steps, err := parser.ParseContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("ParseContent() got %d steps, want 3", len(steps))
	}

	// Verify **Question:** is parsed into the Goal field
	expectedGoals := []string{
		"Were all workflow steps completed without skipping?",
		"Did the user explicitly approve the planning outputs?",
		"Is the festival structure valid and complete?",
	}
	for i, want := range expectedGoals {
		if steps[i].Goal != want {
			t.Errorf("Step %d Goal = %q, want %q", i+1, steps[i].Goal, want)
		}
	}

	// Verify all steps have approval checkpoints
	for i, step := range steps {
		if step.Checkpoint != CheckpointUserApproval {
			t.Errorf("Step %d checkpoint = %v, want CheckpointUserApproval", i+1, step.Checkpoint)
		}
	}

	// Verify step names
	expectedNames := []string{"METHODOLOGY", "APPROVAL", "STRUCTURE"}
	for i, name := range expectedNames {
		if steps[i].Name != name {
			t.Errorf("Step %d name = %q, want %q", i+1, steps[i].Name, name)
		}
	}

	// Verify actions parsed
	if len(steps[0].Actions) != 2 {
		t.Errorf("Step 1 has %d actions, want 2", len(steps[0].Actions))
	}
}
