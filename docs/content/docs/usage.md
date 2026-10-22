---
title: "Using workflow"
weight: 15
---

# Using workflow

Run `workflow` inside a repository. It opens on the issues assigned to you and
reads everything else — the branch, its changes, its pull request, its CI — from
the repository and the services it talks to. Where you are in the work is worked
out from the branch name each time, never stored. A small on-disk store does
remember a few conveniences between sessions — the commit scope you last used,
what you have announced, the last issue list — and the people and group
associations you decided, under your platform's data directory and never a
secret; set `store.disabled` to `true` to keep nothing on disk, and
`workflow db-clean` removes what is there.

```sh
workflow            # open the interface
workflow --dry-run  # the same, with every write held back
workflow --web      # serve the loop in a browser instead
```

The browser's side of it has [a page of its own]({{< relref "/docs/web" >}}).

The steps a script or a shell prompt wants also run as commands, without the
interface. [Scripting]({{< relref "/docs/scripting" >}}) names each, and says
what a script can rely on from them: exit codes, which stream carries what
and how standard error says it, `--json`, `--yes`, `--dry-run`, `--quiet`,
`--verbose`, `--log-format` and `--request-log`.

At a terminal the commands speak in one voice: a note is a plain sentence, a
warning starts `WARN` and the error a command ends with `ERROR`, each tag in
the terminal's own yellow or red; `--help` lays a command's usage, examples
and flags out in titled sections; and a command that waits on a service —
`doctor --online`, `summary`, `status DIR…`, `announce --when-green` — keeps
a live line of what it is doing, and how much of it is done, at the foot of
the terminal until it is, drawn in ASCII under `ui.ascii` and with its mark
still under `ui.motion: reduce`.

## The screen

Here is the whole loop over a made-up day's work: PROJ-412 picked up, its
branch made and the issue moved into progress, its changes staged, committed
and pushed, its pull request opened, and Slack announcing it once CI passes.

{{< image src="images/terminal.gif" alt="The terminal interface running the loop: starting work on PROJ-412 creates its branch and moves it to In Progress, the Commits pane stages its three changes and commits them, the Branch pane pushes the branch, the Review pane opens pull request #128, its code owners proposed as its reviewers, and watches its CI, and the Slack pane, its message shown in Slack's markup, announces it to #dev-workflow once the four checks pass, before the Issues pane is focused again" loading="lazy" >}}

The Slack pane shows the announcement as it will be sent, in Slack's own
markup: a link reads `<url|text>` there, and Slack shows it as just the text
([link markup]({{< relref "/docs/configuration#messaging-slack-teams-discord-or-a-plain-webhook" >}})).

Its parts, drawn as text, as the loop above ends and the Issues pane is
focused again:

```text
 api · ● Issue ━━━━━━━━ ● Branch ━━━━━━━━ ● Commits ━━━━━━━━ ● Review ━━━━━━━━ ● Slack
┏━ 1 Issues ━━━━━━━━━━━━━━━┓┌─ Issues ─────────────────────────────────────────────────────────┐
┃ ▸ ◐○◐ PROJ-412 In Progr… ┃│ PROJ-412 Redact tokens in the request log                        │
┃   ◐·◐ PROJ-418 In Revie… ┃│ Bug · High · In Progress                                         │
┃   ○○◐ PROJ-408 To Do Ca… ┃│                                                                  │
┃   ○·· PROJ-401 To Do Sl… ┃│ Tasks                                                            │
┃   ○·· PROJ-396 Backlog … ┃│   ○ #1 PROJ-412: Redact tokens in the request log                │
┃   ◐·· PROJ-390 In Progr… ┃│                                                                  │
┃   ◐·· PROJ-377 In Revie… ┃│ reported by Ana Lopez                                            │
┡━ 2 Branch ━━━━━━━━━━━━━━━┩│ assigned to Ana Lopez                                            │
│ fix/PROJ-412-redact-tok… ││                                                                  │
│ pushed                   ││ The request log records every header, so a bearer token lands in │
├─ 3 Commits ──────────────┤│ the log file.                                                    │
│ 0 of 0 staged            ││                                                                  │
│ 1 commit on this branch  ││ Redact the Authorization header before the line is written.      │
├─ 4 Review ───────────────┤│                                                                  │
│ #128 fix(config): redac… ││ Comments 1 of 1                                                  │
├─ 5 Slack ────────────────┤│                                                                  │
│ #dev-workflow            ││ Sam Ortiz · 2h ago                                               │
├─ 6 Reviews ──────────────┤│ Repro'd on main with --log; the token is in the third line.      │
│ 4 waiting                ││                                                                  │
├─ 7 Tasks ────────────────┤│                                                                  │
│ 3 pending                ││                                                                  │
├─ 8 Summary ──────────────┤│                                                                  │
│ what you did, read when… ││                                                                  │
├─ 9 Repositories ─────────┤│                                                                  │
│ ~/src/api                ││                                                                  │
└──────────────────────────┘└──────────────────────────────────────────────────────────────────┘
 ● announced to #dev-workflow
 t change status • b start work • / search • f filter • r refresh • ? keys • q quit …
```

