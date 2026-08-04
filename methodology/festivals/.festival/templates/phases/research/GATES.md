---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Research Phase Gate

This gate verifies the research phase achieved its goal and produced actionable findings.

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

**Question:** Were all research questions answered? Do findings satisfy the stated research objective?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated research questions against actual findings
2. Verify each question received findings or an explicit "inconclusive" note
3. Confirm no research questions were silently dropped

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm research goal is met

---

## Step 2: EVIDENCE — Verify Findings Are Documented

**Question:** Are findings documented with sources and confidence levels?

**Actions:**
1. Check that findings reference specific sources
2. Verify confidence levels are noted for conclusions
3. Confirm contradictions or gaps are transparently noted

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm findings are documented

---

## Step 3: ACTIONABILITY — Verify Conclusions Are Usable

**Question:** Are conclusions actionable by downstream phases?

**Actions:**
1. Confirm findings translate into clear recommendations or decisions
2. Verify downstream phases can act on the research without re-investigation
3. Check that open questions are flagged for follow-up

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm conclusions are actionable

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Research goal achieved |
| 2. EVIDENCE | [ ] pending | Findings documented |
| 3. ACTIONABILITY | [ ] pending | Conclusions are usable |
