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
fest_gate_type: commit
fest_autonomy: high
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Gate: Commit Sequence Changes

Commit all changes from this sequence using the `fest commit` command.

## Pre-Commit Checklist

- [ ] All tests pass
- [ ] Linting is clean
- [ ] No debug code or temporary files
- [ ] No secrets or credentials in staged changes

## Commit Command

You **MUST** use `fest commit` — not `git commit`. The `fest commit` command tags
commits with task reference IDs for tracking and metrics.

```bash
fest commit -m "<type>: <summary>"
```

**CRITICAL:** Do NOT use `git commit`, `git add && git commit`, or any other git
commit workflow. Always use `fest commit` so task references are preserved.

## Commit Message Format

```
<type>: <concise summary of changes>

<what changed — list concrete modifications>

<why it changed — purpose and motivation>
```

**Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

The message should describe WHAT changed and WHY. Be specific about files,
functions, or features that were added, modified, or removed.

## Ethical Requirements

The following practices are **prohibited** in commit messages:

- NO "Co-authored-by" tags for AI assistants
- NO AI tool attribution or advertisements
- NO links to AI services or products

## Definition of Done

- [ ] Pre-commit checklist verified
- [ ] Commit created with `fest commit` (not `git commit`)
- [ ] Message describes what changed and why
- [ ] No prohibited content in commit message
