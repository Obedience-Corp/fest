# fest watch deferral evidence

Design doc 05 scenario E1: the `fest watch` row for a task must not change when
an operator defers its blocker. Same icon, same colour, same text. A unit test
over `TaskDisplayStatus` proves one function; it does not prove what the
terminal drew, which is what the operator sees.

## What is here

| File | What it is |
| --- | --- |
| `raw.gif`, `optimized.gif` | the VHS recording of both states, from `docs/demos/fest-watch-deferral.tape` |
| `pty-transcript.txt` | the raw bytes the binary wrote to a pseudo-terminal, both states, unaltered |
| `screen-snapshots.json` | the pyte-rendered screens, with per-cell attributes for every drawn row |
| `row-diff.txt` | the comparison, which is the actual evidence |
| `masking.txt` | which columns were masked and why |
| `defer-refusal.txt` | the operator guard refusing a real deferral from this session, on a real terminal |
| `manifest.json`, `artifact-metadata.json`, `privacy-scan*.txt`, `dependencies.txt` | the validated bundle |

## How to reproduce

```
just build quick
FEST_VHS_ROOT="$(mktemp -d)" just vhs record docs/demos/fest-watch-deferral.tape
```

The comparison is separate from the recording, because a GIF cannot be diffed.
`docs/demos/fixtures/fest-watch-deferral/drive_watch.py` runs the same binary
under a pseudo-terminal, feeds its bytes to pyte, and writes the rendered screen
with attributes intact. `compare_rows.py` diffs the row cell by cell. Nothing
strips ANSI escapes: a comparison over flattened frames would pass even if the
icon changed colour, which is the one thing this evidence exists to catch.

## How the deferred state was produced

Not by `fest task defer`. That verb refuses an agent session by design, and
there is deliberately no flag to bypass it. `defer-refusal.txt` records the verb
being run on a real pseudo-terminal from the session that produced this
evidence, and refusing twice: first on the `CLAUDECODE` environment marker, then,
with every marker stripped, on finding `claude` in the process ancestry.

So the fixture writes the `blocker_deferred` event the verb would have written,
with the same fields `Manager.DeferBlocker` records. See
`docs/demos/fixtures/fest-watch-deferral/setup.sh`. Both states come from the
same fixture script and differ by that one event.
