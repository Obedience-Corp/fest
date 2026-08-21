---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Non-Coding Action Phase Gate

This gate verifies the non-coding action phase achieved its goal and produced documented results.

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

**Question:** Was the action completed and does the outcome match the phase goal?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated action objectives against actual outcome
2. Verify the action was performed (not just planned)
3. Confirm the outcome satisfies the phase's success criteria

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED - Confirm phase goal is met

---

## Step 2: DOCUMENTATION - Verify Results Recorded

**Question:** Are results recorded and artifacts saved?

**Actions:**
1. Check that outcomes are documented
2. Verify any artifacts produced are saved in the appropriate location
3. Confirm unexpected results or issues are noted

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED - Confirm results documented

---

## Step 3: FOLLOW-UP - Verify Downstream Dependencies Identified

**Question:** Are downstream dependencies identified?

**Actions:**
1. Note any follow-up work triggered by this action
2. Verify blockers or risks are flagged for later phases
3. Confirm nothing was left unresolved that affects downstream work

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED - Confirm follow-ups identified

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Goal achievement verified |
| 2. DOCUMENTATION | [ ] pending | Results recorded |
| 3. FOLLOW-UP | [ ] pending | Dependencies identified |
