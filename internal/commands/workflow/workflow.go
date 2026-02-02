// Package workflow provides commands for managing workflow-based phase execution.
package workflow

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewWorkflowCommand creates the workflow command
func NewWorkflowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflow-based phase execution",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Commands for managing workflow-based phases (ingest, research, planning).

These phases use WORKFLOW.md files with step-by-step guidance and checkpoints.
Use 'fest next' to see the current step, then these commands to advance.

Workflow Steps:
  Workflows are defined in WORKFLOW.md files within phase directories.
  Each step has a goal, actions to complete, expected output, and an optional checkpoint.

Checkpoints:
  Some steps require user approval before proceeding. Use 'fest workflow approve'
  to approve or 'fest workflow reject' to request revisions.

State:
  Workflow progress is tracked in .fest/workflow_state.yaml within the phase directory.
  Use 'fest workflow status' to view current progress.

Examples:
  fest workflow status    # Show workflow progress
  fest workflow advance   # Complete current step and move to next
  fest workflow approve   # Approve a blocking checkpoint
  fest workflow reject    # Reject checkpoint with feedback
  fest workflow reset     # Reset workflow to step 1
  fest workflow show      # Display the current step details`,
	}

	// Add subcommands
	cmd.AddCommand(
		newStatusCmd(),
		newAdvanceCmd(),
		newApproveCmd(),
		newRejectCmd(),
		newResetCmd(),
		newShowCmd(),
	)

	return cmd
}

// newStatusCmd is defined in status.go

// newAdvanceCmd is defined in advance.go

// newApproveCmd is defined in approve.go

// newRejectCmd is defined in reject.go

// newResetCmd is defined in reset.go

// newShowCmd is defined in show.go
