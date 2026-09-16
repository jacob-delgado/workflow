---
title: "workflow"
type: docs
---

# workflow

A terminal UI for the loop a developer actually runs all day: pick up a Jira
issue, start a branch for it, open the pull or merge request, and tell the team
in Slack — without leaving the keyboard or rebuilding the same context across
three browser tabs.

## Status

Early, and not yet the whole loop. What works today:

- `workflow` opens the TUI: your assigned Jira issues in a pane rail, with
  keyboard and mouse navigation (`?` lists the keys). `t` moves the selected
  issue to a new status, through the transitions its workflow offers.
- `workflow doctor` reports the repository, tooling and configuration in effect;
  `workflow doctor --online` asks Jira, Slack and your forge whether each
  credential actually works.
- `workflow config init` writes a starting configuration file.
- `workflow config show` prints the configuration in effect, credentials masked.

Not built yet: branching from an issue, committing, opening the pull or merge
request, and posting to Slack. Expect the configuration format to change while
that lands.

## Where to go next

- **[Install]({{< relref "/docs/install" >}})** — `go install`, release binaries,
  or build from source.
- **[Configuration]({{< relref "/docs/configuration" >}})** — every field of
  `.workflow.json`, and how to get the Jira and Slack tokens.
- **[Command reference]({{< relref "/docs/reference" >}})** — every command and
  flag, generated from the code.
- **[Development]({{< relref "/docs/contributing" >}})** — the toolchain, the
  gates, and how a release happens.

workflow is [Apache 2.0](https://github.com/jacob-delgado/workflow/blob/main/LICENSE)
licensed, and the source lives at
[jacob-delgado/workflow](https://github.com/jacob-delgado/workflow).
