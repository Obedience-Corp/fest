---
fest_type: workflow
fest_id: [REPLACE: PHASE_ID]-WF
fest_parent: [REPLACE: PHASE_ID]
---

# Research Phase Workflow

This document guides the agent through the research phase. Follow these steps in order, completing each checkpoint before proceeding.

---

## Step 1: SCOPE — Define Research Questions

**Goal:** Establish clear research objectives.

**Actions:**
1. Review inputs from previous phases
2. Identify key questions that need answering
3. Determine what "good enough" research looks like
4. Create `sources/INDEX.md` with planned sources

**Output:** Research questions and source plan

**Checkpoint:** None — proceed to Step 2

---

## Step 2: DISCOVER — Gather Information

**Goal:** Collect relevant information from identified sources.

**Actions:**
1. Work through each source in `sources/INDEX.md`
2. Document findings in `findings/` directory
3. Note gaps, contradictions, or areas needing more research
4. Update source index with status

**Output:** Raw findings documented

**Checkpoint:** None — proceed to Step 3

---

## Step 3: ANALYZE — Synthesize Findings

**Goal:** Transform raw findings into actionable insights.

**Actions:**
1. Review all findings
2. Identify patterns, themes, recommendations
3. Create `findings/SUMMARY.md` with synthesized conclusions
4. Note confidence levels and caveats

**Output:** Research summary with conclusions

**Checkpoint:** None — proceed to Step 4

---

## Step 4: PRESENT — Get User Validation

**Goal:** Verify research addresses the original questions.

**Actions:**
1. Present summary of findings
2. Highlight key recommendations
3. Note any remaining uncertainties
4. Ask: "Does this research answer your questions?"

**Output:** Summary presented to user

**Checkpoint:** APPROVAL REQUIRED — Wait for user response

---

## Step 5: ITERATE or COMPLETE

**Goal:** Handle feedback or finalize research.

**Actions:**
1. If user needs more: Note gaps, return to Step 2
2. If user approves: Mark phase complete

**Output:** Phase completion or additional research

**Checkpoint:** None — phase ends

---

## Workflow State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. SCOPE | [ ] pending | |
| 2. DISCOVER | [ ] pending | |
| 3. ANALYZE | [ ] pending | |
| 4. PRESENT | [ ] pending | Blocks until user approval |
| 5. COMPLETE | [ ] pending | |
