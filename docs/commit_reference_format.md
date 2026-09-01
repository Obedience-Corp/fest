# Commit Reference Format

This document describes the standardized commit reference format used by `fest commit`.

## Format Specification

```
[FE-{ID}-PH-{PHASE}-SQ-{SEQUENCE}]
```

`FE` identifies a Festival methodology reference. `PH` and `SQ` record where
inside the festival the commit happened, and both are optional.

Inside a camp, the festival reference is one segment of the camp tag
that `camp` and `fest` share:

```
[{campaign-name}:{campaign-id}-FE-{ID}-PH-{PHASE}-SQ-{SEQUENCE}]
```

### ID Types

The `{ID}` portion varies by context:

| ID Type | Format | Example | Source |
|---------|--------|---------|--------|
| Task Reference | `FEST-` plus six digits | `FEST-123456` | `fest_ref` field in task frontmatter |
| Festival ID | `XX0000` | `CS0001` | `metadata.id` field in fest.yaml |

### Position Segments

`PH` and `SQ` carry the leading number of the phase and sequence directory,
exactly as the directory spells it. `001_IMPLEMENT/02_camp_pilot` becomes
`PH-001-SQ-02`; no re-padding is applied, so `12_ROLLOUT/007_wide` becomes
`PH-12-SQ-007`.

The segments are dependent, and each is emitted only when the one it indexes
into is present:

| Segment | Emitted when |
|---------|--------------|
| `PH` | A festival reference is present and the phase is known |
| `SQ` | A phase is present and the sequence is known |

A sequence number is therefore never emitted on its own.

### Full Examples

```
[FE-CS0001] Add user authentication
[FE-FEST-123456] Fix login validation bug
[FE-TODO0001] Update documentation
[FE-CC0008-PH-001-SQ-02] Update camp scaffold
[FE-CC0008-PH-001] Close out the implementation phase
[obey-campaign:8deed8b4-FE-CC0008-PH-001-SQ-02] feat: update camp scaffold
```

## Detection Priority

When `fest commit` generates a reference, it follows this priority:

1. **Explicit `--task` flag**: Use the provided task reference directly
2. **Task frontmatter**: If running inside a festival task directory, use the task's `fest_ref`
3. **Festival metadata**: Fall back to the festival's `metadata.id` from fest.yaml
4. **No reference**: If `--no-tag` is set or no ID can be detected

## Position Resolution

The phase and sequence are resolved separately from the reference, and only
once a reference exists. `--no-tag` suppresses both.

1. **Current directory**: If the working directory sits inside the festival,
   the first two path segments below the festival root supply the phase and
   sequence. A directory whose name does not start with digits followed by an
   underscore contributes nothing, so standing in `001_IMPLEMENT` yields a
   phase with no sequence.
2. **Progress store**: When the working directory settles nothing (the
   festival root, a linked project, anywhere outside the festival), the
   festival's in-progress tasks decide. Their keys are festival-relative task
   paths, and the phase and sequence come from the first two segments.

Resolution is fail-soft and never blocks a commit. Anything ambiguous is
simply omitted:

- Zero in-progress tasks: no position.
- In-progress tasks spread across parallel sequences or phases: no position,
  because no single one describes the commit.
- Legacy progress keys that are bare filenames with no directories: skipped,
  since they carry no position at all.
- An unreadable or absent progress store: no position.

The working directory always wins over the progress store. Committing from
sequence `03` while the in-progress task lives in sequence `02` records `03`.

Resolution reads the progress store without mutating it, so running
`fest commit` never migrates or removes a live navigator's workflow state.

## Use Cases

### Linked Projects

When working in a project linked to a festival (via fest.yaml `project_path` or navigation links), `fest commit` uses the **festival ID** since you're not inside a specific task directory. The working directory is outside the festival, so the position comes from the progress store.

```bash
# In linked project ~/projects/my-app, with one in-progress sequence
fest commit -m "Add feature"
# → [FE-CS0001-PH-001-SQ-02] Add feature

# In the same project, with tasks in progress across parallel sequences
fest commit -m "Add feature"
# → [FE-CS0001] Add feature
```

### Inside Festival Tasks

When working inside a festival's task directory, `fest commit` uses the **task reference** for more granular tracing.

A task reference needs a working directory at least one level *below* the
sequence directory, because that is where `fest commit` looks for the task
document whose `fest_ref` it reads. Standing in the sequence directory itself
yields the festival ID instead.

```bash
# In the sequence directory itself
# ~/festivals/active/client-sync/01_setup/01_auth
fest commit -m "Implement OAuth flow"
# → [FE-CS0001-PH-01-SQ-01] Implement OAuth flow

# One level deeper, where a task document is in reach
# ~/festivals/active/client-sync/01_setup/01_auth/oauth
fest commit -m "Implement OAuth flow"
# → [FE-FEST-123456-PH-01-SQ-01] Implement OAuth flow
```

The reference and the position are two separate walks of the same path with
different depth thresholds: the position needs only the first two segments
below the festival root, while the task reference needs a third. That is why a
commit can carry a phase and sequence but still fall back to the festival ID.

## Design Rationale

### Why This Format?

1. **Component identification**: The `FE` prefix identifies Festival references without a redundant product namespace
2. **Traceability**: Every commit links back to its planning context, down to
   the phase and sequence it belongs to
3. **Machine-parseable**: Consistent format enables automated reporting and tracking

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
git log --grep="FE-CS0001"

# Find commits for a specific task
git log --grep="FE-FEST-123456"

# Find every commit made inside one phase
git log --grep="FE-CC0008-PH-001"

# Find every commit made inside one sequence
git log --grep="FE-CC0008-PH-001-SQ-02"

# Count commits per festival
git log --oneline | grep -oE 'FE-[A-Z0-9]+' | sort | uniq -c
```

## Integration with CI/CD

The standardized format enables automation:

```yaml
# Example: Extract festival context in GitHub Actions
- name: Get Festival Context
  run: |
    COMMIT_MSG=$(git log -1 --format=%s)
    # Stop at the next tag segment so a task ref (FE-FEST-123456) is captured
    # whole instead of truncating to "FEST".
    if [[ $COMMIT_MSG =~ FE-((FEST-)?[A-Za-z0-9]+)(-PH-|-SQ-|-WI-|-NT-|\]|$) ]]; then
      echo "FEST_ID=${BASH_REMATCH[1]}" >> $GITHUB_ENV
    fi
    if [[ $COMMIT_MSG =~ -PH-([0-9]+) ]]; then
      echo "FEST_PHASE=${BASH_REMATCH[1]}" >> $GITHUB_ENV
    fi
    if [[ $COMMIT_MSG =~ -SQ-([0-9]+) ]]; then
      echo "FEST_SEQUENCE=${BASH_REMATCH[1]}" >> $GITHUB_ENV
    fi
```

## Related Documentation

- [Commands Reference](./commands.md) - Full `fest commit` command documentation
- [Configuration](./configuration.md) - Festival configuration including metadata
- [Architecture](./architecture.md) - System design and component overview
