---
id: FESTIVAL_SOFTWARE_PROJECT_MANAGEMENT
aliases: []
tags: []
---

# Festival Planning Methodology

## Overview

Festival methodology is a **collaborative, step-oriented planning approach** between humans and AI agents that enables goal achievement through systematic progression. Unlike traditional project management, festivals focus on identifying and completing the logical steps needed to achieve goals, leveraging unprecedented AI-human efficiency that makes traditional time estimates obsolete.

## Core Principles

1. **Goal-Oriented Step Planning**: Think in terms of steps needed to achieve goals, not time estimates or schedules
2. **Human-AI Collaborative Planning**: Humans provide goals and requirements, AI agents identify and structure the logical steps needed
3. **Requirements-Driven Implementation**: Implementation sequences can ONLY be created after requirements are defined - either through planning phases or external documentation
4. **Just-in-Time Sequence Creation**: Implementation work is added one step at a time as requirements become clear, based on logical progression
5. **Hyper-Efficient AI Execution**: AI-human collaboration works at unprecedented speeds, making traditional time estimates meaningless
6. **Step-Based Progression**: Work is organized as logical steps (phases → sequences → tasks) that build toward goal achievement
7. **Context Preservation**: All decisions and rationale captured in CONTEXT.md to maintain continuity across sessions
8. **Quality Gates**: Every implementation sequence includes verification steps to ensure goal progression
9. **Extensible Methodology**: Extensions available for specialized needs like multi-system coordination

## Step-Based vs Time-Based Thinking

**FUNDAMENTAL PRINCIPLE**: Festival Methodology thinks in **STEPS TO GOALS**, not time estimates.

### Why Steps, Not Time?

Traditional project management focuses on:

- Time estimates and schedules
- Duration-based planning
- Timeline management
- Resource allocation over time

Festival Methodology focuses on:

- **Logical steps toward goal achievement**
- **Completion criteria for each step**
- **Dependencies between steps**
- **Parallel step opportunities**

### The Efficiency Reality

AI-human collaboration operates at unprecedented efficiency levels that make traditional time estimates obsolete. Instead of asking "How long will this take?", Festival Methodology asks:

- "What steps are needed to achieve this goal?"
- "What's the logical order for these steps?"
- "What can be done in parallel?"
- "How do we know each step is complete?"
- "What's the next step after this one completes?"

## Collaborative Workflow

**CRITICAL UNDERSTANDING**: Festival Methodology is NOT about AI agents pre-planning entire projects. It's about **human-AI collaboration** where:

### Human Responsibilities

- Provide project goals and requirements
- Define success criteria and constraints
- Make architectural and design decisions
- Review and approve AI-generated sequences
- Guide iteration and adaptation

### AI Agent Responsibilities

- Identify logical steps needed to achieve goals
- Structure requirements into executable step sequences
- Create detailed task specifications with completion criteria
- Execute implementation steps autonomously at unprecedented speed
- Document decisions and progress toward goal achievement
- Request clarification when requirements are unclear

### The Planning-Implementation Boundary

**Planning Steps (Optional):**

- May be completed before festival creation
- May be first step in festival progression
- May be provided as external documentation
- Results in clear requirements for implementation steps

**Implementation Steps:**

- Can ONLY be created after requirements are defined
- Are added one logical step at a time based on requirements
- Follow goal progression logic, not time schedules
- Emerge from human-provided specifications and goal definitions

## Festival Lifecycle

Festivals move through lifecycle directories as they progress:

```
festivals/
  planning/       # Festivals being planned and designed
  ready/          # Festivals planned and ready for execution
  active/         # Currently executing festivals
  ritual/         # Recurring/repeatable festivals (ritual type)
  dungeon/        # Archived/deprioritized work
    completed/    # Successfully finished festivals
    archived/     # Archived festivals preserved for reference
    someday/      # Deprioritized festivals that may be revisited
```

The lifecycle flow is: `planning/ → ready/ → active/ → dungeon/completed/`

## Festival Types

Festivals come in four types, each designed for different kinds of work:

