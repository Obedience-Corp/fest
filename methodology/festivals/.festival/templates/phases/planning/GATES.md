---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Planning Phase Gate

This gate verifies the planning phase was completed correctly before advancing.

---

## Step 1: METHODOLOGY — Verify Workflow Compliance

**Question:** Did you follow each step of the planning WORKFLOW.md in order (REVIEW, GAP ANALYSIS, DECOMPOSE, DESIGN, STRUCTURE, PRESENT, SCAFFOLD, VALIDATE)?

**Actions:**
1. Confirm no workflow steps were skipped
2. Verify each step produced its expected output
3. Check that checkpoints requiring user approval were honored

**Checkpoint:** APPROVAL REQUIRED — Confirm methodology compliance before proceeding

---

## Step 2: APPROVAL — Verify User Sign-Off

**Question:** Did the user explicitly approve the plan before scaffolding began?

**Actions:**
1. Confirm the PRESENT step received user approval
2. Verify any user feedback was incorporated before scaffolding
3. Check that the plan was not scaffolded without approval

**Checkpoint:** APPROVAL REQUIRED — Confirm user approved the plan

---

## Step 3: STRUCTURE — Verify Festival Integrity

**Question:** Is the scaffolded festival structurally valid with all markers filled?

**Actions:**
1. Run `fest validate` and confirm it passes
2. Verify no `[REPLACE: ...]` markers remain in any document
3. Confirm phases are properly ordered with clear goals
4. Check that implementation sequences have appropriate task specifications

**Checkpoint:** APPROVAL REQUIRED — Confirm structure is valid

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. METHODOLOGY | [ ] pending | Workflow compliance |
| 2. APPROVAL | [ ] pending | User sign-off |
| 3. STRUCTURE | [ ] pending | Festival integrity |
