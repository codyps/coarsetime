# Contributing

Run the checks described in [README.md](README.md#tests-and-benchmarks).

## Commit messages

Use Conventional Commits for changes to the library. When squash-merging a PR,
use its Conventional Commit title as the final commit subject.

- `fix: correct elapsed-time overflow` produces a patch release.
- `perf: reduce clock read overhead` produces a patch release.
- `feat: add a clock operation` produces a minor release.
- `feat!: change the clock API` marks a breaking change. Below v1 this produces
  a minor release; from v1 onward it produces a major release.
- `docs:`, `test:`, `ci:`, `chore:`, and `refactor:` do not normally trigger a
  release. Use `!` or a `BREAKING CHANGE:` footer when a change breaks the API.

Changes confined to `research/` are excluded from release calculation.

## Releases

Release Please maintains a release PR for the root Go module after successful
tests and cross-compilation on pushes to `main`. It updates `CHANGELOG.md` and
`.release-please-manifest.json`. Review the version and notes, then merge that PR.
After CI passes, automation creates a `vX.Y.Z` tag and GitHub Release and requests
the tagged module through the public Go proxy for pkg.go.dev discovery.
