# Dungeon

Holding area for work that's finished, archived, or deferred.

## Purpose

The dungeon contains three categories of "done" work:

- **completed/** - Successfully finished, kept for reference
- **archived/** - Preserved for history, truly done
- **someday/** - Deprioritized but might revisit

## Structure

```
dungeon/
├── OBEY.md           # This file
├── completed/        # Reference material
├── archived/         # Historical archive
└── someday/          # Low priority backlog
```

## Workflow

### Finishing Work
```bash
camp flow move some-item dungeon/completed   # Done, might reference
camp flow move some-item dungeon/archived    # Done, won't need again
```

### Deferring Work
```bash
camp flow move some-item dungeon/someday     # Not now, maybe later
```

### Reviving Work
```bash
camp flow move old-item active    # Back to work
camp flow move old-item ready     # Queue it up
```

## Reviewing Items

Run the interactive crawl to review dungeon contents:
```bash
camp dungeon crawl
```

## Best Practices

1. **completed/** - For things you might reference (successful experiments, shipped features)
2. **archived/** - For true history (old versions, superseded designs)
3. **someday/** - Be honest - if it's been here 6+ months, consider archiving
4. Review periodically with `camp dungeon crawl`
