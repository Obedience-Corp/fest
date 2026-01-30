package workflow

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Compile regex patterns once for efficiency.
var (
	// stepHeaderRe matches step headers like "## Step 1: NAME — Description"
	// NAME can contain spaces (e.g., "ITERATE or COMPLETE")
	// Description is optional (some steps don't have " — Description" suffix)
	stepHeaderRe = regexp.MustCompile(`(?m)^##\s+Step\s+(\d+):\s+(.+?)(?:\s*[—-]\s*(.*))?$`)

	// goalRe matches the goal section
	goalRe = regexp.MustCompile(`(?s)\*\*Goal:\*\*\s*(.+?)(?:\n\n|\n\*\*)`)

	// actionsRe matches the actions section
	actionsRe = regexp.MustCompile(`(?s)\*\*Actions:\*\*\s*(.+?)(?:\n\n|\n\*\*)`)

	// actionItemRe matches individual action items (numbered list)
	actionItemRe = regexp.MustCompile(`(?m)^\s*\d+\.\s+(.+)$`)

	// outputRe matches the output section
	outputRe = regexp.MustCompile(`(?s)\*\*Output:\*\*\s*(.+?)(?:\n\n|\n\*\*|$)`)

	// checkpointRe matches the checkpoint section
	checkpointRe = regexp.MustCompile(`(?s)\*\*Checkpoint:\*\*\s*(.+?)(?:\n\n|---|\n##|$)`)
)

// Parser parses WORKFLOW.md files and extracts steps.
type Parser struct{}

// NewParser creates a new workflow parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse reads and parses a WORKFLOW.md file from the filesystem.
func (p *Parser) Parse(ctx context.Context, path string) ([]WorkflowStep, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return p.ParseContent(ctx, string(content))
}

// ParseContent parses WORKFLOW.md content from a string.
func (p *Parser) ParseContent(ctx context.Context, content string) ([]WorkflowStep, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Find all step headers and their positions
	stepMatches := stepHeaderRe.FindAllStringSubmatchIndex(content, -1)
	if len(stepMatches) == 0 {
		return []WorkflowStep{}, nil
	}

	steps := make([]WorkflowStep, 0, len(stepMatches))

	for i, match := range stepMatches {
		// Determine the section content (from this header to the next or end)
		sectionStart := match[0]
		sectionEnd := len(content)
		if i+1 < len(stepMatches) {
			sectionEnd = stepMatches[i+1][0]
		}
		section := content[sectionStart:sectionEnd]

		// Extract step number
		numStr := content[match[2]:match[3]]
		num, _ := strconv.Atoi(numStr)

		// Extract step name
		name := content[match[4]:match[5]]

		step := WorkflowStep{
			Number: num,
			Name:   name,
		}

		// Extract goal
		if goalMatch := goalRe.FindStringSubmatch(section); len(goalMatch) > 1 {
			step.Goal = strings.TrimSpace(goalMatch[1])
		}

		// Extract actions
		if actionsMatch := actionsRe.FindStringSubmatch(section); len(actionsMatch) > 1 {
			actionItems := actionItemRe.FindAllStringSubmatch(actionsMatch[1], -1)
			step.Actions = make([]string, 0, len(actionItems))
			for _, item := range actionItems {
				if len(item) > 1 {
					step.Actions = append(step.Actions, strings.TrimSpace(item[1]))
				}
			}
		}

		// Extract output
		if outputMatch := outputRe.FindStringSubmatch(section); len(outputMatch) > 1 {
			step.Output = strings.TrimSpace(outputMatch[1])
		}

		// Extract checkpoint
		step.Checkpoint = p.parseCheckpoint(section)

		steps = append(steps, step)
	}

	return steps, nil
}

// parseCheckpoint determines the checkpoint type from the section content.
func (p *Parser) parseCheckpoint(section string) CheckpointType {
	checkpointMatch := checkpointRe.FindStringSubmatch(section)
	if len(checkpointMatch) < 2 {
		return CheckpointNone
	}

	checkpointText := strings.ToUpper(strings.TrimSpace(checkpointMatch[1]))

	switch {
	case strings.Contains(checkpointText, "APPROVAL REQUIRED") ||
		strings.Contains(checkpointText, "USER APPROVAL"):
		return CheckpointUserApproval

	case strings.Contains(checkpointText, "DOCUMENTATION"):
		return CheckpointDocumentation

	case strings.Contains(checkpointText, "VERIFICATION") ||
		strings.Contains(checkpointText, "VERIFY"):
		return CheckpointVerification

	case strings.Contains(checkpointText, "NONE"):
		return CheckpointNone

	default:
		// If there's checkpoint text that's not "None", treat as needing attention
		if len(checkpointText) > 0 && !strings.HasPrefix(checkpointText, "NONE") {
			return CheckpointVerification
		}
		return CheckpointNone
	}
}

// StepCount returns the number of steps in a WORKFLOW.md file without parsing fully.
func (p *Parser) StepCount(ctx context.Context, path string) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	matches := stepHeaderRe.FindAllStringIndex(string(content), -1)
	return len(matches), nil
}
