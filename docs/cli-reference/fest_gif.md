## fest gif

Render a festival's execution as an animated GIF

### Synopsis

Render the festival tree as an animated GIF that replays its execution.
Each task, step, and gate changes state in the order fest recorded it, the
way fest watch shows it live. A gate waiting on the approval judge shows the
judge glyph and "Judge: waiting", then the verdict, including reject and
recheck loops. Each lifecycle hook run appears under the row it fired on. The
last frame matches fest show.

Works on any festival with a progress log, including completed festivals in
the dungeon. The GIF is written to ./<festival>.gif unless --out is given.

```
fest gif [festival] [flags]
```

### Examples

```
  fest gif                          # festival in the current directory
  fest gif my-festival              # by name, from anywhere in a camp
  fest gif --festival DM0001        # by selector
  fest gif -o docs/replay.gif       # choose the output file
```

### Options

```
      --festival string   festival selector (name or ID) from within a camp
  -h, --help              help for gif
  -o, --out string        output file (default ./<festival>.gif)
```

### Options inherited from parent commands

```
      --config string   config file (default: ~/.obey/fest/config.json)
      --debug           enable debug logging
      --no-color        disable colored output
      --verbose         enable verbose output
```

### SEE ALSO

* [fest](fest.md)	 - Festival Methodology CLI - goal-oriented project management for AI agents
