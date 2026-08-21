# Festival Methodology - AI Agent Guide

## Quick Start

The `fest` CLI teaches the methodology and manages all festival operations. Start with the tasks-vs-goals warning, then learn the structure:

```bash
fest understand tasks         # Task vs goal distinction (read this first, it is the common mistake)
fest understand               # Learn core methodology
fest understand methodology   # Deep dive into principles
fest understand structure     # See festival structure and scaffolds
fest understand loop          # How fest next drives execution
fest types festival           # Discover festival types
fest types festival show <type>  # See details for a specific type
```

The `.festival/` directory holds templates, extensions, and configuration; see `.festival/README.md`. Read templates just-in-time, only when you reach the step that needs them: this preserves context window for actual work.

## Directory Structure

```
festivals/
├── planning/       # Festivals being planned and designed
├── ready/          # Festivals ready for execution
├── active/         # Currently executing
├── ritual/         # Recurring/repeatable festivals
├── chains/         # Chain YAML definitions (fest chain create), if any exist
├── .dungeon/       # Terminal statuses (hidden by default)
│   ├── completed/  # Successfully finished festivals
│   ├── archived/   # Preserved for reference
│   └── someday/    # May revisit later
├── .festival/      # Methodology resources (templates, extensions, config)
└── README.md       # This file
```

Each festival directory also carries a `.fest/` runtime folder (`progress_events.jsonl`, `status_history.json`) once execution starts. It is machine state, not something to read directly; use `fest status`, `fest workflow status`, or `fest show`.

## Festival Mental Model

A festival is a nested execution plan:

```text
festival -> phase -> sequence -> task
```

- A **festival** is one goal-oriented body of work.
- A **phase** is one stage of that work: ingest, planning, implementation, and so on.
- A **sequence** is an ordered slice of implementation work inside a phase.
- A **task** is the smallest executable unit an agent should pick up.

Not every phase behaves like implementation:

- **Workflow phases** (`ingest`, `planning`, `research`) are driven by `WORKFLOW.md` instructions and approval checkpoints.
- **Implementation phases** are driven by numbered sequences and task files.

`fest next` returns different things depending on where you are: in a workflow phase it returns the next workflow step (read inputs, write outputs, submit for approval); in an implementation phase it returns a concrete task file with festival, phase, and sequence context inline.

## Festival Types

| Type | When to Use | Creates |
|------|------------|---------|
| **standard** | Most projects, including the beginner path | INGEST, PLAN phases |
| **implementation** | Requirements already defined; not the first-run default | IMPLEMENT phase |
| **research** | Investigation or exploration | INGEST, RESEARCH, SYNTHESIZE phases |
| **ritual** | Recurring processes | Custom structure |

```bash
fest create festival --name "my-project" --type standard
```

New festivals are always created in `planning/` (or `ritual/` for ritual type); `--dest active` is rejected. Use `fest promote` to advance a festival through the lifecycle.

## Phase Types

| Phase Type | Template Files | Purpose |
|-----------|-----------------|---------|
| **planning** | GOAL.md, GATES.md, WORKFLOW.md, inputs/, decisions/ | Design, requirements |
| **implementation** | GOAL.md, GATES.md + numbered sequences with task files | Building features |
| **ingest** | GOAL.md, GATES.md, WORKFLOW.md, input_specs/, output_specs/ | Absorbing inputs |
| **research** | GOAL.md, GATES.md, WORKFLOW.md, sources/, findings/, analysis.md | Investigation |
| **review** | GOAL.md, GATES.md | Validation |
| **non_coding_action** | GOAL.md, GATES.md | Non-code work |

```bash
fest create phase --name "001_RESEARCH" --type research
fest create phase --name "002_IMPLEMENT" --type implementation
```

Numeric prefixes in `--name` are stripped and renumbered by position; use `--after` to control placement.

## The Two Execution Loops

**Workflow phases** (ingest, planning, research):

```bash
fest next               # Shows the current WORKFLOW.md step
# ...do the work...
fest workflow advance   # Complete the step, move to the next
# When a step has a checkpoint:
fest workflow approve   # Approve and proceed
fest workflow reject --reason "..."  # Reject with feedback
fest workflow judge     # Re-run the configured approval judge after a rejection
```

**Implementation phases** (numbered sequences and tasks):

```bash
fest next                        # Get the next task, with full context inline
# ...do the work...
fest task completed --yes        # Mark it done (agents must pass --yes; --json also requires --yes)
fest task blocked --reason "..." --yes  # Mark it blocked instead
```

`fest task completed`, `blocked`, and `reset` prompt for confirmation by default; `--yes` skips the prompt for non-interactive or agent use. `fest task update <percent>` and `fest task unblock` are frictionless progress signals and never prompt.

