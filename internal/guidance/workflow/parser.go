package workflow

import (
	"context"
	"fmt"
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

	// goalRe matches the goal section (also matches **Question:** for phase gate documents)
	goalRe = regexp.MustCompile(`(?s)\*\*(?:Goal|Question):\*\*\s*(.+?)(?:\n\n|\n\*\*)`)

	// actionsRe matches the actions section
	actionsRe = regexp.MustCompile(`(?s)\*\*Actions:\*\*\s*(.+?)(?:\n\n|\n\*\*)`)

	// actionItemRe matches top-level numbered action items.
	actionItemRe = regexp.MustCompile(`^\s*\d+\.\s+(.+)$`)

	// outputRe matches the output section
	outputRe = regexp.MustCompile(`(?s)\*\*Output:\*\*\s*(.+?)(?:\n\n|\n\*\*|$)`)

	// checkpointRe matches the checkpoint section
	checkpointRe = regexp.MustCompile(`(?s)\*\*Checkpoint:\*\*\s*(.+?)(?:\n\n|---|\n##|$)`)

	// checkpointClassRe matches an explicit checkpoint class annotation.
	checkpointClassRe = regexp.MustCompile(`(?im)\*\*Checkpoint class:\*\*[ \t]*([^\r\n]*)`)

	// evidenceRe matches an explicit evidence list for approval judging.
	evidenceRe = regexp.MustCompile(`(?s)\*\*Evidence:\*\*\s*(.+?)(?:\n\n|\n\*\*|---|\n##|$)`)

	// evidenceItemRe matches a bullet evidence path: a single token on its own
	// line. It is the ONLY definition of "this bullet is a path"; a `-`/`*`
	// bullet it does not match is reported as unparsed rather than dropped.
	//
	// Non-bullet lines, other list markers, and empty bullets under
	// **Evidence:** still match neither regex and are still ignored. Closing
	// that would mean classifying every line in the block, which is a wider
	// change than the silent drop this fixes.
	evidenceItemRe = regexp.MustCompile(`(?m)^\s*[-*]\s+(\S+)\s*$`)

	// evidenceBulletRe matches EVERY bullet under **Evidence:**. Each one is
	// then classified against evidenceItemRe, so "not a path" is defined by
	// that regex rather than by a second heuristic that can drift from it.
	evidenceBulletRe = regexp.MustCompile(`(?m)^\s*[-*]\s+(.+?)\s*$`)

	// hooksMarkerRe matches a per-step "**Hooks:**" line in GATES.md.
	hooksMarkerRe = regexp.MustCompile(`(?im)\*\*Hooks:\*\*[ \t]*([^\r\n]*)`)

	// approvalMarkerRe matches a per-step "**Approval:**" line in GATES.md.
	approvalMarkerRe = regexp.MustCompile(`(?im)\*\*Approval:\*\*[ \t]*([^\r\n]*)`)

	// hookListRe matches "pre: [a, b]" or "post: [x]" fragments.
	hookListRe = regexp.MustCompile(`(?i)\b(pre|post)\s*:\s*\[([^\]]*)\]`)
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
			step.Actions = parseActionItems(actionsMatch[1])
		}

		// Extract output
		if outputMatch := outputRe.FindStringSubmatch(section); len(outputMatch) > 1 {
			step.Output = strings.TrimSpace(outputMatch[1])
		}

		// Extract checkpoint
		step.Checkpoint, step.CheckpointText = p.parseCheckpoint(section)
		checkpointClass, err := p.parseCheckpointClass(section)
		if err != nil {
			return nil, fmt.Errorf("step %d checkpoint class: %w", num, err)
		}
		step.CheckpointClass = checkpointClass
		step.EvidencePaths, step.EvidenceUnparsed = p.parseEvidence(section)
		step.Hooks = p.parseStepHooks(section)
		step.Approval = p.parseApproval(section)

		steps = append(steps, step)
	}

	return steps, nil
}

