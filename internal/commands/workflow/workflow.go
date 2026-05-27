// Package workflow provides commands for managing workflow-based phase execution.
package workflow

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// phaseFlag holds the --phase flag value shared across workflow subcommands
var phaseFlag string

// NewWorkflowCommand creates the workflow command
func NewWorkflowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflow-based phase execution",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Commands for managing step-based phase navigation (workflows and phase gates).

These commands work with WORKFLOW.md files (step-by-step guidance for workflow phases)
and GATES.md files (phase-level quality gates for all phase types). Use 'fest next'
to see the current step, then these commands to advance.

Workflow Steps:
  Workflows are defined in WORKFLOW.md files within phase directories.
  Each step has a goal, actions to complete, expected output, and an optional checkpoint.

Phase Gates:
  Gates are defined in GATES.md files and run after all other phase work is complete.
  Each gate step poses a quality/compliance question requiring approval before the
  phase can advance. Gates are available for all phase types.

Checkpoints:
  Some steps require user approval before proceeding. Use 'fest workflow approve'
  to approve or 'fest workflow reject' to request revisions.

State:
  Progress is tracked in <festival>/.fest/progress_events.jsonl.
  Use 'fest workflow status' to view current progress.

Running from Festival Root:
  When run from the festival root (not inside a phase directory), the command
  auto-detects the first incomplete navigable phase (workflow or gate).
  Use --phase to specify a particular phase.

Auto-Routing:
  Commands automatically target the correct document:
  - WORKFLOW.md if incomplete (takes priority)
  - GATES.md if workflow is complete/absent and phase work is done

Examples:
  fest workflow status              # Show workflow or gate progress
  fest workflow status --phase 001_INGEST  # Show specific phase
  fest workflow advance             # Complete current step and move to next
  fest workflow skip --reason "already completed externally" # Operator override
  fest workflow approve             # Approve a blocking checkpoint
  fest workflow reject              # Reject checkpoint with feedback
  fest workflow reset               # Reset workflow or gate to step 1
  fest workflow show                # Display the current step details`,
	}

	// Add persistent --phase flag for all subcommands
	cmd.PersistentFlags().StringVar(&phaseFlag, "phase", "", "specify phase directory (e.g., 001_INGEST)")

	// Add subcommands
	cmd.AddCommand(
		newStatusCmd(),
		newAdvanceCmd(),
		newSkipCmd(),
		newApproveCmd(),
		newRejectCmd(),
		newResetCmd(),
		newShowCmd(),
		// Standalone workflow lifecycle (introduced in WW0001/004.01).
		newInitCmd(),
		newStartCmd(),
		newRunsCmd(),
		newRenumberCmd(),
	)

	return cmd
}

// newStatusCmd is defined in status.go

// newAdvanceCmd is defined in advance.go

// newSkipCmd is defined in skip.go

// newApproveCmd is defined in approve.go

// newRejectCmd is defined in reject.go

// newResetCmd is defined in reset.go

// newShowCmd is defined in show.go
