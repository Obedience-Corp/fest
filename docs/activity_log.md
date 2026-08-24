# Activity Log

`activity.jsonl` is the comprehensive event log for every state-mutating `fest` CLI action. It exists at two levels:

## Files

### Festival-level — `<festival>/.fest/activity.jsonl`

Records every fest CLI action that mutates **this specific festival's** state. Co-exists with `progress_events.jsonl` (which only records task/workflow status transitions).

### Campaign-level — `.campaign/fest/activity.jsonl`

Records lifecycle events (festival creation, promotion, phase/sequence start/completion) for campaign-wide orientation. This is the single scrollable timeline of "what happened in this campaign's festival layer over time."

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
| `scope.festival_path_relative` | where applicable | Path relative to campaign root. |
| `scope.phase` / `sequence` / `task` | where applicable | Null if not scoped to that level. |
| `source_cmd` | yes | Verbatim invoked command with sensitive flags redacted. |
| `data` | yes | Event-specific payload. |
| `result.ok` | yes | Boolean. Failed invocations are still logged. |
| `result.error` | no | Error message when `ok: false`. |

### Redaction rules

- `--token`, `--password`, `--secret`, and `--signing-key` flag values are replaced with `<REDACTED>`.
- Arbitrary user messages (e.g. `--reason "..."`) are logged verbatim.

## Event Catalog

### Festival lifecycle (both campaign + festival level)

| Event | Triggered by |
| --- | --- |
| `festival.created` | `fest create festival` |
| `festival.promoted` | Any status transition: `planning` → `ready` → `active` → `completed`/`archived`/`someday` |
| `festival.deleted` | Destructive removal |
| `festival.linked` / `festival.unlinked` | `fest link` / `fest unlink` |
| `festival.renamed` | Rename path |

### Phase / sequence lifecycle (both campaign + festival level)

| Event | Triggered by |
| --- | --- |
| `phase.created` | `fest create phase` |
| `phase.started` / `phase.completed` | Status transitions |
| `sequence.created` | `fest create sequence` |
| `sequence.started` / `sequence.completed` | Status transitions |

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
| `validate.ran` | `fest validate` |
| `next.resolved` | `fest next` |
| `go.navigated` | `fest go <target>` |
| `workflow.skipped` | `fest workflow skip --reason "..."` |
| `commit.made` | `fest commit` |
| `gate.applied` / `gate.skipped` | Gate operations |

### Read-only commands

Pure read-only commands (`fest status`, `fest show plan`, `fest list`, `fest understand`) **do not emit events**.

## Write semantics

- Append-only, `O_APPEND | O_CREATE`.
- `fsync` after every write.
- Advisory file lock (`flock`) prevents interleaved writes from concurrent fest processes.

## Log rotation

- Default: no rotation. JSONL is cheap and disk cost is negligible.
- Safety hatch: if the file exceeds 50 MiB, rotate to `activity.<N>.jsonl` and start fresh.

## Relationship to existing logs

| Log | Scope | What it records |
| --- | --- | --- |
| `activity.jsonl` (festival) | Festival | Every mutating fest action |
| `activity.jsonl` (campaign) | Campaign | Lifecycle events only |
| `progress_events.jsonl` | Festival | Task/workflow status transitions only |
| `festival_events.jsonl` | Festival | Festival create/move/archive/delete only |
| campaign ledger (`.campaign/events/`) | Campaign | High-intent fest actions via camp's `ledgerkit` |

`activity.jsonl` is complementary to the campaign ledger. The ledger is camp's campaign-level record of high-intent decisions; `activity.jsonl` is fest's comprehensive per-festival audit trail.

## Consumer examples

```bash
# All events for a festival, most recent first
tac festivals/active/my-fest-CS0001/.fest/activity.jsonl

# All phase/sequence/task creations
jq 'select(.event | test("(phase|sequence|task)\\.created"))' \
  festivals/active/my-fest-CS0001/.fest/activity.jsonl

# All errors
jq 'select(.result.ok == false)' \
  festivals/active/my-fest-CS0001/.fest/activity.jsonl

# Campaign-wide festival promotions
jq 'select(.event == "festival.promoted")' \
  .campaign/fest/activity.jsonl

# Commits made in a festival
jq 'select(.event == "commit.made")' \
  festivals/active/my-fest-CS0001/.fest/activity.jsonl
```

## Schema versioning

- Schema version is `1`.
- Additive field additions do not bump the version.
- Removed/renamed fields or changed semantics bump from `v: 1` to `v: 2`.
