---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Implementation Phase Gate

This gate verifies the implementation phase achieved its goal and produced working deliverables.

---

## Step 1: PHASE GOAL — Verify Goal Achievement

**Question:** Does the implementation satisfy the PHASE_GOAL.md objectives? Were all required deliverables produced?

**Actions:**
1. Re-read PHASE_GOAL.md and compare stated objectives against actual results
2. Verify each required deliverable exists and is functional
3. Confirm the implementation solves the problem the phase was created for

**Checkpoint:** APPROVAL REQUIRED — Confirm phase goal is met

---

## Step 2: SEQUENCE OUTCOMES — Verify Sequence Goals Met

**Question:** Did each sequence achieve its stated goal? Do actual results match each SEQUENCE_GOAL?

**Actions:**
1. Compare each sequence's output against its SEQUENCE_GOAL.md
2. Verify all sequence-level quality gates passed
3. Confirm no sequences were skipped or left incomplete

**Checkpoint:** APPROVAL REQUIRED — Confirm all sequence goals achieved

---

## Step 3: QUALITY — Verify Build and Test Health

**Question:** Does the project build cleanly and do all tests pass with no regressions?

**Actions:**
1. Run the project build command and confirm no errors
2. Run the full test suite and confirm all tests pass
3. Check for regressions introduced during implementation
4. Verify no new warnings or linting issues

**Checkpoint:** APPROVAL REQUIRED — Confirm build and tests are green

---

## Step 4: COMPLETENESS — Verify Nothing Left Behind

**Question:** Are all tasks done, all gates passed, and all review feedback addressed?

**Actions:**
1. Confirm every task is marked complete
2. Verify code review findings were incorporated or explicitly deferred with justification
3. Check that iterate gates resolved all flagged issues

**Checkpoint:** APPROVAL REQUIRED — Confirm completeness

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. PHASE GOAL | [ ] pending | Goal achievement verified |
| 2. SEQUENCE OUTCOMES | [ ] pending | All sequence goals met |
| 3. QUALITY | [ ] pending | Build and tests pass |
| 4. COMPLETENESS | [ ] pending | All tasks and gates done |
