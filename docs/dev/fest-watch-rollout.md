# fest watch dev rollout notes

`fest watch` is dev-only in this PR. It is registered only from
`internal/commands/release/register_dev.go` under the `dev` build tag and is
not part of the stable command surface.

## Command contract

- `fest watch` resolves the current festival from a festival cwd.
- `fest watch <selector>` resolves by festival directory name or logical ID.
- From a linked project cwd, `fest watch` resolves the linked festival without
  changing the shell directory.
- From campaign or festivals workspace context in an interactive terminal, the
  command falls back to the festival picker.
- Watch mode delegates to the existing in-progress show watcher and exits with
  `Ctrl+C`.

## Stable user impact

Stable users are unaffected. Stable command-surface checks omit `fest watch`,
stable root help does not list it, and generated stable CLI reference docs are
not updated to include the command.

## Validation run for this PR

- `go test ./internal/commands/... -run '^$'`: passed.
- `go test ./cmd/fest/... -run '^$'`: passed.
- `go test -tags dev ./internal/commands/... -run '^$'`: passed.
- `go test -tags dev ./cmd/fest/... -run '^$'`: passed.
- `go test -tags dev ./internal/commands/watch/... ./internal/commands/shared/...`: passed.
- `TESTCONTAINERS_RYUK_DISABLED=true go test -tags "integration dev" -run TestFestWatch ./tests/integration/...`: passed.
- Stable `./bin/fest __commands` does not include `fest watch`.
- Dev `./bin/fest __commands` includes `fest watch`.

## Final PR state

- PR: https://github.com/Obedience-Corp/fest/pull/171
- Branch: `fest-watch`
- Implementation review commit: `c8aecd1`
- Final summary commit: documentation-only traceability update after review.
- Obey Agent review: commented on the PR with resolver fallback and nil-festival
  handling confirmed resolved.
- Final festival validation: `fest validate` passed with score 100/100.
- Final container validation:
  `TESTCONTAINERS_RYUK_DISABLED=true go test -count=1 -tags "integration dev" -run TestFestWatch ./tests/integration/...`
  passed.

## Filesystem test safety

The watch integration tests create festival trees, navigation links, and watch
state only inside the containerized integration harness. No host
filesystem-mutating unit tests were added for `fest watch`.

## Known limitation

The current Testcontainers TTY helper can allocate a TTY but cannot send
deterministic stdin to select picker entries. Picker selection is covered by
pure resolver tests; a full interactive picker smoke should be done manually or
with a richer TTY harness before stable promotion.

## Self-review checklist

- Command contract: `fest watch [festival-selector]` supports selector,
  current festival, linked project, and picker resolution without changing cwd.
- Resolver: precedence is selector, direct festival, link, picker, then
  actionable no-context error.
- Picker and completion: both use shared festival candidate helpers, exclude
  status-directory targets for watch, and include valid dungeon date-bucket
  festivals.
- Dev gating: command registration is limited to dev builds through
  `internal/commands/release/register_dev.go`; stable help, docs, and
  `__commands` remain clean.
- Tests: resolver, completion, command-surface, and option mapping tests are
  pure; filesystem/link/watch behavior lives in the container integration
  harness.
- Docs: rollout notes document stable user impact, validation commands,
  filesystem test safety, and the remaining picker automation limitation.

## Stable promotion criteria

Promote `fest watch` out of dev-only registration only after:

- selector, direct-festival, linked-project, and picker paths have production
  confidence from tests or real use
- shell completion behavior is verified
- stable CLI reference docs are intentionally updated
- the command has been exercised against real festivals without cwd or watch
  renderer surprises
- review concerns around package boundaries and filesystem test safety are
  resolved
