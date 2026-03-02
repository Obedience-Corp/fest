---
fest_type: phase_gate
fest_id: [REPLACE: PHASE_ID]-GATE
fest_parent: [REPLACE: PHASE_ID]
---

# Deployment Phase Gate

This gate verifies the deployment phase was completed correctly before advancing.

---

## Step 1: VERIFICATION — Verify Deployment Succeeded

**Question:** Was the deployment executed and verified in the target environment?

**Actions:**
1. Confirm the deployment completed without errors
2. Verify the deployed artifact matches the expected version
3. Check that smoke tests or health checks pass in the target environment

**Checkpoint:** APPROVAL REQUIRED — Confirm deployment verified

---

## Step 2: ROLLBACK — Verify Recovery Path Exists

**Question:** Are rollback procedures documented and tested?

**Actions:**
1. Confirm rollback steps are documented
2. Verify the previous version can be restored if needed
3. Check that rollback criteria (when to trigger) are defined

**Checkpoint:** APPROVAL REQUIRED — Confirm rollback path exists

---

## Step 3: COMMUNICATION — Verify Stakeholders Notified

**Question:** Were relevant stakeholders informed of the deployment?

**Actions:**
1. Confirm deployment status was communicated
2. Verify any known issues or caveats were shared
3. Check that monitoring is in place for the deployed changes

**Checkpoint:** APPROVAL REQUIRED — Confirm stakeholders notified

---

## Gate State Tracking

| Step | Status | Notes |
|------|--------|-------|
| 1. VERIFICATION | [ ] pending | Deployment succeeded |
| 2. ROLLBACK | [ ] pending | Recovery path exists |
| 3. COMMUNICATION | [ ] pending | Stakeholders notified |
