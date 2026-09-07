# Contributing

Contributions are welcome. By contributing you agree that your contribution is
licensed under the [Apache License 2.0](LICENSE), the same license as the
project.

## Developer Certificate of Origin

Every commit must be signed off:

```bash
git commit -s
```

The sign-off certifies the [Developer Certificate of Origin 1.1](https://developercertificate.org/):
that you wrote the change, or otherwise have the right to submit it under the
project's license. Pull requests with unsigned commits will be asked to rebase
with sign-offs before merge.

## Practical notes

- Run `just check` before opening a PR. It covers the build, vet, lint,
  `docs-check`, and the unit tests. If you changed a command's help text, run
  `just docs` and commit the regenerated `docs/cli-reference`.
- Install the pre-push hook once per clone with `just hooks install`; it runs
  `just check` on every push. See
  [docs/contributing.md](docs/contributing.md#before-you-push).
- Match the surrounding code's conventions; see the README for project layout.
- Error construction is scoped, not "never `fmt.Errorf`": see
  [docs/contributing.md](docs/contributing.md#error-handling) (fest#342).
