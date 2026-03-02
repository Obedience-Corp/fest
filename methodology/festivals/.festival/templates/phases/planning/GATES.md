---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Planning Phase Gate

This gate verifies the planning phase achieved its goal and produced an approved, valid plan.

---

## Step 1: PHASE GOAL — Verify Goal Achievement

**Question:** Does the plan address the stated planning objective? Is the planned approach sound and complete?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated objectives against the produced plan
2. Verify the plan covers all aspects of the planning objective
3. Confirm the approach is feasible and the decomposition is appropriate

**Checkpoint:** APPROVAL REQUIRED — Confirm planning goal is met

---

## Step 2: APPROVAL — Verify User Sign-Off

**Question:** Did the user explicitly approve the plan?

**Actions:**
1. Confirm the plan received user approval before scaffolding
2. Verify any user feedback was incorporated
3. Check that the plan was not scaffolded without approval

**Checkpoint:** APPROVAL REQUIRED — Confirm user approved the plan

---

## Step 3: STRUCTURE — Verify Festival Integrity

**Question:** Is the scaffolded festival structurally valid?

**Actions:**
1. Run `fest validate` and confirm it passes
2. Verify no `[REPLACE: ...]` markers remain in any document
3. Confirm phases are properly ordered with clear goals

**Checkpoint:** APPROVAL REQUIRED — Confirm structure is valid

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Planning goal achieved |
| 2. APPROVAL | [ ] pending | User sign-off |
| 3. STRUCTURE | [ ] pending | Festival integrity |
