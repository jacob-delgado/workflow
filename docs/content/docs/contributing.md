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
tool — Go and the task runner, every linter and formatter, the coverage and
security scanners, the docs builder — is pinned in `mise.toml` at an exact
version. That file is the single source of truth, and the list: the build
container reads it, and so does CI, so a developer's machine, the container, and
CI cannot drift apart. Naming each tool here would only drift from it.

Prefer not to install anything? `task container:check` runs the entire gate
inside a container built from those same pins, and there is a devcontainer for
VS Code, GoLand, Codespaces, and the devcontainer CLI. Under podman or colima,
give the VM about 8 GiB first (`podman machine set --memory 8192`, or
`colima start --memory 8`): at their 2 GiB default, golangci-lint and knip run
out of memory.

## The gate

```sh
task check
```

That is lint and tests for the Go and for the web frontend, both Go coverage
floors, vulnerability scanning, and secret scanning — the same gates CI runs,
less the browser-driven end-to-end suites, which CI adds: one against the web
frontend alone, and one that commits and pushes through a running
`workflow --web`. Run it before calling any change done.

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

The Go coverage floors live in `Taskfile.yml` and the web's in
`web/vitest.config.ts`, not in prose — a number restated in a document drifts
from the number the gate enforces.

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
version and publishes the release, as
[CONTRIBUTING.md](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#releases)
describes step by step.
