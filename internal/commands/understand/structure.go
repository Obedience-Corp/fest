package understand

import (
	"fmt"

	understanddocs "github.com/Obedience-Corp/fest/embedded/docs/understand"
	"github.com/spf13/cobra"
)

func newUnderstandStructureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "structure",
		Short: "3-level hierarchy: Festival → Phase → Sequence → Task",
		Long: `Understand the Festival Methodology structure.

HIERARCHY:
  Festival    - A complete project with a goal
  └─ Phase    - Major milestone (001_PLANNING, 002_IMPLEMENTATION)
     └─ Sequence - Related tasks grouped together
        └─ Task   - Individual executable work item

Includes visual scaffold examples for simple, standard, and complex festivals.`,
		Run: func(cmd *cobra.Command, args []string) {
			printStructure(findDotFestivalDir())
		},
	}
}

func printStructure(dotFestival string) {
	fmt.Print(`
Festival Structure - Three-Level Hierarchy
==========================================

Festival Methodology uses a three-level hierarchy:

  FESTIVAL (the project)
    └── PHASE (major stage of work)
          └── SEQUENCE (group of related tasks)
                └── TASK (atomic unit of work)

`)

	// Show scaffold trees
	fmt.Print(`

Scaffold: Simple Festival
-------------------------

`)
	printScaffoldTree("simple")

	fmt.Print(`

Scaffold: Standard Festival with Quality Gates
----------------------------------------------

`)
	printScaffoldTree("standard")

	fmt.Print(`

Scaffold: Complex Multi-Phase Festival
--------------------------------------

`)
	printScaffoldTree("complex")

	fmt.Print(`

Phase Types
-----------

Every phase has a type that determines its structure:

  WORKFLOW phases (use WORKFLOW.md, no sequences):
    planning           WORKFLOW.md, inputs/, decisions/, plan/
    research           WORKFLOW.md, sources/, findings/, analysis templates
    ingest             WORKFLOW.md, input_specs/, output_specs/

  SEQUENCE phases (use numbered sequences + task files):
    implementation     Numbered sequences + numbered task files + quality gates

  FREEFORM phases (PHASE_GOAL.md only):
    review             Review criteria and stakeholder sign-off
    non_coding_action  Action items and verification steps

Create: fest create phase --name "001_IMPLEMENT" --type implementation

Parallel Numbering
------------------

Nodes with the same number can run in parallel at any level:

  002_FRONTEND/ + 002_BACKEND/      Parallel phases
  01_auth/ + 01_payments/           Parallel sequences
  01_frontend.md + 01_backend.md    Parallel tasks

Festival Lifecycle Directories
------------------------------

  festivals/
    planning/       Festivals being planned
    ready/          Ready for execution
    active/         Currently executing
    ritual/         Recurring festivals
    dungeon/        Final/non-active states (completed/, archived/, someday/)
`)

	fmt.Print(`

Naming Conventions (MANDATORY)
------------------------------

  Phases:     NNN_PHASE_NAME      3-digit, UPPERCASE
  Sequences:  NN_sequence_name    2-digit, lowercase
  Tasks:      NN_task_name.md     2-digit, lowercase, .md extension

Parallel Execution
------------------

Tasks with the same number execute in parallel:

  01_frontend_setup.md  ┐
  01_backend_setup.md   ├── Run simultaneously
  01_database_setup.md  ┘
  02_integration.md     ← Waits for all 01_ tasks

For detailed requirements: fest understand rules
For template variables:    fest understand templates
`)

	if dotFestival != "" {
		fmt.Printf("\nSource: %s\n", dotFestival)
	}
}

func printScaffoldTree(variant string) {
	fmt.Print(understanddocs.LoadScaffold(variant))
}