func parseActionItems(actionsBlock string) []string {
	var actions []string
	var current []string

	flush := func() {
		if len(current) == 0 {
			return
		}
		actions = append(actions, strings.TrimSpace(strings.Join(current, "\n")))
		current = nil
	}

	for _, rawLine := range strings.Split(actionsBlock, "\n") {
		line := strings.TrimRight(rawLine, " \t\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if match := actionItemRe.FindStringSubmatch(line); len(match) > 1 {
			flush()
			current = append(current, strings.TrimSpace(match[1]))
			continue
		}
		if len(current) > 0 {
			current = append(current, line)
		}
	}
	flush()
	return actions
}

// parseCheckpoint determines the checkpoint type from the section content.
// The second return is the raw checkpoint text (trimmed).
func (p *Parser) parseCheckpoint(section string) (CheckpointType, string) {
	checkpointMatch := checkpointRe.FindStringSubmatch(section)
	if len(checkpointMatch) < 2 {
		return CheckpointNone, ""
	}

	raw := strings.TrimSpace(checkpointMatch[1])
	// Keep only the first line for display/heuristics; body may include more.
	if i := strings.IndexByte(raw, '\n'); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	checkpointText := strings.ToUpper(raw)

	switch {
	case strings.Contains(checkpointText, "APPROVAL REQUIRED") ||
		strings.Contains(checkpointText, "USER APPROVAL"):
		return CheckpointUserApproval, raw

	case strings.Contains(checkpointText, "DOCUMENTATION"):
		return CheckpointDocumentation, raw

	case strings.Contains(checkpointText, "VERIFICATION") ||
		strings.Contains(checkpointText, "VERIFY"):
		return CheckpointVerification, raw

	case strings.Contains(checkpointText, "NONE"):
		return CheckpointNone, raw

	default:
		// If there's checkpoint text that's not "None", treat as needing attention
		if len(checkpointText) > 0 && !strings.HasPrefix(checkpointText, "NONE") {
			return CheckpointVerification, raw
		}
		return CheckpointNone, raw
	}
}

func (p *Parser) parseCheckpointClass(section string) (CheckpointClass, error) {
	m := checkpointClassRe.FindStringSubmatch(section)
	if len(m) < 2 {
		return CheckpointClassUnspecified, nil
	}
	value := strings.ToLower(strings.TrimSpace(m[1]))
	switch value {
	case string(CheckpointClassArtifactReview):
		return CheckpointClassArtifactReview, nil
	case string(CheckpointClassOperatorAttestation):
		return CheckpointClassOperatorAttestation, nil
	default:
		return CheckpointClassUnspecified, fmt.Errorf(
			"unsupported value %q (expected %q or %q)",
			value,
			CheckpointClassArtifactReview,
			CheckpointClassOperatorAttestation,
		)
	}
}

// parseEvidence classifies every bullet under **Evidence:** exactly once: a
// bullet is either a path or it is reported.
//
// One scan on purpose. Splitting this into a path pass and a separate
// "not a path" pass meant two definitions of what a path is, and the readiness
// gate's invariant -- no bullet is silently discarded -- held only while the
// two happened to agree.
func (p *Parser) parseEvidence(section string) (paths, unparsed []string) {
	m := evidenceRe.FindStringSubmatch(section)
	if len(m) < 2 {
		return nil, nil
	}
	for _, bullet := range evidenceBulletRe.FindAllStringSubmatch(m[1], -1) {
		if len(bullet) < 2 {
			continue
		}
		text := strings.TrimSpace(bullet[1])
		if text == "" {
			continue
		}
		if item := evidenceItemRe.FindStringSubmatch(bullet[0]); len(item) >= 2 {
			if path := strings.TrimSpace(item[1]); path != "" {
				paths = append(paths, path)
				continue
			}
		}
		unparsed = append(unparsed, text)
	}
	return paths, unparsed
}

// parseStepHooks extracts pre/post hook name lists from a **Hooks:** marker.
// Names only; malformed fragments yield empty lists (no panic).
func (p *Parser) parseStepHooks(section string) StepHooks {
	m := hooksMarkerRe.FindStringSubmatch(section)
	if len(m) < 2 {
		return StepHooks{}
	}
	value := strings.TrimSpace(m[1])
	if value == "" {
		return StepHooks{}
	}
	var hooks StepHooks
	for _, match := range hookListRe.FindAllStringSubmatch(value, -1) {
		if len(match) < 3 {
			continue
		}
		names := splitHookNames(match[2])
		switch strings.ToLower(match[1]) {
		case "pre":
			hooks.Pre = append(hooks.Pre, names...)
		case "post":
			hooks.Post = append(hooks.Post, names...)
		}
	}
	return hooks
}