| Festival Type          | Purpose                                                              | Auto-Generated Phases                                                                                                | When to Use                                                                   |
| ---------------------- | -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| **standard** (default) | General-purpose festivals with full planning and implementation      | INGEST (ingest), PLAN (planning) auto-scaffolded; IMPLEMENT (implementation) and POLISH (planning) created as needed | Most projects that need both planning and implementation                      |
| **implementation**     | Execution-only festivals for work with existing detailed specs/plans | IMPLEMENT (implementation) auto-scaffolded; skip_ingestion=true                                                      | When requirements are already defined externally and you just need to execute |
| **research**           | Investigation and exploration festivals                              | INGEST (ingest), RESEARCH (research), SYNTHESIZE (planning) auto-scaffolded                                          | When the goal is to investigate, audit, or explore rather than build          |
| **ritual**             | Recurring/repeatable festivals with custom structure                 | No default phases; structure determined by ritual template                                                           | Repeatable processes like releases, audits, or maintenance cycles             |

Create a festival with a specific type:

```bash
fest create festival --type standard my-festival-name
fest create festival --type implementation my-feature
fest create festival --type research my-investigation
fest create festival --type ritual my-recurring-process
```

## Festival Structure and Phase Flexibility

### Three-Level Hierarchy

Festivals use a three-level hierarchy: **Phases → Sequences → Tasks**

- **Phases**: Top-level organization grouping related work (3-digit numbering: 001*, 002*, 003\_)
- **Sequences**: Work that must happen in order within a phase (2-digit numbering: 01*, 02*)
- **Tasks**: Individual work items within sequences (2-digit numbering: 01*, 02*)

### Phase Types

Every phase has a **type** that determines its structural conventions and purpose. There are 6 phase types:

| Phase Type            | Purpose                                       | Structural Conventions                                                                                                                              |
| --------------------- | --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **planning**          | Design, architecture, requirements gathering  | Uses `inputs/` directory and workflow files (WORKFLOW.md). No numbered sequences or task files. Contains decisions, plans, and reference materials. |
| **implementation**    | Writing code, building features               | Uses numbered sequences with task files. Quality gates auto-appended to each sequence. This is where agents work autonomously.                      |
| **research**          | Investigation, exploration, auditing          | Uses WORKFLOW.md with `sources/`, `findings/` directories and analysis templates. No numbered sequences or task files.                              |
| **review**            | Code review, integration testing, validation  | Freeform. PHASE_GOAL.md with review criteria and checklists. No required internal structure.                                                        |
| **ingest**            | Absorbing external content, data migration    | Uses WORKFLOW.md with `input_specs/`, `output_specs/` directories. No numbered sequences or task files.                                             |
| **non_coding_action** | Documentation, process changes, non-code work | Freeform. PHASE_GOAL.md with action items and checklists. No required internal structure.                                                           |

Create a phase with a specific type:

```bash
fest create phase --name "001_RESEARCH" --type research
fest create phase --name "002_IMPLEMENT" --type implementation
fest create phase --name "001_INGEST" --type ingest
```

**Key Principle**: Don't pre-plan phases. Add them as needed when requirements emerge or new work is identified.

### Phase Type Structural Conventions

#### Planning Phases (type: planning)

Planning phases use a different internal structure than implementation phases. Instead of numbered sequences and task files, they use:

- **`inputs/`** — Reference materials, external documents, source content for planning
- **`WORKFLOW.md`** — A workflow file describing the planning process
- **Decision documents** — Capture architectural and design decisions
- **Plan documents** — The resulting plans and specifications

```
001_PLAN/
├── PHASE_GOAL.md
├── WORKFLOW.md            # Planning process/workflow
├── inputs/                # External reference materials
│   ├── requirements.md
│   └── stakeholder_notes.md
├── decisions/             # Captured decisions
│   └── architecture_decision.md
└── plan/                  # Resulting plans
    └── implementation_plan.md
```

**Do NOT** create numbered sequences or task files inside planning phases unless deep, structured planning requires it.

#### Implementation Phases (type: implementation)

Implementation phases MUST have numbered sequences with task files. This is where AI agents work autonomously for extended periods.

```
002_IMPLEMENT/
├── PHASE_GOAL.md
├── 01_backend_foundation/
│   ├── 01_database_setup.md
│   ├── 02_api_endpoints.md
│   ├── 03_testing.md           ← Quality gate
│   ├── 04_review.md            ← Quality gate
│   └── 05_iterate.md           ← Quality gate
├── 02_frontend_implementation/
│   ├── 01_components.md
│   ├── 02_state_management.md
│   ├── 03_testing.md           ← Quality gate
│   ├── 04_review.md            ← Quality gate
│   └── 05_iterate.md           ← Quality gate
└── completed/
```

