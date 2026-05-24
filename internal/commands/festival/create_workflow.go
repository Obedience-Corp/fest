package festival

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
	"github.com/spf13/cobra"
)

// CreateWorkflowOptions holds options for the create workflow command.
type CreateWorkflowOptions struct {
	Steps      string // Inline JSON
	StepsFile  string // Path to JSON file
	Position   string // before|after (default: after)
	Path       string // Phase directory (default: cwd)
	Festival   string // --festival override
	JSONOutput bool
	AgentMode  bool

	Name   string
	Type   string
	NoInit bool
}

// WorkflowInput defines the JSON input structure for workflow creation.
type WorkflowInput struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Steps       []WorkflowStepInput `json:"steps"`
}

// WorkflowStepInput defines a single step in the workflow input.
type WorkflowStepInput struct {
	Name       string   `json:"name"`
	Goal       string   `json:"goal"`
	Actions    []string `json:"actions"`
	Output     string   `json:"output"`
	Checkpoint string   `json:"checkpoint"` // none|approval_required|verification|documentation
}

type createWorkflowResult struct {
	OK          bool             `json:"ok"`
	Action      string           `json:"action"`
	Workflow    map[string]any   `json:"workflow,omitempty"`
	Created     []string         `json:"created,omitempty"`
	Errors      []map[string]any `json:"errors,omitempty"`
	Suggestions []string         `json:"suggestions,omitempty"`
}

// NewCreateWorkflowCommand creates the 'create workflow' subcommand.
func NewCreateWorkflowCommand() *cobra.Command {
	opts := &CreateWorkflowOptions{}
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Create a standalone or phase WORKFLOW.md from structured step definitions",
		Long: `Generate a parseable WORKFLOW.md from prompts, inline JSON, or a steps file.

Outside a festival phase, this creates WORKFLOW.md in the current directory,
initializes .workflow/ runtime state, and starts a tracked run so fest next works
immediately.

Inside a festival phase, or when --path/--festival targets a phase, this writes
the phase WORKFLOW.md without standalone runtime state.

Examples:
  # Standalone workflow in the current directory
  fest create workflow demo

  # Standalone workflow from inline JSON (for agents)
  fest create workflow demo --steps '{"title":"Review","steps":[...]}'

  # Phase workflow from a steps file
  fest create workflow --steps-file steps.json --position after

  # Explicit phase path
  fest create workflow --steps-file steps.json --path ./004_POLISH`,
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Name = args[0]
			}
			return RunCreateWorkflow(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.Steps, "steps", "", "Inline JSON with workflow definition")
	cmd.Flags().StringVar(&opts.StepsFile, "steps-file", "", "Path to JSON file with workflow definition")
	cmd.Flags().StringVar(&opts.Position, "position", "after", "Workflow position relative to sequences (before|after)")
	cmd.Flags().StringVar(&opts.Path, "path", ".", "Phase directory path (festival mode)")
	cmd.Flags().StringVar(&opts.Festival, "festival", "", "Festival root override")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Strict agent mode (implies --json)")
	cmd.Flags().StringVar(&opts.Type, "type", "task", "workflow type (standalone mode only)")
	cmd.Flags().BoolVar(&opts.NoInit, "no-init", false, "skip .workflow/ runtime init (advanced standalone mode)")
	return cmd
}

// RunCreateWorkflow executes the create workflow command. Dispatches to
// the festival or standalone handler based on standalone.Resolve mode (D009).
func RunCreateWorkflow(ctx context.Context, opts *CreateWorkflowOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("RunCreateWorkflow")
	}

	if opts.AgentMode {
		opts.JSONOutput = true
	}

	// Explicit festival targets win over cwd-derived mode: a caller passing
	// --path /tmp/festival/001_PLAN or --festival <name> from outside any
	// festival expects festival-mode handling, not "standalone rejects --path"
	// (review feedback on PR #173).
	hasExplicitFestivalTarget := opts.Festival != "" || (opts.Path != "" && opts.Path != ".")
	if hasExplicitFestivalTarget {
		return runFestivalCreateWorkflow(ctx, opts)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "get cwd")
	}
	res, err := standalone.Resolve(ctx, cwd)
	if err != nil {
		return errors.Wrap(err, "resolve mode").
			WithHint("run inside a campaign root or pass --festival explicitly")
	}
	switch res.Mode {
	case standalone.ModeFestival:
		return runFestivalCreateWorkflow(ctx, opts)
	case standalone.ModeTracked, standalone.ModeAnonymous, standalone.ModeNone:
		return runStandaloneCreateWorkflow(ctx, opts, res, cwd)
	default:
		return errors.New("unknown resolver mode").
			WithField("mode", string(res.Mode))
	}
}

