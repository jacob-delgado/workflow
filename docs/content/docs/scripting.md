---
title: "Scripting"
weight: 22
---

# Scripting

The steps of the loop that a script or a shell prompt wants also run as
commands, without the interface: `status`, `reviews`, `repositories`,
`summary`, `issues`, `issue`, `branch`, `pr`, `finish`, `announce`, `comment`,
`transition`, `assign` and `worklog`, beside `doctor`, `config`, `slack login` and
`db-clean`. This page is what a script
can rely on from them — the exit status, which stream carries what, the JSON shapes, and the
flags that make a write safe to run unattended. Every command and flag is
listed in the [command reference]({{< relref "/docs/reference" >}}).

```sh
workflow status --json                 # where the work stands, as data
workflow reviews --json                # what waits on your review, oldest first
workflow repositories --json | jq -r '.worktrees[].dir'   # this repository's worktrees
workflow issues --view Sprint          # a view's issues, one a line
workflow issue PROJ-7 --json           # one issue in full, as data
cd "$(workflow branch PROJ-7 --fetch --worktree --yes | tail -n 1)"   # start fresh, beside this checkout
workflow pr -n                         # what pr would push and open
workflow pr --yes                      # push, open, link and move, without asking
workflow pr --yes --json | jq .pull.url   # the same, and the address it opened
workflow pr -y --draft --reviewer ana  # open a draft, asking ana rather than the code owners
workflow finish --dry-run              # the three git commands that finish a merged branch
workflow announce --yes                # announce it, unattended
workflow announce -y --channel '#release'   # the same, in #release rather than messaging.channel
workflow summary --json | jq -r .text  # what you did on the previous working day
workflow summary --post --yes          # post it to your team, unattended
git log -1 --format=%B | workflow comment PROJ-7 --yes   # the text on stdin, never quoted
workflow transition PROJ-7 "In Review" --yes   # move it, unattended
workflow assign PROJ-7 bo --yes        # hand it to bo
workflow worklog PROJ-7 2h --comment "pairing" --yes   # log the time spent on it
workflow --log requests.log doctor --online   # a bug report's evidence
```

## Exit status

A script tells one kind of failure from another by the exit status, never by
the message, which is prose and may change.