#### Research Phases (type: research)

Research phases use WORKFLOW.md with `sources/` and `findings/` directories. Focus is on investigation, findings, and analysis rather than building. No numbered sequences or task files.

```
001_RESEARCH/
├── PHASE_GOAL.md
├── WORKFLOW.md
├── sources/
│   └── reference_material.md
└── findings/
    └── analysis_report.md
```

#### Ingest Phases (type: ingest)

Ingest phases use WORKFLOW.md with `input_specs/` and `output_specs/` directories. For absorbing and processing external content before other work begins. No numbered sequences or task files.

```
001_INGEST/
├── PHASE_GOAL.md
├── WORKFLOW.md
├── input_specs/
│   └── external_specs.md
└── output_specs/
    ├── purpose.md
    ├── requirements.md
    ├── constraints.md
    └── context.md
```

#### Review Phases (type: review)

Review phases are freeform — PHASE_GOAL.md defines the review scope, criteria, and checklists. No required internal structure beyond PHASE_GOAL.md.

```
003_VALIDATE/
├── PHASE_GOAL.md
└── (freeform content as needed)
```

### Workflow Files

Workflow files are markdown documents that describe a process or workflow for a phase. They provide structured guidance without requiring the overhead of numbered sequences and tasks.

- **What**: A `WORKFLOW.md` file placed directly inside a phase directory
- **When**: Used in planning, ingest, and research phases
- **Purpose**: Describes the steps, decisions, and activities for the phase
- **Relationship to sequences**: Can be the sole structural element in a phase, OR can coexist with sequences (hybrid phases)

### Hybrid Phases

A phase can contain BOTH workflow files AND numbered sequences. This is called a **hybrid phase**. Use this when a phase has some structured process documentation alongside executable task sequences.

```
001_PLAN/
├── PHASE_GOAL.md
├── WORKFLOW.md                    # Overall planning workflow
├── inputs/                        # Reference materials
│   └── external_requirements.md
├── 01_detailed_analysis/          # Sequence for structured analysis work
│   ├── 01_analyze_requirements.md
│   └── 02_document_findings.md
└── decisions/
    └── architecture_decision.md
```

### Sequence Creation Guidelines

**WHEN TO CREATE SEQUENCES:**

✅ **Create sequences when:**

- Human provides specific requirements or specifications
- Planning phase has been completed with clear deliverables
- External planning documents define what needs to be built
- Human explicitly asks for implementation of specific functionality

❌ **DO NOT create sequences when:**

- No requirements have been provided
- Planning phase hasn't been completed
- Guessing what might need to be implemented
- Making assumptions about user needs

### Sequence Design Guidelines

**Good Sequences** contain 3-6 related tasks that:

- Build on each other logically
- Share common setup or dependencies
- Form a cohesive unit of work (e.g., "user authentication", "API endpoints")
- Can be assigned to one person/agent for focused work
- Are derived from specific requirements or specifications

**Avoid These Sequence Anti-Patterns:**

- Single task per sequence (make it a standalone task instead)
- Unrelated tasks grouped arbitrarily
- Sequences with >8 tasks (break into multiple sequences)
- Mixing different work types (frontend + backend + DevOps in same sequence)
- **Creating sequences without requirements** (the biggest anti-pattern)

**Example Good Sequence:**

```
01_user_authentication/
├── 01_create_user_model.md
├── 02_add_password_hashing.md
├── 03_implement_login_endpoint.md
├── 04_add_jwt_tokens.md
├── 05_testing_and_verify.md       ← Standard quality gate
├── 06_code_review.md              ← Standard quality gate
└── 07_review_results_iterate.md   ← Standard quality gate
```

## Festival Directory Structure

Here's the recommended structure for a festival:

