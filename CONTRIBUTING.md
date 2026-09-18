# Contributing to workflow

Thank you for your interest in contributing to workflow! workflow is a Go
command-line and terminal UI tool that integrates Jira, Slack, and GitHub or
GitLab to help developers run their workflow from the terminal.

Everyone participating in this project is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Before you start

For anything significant (a new feature, a new integration, or a change in
existing behavior), please
[open an issue](https://github.com/jacob-delgado/workflow/issues) first so the
approach can be discussed before you invest time in a pull request.

Small changes such as typo fixes, documentation improvements, or obvious bug
fixes can go straight to a pull request.

## Development setup

The toolchain is pinned with [mise](https://mise.jdx.dev) and the build runner is
[go-task](https://taskfile.dev). Install mise, then:

```sh
git clone https://github.com/jacob-delgado/workflow.git
cd workflow
mise trust && mise install   # Go, task, and every linter, at pinned versions
task setup                   # modules + git hooks (lefthook)
```

`mise install` is the only setup step that installs anything: Go, the task
runner, every linter and formatter, the coverage and security scanners, and the
docs builder all come from `mise.toml` at exact versions, so your local gate and
CI agree. That file is the list — naming each tool here only drifts from it.

Two alternatives, if you would rather not install a toolchain:

- **Devcontainer** — `.devcontainer/` works with VS Code, GoLand/Gateway,
  GitHub Codespaces, and the devcontainer CLI.
- **Build container** — `task container:check` runs the whole gate inside a
  container built from the same pinned versions.

## Day to day

```sh
task --list        # every task, with a description
task run           # run the TUI from source
task test          # tests with the race detector
task fmt           # format everything in place
task check         # the full gate — run this before opening a pull request
```

`task check` is lint, tests with the coverage floor, `govulncheck`, and
`gitleaks`. CI runs the same thing.

## Tests

New behavior and bug fixes must include tests. A bug fix should include a test
that fails without the fix and passes with it.

Tests are black-box: they live in the external test package (`package
config_test`) and drive code through its exported surface.

Each test marks its parts with `// Arrange`, `// Act` and `// Assert` comments,
and a table-driven test is preferred when cases differ only in data. `task lint`
and the pre-commit hook check the markers; CLAUDE.md's TDD process section has
the rules.

Two coverage floors gate a change, both configured in `Taskfile.yml` and both
printing the available ratchet when you clear them:

- **Statements** (`task test:cover`) — did this line run.
- **Conditions** (`task cover:branch`, via [gobco](https://github.com/rillig/gobco))
  — was each branch seen both ways. Its output names every condition observed only
  one way, which is a worklist of the tests still missing. gobco reads every
  package in this module; `scripts/gobco-report.sh` fails if one ever drops out
  without being listed as unreadable.

On a pull request, both numbers are posted as a comment with their delta against
`main`. The comment is informational — the floors are what fail the build.

## Code style

`task lint` is the arbiter — every golangci-lint linter is enabled, so the
feedback is immediate and specific. [CLAUDE.md](CLAUDE.md) explains the standards
behind those settings: function size, complexity limits, error handling, the code
smells worth watching, and the TDD process.

## License headers

Every `.go` file must begin with the following header:

```go
// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0
```

`scripts/check-license-headers.sh` checks this on commit and in CI.

## Commit messages

Commit messages follow the
[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
specification, enforced by a `commit-msg` git hook and re-checked in CI:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Types are `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`,
`chore`, `revert`, and `style`. Keep the subject under 72 characters, in the
imperative, with no trailing period, and use the body to explain *why*:

```text
feat(jira): add command to transition an issue

Moving an issue to In Progress is the first thing anyone does after
picking it up, and doing it in the browser costs the context the TUI
just gathered.

Refs #42
```

Mark a breaking change with a `!` after the type or scope (`feat!: ...`). A
`BREAKING CHANGE:` footer can describe it, but only alongside the `!`.
release-please also finds a breaking change in the body on its own — in a line
that starts with `BREAKING CHANGE:` or looks like `feat!:`, even one a sentence
or an example happened to land on, and in `BREAKING-CHANGE:` anywhere — so the
hook refuses all of those unless the subject has the `!`.
`scripts/check-commit-message.sh` lists exactly what it checks.

One thing no hook can see: a `BEGIN_COMMIT_OVERRIDE` block in a pull request's
description replaces its commits' messages for release-please, even after the
pull request is merged. Write one as carefully as a commit.

## Pull requests

1. Fork the repository and create a branch from `main`, named for the change it
   carries: `feat/<slug>`, `fix/<slug>`, `docs/<slug>`.
2. Keep each pull request focused on a single change.
3. Reference the related issue in the description (for example, `Fixes #42`).
4. Make sure `task check` passes.
5. Be responsive to review feedback. Maintainers may ask for changes before
   merging.

If a git hook blocks you, fix what it found rather than bypassing it.
`LEFTHOOK=0` exists for genuine emergencies, and CI re-runs the same checks
regardless.

`main` is protected: it takes no direct pushes, requires the checks above to
pass, and **merges by rebase only** — merge commits and squash are disabled, so
history stays linear and each commit keeps the message it was written with. That
is also why commits should be individually meaningful: every one of them lands
on `main` as you wrote it.

## Reporting a security issue

Do not open a public issue for a security bug. Report it privately through a
[security advisory](https://github.com/jacob-delgado/workflow/security/advisories/new);
[SECURITY.md](SECURITY.md) explains what to include and what to expect.

## Releases

Releases are automated with
[release-please](https://github.com/googleapis/release-please) and driven by the
commit messages above, so a well-formed commit is also a changelog entry:

1. Commits land on `main`. release-please keeps a release pull request open,
   with the next version and the generated `CHANGELOG.md`.
2. A maintainer merges that pull request when the release is ready. Nothing
   publishes until they do.
3. Merging tags `vX.Y.Z`, which builds the binaries for macOS (arm64), Linux
   (amd64) and Windows (amd64), generates SHA256 checksums, attests the build
   provenance, and publishes the GitHub Release with everything attached.

**Before 1.0, the minor digit is reserved for breaking changes.** Both `feat`
and `fix` bump the patch; only a breaking change — marked with `!`, as in
`feat!` — bumps the minor:

| Commit | Version change |
| --- | --- |
| `fix:` | 0.1.0 → 0.1.1 |
| `feat:` | 0.1.0 → 0.1.1 |
| `feat!:` | 0.1.0 → 0.2.0 |

So a version bump you have to react to means something you depended on actually
changed, rather than merely that features were added. After 1.0 this becomes
ordinary semantic versioning: `feat` bumps the minor and a breaking change bumps
the major.

Downloads can be verified with the checksums, or against their provenance:

```sh
gh attestation verify workflow_darwin_arm64 --repo jacob-delgado/workflow
```

## License

workflow is licensed under the [Apache License, Version 2.0](LICENSE). As
described in section 5 of the license, any contribution you intentionally submit
for inclusion in the project is licensed under the same terms, without any
additional terms or conditions.
