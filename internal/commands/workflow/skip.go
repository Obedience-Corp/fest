package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var workflowSkipTTYCheck = term.IsTerminal

func newSkipCmd() *cobra.Command {
	var reason string
	var terminalState string

	cmd := &cobra.Command{
		Use:   "skip",
		Short: "Operator override: mark workflow steps as skipped/completed",
		Long: `Mark remaining steps in a workflow phase with an explicit terminal state.

Use this when work was already completed outside the normal fest next loop and
you need a documented operator override with an audit reason.
Example: backfilling earlier phases for ai-investor-outreach-system-AI0001.

Security:
  - Human operator only (excluded from agent manifest access)
  - Requires an interactive TTY
  - Requires --reason for auditability`,
		Annotations: map[string]string{
			"scope":         string(scope.Festival),
			"agent_allowed": "false",
			"agent_reason":  "Operator-only override for workflow progression; requires interactive TTY and human intent",
			"interactive":   "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := parseSkipTerminalState(terminalState)
			if err != nil {
				return err
			}
			reason = strings.TrimSpace(reason)
			if reason == "" {
				return fmt.Errorf("--reason is required\n\nUsage: fest workflow skip --reason \"already completed externally\"")
			}
			return runSkip(cmd.Context(), reason, state)
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "human-readable reason for operator override (required)")
	cmd.Flags().StringVar(&terminalState, "as", "skipped", "terminal state to apply: skipped or completed")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func parseSkipTerminalState(raw string) (wf.StepStatus, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "skipped":
		return wf.StepStatusSkipped, nil
	case "completed":
		return wf.StepStatusCompleted, nil
	default:
		return "", fmt.Errorf("invalid --as value %q (expected: skipped or completed)", raw)
	}
}

func requireWorkflowSkipTTY() error {
	stdinIsTTY := workflowSkipTTYCheck(int(os.Stdin.Fd()))
	stderrIsTTY := workflowSkipTTYCheck(int(os.Stderr.Fd()))
	if stdinIsTTY && stderrIsTTY {
		return nil
	}
	return fmt.Errorf("workflow skip requires an interactive TTY (stdin and stderr)\n\nRun this command directly in a terminal as a human operator")
}

func runSkip(ctx context.Context, reason string, terminalState wf.StepStatus) error {
	if err := requireWorkflowSkipTTY(); err != nil {
		return err
	}

	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		return err
	}

	state := nav.GetWorkflowState()
	steps := nav.GetSteps()

	if state.IsComplete() {
		fmt.Println(ui.Success("✓ Workflow already complete!"))
		return nil
	}

	if terminalState != wf.StepStatusSkipped && terminalState != wf.StepStatusCompleted {
		return fmt.Errorf("invalid terminal state: %s", terminalState)
	}

	updatedCount := 0
	for !state.IsComplete() {
		currentStepNum := state.CurrentStep
		if currentStepNum < 1 || currentStepNum > len(steps) {
			return fmt.Errorf("invalid workflow state: current step %d out of range", currentStepNum)
		}

		step := steps[currentStepNum-1]
		if err := nav.SkipCurrentStep(ctx, terminalState, reason); err != nil {
			return fmt.Errorf("skipping step %d: %w", currentStepNum, err)
		}

		fmt.Printf("%s Step %d: %s marked %s\n", ui.Warning("⚠"), currentStepNum, step.Name, terminalState)
		updatedCount++
		state = nav.GetWorkflowState()
	}

	fmt.Println()
	fmt.Printf("%s Applied operator override to %d step(s)\n", ui.Success("✓"), updatedCount)
	fmt.Printf("%s: %s\n", ui.Label("Terminal State"), terminalState)
	fmt.Printf("%s: %s\n", ui.Label("Reason"), reason)
	fmt.Println()

	return showNextStep(ctx, nav, steps)
}
