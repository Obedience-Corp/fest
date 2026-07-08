## fest workflow approve

Approve a blocking checkpoint

### Synopsis

Approve a blocking checkpoint and proceed to the next step.

Some workflow steps require explicit user approval before proceeding.
This is typically used for review gates or major decision points.

After approval:
  - The current step is marked as approved
  - The workflow advances to the next step

Auto approval:
  Manual approval is the default. Use --auto only when an operator has explicitly
  delegated this checkpoint decision to an external judge command.

  The judge command receives JSON on stdin using schema fest.approval.judge/v1
  and must return JSON on stdout with decision "approve" or "reject" and a
  reason. Missing commands, timeouts, non-zero exits, malformed JSON, unknown
  decisions, and empty reasons fail closed and do not approve the checkpoint.

  The default judge command is "ob judge". If that command is not installed in
  your Obedience environment, --auto reports the missing dependency and leaves
  the checkpoint unchanged.

```
fest workflow approve [flags]
```

### Options

```
      --auto                         delegate this checkpoint decision to the configured approval judge command
  -h, --help                         help for approve
      --judge-command string         approval judge command used with --auto (default "ob judge")
      --judge-timeout duration       maximum time to wait for the approval judge (default 2m0s)
```

### Options inherited from parent commands

```
      --config string   config file (default: ~/.config/fest/config.json)
      --debug           enable debug logging
      --no-color        disable colored output
      --phase string    specify phase directory (e.g., 001_INGEST)
      --verbose         enable verbose output
```

### SEE ALSO

* [fest workflow](fest_workflow.md)	 - Manage workflow-based phase execution