func splitHookNames(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var names []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		name = strings.Trim(name, `"'`)
		if name == "" {
			continue
		}
		// Names only — reject map-like or assignment fragments.
		if strings.Contains(name, ":") || strings.ContainsAny(name, "{}") {
			continue
		}
		names = append(names, name)
	}
	return names
}

func (p *Parser) parseApproval(section string) string {
	m := approvalMarkerRe.FindStringSubmatch(section)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// ClassifyCheckpoint resolves the checkpoint class for a step.
// Explicit annotations win; otherwise heuristics inspect name/goal/checkpoint text.
//
// PRESENT / "wait for user response" steps remain artifact_review: the judge
// reviews the presentation pack. Phrases that ask whether the user already
// validated/approved are operator_attestation and cannot be auto-judged.
// Ambiguous legacy checkpoints fail closed as operator_attestation; shipped
// templates carry explicit classes so heuristics are compatibility-only.
func ClassifyCheckpoint(step WorkflowStep) CheckpointClass {
	if step.CheckpointClass == CheckpointClassArtifactReview ||
		step.CheckpointClass == CheckpointClassOperatorAttestation {
		return step.CheckpointClass
	}
	blob := strings.ToLower(strings.Join([]string{
		step.Name,
		step.Goal,
		step.CheckpointText,
		step.Output,
	}, "\n"))

	// Operator-attestation: the checkpoint asserts human acceptance already happened.
	attestationHints := []string{
		"did the user",
		"user has approved",
		"user validated",
		"user validate the",
		"confirm user validated",
		"confirm the user reviewed and approved",
		"explicitly approve the plan",
		"did the user explicitly approve",
	}
	for _, h := range attestationHints {
		if strings.Contains(blob, h) {
			return CheckpointClassOperatorAttestation
		}
	}
	if IsPresentationStep(step) {
		return CheckpointClassArtifactReview
	}
	return CheckpointClassOperatorAttestation
}

// IsPresentationStep reports whether the step is a PRESENT-style deliverable review.
func IsPresentationStep(step WorkflowStep) bool {
	name := strings.ToUpper(strings.TrimSpace(step.Name))
	if strings.Contains(name, "PRESENT") {
		return true
	}
	blob := strings.ToLower(step.CheckpointText + " " + step.Goal + " " + step.Output)
	return strings.Contains(blob, "summary presented") ||
		strings.Contains(blob, "presentation") ||
		strings.Contains(blob, "present to user")
}

// DefaultEvidencePaths returns conventional phase-relative evidence paths for a step.
func DefaultEvidencePaths(step WorkflowStep) []string {
	if len(step.EvidencePaths) > 0 {
		return append([]string(nil), step.EvidencePaths...)
	}
	if IsPresentationStep(step) {
		return []string{
			"output_specs/PRESENTATION.md",
			"output_specs/purpose.md",
			"output_specs/requirements.md",
			"output_specs/constraints.md",
			"output_specs/context.md",
		}
	}
	name := strings.ToUpper(step.Name)
	switch {
	case strings.Contains(name, "PHASE GOAL") || strings.Contains(name, "GOAL"):
		return []string{
			"PHASE_GOAL.md",
			"output_specs/purpose.md",
			"output_specs/requirements.md",
			"output_specs/constraints.md",
			"output_specs/context.md",
			"output_specs/PRESENTATION.md",
		}
	case strings.Contains(name, "COMPLETE"):
		return []string{
			"output_specs/context.md",
			"output_specs/requirements.md",
			"output_specs/PRESENTATION.md",
		}
	default:
		return nil
	}
}

// StepHeaderRegexp returns the compiled regexp used to match step headers.
// Callers that need to locate or rewrite "## Step N: ..." headings (for example,
// renumber tooling) should use this accessor instead of redefining the pattern.
func StepHeaderRegexp() *regexp.Regexp {
	return stepHeaderRe
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
