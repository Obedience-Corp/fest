# Hook Evidence Contract (read-by-approver)

> Parent guide: [hooks.md](./hooks.md) (layers, schema, bindings, human gates).

Default evidence mode for fest hooks is **read-by-approver** (`evidence: paths`).
The transport carries paths; the approver (judge or human) reads files itself.

## Payload shape

A hook stdin request typically includes:

- `festival_path` — festival root
- `phase_path` — phase directory used as the evidence root
- `evidence` — list of phase-relative file paths (present, non-empty regular files only)

The concrete judge request type is `approvalJudgeRequest` in
`internal/commands/workflow/approve.go` (`schema_version: fest.approval.judge/v1`).
That schema is **not** version-bumped for this contract; path mode remains
identical to the pre-hooks-system judge path.

## Resolution rules

Shared helper: `internal/hooks` (`NormalizeEvidencePath`, `WithinRoot`,
`ResolvePhaseRelative`).

1. Each path is **phase-relative**. Absolute paths are rejected.
2. Paths are cleaned with `filepath.Clean`. Paths that escape the phase directory
   via `..` are rejected.
3. After `filepath.EvalSymlinks`, the candidate must remain contained under the
   resolved phase root. Symlinks that escape are rejected.
4. Only **non-empty regular files** count as present. Empty files and directories
   are dropped.
5. Missing or unreadable paths are **dropped** from the present set, not a
   transport-level abort. Whether missing evidence rejects approval is the
   approver's rubric decision.

## Failure semantics

- Transport never aborts solely because an evidence path is missing.
- Read failures during resolution/embed are skipped for that file.
- Fail-closed orchestration still applies at the hook fail-policy layer when a
  hook command itself exits non-zero.

## Opt-in embed mode

When a hook definition sets `evidence: embed`, fest may also attach
`evidence_files` on the request (additive JSON field; older judges ignore it):

- Total content budget: **256KB** across all embedded files
  (`hooks.EvidenceEmbedCapBytes`).
- The file that crosses the budget is truncated and marked
  `truncated: true` with a `[TRUNCATED: ...]` marker.
- Files after the budget is exhausted are **not** embedded; their paths still
  appear in `evidence`.
- `evidence` paths are always populated so path-mode judges keep working.

## Compatibility

- `fest.approval.judge/v1` decision protocol is unchanged.
- Default `evidence: paths` marshals without `evidence_files`.
- Legacy `hooks.approval_judge.command` continues to use path mode with
  `timeout: 0`.