| Status | Meaning | For example |
| --- | --- | --- |
| 0 | Success. | |
| 1 | Any other failure. | Standard output that could not be written — a pipe whose reader has gone, a full disk — whatever else the command found; an issue that is not in the tracker, a `pr --template` the repository has no template of, named with the templates it has, a `pr --base` no branch has, a push that was rejected, a `branch --fetch` whose fetch failed, a `finish` whose git step failed, git missing, a repository whose branch cannot be read; a `db-clean` that finds a symlink or a directory where a database file belongs. |
| 2 | Usage: the command was called wrongly. | An unknown flag, command or subcommand; a wrong number of arguments; a `summary --from` or `--to` that is not a date written `YYYY-MM-DD`, or a period that runs backwards or is longer than a year and a day; `summary --json --post`, or `--yes` or `--channel` without `--post`; an `announce` or `summary` `--channel` where messaging posts through a webhook; `status --plain` with `--json`; a `pr --base` git would refuse as a branch's name, or `pr --body` with `--template`; `reviews --sort` with an order it does not know; an `issues --view` naming no view `jira.views` configures; `--port` without `--web`, or outside 1 to 65535; a confirmation, or the guided `config init`, with no terminal to answer it; a `slack login` answer left blank; a `comment` with nothing but white space on standard input; a `transition` whose status is blank; an `assign` whose username is blank; a `worklog` with a blank duration, or on a forge issue, which keeps no worklog. |
| 3 | Configuration: fix the file, a credential or a login. | No `.workflow.json` for `doctor`, `config show` or `slack login`; a file that does not parse; a required field left empty, or set unusably; a file other users can read; a `ui.keys` map the interface refuses to start on; no messaging configured for `announce` or `summary --post`; a `worklog` with no Jira configured, where the forge's issues are the tracker; a `slack login` where Slack posts through a webhook, or `messaging.kind` names another service, or whose credentials Slack refuses to refresh; no repository remote for `reviews` to find the forge from, or one on a host other than `github.com`, a `ghe.com` tenant, `gitlab.com` or the `forge.host` a `forge.kind` of `github` or `gitlab` describes; a credential that is missing — a Jira token, or a forge token from the file, the environment or `gh auth login` — or one a service rejected, a Jira 403 that says what the token may not do among them. |
| 4 | A refused precondition: the command would not go ahead because of what it found. | A pull request already open; no commits to open one for; no pull request to announce; a `finish` on a branch that has not merged, has no base to return to, holds commits that neither merged nor are on its upstream, or is not a branch of its own; a branch or configuration file that already exists; a directory that is not a git repository; a `db-clean` that could not remove a database file, as one another program holds open can be on Windows; a `summary --post` too long for the messaging service; a `transition` to a status the tracker does not offer from where the issue stands, which names those it does, or one whose move asks for fields, which names them. |
| 5 | Unreachable: a service did not answer, or asked you to wait. | Jira, the forge or the messaging service could not be reached — Slack among them when `slack login` refreshes — began an answer it did not finish in time, or answered with a redirect — a sign-in gateway in front of it, say — which is refused so the credential goes nowhere else; rate limiting. |
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
| 2 usage | `bad_request` (400); `method_not_allowed` (405), for a method the path does not answer; `precondition_required` (428), for a configuration save that names no revision to write over (no `If-Match`) |
| 3 configuration | `not_set_up` (422), for a service with nothing set up to ask: a Jira token not configured, a forge token not found, an origin that names no forge or one workflow cannot tell, no Taskwarrior installed or never run, no git `user.email`; `unprocessable` (422), for a missing messaging service, an invalid configuration body or `ui.keys` map, a configuration file on disk that no longer reads as valid, a Jira token not accepted, or refused with a 403 that says what it may not do, a `jira.base_url` that is not a usable address, a forge token not accepted, and a `forge.kind` set without its `forge.host` |
| 4 refused precondition | `conflict` (409), a local database file that could not be removed among them; `too_long` (422), for a Summary longer than the messaging service takes in one message, as `summary --post` refuses one |
| 5 unreachable | `unreachable` (502), for a service that could not be reached or answered with a redirect; `rate_limited` (503), for one that asked to wait, with `Retry-After` when it said how long |
| 1 failure | `internal` (500); the web also answers `not_found` (404) for a missing issue or pull request template, `unprocessable` (422) for any other change Jira refused, a `jira.base_url` with no Jira API behind it, a forge address with no forge API behind it, or a request the forge refused for the token's permissions, or a clean of the local data that found something other than the store's own files, `unreachable` (502) for a status the forge does not document, and `fetch_failed` (502) for a fetch of the base remote that failed before starting work, as `branch --fetch` can — all of which the command line counts as a plain failure |
| none | `check_failed` (422), for a first configuration's Jira check that did not pass, which `config init` instead asks about, offering to save the token anyway; `unauthorized` (401), for a request that presented no session of this run, which the command line never makes |

A missing Jira or forge credential answers `not_set_up`, saying how to set it
up, and a rejected one `unprocessable`, pointing at `workflow doctor`; the
command line exits 3 for both — except a Jira 403 that says what the
token may not do, which says Jira refused the request: doctor checks who a
token is, not what it may do.

The reads a script can make of the web's API are listed under [Scripting the
API]({{< relref "/docs/web#scripting-the-api" >}}).

## Standard output and standard error

Standard output carries the artifact — what a script would capture: the
JSON, the preview of what a write will do, the draft, the rows, and what a
write created. Standard error carries everything said *about* it: dry-run
lines, declined and done notices, warnings, guidance, the questions a command
asks, and the error itself, prefixed `workflow:`.