// runFestivalCreateWorkflow is the original festival-mode handler. Body
// preserved unchanged from the pre-dispatch implementation (D009 task 08).
func runFestivalCreateWorkflow(ctx context.Context, opts *CreateWorkflowOptions) error {
	input, err := parseWorkflowInput(opts)
	if err != nil {
		return emitWorkflowError(opts, err)
	}

	if err := validateWorkflowInput(input); err != nil {
		return emitWorkflowError(opts, err)
	}

	if err := validateWorkflowPosition(opts.Position); err != nil {
		return emitWorkflowError(opts, err)
	}

	phasePath, err := resolveWorkflowPhasePath(opts)
	if err != nil {
		return emitWorkflowError(opts, err)
	}

	workflowPath := filepath.Join(phasePath, "WORKFLOW.md")
	if _, statErr := os.Stat(workflowPath); statErr == nil {
		return emitWorkflowError(opts,
			errors.Validation("WORKFLOW.md already exists in this phase").
				WithField("path", workflowPath).
				WithHint("Remove the existing WORKFLOW.md first or edit it directly"))
	}

	phaseID := filepath.Base(phasePath)
	content := renderWorkflowContent(input, phaseID, opts.Position)

	if err := os.WriteFile(workflowPath, []byte(content), 0644); err != nil {
		return emitWorkflowError(opts, errors.IO("writing WORKFLOW.md", err).WithField("path", workflowPath))
	}

	return emitWorkflowOutput(opts, phasePath, workflowPath, input, phaseID)
}

// parseWorkflowInput reads and parses the JSON input from --steps or --steps-file.
func parseWorkflowInput(opts *CreateWorkflowOptions) (*WorkflowInput, error) {
	hasSteps := strings.TrimSpace(opts.Steps) != ""
	hasFile := strings.TrimSpace(opts.StepsFile) != ""

	if !hasSteps && !hasFile {
		return nil, errors.Validation("one of --steps or --steps-file is required")
	}
	if hasSteps && hasFile {
		return nil, errors.Validation("only one of --steps or --steps-file may be provided")
	}

	var data []byte
	if hasFile {
		var err error
		data, err = os.ReadFile(opts.StepsFile)
		if err != nil {
			return nil, errors.IO("reading steps file", err).WithField("path", opts.StepsFile)
		}
	} else {
		data = []byte(opts.Steps)
	}

	var input WorkflowInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, errors.Parse("invalid JSON in workflow steps", err)
	}

	return &input, nil
}

// validateWorkflowInput checks that the input has the required fields.
func validateWorkflowInput(input *WorkflowInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return errors.Validation("workflow title is required")
	}

	if len(input.Steps) == 0 {
		return errors.Validation("at least one workflow step is required")
	}

	for i, step := range input.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return errors.Validation(fmt.Sprintf("step %d: name is required", i+1))
		}
		if strings.TrimSpace(step.Goal) == "" {
			return errors.Validation(fmt.Sprintf("step %d: goal is required", i+1))
		}
		cp := strings.TrimSpace(step.Checkpoint)
		if cp != "" && cp != "none" && cp != "approval_required" && cp != "verification" && cp != "documentation" {
			return errors.Validation(fmt.Sprintf("step %d: invalid checkpoint %q (must be none, approval_required, verification, or documentation)", i+1, cp))
		}
	}

	return nil
}

// validateWorkflowPosition checks that the position flag is valid.
func validateWorkflowPosition(position string) error {
	switch position {
	case "before", "after":
		return nil
	default:
		return errors.Validation(fmt.Sprintf("invalid position %q: must be before or after", position))
	}
}

// resolveWorkflowPhasePath resolves the target phase directory.
func resolveWorkflowPhasePath(opts *CreateWorkflowOptions) (string, error) {
	path := opts.Path
	if opts.Festival != "" {
		path = opts.Festival
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", errors.Wrap(err, "resolving path").WithField("path", path)
	}

	// Check if this is a phase directory (has PHASE_GOAL.md)
	phaseGoalPath := filepath.Join(absPath, "PHASE_GOAL.md")
	if _, err := os.Stat(phaseGoalPath); err == nil {
		return absPath, nil
	}

	// Try to detect phase from cwd using shared helper
	festivalPath := ResolveFestivalPath(absPath)
	if festivalPath != "" {
		phasePath := shared.ResolvePhasePath(absPath, festivalPath)
		if phasePath != "" {
			return phasePath, nil
		}
	}

	return "", errors.Validation("not inside a phase directory (no PHASE_GOAL.md found)").
		WithHint("Navigate to a phase directory or use --path to specify one")
}

