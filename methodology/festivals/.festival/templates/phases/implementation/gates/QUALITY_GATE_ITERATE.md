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

Address all findings from testing and code review. Iterate until clean.

- [ ] All critical findings fixed
- [ ] Tests re-run and pass
- [ ] Linting passes
- [ ] Ready to commit