```text
festivals/
├── planning/                  # Festivals being planned
├── ready/                     # Festivals ready for execution
├── active/                    # Currently executing festivals
├── ritual/                    # Recurring/repeatable festivals
├── dungeon/                   # Archived/deprioritized work
│   ├── completed/             # Successfully finished festivals
│   ├── archived/              # Archived for reference
│   └── someday/               # May revisit later
└── active/festival_<id>/
    ├── FESTIVAL_GOAL.md       # Festival objective and success criteria
    ├── FESTIVAL_OVERVIEW.md   # High-level goal, systems, and features overview
    ├── FESTIVAL_RULES.md      # Rules and principles to follow throughout the festival
    ├── fest.yaml              # Festival configuration (quality gates, excluded patterns)
    ├── 001_RESEARCH/          # PHASE (type: research): Investigation
    │   ├── PHASE_GOAL.md
    │   ├── 01_audit_current_state/    # SEQUENCE: Audit work
    │   │   ├── 01_inventory_files.md          # TASK
    │   │   ├── 02_analyze_gaps.md             # TASK
    │   │   └── 03_document_findings.md        # TASK
    │   └── completed/
    ├── 002_IMPLEMENT/         # PHASE (type: implementation): Build
    │   ├── PHASE_GOAL.md
    │   ├── 01_backend_foundation/             # SEQUENCE: Backend implementation
    │   │   ├── 01_database_setup.md           # TASK (parallel tasks have same number)
    │   │   ├── 01_api_endpoints.md            # TASK (can work simultaneously)
    │   │   ├── 02_integration_layer.md        # TASK (must complete after 01_ tasks)
    │   │   ├── 03_testing.md                  # TASK ← Quality gate
    │   │   ├── 04_review.md                   # TASK ← Quality gate
    │   │   ├── 05_iterate.md                  # TASK ← Quality gate
    │   │   └── results/                       # Sequence results
    │   ├── 02_frontend_implementation/        # SEQUENCE: Frontend implementation
    │   │   ├── 01_components.md               # TASK
    │   │   ├── 02_state_management.md         # TASK
    │   │   ├── 03_testing.md                  # TASK ← Quality gate
    │   │   ├── 04_review.md                   # TASK ← Quality gate
    │   │   ├── 05_iterate.md                  # TASK ← Quality gate
    │   │   └── results/                       # Sequence results
    │   └── completed/                         # Completed sequences in this phase
    ├── completed/             # Completed phases
    └── dungeon/               # Archived/deprioritized work
```

Note: This structure is a guideline, not a rigid requirement. Adapt it to fit your festival's specific needs.

## Planning Process

### 1. Define the Goal

Start with a clear, concrete objective. This should be outcome-focused, not activity-focused. Goals can range from simple (a single Jira ticket) to complex (a company-wide vision). Anyone with a goal and understanding of what needs to be done can be a festival planner.

### 2. Break Down into Systems and Features

- **Systems**: Major components or areas of work
- **Features**: Specific functionality within systems

### 3. Plan Implementation Approach

**Define how implementation work will be organized and executed.**

For most projects:

1. **Define Implementation Approach**: Identify the components and systems that need to be built
2. **Plan Implementation Structure**: Organize implementation work into logical sequences:
   - System components and their responsibilities
   - Expected behavior and constraints
   - Error handling approaches
   - Integration patterns
3. **Review and Iterate**: Have stakeholders and technical leads review the implementation plan

**For Multi-System Projects**: Consider the [Interface Planning Extension](extensions/interface-planning/) to add formal interface definition phases when system coordination is critical.

**Benefits of Clear Implementation Planning:**

- Enables organized development across teams and agents
- Reduces confusion and rework through clear structure
- Provides clear understanding of system components
- Allows systematic festival execution with minimal iterations

### 4. Create FESTIVAL_OVERVIEW.md

Document:

- The high-level goal
- Systems breakdown (if applicable)
- Features breakdown (if applicable)
- Success criteria

### 5. Define FESTIVAL_RULES.md

Establish the principles and quality standards that all workers must follow throughout the festival. This ensures consistent quality and reminds workers of best practices at each step. Rules should cover:

- Engineering excellence principles
- Quality standards and gates
- Development process requirements
- Decision-making guidelines

### 6. Organize Work into Flexible Phases

**Phases group related sequences together logically.** Choose phase types based on the work needed. Phases can be customized, repeated, or reordered based on project needs.

**Understanding the Three-Level Hierarchy:**

- **Phases**: High-level organization grouping related sequences (use 3-digit numbering: 001*, 002*, 003\_)
- **Sequences**: Work that must happen in order within a phase (use 2-digit numbering: 01*, 02*)
- **Tasks**: Individual work items within sequences (use 2-digit numbering: 01*, 02*)

#### Common Phase Patterns

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

**Custom Phases**: Add specialized phases like `005_SECURITY_AUDIT/`, `006_PERFORMANCE_OPTIMIZATION/`, or `007_MIGRATION/` as needed.

