---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Ingest Phase Gate

This gate verifies the ingest phase achieved its goal and produced approved structured output.

## Before you submit any step below

The judge reads what this phase wrote down. It cannot browse your repositories,
and on a provider without tool access it cannot open anything at all, so work
that is real but only visible in a working directory does not reach it.

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

**Question:** Does the structured output capture the user's intent as specified in the ingest objective?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated ingest objectives against produced output
2. Verify the structured output faithfully represents the original input meaning
3. Confirm interpretive decisions are documented and justified

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm ingest goal is met

---

## Step 2: COMPLETENESS — Verify All Inputs Processed

**Question:** Were all input specifications processed?

**Actions:**
1. Confirm every file in `input_specs/` was read completely
2. Verify no inputs were overlooked or partially processed
3. Check that ambiguities and questions were noted

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Confirm all inputs processed

---

## Step 3: APPROVAL — Verify User Validated Output

**Question:** Did the user validate the structured output?

**Actions:**
1. Confirm the user reviewed and approved the output specifications
2. Verify any user corrections were incorporated
3. Check that requirements are clear enough for downstream planning

**Checkpoint class:** operator_attestation

**Checkpoint:** APPROVAL REQUIRED — Confirm user validated output

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Ingest goal achieved |
| 2. COMPLETENESS | [ ] pending | All inputs processed |
| 3. APPROVAL | [ ] pending | User validated output |
