---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Implementation Phase Gate

This gate verifies the implementation phase was completed correctly before advancing.

---

## Step 1: COMPLETENESS — Verify All Tasks Done

**Question:** Were all sequences and their tasks completed, including all sequence-level quality gates (testing, review, iterate, commit)?

**Actions:**
1. Confirm every sequence has all tasks marked complete
2. Verify all sequence-level quality gates passed
3. Check that no tasks were skipped or left incomplete

**Checkpoint:** APPROVAL REQUIRED — Confirm all tasks and gates complete

---

## Step 2: QUALITY — Verify Build and Test Health

**Question:** Does the project build cleanly and do all tests pass?

**Actions:**
1. Run the project build command and confirm no errors
2. Run the full test suite and confirm all tests pass
3. Check for any regressions introduced during implementation
4. Verify no new warnings or linting issues

**Checkpoint:** APPROVAL REQUIRED — Confirm build and tests are green

---

## Step 3: REVIEW — Verify Code Review Findings Addressed

**Question:** Were all code review findings from sequence-level review gates addressed?

**Actions:**
1. Confirm review feedback was incorporated or explicitly deferred with justification
2. Verify no critical findings were ignored
3. Check that iterate gates resolved all flagged issues

**Checkpoint:** APPROVAL REQUIRED — Confirm review findings addressed

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. COMPLETENESS | [ ] pending | All tasks and gates done |
| 2. QUALITY | [ ] pending | Build and tests pass |
| 3. REVIEW | [ ] pending | Review findings addressed |
