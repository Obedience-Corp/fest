---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Ingest Phase Gate

This gate verifies the ingest phase was completed correctly before advancing.

---

## Step 1: COMPLETENESS — Verify All Inputs Processed

**Question:** Were all input specifications read completely and not skimmed?

**Actions:**
1. Confirm every file in `input_specs/` was read
2. Verify no inputs were overlooked or partially processed
3. Check that ambiguities and questions were noted

**Checkpoint:** APPROVAL REQUIRED — Confirm all inputs processed

---

## Step 2: ACCURACY — Verify Output Specs Capture Intent

**Question:** Do the output specifications accurately capture the user's intent?

**Actions:**
1. Check that `output_specs/` documents exist (purpose, requirements, constraints, context)
2. Verify the structured output matches the original input meaning
3. Confirm interpretive decisions are documented

**Checkpoint:** APPROVAL REQUIRED — Confirm output specs are accurate

---

## Step 3: APPROVAL — Verify User Validated Output

**Question:** Did the user review and approve the structured output?

**Actions:**
1. Confirm the PRESENT step received user approval
2. Verify any user corrections were incorporated
3. Check that requirements are clear enough for downstream planning

**Checkpoint:** APPROVAL REQUIRED — Confirm user validated output

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. COMPLETENESS | [ ] pending | All inputs processed |
| 2. ACCURACY | [ ] pending | Output specs capture intent |
| 3. APPROVAL | [ ] pending | User validated output |
