---
title: "The web interface"
weight: 17
---

# The web interface

`workflow --web` serves the same loop in a browser: the same issues, branch,
pull request and announcement the terminal interface shows, read from the same
repository and services through the same code. It is another way in, not a
second implementation, and the terminal stays the fuller of the two.

## Start it

```sh
workflow --web              # serve http://127.0.0.1:13579
workflow --web --dry-run    # the same, read-only
workflow --web --port 7001  # serve http://127.0.0.1:7001
```

Run it inside a repository, then open the address it prints on stderr. It
listens on the loopback interface alone, on port 13579 unless `--port` names
another (1 to 65535; `--port` goes only with `--web`), and refuses a write
from a page served anywhere else, so a site open in another tab cannot drive
it. `ctrl+c` in the terminal stops it.

Every build carries the web app inside it: the release binaries, `task build`
and `go install` alike.

**Read-only with `--dry-run`.** A banner under the header says so, and every
write is held back in the browser before it is sent, saying *Held back by
--dry-run: nothing was sent.* The server refuses every write as well, so
nothing changes even if a request gets past the page. Unlike the terminal, the
web does not narrate what a held-back write would have done.

## The page

The header holds the version, the theme and the stream's state, and the
Taskwarrior task you have started, when there is one. The rail down the left
holds the seven sections; the one you are in, and its heading, take the hue of
the system it belongs to. Below a medium width the rail keeps its icons
alone; each still has its name, shown on hover and read by a screen reader.

**The theme** button cycles System, Light and Dark. System follows your
operating system; the choice is remembered in this browser.

**The stream** keeps the page current. The server reads the repository and
the services every five seconds and pushes what it finds, so nothing on the
page needs refreshing by hand. The forge — the pull request, its reviews and
its CI — is asked less often: at most once every `timing.ci_interval` (twenty
seconds unless set), however many tabs are open, and at once when another
branch is checked out, its head commit moves, or the page opens a pull
request. A forge read that fails keeps the last answer for the same branch
and head commit; with no answer to keep, the page shows no pull request
until a read succeeds. The stream's state is beside the theme:

- **Connecting** — the page has not had its first update yet.
- **Live** — updates are arriving.
- **Reconnecting** — the connection dropped; the browser is finding its way
  back, and the page shows the last update until it does.
- **Out of date** — updates arrive but this page cannot read them, which
  happens when the server is a newer or older build than the page. Reload the
  page. Why is shown on hover and read by a screen reader.

**The forge's own words.** On GitLab the page says *merge request* and numbers
one `!12`; on GitHub, *pull request* and `#12` — as the terminal does.

## The sections

### Issues

The issues in a view — the default is those assigned to you and not done —
with a **View** select for the views your configuration defines, each scoped
to your issues unless its query names the assignee, a **Filter** over the
issues loaded so far, by key or summary, **Where** buttons that narrow them to
places, and **Load more** while the view holds more. Where offers the statuses
the loaded issues are in and the marks *in flight*, *task active*, *tracked*,
*task done* and *forge issue*, each with how many issues it holds; pressing one narrows the
list, and pressing it again undoes that. Statuses widen each other, as marks
do, and the two narrow together, as the terminal's `p` does. A repository
that lists its forge's issues beside Jira's (`issues.forge`) numbers them as
the forge does, `#57`, links each to its page with **Open in GitHub** or
**Open in GitLab**, and says so above the list, among its controls, when the
forge's issues could not be read. An issue with a branch is marked *in
flight*, with **Check out** beside it when that branch is not the one checked out — a local branch, or one
only the remote has, which checking out creates here. The branches counted are
those naming an issue assigned to you and not done, so a branch whose issue
was finished or reassigned no longer marks it; while the tracker cannot be
asked, its last answer stands, and a branch it has not answered for yet counts
until it does. Once Taskwarrior has answered, a row also marks how its issue's
tasks stand: *tracked*, *task active* or *task done*.

