---
title: "workflow"
type: docs
---

# workflow

The [README](https://github.com/jacob-delgado/workflow#readme) says what
workflow is. Below is what works today, and where to read next.

## Status

The whole loop works, and is new: expect rough edges, and a configuration
format that may still change before 1.0.

- `workflow` opens the TUI. Pick up an assigned Jira issue, comment on it or
  change its status, branch for it, stage and commit through the repository's
  own hooks — opening a failure at its line in `$EDITOR` — push, open the pull
  or merge request from the repository's template, follow its CI, and announce
  it to your team. `?` lists the keys.
- `workflow --dry-run` does all of that with every write held back, saying what
  it would have done.
- `workflow --web` serves the same loop in a browser, on `127.0.0.1` alone —
  issues, the branch and its commits, the pull request and its announcement,
  your review queue, tasks, the summary, your repositories and the settings —
  pushed live as the repository changes.
- With no Jira configured, the Issues pane lists the issues assigned to you on
  your forge (GitHub or GitLab) instead — pick one up, branch for it, and the
  pull request closes it on merge.
- A repository with hooks in `.git/hooks` and no lefthook configuration is
  offered a `lefthook.yml` that runs them.
- `workflow doctor` reports the repository, tooling and configuration in effect;
  `workflow doctor --online` asks Jira and your forge whether their credentials
  work, and Slack whether your user token does, refreshing it if it is due; a
  webhook cannot be checked without posting, so it is reported unchecked.
- `workflow config init` sets up the configuration, answering the prompts
  (`--template` writes a blank file to edit) — or, with no file, the terminal
  interface and the web's Settings ask the same questions —
  and `workflow config show` prints the one in effect, credentials masked.
- `workflow summary` says what you did over a period — the commits you wrote,
  the tasks you touched, what you did to Jira issues and the pull requests you
  opened, had merged and reviewed — as the Summary pane reads it; `--json`
  prints it for a script, and `--post` posts it to your team after a preview.
- `workflow reviews` lists the pull requests on your forge that are waiting on
  your review — the longest-waiting first, with the author, how CI stands and
  how long each has waited.

## Where to go next

- **[Install]({{< relref "/docs/install" >}})** — `go install`, release binaries,
  or build from source.
- **[Using workflow]({{< relref "/docs/usage" >}})** — the panes, the keys,
  and the loop from issue to announcement.
- **[The web interface]({{< relref "/docs/web" >}})** — `workflow --web`: its
  sections, its live stream, and what it leaves to the terminal.
- **[Configuration]({{< relref "/docs/configuration" >}})** — every field of
  `.workflow.json`, and how to get the Jira token and a messaging webhook or
  Slack user token.
- **[Scripting]({{< relref "/docs/scripting" >}})** — the commands without the
  interface: exit codes, streams, `--json`, `--yes`, `--dry-run` and `--log`.
- **[Command reference]({{< relref "/docs/reference" >}})** — every command and
  flag, generated from the code.
- **[Development]({{< relref "/docs/contributing" >}})** — the toolchain, the
  gates, and how a release happens.

workflow is [Apache 2.0](https://github.com/jacob-delgado/workflow/blob/main/LICENSE)
licensed, and the source lives at
[jacob-delgado/workflow](https://github.com/jacob-delgado/workflow).
