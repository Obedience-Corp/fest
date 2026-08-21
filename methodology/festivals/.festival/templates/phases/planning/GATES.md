---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Planning Phase Gate

This gate verifies the planning phase achieved its goal and produced an approved, valid plan.

## Before you submit any step below

Whether the judge can open your repositories depends on its provider:
CLI-backed judges read the declared working directories themselves, while
judges without tool access see only what this phase wrote down. Writing the
essentials here keeps the gate independent of which judge you drew, and leaves
the festival a durable record after the worktree is gone.

Write the phase's evidence into `output_specs/` before submitting the first
step. These are picked up automatically; you do not need to list them:

| File | What belongs in it |
| --- | --- |
| `output_specs/requirements.md` | each required outcome from PHASE_GOAL.md, with the raw command output that verifies it |
| `output_specs/context.md` | where the work landed: commits, PRs, branches, build provenance |
| `output_specs/constraints.md` | each quality standard, with its proof |
| `output_specs/PRESENTATION.md` | the summary, and anything that needs a human decision |

Paste real terminal output, not summaries of it. "All tests pass" is an
assertion; the test run is evidence. A gate that rejects for missing evidence is
usually correct, and the fix is to attach the artifact rather than to re-run the
judge.

Only add an `**Evidence:**` list to a step when you want to name files beyond
these. Every path in such a list must exist and be non-empty or the step blocks
before the judge runs, which is why the conventional files above are left
implicit.

---

## Step 1: PHASE GOAL - Verify Goal Achievement

**Question:** Does the plan address the stated planning objective? Is the planned approach sound and complete?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated objectives against the produced plan
2. Verify the plan covers all aspects of the planning objective
3. Confirm the approach is feasible and the decomposition is appropriate

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED - Confirm planning goal is met

---

## Step 2: APPROVAL - Verify User Sign-Off

**Question:** Did the user explicitly approve the plan?

**Actions:**
1. Confirm the plan received user approval before scaffolding
2. Verify any user feedback was incorporated
3. Check that the plan was not scaffolded without approval

**Checkpoint class:** operator_attestation

**Checkpoint:** APPROVAL REQUIRED - Confirm user approved the plan

---

## Step 3: STRUCTURE - Verify Festival Integrity

**Question:** Is the scaffolded festival structurally valid?

**Actions:**
1. Run `fest validate` and confirm it passes
2. Verify no `[REPLACE: ...]` markers remain in any document
3. Confirm phases are properly ordered with clear goals

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED - Confirm structure is valid

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Planning goal achieved |
| 2. APPROVAL | [ ] pending | User sign-off |
| 3. STRUCTURE | [ ] pending | Festival integrity |
