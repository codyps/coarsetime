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

The first release is configured as `v0.1.0`. The empty initial manifest means
there has not been a release yet. Existing free-form commits do not supply release
notes: include a release-triggering Conventional Commit (for example,
`feat: add coarse elapsed and wall clocks`) to start the first release PR. There
is no need to rewrite old commits or update a version in `go.mod`.

Automation uses the repository's built-in `GITHUB_TOKEN`; no extra secret is
required. Repository Settings > Actions > General must allow GitHub Actions to
create and approve pull requests. GitHub may require a maintainer to approve
workflow runs on bot-created PRs. This does not automatically approve or merge
the release PR.

Discovery runs directly in the same workflow because tags created with
`GITHUB_TOKEN` do not trigger another push workflow. Manually pushed `v*` tags
retain the existing test-and-discover behavior. If discovery fails, rerun its
failed job rather than moving or recreating the published tag.

Keep published version tags immutable. Review a transition to v1 explicitly;
before releasing v2 or later, migrate the module path and imports to the required
major-version suffix (for example, `github.com/codyps/coarsetime/v2`).