While a slow command waits on a service — `doctor --online` on each
credential, `summary` on each source, `status DIR…` on each directory — it
keeps one line on standard error saying what it is reading (`Checking Jira…`,
`Reading GitHub…`, `Reading DIR…`), each replacing the last in place, and
erases it before printing anything and when it ends. It writes that line only
when standard error is a terminal, so a pipe, a redirect or a log never
holds it.

| Command | stdout | stderr |
| --- | --- | --- |
| `status` | the line, or one row per directory, or the JSON | each service that refused to answer — `Jira could not be read: …`, `GitHub could not be read: …` — then each that is not set up, with how to set it up, in the words the web's `not_set_up` problem gives — `GitHub is not set up: no forge token was found; …`, `The forge is not set up: origin does not name a repository on a forge; …` — each labeled with its directory when several are named; neither changes the exit status |
| `reviews` | one line per review, its number marked `#` (`!` on GitLab), or the JSON | "No pull requests are waiting on your review." ("merge requests" on GitLab) |
| `repositories` | a line for where it works, then one per worktree and one per favorite, or the JSON | why the worktrees could not be read |
| `doctor` | the report, or the JSON, and with no configuration file how to create one: the report is what a bug report pastes, so its guidance stays in it | |
| `config show` | the configuration as JSON, credentials masked | the file it came from (`# PATH`); how to create one when there is none |
| `config init` | with `--dry-run`, the file it would write, as JSON, masked | progress, the checks, "Wrote …", what to do next, a warning when the file is not ignored by git |
| `summary` | the summary as Markdown, or the JSON | with `--post`, `to …`, where it goes and how long the summary is against what the service takes; the dry-run line, "Not posted.", "Posted to …", each source left out as not set up — `Taskwarrior is not set up, so it was left out: …`, with how to set it up — and each source that could not be read |
| `issues` | one line per issue — its key (a forge issue's bare number), its summary and its status in parentheses — or the JSON | "No issues in VIEW.", "Listed the first N of TOTAL issues in VIEW." for a view longer than its first page, and "Not read: …" for each tracker that could not be read, which does not change the exit status |
| `issue` | the issue in full — key and summary, type · status · priority, who reported it and who it is assigned to, its link, its description, and each comment's author · date with its body indented — or the JSON | |
| `branch` | `Start work on KEY: create NAME from BASE and switch to it`, then `Created NAME`; with `--fetch` the plan opens `fetch REMOTE, then`, naming the base remote (upstream in a fork's clone, else origin); with `--worktree` it ends `in a new worktree beside the repository`, and the worktree's directory follows alone on the last line | the dry-run line, "Not created.", and with `--worktree` "Created NAME in a new worktree." |
| `pr` | `Open TITLE`, `BRANCH → BASE` — marked `(draft)` for a draft — and the reviewers asked, `(code owners)` unless `--reviewer` named them, a blank line, and the whole body; then `Opened #N URL` (`!N` on GitLab); with `--json`, the JSON alone | the dry-run lines, "Not opened.", the offers to link it on the issue and to move the issue to the review status, and their outcomes; with messaging set up, "Announce it with workflow announce." once it is open; with `--json`, the preview and the `Opened` line too |
| `finish` | `Finish BRANCH: #N merged` (`!N` on GitLab), then the three git commands it runs, each indented | the dry-run line, "Not finished.", "Finished BRANCH: switched to BASE, caught it up and deleted BRANCH." |
| `announce` | the message and where it goes | that an earlier session already announced this moment, the dry-run line, "Not announced.", "Announced to …", then, when the store could not remember it, "Posted, but not remembered: it may be offered again." and why |
| `comment` | `Comment on KEY:` and the comment as the tracker will store it | the dry-run line, "Not posted.", "Commented on KEY." |
| `transition` | `Move KEY to STATUS (MOVE)`, the move the tracker offers | the dry-run line, "Left KEY as it is.", "Moved KEY to STATUS." |
| `assign` | `Assign KEY to USERNAME` | the dry-run line, "Not assigned.", "Assigned KEY to USERNAME." |
| `worklog` | `Log DURATION on KEY`, and the `--comment` note indented beneath it | the dry-run line, "Not logged.", "Logged DURATION on KEY." in Jira's own words for the time |
| `slack login` | | the dry-run line, "Logged in to Slack as …" |
| `db-clean` | the store's directory and each database file in it | the warning before `--all` removes `kept.db`, the dry-run line, "Nothing to remove.", "Nothing removed.", "Removed …" |
| `workflow --web` | | the address it serves on; with no configuration file, the ways to set one up (Settings, `workflow config init`); why the configuration did not load cleanly, and the cause of each failure it answers as `internal`, credentials masked |

So `workflow config show | jq .` parses, and `workflow reviews | wc -l` counts
reviews and nothing else.

## JSON

`--json`, or `-j`, is on the reads: `status`, `reviews`, `repositories`, `summary`,
`issues`, `issue` and `doctor`; and on `pr`, for what it opened. `config show` prints JSON always. None of them carries a credential: `config show` masks
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
`running`, `passed`, `failed` or `none`. The `Review` stage reads `done` once
CI has passed with no changes requested, or once the pull request has
merged; a merged one has no CI left to read, so `ci` is then `none`. The last
stage is named for the messaging service `messaging.kind` configures —
`Slack`, `Teams`, `Discord` or `Webhook` — so read the stages by position
rather than by that name. That stage reads `done` once the branch's pull
request was announced at the moment it is at now — ready for review, CI red,
or merged — by `announce` or the interface, as the store remembers it; with
`store.disabled` it never does, nor for a pull request closed without merging,
which the loop no longer follows.
`issue` and `summary` are left out when the branch names no issue.

`workflow status --json DIR…` prints an array, one object per directory in
the order given, each with `dir`, the directory as it was given, and a
`repository` label: the name of the directory it leads to — `.` is named
for the directory the command runs in, `..` for the one above — or, where
two directories given lead to directories of one name, each one's path
written from your home. A directory that cannot be read has an `error`
instead of the stages — `"not a git repository"`, or why its repository
could not be read — and the command then exits non-zero after printing the
whole array.

`workflow reviews --json` prints an array, oldest first, of the requests
`GET /api/reviews` answers (see [Scripting the API]({{< relref "/docs/web#scripting-the-api" >}})):
`opened_at` is when each was opened, an RFC 3339 time, from which a script
works out how long it has waited, and is left out when the forge gave no time;
the line's `3d` is for reading. `facets` is the value each holds in the four
facets the terminal's and the browser's filter narrow the queue by — its
repository, its CI, draft or ready, and who asks — each with its `label`.

```json
[
  {
    "author": "ana",
    "ci": "passed",
    "draft": false,
    "facets": [
      {"kind": "repository", "label": "acme/api", "value": "acme/api"},
      {"kind": "ci", "label": "CI passed", "value": "passed"},
      {"kind": "draft", "label": "ready", "value": "ready"},
      {"kind": "author", "label": "by ana", "value": "ana"}
    ],
    "number": 42,
    "opened_at": "2026-10-05T09:12:00Z",
    "repository": "acme/api",
    "title": "fix(config): redact the webhook",
    "url": "https://github.com/acme/api/pull/42"
  }
]
```

`workflow repositories --json` prints the object `GET /api/repositories`
answers (see [Scripting the API]({{< relref "/docs/web#scripting-the-api" >}})):
`here`, where it runs (`dir`, `shown` — the path written from your home —
`root`, `root_shown`, `within`, `origin` and `config`, the configuration
files that apply); `worktrees`, the repository's working trees, the main one
first (`dir`, `shown`, `branch`, `head`, `state` — `here`, `worktree` or
`missing` — and `locked`), empty outside a repository; `worktrees_error`, why
they could not be read, or empty; `favorites` (`dir`, `shown`, `state` —
`here`, `repository`, `directory` or `missing` — and `origin`); and
`favorites_kept`, false when the store keeps nothing or under `--dry-run`. A
store whose favorites cannot be read lists none, and the command then exits
non-zero after printing.

```sh
workflow repositories --json | jq -r '.worktrees[0].dir'
```

`workflow issues --json` prints the page `GET /api/issues` answers for the
same view (see [Scripting the API]({{< relref "/docs/web#scripting-the-api" >}})):
`issues`, each with `key`, `tracker` — `jira`, or `forge` for the forge's
issues, whose key is the bare number — `summary`, `status`,
`status_category` (`new`, `indeterminate` or `done`), `type` and, when
there is one, `priority`; `total`, how many the view holds; `start_at`,
always `0`; and `unavailable`, each tracker that could not be read. It is
the view's first page: its first 50 issues and, on the first view when
`issues.forge` is on, the issues assigned to you on the repository's forge
ahead of them, which `total` counts too. A longer view's `total` says how
many more there are.

```json
{
  "issues": [
    {"key": "PROJ-7", "tracker": "jira", "summary": "Fix login", "status": "In Progress",
     "status_category": "indeterminate", "type": "Bug", "priority": "High"}
  ],
  "start_at": 0,
  "total": 1,
  "unavailable": []
}
```

`workflow issue KEY --json` prints the issue `GET /api/issues/{key}`
answers: the fields above, then `reporter`, `assignee` (left out when it is
unassigned), `description`, `comments` (`author`, `body` and, when the
tracker gave one, `created`, oldest first), `comment_total` and `url`, the
issue's page in its tracker.

```sh
workflow issue PROJ-7 --json | jq -r '.comments[-1].body'
```

`workflow summary --json` prints the object `GET /api/activity` answers
for the same period, read through the same seams, so a period gives the
same items, and names the same sources as unread or not set up, here, in the
Summary pane and in the web's Summary section: `from`, `to` and `today`
(`YYYY-MM-DD`); `sources`, each source asked (`source` — `git`, `tasks`,
`jira` or `forge` — `name`, `state`, `truncated` and `detail`). `state` is
`read`; `not_set_up`, for a source with nothing set up to ask — no Jira or
forge token, no forge the origin names, no Taskwarrior installed, no git
`user.email` — which is left out, its `detail` saying how to set it up; or
`failed`, for a source that is set up and could not be read, its `detail`
saying why. A `detail` never names a host; `years`, the items nested by `months`, `days` and
`hours`, each item with `at`, `source`, `verb`, `ref`, `title`, `url` and
`repository`; and `text`, the same as Markdown. `--from` and `--to` name the
first and last day, written `YYYY-MM-DD`; one alone is that day, and neither
is the previous working day — yesterday, or on a Monday the Friday and the
weekend after it. A source that could not be read is named in `sources`, and
the command then exits non-zero after printing, with the first family its
failures belong to: a Jira token refused exits 3. A source that is not set up
is left out, with a note on standard error, and does not change the exit
status: with nothing else wrong, `summary` exits 0.

```sh
workflow summary --from 2026-10-01 --to 2026-10-02 --json | jq -r '.years[].months[].days[].hours[].items[].title'
```

`workflow pr --json` prints what it opened as one object, the web's
`OpenedPullRequest` with the branches the pull request joins and its
body, and with whether each offer that followed was taken:

```json
{
  "pull": {
    "number": 7,
    "url": "https://github.com/acme/api/pull/7",
    "title": "fix(config): redact the webhook",
    "draft": false,
    "head": "fix/PROJ-412-redact",
    "base": "main",
    "body": "## Summary\n\nRedact the webhook URL.\n\nPROJ-412\n"
  },
  "follow_ups": [
    { "action": "link", "issue_key": "PROJ-412", "done": true },
    { "action": "transition", "issue_key": "PROJ-412", "status": "In Review", "done": true }
  ]
}
```

`warning` is added when the pull request opened but some of its reviewers
or assignees could not be. `follow_ups` lists the offers made — none when
the branch names no Jira issue — and `done` is false for one declined or
one that failed, which also fails the command after the JSON is printed.
Under `--dry-run` it prints the pull request it would open, with `number`
`0`, `url` empty and every offer not done. A pull request that was not
opened — declined, refused or failed — prints nothing.

`workflow doctor --json` prints the report as an object: `version`; `repository`
(`inside_work_tree`, `root`, `branch`, `detached`, `remote`, `forge`, or, outside
a work tree, `problem`: that git is not on `PATH`, that the directory is in no
work tree, or that the working directory cannot be read);
`tooling`, one entry per program (`name`, `found`, `required`, `effect`, and
`detail` where finding it took more than a look at `PATH`: the Taskwarrior
version and path found, or why none is usable);
`store` (`dir`, where the store keeps its files; `disabled`, `true` when
`store.disabled` turned it off; or `problem`, why no data directory could be
found, which leaves the store keeping nothing);
`configuration` (`path`, the file a save writes; `files`, every file read,
the home directory's before the repository's; `tracker` — `jira`, or `forge` when no `jira.base_url`
leaves the forge's issues as the tracker — `jira_url`, `jira_auth_mode`,
`messaging_target`, `messaging_mode`, `world_readable`, then `missing` and
`problems`: the required fields still empty and the values filled in wrong, each
`null` when there are none), or `config_problem` when the file did not load; and
`credentials` (`checked`, and with `--online` over a file that loaded,
`results`: `service`, `status` — `ok`, `missing`, `rejected`, `unreachable` or
`unchecked` — and `detail`). It exits as the prose report does: a field missing,
a value filled in wrong or a file other users can reach exits 3. A `rejected`
credential is one the service refused. A credential that is `missing` — none
configured, or a keychain item, `token_command` or `token_env` that gave
none — exits 3 as a
`rejected` one does, though it was never put to the service. A service that
answers with a redirect, or asks you to wait, is `unreachable` and exits 5: it
never judged the credential. A check `doctor` could not make is `unchecked` and
counts toward no exit status: a webhook, which only posting would test; Jira
when the forge's issues are the tracker, since there is no Jira to ask; and a
forge it cannot name — no repository remote, or a host other than
`github.com`, a `ghe.com` tenant or `gitlab.com` with no `forge.kind` and
`forge.host` for it. A `jira.base_url` that is not an https address, or http to
this machine, or that carries a login, and a `forge.kind` that names neither
forge, are refused when the file loads, so they fail the configuration and
nothing is checked.

`workflow config show` prints the configuration file's own shape, as
[Configuration]({{< relref "/docs/configuration" >}}) describes it.

## Writing without a person: `--yes` and `--dry-run`

`branch`, `pr`, `finish`, `announce`, `comment`, `transition`, `assign`, `worklog`,
`summary --post` and `db-clean` print a preview and ask before they write. Two
flags change that, each spelled the same short way on every command that takes it:

- **`--yes`** (`-y`) goes ahead without asking. On `pr` it answers every
  question: the push, the open, and the offers that follow it — to link the
  pull request on the branch's issue and to move the issue to the review
  status. A link
  that fails is said on stderr, the move is still made, and the command exits
  non-zero. On `announce` it never repeats an announcement: when the store
  says this pull request was already announced at the moment it is at, it
  says so on stderr, announces nothing, and exits 0 — run without `--yes` to be
  asked. With `--dry-run` as well, it still prints the announcement, and its
  dry-run line says it would not announce it again. On `summary` it answers
  only `--post`'s question, so it is refused without `--post`. A summary is
  posted even when a source could not be read — the text says which — and
  the command still exits non-zero for it.
- **`--dry-run`** (`-n`) prints the preview and what the command would do,
  and writes nothing, the store included: `announce` still reads what an earlier
  session announced, when there is a store on disk, but never creates it or
  changes what it holds (SQLite may leave its two companion files, `-wal` and
  `-shm`, beside it until the next session). What `pr` would do is a line for
  the push and the open, then one for each offer the open would lead to —
  `dry run: would link it on KEY` and `dry run: would move KEY to STATUS`.
  It is one flag for every command, given before the command's name or after
  it: `workflow --dry-run pr`, `workflow pr --dry-run` and `workflow pr -n`
  are the same.
  `config init --dry-run` runs its checks and prints the file it would write,
  masked, without writing it or storing anything in the keychain. Bare
  `workflow --dry-run` is the interface with every write held back.

A write run without `--yes` and without a terminal — stdin piped or closed —
has no way to be answered, so it stops, says to pass `--yes`, and exits 2.
`comment` reads its text from stdin to the end, so piped it always needs
`--yes`; at a terminal, end the text with Ctrl+D and the question is asked
after it. The guided `config init`, with nothing to answer its questions,
stops the same way and exits 2, saying to pass `--template`, which writes a
file to edit by hand without asking. Two
exceptions say to run it at a terminal instead: `announce` at a moment already
announced, which `--yes` would leave as it is, and `slack login`, which takes
no `--yes` because what it asks for are secrets.

## Posting elsewhere: `--channel`

`announce` and `summary --post` post in `messaging.channel`. With a Slack user
token, `--channel CHANNEL` posts the one message in `CHANNEL` instead — the
preview's `to` line, and the line that says it was posted, name it. A webhook
posts only to the channel it was made for, so there `--channel` exits 2.
What the store remembers is still the pull request at its moment, wherever it
was announced: once announced in one channel, `announce --yes --channel`
leaves it as it is, as it would in the same channel.

## Choosing what `pr` opens

`pr` opens the pull request it composes, as the interface does, unless its
flags say otherwise; each stands for an edit the terminal's composer and the
web's form allow:

- **`--title TEXT`** is the title in place of the one composed.
- **`--body TEXT`** is the whole description, in place of the one composed
  from the repository's template, the commits and the issue's link.
- **`--template NAME`** starts the description from the repository's template
  of that name rather than its first, as `ctrl+t` in the composer and the
  form's Template do. A name the repository has no template of exits 1,
  naming the templates it has; beside `--body`, which leaves a template
  nothing to start, it exits 2.
- **`--base BRANCH`** merges it into another branch, and drafts it against
  `BRANCH`: the title and body come from the branch's commits past `BRANCH`,
  and `BRANCH`'s code owners are asked to review, as the composer and the form
  draft again when their base is edited. `BRANCH` is read on the base remote
  when it holds one, else as a local branch. A name git would refuse for a
  branch exits 2 before anything is read; a name no branch has exits 1.
- **`--reviewer NAME`** asks `NAME` to review in place of the code owners;
  `org/team` asks a team. Repeat it, or separate names with commas.
- **`--draft`** opens it as a draft: the preview marks the branches `(draft)`,
  and the question asks to open the draft pull request.

`--json` reports what was opened, `base` and `draft` among it.

## A log for a bug report: `--log`

`--log FILE` appends a one-line outline of every request a command makes —
the time, the service, the method, the path, the status and how long it took
— to `FILE`. Like `--dry-run`, it is accepted before or after any command.
It records nothing else: never a header, a body, a query string or a host,
and never the path of a messaging webhook — Slack's, Teams', Discord's or a
plain one — which is itself the credential. The file is created readable only
by you. A line that cannot be written — on a full disk, say — neither stops
the command nor changes its exit status: as it ends, the command says once on
stderr that the request log could not be fully written.

```text
2026-09-22T18:04:11Z jira  GET  /rest/api/2/myself 200 184ms
```

`workflow --log requests.log doctor --online` is the run a bug report most
wants: every credential check, each under its service's name.
