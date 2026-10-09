# Contributing to workflow

Thank you for your interest in contributing to workflow! The
[README](README.md) says what it is and what it does.

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
  container built from the same pinned versions. Under podman or colima, give
  the VM about 8 GiB first (`podman machine set --memory 8192`, or
  `colima start --memory 8`): at their 2 GiB default, golangci-lint and knip
  run out of memory.

## Day to day

```sh
task --list        # every task, with a description
task run           # run the TUI from source
task test          # tests with the race detector
task fmt           # format everything in place
task check         # the full gate — run this before opening a pull request
```

`task check` is the gate, each part a task of its own:

| Gate | What it enforces |
| --- | --- |
| `task lint` | Every `lint:*` task — golangci-lint and each other linter and repository check — plus `docs:check` (the command reference still matches the code) and `gen:verify` (the generated Go API code still matches `api/openapi.yaml`); `task --list` names each |
| `task web:lint` | The web frontend's eslint (accessibility at strict), `tsc`, prettier, knip, and its import boundaries |
| `task web:gen:check` | The generated TypeScript client still matches `api/openapi.yaml` |
| `task web:dist:check` | The committed web bundle in `internal/web/dist` is what `task web:build` makes of `web/src` |
| `task test:cover` | Tests with the race detector, above the statement coverage floor |
| `task web:test` | The web frontend's unit tests, above their own coverage floor |
| `task cover:branch` | Condition coverage via gobco: was each condition seen both ways |
| `task vuln` | `govulncheck` against the dependency graph |
| `task secrets` | `gitleaks` over the working tree |

The local gate needs Node, which `mise install` provisions, and installs the
frontend's dependencies when they are missing. CI runs the same gates, and adds
the Playwright end-to-end suites, which `task e2e` runs locally: the web
frontend alone, and then the page served by the binary `task build` makes,
staging, committing and pushing through it. Each needs a browser, installed
once with `yarn playwright install chromium` in `web/`; run them for a change
under `web/` or `internal/webserver`. One gate is narrower in CI: a pull
request that changes no Markdown file, nothing under `docs/`, not
`.markdownlint-cli2.yaml` and not `mise.toml`, which pins the Markdown linter,
skips the Markdown lint (`scripts/markdown-changed.sh` decides); a push to
`main` always runs it, and `task lint` locally always does.

The frontend's production build lives in `internal/web/dist` and is committed,
because `go install` fetches committed files alone and cannot run the Node
build. A change under `web/` that changes that build commits the rebuilt bundle
with it: run `task web:build` and stage `internal/web/dist`. `task check` and CI
fail when the two disagree.

## Tests

New behavior and bug fixes must include tests. A bug fix should include a test
that fails without the fix and passes with it.

Tests are black-box: they live in the external test package (`package
config_test`) and drive code through its exported surface.

Each test marks its parts with `// Arrange`, `// Act` and `// Assert` comments,
and a table-driven test is preferred when cases differ only in data. `task lint`
and the pre-commit hook check the markers; CLAUDE.md's TDD process section has
the rules.

A test that runs git must first clear the variables git exports to a hook, the
names `git rev-parse --local-env-vars` prints. From a linked worktree, a hook's
or `git rebase --exec`'s `GIT_DIR` is an absolute path into the shared
repository, and a test's `git -C <temp dir>` would work on that repository
instead: one pre-push set `core.bare`, moved `main` and rewrote the config that
way. The gate is safe to run from a hook or a linked worktree only because of
this. The Go packages that run git (`internal/cli`, `internal/wiring`) clear
them in `TestMain`, each proven by
`TestTheTestsLeaveTheRepositoryAHookNamesAlone`. Every script test that runs git
unsets them, which `scripts/hook-environment_test.sh` checks. The pre-push hook,
`task test`, `task test:cover`, `task cover:branch` and `task test:summary` all
unset them before running the Go tests as well. A new package or script test
that runs git needs the same.

Two Go coverage floors gate a change, both configured in `Taskfile.yml` and
both printing the available ratchet when you clear them:

- **Statements** (`task test:cover`) — did this line run.
- **Conditions** (`task cover:branch`, via [gobco](https://github.com/rillig/gobco))
  — was each condition seen both ways, each operand of an `a && b` on its own.
  Its output names every condition observed only one way, which is a worklist
  of the tests still missing. A condition no test evaluated, or an error check
  whose error arm no test reached, fails it outright, unless
  `scripts/gobco-allowlist.txt` keeps that arm for a trade-off `TECH_DEBT.md`
  records. gobco measures every package in this module that
  has tests. It reads a package of build-tagged twins one file at a time, so
  each file the build takes must stand alone, and any package or file it cannot
  read fails the gate.

The web frontend's unit tests (`task web:test`) hold their own floor, the
`thresholds` in `web/vitest.config.ts`.

On a pull request, both numbers are posted as a comment with their delta against
`main`, beneath how many tests each suite — Go unit, web unit, E2E and E2E
(server) — passed, skipped and failed, counted by the job that ran it
(`scripts/test-counts.sh`, rendered by `scripts/pr-comment.mjs`). The comment is
informational — the floors and the suites' own jobs are what fail the build.

## Code style

`task lint` is the arbiter, so the feedback is immediate and specific.
golangci-lint runs every linter it ships but five, which `.golangci.yml` turns
off with the reason beside each: `gomodguard_v2`, with no module list to hold;
`exhaustruct_v5`, unusable on third-party structs; and the deprecated
`exhaustruct`, `gomodguard` and `wsl`, each superseded by a `_v2` or `_v5`
linter of the same name. [CLAUDE.md](CLAUDE.md) explains the standards
behind those settings: function size, complexity limits, error handling, the
code smells worth watching, and the TDD process.

## License headers

Every `.go` file must begin with the following header:

```go
// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0
```

`scripts/check-license-headers.sh` checks this on commit and in CI.

The year is a fixed string — the project's first-publication year — not the
current one, which is common practice for a copyright line. A file created in a
later year still says 2026, and the check accepts only this exact line, so a
contributor's own copyright line is not added here; contributions are under the
Apache-2.0 license the header names.

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
`chore`, `revert`, and `style`. Keep the subject to at most 72 characters, in
the imperative, with no trailing period, and use the body to explain *why*:

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
   with the next version and the generated `CHANGELOG.md`, and moves the
   pinned `go install` version in `README.md` and the install page to it.
2. A maintainer merges that pull request when the release is ready. Nothing
   publishes until they do.
3. Merging tags `vX.Y.Z`, which runs `.github/workflows/release.yml`: it runs
   the gate, builds a binary for each platform `RELEASE_PLATFORMS` in
   `Taskfile.yml` names, publishes the GitHub Release with everything
   [a release carries](https://jacob-delgado.github.io/workflow/docs/install/#from-a-release)
   attached, and then republishes the documentation site.

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

## License

workflow is licensed under the [Apache License, Version 2.0](LICENSE). As
described in section 5 of the license, any contribution you intentionally submit
for inclusion in the project is licensed under the same terms, without any
additional terms or conditions.
