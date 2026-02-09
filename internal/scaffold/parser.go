package scaffold

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Regex patterns for parsing plan document structure.
var (
	// festivalPattern matches: - **Festival:** name
	festivalPattern = regexp.MustCompile(`^\s*-\s+\*\*Festival:\*\*\s+(.+)$`)

	// phasePattern matches: - **Phase NNN:** NAME (type) optionally followed by - STATUS
	phasePattern = regexp.MustCompile(`^\s*-\s+\*\*Phase\s+(\d+):\*\*\s+(\w+)\s+\((\w+)\)(?:\s*-\s*(.+))?$`)

	// sequencePattern matches: - Sequence NN: name optionally followed by (requirement)
	sequencePattern = regexp.MustCompile(`^\s*-\s+Sequence\s+(\d+):\s+(.+?)(?:\s+\(([^)]+)\))?\s*$`)

	// taskPattern matches: - Task NN: description
	taskPattern = regexp.MustCompile(`^\s*-\s+Task\s+(\d+):\s+(.+)$`)

	// goalPattern matches: ## Festival Goal followed by content
	goalPattern = regexp.MustCompile(`^##\s+Festival\s+Goal\s*$`)
)

// PlanParser reads markdown plan documents and extracts festival hierarchy.
type PlanParser struct{}

// NewPlanParser creates a new PlanParser.
func NewPlanParser() *PlanParser {
	return &PlanParser{}
}

// Parse reads a plan document and extracts the festival structure.
func (p *PlanParser) Parse(ctx context.Context, content string) (*ParsedPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	plan := &ParsedPlan{}
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentPhase *ParsedPhase
	var currentSequence *ParsedSequence
	var readingGoal bool
	var goalLines []string

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()

		// Check for goal section header
		if goalPattern.MatchString(line) {
			readingGoal = true
			continue
		}

		// Read goal content until next heading
		if readingGoal {
			if strings.HasPrefix(line, "##") || strings.HasPrefix(line, "- **") {
				readingGoal = false
				plan.Goal = strings.TrimSpace(strings.Join(goalLines, " "))
				// Fall through to process this line
			} else {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					goalLines = append(goalLines, trimmed)
				}
				continue
			}
		}

		// Try matching patterns in order of specificity
		if matches := festivalPattern.FindStringSubmatch(line); matches != nil {
			plan.FestivalName = strings.TrimSpace(matches[1])
			continue
		}

		if matches := phasePattern.FindStringSubmatch(line); matches != nil {
			// Save previous phase
			if currentPhase != nil {
				if currentSequence != nil {
					currentPhase.Sequences = append(currentPhase.Sequences, *currentSequence)
					currentSequence = nil
				}
				plan.Phases = append(plan.Phases, *currentPhase)
			}

			num, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("invalid phase number %q: %w", matches[1], err)
			}

			currentPhase = &ParsedPhase{
				Number: num,
				Name:   strings.TrimSpace(matches[2]),
				Type:   strings.TrimSpace(matches[3]),
				Status: strings.TrimSpace(matches[4]),
			}
			continue
		}

		if matches := sequencePattern.FindStringSubmatch(line); matches != nil {
			// Save previous sequence
			if currentSequence != nil && currentPhase != nil {
				currentPhase.Sequences = append(currentPhase.Sequences, *currentSequence)
			}

			num, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("invalid sequence number %q: %w", matches[1], err)
			}

			currentSequence = &ParsedSequence{
				Number:      num,
				Name:        strings.TrimSpace(matches[2]),
				Requirement: strings.TrimSpace(matches[3]),
			}
			continue
		}

		if matches := taskPattern.FindStringSubmatch(line); matches != nil {
			num, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("invalid task number %q: %w", matches[1], err)
			}

			// Strip backticks from task names for cleaner filenames
			taskName := strings.ReplaceAll(strings.TrimSpace(matches[2]), "`", "")

			task := ParsedTask{
				Number: num,
				Name:   taskName,
			}

			if currentSequence != nil {
				currentSequence.Tasks = append(currentSequence.Tasks, task)
			}
			continue
		}

		// Collect phase descriptions (indented text under a phase, not a sequence or task)
		if currentPhase != nil && currentSequence == nil {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(line, "    ") && trimmed != "" && !strings.HasPrefix(trimmed, "-") {
				if currentPhase.Description != "" {
					currentPhase.Description += " "
				}
				currentPhase.Description += trimmed
			}
		}
	}

	// Save the last phase/sequence
	if currentSequence != nil && currentPhase != nil {
		currentPhase.Sequences = append(currentPhase.Sequences, *currentSequence)
	}
	if currentPhase != nil {
		plan.Phases = append(plan.Phases, *currentPhase)
	}

	// Save any remaining goal content
	if readingGoal && len(goalLines) > 0 {
		plan.Goal = strings.TrimSpace(strings.Join(goalLines, " "))
	}

	if plan.FestivalName == "" && len(plan.Phases) == 0 {
		return nil, fmt.Errorf("no festival structure found in plan document")
	}

	return plan, nil
}

// ParseFile reads a plan document file and extracts the festival structure.
func (p *PlanParser) ParseFile(ctx context.Context, path string) (*ParsedPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := readFileContent(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file %s: %w", path, err)
	}

	return p.Parse(ctx, string(data))
}
