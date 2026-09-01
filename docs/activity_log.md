# Activity Log

`activity.jsonl` records the wired mutating `fest` CLI actions listed in the Event Catalog. It exists at two levels. Commands that are not in the catalog (including `fest link` / `unlink`, `fest gates apply`, and most other mutators) do not emit.

## Files

### Festival-level — `<festival>/.fest/activity.jsonl`

Records the live events for **this specific festival**. Co-exists with `progress_events.jsonl` (which only records task/workflow status transitions).

### Camp-level — `.campaign/fest/activity.jsonl`

Records DestBoth lifecycle events actually emitted today (`festival.created`, `festival.promoted`, `phase.created`, `sequence.created`) for camp-wide orientation. This is the scrollable timeline of those lifecycle events in this camp's festival layer.

## Schema

JSONL: one JSON object per line, newline-terminated, append-only.

```json
{
  "v": 1,
  "ts": "2026-04-12T19:42:11.123456789Z",
  "event": "phase.created",
  "actor": {
    "user": "lancekrogers",
    "host": "workstation.local",
    "fest_version": "0.14.2"
  },
  "scope": {
    "campaign_root": "/Users/lance/Dev/AI/obey-campaign",
    "festival_id": "CS0003",
    "festival_name": "corp-site-build",
    "festival_path_relative": "festivals/active/corp-site-build-CS0003",
    "phase": "002_PLAN_SITE_CONTENT",
    "sequence": null,
    "task": null
  },
  "source_cmd": "fest create phase --name PLAN_SITE_CONTENT --type planning",
  "data": {
    "phase_type": "planning",
    "phase_order": 2,
    "created_path": "festivals/active/corp-site-build-CS0003/002_PLAN_SITE_CONTENT"
  },
  "result": {
    "ok": true,
    "error": null
  }
}
```

### Field conventions

| Field | Required | Notes |
| --- | --- | --- |
| `v` | yes | Schema version. Integer. Starts at `1`. |
| `ts` | yes | RFC3339Nano UTC. |
| `event` | yes | Dotted namespace: `<noun>.<verb>`. See Event Catalog. |
| `actor.user` | yes | `$USER` at emission time. |
| `actor.host` | yes | Hostname. |
| `actor.fest_version` | yes | `fest --version`. |
| `scope.campaign_root` | yes | Absolute path. |
| `scope.festival_id` | where applicable | e.g. `CS0003`. |
| `scope.festival_name` | where applicable | Human-readable festival name. |
| `scope.festival_path_relative` | where applicable | Path relative to camp root. |
| `scope.phase` / `sequence` / `task` | where applicable | Null if not scoped to that level. |
| `source_cmd` | yes | Verbatim invoked command with sensitive flags redacted. |
| `data` | yes | Event-specific payload. |
| `result.ok` | yes | Boolean. Commands emit only after a successful mutation, so production events have `ok: true`. Fail-path logging (`result.ok: false` via `WithError`) is not wired. |
| `result.error` | no | Error message when `ok: false`. Unused on current command paths. |

### Redaction rules

- `--token`, `--password`, `--secret`, and `--signing-key` flag values are replaced with `<REDACTED>`.
- Arbitrary user messages (e.g. `--reason "..."`) are logged verbatim.

## Event Catalog

Only events in this table are emitted. Commands emit after a successful mutation; they do not emit on error returns.

### Festival lifecycle (both camp + festival level)

| Event | Triggered by |
| --- | --- |
| `festival.created` | `fest create festival` |
| `festival.promoted` | Status transition: `planning` → `ready` → `active` → `completed`/`archived`/`someday` |

### Phase / sequence lifecycle (both camp + festival level)

| Event | Triggered by |
| --- | --- |
| `phase.created` | `fest create phase` |
| `sequence.created` | `fest create sequence` |

### Granular scaffolding (festival-level only)

| Event | Triggered by |
| --- | --- |
| `task.created` | `fest create task` |
| `task.completed` | Task completion |
| `task.blocked` | `fest task blocked` |
| `task.reset` | Task reset |

### Operations (festival-level only)

| Event | Triggered by |
| --- | --- |
| `validate.ran` | `fest validate` (emitted after the checks run; `data.ok` is the validation result, not a command failure) |
| `next.resolved` | `fest next` |
| `go.navigated` | `fest go <target>` |
| `workflow.skipped` | `fest workflow skip --reason "..."` |

### Read-only commands

Pure read-only commands (`fest status`, `fest show plan`, `fest list`, `fest understand`) **do not emit events**.

### Future events (not emitted)

These names are reserved. No command in this PR writes them.

| Event | Planned trigger |
| --- | --- |
| `festival.deleted` / `festival.renamed` | Destructive removal / rename |
| `festival.linked` / `festival.unlinked` | `fest link` / `fest unlink` |
| `phase.started` / `phase.completed` / `phase.deleted` | Phase status transitions / removal |
| `sequence.started` / `sequence.completed` / `sequence.deleted` | Sequence status transitions / removal |
| `task.started` / `task.deleted` / `task.renamed` | Task start / removal / rename |
| `gate.applied` / `gate.skipped` | `fest gates apply` and related |
| `init.ran` / `tui.action` | `fest init` / TUI mutations |
| `commit.made` | `fest commit`; reserved until its persistence contract can record the final SHA without leaving a generated-only change after every commit |

## Write semantics

- Append-only, `O_APPEND | O_CREATE`.
- `fsync` after every write.
- Advisory file lock (`flock`) prevents interleaved writes from concurrent fest processes.

## Log rotation

- Default: no rotation. JSONL is cheap and disk cost is negligible.
- Safety hatch: if the file exceeds 50 MiB, close and unlock it, rename it to `activity.<N>.jsonl`, then create a new canonical `activity.jsonl` and write the triggering event there.

## Relationship to existing logs

| Log | Scope | What it records |
| --- | --- | --- |
| `activity.jsonl` (festival) | Festival | Live catalog events for that festival |
| `activity.jsonl` (camp) | Camp | DestBoth lifecycle events only |
| `progress_events.jsonl` | Festival | Task/workflow status transitions only |
| `festival_events.jsonl` | Festival | Festival create/move/archive/delete only |
| camp ledger (`.campaign/events/`) | Camp | High-intent fest actions via camp's `ledgerkit` |

`activity.jsonl` is complementary to the camp ledger. The ledger is Camp's record of high-intent decisions at the camp layer; `activity.jsonl` is fest's per-festival audit trail for the wired catalog events.

## Consumer examples

```bash
# All events for a festival, most recent first
tac festivals/active/my-fest-CS0001/.fest/activity.jsonl

# All phase/sequence/task creations
jq 'select(.event | test("(phase|sequence|task)\\.created"))' \
  festivals/active/my-fest-CS0001/.fest/activity.jsonl

# result.ok == false (none today; fail-path logging is not wired)
jq 'select(.result.ok == false)' \
  festivals/active/my-fest-CS0001/.fest/activity.jsonl

# Camp-wide festival promotions
jq 'select(.event == "festival.promoted")' \
  .campaign/fest/activity.jsonl

```

## Schema versioning

- Schema version is `1`.
- Additive field additions do not bump the version.
- Removed/renamed fields or changed semantics bump from `v: 1` to `v: 2`.
