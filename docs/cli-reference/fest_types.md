## fest types

Discover types for `fest create`

### Synopsis

List festival, phase, sequence, and task types available for create.

Festival workflow types (standard, implementation, research, ritual) come from
`festival_types.yaml`. Phase scaffold types come from the methodology templates
tree under `festivals/.festival/templates/phases/` (also
`~/.obey/fest/festivals/.festival/templates` after `fest system sync`).

With no subcommand, behaves like `fest types list`.

Examples:
```bash
  fest types                             # Same as fest types list
  fest types list --level festival       # Values for create festival --type
  fest types list --level phase          # Values for create phase --type
  fest types show standard               # Festival workflow type details
  fest types show implementation --level phase
  fest types festival                    # Festival workflow types (alias)
```

### Options

```
  -h, --help   help for types
```

### Options inherited from parent commands

```
      --config string   config file (default: ~/.obey/fest/config.json)
      --debug           enable debug logging
      --no-color        disable colored output
      --verbose         enable verbose output
```

### SEE ALSO

* [fest types list](fest_types_list.md)	 - List available types
* [fest types show](fest_types_show.md)	 - Show details about a type
* [fest types festival](fest_types_festival.md)	 - Discover festival workflow types
