## fest run

Report whether the next slice is leaveable; optionally loop a caller-supplied command

### Synopsis

Inspect the same slice fest next would show and say whether you can leave.

fest run does not launch an agent. Festival is agent-agnostic: the plan and
the loop state live here; whoever executes a slice is the caller's business.

Default (and --dry): classify the next slice, record it, print a report.
Human gates, blocked tasks, and live judges are successful stops.

--exec <command> loops: run that command with the slice on stdin, then
advance. The command can be any worker. Fest does not name or launch a harness.

--status prints the morning report without appending to the ledger.

v1 --exec drives standalone tracked WORKFLOW.md files. Festival tasks are
classified only.

```
fest run [flags]
```

### Examples

```bash
  fest run
  fest run --dry
  fest run --status --json
  fest run --exec ./my-worker --max-tasks 8 --max-minutes 240
```

### Options

```
      --dry              classify the next slice and exit (default when --exec is omitted)
      --exec string      optional worker command; slice prompt is on stdin. omitted: classify only
  -h, --help             help for run
      --json             machine-readable status
      --max-minutes int  stop after this many minutes (with --exec) (default 240)
      --max-tasks int    stop after this many driven slices (with --exec) (default 8)
      --resume           continue the existing ledger (default: always resumes if present)
      --status           print the morning report without driving
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
