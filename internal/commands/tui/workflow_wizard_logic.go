package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/festival"
	"github.com/Obedience-Corp/fest/internal/errors"
)

// workflowDraft holds human answers for the standalone WORKFLOW.md create wizard.
type workflowDraft struct {
	Name        string // directory-facing name → workflow_id slug
	Title       string // WORKFLOW.md H1 (defaults to Name)
	Description string // one-line intent
	StepsText   string // Name|Goal per line
}

// parseWorkflowStepsText parses "Name|Goal" lines (empty lines skipped).
// A line without "|" uses the whole line as the name and a default goal.
func parseWorkflowStepsText(raw string) ([]festival.WorkflowStepInput, error) {
	var steps []festival.WorkflowStepInput
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, errors.Validation(fmt.Sprintf("step line %d: name is required", i+1)).
				WithHint("use Name|Goal per line")
		}
		goal := "Describe this step."
		if len(parts) == 2 {
			if g := strings.TrimSpace(parts[1]); g != "" {
				goal = g
			}
		}
		steps = append(steps, festival.WorkflowStepInput{Name: name, Goal: goal})
	}
	if len(steps) == 0 {
		return nil, errors.Validation("at least one workflow step is required").
			WithHint("enter Name|Goal lines, one step per line")
	}
	return steps, nil
}

// workflowInputFromDraft builds the CLI WorkflowInput used by RunCreateWorkflow.
func workflowInputFromDraft(d *workflowDraft) (*festival.WorkflowInput, error) {
	if d == nil {
		return nil, errors.Validation("workflow draft is required")
	}
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return nil, errors.Validation("workflow name is required")
	}
	title := strings.TrimSpace(d.Title)
	if title == "" {
		title = name
	}
	steps, err := parseWorkflowStepsText(d.StepsText)
	if err != nil {
		return nil, err
	}
	return &festival.WorkflowInput{
		Title:       title,
		Description: strings.TrimSpace(d.Description),
		Steps:       steps,
	}, nil
}

// workflowStepsJSON marshals the draft into --steps JSON for RunCreateWorkflow.
func workflowStepsJSON(d *workflowDraft) (string, error) {
	input, err := workflowInputFromDraft(d)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(input)
	if err != nil {
		return "", errors.Wrap(err, "marshal workflow steps")
	}
	return string(b), nil
}

// workflowConfirmSummary is a short multi-line note for the confirm step.
func workflowConfirmSummary(d *workflowDraft) string {
	input, err := workflowInputFromDraft(d)
	if err != nil {
		return err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Name:  %s\n", strings.TrimSpace(d.Name))
	fmt.Fprintf(&b, "Title: %s\n", input.Title)
	if input.Description != "" {
		fmt.Fprintf(&b, "Intent: %s\n", input.Description)
	}
	fmt.Fprintf(&b, "Steps (%d):\n", len(input.Steps))
	for i, s := range input.Steps {
		fmt.Fprintf(&b, "  %d. %s — %s\n", i+1, s.Name, s.Goal)
	}
	b.WriteString("\nWrites WORKFLOW.md in the current directory and starts a run (fest next).")
	return b.String()
}
