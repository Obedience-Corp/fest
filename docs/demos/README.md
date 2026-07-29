# Canonical TUI recording

The explorer demo is produced from the real Fest binary in a disposable PTY
using the shared Obedience Corp fire palette. The committed GIF is the
optimized delivery artifact; raw GIF, clean-fixture rerun, PTY transcript,
frame captures, and build details remain in the private VHS evidence bundle.

| Journey | Tape | Delivery GIF | Manifest |
| --- | --- | --- | --- |
| Explorer V3 | [fest-explorer-v3.tape](fest-explorer-v3.tape) | [README embed](../../README.md) | [fest-explorer-v3.manifest.json](fest-explorer-v3.manifest.json) |
| Create what? menu | [fest-create-menu.tape](fest-create-menu.tape) | [fest-create-menu.gif](fest-create-menu.gif) | — |
| Festival create wizard | [fest-create-festival-wizard.tape](fest-create-festival-wizard.tape) | [fest-create-festival-wizard.gif](fest-create-festival-wizard.gif) | — |

### Create TUI demos

Human create surface (WI-7acd71 / design `fest-create-tui-2026-07-28`):

![Create what? menu](fest-create-menu.gif)

**`fest create`** — outer menu with Festival, Standalone workflow (stub), Phase, Sequence, Task.

![Festival wizard](fest-create-festival-wizard.gif)

**`fest create festival`** — multi-step wizard: type (from config) → identity → project → seed → tags → confirm → human next steps.

```sh
# Record create demos (disposable fixture with branch binary + fest init workspace)
# Fixture layout:
#   $FEST_VHS_ROOT/bin/fest
#   $FEST_VHS_ROOT/config/config.json   # copy fixtures/fest-create-tui/config.json (theme: dark)
#   $FEST_VHS_ROOT/workspace/           # fest init .
FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-menu.tape
FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-festival-wizard.tape
cp out/fest-create-*.gif docs/demos/
```

Use `tui.theme: dark` in the fixture config so adaptive background detection does not
pick a light palette inside VHS/ttyd (which looks uncolored on the dark demo theme).

The private evidence run is `fest/fest-explorer-v3/20260719T105500Z`. Its
manifest records source revision `e4b491d`, artifact hashes/metadata, the
12-state PTY/pyte matrix, privacy PASS, and the secret-Gist handoff.

## Reproduce

Use a disposable fixture containing the branch binary and explorer state, then
run the real VHS tape from the repository root:

```sh
FEST_VHS_ROOT=/path/to/fest-explorer-v3-fixture just vhs record docs/demos/fest-explorer-v3.tape
```

The tape sets a fake config directory and terminal identity. It writes raw
output under `out/`; keep raw recordings and PTY evidence in the private
bundle, and publish only the optimized GIF after the privacy scan passes.
