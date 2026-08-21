# Lifecycle Hooks

fest hooks are **named commands** bound to festival lifecycle events (task
start, task complete, sequence complete, phase complete, gate approve). The
approval judge is one hook among them, not a special case.

Related:

- What the judge receives: [hook-evidence-contract.md](./hook-evidence-contract.md)
- Inspect resolution: `fest hooks list` / `fest hooks list --json`
- Learn in-terminal: `fest understand hooks`

## Two primitives

| Primitive | What it is | Controlled by |
| --- | --- | --- |
| **Automation hooks** | Commands on lifecycle events (judge, linters, tests, notifications) | `hooks.*` config + step bindings |
| **Human gates** | Person-required checkpoints | `approval: human-required` on the step |

Human gates are **not** hooks. No `hooks.enabled` / level / per-hook switch can
disable them.

## Declaration layers

Hook **definitions** live only at three layers. Each layer overrides the
previous **by hook name** (whole definition replace; no field-level merge). A
layer that declares nothing inherits everything above it.

| Layer | Location | Format | Scope |
| --- | --- | --- | --- |
| 1. Machine | `~/.obey/fest/config.json` | JSON | every campaign on this machine |
| 2. Festivals | `festivals/.festival/config.yaml` | YAML | every festival in the campaign |
| 3. Festival | `fest.yaml` | YAML | this festival only |

**Default at every layer is empty:** a fresh machine, campaign, or festival
runs no hooks until you declare some.

### Schema

```yaml
hooks:
  enabled: true            # layer-wide switch; most specific layer wins
  levels:                  # per-level switches; default all true
    phase: true
    sequence: true
    task: true
  definitions:
    approval_judge:
      command: ob judge    # required; cwd = festival root
      fail: closed         # closed (default) | open
      timeout: 0           # 0 = no deadline; default 120s, except approval_judge
      enabled: true        # per-hook switch
```

Field notes:

- `command` is the only required definition field.
- `timeout` defaults to 120s, **except for `approval_judge`, which defaults to
  no deadline**. Judges call an LLM and a large checkpoint can legitimately run
  for many minutes; since a timeout fails closed, a wall-clock default there
  would block gates rather than judge them. Set an explicit `timeout` on
  `approval_judge` to opt back into a deadline.
- `definitions` is a map keyed by hook name (unit of layer override and binding).
- `hooks.enabled` and `hooks.levels` resolve most-specific-wins across layers,
  independently of `definitions`.
- Commands are split on whitespace (same model as today's judge exec): no shell
  operators or quoting in the command string.

JSON machine-layer example:

```json
{
  "hooks": {
    "enabled": true,
    "definitions": {
      "lint": {
        "command": "just lint",
        "fail": "open",
        "timeout": "60s"
      }
    }
  }
}
```

### Resolution (operator view)

```
effective definitions = merge least → most specific by name (replace whole def)
enabled  = most_specific_set(hooks.enabled, default true)
levels   = most_specific_set(hooks.levels.*, default true)
runnable(name, level) =
    enabled && levels[level] && definition.enabled
```

Inspect with:

```bash
fest hooks list
fest hooks list --json
```

Differing upper-layer definitions appear under `shadows` so a local override
cannot silently hide an updated default without visibility.

## Step bindings

Definitions alone do nothing. Steps **bind** names to pre/post timing:

### Frontmatter (phase / sequence / task documents)

```yaml
---
fest_type: task
# ...
hooks:
  pre: [lint]
  post: [approval_judge, notify]
  start: # task documents only; runs around task_start
    pre: [anchor]
    post: [announce]
---
```

Bare `pre`/`post` bind around the step's **terminal** verb (`task_complete`,
`sequence_complete`, `phase_complete`, `gate_approve`). The nested `start:`
stage binds around **`task_start`** and is only honored on task documents;
`fest validate` warns when it appears on a goal document.

### GATES.md body markers

```markdown
## Step 2: REVIEW

**Hooks:** post: [approval_judge], pre: [lint]
```

Bindings only reference names. They never set `command`, `fail`, or `timeout`.

### Lifecycle verbs (v1)

| Verb | When |
| --- | --- |
| `task_start` | first transition into work (`fest task in-progress`, a direct completion, or the first `--progress` above zero) |
| `task_complete` | any transition into completed (`fest task completed`, `fest status set ... completed`, a programmatic 100% progress update) |
| `sequence_complete` | sequence completion |
| `phase_complete` | phase completion |
| `gate_approve` | GATES.md blocking approval checkpoint |

Timings: **pre** (before the verb applies) and **post** (after the transition
has already been applied).

