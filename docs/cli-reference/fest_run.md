## fest run

Drive fest next until the plan is done, blocked on you, or the cap hits

### Synopsis

Drive the current festival or tracked WORKFLOW.md without babysitting the loop.

fest run inspects the same slice fest next would show. Human gates, blocked
tasks, and live judges stop the run — that stop is a successful night, not a
failure. Successful slices are committed when the working directory is a git
repo. The campaign is never git-reset.

Use --dry to classify the next slice without invoking an agent.
Use --status to print the morning report without appending to the ledger.

v1 drives standalone tracked WORKFLOW.md files. Festival task execution is
classified by --dry; driving those slices is not enabled yet.

```
fest run [flags]
```

### Examples

```bash
  fest run --dry
  fest run --status
  fest run --agent claude --max-tasks 8 --max-minutes 240
  fest run --resume
```

### Options

```
      --agent string     agent binary (claude uses -p; anything else gets the prompt on stdin) (default "claude")
      --dry              classify the next slice and exit
  -h, --help             help for run
      --json             machine-readable status
      --max-minutes int  stop after this many minutes (default 240)
      --max-tasks int    stop after this many driven slices (default 8)
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
