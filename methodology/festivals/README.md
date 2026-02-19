# Festival Methodology - AI Agent Guide

## Quick Start

The fastest way to learn the methodology is through the `fest` CLI:

```bash
fest understand              # Learn core methodology
fest understand methodology  # Deep dive into principles
fest understand structure    # See festival structure and scaffolds
fest types festival          # Discover festival types
fest types festival show <type>  # See details for a specific type
```

## Mandatory First Steps

### Step 1: Verify Methodology Resources

```bash
ls -la .festival/
```

### Step 2: Read the Implementation Guide

Read `.festival/README.md` for navigation and resource overview.

### Step 3: Understand Core Methodology

Read `.festival/FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT.md` for the full methodology spec.

## Context Preservation Rules

**DO NOT READ TEMPLATES UNTIL YOU NEED THEM.** Templates are in `.festival/templates/` but should ONLY be read when you reach the specific step requiring them. This preserves context window for actual work.

## Directory Structure

```
festivals/
├── planning/       # Festivals being planned and designed
├── ready/          # Festivals ready for execution
├── active/         # Currently executing festivals
├── ritual/         # Recurring/repeatable festivals
├── dungeon/        # Archived/deprioritized work
│   ├── completed/  # Successfully finished festivals
│   ├── archived/   # Preserved for reference
│   └── someday/    # May revisit later
├── .festival/      # Methodology resources (read just-in-time)
└── README.md       # This file
```

## Festival Types

Choose the right type for your work:

| Type | When to Use | Creates |
|------|------------|---------|
| **standard** | Most projects (planning + implementation) | INGEST, PLAN phases |
| **implementation** | Requirements already defined | IMPLEMENT phase |
| **research** | Investigation or exploration | INGEST, RESEARCH, SYNTHESIZE phases |
| **ritual** | Recurring processes | Custom structure |

```bash
fest create festival --type standard "my-project"
```

## Phase Types

Every phase has a type that determines its structure:

| Phase Type | Structure | Purpose |
|-----------|-----------|---------|
| **planning** | inputs/, WORKFLOW.md, decisions/ | Design, requirements |
| **implementation** | Numbered sequences + task files | Building features |
| **research** | Sequences with investigation tasks | Investigation |
| **review** | Sequences with verification tasks | Validation |
| **ingest** | Sequences with ingestion tasks | Absorbing inputs |
| **non_coding_action** | Sequences with action tasks | Non-code work |

```bash
fest create phase --name "001_RESEARCH" --type research
fest create phase --name "002_IMPLEMENT" --type implementation
```

**Key rule**: Planning phases use `inputs/` and workflow files. Implementation phases use numbered sequences with task files.

## Working with Festivals

### Creating a Festival

```bash
fest create festival --type standard "my-project"
```

### Executing a Festival

```bash
fest next            # Get the next task to work on
fest task completed  # Mark current task as done
fest status          # Check progress
fest validate        # Validate structure
```

### Creating Structure

```bash
fest create phase --name "001_IMPLEMENT" --type implementation
fest create sequence --name "01_backend_api"
fest create task --name "01_setup_database"
```

## Creating Your Festival - Step by Step

1. **Choose festival type** based on your work (`fest types festival`)
2. **Create the festival** (`fest create festival --type <type> <name>`)
3. **Create core documents** (FESTIVAL_OVERVIEW.md, FESTIVAL_RULES.md from templates)
4. **Add phases as needed** with appropriate types
5. **Create sequences and tasks** within implementation phases
6. **Execute with `fest next`** and mark tasks done with `fest task completed`

## Template Reading Strategy

Read templates ONE AT A TIME as you need them:

| When Creating | Read Template |
|--------------|---------------|
| Festival overview | `.festival/templates/FESTIVAL_OVERVIEW_TEMPLATE.md` |
| Festival goals | `.festival/templates/FESTIVAL_GOAL_TEMPLATE.md` |
| Rules/standards | `.festival/templates/FESTIVAL_RULES_TEMPLATE.md` |
| Phase goals | `.festival/templates/PHASE_GOAL_TEMPLATE.md` |
| Sequence goals | `.festival/templates/SEQUENCE_GOAL_TEMPLATE.md` |
| Tasks | `.festival/templates/TASK_TEMPLATE.md` |
| Progress tracking | `.festival/templates/FESTIVAL_TODO_TEMPLATE.md` |

## Lifecycle Directories

| Directory | Purpose |
|-----------|---------|
| `planning/` | Festivals being designed |
| `ready/` | Planned and ready for execution |
| `active/` | Currently executing |
| `ritual/` | Recurring/repeatable festivals |
| `dungeon/completed/` | Successfully finished |
| `dungeon/archived/` | Preserved for reference |
| `dungeon/someday/` | May revisit later |

---

**For Agents**: Use `fest understand` and `fest next` as your primary tools. Read documentation just-in-time, not upfront.
