# Commit Reference Format

This document describes the standardized commit reference format used by `fest commit` and other Obey workflow tools.

## Format Specification

```
[OBEY-{COMPONENT}-{ID}]
```

### Components

| Component | Code | Description |
|-----------|------|-------------|
| Festival  | `FE` | Festival methodology workflow tool |

*Additional components will be added as the Obey ecosystem expands.*

### ID Types

The `{ID}` portion varies by context:

| ID Type | Format | Example | Source |
|---------|--------|---------|--------|
| Task Reference | `FEST-xxxxxx` | `FEST-a3b2c1` | `fest_ref` field in task frontmatter |
| Festival ID | `XX0000` | `CS0001` | `metadata.id` field in fest.yaml |

### Full Examples

```
[OBEY-FE-CS0001] Add user authentication
[OBEY-FE-FEST-a3b2c1] Fix login validation bug
[OBEY-FE-TODO0001] Update documentation
```

## Detection Priority

When `fest commit` generates a reference, it follows this priority:

1. **Explicit `--task` flag**: Use the provided task reference directly
2. **Task frontmatter**: If running inside a festival task directory, use the task's `fest_ref`
3. **Festival metadata**: Fall back to the festival's `metadata.id` from fest.yaml
4. **No reference**: If `--no-tag` is set or no ID can be detected

## Use Cases

### Linked Projects

When working in a project linked to a festival (via fest.yaml `project_path` or navigation links), `fest commit` uses the **festival ID** since you're not inside a specific task directory.

```bash
# In linked project ~/projects/my-app
fest commit -m "Add feature"
# → [OBEY-FE-CS0001] Add feature
```

### Inside Festival Tasks

When working inside a festival's task directory, `fest commit` uses the **task reference** for more granular tracing.

```bash
# In ~/festivals/active/client-sync/01_setup/01_auth/03_oauth.md
fest commit -m "Implement OAuth flow"
# → [OBEY-FE-FEST-a3b2c1] Implement OAuth flow
```

## Design Rationale

### Why This Format?

1. **Namespace isolation**: The `OBEY-` prefix prevents conflicts with other tooling
2. **Component identification**: The two-letter component code (`FE`) allows multiple tools to share the format
3. **Traceability**: Every commit links back to its planning context
4. **Machine-parseable**: Consistent format enables automated reporting and tracking

### Future Components

The format is designed to accommodate additional Obey workflow components:

| Component | Code | Purpose |
|-----------|------|---------|
| Camp | `CA` | Campaign navigation and scaffolding |
| Intent | `IN` | Intent tracking system |
| Review | `RV` | Code review workflow |

These are reserved codes that may be implemented in future versions.

## Configuration

### Disabling Reference Tags

Use `--no-tag` to create commits without the reference prefix:

```bash
fest commit --no-tag -m "Minor cleanup"
# → Minor cleanup
```

### Disabling Auto-Stage

By default, `fest commit` stages all changes. Disable with `--stage=false`:

```bash
fest commit --stage=false -m "Commit only staged"
```

## Querying Commits

Use standard git tools to find commits by reference:

```bash
# Find all commits for a festival
git log --grep="OBEY-FE-CS0001"

# Find commits for a specific task
git log --grep="OBEY-FE-FEST-a3b2c1"

# Count commits per festival
git log --oneline | grep -oE '\[OBEY-FE-[A-Z0-9]+\]' | sort | uniq -c
```

## Integration with CI/CD

The standardized format enables automation:

```yaml
# Example: Extract festival context in GitHub Actions
- name: Get Festival Context
  run: |
    COMMIT_MSG=$(git log -1 --format=%s)
    if [[ $COMMIT_MSG =~ \[OBEY-FE-([A-Z0-9-]+)\] ]]; then
      echo "FEST_ID=${BASH_REMATCH[1]}" >> $GITHUB_ENV
    fi
```

## Related Documentation

- [Commands Reference](./commands.md) - Full `fest commit` command documentation
- [Configuration](./configuration.md) - Festival configuration including metadata
- [Architecture](./architecture.md) - System design and component overview
