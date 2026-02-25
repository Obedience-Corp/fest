---
# Template metadata (for fest CLI discovery)
id: QUALITY_GATE_FEST_COMMIT
aliases:
  - fest-commit
  - qg-fest-commit
description: Standard quality gate task for committing sequence changes with fest commit

# Fest document metadata (becomes document frontmatter)
fest_type: gate
fest_id: {{ .GateID }}
fest_name: Fest Commit Sequence Changes
fest_parent: {{ .SequenceID }}
fest_order: {{ .TaskNumber }}
fest_gate_type: iterate
fest_autonomy: high
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Gate: Commit Sequence Changes

Commit all changes from this sequence using `fest commit`.

## Pre-Commit

- [ ] All tests pass
- [ ] Linting is clean
- [ ] No debug code or temporary files

## Commit

**Use fest commit** so task references are preserved:

    fest commit -m "<type>: <summary>"

Do NOT use raw `git commit`. The `fest commit` command tags commits for tracking.