`task_start` fires on the **first** transition into work, whichever surface
causes it: `fest task in-progress`, `fest status set ... in_progress`, a direct
completion, or the first `fest task update --progress N` above zero. Reporting
a blocker before work begins leaves the start hook armed. Resuming a task that
was already started or re-marking it in progress never re-fires the hook;
`fest task reset` clears the recorded start, so a reset task re-anchors on its
next start. A `fail: closed`
pre failure blocks the transition and leaves the task untouched. A `fail:
closed` post failure keeps the task started and is recorded in the audit trail
plus a stderr warning.

`task_complete` fires on **every** completion surface, not just `fest task
completed`: `fest status set ... completed` and programmatic 100% progress
updates run the same bindings. Completion hooks fire on the transition into
completed; repeating a 100% progress update on an already-completed task never
re-fires them, while explicitly re-completing via `fest task completed` does
(matching its historical behavior). When an unstarted task is completed
directly, the pre order is `task_complete` pre, then `task_start` pre; after
the transition, `task_start` post runs before `task_complete` post.

Orchestration:

- Sequential, binding-list order (no parallel v1).
- **`fail: closed` pre-hooks:** a non-zero exit / timeout / spawn error
  short-circuits remaining pre-hooks and **blocks the verb** (completion or
  approval is not applied).
- **`fail: closed` post-hooks:** run **after** the verb is already applied.
  Failure short-circuits remaining post-hooks and surfaces an audited warning;
  it does **not** roll back task completion, sequence/phase completion, or gate
  approval.
- `fail: open`: failure is recorded; remaining hooks at that timing still run.
- Undeclared bound names: **skip + warn** (not an error). Scaffolded templates
  may bind `approval_judge` while config is still empty.

## Hook process context

Every executed automation hook receives the lifecycle coordinates so commands
such as `camp buzz post --from-hook` can render task/verb/festival without
scraping the TUI.

1. **Environment.** Extra `FEST_*` variables are appended to the process
   environment (they override inherited values of the same name):

   | Variable | Meaning |
   | --- | --- |
   | `FEST_HOOK=1` | This process is a fest lifecycle hook |
   | `FEST_HOOK_SCHEMA` | `fest.hook.context/v1` |
   | `FEST_HOOK_NAME` | Bound hook name |
   | `FEST_TASK` | Task or goal coordinate (when applicable) |
   | `FEST_VERB` | `task_start`, `task_complete`, `sequence_complete`, `phase_complete`, `gate_approve` |
   | `FEST_LEVEL` | `task`, `sequence`, `phase`, `gate` |
   | `FEST_TIMING` | `pre` or `post` |
   | `FEST_PHASE` | Phase directory name |
   | `FEST_STEP` | 1-based gate step (omitted when 0) |
   | `FEST_FESTIVAL_PATH` | Festival root |
   | `FEST_FESTIVAL` | `metadata.id` from `fest.yaml` when known |

2. **Stdin JSON.** The same fields as a `fest.hook.context/v1` object, one line.
   Empty optional fields are omitted.

Callers that already supply stdin (the approval judge's
`fest.approval.judge/v1` payload) keep that stdin. Lifecycle verbs that used to
pass `nil` now get the context object. Hooks that ignore stdin still see `FEST_*`.

cwd remains the festival root.

## Human gates

```yaml
# frontmatter on the step document, or:
# **Approval:** human-required   (GATES.md marker)
approval: human-required
```

When the loop reaches a human-required step:

1. Automation hooks for that step are skipped (`skipped: human-gate` in audit).
2. `fest next` shows a `HUMAN APPROVAL REQUIRED` banner and **parks** the loop.
3. Clear only with interactive-TTY `fest workflow approve` / `reject`.
4. `fest workflow approve --auto` and `fest workflow judge` **refuse** with an
   error that names the gate.

`fest workflow status --json` includes `human_approval_required: true` on
affected steps.

## Evidence

The request carries **paths, not content**. fest guarantees the listed
deliverables exist and are non-empty, and the approver reads them itself. There
is no embedded-content mode. See
[hook-evidence-contract.md](./hook-evidence-contract.md).

The approval judge protocol remains `fest.approval.judge/v1` (JSON stdin/stdout).

## Audit trail

Hook runs append `wf_hook_run` events to the festival-local ledger:

```text
<festival>/.fest/progress_events.jsonl
```

These are **events**, not campaign-ledger decisions. Judge approve/reject still
records decisions only through the existing judge/approve paths.

## Validate

`fest validate` may emit **non-blocking** warnings for:

- differing layer shadows
- bindings that name undeclared hooks

Warnings never fail the validate exit path by themselves (only errors do).

## Quick operator checklist

1. Declare definitions at the layer you want (machine / festivals / festival).
2. Bind names on the steps that should run them.
3. `fest hooks list` — confirm source layer, enabled state, shadows.
4. Exercise the lifecycle verb (`fest task completed`, gate approve, …).
5. On human-required steps, use a real TTY for approve — never `--auto`.
