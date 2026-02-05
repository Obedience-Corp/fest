---
# Template metadata (for fest CLI discovery)
id: TASK_TEMPLATE
description: Standard task template

# Fest document metadata
fest_type: task
fest_id: [REPLACE: TASK_ID]
fest_name: [REPLACE: Task Name]
fest_parent: [REPLACE: SEQUENCE_ID]
fest_order: [REPLACE: N]
fest_status: pending
fest_autonomy: [REPLACE: high|medium|low]
fest_tracking: true
fest_created: { { .created_date } }
---

# Task: [REPLACE: Task Name]

## Objective

[REPLACE: One sentence describing what this task accomplishes]

## Requirements

- [ ] [REPLACE: Requirement 1]
- [ ] [REPLACE: Requirement 2]

## Implementation

[REPLACE: How to implement - steps, code locations, patterns to follow, actual code for each location, assume this document will be followed like a tutorial]

## Done When

- [ ] All requirements met
- [ ] [REPLACE: Specific verification criterion]
