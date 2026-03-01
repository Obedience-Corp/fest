# Festival Lifecycle

Complete reference for festival status directories, transitions, and lifecycle management.

## Status Directories

Festivals progress through status directories within the `festivals/` workspace:

| Directory | Purpose | Promote Target | Can Transition To |
|-----------|---------|---------------|-------------------|
| `planning/` | Being designed and planned | `ready/` | `ready/`, `dungeon/archived/`, `dungeon/someday/` |
| `ready/` | Validated, waiting to begin | `active/` | `active/`, `planning/`, `dungeon/archived/`, `dungeon/someday/` |
| `active/` | Currently executing | `completed` | `dungeon/completed/`, `dungeon/archived/`, `dungeon/someday/` |
| `ritual/` | Repeatable festival templates | N/A (use `fest ritual run`) | `active/` (via copy, not move) |
| `dungeon/completed/` | Successfully finished | Terminal | N/A |
| `dungeon/archived/` | Archived for reference | Terminal | `planning/` (re-activate) |
| `dungeon/someday/` | Deprioritized / maybe later | Terminal | `planning/` (re-activate) |

### Canonical Source

The authoritative list of status directories is defined in source code:

```go
// internal/id/generator.go
var StatusDirectories = []string{
    "planning", "ready", "active", "ritual",
    "dungeon/completed", "dungeon/archived", "dungeon/someday",
}
```

## Promote Command

`fest promote` advances festivals through the primary lifecycle path:

```
planning -> ready -> active -> completed
```

Each step is defined in `validTransitions`:

| Current Status | Promote Target |
|---------------|----------------|
| `planning` | `ready` |
| `ready` | `active` |
| `active` | `completed` (stored in `dungeon/completed/YYYY-MM/`) |

### Usage

```bash
# Promote current festival to the next status
fest promote

# Force promotion (skip validation)
fest promote --force

# JSON output
fest promote --json
```

### Validation

Before promoting, `fest promote` may validate readiness:

- **planning -> ready**: Checks for `FESTIVAL_GOAL.md` existence
- **ready -> active**: Checks festival structure is valid
- **active -> completed**: Standard move (no additional validation)

Use `--force` to skip validation checks.

## Status Set Command

`fest status set` allows moving a festival to any valid status, not just the next promote target:

```bash
# Move to a specific status
fest status set ready
fest status set active
fest status set completed

# Dungeon sub-statuses
fest status set dungeon/archived
fest status set dungeon/someday

# Bare "dungeon" prompts for sub-status selection
fest status set dungeon

# Force (skip transition validation and confirmation)
fest status set --force active
```

## Date-Based Completion

When a festival reaches `completed` status (via promote or status set), it is organized by month:

```
dungeon/completed/
├── 2026-01-15/
│   └── my-january-festival-MJ0001/
├── 2026-02-28/
│   ├── api-refactor-AR0001/
│   └── ui-redesign-UR0001/
└── ...
```

The date directory uses `YYYY-MM-DD` format based on the timestamp. This is handled by `CalculateDateDir()` in `internal/commands/status/date_directory.go`. All dungeon statuses (`completed`, `archived`, `someday`) use date-based directories.

### Implementation Details

1. `AtomicStatusChange()` detects `dungeon/*` statuses
2. `CalculateDateDir(time.Now())` generates the YYYY-MM-DD string
3. `MoveToDateDirectory()` moves the festival to `dungeon/<status>/YYYY-MM-DD/`
4. If `os.Rename` fails (cross-filesystem), falls back to copy+delete

## Ritual Workflow

Ritual festivals are repeatable templates that live in `ritual/` permanently:

1. **Create**: `fest create festival --type ritual` places the festival in `ritual/`
2. **Run**: `fest ritual run <name>` copies the template to `active/` with a hex counter suffix
3. **Complete**: Each run follows the normal lifecycle (active -> completed)

The ritual template in `ritual/` is never moved. See `docs/ritual.md` for complete ritual documentation.

## Status History

Every status change is recorded in `.fest/status_history.json`:

```json
[
  {
    "timestamp": "2026-02-10T14:30:00Z",
    "from_status": "planning",
    "to_status": "ready",
    "note": ""
  },
  {
    "timestamp": "2026-02-11T09:00:00Z",
    "from_status": "ready",
    "to_status": "active",
    "note": ""
  }
]
```

View history with:

```bash
fest status history
fest status history --limit 5
fest status history --json
```
