---
title: "Development"
weight: 30
---

# Development

The full contributor guide lives in
[CONTRIBUTING.md](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md),
and the code standards in
[CLAUDE.md](https://github.com/jacob-delgado/workflow/blob/main/CLAUDE.md). This
page is the shape of the thing.

## Setup

```sh
git clone https://github.com/jacob-delgado/workflow.git
cd workflow
mise trust && mise install   # the whole toolchain, at pinned versions
task setup                   # modules + git hooks
```

[mise](https://mise.jdx.dev) is the only thing you install by hand. Every other
tool — Go, golangci-lint, lefthook, shellcheck, shfmt, yamllint, hadolint,
actionlint, typos, govulncheck, gitleaks, gobco, Hugo — is pinned in `mise.toml`
at an exact version. That file is the single source of truth: the build
container reads it, and so does CI, so a developer's machine, the container, and
CI cannot drift apart.

Prefer not to install anything? `task container:check` runs the entire gate
inside a container built from those same pins, and there is a devcontainer for
VS Code, GoLand, Codespaces, and the devcontainer CLI.

## The gate

```sh
task check
```

That is lint, tests, both coverage floors, vulnerability scanning, and secret
scanning — the same thing CI runs. Run it before calling any change done.

| Gate | What it enforces |
| --- | --- |
| `task lint` | Every golangci-lint linter, plus shell, YAML, Dockerfile, Actions, spelling, license headers, test markers, and docs drift |
| `task test:cover` | Tests with the race detector, above the statement coverage floor |
| `task cover:branch` | Condition coverage via gobco: was each branch seen both ways |
| `task vuln` | `govulncheck` against the dependency graph |
| `task secrets` | `gitleaks` over the working tree |

Both coverage floors live in `Taskfile.yml`, not in prose — a number restated in
a document drifts from the number the gate enforces.

## Documentation

This site is Hugo, built from Markdown in `docs/`:

```sh
task docs:serve    # preview at http://localhost:1313
task docs:build    # what CI builds and publishes
```

The command reference is **generated from the Cobra command tree** by `task
docs:gen`. Do not edit those pages by hand — `task docs:check` fails when they
disagree with the code, which is what keeps a new flag from shipping
undocumented.

## Commits and releases

Commits follow [Conventional Commits](https://www.conventionalcommits.org),
enforced by a git hook and re-checked in CI. That is also what drives releases:
release-please keeps a release pull request open, and merging it tags the
version and publishes binaries with checksums and build provenance.