- **The top row** is how far along the loop the work is, drawn as a track.
  `○` not started, `◐` in flight, `●` done, `✗` failed, and after each stage a
  rule: filled `━` in the stage's color once it is done, open `─` while it is
  still to come, and after Review filled as far as CI's checks have finished.
  Stages still to come are faint. It is derived, not recorded: the Issue stage
  is done once the branch names an issue, Commits is in flight while a
  feature branch holds changes not yet committed (changes on the base branch
  leave it not started, since no branch has been made for them), and Review
  follows the pull request's CI. The last stage is named for your messaging
  service, as its pane is. It starts with where you work, faint — the
  repository's name, as `api` at its root, and below it the path within it,
  as `api/cmd` — which gives way first when the row is short; the
  [Repositories](#repositories) pane says it in full. At its right end, while
  you have one started, is the [Taskwarrior](https://taskwarrior.org) task
  and how long it has run; the drawing above has none. When a stage
  completes, its rule fills left to right; `ui.motion: reduce` draws it
  filled at once.
- **The rail** on the left is the nine panes in one box, a light rule between
  them. The focused one is drawn with heavy rules and a bold title, and takes
  the most room; the rest keep a few rows each. The first five follow the
  work — the fifth is named for your messaging service, Slack above — and the
  sixth, Reviews, is the other side of it: the pull requests on your forge
  that wait on your review, the longest-waiting first until `O` sorts them
  another way. `f` filters them by repository, CI state, draft or ready, and
  author — values in one of those widen the list, and the four narrow it
  together, and a value checked stays, at zero, once no request holds it, and
  goes last when it is a repository or author — and a line above the queue
  names the sort and the filters while either is not the usual. The seventh,
  Tasks, is your own [Taskwarrior](#track-it-in-taskwarrior) list, the eighth,
  [Summary](#summary), what you did over a day or a range of them, and the
  ninth, [Repositories](#repositories), where you work and the directories
  you keep as favorites.
- **The detail pane** on the right shows the focused pane in full. Pickers,
  composers and previews open here too, and take the keyboard until they close.
- **Dialogs** float over the screen for the list of keys (`?`), every last
  look, and the checklists that narrow a list: centered in the upper third,
  the panes faint behind them, the top row and the bottom row as they were,
  the bottom row naming the dialog's keys. A click inside one picks its row;
  a click outside it does nothing. Below 60 columns a dialog is a sheet over
  the whole body instead.
- **The bottom row** shows what the focused pane can do right now; it changes
  with the pane and with what that pane has loaded. On a narrow terminal the
  keys that do not fit are dropped whole and an ellipsis says so: the keys
  that only move first, then the pane's verbs from the end; `?`, which lists
  every key, and the way out — `q`, or `esc` in an overlay — go last.
- **A result** — a push sent, an announcement made, a change refused — appears
  on its own row above the keys, where it stays while you look around and
  clears when the next action starts. Where that row would leave the focused
  pane fewer than four rows, as at 80 by 24, the result takes the bottom row
  instead, and a search being typed shares it with its keys.

Each pane fails on its own. A Jira that cannot be reached puts its reason in
the Issues pane, and the repository panes carry on.

The layout follows the terminal. Below 80 columns the rail and the detail pane
take turns rather than sharing the width; the detail pane drops its border once
it has fewer than 60 columns, and the rail then keeps the focused pane's heavy
rules, even while an overlay is open; below 60 columns or 24 rows the top row
shrinks to a short form, each stage its initial and glyph, with one-column
rules between them. Nine panes share the rail's rows: each
pane but the focused one keeps one row, a line of what it holds, until the
focused one has eight; rows past that give the others a second row each, top
first, and then go to the focused one, so a taller terminal never gives it
fewer. A terminal too short even for that splits the rows evenly.

## Keys

`tab` and `shift+tab` move between panes, and `1`–`9` jump straight to one.
Moving to a pane, by key or by click, reads it again when it last did so more
than 30 seconds ago, so flicking between panes asks Jira and the forge
nothing. Branch, Commits, Review and the messaging pane all read the branch,
its pull request and CI, so reading one of them again counts for all four;
the Issues list is left as it is while it holds further pages, which a reload
would drop. `r` reads the pane again whenever you press it. While a read is
out, the pane's title carries `◐` until its answer arrives, and a pane read
for the first time says `◐ reading…`.
`j`/`k` or the arrow keys move within a list, `home` and `end` or `G` jump
to its first and last row — or, on a pane with no list, to the top and the
bottom of the detail — and `J`/`K` or `pgdn`/`pgup` scroll the detail pane.
Each pane keeps its own place: come back to one and its detail is scrolled
where you left it, unless it shows another branch, issue or pull request,
which starts at the top, or its list reloaded while you were away, which
scrolls to keep the selection in sight. The table below holds every key `?`
lists, by where it works.

| Where | Key | Does |
| --- | --- | --- |
| Moving around | `tab` / `shift+tab` | Next pane, previous pane |
| | `1`–`9` | Jump to a pane |
| | `j`/`k` or `↓`/`↑` | Move within a list |
| | `home` / `end` or `G` | Jump to the first or last row of a list, or the top or bottom of a pane's detail |
| | `J`/`K` or `pgdn`/`pgup` | Scroll the detail pane, or half a page of a check's log |
| 1 Issues | `t` | Change the selected issue's status |
| | `c` | Comment on it |
| | `a` | Assign it |
| | `w` | Log work on it |
| | `b` | Start work on it: a branch for it, or a branch for no issue when none is selected |
| | `o` / `y` | Open the issue in the browser, or copy its URL |
| | `/` | Search the list as you type; `enter` keeps the search, `esc` clears it |
| | `f` | Filter the list by where issues are: a status, in flight, or how their tasks stand |
| | `v` | Switch which issue list is shown |
| | `ctrl+n` | Load the next page of the list |
| | `r` | Search again |
| | `T` | Track the issue in Taskwarrior, or go to the task that tracks it; offered once Taskwarrior has answered |
| | `enter` / `esc` | Below 80 columns, read the selected issue in full, then go back to the list |
| | `enter` | With no configuration file, set one up (see [Setting up](#setting-up)) |
| 2 Branch | `b` | Start a branch |
| | `s` | Switch branch: to the branch of another of your issues |
| | `i` | Link the branch to an issue, for work begun outside workflow, or unlink the one it is linked to |
| | `u` | Rebase the branch onto its base, after a last look |
| | `P` | Push a branch that has unpushed commits, after a last look |
| | `r` | Read the repository again |
| 3 Commits | `space` | Stage or unstage the selected file |
| | `a` | Stage every file |
| | `U` | Unstage every file, at once: the work tree keeps every edit |
| | `x` | Discard the selected file's changes, staged and not — an untracked file is deleted — after a last look at the file, since it cannot be undone |
| | `c` | Commit what is staged |
| | `A` | Amend the last unpushed commit with what is staged, after a preview |
| | `f` | Record what is staged as a `fixup!` of an unpushed commit you pick |
| | `h` | Run a hook lefthook configures — `pre-commit`, `pre-push` or one of its own — without committing or pushing |
| | `g` | Set up lefthook for hooks it does not manage |
| | `r` | Read the repository again |
| 4 Review | `n` | Open a pull or merge request |
| | `e` | Edit the open one's title and description |
| | `c` | List its CI checks, and open one's page in the browser |
| | `R` | Re-run failed CI, after a last look at the pull request |
| | `M` | Merge a green, approved pull request, after a preview of the methods the repository permits |
| | `F` | Finish a merged branch — switch to the base, catch it up, delete the branch — after a preview of the commands |
| | `o` / `y` | Open the pull request in the browser, or copy its URL |
| | `r` | Look for the pull request and its CI again |
| 5, your service | `p` | Preview the announcement of the pull request; while one waits for CI, the footer names it *announce now instead*, after `x` |
| | `P` | People and groups: whom each code owner is on Slack, and the user groups the repository tags; with a Slack user token and a store |
| | `x` | Stop waiting: drop the announcement waiting for CI to pass, unposted |
| | `r` | Read what was announced, and the pull request and its CI, again |
| 6 Reviews | `o` / `y` | Open the selected request in the browser, or copy its URL |
| | `O` | Sort them oldest first, newest first, or by repository |
| | `f` | Filter them by repository, CI state, draft or ready, and author |
| | `r` | Ask the forge again |
| 7 Tasks | `s` | Start the selected task, or stop it once started |
| | `d` | Mark it done, after a last look |
| | `a` | Add a task, typed in Taskwarrior's own grammar |
| | `A` | Annotate it |
| | `e` | Modify it, typed in Taskwarrior's grammar |
| | `u` | Undo Taskwarrior's last change, after a last look |
| | `S` | Sync Taskwarrior, when its taskrc names a sync backend, after a last look |
| | `/` | Search them as you type: description, project, `+tag`, issue key or `#id` |
| | `f` | Filter them by state, priority, project, tag, and whether they have an issue |
| | `O` | Sort them by urgency, state, id, tag, issue or priority |
| | `enter` | Go to its issue, when the Issues pane lists that issue |
| | `o` / `y` | Open its issue in the browser, or copy its URL |
| | `r` | Read the tasks again |
| 8 Summary | `[` / `]` | The period before or after, read once the key rests |
| | `t` | Today |
| | `c` | Pick a day, a month, a year or a range in the calendar |
| | `Y` | Copy the summary as Markdown |
| | `p` | Post the summary to your messaging service, after a preview |
| | `o` / `y` | Open the selected item in the browser, or copy its URL |
| | `r` | Read the period again |
| 9 Repositories | `s` | Switch to the selected directory, after a last look |
| | `f` | Add the selected directory to your favorites, or remove it |
| | `g` | Browse for a directory to switch to, in a picker that narrows as you type |
| | `S` | Settings: read and change the configuration the web's Settings edits; with no file, set one up |
| | `L` | Local data: the store's files, and removing them after a last look |
| | `r` | Read the favorites again |
| A composer or preview | `tab` / `shift+tab` | Next field, previous field |
| | `←` / `→` | Change the commit's type; in the announcement preview, change the channel; in the calendar's Day column, move a day; in Settings, move the selected choice on or back; in a form, turn a yes or no |
| | `ctrl+o` | Write the commit's body, the pull request's description, or a comment, in your editor |
| | `ctrl+x` | Mark the commit a breaking change |
| | `ctrl+t` | Use the repository's next pull request template |
| | `ctrl+r` | Open the pull request as a draft, or not |
| | `e` | Edit an announcement in your editor before it is sent |
| | `space` | Pick an option in a field that takes several; in Settings, turn the selected setting on or off |
| | `v` | Keep every existing hook whole as a script, in the lefthook offer |
| | `ctrl+g` | In the branch creator, create the branch in a new git worktree rather than switching to it |
| | `l` | In the checks list, show the end of a failed check's log, where a GitHub Actions run or a GitLab job keeps one |
| | `w` | In the announcement preview, announce once CI passes |
| | `u` | In the link form, on a branch already linked, unlink its issue |
| | `c` / `C` | In Local data, remove the cache, or everything, after a last look |
| | `ctrl+s` | In Settings, save every edit together; in a form open over Settings, keep what it holds |
| | `D` | In Settings, remove the selected view, prefix or moved key; on a stored credential or header, remove it from the file after a last look |
| | `j`/`k` or `↓`/`↑` | In the announcement preview, move between the code owners and groups it can tag |
| | `space` | In the announcement preview, tag the selected group, or untag it |
| | `a` | In the announcement preview, link the selected code owner to someone on Slack, or a team to a user group |
| | `x` | In the announcement preview, remember that the selected code owner is not on Slack |
| | `d` | In People and groups, forget what was decided for the selected owner, after a last look, so they are asked again |
| Writing a comment | `i` / `a` | Type before or after the cursor |
| | `A` / `o` | Type at the end of the line, or on a new line below it |
| | `h`/`l` or `←`/`→`, `j`/`k` or `↓`/`↑` | Move the cursor |
| | `esc` | Back to normal mode; in normal mode, close the box and keep the draft |
| | `enter` | Preview the comment |
| While a command runs | `s` | Stop it |
| | `r` | Run it again, once it has ended |
| | `o` | Show its full output, or every place a failed hook reported |
| On every pane | `m` | Turn mouse capture off or on, for this session |
| | `?` | Every key; in the help, close it |
| | `q` | Quit, from a pane or the help |
| On panes and in overlays | `enter` | Do what the bottom row names, wherever it names one; the help, a job's log and Local data name none |
| | `esc` | Leave without doing it; the bottom row says how: *discard* drops what you wrote, *close* keeps it — a composer says the draft was kept — *cancel* leaves a form or a last look unsent, *back* steps out of a nested step, *skip* passes an offer up, and *stay* leaves a guard |
| | `ctrl+c` | Quit from anywhere, even while typing or with a preview open |

Every key here but the pane numbers, `1`–`9`, can be rebound with `ui.keys`;
see [Configuration]({{< relref "/docs/configuration" >}}). A map that moves an
action live on a pane onto a pane number, or that gives two actions live in
one place — a pane, an open overlay, or the Branch and Commits panes, which
the `--web` page shows together, for two actions the page binds — the same
key, stops workflow from starting, and `workflow doctor` names it.

### Forms

A form — the fields a status change needs, the first run's questions, a
section of Settings, the pull request composer — asks its questions in
fields drawn one under another, the one with the keyboard barred down its
left. `enter` or `tab` goes on to the next field and `shift+tab` back to the
one before; `enter` on the last field applies the form. The pull request
composer is the exception: there only `tab` goes on to the next field, and
`enter` from any field goes on to its second form, or from the second opens
the pull request, as [Open the pull request](#open-the-pull-request) says. A
choice moves with `↑` and `↓`, `space` picks one of several, and `←`, `→` or
`space` turns a yes or no. A credential is masked as it is typed, a star a
character. A value the form cannot take is said under its field, in red, and
the form stays on it. The bottom row names what the field with the keyboard
answers, with `esc` always last to go on a narrow screen, and every one of
these keys follows `ui.keys`. With
[`ui.accessible`]({{< relref "/docs/configuration#accessible-forms" >}}) on, or
`ACCESSIBLE` set, a form instead takes the whole terminal and asks each
question as a plain prompt, then hands the screen back.

## The loop

### Pick up an issue

The Issues pane lists what is assigned to you and not done, most recently
updated first. The detail pane shows the selected issue's description and
comments. If the current branch names an issue, that issue is selected when
workflow opens, so you land back where you left off. `v` moves to the next of
the lists `jira.views` names; each is narrowed to the issues assigned to you
unless its query names the assignee itself, as
[Issue views]({{< relref "/docs/configuration#issue-views" >}}) explains.

Each row shows the issue's status by name, and a second mark once a branch
names any listed issue: in flight where a local or remote branch names it.
**Filter** (`f`) narrows the list to places: the statuses the loaded issues are
in, whatever your Jira workflow calls them, in its order — to do, then in
progress, then done, and by name within each, so a status keeps its place as
issues move — and the marks *in flight*, *task active*, *tracked* and
*task done*, each with how many issues it holds. `space` checks a place and
`enter` applies them; `esc` leaves the list as it was. Statuses widen each
other, as marks do, and the two narrow together, so *In development* with
*in flight* lists the issues in development that have a branch. The places
and the `/` search apply together, both over what is loaded, and the row
above the keys names them while they narrow the list. Switching view drops
them, as it drops the search.

**Change status** (`t`) lists the transitions Jira's workflow offers from the
issue's status. A transition that needs fields filled in says which. Choosing
it asks for them in a form, one at a time: a field with a fixed set of values
gets a picker, and one that takes several of them a picker where `space`
selects as many as you need, at least one; a text, user or date field gets an
input, a user by username and a date written `YYYY-MM-DD`. `enter` or `tab`
goes on to the next field, `shift+tab` back to the one before, `enter` on the
last one applies the change, and `esc` steps back to the transitions; each
follows `ui.keys` as it does everywhere else. A value Jira's field check
refuses is said under the field, in red, and nothing is sent. With
[`ui.accessible`]({{< relref "/docs/configuration#accessible-forms" >}}) on, or `ACCESSIBLE` set,
the form instead takes the whole terminal and asks each question as a plain
prompt, then hands the screen back. A field of
any other kind, such as a cascading select, is named with a pointer to Jira's
own screen, because guessing at it would send something you did not choose.

**Comment** (`c`) opens the comment box beside the issue, in normal mode, as
vim starts: keys there are commands, not text. `i`, `a`, `A` or `o` start
typing, and from then every key types — `q`, `j` and digits included — until
`esc` goes back to normal mode. In normal mode `enter` shows the comment as it
will be posted, `ctrl+o` hands the draft to your editor and back, and `esc`
closes the box. The draft is kept for that issue until you quit, so `c` picks
it up again; a search or a prompt elsewhere drops its text on `esc`, but the
comment box never does. In the preview `enter` posts it and `esc` goes back to
the draft.

**Without Jira**, when `jira.base_url` is empty, your forge's issues are the
tracker: the pane lists the open issues assigned to you on the repository's
GitHub or GitLab project. A branch for one is named by its number rather than
a key, as in `feat/42-fix-typo`, and that number is how workflow finds the
issue again. **Change status** (`t`) offers only Close. Assigning (`a`),
commenting (`c`) and opening or copying the issue's page (`o` / `y`) work as
for Jira. A comment on a forge issue is posted as written, since the forge
renders Markdown itself, whatever `jira.markdown_comments` says; on GitLab a
line starting with `/`, such as `/close`, runs as a quick action rather than
being posted, and the comment box says so. Log work (`w`), linking the pull
request on the issue and the `jira.views` issue lists (`v`) do not apply. `workflow doctor` names the
tracker in effect.

**Beside Jira**, a repository whose project tracks its work on GitHub or
GitLab can list those issues too: `{"issues": {"forge": true}}` in that
repository's `.workflow.json`, layered over your home file, puts the issues
assigned to you on its forge at the head of the first issue list, numbered
(`#42`) where Jira's are keyed (`PROJ-12`). Each issue goes to its own tracker:
a forge issue offers what one does without Jira, and a Jira issue everything
it always has. When the forge cannot be read, Jira's issues are listed and the
list says the forge's are missing.

### Branch

`b`, **Start work on** the issue, proposes a name from the selected issue: `fix/` for a bug and `feat/` for
anything else, then the key and the summary, as in
`fix/PROJ-412-token-redaction`. Edit it freely; a name git would refuse says so
as you type. The branch starts from the base remote's default branch — origin's,
or upstream's in a fork's clone — which the overlay names with how long ago it last moved, as in
`from origin/main, fetched 3d ago`. `enter` fetches the base remote first, so
the branch starts from what it holds now; if the fetch fails, the overlay says
why, and `enter` again branches from what you already have. The branch is not
set to track its base, so it reads as unpushed until it is. `ctrl+g` creates it
in a new git worktree beside the repository instead of switching to it, and
then offers to switch to the worktree: `enter` opens workflow again there, as
a switch from the Repositories pane does, and `esc` stays where you are.

`s` on the Branch pane, **Switch branch**, switches to the branch of another
issue. It lists the
branches that name an issue assigned to you and not done: the local ones, then
those only the remote has, marked `(remote)`, which switching to creates here —
from the base remote when both it and another remote hold it.
A branch whose issue is finished, or reassigned — to QA, say — leaves the list,
even one you have locally. When the tracker cannot be asked which issues are
yours, the list holds every branch that names an issue, under a line saying
why. A switch is refused while the working tree holds uncommitted changes,
rather than carrying them onto the other branch. git will not check out a
branch another worktree has checked out, so such a branch is marked with that
worktree, as `(worktree at ~/src/api-feat-x)`, and switching to it goes there
instead, as a switch from the Repositories pane does; one held by a worktree
whose directory is gone is marked `(worktree gone)`, and choosing it says that
`git worktree prune` frees it.

`i` on the Branch pane links the branch to an issue, for work begun outside
workflow on a branch whose name names none. It offers the issue selected in the
Issues list; type over it to name another, a Jira key or a forge number such as
`#42`. With a pull request open from the branch that workflow can edit, it
first shows the description with the issue's line added — `Jira: [PROJ-7](…)`, or `Closes #42`
for a forge issue — and `enter` links the branch and updates the description
together; a Jira issue is then offered the pull request's link, as opening one
would. The link is kept in the repository's git configuration, as
`branch.<name>.workflow-issue`, so it goes when git deletes the branch.

On a branch already linked, `i` names the issue instead, and `u` unlinks it at
once: only the link in git's configuration goes, and the pull request's
description is left as it is, so nothing leaves the machine and there is no
last look. Under `--dry-run` it says what it would have unlinked.

Which issue a branch is for is read, in order, from that link, from the
branch's name, and from its pull request's title or description, so a pull
request opened elsewhere that names its issue is followed too.

The Branch pane shows the branch, its base, its upstream and its commits. When
a push of it goes somewhere other than the base remote — a fork's clone pushes
to origin while its base is upstream's — a `push to` line names that remote,
and `workflow pr` names it in its question too, as in
`Push fix/PROJ-7-login to origin and open the pull request?`.

### Stage and commit

The Commits pane lists changed files, one per row. Staging is by whole file.
Beneath the list is the selected file's diff against the last commit, staged
and unstaged changes together, with each added line marked `+` and each removed
one `-`, so the marks read without color. An untracked file has no diff until
it is staged, and says so.

`c` opens the commit composer. The subject is built from its parts so it is
always a well-formed [Conventional Commit](https://www.conventionalcommits.org/):
`←`/`→` choose the type, `tab` moves to the scope and the description, and a
ruler counts against the subject limit, 72 characters by default
(`commit.subject_limit`). `ctrl+x` marks it a breaking change, and `ctrl+o`
writes the body in your editor. A trailer naming the issue — `Refs:` by
default, relabeled by `commit.refs_trailer` — is added unless the body already
has one.

With something staged and a commit not yet pushed, `A` folds the staged changes
into the last commit and `f` records them as a `fixup!` of one you pick, each
through the same hooks a commit runs.

`enter` runs `git commit`, so the repository's own hooks run exactly as they
would in a terminal. Their output streams into the detail pane as it happens.

### When a hook fails

A failed hook lists every `file:line` it reported. Pick one and press `enter`
to open your editor at that line; fix it, and `r` runs the commit again. A place
that names no file from the repository's root, as `go test` prints for a file in
a package, says so and opens nothing. What
you composed is kept, so nothing needs retyping. `esc` leaves the run and reads
the repository again.

`h` in the Commits pane lists the hooks of lefthook's configuration, merged
as lefthook reads it (`lefthook-local.yml` included), and runs the one you
choose as git would, without committing or pushing. `pre-commit` comes first,
so `h` then `enter` runs it. lefthook skips a pre-commit job when nothing is
staged, as a commit would, so stage something first. `pre-push` checks the
commits not yet pushed, against `@{push}` or, before the first push, the
default branch, and skips every job when nothing is unpushed. Run by hand,
`pre-push` gets neither the remote nor the refs a push would hand it, and Git
LFS's upload does not run. A hook git hands what it checks — `commit-msg` its
message, `post-checkout` the commits it moved between — is listed with that
reason under the ones you can run, and cannot be chosen.

### Open the pull request

`n` in the Review pane opens the composer. It first reads what the pull
request starts from — the templates, the remote branches, the issue for the
title and the code owners for the reviewers — its mark turning while it does,
so nothing it reads changes a field under the cursor. Then it asks in two
forms: where the pull request goes, the base branch and, where the
repository has more than one, the template; and, after `enter`, what it says:
the title, whether it is a draft, the reviewers, the assignees and the
labels. `enter` on any field of the second opens it, and `shift+tab` on its
title goes back to the first.

- **The title** is, by default, the branch's oldest commit subject, which on a
  branch of Conventional Commits already reads as one. Set
  `pull_request.title_source` to `issue` to start from the issue's key and
  summary instead.
- **The description** is the repository's pull request template, found where
  GitHub or GitLab looks for it, shown below the form. The first form chooses
  between several, the description following the choice as it moves, and
  `ctrl+t` moves on to the next from either; once you have edited the
  description, neither offers another template, so none replaces it. With no
  template it lists the branch's commits. Either way it links the branch's
  issue.
- `ctrl+o` edits the description in your editor — the form keeps no editor of
  its own — and `ctrl+r` marks it a draft, or not, from either form.
- **The base branch** starts as the one the branch left, and `tab` completes
  it to a remote branch. Change it and go on, and the branch is read again
  past the base you named before the second form asks: the title while you
  have not typed over it, the description while you have not edited it, and
  the reviewers while you have not typed them are proposed again from the
  commits that pull request would merge. A base no remote and no local branch
  holds keeps the draft as it was.
- **The reviewers** start as the code owners of the paths the branch changes,
  as the base branch's CODEOWNERS names them — people first, then teams as
  `org/team` — leaving you out. A draft you closed and reopen keeps the
  reviewers it had, or, closed before the owners answered and its reviewers
  never typed, reads them again. Edit them as any
  field; see [CODEOWNERS]({{< relref "/docs/configuration#codeowners-proposes-the-reviewers" >}})
  for which file is read and how.

`enter` pushes the branch first if it is not pushed — pre-push hooks stream just
as commit hooks do — and opens the pull request only if the push succeeded.

With messaging set up, opening a pull request also arms its announcement: it
waits for CI to pass, as `w` in the announcement preview does, then posts to
the default channel, tagging those `workflow announce --yes` would propose —
the linked code owners of its changes against the base it opens into, and the
groups that start checked — as far as the Slack token's scopes let a preview
tag them. The notice says so, and the messaging pane shows it waiting; `x`
there stops it, and `messaging.auto_announce: false` turns arming off. A draft
is never armed: the notice says to announce it once it is ready.

Reviewers, and on GitLab assignees, are added best effort. A reviewer the forge
will not take — a name GitLab does not know, or someone GitHub cannot ask — or
an assignee GitLab does not know does not stop the pull request: it opens with
every reviewer and assignee the forge took, and a note names the ones it left
off.

The Review pane then follows CI while checks run, asking every twenty seconds
by default (`timing.ci_interval`). `c` lists the checks and opens the selected
one's page; `l` on a failed one shows the end of its log, where a GitHub
Actions run or a GitLab job keeps one, and `J`/`K` or `pgdn`/`pgup` move
through a log taller than the pane half a page at a time. `R` re-runs the
failed ones, and `u` on the Branch pane rebases the branch onto its base; like
a push, each first shows a last look naming what it acts on, and does nothing
until `enter`. `e` edits the pull request's title and description. Once it is
green and approved, `M` previews the merge methods the repository permits and
merges by the one you choose; once it has merged, `F` previews the three git
commands that finish the branch — switch to the base, catch it up from the base
remote (upstream in a fork's clone, else origin), delete the branch — and runs
them on `enter` (`workflow finish` does the same from a shell). The delete is a
force delete, so `F` is offered only when it loses nothing — the branch is at
the commit that merged, or its upstream holds every commit on it — and the pane
says why not otherwise. `n` stays on offer after a merge, as `workflow pr`
allows, so commits made on the branch since can go up in a new pull request.

### Announce it

`p` in the messaging pane — named for your service, as the top row's last stage
is — previews the announcement:

```text
jacob opened a pull request: <https://github.com/…/pull/42|fix(config): redact tokens>
<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction
```

`e` edits it in your editor, `enter` announces it now, and `w` announces it once
CI passes; with a Slack user token and more than one channel to choose from
(`messaging.channels` names the others), `←`/`→` change the channel. An
announcement waiting for CI waits through a failed run: the messaging pane says
*a check failed; waiting on a fix*, and it goes once a later run passes. While
no check is reported it says *no checks reported yet*, and after ten minutes
with none, counted from the last read that saw one, it gives up, saying to
announce it by hand. `x` on the messaging pane stops waiting, and nothing is
announced. While one waits, the footer offers `p` after `x`, as *announce now
instead*: it opens the same preview, and announcing from it replaces the one
that is waiting, so the channel never reads it twice. One armed as `n` opened
its pull request waits the same way, and so does
`workflow announce --when-green` from a shell, in the foreground until CI passes
or `ctrl+c`.

Nothing is announced while a check has failed on the pull request. The
announcement reads its CI again as it goes, whatever the pane last read, and a
failed check keeps it from going, ready or merged alike: the preview stays open
saying why. While the last read found a check failed, the preview offers only
`w`, which waits for a fix. A merge has no CI to wait for: its refusal says to
re-run its failed checks on the forge, then announce it once they pass. A CI
that cannot be read keeps it from going too, since nothing failing cannot be
confirmed. `workflow announce` and the `--web` page hold to the same rule.

#### Tag the code owners

With a Slack user token, the ready-for-review announcement also tags people:
the code owners of the paths the branch changes, as CODEOWNERS on the base
names them, and the Slack user groups the repository tags. The preview lists
them under the text:

```text
to  #dev-workflow
Code owners
▸ ben                  ? not linked
  carla                → Carla Diaz
  dan                  · not on Slack
Groups
  [ ] @api-reviewers
  [x] @control-plane-pod   owns changed paths
tags  @Carla Diaz @control-plane-pod
```

A forge username is not a Slack user, so each owner is asked about once.
`j`/`k` move between the rows; `a` on an owner opens a list of the channel's
members — for a team, the workspace's user groups — narrowed as you type, with
a "Not on Slack" row; `x` says the owner is not on Slack. Either is saved at
once, for every repository on the same forge host, and the row changes to
show it. People and groups, below, changes it later.

`space` tags a group or untags it. The groups offered are the repository's
own, chosen in People and groups, and any a team owning the changes is linked
to, which start checked; the
rest start as you left them last time. The `tags` line names everyone the
post will tag, and the post ends with a line tagging them. A post `w` holds for CI keeps the tags it was given, so `w` waits until
whom to tag has been read.

Only the ready-for-review announcement tags anyone. Tagging needs the Slack
scopes `users:read`, `channels:read`, `groups:read` and `usergroups:read`;
a token without one says which. Without `usergroups:read` it still tags the
people it can and offers no group; without any other, it posts untagged.
Links are kept per Slack workspace, so a token whose workspace Slack will not
name tags no one: the preview says why, and the post goes out untagged. A
directory Slack will not read, or a link that will not save, is shown and
never holds the post back.
`--dry-run` reads whom each owner was linked to, if a store is already on
disk, but keeps nothing new: it offers no linking, nor People and groups, and
says whom the post would have tagged.

#### People and groups

`P` in the messaging pane opens People and groups, offered with a Slack user
token and the store on. `tab` switches between its two tabs:

```text
‹People›  Groups

▸ carla                → Carla Diaz
  dan                  · not on Slack
  acme/control-plane   → @control-plane-pod
  ben                  ? not asked yet
```

People lists every owner decided on this forge host, then the owners of this
branch's changes not asked about yet. `enter` changes whom the selected owner
is, from the same list the preview offers; `x` says they are not on Slack;
`d` forgets them, after a last look, so the next announcement asks again;
`esc` on the look goes back to the list.

Groups is a checklist of the workspace's user groups, the ones this
repository tags checked. `space` checks or unchecks one, and `r` reads
Slack's directory again, for a group or a person added since it was last
read. Nothing can be checked until the repository's own groups are read, nor
when they cannot be, since each change saves the whole list. Every change is
saved at once, so `esc` loses nothing; one Slack or the store refuses stays
under the title until the next, and the checklist goes back to what is kept.

An announcement waits for the pull request it was written for, and no other.
Switch to another branch while it waits, or replace the pull request, and it is
dropped, saying so, rather than sent for something you never previewed. A pull
request is announced once at each moment — ready for review, then merged —
and the messaging pane and the top row say whether the one on screen has been.
One closed without merging is none the loop follows: both read it as not
announced, whatever was announced of it before, and the messaging pane, like
`workflow announce` and the `--web` page, has nothing to announce until a pull
request is open again. The store remembers what was announced, so a later
session does not offer the same announcement again. One it cannot remember — a
full disk, say — is posted all the same, and the notice adds *Posted, but not
remembered: it may be offered again.* and why; `workflow announce` says the
same, and the `--web` page the sentence alone.

### Track it in Taskwarrior

With [Taskwarrior](https://taskwarrior.org) 3.5.0 or newer installed, the
Tasks pane is your own task list beside the loop, and a task can track each
issue. workflow runs Taskwarrior's `task` for every read and write, keeps
nothing of its own, and changes a task only when you press a key or accept an
offer. Without a usable Taskwarrior the pane says why — not installed, turned
off, or a `task` on `PATH` that is another program — and the rest of the
interface carries on as before;
[Configuration]({{< relref "/docs/configuration#taskwarrior" >}}) says how it
is found.

**Tracking an issue.** `T` on the Issues pane opens a line that adds a task
for the selected issue, filled in with Taskwarrior's grammar:

```text
jiraid:PROJ-412 jiraurl:https://jira.example.com/browse/PROJ-412 +jira priority:H -- PROJ-412: Fix token redaction
```

`jiraid` and `jiraurl` link the task to the issue, `+jira` tags it, and
`priority:H` is the issue's priority as Taskwarrior's `H`, `M` or `L`. That
word is added only when the issue has a priority that maps to one of the three
([Configuration]({{< relref "/docs/configuration#taskwarrior" >}}) has the
map); for an issue with no priority, or one that maps to none, the line has no
`priority:` at all. Edit the line as you like; `enter` adds the task and then
annotates it with the issue's page. On an issue a task already tracks, `T` goes
to that task in the Tasks pane instead, and the bottom row names it "go to
task"; when the pane does not list that task — it waits, or the active context
hides it — a notice names the task and why.

**In Taskwarrior's words.** `a` (add), `A` (annotate), `e` (modify) and `T`
each take a line typed as it would be after `task add`, `task 12 annotate` or
`task 12 modify` in a shell, attributes such as `project:web`, `due:friday` or
`+review` among the words. It is not read as a shell reads it, though: the line
is split at its spaces, and each word goes to Taskwarrior as an argument of its
own, so runs of spaces collapse and quotes are passed on as they are typed. A
value therefore cannot hold a space — `project:"my web"` is two words, quotes
and all — and an apostrophe, as in `don't`, needs no quoting. Words after `--`
stay words, which is how the track line keeps a summary such as
"Fix due:tomorrow" in the description rather than setting a due date; an
annotation is taken as words throughout. A line Taskwarrior refuses stays open
under its reason, to fix and send again.

**The Tasks pane** lists your pending tasks in Taskwarrior's active context,
most urgent first: those for an issue the Issues pane lists, then the rest
under a faint **Other**. A waiting task is counted at the foot rather than
listed. Each row says whether the task is started (`◐`) or not (`○`), its id
and what it is, then faintly its issue, when it is due, and its urgency. Below
the list is the selected task: its facts, the issue it tracks — with the
branch and its pull request when the checked-out branch names that issue — and
its annotations. The rail counts what is pending and started, and names the
active context, as the pane's title does.

`O` sorts the tasks within each group, cycling from most urgent first through
by state (started, then pending, then waiting), by id, by tag, by issue (in
natural order, so `PROJ-2` comes before `PROJ-10`, with unlinked tasks last)
and by priority (`H`, `M`, `L`, any other value your taskrc allows, then
none). A faint line above the rows names
the order, every tie stays most urgent first, and when the order is by tag or
by priority each row's tail shows it, as the filter names it: `no tag` or `no
priority` for a task with none. The order lasts for the session, through
every read.

`/` searches the list as you type, as the Issues search does: a task stays when
its description, project, a tag written `+tag`, its issue key or its `#id`
holds the text, ignoring case; `enter` keeps the search and `esc` clears it.
`f` opens **Filter**, a checklist of every state, priority, project and tag the
tasks hold, and whether they have an issue, each with how many tasks hold it (a
waiting task counts only toward its state): values checked in one group widen
the list, and the groups narrow it together. A value checked stays, at zero,
once no task holds it, so it can be unchecked, and a named project or tag, or a
named priority other than H, M and L, that no task holds any longer comes last.
Checking **waiting** lists the waiting tasks, each saying until when. The faint
line above the rows names what narrows the list; the rail still counts every
pending task, and a task the list hides still tracks its issue, so `T` on that
issue names it and why it is hidden. A filter that leaves nothing says "No task
matches the filters."

`d` (mark done), `u` (undo) and `S` (sync) each first show a last look and
do nothing until `enter`: marking done runs the task's hooks, Taskwarrior has
no redo for an undo, and a sync sends your tasks off the machine. `s` starts
and stops at once, since the other undoes it.

The pane sends one change at a time, so an undo never races the change before
it. While one is on its way the rail reads `◐ sending…`, and the keys that
change a task — `s`, `d`, `a`, `A`, `e`, `u` and `S` — leave the bottom row and
do nothing until Taskwarrior answers; moving, `enter`, `o`, `y` and `r` still
work. `T` on an issue no task tracks waits the same way, and an offer accepted,
or a line sent, meanwhile stays open and says to try again once it answers.
Each change reads the tasks again once it has answered; until that read
answers, a task just marked done is still listed as it was, and `s` and `d`
leave the bottom row and do nothing on it.

**Marks on the issues.** Once Taskwarrior has answered, each Issues row carries
a second glyph after its status: `◐` a task for the issue is started, `○` one
is still to do, `●` every task for it is done, and `·` none tracks it. The
issue's detail gains a **Tasks** block that lists each of its tasks — started
how long ago, or when it is due — or says that `T` tracks it. Until
Taskwarrior answers, and wherever it cannot — go-task on `PATH`, a read that
failed — there is no column and no block, rather than a claim that no task
tracks the issue.

**The started task** ends the top row, in cyan:
`◐ PROJ-412: Fix token redaction · 1h12m`. Short of room the description is
cut with an ellipsis, and then left out for `◐ 1h12m` alone, before the
stages give up their names; the short top row shows only `◐ 1h12m`.

**Offers at the loop's moments.** As the loop reaches a moment a task follows,
workflow offers the matching change: a last look at it, or, for an issue no
task tracks yet, the track line with the new task to be started once added.
`enter` makes it, and `esc` leaves Taskwarrior as it was. It never makes one
unasked.

| Moment | Offer |
| --- | --- |
| A branch created for an issue with `b` — not one made in a worktree with `ctrl+g` — or switched to with `s` — not one another worktree has, which `s` leaves for | Start the issue's task, or, with none, track the issue and start the new task |
| A pull request opened for the branch's Jira issue | Annotate its task with the pull request's number and URL |
| That pull request merged with `M` | Mark the task done |
| The issue moved to a done status with `t` | Mark the task done |

The offers keep one task started at a time: when another is — another
issue's, or one that tracks no issue — the offer is to **Switch the task**: it
names each task it stops first and the one it then starts, or, for an issue no
task tracks, stops them and then opens the track line. `s` in the Tasks pane,
and **Start** on the web, start a task without stopping any other; with more
than one started, the top row shows the first. No start is offered for an issue
whose own task is already started, and nothing is offered before Taskwarrior
has answered, or without it. Until the read after a done answers, the task
just marked done is offered nothing — no start, no note, no done again — and
is not counted as started, so a switch made meanwhile stops nothing on its
account. An offer for its issue goes to another task that still tracks the
issue, where one does, and otherwise nothing is offered, not even a new track.
An offer that comes while something else is being
asked — the status picker after a new branch, the link and review-status offers
after a pull request — opens once that closes. Starting a task runs
Taskwarrior's hooks, as `task start` in a shell does, so a Timewarrior hook
starts timing too.

## Summary

The Summary pane (`8`) is what you did, read back from where the work left a
trace: the commits you wrote in this repository and in every favorite (pane
`9`) that is one, the Taskwarrior tasks you
added, annotated and completed, and the ones you started that are still
going, the Jira issues you reported, moved, logged work on and commented on,
and the pull or merge requests you opened, had merged and reviewed on your
forge, in any repository. Nothing is kept: it is read again each time, and only
when you look, never at startup.

It opens on the previous working day — yesterday, or on a Monday, Friday and
the weekend after it — grouped by year, month, day and hour, oldest first.
`[` and `]` move to the period before or after — a whole month to the month,
a whole year to the year, any other period by its own length — and read it
once the key has rested, so holding one reads only where it stops; `t` shows
today. `c` opens a calendar of three columns, Year, Month and Day: `tab` and
`shift+tab` move between them and `j`/`k` within one — a year, a month, or in
the Day column, drawn as a month of weeks, a week, where `←`/`→` move a day —
and `enter` shows what the column the
cursor is in names — the whole year, the whole month, or the day; `space`
marks the cursor's day as one end of a range, and `enter` then shows from it
to the cursor, up to a year and a day. Each source fills in as it answers, and
one that cannot be read says so above the rest, as does one that had more
than it gave. A source that is not set up — no Jira or forge token, no forge
the base remote names, no Taskwarrior installed, no git `user.email` — is left
out, and says so in the muted color with the not-started mark `○`, with how to set
it up, rather than as a failure: nothing was asked, so nothing refused. A
period that has ended and has been read in full — every source answered, none
failed or left out — is not read again when you come back to the
pane; `r` reads it again. `Y` copies it as Markdown, and `o` or `y` opens or
copies the link of the item the cursor is on.

Once every source has answered, `p` posts the summary to your team. It opens a
preview of the Markdown and where it goes — the channel, which `←`/`→` change
when Slack offers more than one, or the channel a webhook is bound to — and
nothing is posted until `enter`; `e` edits the text first, and `esc` discards
it. The post shows the headings and the list as the service does: on Slack, a
heading is a bold line and an item a bullet. The preview says how long the
post is against the most the service takes — 2,000 characters on Discord,
40,000 on Slack, a 28,000-byte payload on Teams — and a summary longer than
that is refused with how long it is, and nothing is sent: pick a shorter
period. Nothing about the post is kept, as an announcement is. `workflow
summary --post` posts the same from a script.

Commits are your own, told by the `user.email` git commits under, and placed
by when you wrote them, so a commit rebased since keeps its hour; a repository
with no `user.email` says so rather than showing everyone's. Each repository
is read once, however many of its directories or worktrees are favorites, and
a commit two of them share — a fork, a second clone — is listed once. When more
than one is read, a commit is named as GitHub names one in another repository,
`acme/web@abc1234`, by its origin's path or else its directory's name; two
that would read alike are told apart by the end of their directories,
`…/work/api` and `…/oss/api`, marked so neither passes for a forge repository. One that
cannot be read is named in why git could not be read, and the others are
listed still. Jira finds the
issues you touched by JQL, which cannot ask for a comment alone, so a comment
on an issue you did nothing else to is not listed. Jira and GitHub are asked
for the issues and pull requests touched earliest first, so a period some way
back is found among them; one with more than is read says so. Taskwarrior
forgets when a task was started once it is stopped or done, so a start shows
only while the task is still going.

## Repositories

The Repositories pane (`9`) is where workflow works, relative to what it
changes: the directory it was started in, the repository that directory is
in and the path within it, origin's host and path — and, in a fork's clone,
the base's beside it — and the configuration files that apply there — the repository's over your home's — each written
from your home, as `~/src/api`. Outside a repository it says so, and the
panes that need one say what they have no data for.

Below that is the directory you work in, as the list's first row, then the
repository's other worktrees, as `git worktree list` lists them, each with the
branch it has checked out — or the commit, when its HEAD is detached — and
whether git keeps it locked, or finds its directory gone, a locked one's
included. They are read again each time the pane is opened, so one made or
removed since shows.

Then come your favorites: every other directory you marked, each with what is
there now — a repository and its origin, a directory that is not a repository,
or one that is not there any more. A favorite that is one of the worktrees is
starred there rather than listed twice. `f` adds the directory the cursor is on to your favorites, or removes
it. Favorites are kept in the store's `kept.db`, by path alone; whether one is
still there is read from the disk each time the pane is opened, and they are
not read at startup. Under `--dry-run` the pane lists them and says what `f`
would have done; with `store.disabled` there are none to keep.

`s` switches to the worktree or directory the cursor is on — not to a worktree
whose directory is gone — and `g` opens a picker to browse for one. Its field
takes a path from where you work, as `../web`, or from your home after a `~`, as
`~/src/web`, and lists the directories where it points, narrowed as you type:
those whose names start with what you typed first, then those holding its
letters in order, the letters matched in bold and a repository marked in yellow;
a hidden one once you type its dot, beside the others holding what you typed.
Opened empty, it lists the other worktrees and the favorites first, leaving out
those whose directory is gone. In the field every key types, `j` and `q`
included: `↑` and `↓` move the highlight, `home` and `end` move in the text, `→`
at its end goes into the highlighted directory, `backspace` there goes up a
level once nothing follows the last `/`, and `enter` goes into a directory, or
toward switching to a worktree, a favorite or "↳ switch here": where the field
points, or the path typed whole while no directory listed matches the name it
ends in. Once a name follows the path, the directories matching it come before
"↳ switch here", so `enter` takes the first of them. `tab` hands the keys to the
list, where `j` and `k` move, `←` and `→` go up and in, `s` switches to the
highlighted row, a listed directory included, and a letter goes back to the
field and is typed. A directory of more than a thousand is listed again narrowed
by the name you type, keeping beside what that finds those of the first thousand
that hold its letters in order; past 200 that match, the rest are counted, and
typing more narrows them. Every switch, from either key, first asks "Switch to
DIR?" and does nothing until `enter`; `esc` stays, or from the picker goes back
to it. `enter` on a row switches nothing, so a double press cannot ask and then
switch at once. A switch ends the interface and opens it again in that
directory, wired to it as if workflow had been started there: its repository,
its forge, the configuration files that apply there and the store's keys all
follow. A directory not there, or a configuration whose `ui.keys` the interface
would refuse, leaves it where it was, saying why.

What belongs to the session rather than the repository goes with you: the
comments you were writing on Jira issues, when the directory switched to uses
the same Jira, how the Tasks list is sorted and narrowed, and the Summary's
period. What belongs to the repository left — a
commit message or a pull request being written, a comment on one of its forge
issues, an announcement waiting for CI — would be lost, so the switch's last
look names it too; and while an announcement or a change to a task is being sent,
the switch waits for it.

## Working from a fork

A fork's clone has two remotes: `origin`, the fork you push to, and the
repository it was forked from, which pull requests merge into. workflow asks
git which is which rather than keeping a setting of its own.

- **The base remote** is the remote gh marked as the base
  (`remote.<name>.gh-resolved` set to `base`, which `gh repo set-default`
  writes), else a remote named `upstream`, else `origin`.
- **The push remote** of a branch is `branch.<name>.pushRemote`, else
  `remote.pushDefault`, else the remote it tracks (`branch.<name>.remote`, as
  `git push -u fork` records), else `origin` — the order git itself pushes by.

What follows the base remote: the base a new branch starts from
(`upstream/main` in a fork's clone), the fetch Start work makes first, the
commits counted as the branch's own and the base a rebase replays onto, the
CODEOWNERS the reviewers are proposed from, the pull a finish catches the base
up with, and which remote's copy switching to a branch both remotes hold
creates here. What follows the push remote: the push, whether the branch reads
as pushed, and the `push to` line the Branch pane shows when it is not the
base remote.

The forge is reached at the base remote too. The pull or merge request opens
on the base remote's repository from the branch on the push remote's fork —
on GitHub with an `owner:branch` head (and `head_repo` for a fork in the base's
own organization), on GitLab as a merge request posted to the fork into its
parent — and is found there again. Its CI, reviews and merge are read there,
and a GitLab pipeline that ran in the fork has its failed jobs, its retry and
its logs read in the fork. Issues and pull request templates come from the base
too. A push remote whose address names no repository on a forge — a local
mirror, say — is refused, naming it, rather than guessed at.

Merging and re-running checks stay offered, though a contributor seldom has
the rights to either on the base: the forge's refusal says so, naming the
scope it wanted. A first-time contributor's GitHub Actions run may wait for a
maintainer's approval, and reads as none or running until then.

What workflow keeps stays keyed by `origin` — the last scope, announcements,
the issue cache, whom a forge owner is on Slack, favorites — so it is shared
with every other clone of your fork. The Repositories pane, the web's Working
in and `workflow doctor` name the base beside origin.

With neither `upstream` nor gh's mark, the base remote is `origin`, and
everything reads as it does in a clone that is no fork.

## Setting up

With no `.workflow.json` to read, the Issues pane says so and `enter` sets one
up there, as `S` on the Repositories pane does; `workflow config init` asks the
same questions at a prompt. It asks them in a dialog over the screen, one at a
time, in [forms](#forms): where the file goes — the repository, so it applies
across it, or your home directory, so it applies everywhere — then Jira's
address and your personal access token, which is masked as it is typed and
checked with Jira while the dialog's mark turns. A check that does not pass
names why and offers to type the token again, keep both anyway, or leave Jira
out; an address that is not an https address, or
http to this machine, or that carries a username and password, is never kept,
so it offers to type the address again or leave Jira out. With the OS
keychain wired (macOS), it then asks where to keep the token: in the keychain
item for Jira's address, so the file holds only `"keychain": true`, which
reads it back from there, or in the file, which only you can read. It asks
for either file, since an item is only ever read for its own address. Last comes
a Slack incoming webhook, masked too, saved unchecked, or left blank to post
with your Slack user token after `workflow slack login`. A blank address or
webhook skips that question, and `esc` goes back one, as `shift+tab` does on
the last look.

`enter` on the last look writes the file and reopens workflow in the same
directory with it, on the Issues pane, as a save in Settings does; a file in a
repository that git does not ignore is named, since it holds credentials. A file
already there is never written over. Under `--dry-run` the last look says what
it would have written.

## Settings

`S` on the Repositories pane opens Settings: the configuration the web's
Settings edits, in the same eleven parts — Jira, messaging, the forge,
commits, branches, pull requests, the store, Taskwarrior, timing, the terminal
and the keyboard — a row per setting, with what the selected one does below. A
credential is shown masked, as `****9999`, and typing a new one shows only a
mask. `enter` on a row opens its part's settings together in a
[form](#forms), on the setting the row stands for — a credential's field
starts empty, and left empty keeps the stored one — and only what you change
in it becomes an edit. `ctrl+s` keeps the form from any field, as `enter` on
its last does, and `esc` leaves it keeping nothing. `space` turns a setting on
or off on its row, and `←` and `→` move a choice there; `ctrl+s` on the rows
saves every edit together.

Jira's views and headers and the branch prefixes are a row each, after them
a row that adds one: `enter` there asks the new one's name and its value
together (a view's JQL, a header's value, which is masked, a type's prefix),
refusing a name already listed when you go on from it. `enter` on one edits
its value, and `D` removes it with the next save. The channels
are typed comma-separated, as the commit types are. The keyboard part lists
every action the interface binds, each with the key it is moved to, or the
default it keeps; `enter` types a key, and `D` puts its default back.

`D` on a credential the file holds — the Jira token, the forge token,
Slack's client secret and refresh token, the webhook URL, or a Jira header —
asks first, then writes the file as Settings read it without that credential,
at once: it cannot be undone, and removing the client secret or the refresh
token takes the access token made from them too. With no other edits,
workflow then reopens as after a save; otherwise Settings stays open, says
what was removed, and keeps your edits for `ctrl+s`.

A save is checked as a file on disk is, and as the web's is, and nothing is
written while anything is refused. Each setting refused, by that check or by the
write after it, is marked under its own row with why — a list under its first
row — and the cursor goes to the first of them. The marks stay as you move, and
changing a refused setting takes its mark away. A save held back by that check
leaves what the write refused marked, with the write's reason where both refused
one setting, and goes to the first setting the check refused that the write had
not, or, when there is none, to the first the write refused; a save that is
sent takes every mark away, and the write judges every setting afresh. A
refused removal keeps every mark, since it writes the file as read, without
your edits; it marks each setting it refuses beside them, with its own reason
where one was marked already, and goes to the first of those. While a form is
open over the rows, hiding their marks, each refusal still standing is said
below it instead, named by its section and setting, as Jira › Project. A
refusal no row stands for, such as a `ui.keys` map the interface would not
start on, is said below the rows. A save writes the file Settings read, the
repository's over your home's as described in
[Configuration]({{< relref "/docs/configuration" >}}), and only while that file
is as Settings found it: one changed since — edited on disk, or saved from the
web — is not written over, and `r` reads it again in place of your edits.
Slack's user-token secrets are kept where the web's Settings keeps them, and so
is a Jira token you type: on macOS, in the keychain item for the Jira address,
the file saved reading it from there, which the row under the token turns on and
off. Once saved, workflow reopens in the same directory, as a switch to another
one does, so what was saved applies at once; the session's Jira comments, Tasks
view and Summary period go with it. When reopening would lose work — a commit
message or pull request being written, an announcement waiting for CI, a comment
on a forge issue — it asks first, naming what, and `esc` stays, keeping the save
for when workflow next opens. A directory that cannot be reopened with the new
configuration reopens as it was, saying why. `esc` leaves Settings without
saving, and under `--dry-run` `ctrl+s` says what it would have saved.

## Dry run

`workflow --dry-run` reads everything as usual and writes nothing. Every action
that would change something — a status change, a comment, a branch, staging, a
commit, a push, a pull request, an announcement, a change to a Taskwarrior task,
a generated `lefthook.yml` — says what it would have done instead, as in
`dry run: would start task 12`; the Tasks pane still reads and lists your tasks.
The top row starts with `DRY RUN` while it is on. It reads nothing from the
cache, so it starts without the cached issue list, your last commit scope and
what was announced before. What you decided — whom each code owner is on
Slack, the groups a repository tags, your favorites — it reads from the
store's `kept.db`, read-only, when one is already there, and it never makes or
changes either of the
[store's files]({{< relref "/docs/configuration#what-is-kept-between-sessions" >}}).

## Cleaning the local data

`workflow db-clean` prints where the store lives and each database file there,
with its size and what it holds, then asks before it removes anything:

```text
$ workflow db-clean
Local data in /home/ana/.local/state/workflow
  workflow.db  cache    92.0 KiB  scopes: 3, announcements: 5, cached views: 2, cached issues: 41
  kept.db      kept     24.0 KiB  owner decisions: 4, owners on Slack: 3, owners not on Slack: 1, Slack users and groups: 5, repository groups: 2, group choices: 1, chosen groups: 1, favorite directories: 2
Remove workflow.db from /home/ana/.local/state/workflow? [y/N]:
```

By default it removes only the cache, `workflow.db`, which the next session
makes again. `--all` removes `kept.db` too, after a warning: whom each code
owner is on Slack and each repository's groups go with it, and people and group
associations will be asked again. `--yes` removes without asking; `--dry-run`
only says what it would remove. A store with nothing in it says there is nothing
to remove. A database file another program holds open — on Windows, another
workflow session — fails the clean without removing anything (exit status 4);
something other than the store's own file where a database belongs, such as a
symlink, is refused (exit status 1). The web interface's Settings has the same
two cleans under **Local data**, and so has the terminal: `L` on the
Repositories pane lists the same files, and `c` removes the cache, `C`
everything, each after a last look that says what goes with it, since neither
can be undone. Under `--dry-run` it says what it would have removed.

## Existing git hooks

When a repository has hooks in `.git/hooks` and no lefthook configuration, the
Commits pane says so and `g` opens an offer to write a `lefthook.yml` that runs
them:

- `enter` turns each hook made only of plain commands into lefthook jobs, run
  in order and stopping at the first failure as `set -e` would. A hook that
  reads its arguments, branches, or sets variables is kept whole as a script
  under `.lefthook/` rather than guessed at.
- `v` keeps every hook whole as a script.
- `esc` skips it.

Both write the configuration, then run `lefthook install`, which keeps the old
hooks as `.git/hooks/*.old`. Should that install fail, the offer closes and says
why rather than offer to write the file again; run `lefthook install` once the
cause is fixed. An existing `lefthook.yml` is never overwritten. The offer only
appears when `lefthook` is installed.

## Editor

Commit bodies, pull request descriptions and announcements — and a comment,
when `ctrl+o` hands it over — are written in `$GIT_EDITOR`, else `$VISUAL`, else `$EDITOR`, else `vi` (`notepad`
on Windows) — git's own order, though git's `core.editor` setting is not read.
Everything below the scissors line (a `>8` cut mark) is help and is not kept.
An editor that has to be told to wait needs saying so: `EDITOR="code --wait"`.

Opening a hook failure at its line works for vi, Vim, Neovim, nano, Emacs,
micro, Kakoune, mg, VS Code, VSCodium, Cursor, Helix, Sublime Text and Zed.
Other editors open the file at the top.

## Mouse and ASCII

A click focuses a pane; a click on a row of the focused pane selects it, and the
wheel scrolls. Mouse capture stops your terminal's own click-and-drag selection,
so `m` turns it off for the session and `ui.mouse` turns it off for good.

`ui.ascii` draws borders and glyphs in plain ASCII, for a font that does not
have them, and so does the live line a command keeps while it waits. See
[Configuration]({{< relref "/docs/configuration" >}}).

## Limits

What follows is what workflow does not do, by design or for now. For what to do
when something it does goes wrong, see [Questions and
troubleshooting]({{< relref "/docs/faq" >}}).

- **A push cannot ask for a password.** The interface owns the terminal, so git
  is told not to prompt. Push over SSH, or over HTTPS with a credential helper.
- **Staging is by whole file.** Staging hunks is lazygit's whole project; use
  it, or `git add -p`, alongside.
- **A quit waits for your writes; a signal does not.** Quitting, or switching to
  another directory, stops every read in flight, such as a list loading or a
  pane reading the forge, but a read of the Slack directory, which others may be
  waiting on, and a renewal of your Slack user token that a read began: those
  run to their end, or until workflow exits. A quit or a switch stops none of
  the writes you sent: a commit, a push, a post, a comment, a status change, a
  settings save. Quitting the last interface then waits for those writes to
  land, saying `finishing 2 writes…`, and exits once they have; it gives up
  after `timing.request_timeout`, naming each write it gave up on, which may be
  cut short as workflow exits. A switch does not wait: its writes go on in the
  directory left, and the quit waits for them. A page you open in the browser is
  not a write: a quit neither stops its opening nor waits for it. A signal
  (SIGTERM, or the SIGINT a second `ctrl+c` sends while workflow is finishing)
  stops reads and writes alike, and workflow exits at once. Work that would
  carry on regardless, such as Settings renewing a Slack user token on macOS, is
  cut short by that exit: a signal after Slack has taken the renewal and before
  the keychain keeps the new token can lose the token, since Slack retires the
  old one as it answers; `workflow slack login` then sets it up again. An
  announcement waiting for CI is not yet a write: it is not sent if you quit
  first.
- **An announcement waits only for CI that reports.** One waiting for a pull
  request that reports no check gives up after ten minutes, counted from the
  last read that saw one; announce a repository with no CI by hand. On GitHub,
  CI is read at the branch's head here, so a fix pushed from another clone is
  seen once this branch moves to it.
- **A branch is worked on under the name it shows.** git allows characters in a
  branch's name that cannot be drawn as they are, such as one with no width or
  one that reverses the text after it. A branch, upstream or base named with
  one is refused, and the Branch pane says which.
- **Each announcement is its own message.** A pull request's later
  announcements are posted beside the first rather than in its thread;
  replying in the thread is an idea on the list,
  [FEAT-81](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-81-reply-in-the-announcements-own-thread).
