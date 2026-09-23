---
title: "Scripting"
weight: 22
---

# Scripting

The steps of the loop that a script or a shell prompt wants also run as
commands, without the interface: `status`, `reviews`, `standup`, `branch`,
`pr` and `announce`, beside `doctor` and `config`. This page is what a script
can rely on from them — the exit status, which stream carries what, the JSON shapes, and the
flags that make a write safe to run unattended. Every command and flag is
listed in the [command reference]({{< relref "/docs/reference" >}}).

```sh
workflow status --json                 # where the work stands, as data
workflow --dry-run pr                  # what pr would push and open
workflow pr --yes                      # push, open, link and move, without asking
workflow standup --no-edit --yes       # post the day's standup, unattended
workflow --log requests.log doctor --online   # a bug report's evidence
```

## Exit status

A script tells one kind of failure from another by the exit status, never by
the message, which is prose and may change.

| Status | Meaning | For example |
| --- | --- | --- |
| 0 | Success. | |
| 1 | Any other failure. | An issue that is not in the tracker, a push that was rejected, git missing, a repository whose branch cannot be read. |
| 2 | Usage: the command was called wrongly. | An unknown flag, command or subcommand; a wrong number of arguments; `standup --days 0`; a confirmation with no terminal to answer it. |
| 3 | Configuration: fix the file, a credential or a login. | No `.workflow.json` for `doctor` or `config show`; a file that does not parse; a required field left empty, or set unusably; a file other users can read; no messaging configured for `announce`; a credential that is missing — a Jira token, or a forge token from the file, the environment or `gh auth login` — or one a service rejected. |
| 4 | A refused precondition: the command would not go ahead because of what it found. | A pull request already open; no commits to open one for; no pull request to announce; a branch or configuration file that already exists; a directory that is not a git repository. |
| 5 | Unreachable: a service did not answer, or asked you to wait. | Jira, the forge or the messaging service could not be reached; rate limiting. |
| 130 | Interrupted by Ctrl+C. | |

cobra's own `help` command is the one exception to 2: asked about a command
it does not know, it prints the root's help and exits 0.

A command that reports several problems at once — `doctor` checks every
section, `status DIR…` every directory — exits with the first family any of
them belongs to, in the order 2, 3, 4, 5, 1. So `doctor` with no configuration
and no git exits 3: the configuration is what to fix first.

A `.workflow.json` that exists but does not parse stops every command with 3,
rather than being replaced by the defaults; one that cannot be opened stops
it with 1. With no file at all, the commands that can run on the defaults
still do.

With `forge.cli` set, the forge is asked through `gh` or `glab`, and a CLI
that is not signed in reads as a forge that could not be reached: 5, not 3.
`workflow doctor --online` says which it is.

### The same families on the web

`workflow --web` answers a failed request with a problem code (see
[Web API errors]({{< relref "/docs/errors" >}})). The families line up:

| Exit status | Web problem code |
| --- | --- |
| 2 usage | `bad_request` (400) |
| 3 configuration | `unprocessable` (422), for a missing messaging service, an invalid configuration body, a Jira token not configured or not accepted, and a `jira.base_url` that is not a usable address |
| 4 refused precondition | `conflict` (409) |
| 5 unreachable | `unreachable` (502), for a service that could not be reached or asked to wait |
| 1 failure | `internal` (500); the web also answers `not_found` (404) for a missing issue, and `unprocessable` (422) for a change Jira refused or a `jira.base_url` with no Jira API behind it — all of which the command line counts as a plain failure |

The web has no problem code of its own for a missing or rejected credential. A
Jira credential answers `unprocessable` and points at `workflow doctor`, as the
command line exits 3; a forge credential that fails a read still answers
`internal`.

## Standard output and standard error

Standard output carries the artifact — what a script would capture: the
JSON, the preview of what a write will do, the draft, the rows, and what a
write created. Standard error carries everything said *about* it: dry-run
lines, declined and done notices, warnings, guidance, the questions a command
asks, and the error itself, prefixed `workflow:`.

| Command | stdout | stderr |
| --- | --- | --- |
| `status` | the line, or one row per directory, or the JSON | |
| `reviews` | one line per review, or the JSON | "No pull requests are waiting on your review." |
| `doctor` | the report, or the JSON | |
| `config show` | the configuration as JSON, credentials masked | the file it came from (`# PATH`); how to create one when there is none |
| `config init` | with `--dry-run`, the file it would write, as JSON, masked | progress, the checks, "Wrote …", what to do next, a warning when the file is not ignored by git |
| `standup` | the draft | "Nothing to share.", the dry-run line, "Not posted.", "Posted to …" |
| `branch` | `Branch NAME from BASE and switch to it`, then `Created NAME` | the dry-run line, "Not created." |
| `pr` | `Open TITLE` and `BRANCH → BASE`, then `Opened #N URL` (`!N` on GitLab) | the dry-run line, "Not opened.", the offers to link it on the issue and to move the issue to the review status, and their outcomes |
| `announce` | the message and where it goes | that an earlier session already announced this moment, the dry-run line, "Not announced.", "Announced to …" |
| `workflow --web` | | the address it serves on |