The selected issue's detail shows its type, priority, reporter and assignee, a
link to open it in Jira, its description and its comments, and its **work
story**: the branch, the changes, the pull request and the announcement, each
step opening the section it belongs to. A step reads done, not yet, or failed
by the rules of the terminal's top row and `workflow status`: the changes are
done once there is a commit, and the pull request is done once its CI passes
or it merges, and failed on a CI failure or changes asked for. The story never
marks the announcement done, because the web keeps no record of one. From the
story, **Start work** creates and checks out a branch named for the issue, and
**Check out this branch** switches to one it already has. Once Taskwarrior has
answered, a **Tasks** card sits between the story and the description;
[Tasks](#tasks) says what it holds. Below a large width the list sits over the
detail rather than beside it.

The comments are drawn from Jira's wiki markup: bold, italic, struck and code
text, links, headings, quotes, lists and code blocks. Anything else, such as a
table, a panel or a color, shows as the text it is, and an image is offered as
a link rather than loaded. Under a Jira issue's thread, a box takes a comment.
With `jira.markdown_comments` on, the comment is Markdown: **Write** and
**Preview** are tabs, Preview shows it as Jira will, and the buttons beside
them write bold, italic, code, a link or a list around what is selected. With
it off, the comment is posted as typed, and Jira reads it as wiki markup.
Turning it on or off in Settings applies to the next comment, with no
restart.
**Comment** posts it and reads the issue again, so the thread shows it as Jira
keeps it. A forge issue takes no comment here.

### Branch

The checked-out branch, its base, its upstream and how far it is ahead or
behind, and the commits on it. **Push branch** publishes it, after a
confirmation. **Link an issue** ties a branch begun outside workflow, whose
name names no issue, to one: a Jira key or a forge number such as `#42`. When
its pull request's description does not name the issue yet, it is shown with
the issue's line added first, with **Link and update** or **Link only**. A
linked branch says so, with **Unlink**; the link is kept in the repository's
git configuration, as the terminal's `i` keeps it.

Under **Working tree**, each changed file has its own **Stage** or
**Unstage**, and **Stage all** stages the rest. The commit form builds a
Conventional Commit from its type, scope, subject, body and a breaking-change
box, with the scope the terminal would suggest already filled in; **Commit
staged changes** commits, adds the trailer naming the branch's issue (`Refs:`
unless `commit.refs_trailer` relabels it), and runs the repository's own
hooks. A hook that refuses the commit says why.

### Review

The branch's pull request — its number, title and state: Draft or Ready for
review while it is open, Merged once it has merged. While it is open, the
section also shows its mergeability, approvals and requested changes, the
issue it is for, linked to its page, and its CI checks: a failed one names the
stage it ran in and why it failed, and **Show log** reads the end of its log
when the forge keeps one. With no pull request for the branch, **Open a pull request**
composes one as `workflow pr` would and shows it as a form: the title, the
base, the reviewers, assignees and labels, the description, and whether it is
a draft. The reviewers start as the code owners of the paths the branch
changes, as the base's CODEOWNERS names them, leaving you out; a team is
written `org/team` and is requested as a team. **Open pull request** pushes the branch first when it is not
published, then opens it. If the forge would not add every reviewer, assignee
or label, a warning says so.

Right after the page opens one, the section offers what the terminal and
`workflow pr` offer next: **Link it on** the branch's issue, and **Move** the
issue to the review status your configuration names, when that move needs no
fields filled in. A pull request opened in the terminal or with `workflow pr`
gets neither offer here.

### The messaging service

Named for the service your configuration uses — Slack, Teams, Discord or
Webhook. It shows where announcements go and who they are from. **Announce to**
the service composes the announcement of the branch's pull request and shows
it, with the channel to send it to where there is a choice; **Announce now**
sends it. Nothing is sent before that second press, and what is sent is the
text shown: when the announcement changed in between — CI turned red, the pull
request merged — nothing is sent, the page says so, and **Announce to** shows
the new one.

