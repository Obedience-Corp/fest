---
# Template metadata (for fest CLI discovery)
id: QUALITY_GATE_REVIEW
aliases:
  - code-review
  - qg-review
description: Standard quality gate task for code review

# Fest document metadata (becomes document frontmatter)
fest_type: gate
fest_id: {{ .GateID }}
fest_name: Code Review
fest_parent: {{ .SequenceID }}
fest_order: {{ .TaskNumber }}
fest_gate_type: review
fest_autonomy: low
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Gate: Code Review

Review all changes in this sequence for correctness and standards compliance.

- [ ] Linting passes without warnings
- [ ] Code is readable and follows project conventions
- [ ] No obvious bugs or security issues
- [ ] Changes align with sequence goal

**Findings:** Document any issues that must be addressed before commit.