Other useful checks:

```bash
fest status     # Check progress
fest validate   # Validate structure (--fix applies safe automatic fixes)
```

## Standalone Workflows

Not every step-by-step loop needs a full festival. `fest create workflow <name>` scaffolds a standalone `WORKFLOW.md` with `.workflow/` runtime state anywhere in a project, and `fest next` works immediately. See `fest understand loop` for the simplest way in, and `fest workflow start|runs|reset|status|show` for managing runs.

## Committing Work

```bash
fest commit -m "..."
```

Run inside a festival directory, a linked project (`fest link`), or with `--festival <id>`. It stages changes, prepends the festival/task reference to the message, and creates up to two commits: the project commit (if linked) and a campaign-root commit scoped to festival-scoped files (never `git add -A` at the root). Every implementation sequence gets a `fest_commit` gate task that expects this.

## Lifecycle and Promotion

```bash
fest promote                 # Advance the current festival: planning -> ready -> active -> completed
fest promote my-feature      # Advance a festival by name from elsewhere
fest promote --dungeon someday    # Shelve for later
fest promote --dungeon archived   # Archive
fest promote --dungeon completed  # Mark completed (skips task validation)
```

| Directory | Purpose |
|-----------|---------|
| `planning/` | Festivals being designed |
| `ready/` | Planned and ready for execution |
| `active/` | Currently executing |
| `ritual/` | Recurring/repeatable festivals |
| `.dungeon/completed/` | Successfully finished |
| `.dungeon/archived/` | Preserved for reference |
| `.dungeon/someday/` | May revisit later |

## Navigation and Utilities

```bash
eval "$(fest shell-init zsh)"   # One-time: adds fgo (navigate) and fls (list) shell helpers
fgo                              # Navigate to festivals root
fgo 2/1                          # Navigate to phase 002, sequence 01 (fuzzy match works too)
fest go --print                  # Bare path, for scripts

fest link /path/to/project       # Link the current festival to a project directory
fest links                       # List all festival-project links

fest list                        # Active, ready, planning, ritual festivals
fest list all                    # Every festival, grouped by status

fest gates show                  # Effective gate policy for the current festival
fest gates apply --approve       # Apply configured quality gates to sequences

fest chain create --name "..."   # Chain festivals with dependencies (writes to festivals/chains/)
```

## Creating Your Festival, Step by Step

1. Choose a type (`fest types festival`) and create the festival (`fest create festival --name "<name>" --type <type>`).
2. Fill the `[REPLACE:...]` markers in the auto-created documents (FESTIVAL_OVERVIEW.md, FESTIVAL_RULES.md, FESTIVAL_GOAL.md), then run `fest validate` before your first `fest next`.
3. Add phases, sequences, and tasks as needed.
4. Execute with `fest next`, following the workflow or task loop above, and commit with `fest commit`.

## Reference: fest.yaml Configuration

Quality gates are configured in `fest.yaml` at the festival root:

```yaml
quality_gates:
    enabled: true
    auto_append: true
    implementation:
        - id: testing
          template: gates/implementation/QUALITY_GATE_TESTING
          name: Testing and Verification
          enabled: true
        - id: fest-commit
          template: gates/implementation/QUALITY_GATE_FEST_COMMIT
          name: Fest Commit Changes
          enabled: true
        # ...plus review and iterate gates by default
excluded_patterns:
    - '*_planning'
    - '*_research'
    - '*_requirements'
    - '*_docs'
    # ...12 patterns ship by default; see 'fest gates show'
templates:
    task_default: tasks/SIMPLE
    prefer_simple: true
```

`fest gates show` prints the effective policy for a live festival, including the built-in `commit` gate alongside the ones configured above.

## Reference: JSON Output

Most `fest` commands support `--json` for machine-readable output:

```bash
fest create festival --name "my-project" --type standard --json
```

```json
{
  "ok": true,
  "action": "create_festival",
  "festival": {
    "dest": "planning",
    "directory": "my-project-MP0001",
    "id": "MP0001",
    "name": "my-project",
    "type": "standard"
  },
  "created_path": "festivals/planning/my-project-MP0001",
  "auto_phases_created": ["001_INGEST", "002_PLAN"],
  "markers_total": 37
}
```

```bash
fest validate --json
```

```json
{
  "ok": true,
  "action": "validate",
  "festival": "my-project-MP0001",
  "valid": true,
  "score": 100
}
```

`issues` is only present when validation finds problems.

---

**For Agents**: Use `fest understand` and `fest next` as your primary tools. Read documentation just-in-time, not upfront.
