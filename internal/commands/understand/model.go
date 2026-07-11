package understand

import (
	"fmt"

	understanddocs "github.com/Obedience-Corp/fest/embedded/docs/understand"
	"github.com/spf13/cobra"
)

func newUnderstandLifecycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "lifecycle",
		Short:      "Festival lifecycle: planning -> ready -> active -> dungeon",
		Long:       `How festivals move through lifecycle directories, and how fest promote advances them.`,
		Hidden:     true,
		Deprecated: "unused; run 'fest understand' for the current methodology overview",
		Run:        func(cmd *cobra.Command, args []string) { printLifecycle() },
	}
}

func printLifecycle() { fmt.Print(understanddocs.Load("lifecycle.txt")) }

func newUnderstandRolesCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "roles",
		Short:      "Human vs agent responsibilities and the planning/implementation boundary",
		Long:       `Who does what in the human-AI collaboration, and the rule that implementation steps only follow defined requirements.`,
		Hidden:     true,
		Deprecated: "unused; run 'fest understand' for the current methodology overview",
		Run:        func(cmd *cobra.Command, args []string) { printRoles() },
	}
}

func printRoles() { fmt.Print(understanddocs.Load("roles.txt")) }

func newUnderstandPlanningCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "planning",
		Short:      "The planning process: turning a goal into a structured plan",
		Long:       `How a goal becomes a festival: define, break down, organize into phases, plus common phase patterns.`,
		Hidden:     true,
		Deprecated: "unused; run 'fest understand' for the current methodology overview",
		Run:        func(cmd *cobra.Command, args []string) { printPlanning() },
	}
}

func printPlanning() { fmt.Print(understanddocs.Load("planning.txt")) }

func newUnderstandChainsCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "chains",
		Short:      "Chains: dependencies between festivals",
		Long:       `Link festivals so a downstream festival waits for its upstream dependencies, via fest chain.`,
		Hidden:     true,
		Deprecated: "unused; run 'fest understand' for the current methodology overview",
		Run:        func(cmd *cobra.Command, args []string) { printChains() },
	}
}

func printChains() { fmt.Print(understanddocs.Load("chains.txt")) }

func newUnderstandRitualsCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "rituals",
		Short:      "Ritual festivals: reusable templates for recurring work",
		Long:       `Define recurring work once as a ritual, then spin up fresh runs with fest ritual run.`,
		Hidden:     true,
		Deprecated: "unused; run 'fest understand' for the current methodology overview",
		Run:        func(cmd *cobra.Command, args []string) { printRituals() },
	}
}

func printRituals() { fmt.Print(understanddocs.Load("rituals.txt")) }
