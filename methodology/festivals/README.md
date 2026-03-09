# Festival Methodology - AI Agent Guide

## Quick Start

The `fest` CLI teaches the methodology and manages all festival operations:

```bash
fest understand              # Learn core methodology
fest understand methodology  # Deep dive into principles
fest understand structure    # See festival structure and scaffolds
fest understand tasks        # Task vs goal distinction (common mistake)
fest types festival          # Discover festival types
fest types festival show <type>  # See details for a specific type
```

## First Steps

### Step 1: Learn the Methodology

```bash
fest understand
```

### Step 2: Understand the Structure

```bash
fest understand structure
```

### Step 3 (Optional): Browse Methodology Resources

The `.festival/` directory contains templates, extensions, and configuration. See `.festival/README.md` for details. You do not need to read these — the `fest` CLI handles scaffolding automatically.

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
├── .festival/      # Methodology resources (templates, extensions, config)
└── README.md       # This file
```

## Festival Types

Choose the right type for your work:

| Type | When to Use | Creates |
|------|------------|---------|
| **standard** | Most projects, including the beginner path | INGEST, PLAN phases |
| **implementation** | Requirements already defined; not the first-run default | IMPLEMENT phase |
| **research** | Investigation or exploration | INGEST, RESEARCH, SYNTHESIZE phases |
| **ritual** | Recurring processes | Custom structure |

For a first festival, use `--type standard` explicitly. Choose `implementation` only when the requirements are already defined and you only need execution scaffolding.

```bash
fest create festival --name "my-project" --type standard
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
fest create festival --name "my-project" --type standard
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
2. **Create the festival** (`fest create festival --name "<name>" --type <type>`)
3. **Review scaffolded documents** (FESTIVAL_OVERVIEW.md, FESTIVAL_RULES.md, FESTIVAL_GOAL.md are auto-created)
4. **Add phases as needed** with appropriate types
5. **Create sequences and tasks** within implementation phases
6. **Execute with `fest next`** and mark tasks done with `fest task completed`

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
