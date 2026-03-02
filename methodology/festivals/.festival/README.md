# .festival/ — Methodology Resources

This directory contains templates, extensions, and configuration used by the `fest` CLI to scaffold and manage festivals.

## Learning the Methodology

Use the `fest` CLI — it teaches the methodology directly:

```bash
fest understand              # Overview
fest understand methodology  # Core principles
fest understand structure    # Three-level hierarchy
fest understand tasks        # Task vs goal distinction
fest understand rules        # Mandatory naming/structure rules
fest understand templates    # Template variable system
fest understand gates        # Quality gates
```

Run `fest --help` for the full command reference.

## What's in This Directory

```
.festival/
├── README.md                                  # This file
├── FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT.md    # Core methodology (override source)
├── festival_types.yaml                        # Festival type definitions
├── templates/                                 # Scaffold templates
│   ├── festival/                              # Festival-level (GOAL, OVERVIEW, RULES, TODO)
│   ├── phases/                                # Phase-level by type
│   │   ├── planning/                          # GOAL, WORKFLOW, GATES, inputs/, decisions/
│   │   ├── implementation/                    # GOAL, GATES, quality gates
│   │   ├── research/                          # GOAL, WORKFLOW, GATES, findings/, sources/
│   │   ├── ingest/                            # GOAL, WORKFLOW, GATES, input_specs/, output_specs/
│   │   ├── review/                            # GOAL, GATES
│   │   ├── non_coding_action/                 # GOAL, GATES
│   │   └── deployment/                        # GATES
│   ├── sequences/                             # GOAL, GOAL_MINIMAL
│   └── tasks/                                 # TASK
├── examples/                                  # Reference examples
│   └── TASK_EXAMPLES.md                       # 15+ concrete task examples
└── extensions/                                # Optional extensions
    ├── interface-planning/                    # Multi-system interface coordination
    └── orchestration/                         # Multi-agent orchestration patterns
```

## Templates

Templates are used by `fest create` commands to scaffold festival structure. You don't need to read them — the CLI renders them with the correct variables when you create festivals, phases, sequences, and tasks.

If you need to customize the default scaffolding, edit the templates here. Changes apply to all future `fest create` operations in this workspace.

## Extensions

Extensions add optional capabilities discovered by `fest understand extensions`:

- **interface-planning** — Templates for defining interfaces between systems before implementation. Useful for multi-service projects.
- **orchestration** — Patterns and templates for multi-agent workflows. Includes agent archetypes and orchestration plan templates.

## How This Directory is Managed

- `fest system sync` distributes the methodology source to workspaces
- `fest system update` updates this directory from the methodology source, identifying new, changed, and orphaned files
- You can customize templates locally — updates will flag conflicts rather than overwrite

## Key Files

| File | Purpose |
|------|---------|
| `FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT.md` | Core methodology principles. Used by `fest understand methodology` as an optional override source. |
| `festival_types.yaml` | Defines available festival types. Used by `fest types festival`. |
