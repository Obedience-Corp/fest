# TUI recordings

These are fest's own local VHS tapes, recorded from the real binary against a
disposable fixture. The README hero is not one of them: it comes from the
shared [termcast](https://github.com/Obedience-Corp/termcast) festival content
pack, so that camp, fest, and the Festival docs site all publish one palette
and one fixture. See [Reproduce](#reproduce) below.

| Journey | Tape | Delivery GIF | Manifest |
| --- | --- | --- | --- |
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
#   $FEST_VHS_ROOT/workspace/.campaign/ # project picker only lists inside a campaign
#   $FEST_VHS_ROOT/workspace/projects/demo-app
#   $FEST_VHS_ROOT/workspace/projects/payments-api
# Keep $FEST_VHS_ROOT short (e.g. /tmp/fest-create-demo): the success screen
# prints the festival's absolute path.
FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-menu.tape
FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-festival-wizard.tape
cp out/fest-create-*.gif docs/demos/
```

Use `tui.theme: dark` in the fixture config so adaptive background detection does not
pick a light palette inside VHS/ttyd (which looks uncolored on the dark demo theme).

## Reproduce

The README hero (`docs/images/fest-loop.gif`) is the `fest-loop` tape in the
termcast festival pack. Its seed plans a real festival in a throwaway campaign
under `/tmp/campaigns` with a redirected `HOME`, and builds the newest released
fest tag with no build tags, so the recording is the stable command surface a
reader can actually install:

```sh
cd /path/to/termcast
node bin/termcast.mjs tui fest-loop --width 860
cp out/tui-fest-loop-opt.gif /path/to/fest/docs/images/fest-loop.gif
```

Pass `FEST_SRC=/path/to/fest` to record an unreleased change instead of the
released tag; the build stays tag-free either way.

The local create-TUI tapes above run from a disposable fixture containing the
branch binary and a `fest init` workspace. They write raw output under `out/`;
publish only the optimized GIF, and keep raw recordings and PTY evidence out of
the repository.
