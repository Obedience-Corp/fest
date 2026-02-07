// Package task provides the fest task command group for managing task status.
package task

import (
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewTaskCommand creates the task command group.
func NewTaskCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage task status (show, edit, complete, block, reset)",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Commands for managing individual task status within a festival.

These commands provide a simpler interface for viewing, editing, marking
tasks complete, blocked, or resetting them. Each mutation requires
interactive confirmation to ensure agents verify their work before proceeding.

Task Resolution:
  When [task] is omitted, the command auto-detects the current task:
    1. Finds the current in_progress task from the progress store
    2. Falls back to the next pending task (same logic as 'fest next')
    3. Errors if neither is found

  When [task] is provided, it resolves via:
    - Full relative path: 002_FOUNDATION/01_scaffold/01_design.md
    - Bare filename: 01_design.md (searches for unique match)

Examples:
  fest task show                          # Show current task details
  fest task show 01_design.md             # Show specific task
  fest task edit                          # Open current task in editor
  fest task completed                     # Mark current task complete (Y/n)
  fest task blocked --reason "need API"   # Mark task blocked (Y/n)
  fest task reset                         # Reset task to pending (Y/n)`,
	}

	cmd.AddCommand(
		newShowCmd(),
		newEditCmd(),
		newCompletedCmd(),
		newBlockedCmd(),
		newResetCmd(),
	)

	return cmd
}