#### Extensions for Specialized Needs

For projects requiring system coordination, use the [Interface Planning Extension](extensions/interface-planning/) which adds interface definition phases. See the [Extensions Guide](extensions/) for other specialized workflow patterns.

**Within Each Phase:**

- **Phases** use 3-digit numbering (001*, 002*, 003\_) to support hundreds of phases
- **Sequences** within phases use 2-digit numbering (01*, 02*, etc.) for proper ordering
- **Tasks** within sequences use 2-digit numbering (01_task.md, 02_task.md, etc.)
- Tasks with the same number can be executed in parallel (e.g., 01_task_a.md, 01_task_b.md, 01_task_c.md)
- Each task gets its own markdown file with clear requirements
- Every implementation sequence must include quality gate tasks:
  - `XX_testing.md` - Testing and verification (where XX follows implementation tasks)
  - `XX+1_review.md` - Code review
  - `XX+2_iterate.md` - Review results and iterate if needed
- Create a `results/` subdirectory in each sequence for testing results and code review documents

**Numbering System Benefits:**

- **3-digit phases**: Supports up to 999 phases for large, long-running festivals
- **2-digit sequences/tasks**: Maintains readability while supporting up to 99 items per level
- **Proper sorting**: Ensures correct alphabetical and numerical ordering in directory trees
- **Visual consistency**: Clear hierarchy distinction between organizational levels

## Phase Flexibility Benefits

The flexible phase approach provides significant advantages:

**Adapt to Your Needs**: Phases match your actual work, not a rigid template

- Planning phases: Use workflow files and inputs/ directories
- Implementation phases: Use numbered sequences with task files and quality gates
- Research phases: Use sequences with investigation tasks
- Multiple implementation phases: Add as many as needed for complex projects
- Skip unnecessary phases: No planning phase if requirements provided

**Reduced Overhead**: Only add structure where it provides value
**Clear Purpose**: Each phase type has distinct structural conventions
**Better AI Execution**: Implementation phases optimized for autonomous work
**Natural Progression**: Phases emerge as requirements become clear

## Flexibility and Scaling

Festivals scale from simple to complex:

- **Simple Festival**: Implementation type with a single IMPLEMENT phase containing sequences
- **Medium Festival**: Standard type with INGEST, PLAN, and IMPLEMENT phases
- **Complex Festival**: Multiple implementation phases with research, planning, and validation

The optional directories serve specific purposes:

- **docs/**: House documentation directly related to the festival's goal (inside phases)
- **inputs/**: Reference materials and external content for planning/ingest phases
- **results/**: Testing results and review documents (inside sequences)
- **completed/**: Move successfully finished sequences here to keep the active workspace clean
- **dungeon/**: Archived work - deprioritized festivals that may be valuable later

These directories can be included at any complexity level as needed and are only created when necessary.

## Key Advantages

1. **No Process Overhead**: No daily standups, sprint planning, or retrospectives unless actually needed
2. **Clear Dependencies**: Sequential directories make dependencies obvious
3. **Flexible Scope**: Add requirements as you discover them
4. **Goal Achievement**: Success is measured by goal completion, not velocity metrics
5. **Deep Understanding**: Requires and encourages thorough problem understanding upfront

## When to Use Festival Methodology

Festival methodology works best when:

- You have a clear goal to achieve (from a simple bug fix to a major product launch)
- You want to minimize process overhead
- The work has natural dependencies and multiple system interfaces
- You need flexibility in scope and timeline
- Team members can work independently on well-defined tasks with clear interface contracts
- You're working solo or collaboratively - festivals adapt to both modes
- You can define system interfaces upfront to enable parallel development

## Example Festival

```text
festivals/active/
└── festival_user_onboarding/
    ├── FESTIVAL_GOAL.md
    ├── FESTIVAL_OVERVIEW.md
    ├── FESTIVAL_RULES.md
    ├── fest.yaml
    ├── 001_INGEST/                        # PHASE (type: ingest)
    │   ├── PHASE_GOAL.md
    │   ├── inputs/
    │   │   ├── product_requirements.md
    │   │   └── stakeholder_notes.md
    │   └── 01_process_requirements/
    │       ├── 01_organize_inputs.md
    │       └── 02_identify_gaps.md
    ├── 002_PLAN/                          # PHASE (type: planning)
    │   ├── PHASE_GOAL.md
    │   ├── WORKFLOW.md                    # Planning workflow
    │   ├── inputs/
    │   │   └── processed_requirements.md
    │   ├── decisions/
    │   │   └── architecture_decision.md
    │   └── plan/
    │       └── implementation_plan.md
    ├── 003_IMPLEMENT/                     # PHASE (type: implementation)
    │   ├── PHASE_GOAL.md
    │   ├── 01_backend_foundation/
    │   │   ├── 01_user_model.md
    │   │   ├── 01_api_endpoints.md        # Parallel with above
    │   │   ├── 01_database_migrations.md  # Parallel
    │   │   ├── 02_integration_layer.md    # After 01_ tasks
    │   │   ├── 03_testing.md              # Quality gate
    │   │   ├── 04_review.md               # Quality gate
    │   │   ├── 05_iterate.md              # Quality gate
    │   │   └── results/
    │   ├── 02_frontend_implementation/
    │   │   ├── 01_registration_flow.md
    │   │   ├── 01_verification_ui.md      # Parallel
    │   │   ├── 02_error_handling.md       # After 01_ tasks
    │   │   ├── 03_testing.md              # Quality gate
    │   │   ├── 04_review.md               # Quality gate
    │   │   ├── 05_iterate.md              # Quality gate
    │   │   └── results/
    │   └── completed/
    ├── completed/
    └── dungeon/
```

## Festival Rules

Festival Rules are a critical component that ensures quality and consistency
throughout the festival. Every festival should include a FESTIVAL_RULES.md file
that workers reference before and during task execution.

### Purpose of Festival Rules

- **Maintain Quality Standards**: Establish clear quality gates and acceptance
  criteria
- **Ensure Consistency**: All workers follow the same principles and practices
- **Reduce Rework**: Prevent common mistakes by providing clear guidelines
  upfront
- **Promote Excellence**: Embed staff-level engineering principles into every
  task

### Common Rules for Software Festivals

1. **Engineering Excellence**
   - Prefer refactoring existing code over rewriting from scratch
   - Follow established patterns and conventions in the codebase
   - Apply SOLID principles and avoid over-engineering (YAGNI)
   - Keep functions under 50 lines, files under 500 lines

2. **Quality Standards**
   - Write tests for all new functionality
   - Maintain or improve code coverage
   - Run linters and type checkers before marking tasks complete
   - Document architectural decisions and breaking changes

3. **Development Process**
   - Create small, focused pull requests (one logical change)
   - Update documentation alongside code changes
   - Consider security implications in all changes
   - Maintain backward compatibility unless explicitly approved

### Task Integration

Each task should include a "Rules Compliance" section that:

- References relevant rules from FESTIVAL_RULES.md
- Includes a pre-task checklist
- Provides a completion checklist for verification

## Creating Actionable Tasks

### The Problem with Abstract Tasks

AI agents often create generic, high-level task descriptions that don't lead to concrete implementation. This defeats the purpose of the festival methodology.

### Bad Task Examples (Abstract and Vague)

```markdown
# Task: 01_user_management.md

## Objective

Implement user management functionality

## Requirements

- [ ] Create user system
- [ ] Add authentication
- [ ] Handle user data

## Deliverables

- User management feature
- Authentication system
```

**Problems**:

- No specific file names or code examples
- Vague requirements that don't specify implementation details
- Generic deliverables that could mean anything

### Good Task Examples (Concrete and Specific)

````markdown
# Task: 01_create_user_table_and_model.md

## Objective

Create PostgreSQL user table and Sequelize model with email/password authentication fields

## Requirements

- [ ] Create `users` table with id, email, password_hash, created_at, updated_at
- [ ] Create `models/User.js` with Sequelize model definition
- [ ] Add email validation method with regex: /^[^\s@]+@[^\s@]+\.[^\s@]+$/
- [ ] Add bcrypt password hashing with salt rounds = 12

## Implementation Steps

1. Run: `npx sequelize-cli migration:generate --name create-users-table`
2. Edit migration file with SQL schema
3. Create `models/User.js` with Sequelize model
4. Add bcrypt dependency: `npm install bcrypt`
5. Run: `npx sequelize-cli db:migrate`

## Testing Commands

```bash
npm test -- tests/models/User.test.js
node -e "const User = require('./models/User'); console.log('User model loaded');"
```
````

## Deliverables

- [ ] `migrations/001_create_users_table.js` migration file
- [ ] `models/User.js` Sequelize model with authentication methods
- [ ] `tests/models/User.test.js` unit tests
- [ ] Updated `package.json` with bcrypt dependency

````

**Why This Works**:
- Specific file names and directory paths
- Exact code snippets and SQL schemas
- Concrete commands to execute
- Testable deliverables with clear file paths

### Guidelines for Writing Actionable Tasks

#### 1. Use Specific Names and Paths
- `Create models/User.js with Sequelize model` (good)
- `Create user model` (bad)

#### 2. Include Implementation Steps with Code
- Provide exact SQL, JavaScript, commands (good)
- Say "implement database schema" (bad)

#### 3. Specify Testing Commands
- `npm test -- tests/models/User.test.js` (good)
- "Test the functionality" (bad)

#### 4. List Exact Deliverables
- `src/components/LoginForm.jsx`, `tests/LoginForm.test.js` (good)
- "Login component and tests" (bad)

### Task Complexity Levels

#### Level 1: Single File Creation
**Good for**: Creating individual files, small components, utility functions
```markdown
Objective: Create EmailValidator utility with regex validation
Requirements:
- [ ] Create `utils/EmailValidator.js` with isValid() method
- [ ] Use regex pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/
- [ ] Export as CommonJS module
Commands: node -e "const validator = require('./utils/EmailValidator'); console.log(validator.isValid('test@example.com'));"
````

#### Level 2: Multi-File Implementation

**Good for**: API endpoints, React components with styling, database operations

```markdown
Objective: Implement user registration API endpoint with validation
Requirements:

- [ ] Create routes/users.js with POST /users endpoint
- [ ] Create middleware/validation.js with registration validation
- [ ] Add bcrypt password hashing
- [ ] Create tests/routes/users.test.js
      Commands: curl -X POST localhost:3000/api/users -d '{"email":"test@example.com","password":"SecurePass123"}'
```

#### Level 3: Feature Implementation

**Good for**: Complete features spanning multiple files and systems

```markdown
Objective: Build user authentication flow with database, API, and frontend
Requirements:

- [ ] Database: Create users table with authentication fields
- [ ] Backend: Implement registration/login endpoints with JWT
- [ ] Frontend: Create LoginForm and RegistrationForm components
- [ ] Testing: Unit tests for all components and integration tests
```

### Common Mistakes to Avoid

#### 1. Using Placeholders Instead of Real Examples

- `interface [ComponentName]Props` (bad)
- `interface LoginFormProps` (good)

#### 2. Abstract Requirements

- "Handle user authentication" (bad)
- "Implement JWT authentication with 7-day expiry using jsonwebtoken library" (good)

#### 3. Missing Implementation Details

- "Create database schema" (bad)
- "Create users table with: id SERIAL PRIMARY KEY, email VARCHAR(255) UNIQUE NOT NULL, password_hash VARCHAR(255) NOT NULL" (good)

#### 4. Vague Testing Instructions

- "Test the feature" (bad)
- "Run: curl -X POST localhost:3000/api/users -H 'Content-Type: application/json' -d '{\"email\":\"<test@example.com>\"}'" (good)

### Reference Resources

- **TASK_EXAMPLES.md**: 15+ concrete examples across database, API, frontend, DevOps, and testing domains
- **COMMON_INTERFACES_TEMPLATE.md**: Real interface examples instead of placeholders
- **TASK_TEMPLATE.md**: Enhanced template with good vs bad examples

### Implementation-Ready Principle

Every task should be "implementation-ready" - meaning a developer (human or AI) can start coding immediately without needing additional clarification or research.

**Test**: Can someone copy-paste the code examples and commands from your task and get working results?

## Best Practices

1. **Keep Goals Concrete**: Vague goals lead to scope creep
2. **Write Implementation-Ready Tasks**: Include exact code, commands, and file names
3. **Use Real Examples**: Avoid placeholders - use concrete data and realistic scenarios
4. **Sequence Thoughtfully**: Consider dependencies when creating sequences
5. **Stay Flexible**: Add new sequences or tasks as needed
6. **Complete Before Proceeding**: Finish each sequence before starting the next
7. **Organize Finished Work**: Move completed sequences to completed/,
   archived/deprioritized work to dungeon/
8. **Follow Festival Rules**: Reference and adhere to FESTIVAL_RULES.md
   throughout execution
9. **Test Everything**: Include specific testing commands and expected outputs