With a Slack user token, a ready-for-review announcement's preview also says
whom it tags. **Tag code owners** lists the code owners of the branch's
changes: one linked to Slack shows their Slack name, one decided not on Slack
says so, and one not asked yet has a choice of the members of the channel the
preview posts to, following the channel as you change it (a team's, of the
workspace's user groups), and a **Not on Slack** button.
The choice is saved as you make it, for this and every later announcement on
the same forge host, and the page says it was saved for next time;
**Announce now** waits while a link is being saved, and a group checked
meanwhile stays checked. **Tag groups**
checks the user groups the repository offers; a group a team owning the
changed paths is linked to starts checked and says it "owns changed paths",
and the rest start as you left them last time. **Tags:** above **Announce
now** names everyone the post will tag; the tags go on a line after the
text. The post tags only the people the preview showed: when a link changed
in Settings or a terminal since, nothing is posted and the page asks you to
preview again. A Slack token without a scope linking needs (`users:read`,
`channels:read`, `groups:read` or `usergroups:read`) for the channel picked
is named in a note, and the announcement still posts, tagging whom it can.
Links are kept per Slack workspace: when Slack will not say which workspace
the token is for, a note says so, nobody is offered, and the announcement
posts untagged; Settings' people and groups say the same. Under `--dry-run`
no owner is offered to link. Teams, Discord and a webhook
tag no one, and so does a Slack webhook saved in Settings while the server
runs: tagging follows the configuration in effect.

### Reviews

The pull requests on your forge that wait on your review, the longest-waiting
first: where each is, who asks, how long it has waited, whether it is a draft,
and how its CI stands, each with a link to open it and **Copy URL**. **Sort**
lists them **Oldest first**, **Newest first** or **By repository**, which heads
each repository's requests with its name, the longest-waiting first within it —
the orders the terminal's `s` cycles through. **Filter** narrows them, as the
terminal's `f` does: a button for each repository, CI state, draft or ready, and
author the queue holds, each with how many requests hold it. Values in one of
those widen the list, and the four narrow it together. The order and the
filter chosen stay while you visit other sections. The section asks the
forge when you open it, unless it asked within the last 30 seconds, and
**Refresh** asks again.

### Tasks

Your pending Taskwarrior tasks, most urgent first — what the terminal's Tasks
pane lists, in Taskwarrior's violet: those for an issue the Issues list holds
first, then the rest, under **For my issues** and **Other tasks** when there
are both. A waiting task is counted below rather than listed, and an active
context says it narrows the list. Each row marks whether the task is started,
then its id, what it is, and quietly its issue, when it is due and its
urgency. **Sort** orders each group by urgency, state, ID, tag, issue or
priority, as the terminal's `O` does, and sorted by tag or priority each row
shows its tags or priority; the order stays while you visit other sections.
The section reads Taskwarrior when you open it and when **Refresh**
asks, rather than from the stream. It needs Taskwarrior 3.5.0 or newer, found
as [Configuration]({{< relref "/docs/configuration#taskwarrior" >}})
describes; without one it says why — and, where the `task` on `PATH` is
another program, go-task most likely, that Settings can name Taskwarrior's
with `taskwarrior.program`, which applies once workflow restarts.

The line at the top adds a task from what you would type after `task add`, in
Taskwarrior's own grammar — `project:web`, `due:friday` or `+review` among the
words — and says which task it added. The line is not read as a shell reads
it: each space-separated word goes to Taskwarrior as an argument of its own,
and quotes are passed on as typed, as
[Using workflow]({{< relref "/docs/usage#track-it-in-taskwarrior" >}})
explains. The selected task shows its state, project, priority, tags, due
date, urgency, id and issue, a link to the issue's page and, when the Issues
list holds the issue, **Open in Issues**. For a task that names an issue the
page is the tracker's, as the terminal's `o` opens it; with the forge's
issues as the tracker, or for a task that names none, it is the task's
`jiraurl`, and only an http or https address.
Then come **Start** or **Stop**,
**Done**, its annotations, and a line each to **Annotate** it and **Modify**
it in the same grammar. **Undo** reverts Taskwarrior's last change, whatever
made it, and **Sync**, shown when the taskrc names a sync backend, syncs. Each
write says what it did, and the list redraws from Taskwarrior's answer at
once. A write Taskwarrior refuses says why below its button, in Taskwarrior's
own words, with your home and data directories put in fixed words and any
line naming a server left out; a failed sync answers in fixed words only, and
`task sync` in a terminal shows why. A write that changed nothing — **Start**
on a task already started, say, or **Undo** with nothing to undo — shows the
sentence its `409` carries.

Once Taskwarrior has answered, the rest of the page shows your tasks too: the
header carries the task you have started and how long it has run, and opens
this section; each Issues row marks how its issue's tasks stand; and an
issue's detail has a **Tasks** card listing each of its tasks, with **Start**
or **Stop** and **Done** on each still to do. While none is, the card offers
**Track in Taskwarrior**, which adds the task the terminal's `T` would,
annotated with the issue's page, and says which task now tracks it. Until
Taskwarrior has answered, and where it could not, none of them is drawn,
rather than a claim that no task tracks the issue.

### Settings

The configuration file in effect, in eight parts — Jira, messaging, the forge,
commits, branches, pull requests, the store and Taskwarrior — and
**Save changes** writes it back. A credential is shown masked and kept as it is
unless you type a new one. A change to the forge part — its token, host,
kind or CLI — applies at once: the next call to the forge uses it, and the
page names the forge it points at, with no restart. A change to the
messaging part applies to the next announcement. Slack's user token is
three fields — Client ID, Client secret and Refresh token; on macOS a save
refreshes the token with them and keeps the secrets in the keychain rather
than the file, and where the file already keeps them it keeps what you type
(see [Configuration]({{< relref "/docs/configuration" >}})). The access token
and its expiry have no field: workflow writes them. A change to the
Taskwarrior part applies when workflow restarts, as the part says: workflow finds Taskwarrior as it starts,
and until the restart the Tasks section says to restart rather than read
Taskwarrior. What the form has no field for yet is kept unchanged
when you save: `version`, all of `ui` and `timing`, `jira.token_command`,
`jira.token_env`, `jira.headers`, `jira.views`, `messaging.channels` and
`branch.prefixes`. Settings
reads the file each time it opens, and a save checks that the file has not
changed since: when it has (edited on disk, rewritten by
`workflow config init --force`, or saved from another tab), nothing is written,
and **Reload** reads it again in place of your edits so you can make the change
again. The check and the write are not one step, so a change that lands in the
moment between them is still written over. When the file on disk is not valid,
Settings says so in place of the form, and `workflow doctor` says what is wrong
with it.

Below the form, **People and groups** keeps what the announcement preview
asks: a table of every code owner decided on this repository's forge host,
then the branch's owners not decided yet, each with a choice of whom they are
on Slack — not decided yet, not on Slack, or a member of the configured
channel (a team, a user group) — saved as you change it. **Forget…** asks
first, in place, and a forgotten owner is asked about again at the next
ready-for-review announcement. **Groups for** the repository is a checkbox per
user group in the workspace, kept with **Save groups**; a ready-for-review
announcement offers those groups to tag. Each saves on its own, apart from the
configuration, into `kept.db`, and the label kept is the one Slack's directory
gives. A group already saved keeps its label, so one Slack no longer lists,
or a token that cannot read the groups, can still be kept or unchecked, and
clearing every group needs no Slack at all. A token without a scope the
choices need is named in a note. Under
`--dry-run` the area says its changes are held back and its controls are off.

Below that, **Local data** shows where workflow keeps what it learns
between sessions and a row for each database file: the cache (`workflow.db`)
and the kept associations (`kept.db`), each with its size and what it holds.
It is read each time Settings opens and saved apart from the configuration.
**Clean cache…** and **Clean everything…** each ask first, in place, with
**Cancel** and **Clean**; cleaning everything says that people and group
associations will be asked again, and People and groups above is read again
to show them gone. What the clean did is said below the buttons, and the
listing is read again. A file another program holds open fails the clean, and
the area says why and reads the listing again, since a file that could not be
deleted after the rest were set aside may leave others gone. A clean waits
for a change to People and groups under way, and the other way round. Under
`--dry-run` the buttons are replaced by a sentence saying cleaning is held
back. It is the web's `workflow db-clean`
(see [Using workflow]({{< relref "/docs/usage#cleaning-the-local-data" >}})).

## What stays in the terminal

The web covers the loop's main line. These stay with the terminal interface, or
the command line, by decision rather than by oversight; most are written down
as ideas in
[FEATURES.md](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md)
or [UX.md](https://github.com/jacob-delgado/workflow/blob/main/UX.md):

- **Re-running failed CI, merging, finishing a merged branch and editing an
  open pull request** — the terminal's `R`, `M`, `F` and `e`
  ([FEAT-79](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-79-review-actions-on-the-web)).
- **Moving an issue to any status but the review status, assigning and
  logging work** — the terminal's `t`, `a` and `w`
  ([FEAT-80](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-80-issue-writes-on-the-web)).
- **Editing an announcement before it is sent, and announcing once CI
  passes** — the terminal's `e` and `w` in the announcement preview.
- **Knowing an announcement was already made** — the web neither records an
  announcement nor reads one, so it offers one again that the terminal or
  `workflow announce` already made
  ([FEAT-84](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-84-the-web-remembers-what-was-announced)).
- **Rebasing onto the base, amending, fixing up, reading a file's diff,
  choosing among pull request templates, creating a branch in a worktree,
  running the pre-commit hook on its own and generating a `lefthook.yml`** —
  the terminal alone.
- **Offering a task change at the loop's moments** — starting the issue's
  task when its branch is made or checked out, noting the pull request on it,
  completing it on a merge or a move to done; on the web those are the
  **Tasks** card's and the Tasks section's buttons, pressed when you choose.
- **Writing a first configuration file** — `workflow config init`; Settings
  edits a file that already exists.

## When a request fails

A failed request says why beside the button that made it, in words meant for
you, and the button is there to try again. Behind that, the API answers a
failed request with an RFC 9457 problem details object whose `code` a script
can rely on; only the loopback, same-origin and `--dry-run` guards, which
refuse a request before it reaches the API, answer in plain text. [Web API
errors]({{< relref "/docs/errors" >}}) lists the codes.

## Scripting the API

Five reads answer what a section shows, for a script on the same machine; the
page itself takes the first four from its stream, and reads the fifth as its
Tasks section opens.
[`api/openapi.yaml`](https://github.com/jacob-delgado/workflow/blob/main/api/openapi.yaml)
describes each answer's fields, and every other request the API serves.

| Request | Answers with |
| --- | --- |
| `GET /api/branch` | The checked-out branch: its base, its upstream, how far it is ahead or behind, and its commits |
| `GET /api/changes` | The working tree's changes |
| `GET /api/review` | The branch's pull request and its CI |
| `GET /api/messaging` | The messaging service, and where and as whom an announcement would post |
| `GET /api/tasks` | Your pending Taskwarrior tasks, most urgent first, waiting ones included, with the active context and whether a sync backend is set |

```sh
curl -s http://127.0.0.1:13579/api/review
```

They are reads, so they answer under `--dry-run` too. When git or the forge
fails, the branch, changes and review reads answer a problem where the page
shows an empty panel, so a script can tell a failure from nothing to show. The
tasks read answers `available: false` and why, never a problem, when there is
no Taskwarrior to ask; a Taskwarrior that is there but fails the read answers
a problem.

The Tasks section's writes are `POST`s beside that read: `/api/tasks` to add a
task, `/api/tasks/track` to track an issue, `/api/tasks/undo`,
`/api/tasks/sync`, and `start`, `stop`, `done`, `annotations` and `modify`
under `/api/tasks/{uuid}/`. Each answers the task list as it stands after the
write, and a task Taskwarrior changed nothing on answers `409`.

People and groups read `GET /api/people` and `GET /api/repo-groups`, and
write with `PUT` to each and `DELETE /api/people?owner=`; the choices come
from `GET /api/slack/members` and `GET /api/slack/groups`, which answer a
missing scope as a `200` naming it. `GET /api/announcement?channel=` checks
an owner not yet linked against that channel's members. `POST /api/announce`
takes `mentions`: `users`, the people the preview showed tagged, and
`groups`, the user groups checked. The linked owners it tags are read from
what is kept, never from the request, and a post whose linked owners are not
`users` is a `409`.
