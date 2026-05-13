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

## Filesystem test safety

The watch integration tests create festival trees, navigation links, and watch
state only inside the containerized integration harness. No host
filesystem-mutating unit tests were added for `fest watch`.

## Known limitation

The current Testcontainers TTY helper can allocate a TTY but cannot send
deterministic stdin to select picker entries. Picker selection is covered by
pure resolver tests; a full interactive picker smoke should be done manually or
with a richer TTY harness before stable promotion.

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
