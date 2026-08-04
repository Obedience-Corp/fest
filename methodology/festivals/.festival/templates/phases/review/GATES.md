---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Review Phase Gate

This gate verifies the review phase achieved its goal and produced incorporated feedback.

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

## Step 1: PHASE GOAL — Verify Goal Achievement

**Question:** Did the review achieve its stated objective? Were the right things reviewed against the right criteria?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated review objectives against actual review work
2. Verify the review criteria were appropriate for the deliverables examined
3. Confirm the review answered the questions the phase was created to answer

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm review goal is met

---

## Step 2: COVERAGE — Verify All Items Reviewed

**Question:** Were all items in scope examined?

**Actions:**
1. Confirm every item in scope was reviewed
2. Verify no items were skipped or deferred without justification
3. Check that review criteria were applied consistently

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm all items reviewed

---

## Step 3: INCORPORATION — Verify Feedback Applied

**Question:** Was feedback applied to relevant deliverables?

**Actions:**
1. Confirm fixes were applied for accepted findings
2. Verify deferred items have clear justification and tracking
3. Check that the reviewed deliverables reflect the feedback

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm feedback incorporated

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Review goal achieved |
| 2. COVERAGE | [ ] pending | All items reviewed |
| 3. INCORPORATION | [ ] pending | Feedback applied |
