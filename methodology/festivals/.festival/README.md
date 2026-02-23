# Festival Methodology Resources Guide

This directory contains all the resources needed to implement Festival Methodology in your projects. This guide helps you navigate and use these resources effectively.

## Step-Based Reading Strategy

**CRITICAL**: To preserve context window and focus on goal progression, follow these rules:

### When to Read What

| Resource              | Read When                                 |
| --------------------- | ----------------------------------------- |
| This README           | Immediately - provides navigation         |
| Core methodology docs | During initial understanding              |
| Templates             | ONLY when creating that specific document |
| Examples              | ONLY when stuck or need clarification     |
| Agents                | ONLY when using that specific agent       |

### Never Do This

- Reading all templates upfront "to understand them"
- Loading all agents at once

### Always Do This

- Read templates one at a time as needed
- Read examples only when stuck
- Keep templates closed after use
- Focus context on actual work, not documentation

## Quick Navigation

- **[Festival Types](#festival-types)** - Choose the right festival type for your work
- **[Phase Types](#phase-types)** - Understand the 6 phase types and their conventions
- **[Phase Adaptability](#phase-adaptability)** - How to customize phases for your project needs
- **[Templates](#templates)** - Document templates for creating festivals
- **[Agents](#ai-agents)** - Specialized AI agents for festival workflow
- **[Examples](#examples)** - Concrete examples and patterns
- **[Core Documentation](#core-documentation)** - Methodology principles and theory

## Festival Types

Festivals come in four types. Choose the type that matches your work:

| Festival Type | When to Use | Auto-Scaffolded Phases |
|--------------|------------|----------------------|
| **standard** (default) | Most projects needing planning + implementation | INGEST (ingest), PLAN (planning) |
| **implementation** | Requirements already exist, just need to execute | IMPLEMENT (implementation) |
| **research** | Investigation, auditing, or exploration work | INGEST (ingest), RESEARCH (research), SYNTHESIZE (planning) |
| **ritual** | Recurring/repeatable processes | Custom structure from ritual template |

```bash
fest create festival --type standard my-festival
fest create festival --type implementation my-feature
fest create festival --type research my-investigation
fest create festival --type ritual my-recurring-process
```

Use `fest types festival` to list types and `fest types festival show <type>` for details.

## Phase Types

Every phase has a **type** that determines its structural conventions. There are 6 phase types:

| Phase Type | Purpose | Structure |
|-----------|---------|-----------|
| **planning** | Design, architecture, requirements | Uses `inputs/`, workflow files (WORKFLOW.md). No numbered sequences. |
| **implementation** | Writing code, building features | Numbered sequences with task files. Quality gates auto-appended. |
| **research** | Investigation, exploration, auditing | WORKFLOW.md with `sources/`, `findings/`. No numbered sequences. |
| **review** | Code review, testing, validation | Freeform. PHASE_GOAL.md with review criteria. |
| **ingest** | Absorbing external content | WORKFLOW.md with `input_specs/`, `output_specs/`. No numbered sequences. |
| **non_coding_action** | Documentation, process changes | Freeform. PHASE_GOAL.md with action items. |

```bash
fest create phase --name "001_RESEARCH" --type research
fest create phase --name "002_IMPLEMENT" --type implementation
fest create phase --name "001_INGEST" --type ingest
```

### Planning Phase Structure

Planning phases use `inputs/` directories and workflow files instead of numbered sequences:

```
001_PLAN/
├── PHASE_GOAL.md
├── WORKFLOW.md            # Planning process
├── inputs/                # Reference materials
│   └── requirements.md
├── decisions/             # Captured decisions
└── plan/                  # Resulting plans
```

### Implementation Phase Structure

Implementation phases MUST have numbered sequences with task files:

```
002_IMPLEMENT/
├── PHASE_GOAL.md
├── 01_backend_foundation/
│   ├── 01_database_setup.md
│   ├── 02_api_endpoints.md
│   ├── 03_testing.md           ← Quality gate
│   ├── 04_review.md            ← Quality gate
│   └── 05_iterate.md           ← Quality gate
└── completed/
```

### Hybrid Phases

A phase can contain BOTH workflow files AND numbered sequences when needed:

```
001_PLAN/
├── WORKFLOW.md                    # Overall process
├── inputs/                        # Reference materials
├── 01_detailed_analysis/          # Structured analysis work
│   ├── 01_analyze_requirements.md
│   └── 02_document_findings.md
└── decisions/
```

## Phase Adaptability

**CRITICAL**: Festival phases are guidelines, not rigid requirements. Adapt the structure to match your actual work needs.

## Requirements-Driven Implementation

**MOST CRITICAL**: Implementation sequences can ONLY be created after requirements are defined. This is the core principle of Festival Methodology.

### When Implementation Sequences Can Be Created

**Create implementation sequences when:**

- Human provides specific requirements or specifications
- Planning phase has been completed with deliverables
- External planning documents define what to build
- Human explicitly requests implementation of specific functionality

**NEVER create implementation sequences when:**

- No requirements have been provided
- Planning phase hasn't been completed
- Guessing what might need to be implemented
- Making assumptions about functionality

### The Human-AI Collaboration Model

**Human provides:**

- Project goals and vision
- Requirements and specifications
- Architectural decisions
- Feedback and iteration guidance

**AI agent creates:**

- Structured sequences from requirements
- Detailed task specifications
- Implementation plans
- Progress tracking and documentation

### Common Phase Patterns

Phases are chosen based on need, not a rigid template:

**Implementation Only** (requirements already provided):
`001_IMPLEMENT`

**Research + Implementation**:
`001_RESEARCH → 002_IMPLEMENT`

**Standard with Planning**:
`001_INGEST → 002_PLAN → 003_IMPLEMENT`

**Full Lifecycle**:
`001_INGEST → 002_PLAN → 003_IMPLEMENT → 004_VALIDATE`

**Multiple Implementation Phases**:
`001_PLAN → 002_IMPLEMENT_CORE → 003_IMPLEMENT_FEATURES → 004_IMPLEMENT_UI`

**Research Festival**:
`001_INGEST → 002_RESEARCH → 003_SYNTHESIZE`

**Bug Fix or Enhancement**:
`001_ANALYZE → 002_IMPLEMENT`

### Phase Design Guidelines

**Good Phase:**

- Represents a distinct step toward goal achievement
- Has clear purpose matching its phase type
- Planning phases: Use inputs/ and workflow files
- Implementation phases: Must have sequences and tasks
- Added when needed, not pre-planned

**Bad Phase:**

- Created just to follow a pattern
- Planning phase with numbered sequences/tasks (use inputs/ instead)
- Single sequence worth of work
- Time-based rather than goal-based

### Sequence vs Task Decision

**Create a Sequence When:**

- You have 3+ related tasks
- Tasks must be done in order
- Tasks share common setup/teardown
- Work forms a logical unit

**Make it a Single Task When:**

- Work is atomic and self-contained
- No clear subtasks
- Can be completed in one session
- Doesn't benefit from breakdown

### Standard Quality Gates

**EVERY implementation sequence should end with quality gate tasks:**

```
XX_implementation_tasks.md
XX_testing.md
XX_review.md
XX_iterate.md
```

**Example Implementation Sequence:**

```
01_backend_api/
├── 01_create_user_endpoints.md
├── 02_add_authentication.md
├── 03_implement_validation.md
├── 04_testing.md              ← Quality gate
├── 05_review.md               ← Quality gate
└── 06_iterate.md              ← Quality gate
```

These quality gates ensure:

- Functionality works as specified
- Code meets project standards
- Issues are identified and resolved
- Knowledge is transferred through review

## Goal Files

Goal files provide clear objectives and evaluation criteria at every level of the festival hierarchy. They ensure each phase and sequence has a specific goal to work towards and can be evaluated upon completion.

### Goal File Hierarchy

```
festival/
├── FESTIVAL_GOAL.md          # Overall festival goals and success criteria
├── 001_PLAN/
│   ├── PHASE_GOAL.md         # Phase-specific goals
│   ├── 01_requirements/
│   │   └── SEQUENCE_GOAL.md  # Sequence-specific goals
│   └── 02_architecture/
│       └── SEQUENCE_GOAL.md
└── [continues for all phases and sequences]
```

### Goal Templates

1. **FESTIVAL_GOAL_TEMPLATE.md**
   - Comprehensive festival-level goals
   - Success criteria across all dimensions
   - KPIs and stakeholder metrics
   - Post-festival evaluation framework

2. **PHASE_GOAL_TEMPLATE.md**
   - Phase-specific objectives
   - Contribution to festival goal
   - Phase evaluation criteria
   - Lessons learned capture

3. **SEQUENCE_GOAL_TEMPLATE.md**
   - Sequence-level objectives
   - Task alignment verification
   - Progress tracking metrics
   - Post-completion assessment

### Using Goal Files

**When Planning Goal Progression:**

1. Create FESTIVAL_GOAL.md from template (overall goal achievement criteria)
2. Create PHASE_GOAL.md for each phase (step toward festival goal)
3. Create SEQUENCE_GOAL.md for each sequence (step toward phase goal)
4. Ensure alignment: Sequence goals → Phase goals → Festival goal achievement

**During Execution:**

- Track progress against goal metrics
- Update completion status
- Identify risks to goal achievement

**At Completion:**

- Evaluate goal achievement
- Document lessons learned
- Capture recommendations
- Get stakeholder sign-off

## Templates

Templates provide standardized structures for festival documentation. Each template includes inline examples and clear instructions.

### Essential Templates

1. **FESTIVAL_OVERVIEW_TEMPLATE.md**
   - Define project goals and success criteria
   - Create stakeholder matrix
   - Document problem statement
   - _Use this first when starting a new festival_

2. **Interface Planning Extension** (When Needed)
   - For multi-system projects requiring coordination
   - Define interfaces before implementation
   - Enables parallel development
   - _See extensions/interface-planning/ for templates_

3. **FESTIVAL_RULES_TEMPLATE.md**
   - Project-specific standards and guidelines
   - Quality gates and compliance requirements
   - Team agreements and conventions
   - _Customize for your project's needs_

4. **TASK_TEMPLATE.md**
   - Comprehensive task structure (full version)
   - Detailed implementation steps
   - Testing and verification sections
   - _Use for complex or critical tasks_

5. **FESTIVAL_TODO_TEMPLATE.md** (Markdown)
   - Human-readable progress tracking
   - Checkbox-based task management
   - Visual project status
   - _Use for manual tracking and documentation_

6. **FESTIVAL_TODO_TEMPLATE.yaml** (YAML)
   - Machine-readable progress tracking
   - Structured data for automation
   - CI/CD integration ready
   - _Use for automated tooling and reporting_

### When to Use Each Format

**Use Markdown (.md) when:**

- Working directly with AI agents
- Manual progress tracking
- Creating documentation
- Sharing with stakeholders

**Use YAML (.yaml) when:**

- Integrating with CI/CD pipelines
- Building automation tools
- Generating reports programmatically
- Need structured data parsing

## AI Agents

Specialized agents help maintain methodology consistency and guide festival execution.

### Available Agents

1. **festival_planning_agent.md**
   - Conducts structured project interviews
   - Creates complete festival structures
   - Ensures proper three-level hierarchy
   - _Trigger: Starting a new project or festival_

2. **festival_review_agent.md**
   - Validates festival structure compliance
   - Reviews quality gates
   - Ensures methodology adherence
   - _Trigger: Before moving phases or major milestones_

3. **festival_methodology_manager.md**
   - Enforces methodology during execution
   - Prevents process drift
   - Provides ongoing governance
   - _Trigger: During active development_

### Using Agents with Claude Code

```
You: Please use the festival planning agent to help me create a festival for [project description]

Claude: [Loads festival_planning_agent.md and conducts structured interview]
```

### Agent Collaboration Pattern

```
Planning → Review → Execution Management
    ↑         ↓           ↓
    └─────────────────────┘
         (Iterate)
```

## fest CLI Tool

Use the `fest` CLI for efficient festival management. It saves tokens and ensures correct structure.

```bash
# Learn the methodology
fest understand

# Discover festival and phase types
fest types festival
fest types festival show standard

# Create structure
fest create festival --type standard "my-project"
fest create phase --name "001_IMPLEMENT" --type implementation

# Navigate and execute
fest next                  # Get next task
fest task completed        # Mark current task done
fest status                # View festival progress
fest validate              # Validate festival structure
```

Run `fest understand` for methodology guidance, or `fest --help` for command details.

## Examples

Learn from concrete implementations and proven patterns.

### Available Examples

1. **TASK_EXAMPLES.md**
   - 15+ real task examples
   - Covers different domains (database, API, frontend, DevOps)
   - Shows good vs bad patterns
   - Reference for writing effective tasks

2. **FESTIVAL_TODO_EXAMPLE.md**
   - Complete festival tracking example
   - Shows all states and transitions
   - Demonstrates progress reporting
   - Template for your TODO.md files

### Common Patterns

**Pattern 1: Research-First Development**

```
Phase 001: Ingest external inputs
Phase 002: Plan implementation approach
Phase 003: Implement features
Phase 004: Validate results
```

**Pattern 2: Quality Gates**

```
Every implementation sequence ends with:
- XX_testing
- XX_review
- XX_iterate
```

**Pattern 3: Parallel Task Execution**

```
Tasks with same number can run in parallel:
- 01_frontend_setup.md
- 01_backend_setup.md
- 01_database_setup.md
```

## Core Documentation

Understanding the methodology principles and theory.

### Essential Reading Order

1. **FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT.md**
   - Core methodology principles
   - Three-level hierarchy explanation
   - Phase types and festival types
   - Workflow files and inputs/ directories
   - _Read this first to understand the "why"_

2. **PROJECT_MANAGEMENT_SYSTEM.md**
   - Markdown/YAML tracking system
   - Progress calculation methods
   - Automation opportunities
   - _Read this to understand tracking mechanics_

## Creating Your First Festival

### Quick Start Process

1. **Understand the Goal**

   ```
   Read: FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT.md (core principles)
   Step: Learn step-based goal achievement approach
   ```

2. **Choose a Festival Type**

   ```
   Run: fest types festival
   Pick: standard (general), implementation (specs ready),
         research (investigation), ritual (recurring)
   ```

3. **Create the Festival**

   ```bash
   fest create festival --type standard "my-project-name"
   ```

4. **Create Core Documents**

   ```
   Templates needed:
   - FESTIVAL_OVERVIEW_TEMPLATE.md → FESTIVAL_OVERVIEW.md
   - FESTIVAL_RULES_TEMPLATE.md → FESTIVAL_RULES.md
   - (Optional) Interface templates if multi-system project
   ```

5. **Add Phases as Needed**

   ```bash
   fest create phase --name "001_RESEARCH" --type research
   fest create phase --name "002_IMPLEMENT" --type implementation
   ```

6. **Create Tasks**

   ```
   Use: TASK_TEMPLATE.md
   Reference: TASK_EXAMPLES.md
   ```

7. **Track Progress**

   ```
   Create: TODO.md from FESTIVAL_TODO_TEMPLATE.md
   Update: As tasks complete
   ```

## Festival Lifecycle

Festivals move through lifecycle directories:

```
festivals/
  planning/       # Being planned and designed
  ready/          # Ready for execution
  active/         # Currently executing
  ritual/         # Recurring festivals
  dungeon/        # Archived/deprioritized
    completed/    # Successfully finished
    archived/     # Preserved for reference
    someday/      # May revisit later
```

## Template Customization Guide

### Adapting Templates to Your Needs

All templates are starting points. Customize them by:

1. **Removing irrelevant sections**
   - Not every project needs every section
   - Keep what adds value

2. **Adding project-specific sections**
   - Add sections for your domain
   - Include compliance requirements
   - Add team-specific needs

3. **Adjusting complexity**
   - Simple projects: Use minimal sections
   - Complex projects: Use comprehensive templates
   - Critical tasks: Include all verification steps

### Template Metadata (Frontmatter)

Templates include YAML frontmatter for tooling:

```yaml
---
id: TEMPLATE_NAME
aliases: [alternative, names]
tags: []
created: "YYYY-MM-DD"
modified: "YYYY-MM-DD"
---
```

This metadata:

- Enables search and indexing
- Supports knowledge management tools
- Provides version tracking
- Can be safely ignored if not needed

## Best Practices

### Do's

- Start with planning agent for new festivals
- Choose the right festival type for your work
- Match phase types to the work being done
- Use inputs/ and workflow files for planning phases
- Include quality gates in every implementation sequence
- Update TODO.md as you progress
- Customize templates to fit your project

### Don'ts

- Start coding before planning is complete
- Put numbered sequences inside planning phases (use inputs/ instead)
- Ignore quality verification tasks
- Use templates without customization
- Mix parallel and sequential tasks incorrectly

## Troubleshooting

### Common Issues and Solutions

**Issue: Festival structure too complex**

- Solution: Start with fewer sequences per phase
- Expand as you understand the project better

**Issue: Tasks too abstract**

- Solution: Reference TASK_EXAMPLES.md
- Make tasks concrete with specific deliverables

**Issue: Losing methodology compliance**

- Solution: Engage festival_methodology_manager.md
- Regular reviews with festival_review_agent.md

**Issue: Multi-system coordination**

- Solution: Use Interface Planning Extension for multi-system projects
- See extensions/interface-planning/ for templates and guidance

## Integration with Development Workflow

### With Version Control

```bash
your-project/
├── .git/
├── src/                    # Your code
├── festivals/              # Festival planning
│   ├── planning/           # Festivals being planned
│   ├── active/             # Current festivals
│   └── .festival/          # This directory
└── README.md
```

### With CI/CD

- Parse YAML TODO files for progress metrics
- Generate dashboards from festival status
- Automate phase transitions based on completion
- Validate task completion criteria

### With Project Management Tools

- Export TODO.md to JIRA/Linear/etc.
- Generate Gantt charts from task dependencies
- Calculate velocity from completion rates
- Create burndown charts from progress data

## Summary

This directory contains everything needed to implement Festival Methodology successfully:

1. **Templates** - Start with these, customize as needed
2. **Agents** - Use for guidance and quality control
3. **Examples** - Learn from concrete implementations
4. **Documentation** - Understand the principles

Remember: Festival Methodology is a framework, not a prescription. Adapt it to your needs while maintaining the core principles of step-based goal achievement and the three-level hierarchy (Phases → Sequences → Tasks).

For questions or contributions, see the main [CONTRIBUTING.md](../../CONTRIBUTING.md) file.
