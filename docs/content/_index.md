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

The whole loop works, and is new: expect rough edges, and a configuration
format that may still change before 1.0.

- `workflow` opens the TUI. Pick up an assigned Jira issue, comment on it or
  change its status, branch for it, stage and commit through the repository's
  own hooks — opening a failure at its line in `$EDITOR` — push, open the pull
  or merge request from the repository's template, follow its CI, and announce
  it in Slack. `?` lists the keys.
- `workflow --dry-run` does all of that with every write held back, saying what
  it would have done.
- A repository with hooks in `.git/hooks` and no lefthook configuration is
  offered a `lefthook.yml` that runs them.
- `workflow doctor` reports the repository, tooling and configuration in effect;
  `workflow doctor --online` asks Jira, your messaging service and your forge
  whether each credential actually works.
- `workflow config init` writes a starting configuration file, and
  `workflow config show` prints the one in effect, credentials masked.

## Where to go next

- **[Install]({{< relref "/docs/install" >}})** — `go install`, release binaries,
  or build from source.
- **[Using workflow]({{< relref "/docs/usage" >}})** — the panes, the keys,
  and the loop from issue to Slack.
- **[Configuration]({{< relref "/docs/configuration" >}})** — every field of
  `.workflow.json`, and how to get the Jira and Slack tokens.
- **[Scripting]({{< relref "/docs/scripting" >}})** — the commands without the
  interface: exit codes, streams, `--json`, `--yes`, `--dry-run` and `--log`.
- **[Command reference]({{< relref "/docs/reference" >}})** — every command and
  flag, generated from the code.
- **[Development]({{< relref "/docs/contributing" >}})** — the toolchain, the
  gates, and how a release happens.

workflow is [Apache 2.0](https://github.com/jacob-delgado/workflow/blob/main/LICENSE)
licensed, and the source lives at
[jacob-delgado/workflow](https://github.com/jacob-delgado/workflow).
