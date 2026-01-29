---
# Template metadata (for fest CLI discovery)
id: phase-goal-ingest
aliases:
  - pgi
  - ingest-phase
description: Phase goal template for ingest phases that transform unstructured input into structured specifications
phase_type: ingest

# Fest document metadata (becomes document frontmatter)
fest_type: phase
fest_id: [REPLACE: PHASE_ID]
fest_name: [REPLACE: Phase Name]
fest_parent: [REPLACE: FESTIVAL_ID]
fest_order: [REPLACE: N]
fest_phase_type: ingest
fest_status: pending
fest_tracking: true
fest_created: {{ .created_date }}
---

# Phase Goal: [REPLACE: Phase name like 001_INGEST]

**Phase:** [REPLACE: Phase ID] | **Status:** Pending | **Type:** Ingest

## Phase Objective

**Primary Goal:** [REPLACE: What this ingest phase will transform and why]

**Context:** [REPLACE: Where the input came from and how the structured output will be used]

## Input Sources

Place all raw input materials in `input_specs/`:

- [ ] [REPLACE: Input source 1 - e.g., user requirements document]
- [ ] [REPLACE: Input source 2 - e.g., reference materials]
- [ ] [REPLACE: Additional sources as needed]

## Expected Outputs

The following structured documents will be created in `output_specs/`:

| Output | Purpose |
|--------|---------|
| `purpose.md` | Festival purpose, success criteria, motivation |
| `requirements.md` | Prioritized requirements (P0/P1/P2) with traceability |
| `constraints.md` | Technical and process constraints |
| `context.md` | Prior art, related systems, key references |

## Success Criteria

This ingest phase is complete when:

- [ ] All input sources reviewed and understood
- [ ] Output specs created following standard structure
- [ ] User has approved the structured output
- [ ] No unresolved questions or ambiguities

## Workflow

This phase uses step-based workflow guidance. See `WORKFLOW.md` for the step-by-step process.

Use `fest next` to see the current step.
Use `fest workflow advance` to move to the next step.

## Notes

[REPLACE: Any assumptions, constraints, or open items]

---

*Ingest phases transform unstructured input into structured specifications ready for planning.*