// renderWorkflowContent generates the full WORKFLOW.md content with frontmatter.
func renderWorkflowContent(input *WorkflowInput, phaseID, position string) string {
	var sb strings.Builder

	// Frontmatter — minimal fields matching existing WORKFLOW.md conventions
	workflowID := phaseID + "-WF"
	sb.WriteString("---\n")
	sb.WriteString("fest_type: workflow\n")
	fmt.Fprintf(&sb, "fest_id: %s\n", workflowID)
	fmt.Fprintf(&sb, "fest_parent: %s\n", phaseID)
	fmt.Fprintf(&sb, "fest_workflow_position: %s\n", position)
	sb.WriteString("---\n\n")

	// Title
	fmt.Fprintf(&sb, "# %s\n", input.Title)
	sb.WriteString("\n")

	// Description
	if strings.TrimSpace(input.Description) != "" {
		sb.WriteString(input.Description)
		sb.WriteString("\n")
	}

	totalSteps := len(input.Steps)

	for i, step := range input.Steps {
		stepNum := i + 1

		sb.WriteString("\n---\n\n")

		// Step header
		fmt.Fprintf(&sb, "## Step %d: %s — %s\n", stepNum, step.Name, step.Goal)
		sb.WriteString("\n")

		// Goal
		fmt.Fprintf(&sb, "**Goal:** %s\n", step.Goal)
		sb.WriteString("\n")

		// Actions
		if len(step.Actions) > 0 {
			sb.WriteString("**Actions:**\n")
			for j, action := range step.Actions {
				fmt.Fprintf(&sb, "%d. %s\n", j+1, action)
			}
			sb.WriteString("\n")
		}

		// Output
		if strings.TrimSpace(step.Output) != "" {
			fmt.Fprintf(&sb, "**Output:** %s\n", step.Output)
			sb.WriteString("\n")
		}

		// Checkpoint
		checkpointText := formatCheckpoint(step.Checkpoint, stepNum, totalSteps)
		fmt.Fprintf(&sb, "**Checkpoint:** %s\n", checkpointText)
	}

	return sb.String()
}

// formatCheckpoint returns the display text for a checkpoint value.
func formatCheckpoint(checkpoint string, stepNum, totalSteps int) string {
	switch strings.TrimSpace(checkpoint) {
	case "approval_required":
		return "APPROVAL REQUIRED — Wait for user response"
	case "verification":
		return "VERIFICATION — Verify output before continuing"
	case "documentation":
		return "DOCUMENTATION — Document findings before continuing"
	default: // "none" or ""
		if stepNum < totalSteps {
			return fmt.Sprintf("None — proceed to Step %d", stepNum+1)
		}
		return "None — workflow complete"
	}
}

// emitWorkflowOutput handles both JSON and human-readable output.
func emitWorkflowOutput(opts *CreateWorkflowOptions, _, workflowPath string, input *WorkflowInput, phaseID string) error {
	if opts.JSONOutput {
		return emitWorkflowJSON(opts, createWorkflowResult{
			OK:     true,
			Action: "create_workflow",
			Workflow: map[string]any{
				"phase_id": phaseID,
				"id":       phaseID + "-WF",
				"title":    input.Title,
				"steps":    len(input.Steps),
				"position": opts.Position,
			},
			Created: []string{workflowPath},
			Suggestions: []string{
				"fest status        - View festival progress",
				"fest next          - Find what to work on next",
				"fest show plan     - View the execution plan",
			},
		})
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())
	display.Success("Created workflow: %s", filepath.Base(workflowPath))
	display.Info("  Phase:    %s", phaseID)
	display.Info("  Steps:    %d", len(input.Steps))
	display.Info("  Position: %s", opts.Position)
	fmt.Println()
	fmt.Println(ui.H2("Next Steps"))
	fmt.Printf("  %s\n", ui.Info("1. fest status    - Verify workflow appears in festival plan"))
	fmt.Printf("  %s\n", ui.Info("2. fest next      - Find what to work on next"))
	fmt.Printf("  %s\n", ui.Info("3. fest show plan - View the full execution plan"))
	return nil
}

// emitWorkflowError handles error output in JSON or human-readable format.
func emitWorkflowError(opts *CreateWorkflowOptions, err error) error {
	if opts.JSONOutput {
		_ = emitWorkflowJSON(opts, createWorkflowResult{
			OK:     false,
			Action: "create_workflow",
			Errors: []map[string]any{{
				"code":    "error",
				"message": err.Error(),
			}},
		})
		return nil
	}
	return err
}

// emitWorkflowJSON encodes the result as JSON to stdout.
func emitWorkflowJSON(_ *CreateWorkflowOptions, res createWorkflowResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
