---
fest_type: workflow
fest_id: [REPLACE: PHASE_ID]-WF
fest_parent: [REPLACE: PHASE_ID]
---

# Ingest Phase Workflow

This document guides the agent through the ingest phase. Follow these steps in order, completing each checkpoint before proceeding.

---

## Step 1: READ — Understand All Input

**Goal:** Build comprehensive understanding of what the user has provided.

**Actions:**
1. List all files in `input_specs/`
2. Read each file completely — do not skim
3. Identify: What is the user trying to accomplish? What problem are they solving?
4. Note any questions or ambiguities

**Output:** Mental model of the user's intent (no document yet)

**Checkpoint:** None — proceed to Step 2

---

## Step 2: EXTRACT — Identify Key Elements

**Goal:** Pull out the essential information that needs to be structured.

**Actions:**
1. Extract festival purpose (end goal, "done" criteria, why it matters)
2. Extract requirements (what needs to happen, acceptance criteria, priorities)
3. Extract constraints (technical, process, timeline)
4. Extract context (prior art, related systems, references)

**Output:** Notes on each element (can be rough)

**Checkpoint:** None — proceed to Step 3

---

## Step 3: STRUCTURE — Produce Output Specs

**Goal:** Transform extracted elements into structured documents.

**Actions:**
1. Create `output_specs/purpose.md` with festival purpose, success criteria, motivation
2. Create `output_specs/requirements.md` with prioritized requirements (P0/P1/P2)
3. Create `output_specs/constraints.md` with technical and process constraints
4. Create `output_specs/context.md` with prior art and key references

**Output:** Four documents in `output_specs/`

**Checkpoint:** None — proceed to Step 4

---

## Step 4: PRESENT — Get User Approval

**Goal:** Verify the structured output captures the user's intent.

**Actions:**
1. Summarize what you've produced (don't dump full documents)
2. Highlight any interpretations you made or questions you have
3. Ask: "Do these specs accurately capture what you want to accomplish?"

**Output:** Summary presented to user

**Checkpoint:** APPROVAL REQUIRED — Wait for user response

---

## Step 5: ITERATE or COMPLETE

**Goal:** Handle user feedback or finalize the phase.

**Actions:**
1. If user rejects: Note feedback, return to Step 3, update specs
2. If user approves: Mark phase complete, note any caveats

**Output:** Phase completion or iteration

**Checkpoint:** None — phase ends

---

## Workflow State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. READ | [ ] pending | |
| 2. EXTRACT | [ ] pending | |
| 3. STRUCTURE | [ ] pending | |
| 4. PRESENT | [ ] pending | Blocks until user approval |
| 5. COMPLETE | [ ] pending | |
