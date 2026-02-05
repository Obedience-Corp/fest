---
fest_type: workflow
fest_id: [REPLACE: PHASE_ID]-WF
fest_parent: [REPLACE: PHASE_ID]
---

# Planning Phase Workflow

This document guides the agent through the planning phase. Follow these steps in order, completing each checkpoint before proceeding.

---

## Step 1: REVIEW — Understand the Inputs

**Goal:** Build comprehensive understanding of what needs to be built.

**Actions:**
1. Read all output specs from ingest phase (if exists)
2. Read research summary (if exists)
3. Identify key requirements, constraints, recommendations

**Output:** Mental model of what needs to be planned

**Checkpoint:** None — proceed to Step 2

---

## Step 2: GAP ANALYSIS — Identify What's Missing

**Goal:** Find unclear requirements or needed decisions.

**Actions:**
1. Note anything unclear or ambiguous
2. Identify decisions that need to be made
3. List questions you'd need answered
4. Create `inputs/gaps.md` if significant gaps exist

**Output:** List of gaps and questions

**Checkpoint:** If critical gaps exist, present to user for clarification

---

## Step 3: DECOMPOSE — Break Down Goals into Festival Structure

**Goal:** Transform requirements into the festival hierarchy (core methodology).

**Actions:**
1. Identify the **Festival Goal** — what the entire festival accomplishes
2. Break into **Phase Goals** — major stages of work:
   - What planning phases are needed?
   - What implementation phases are needed?
   - What review phases are needed?
3. For each phase, identify **Sequence Goals** — groups of related tasks
4. For each sequence, identify **Task Specifications** — atomic units of work
5. Document the hierarchy in `plan/STRUCTURE.md`

**Output:** Documented festival structure showing:
- Phase breakdown with goals
- Sequence breakdown within phases
- Task list within sequences
- Dependencies between components

**Checkpoint:** None — proceed to Step 4

---

## Step 4: DESIGN — Make Architecture Decisions

**Goal:** Make and document key design decisions.

**Actions:**
1. For each significant decision, document options and tradeoffs
2. Create `decisions/D###_title.md` for each decision
3. Update `decisions/INDEX.md`

**Output:** Documented architecture decisions

**Checkpoint:** None — proceed to Step 5

---

## Step 5: STRUCTURE — Create Implementation Plan Document

**Goal:** Define phases, sequences, and tasks in detail.

**Actions:**
1. Create `plan/IMPLEMENTATION_PLAN.md` with:
   - Overview of what will be implemented
   - Phases with their goals
   - Sequences within each phase
   - Tasks within each sequence
   - Dependencies and ordering

**Output:** Complete implementation plan document

**Checkpoint:** None — proceed to Step 6

---

## Step 6: PRESENT — Get User Approval

**Goal:** Verify plan is ready for implementation.

**Actions:**
1. Summarize the plan (phases, sequences, key decisions)
2. Note areas of uncertainty
3. Ask: "Is this plan ready for implementation?"

**Output:** Summary presented to user

**Checkpoint:** APPROVAL REQUIRED — Wait for user response

---

## Step 7: SCAFFOLD — Create Festival Structure

**Goal:** Create the festival directory structure using fest CLI.

**Actions:**
1. If user rejects: Note feedback, return to relevant step
2. If user approves, **first learn the structure rules:**
   a. Run `fest understand structure` — learn the 3-level hierarchy, required files, and what a well-formed festival looks like
   b. Run `fest understand rules` — learn mandatory naming conventions (phase/sequence/task prefixes), required files at each level, and quality gate placement
   c. Run `fest understand tasks` — learn when task files are required (implementation phases MUST have them) vs. optional (planning/review/research phases)
   d. Run `fest understand templates` — learn template variables you can pass to `fest create` to generate pre-filled documents and avoid post-creation editing
3. **Then scaffold the structure using fest CLI:**
   a. Create phases: `fest create phase --type <type> <name>`
   b. Create sequences: `fest create sequence <name>`
   c. Create tasks: `fest create task --name "<name>"`
4. **For each created file:**
   - Read the template that was used
   - Replace ALL `[REPLACE: ...]` markers with actual values
   - Ensure no markers remain unfilled

**Output:** Scaffolded festival structure with all markers filled

**Checkpoint:** None — proceed to Step 8

---

## Step 8: VALIDATE — Verify Structure and Apply Gates

**Goal:** Ensure festival is structurally valid and ready for execution.

**Actions:**
1. Run `fest validate` to check festival structure
2. Fix any validation errors
3. **Fill gate markers at phase level:**
   - Navigate to each implementation phase's `gates/` directory
   - Read each gate template file
   - Replace markers with project-specific values:
     - Test commands (e.g., `go test ./...`)
     - Coverage thresholds (e.g., `80%`)
     - Project-specific verification steps
4. Run `fest gates apply --approve` to propagate gates to sequences
5. Run `fest validate` again to confirm no unfilled markers

**Output:** Valid festival structure ready for implementation

**Checkpoint:** None — phase ends

---

## Workflow State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. REVIEW | [ ] pending | |
| 2. GAP ANALYSIS | [ ] pending | May checkpoint if critical gaps |
| 3. DECOMPOSE | [ ] pending | Core methodology step |
| 4. DESIGN | [ ] pending | |
| 5. STRUCTURE | [ ] pending | |
| 6. PRESENT | [ ] pending | Blocks until user approval |
| 7. SCAFFOLD | [ ] pending | Fill all markers |
| 8. VALIDATE | [ ] pending | Run fest validate |
