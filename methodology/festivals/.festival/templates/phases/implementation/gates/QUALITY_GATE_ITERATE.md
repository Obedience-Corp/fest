---
# Template metadata (for fest CLI discovery)
id: QUALITY_GATE_ITERATE
aliases:
  - review-iterate
  - qg-iterate
description: Standard quality gate task for addressing review findings and iterating

# Fest document metadata (becomes document frontmatter)
fest_type: gate
fest_id: {{ .GateID }}
fest_name: Review Results and Iterate
fest_parent: {{ .SequenceID }}
fest_order: {{ .TaskNumber }}
fest_gate_type: iterate
fest_autonomy: medium
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Gate: Review Results and Iterate

Address all findings from testing and code review. Iterate until the sequence meets quality standards.

## Findings to Address

### From Testing

- [ ] (list findings from testing gate)

### From Code Review

- [ ] (list findings from review gate)

## Iteration

For each finding:

1. Fix the issue
2. Re-run affected tests
3. Verify linting passes

## Definition of Done

- [ ] All critical findings fixed
- [ ] All tests pass after changes
- [ ] Linting passes
- [ ] Code review findings addressed
- [ ] Ready to commit
