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

That assumption is not universally true, and the consumer side checks it. See
[When the judge cannot inspect](#when-the-judge-cannot-inspect) below: fest
always sends orientation, but `ob judge` falls back to inlining content for
providers that get no tools.

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
| `campaign_root` | when detected | Camp root; the root the camp-relative `working_dirs` join against |
| `working_dirs` | when declared | Where the phase's work actually landed |

`document` names a file, it does not carry one. For a `WORKFLOW.md` checkpoint
it is only the step definition, which is why `evidence` exists.

## Verdict shape

The judge writes JSON on stdout. Required fields are unchanged
(`schema_version`, `decision`, `reason`). `evidence_status` is additive so a
stale binary is diagnosable from the ledger instead of looking like every
other reject:

| Field | Required | Meaning |
| --- | --- | --- |
| `schema_version` | yes | `fest.approval.judge/v1` |
| `decision` | yes | `approve` or `reject` |
| `reason` | yes | Free-text justification |
| `evidence_status` | no | `none` (loaded no deliverables), `insufficient` (loaded files that did not support approval), `ok` (loaded files) |

`ob judge` overlays `evidence_status` from the files it actually loaded. It
does not trust a model-authored value. fest records the status on the
approval audit string *before* `reason=` so `DisplayFeedback` can still
unquote the reason.

When fest sent a non-empty `evidence` list and the verdict omits
`evidence_status`, fest warns on stderr that the binary may predate the
evidence field (the OC0001 stale-`ob` failure mode). When fest sent evidence
and the verdict is `approve` with `evidence_status=none`, fest fails closed:
a judge that attests it saw nothing cannot verify the claim.

## Evidence is a starting manifest

`evidence` lists phase-relative deliverable files, and every entry is one the
readiness gate already confirmed exists as a non-empty regular file. It is a
starting point, not a boundary: a judge is free to read anything else under
`phase_path`, `festival_path`, or a working dir, and often should.

Resolution rules, applied by `hooks.ResolvePhaseRelative` (which composes
`NormalizeEvidencePath` and `WithinRoot`):

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

- `path` is **camp-relative**, taken from each sequence's `fest_working_dir`
  and normalized by `pathutil.NormalizeWorkingDir`. Absolute, `~`-prefixed, and
  `..`-escaping values are rejected.
- Paths stay relative deliberately. A judge request is rendered into a prompt
  that reaches a model provider and is recorded in ledgers and transcripts; an
  absolute path per working dir would repeat the operator's home directory and
  username across all of them. One absolute root plus relative entries keeps
  that to a single occurrence, and the judge joins.
- To be precise about what is absolute: `campaign_root`, `festival_path`, and
  `phase_path` are all absolute, because each is a filesystem location fest
  resolved. What stays relative is the *repeated* material, `working_dirs[].path`
  and `evidence[]`, which is where per-entry absolute paths would multiply.
- `campaign_root` comes from `workspace.DetectCampaign`, so it honors `CAMP_ROOT`
  and matches the root the rest of fest resolves. Empty when detection fails,
  which leaves the relative paths intact.
- Containment is enforced on the path **string** only. A camp-relative entry
  that is a symlink out of the camp still normalizes cleanly, so a judge that
  opens these directories must re-validate before reading, unlike `evidence`,
  which is `EvalSymlinks`-resolved and root-checked here.
- Sequences that declare no working dir are skipped: the work is in the festival
  itself, which the judge already sees through `phase_path`. A declaration that
  was present but rejected is reported on stderr rather than dropped silently.

## What the audit trail can and cannot tell you

Inlining had one virtue nobody designed for: the judge's inputs were fixed, so
the prompt was derivable from the request and the record showed exactly what a
decision rested on.

An inspecting judge chooses what to open. Two runs of the same checkpoint can
consult different files, and nothing fest observes distinguishes them. That is a
real cost of this contract, accepted deliberately rather than absorbed quietly.

What the ledger records, on `wf_judge_started`:

| Field | Meaning |
| --- | --- |
| `judge_evidence_offered` | deliverable paths the judge was **pointed at** |
| `judge_working_dirs_offered` | working dirs the judge was **pointed at** |

**Offered, not read.** The names are deliberate. fest knows what it put in the
prompt; it does not know what the judge opened, and a field named `read` would
claim provenance the record does not have.

Two rejected alternatives, so the choice is not relitigated from scratch:

- **Have the judge report what it read.** Self-reported by the component under
  audit. A judge that fabricates an inspection fabricates the list too, so it
  reads like provenance while being model-authored. Worse than recording less.
- **Capture real tool calls from the daemon session.** The only unfakeable
  option, since the daemon sees actual invocations. Also the most expensive, and
  it couples the judge audit trail to session internals. Worth revisiting if a
  disputed verdict ever needs adjudicating; not worth paying for in advance.

The offer is recorded at launch rather than on return, so a judge that crashes
or times out still leaves behind what it was asked to look at.

## When the judge cannot inspect

Orientation assumes the judge can open a file. That is a property of the
provider, not of fest, and it is not universally true: only CLI-backed providers
(`claude-code`, `codex`, `grok`) get a populated tool set for the judge session.
A chat provider such as `ollama` is handed paths it has no way to reach.

Measured behavior on that path was not a clean failure. The model approved with
high confidence while inventing the file's contents, which is worse than a
rejection: it advances unreviewed work behind a confident-looking record.

**fest's side of the contract does not change.** It always sends orientation.
The check lives in the consumer, `ob judge`, which selects on
`config.ProviderUsesBinary`:

| provider kind | what the judge receives |
| --- | --- |
| CLI-backed | the path manifest described above, no content |
| everything else | deliverable content inlined, under per-file and aggregate byte caps |

Two consequences worth knowing:

- A camp configured for `ollama` never exercises the orientation path. Any
  validation done only against the configured default is validating the
  fallback.
- The fallback is not the retired `evidence: embed` knob returning. It is not
  configurable and there is no key to set. Capability decides, not preference.

Implementers of a different judge consumer should make the same check rather
than assuming the manifest is openable.

## Failure semantics

- The transport never aborts solely because an evidence path is missing.
- Read failures during resolution are skipped for that file.
- Fail-closed orchestration still applies at the hook fail-policy layer when a
  hook command itself exits non-zero.

## Compatibility

- The `fest.approval.judge/v1` decision protocol is unchanged: `decision` and
  `reason` remain required.
- `evidence_status` on the verdict is additive. An older judge omits it, and
  fest warns rather than failing closed, except `approve` + `evidence_status=none`
  when evidence was sent (see [Verdict shape](#verdict-shape)).
- `evidence`, `campaign_root`, and `working_dirs` are omitted when empty.
- fest emits no field a judge cannot act on. There is no embedded-content mode:
  a `hooks.definitions.<name>.evidence` key in a config is an unrecognized key
  and is ignored.
