package understand

import (
	"fmt"

	understanddocs "github.com/Obedience-Corp/fest/embedded/docs/understand"
	"github.com/spf13/cobra"
)

func newUnderstandLoopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "loop",
		Short: "The fest next loop: festivals and standalone WORKFLOW.md as work loops",
		Long: `Understand how execution works: the fest next loop.

fest next is the engine that walks an agent through a festival (or a standalone
WORKFLOW.md) one step at a time, with full context inline. Covers the loop, the
simplest standalone entry point (fest create workflow, anywhere, with run
tracking), and when to reach for a full festival instead.`,
		Run: func(cmd *cobra.Command, args []string) {
			printLoop()
		},
	}
}

func printLoop() {
	fmt.Print(understanddocs.Load("loop.txt"))
}
