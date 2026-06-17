package workflow

import (
	"github.com/Obedience-Corp/fest/internal/commands/festival"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// newCreateCmd registers `fest workflow create`, an alias for `fest create
// workflow` that delegates to the same festival.RunCreateWorkflow.
func newCreateCmd() *cobra.Command {
	opts := &festival.CreateWorkflowOptions{}
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Scaffold a new standalone WORKFLOW.md (alias of 'fest create workflow')",
		Long: `Alias of 'fest create workflow'.

` + createAliasIntro + `

Unlike the other 'fest workflow' subcommands (init, start, show, advance), which
operate on an existing WORKFLOW.md, this one scaffolds a brand-new document.

Examples:
  fest workflow create demo
  fest workflow create demo --steps '{"title":"Review","steps":[...]}'
  fest workflow create demo --steps-file steps.json`,
		Annotations: map[string]string{"scope": string(scope.Global)},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject the inherited --phase persistent flag rather than ignore it.
			if phaseFlag != "" {
				return festerrors.Validation("--phase is not supported by 'fest workflow create'").
					WithHint("scaffold in the current directory, or use 'fest create workflow --path <phase>' for phase mode")
			}
			if len(args) == 1 {
				opts.Name = args[0]
			}
			return festival.RunCreateWorkflow(cmd.Context(), opts)
		},
	}
	festival.BindCreateWorkflowFlags(cmd, opts)
	return cmd
}

const createAliasIntro = `Outside a festival phase, this creates WORKFLOW.md in the current directory,
initializes .workflow/ runtime state, and starts a tracked run so 'fest next'
works immediately.`
