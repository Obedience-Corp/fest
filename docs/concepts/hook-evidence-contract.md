# Judge Orientation Contract

> Parent guide: [hooks.md](./hooks.md) (layers, schema, bindings, human gates).

fest does not hand the approval judge the work. It hands the judge **where the
work is**, having already guaranteed the work exists, and the judge reads what
it needs for itself.

## Why orientation and not content

The judge is a tool-capable agent session with a working directory, not a
one-shot completion. It can open files, walk directories, and read a diff. The
whole design rests on that assumption, so it is worth stating plainly rather
than leaving it implied by the schema.

Sending content instead of paths costs something and buys nothing:

- fest has to guess what matters before the judge has looked at anything.
- Every guess is paid for in prompt tokens whether or not it gets read.
- A judge that can only see what fest chose to inline cannot notice what fest
  left out, which is often exactly where a phase went wrong.

So the request is a manifest, not a payload. Paths are cheap, and the judge
spends its own tool calls on the ones that turn out to matter.

## Payload shape

The concrete type is `approvalJudgeRequest` in
`internal/commands/workflow/approve.go` (`schema_version: fest.approval.judge/v1`).

| Field | Always sent | Meaning |
| --- | --- | --- |
| `schema_version` | yes | `fest.approval.judge/v1` |
| `festival_path` | yes | Festival root |
| `phase_path` | yes | Phase directory, and the root every `evidence` path is relative to |
| `document` | yes | `GATES.md` for a gate, `WORKFLOW.md` otherwise |
| `step_number` | yes | 1-based step index |
| `step_name` | yes | Step name, e.g. `VERIFY` |
| `checkpoint` | yes | Checkpoint text when the step has one, else the checkpoint kind |
| `goal` | when set | The step's question or goal |
| `actions` | when set | The step's declared actions |
| `output` | when set | What the step was supposed to produce |
| `evidence` | when non-empty | Phase-relative deliverables that exist and are non-empty |
| `campaign_root` | when detected | Campaign directory, the one absolute path in the request |
| `working_dirs` | when declared | Where the phase's work actually landed |

`document` names a file, it does not carry one. For a `WORKFLOW.md` checkpoint
it is only the step definition, which is why `evidence` exists.

## Evidence is a starting manifest

`evidence` lists phase-relative deliverable files, and every entry is one the
readiness gate already confirmed exists as a non-empty regular file. It is a
starting point, not a boundary: a judge is free to read anything else under
`phase_path`, `festival_path`, or a working dir, and often should.

Resolution rules, applied by `internal/hooks` (`NormalizeEvidencePath`,
`WithinRoot`):

1. Each path is **phase-relative**. Absolute paths are rejected.
2. Paths are cleaned with `filepath.Clean`. Paths that escape the phase
   directory via `..` are rejected.
3. After `filepath.EvalSymlinks`, the candidate must remain contained under the
   resolved phase root. Symlinks that escape are rejected.
4. Only **non-empty regular files** count as present. Empty files and
   directories are dropped.
5. Missing or unreadable paths are dropped from the present set rather than
   aborting the transport.

## The readiness gate runs first

Before any judge process starts, fest checks that the step's declared
deliverables exist and are non-empty. A step that produced nothing blocks
there, deterministically, at zero token cost, and the operator is told which
paths are missing.

This is what makes orientation safe. "Let the judge figure it out" would spend
a model call to discover an empty directory. The gate spends nothing.

The distinction matters when reading the two failure modes:

- **Readiness block** — deterministic, pre-model, no judge invoked. Recorded as
  an agent decision naming the missing paths.
- **Judge rejection** — the judge ran, looked, and decided. Subject to the
  hook's fail policy.

## Working directories

An implementation phase's deliverable is code in another repository. Without it
the judge can only read the executor's own account of what it did, so fest also
sends where the work landed:

```json
{
  "campaign_root": "/campaigns/demo",
  "working_dirs": [
    { "sequence": "01_implement", "path": "projects/camp" }
  ]
}
```

- `path` is **campaign-relative**, taken from each sequence's `fest_working_dir`
  and normalized by `pathutil.NormalizeWorkingDir`. Absolute, `~`-prefixed, and
  `..`-escaping values are rejected.
- Paths stay relative deliberately. A judge request is rendered into a prompt
  that reaches a model provider and is recorded in ledgers and transcripts; an
  absolute path per working dir would put the operator's home directory and
  username into all of them. `campaign_root` is the single absolute path, and
  the judge joins.
- `campaign_root` comes from `workspace.DetectCampaign`, so it honors `CAMP_ROOT`
  and matches the root the rest of fest resolves. Empty when detection fails,
  which leaves the relative paths intact.
- Containment is enforced on the path **string** only. A campaign-relative entry
  that is a symlink out of the campaign still normalizes cleanly, so a judge that
  opens these directories must re-validate before reading, unlike `evidence`,
  which is `EvalSymlinks`-resolved and root-checked here.
- Sequences that declare no working dir are skipped: the work is in the festival
  itself, which the judge already sees through `phase_path`. A declaration that
  was present but rejected is reported on stderr rather than dropped silently.

## Failure semantics

- The transport never aborts solely because an evidence path is missing.
- Read failures during resolution are skipped for that file.
- Fail-closed orchestration still applies at the hook fail-policy layer when a
  hook command itself exits non-zero.

## Compatibility

- The `fest.approval.judge/v1` decision protocol is unchanged.
- `evidence`, `campaign_root`, and `working_dirs` are omitted when empty.
- fest emits no field a judge cannot act on. There is no embedded-content mode:
  a `hooks.definitions.<name>.evidence` key in a config is an unrecognized key
  and is ignored.
