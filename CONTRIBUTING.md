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

Release Please uses a fine-grained personal access token stored as the repository
Actions secret `RELEASE_PLEASE_TOKEN`. Select resource owner `codyps`, limit access
to `coarsetime`, and grant these repository permissions:

- Contents: read and write (release branches, commits, tags, and releases).
- Pull requests: read and write (release PRs).
- Issues: read and write (release labels).
- Metadata: read-only (automatically included by GitHub).

Add the secret under Settings > Secrets and variables > Actions. No Actions,
Workflows, or Administration write permission is needed for this configuration.
Renew the secret before the token expires. The built-in `GITHUB_TOKEN` remains
read-only; a missing release token fails the release job with a setup message.

Using the PAT lets release PRs trigger CI automatically. Branch protection should
require the test and cross-compile checks before merging; the token does not need
a protection bypass, and automation does not approve or merge release PRs.

PAT-created `v*` tags trigger a separate test workflow, just like manually pushed
tags. Discovery runs there after the tag's tests and cross-compilation pass. If
discovery fails, rerun its failed job rather than moving or recreating the
published tag.

Keep published version tags immutable. Review a transition to v1 explicitly;
before releasing v2 or later, migrate the module path and imports to the required
major-version suffix (for example, `github.com/codyps/coarsetime/v2`).
