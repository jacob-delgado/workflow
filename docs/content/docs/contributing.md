---
title: "Development"
weight: 30
---

# Development

How to contribute is written down once, in
[CONTRIBUTING.md](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md),
and the code standards in
[CLAUDE.md](https://github.com/jacob-delgado/workflow/blob/main/CLAUDE.md):

- [Development setup](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#development-setup)
  — mise and the pinned toolchain, the git hooks, and the containers that run
  the gate with nothing installed.
- [Day to day](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#day-to-day)
  — the tasks, and `task check`, the gate, part by part.
- [Tests](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#tests)
  — black-box, Arrange-Act-Assert, and the coverage floors.
- [Commit messages](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#commit-messages)
  and [Releases](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#releases)
  — Conventional Commits, and how they cut a release.

This page covers the one thing those leave to it: this site.

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