So `workflow config show | jq .` parses, and `workflow reviews | wc -l` counts
reviews and nothing else.

## JSON

`--json` is on the reads: `status`, `reviews` and `doctor`. `config show`
prints JSON always. None of them carries a credential: `config show` masks
each to its last four characters, and the others never print one.

`workflow status --json` prints one object:

```json
{
  "issue": "PROJ-412",
  "summary": "Fix token redaction",
  "stages": [
    { "name": "Issue", "state": "done" },
    { "name": "Branch", "state": "done" },
    { "name": "Commits", "state": "done" },
    { "name": "Review", "state": "in_flight" },
    { "name": "Slack", "state": "not_started" }
  ],
  "ci": "running"
}
```

A stage's `state` is `done`, `in_flight`, `failed` or `not_started`; `ci` is
`running`, `passed`, `failed` or `none`. The last stage is named for the
messaging service `messaging.kind` configures — `Slack`, `Teams`, `Discord`
or `Webhook` — so read the stages by position rather than by that name. `issue` and `summary` are left out
when the branch names no issue.

`workflow status --json DIR…` prints an array, one object per directory, each
with a `repository` label. A directory that cannot be read has an `error`
instead of the stages — `"not a git repository"`, or why its repository
could not be read — and the command then exits non-zero after printing the
whole array.

`workflow reviews --json` prints an array, oldest first:

```json
[
  {
    "number": 42,
    "title": "fix(config): redact the webhook",
    "author": "ana",
    "repository": "acme/api",
    "draft": false,
    "ci": "passed",
    "age": "3d",
    "url": "https://github.com/acme/api/pull/42"
  }
]
```

`workflow doctor --json` prints the report as an object: `version`;
`repository` (`inside_work_tree`, `root`, `branch`, `detached`, `remote`,
`forge`); `tooling`, one entry per program (`name`, `found`, `required`,
`effect`); `configuration` (`path`, `jira_url`, `jira_auth_mode`,
`messaging_target`, `messaging_mode`, `world_readable`, `missing`), or
`config_problem` when the file did not load; and `credentials` (`checked`,
and with `--online`, `results`: `service`, `status` — `ok`, `rejected` or
`unreachable` — and `detail`). It exits as the prose report does.

`workflow config show` prints the configuration file's own shape, as
[Configuration]({{< relref "/docs/configuration" >}}) describes it.

## Writing without a person: `--yes` and `--dry-run`

`branch`, `pr`, `announce` and `standup` print a preview and ask before they
write. Two flags change that:

- **`--yes`** goes ahead without asking. On `pr` it answers every question:
  the push, the open, and the offers that follow it — to link the pull request
  on the branch's issue and to move the issue to the review status. A link
  that fails is said on stderr, the move is still made, and the command exits
  non-zero. On `announce` it never repeats an announcement: when the store
  says this pull request was already announced at the moment it is at, it
  says so on stderr, announces nothing, and exits 0 — run without `--yes` to be
  asked. It does not skip `standup`'s editor: add `--no-edit` for that.
- **`--dry-run`** prints the preview and what the command would do, and
  writes nothing. It is one flag for every command, given before the command's
  name or after it: `workflow --dry-run pr` and `workflow pr --dry-run` are
  the same. `config init --dry-run` runs its checks and prints the file it
  would write, masked, without writing it or storing anything in the keychain.
  Bare `workflow --dry-run` is the interface with every write held back.

A write run without `--yes` and without a terminal — stdin piped or closed —
has no way to be answered, so it stops, says to pass `--yes`, and exits 2. The
one exception is `announce` at a moment already announced, which `--yes` would
leave as it is: it says to run it at a terminal instead.

## A log for a bug report: `--log`

`--log FILE` appends a one-line outline of every request a command makes —
the time, the service, the method, the path, the status and how long it took
— to `FILE`. Like `--dry-run`, it is accepted before or after any command.
It records nothing else: never a header, a body, a query string or a host,
and never the path of a messaging webhook — Slack's, Teams', Discord's or a
plain one — which is itself the credential. The file is created readable only
by you.

```text
2026-09-22T18:04:11Z jira  GET  /rest/api/2/myself 200 184ms
```

`workflow --log requests.log doctor --online` is the run a bug report most
wants: every credential check, each under its service's name.
