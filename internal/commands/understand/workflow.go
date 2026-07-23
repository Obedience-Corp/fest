package understand

import (
	"fmt"
	"path/filepath"

	understanddocs "github.com/Obedience-Corp/fest/embedded/docs/understand"
	"github.com/spf13/cobra"
)

func newUnderstandWorkflowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "workflow",
		Short: "Just-in-time reading plus workflow/gate execution",
		Long: `Learn the just-in-time approach to reading templates and documentation,
preserving context for actual work, and how to use 'fest workflow' for
WORKFLOW.md phases and GATES.md phase gates.`,
		Run: func(cmd *cobra.Command, args []string) {
			printWorkflow(findDotFestivalDir())
		},
	}
}

func newUnderstandHooksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hooks",
		Short: "Lifecycle hooks schema, bindings, and human gates",
		Long: `Learn how fest resolves hooks across machine, festivals, and festival
layers; how step bindings fire; legacy approval_judge alias behavior; and
non-bypassable human gates (approval: human-required).

See also docs/concepts/hooks.md and docs/concepts/hook-evidence-contract.md.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(understanddocs.Load("hooks.txt"))
		},
	}
}

func newUnderstandRulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rules",
		Short: "MANDATORY structure rules for automation",
		Long:  `Learn the RIGID structure requirements that enable Festival automation: naming conventions, required files, quality gates, and parallel execution.`,
		Run: func(cmd *cobra.Command, args []string) {
			printRules(findDotFestivalDir())
		},
	}
}

func newUnderstandTemplatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "templates",
		Short: "Template variables that save tokens",
		Long:  `Learn how to pass variables to fest create commands to generate pre-filled documents, minimizing post-creation editing and saving tokens.`,
		Run: func(cmd *cobra.Command, args []string) {
			printTemplates()
		},
	}
}

func newUnderstandChecklistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "checklist",
		Short: "Quick festival validation checklist",
		Long: `Show a quick checklist for validating your festival structure.

This is a quick reference. For full validation, run 'fest validate checklist'.

Checklist:
  1. FESTIVAL_OVERVIEW.md exists and is filled out
  2. Each phase has PHASE_GOAL.md
  3. Each sequence has SEQUENCE_GOAL.md
  4. Implementation sequences have TASK FILES (not just goals!)
  5. Quality gates present in implementation sequences
  6. No unfilled template markers ([FILL:], {{ }})`,
		Run: func(cmd *cobra.Command, args []string) {
			printChecklist()
		},
	}
}

func printWorkflow(dotFestival string) {
	// Hybrid: try .festival/ supplements first, fall back to embedded defaults
	if dotFestival != "" {
		readmePath := filepath.Join(dotFestival, "README.md")
		hasContent := false

		// Check if .festival/ has workflow content
		if content := extractSection(readmePath, "When to Read What", "### Never Do This"); content != "" {
			fmt.Print("\nFestival Workflow - Just-in-Time Reading\n")
			fmt.Println("========================================")
			fmt.Println("\nThe just-in-time approach preserves context window for actual work.")
			fmt.Println("\nWhen to Read What (from .festival/)")
			fmt.Println("------------------------------------")
			fmt.Println(content)
			hasContent = true

			if never := extractSection(readmePath, "### Never Do This", "### Always Do This"); never != "" {
				fmt.Println("\nNEVER Do This")
				fmt.Println("-------------")
				fmt.Println(never)
			}

			if always := extractSection(readmePath, "### Always Do This", "## Quick Navigation"); always != "" {
				fmt.Println("\nALWAYS Do This")
				fmt.Println("--------------")
				fmt.Println(always)
			}
		}

		if hasContent {
			fmt.Printf("\nSource: %s\n", dotFestival)
			return
		}
	}

	// Default: use embedded content
	fmt.Print("\n")
	fmt.Print(understanddocs.Load("workflow.txt"))
}

func printRules(dotFestival string) {
	// Hybrid: try festival-specific FESTIVAL_RULES.md first
	rulesPath := findFestivalRulesFile(dotFestival)
	if rulesPath != "" {
		content := readFileContent(rulesPath)
		if content != "" && !hasSignificantUnfilledMarkers(content) {
			fmt.Print("\n")
			fmt.Print(content)
			fmt.Printf("\n---\nSource: %s\n", rulesPath)
			return
		}
	}

	// Default: use embedded content
	fmt.Print("\n")
	fmt.Print(understanddocs.Load("rules.txt"))
	fmt.Print("\n---\nSource: [EMBEDDED DEFAULT]\n")
}

func printTemplates() {
	fmt.Print("\n")
	fmt.Print(understanddocs.Load("templates.txt"))
}

func newUnderstandContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "context",
		Short: "CONTEXT.md - session memory for AI agents (CREATE FIRST)",
		Long: `Learn about CONTEXT.md - the "memory" document that preserves decisions,
blockers, and handoff notes between AI sessions.

CREATE CONTEXT.md FIRST when planning a new festival. It captures WHY
the festival exists and prevents agents from losing focus on purpose.`,
		Run: func(cmd *cobra.Command, args []string) {
			printContext()
		},
	}
}

func printContext() {
	fmt.Print("\n")
	fmt.Print(understanddocs.Load("context.txt"))
}

func newUnderstandNodeIDsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nodeids",
		Short: "Node reference system for code traceability",
		Long: `Learn about the node reference system for tracing code changes back to
specific festival tasks.

Node references like GU0001:P002.S01.T03 create a clear audit trail
connecting code comments to planning documents.`,
		Run: func(cmd *cobra.Command, args []string) {
			printNodeIDs()
		},
	}
}

func printNodeIDs() {
	fmt.Print("\n")
	fmt.Print(understanddocs.Load("nodeids.txt"))
}

func printChecklist() {
	fmt.Print(`
Festival Validation Checklist
=============================

Before executing a festival, verify:

  ✓ Festival Level
    □ FESTIVAL_OVERVIEW.md exists and defines goals
    □ FESTIVAL_RULES.md exists with quality standards

  ✓ Phase Level
    □ Each phase has PHASE_GOAL.md
    □ Phases are numbered correctly (001_, 002_, ...)
    □ Phase types are correctly assigned

  ✓ Workflow Phases
    □ Workflow phases (planning, research, ingest) have WORKFLOW.md
    □ Required subdirectories present (inputs/, decisions/, sources/, etc.)
    □ No numbered sequences in workflow phases

  ✓ Sequence Level
    □ Each implementation sequence has SEQUENCE_GOAL.md
    □ Sequences are numbered correctly (01_, 02_, ...)

  ✗ CRITICAL: Task Files
    □ Implementation sequences have TASK FILES
    □ Not just SEQUENCE_GOAL.md - actual task files!
    □ Goals define WHAT; tasks define HOW
    □ AI agents execute TASK FILES

  ✓ Quality Gates
    □ Implementation sequences end with quality gates
    □ XX_testing.md
    □ XX_review.md
    □ XX_iterate.md
    □ XX_commit.md

  ✓ Templates
    □ No [FILL:] markers remaining
    □ No {{ }} template syntax in final docs


Quick Validation Commands
-------------------------

  fest validate                # Full validation
  fest validate tasks          # Check task files exist
  fest validate checklist      # Detailed checklist with auto-checks

`)
}
