---
# Template metadata (for fest CLI discovery)
id: QUALITY_GATE_TESTING
aliases:
  - testing-verify
  - qg-test
description: Standard quality gate task for testing and verification

# Fest document metadata (becomes document frontmatter)
fest_type: gate
fest_id: {{ .GateID }}
fest_name: Testing and Verification
fest_parent: {{ .SequenceID }}
fest_order: {{ .TaskNumber }}
fest_gate_type: testing
fest_autonomy: medium
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Gate: Testing and Verification

Run all tests for this sequence. Verify no regressions.

- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] No regressions introduced
- [ ] Build completes without warnings
