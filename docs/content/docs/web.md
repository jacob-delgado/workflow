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
it. `ctrl+c` in the terminal stops it. With no `.workflow.json` to read, it
says so on stderr and serves anyway: **Settings** sets a first one up (see
[Setting up](#setting-up)), as `workflow config init` does at a prompt.

Every build carries the web app inside it: the release binaries, `task build`
and `go install` alike.

**Read-only with `--dry-run`.** A banner under the header says so, and every
write is held back in the browser before it is sent, saying *Held back by
--dry-run: nothing was sent.* The server refuses every write as well, so
nothing changes even if a request gets past the page. Unlike the terminal, the
web does not narrate what a held-back write would have done.

## The page

The header holds the version, where the server works, the theme and the
stream's state, and the Taskwarrior task you have started, when there is one.
The rail down the left holds the nine sections; the one you are in, and its heading, take the hue of
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

**The keyboard.** Beyond Tab, the page answers the terminal's own keys, read
from its bindings with `ui.keys` applied, so a key you moved in the file moves
here too. `?` opens a sheet of the actions the page has a control for, under
the terminal's groups and in its words, with the key for each; `Escape`
closes it. `Ctrl+K`, or `⌘K` on a Mac, opens a palette of what the section
you are in can do, and of the other sections: type part of an action's
name, choose with the arrow keys, and `Enter` runs it through its own
button, so a push or a merge still asks first. Single keys — `c` comments,
`a` stages everything on Branch, `2` opens Branch — are off until you turn
on **Single-key shortcuts** in Settings (`ui.web_shortcuts`), since a key
that acts on its own surprises a screen reader or speech user; `?` and the
palette work either way. Even with them on, a key types into the field that
has the focus, a search box or the comment box among them, and acts only
outside one. Each control a key reaches says so to assistive technology
(`aria-keyshortcuts`). See
[Keys on the web]({{< relref "/docs/configuration#keys-on-the-web" >}}).

## The sections

### Issues

The issues in a view — the default is those assigned to you and not done —
with a **View** select for the views your configuration defines, each scoped
to your issues unless its query names the assignee, a **Search** over the
issues loaded so far, by key or summary, **Filter** buttons that narrow them to
places, and **Load more** while the view holds more. Filter offers the statuses
the loaded issues are in and the marks *in flight*, *task active*, *tracked*,
*task done* and *forge issue*, each with how many issues it holds; pressing one narrows the
list, and pressing it again undoes that. Statuses widen each other, as marks
do, and the two narrow together, as the terminal's `p` does. A repository
that lists its forge's issues beside Jira's (`issues.forge`) numbers them as
the forge does, `#57`, links each to its page with **Open in GitHub** or
**Open in GitLab**, and says so above the list, among its controls, when the
forge's issues could not be read. An issue with a branch is marked *in
flight*, with **Switch branch** beside it when that branch is not the one checked out — a local branch, or one
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
or it merges, and failed on a CI failure or changes asked for. The
announcement is done once the pull request was announced at the moment it is
at now — from the browser, the terminal or `workflow announce`, as the store
remembers. From the
story, **Start work** fetches origin, so the branch starts from what origin
holds now, then creates and checks out a branch named for the issue, and
**Switch branch** switches to one it already has. **Start work in a new
worktree** creates the branch in a new git worktree beside the repository
instead, leaving the checkout here as it is, then says where and offers
**Switch to it**. A branch another worktree has checked out, which git will not
check out twice, offers **Switch to its worktree** in place of the checkout.
Either asks "Switch to DIR?" first, as every switch in Repositories does, and
switches only on **Switch** there; one held by a worktree whose directory is gone says that `git worktree prune`
frees it. When the fetch fails — the network, origin's address, your
credential — nothing is made, the reason is said, and **Branch from what you
have** starts the branch from the base as it was last fetched, as the
terminal's branch creator offers. Once Taskwarrior has
answered, a **Tasks** card sits between the story and the description;
[Tasks](#tasks) says what it holds. Below a large width the list sits over the
detail rather than beside it.

Under the people, three buttons change the issue, as the terminal's `t`, `a`
and `w` do. **Change status** lists every status change the tracker offers
from where the issue stands, each by where it leads, named when the change
has a name of its own, and saying which fields it needs. Choosing one shows
its form: a choice of one value, a box per value for a list such as fix
versions, a date, or text or a username. **Change to** the status sends the
change with what was filled, and nothing is sent before that press; a field
left empty or given something it does not take is named beside the form, and
nothing changes. A change that needs a field only Jira's own screen can fill,
such as a cascading select, says so and cannot be sent from here. A forge
issue offers one change, Close. **Assign** sets the assignee to the username
typed, as the issue's tracker knows it. **Log work**, on a Jira issue alone,
logs a duration in Jira's words — `2h`, `30m`, `1d 4h` — with an optional
note. Each form says what it did once the tracker has it — *Changed PROJ-1 to
Resolved.*, *Assigned PROJ-1 to ana.*, *Logged 2h on PROJ-1.* — and reads the
issue again; a refusal stays beside the form, which keeps what was entered.
**Cancel** sends nothing.

The comments are drawn from Jira's wiki markup: bold, italic, struck and code
text, links, headings, quotes, lists and code blocks. Anything else, such as a
table, a panel or a color, shows as the text it is, and an image is offered as
a link rather than loaded. Under the thread, a box takes a comment; on a Jira
issue it says it is reading how comments are written until the configuration
answers, then draws in that shape. On a Jira
issue the comment is Markdown, as `jira.markdown_comments` is on by default:
**Write** and **Preview** are tabs, Preview shows it as Jira will, and the
buttons beside them write bold, italic, code, a link or a list around what is
selected. With it off, the comment is posted as typed, and Jira reads it as
wiki markup. Turning it on or off in Settings applies to the next comment,
with no restart. **Comment** posts it and reads the issue again, so the thread
shows it as Jira keeps it.

A forge issue, numbered such as `#57`, takes a comment too. Its thread and its
comment are always Markdown, which the forge renders itself, so the box always
has Write and Preview, and the comment is posted as written. On GitLab the box
also says that a line starting with `/`, such as `/close`, runs as a quick
action rather than being posted. A comment over 65,536 characters, or with a
line over 1,000, is shown as plain text in the thread and in Preview, and so
are a thread's older comments once the newer ones add up to that many.

### Branch

The checked-out branch, its base, its upstream and how far it is ahead or
behind, and the commits on it. **Push branch** publishes it, after a
confirmation naming the branch and the remote it goes to; it is not offered
on the base branch itself. **Link an issue** ties a branch begun outside workflow, whose
name names no issue, to one: a Jira key or a forge number such as `#42`. When
its pull request's description does not name the issue yet, it is shown with
the issue's line added first, with **Link and update** or **Link only**. A
linked branch says so, with **Unlink**; the link is kept in the repository's
git configuration, as the terminal's `i` keeps it. **Rebase onto** the base
replays the branch onto it, as the terminal's `u` does, after a last look
that says its commits are rewritten.

Under **Working tree**, each changed file has its own **Stage** or
**Unstage**, and **Stage all** stages the rest; **Unstage all** beside it
takes everything out of the index at once, as the terminal's `U` does,
leaving the work tree as it is. **Discard…** on a file asks first, as the
terminal's `x` does, since it cannot be undone: **Discard** puts the file
back as the last commit has it, staged and unstaged changes alike, and
deletes a file the last commit does not have. The commit form builds a
Conventional Commit from its type, scope, subject, body and a breaking-change
box, with the scope the terminal would suggest already filled in; **Commit
staged changes** commits, adds the trailer naming the branch's issue (`Refs:`
unless `commit.refs_trailer` relabels it), and runs the repository's own
hooks. A hook that refuses the commit says why. **Show diff** under a changed
file reads its diff against HEAD, staged and unstaged together, and **Hide
diff** puts it away.

Under the commits, **Run pre-commit** runs the pre-commit hook on what is
staged without committing, as the terminal's `h` does, at once: it changes
nothing that leaves the machine. While something is staged and a commit is
not yet pushed, **Amend last commit** folds the staged changes into the last
one, keeping its message, and **Fix up a commit** records a `fixup!` of the
one chosen among those not yet pushed — the terminal's `A` and `f` — each
after a last look at the commit it rewrites. A rebase, a run of pre-commit, an
amend and a fixup each show their output as it is written, with **Stop**,
which stops the program and all it started, then how it ended, with
**Close**. One goes at a time, and it holds the index, so a stage or a commit
asked for meanwhile waits for it. It goes on to its end if the page is
closed, and a page opened meanwhile shows it running with its last lines. A
conflict stops a rebase midway, for a terminal to finish.

When the repository's `.git/hooks` holds hooks lefthook does not manage, and
it configures no lefthook, **Set up lefthook** shows the hooks and the
`lefthook.yml` that runs them, as the terminal's `g` offers it; **Write
lefthook.yml** writes it and installs lefthook, and **Write every hook as a
script** keeps each hook whole under `.lefthook` instead. lefthook keeps the
old hooks as `.git/hooks/*.old`.

### Review

The branch's pull request — its number, title and state: Draft or Ready for
review while it is open, Merged or Closed once it is not — and the issue it is
for, linked to its page. While it is open, the section also shows its
mergeability, approvals and requested changes, and its CI checks, headed by
how many are done when the forge counts them and by how CI stands when it does
not, as for a GitLab pipeline, or "No checks reported." when there are none: a
failed one names the stage it ran in and why it failed, and **Show log** reads the end of
any check's log the forge keeps one for, passed or failed.

Under the state, the terminal's Review pane's writes, each offered only when
it can go and each sent from a form that is its last look: **Edit pull
request** reads its title and description afresh and **Save**s them, as `e`
does; **Merge**, once it is ready for review, free of conflicts, approved with
no changes asked for and green, reads the methods the repository permits and
merges by the one chosen, as `M` does; **Re-run failed checks**, while its CI
has failed, restarts the failed jobs, as `R` does, and says when a failure had
no job to restart; and once it has merged, **Finish the branch** shows the
three git commands it runs — switch to the base, pull, delete the branch — as
`F` does, when the branch holds no commit origin lacks. The server reads the
pull request and its CI again before each, so nothing goes on what the page
showed a moment ago. A token without the scope a write needs is told in the
forge's own words, naming the scope.

With no pull request for the branch, **Open a pull request**
composes one as `workflow pr` would and shows it as a form: the title, the
base, the reviewers, assignees and labels, the description, and whether it is
a draft. When the repository has several pull request templates, **Template**
chooses the one the description starts from, as the terminal's `ctrl+t`
does, until the description is edited. The reviewers start as the code owners of the paths the branch
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
the new one. Once a pull request was announced at the moment it is at now —
ready for review, its CI red, merged — from here, the terminal or `workflow
announce`, the section says so in place of the offer, as the terminal offers
no announcement it has made; each announcement made here is remembered for
the terminal and `workflow announce` too.

**Edit**, in the preview, turns the message into a box to change it in, as
the terminal's `e` opens it in your editor; **Announce now** then sends the
text as edited. An edit still goes only while the announcement it began from
is the one composed now: when CI turned red or the pull request merged since,
nothing is sent, as for an unedited one. An edit emptied of text is refused.

While the pull request's CI is still running, a ready-for-review
announcement's preview also offers **Announce when CI passes**, the terminal's
`w`: the server holds the announcement — edited or not, with its tags — and
the section says *Waiting for CI on #42 to pass, then announcing to #dev.*,
with **Stop waiting** to drop it unposted. It is posted once the forge reports
the CI passed, and the section then says it was announced; when the CI fails,
or the branch's pull request is another one or merged first, it is dropped
and the section says why in red. **Announce now** drops a held announcement,
so the channel never reads it twice. The server, not the page, holds it: it
reads the CI for it every `timing.ci_interval` (twenty seconds unless set)
whether a page is open or not, and settles it on each frame a page shows. It
is kept in memory alone, so it is lost, unposted, if `workflow --web` stops
or switches directory before the CI passes — as one held in the terminal is
lost when the terminal closes.

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

Your pending Taskwarrior tasks, most urgent first unless you sort them — what the terminal's Tasks
pane lists, in Taskwarrior's violet: those for an issue the Issues list holds
first, then the rest, under **For my issues** and **Other tasks** when there
are both. A waiting task is counted below rather than listed, and an active
context says it narrows the list. Each row marks whether the task is started,
then its id, what it is, and quietly its issue, when it is due and its
urgency. **Sort** orders each group by urgency, state, ID, tag, issue or
priority, as the terminal's `O` does, and sorted by tag or priority each row
shows its tags or priority. **Search** keeps the tasks whose description,
project, `+tag`, issue key or `#id` holds what you type, and the **Filter**
chips narrow the list by state, priority, project, tag and whether a task has
an issue, each with its count; pressing **waiting** lists the waiting tasks.
The order and the chips stay while you visit other sections.
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
**Mark done…**, its annotations, and a line each to **Annotate** it and **Modify**
it in the same grammar. **Undo…** reverts Taskwarrior's last change, whatever
made it, and **Sync…**, shown when the taskrc names a sync backend, syncs.
Those three ask first and send nothing until confirmed — marking done runs the
task's hooks, Taskwarrior has no redo, and a sync sends your tasks off the
machine — while **Start** and **Stop** act at once. Each write says what it did, and the list redraws from Taskwarrior's answer at
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
or **Stop** and **Mark done…** on each still to do. While none is, the card offers
**Track in Taskwarrior**, which adds the task the terminal's `T` would,
annotated with the issue's page, and says which task now tracks it. Until
Taskwarrior has answered, and where it could not, none of them is drawn,
rather than a claim that no task tracks the issue.

### Summary

What you did, read back each time from where the work left a trace: the
commits you wrote in the repository the server runs in and in every favorite
that is one — each named, `acme/web@abc1234`, when there is more than one —
the Taskwarrior tasks
you added, annotated and completed and the ones you started that are still
going, what you did to Jira issues, and
the pull or merge requests you opened, had merged and reviewed on the forge,
in any repository. It opens on the previous working day — yesterday, or on a
Monday, Friday and the weekend after it — in the time zone the server runs in.

The day reads as a timeline: each hour down the left, and what was done in it
beside it, the verb in the hue of the system it was done in — git's, Taskwarrior's,
Jira's or the forge's — then what it was done to, linked where it has a page,
and its title. A period of several days or months is headed by month and by
day. A source that could not be read says why above it — naming the
repository, when one of several could not be read — in words that never
name a host, and one that had more than it gave says so. **Copy as Markdown**
puts the summary on the clipboard. **Post…**, beside it once messaging is set
up, opens a preview of the same Markdown and where it goes, as the Messaging
section previews an announcement: **Edit** changes the text, the channel is
chosen when Slack offers more than one — a webhook posts to the channel it is
bound to, which the preview says — and nothing is posted until **Post**; the
service shows the headings and the list its own way. Nothing about the post is
kept.

**Earlier** and **Later** move a period back or on — a whole month to the
month, a whole year to the year, any other period by its own length — and
**Today** shows today. The calendar below them — beside them on a wide window —
shows a month, chosen by year and month, in which the arrow keys move a day or
a week, Page Up and Page Down a month, and Home and End to the week's ends;
Enter or a click shows that day, and with Shift, the days from the first one
shown to it. **Whole month** and **Whole year** show the month or the year the
calendar shows. A period runs up to a year and a day.

### Repositories

Where the server works, relative to what it changes: the repository it is in,
then the path within it, muted, as `~/src/api/cmd`; origin's host and path;
and the configuration files that apply there, the repository's over your
home's. The header carries the same place, briefly, as `api/cmd`, and opens
this section.

**Worktrees** are the repository's working trees, as git lists them, each
with the branch it has checked out — or the commit, when detached — and
whether git keeps it locked, or finds its directory gone; **Switch** offers to
switch to one, other than where the server works and one gone. With none to
list, outside a repository, the list is left out.

**Favorites** are the directories you keep, each with what is there now — a
repository and its origin, a directory in no repository, or one not there any
more. **Add to favorites** keeps where the server works, **Remove** forgets
one, and **Switch** offers to switch to it. **Open another directory** browses
for one: type a path — from where the server works, or from your home after
`~` — and **Show** lists the directories in it; open one by its name, **Up**
goes to the one above, and **Switch here** offers to switch to the one shown.

A switch is asked once more, then made: the server wires the directory as it
wired the first, and every section is read again for it. A directory whose
configuration did not load, or binds keys workflow refuses, is refused rather
than served on the defaults. The section you are
in, the views' orders and narrowing and the Summary's period stay; the issue
shown stays when it is a Jira issue, which is the same wherever you work. A
page that has not yet noticed a switch — another tab, say — has its writes
refused until it reads again, and a switch is refused while a write is being
made. Under `--dry-run` the switch and the favorites are held back with every
other write, and browsing still works.

### Settings

The configuration file in effect, in nine parts — Jira, messaging, the forge,
commits, branches, pull requests, the store, Taskwarrior and the keyboard — and
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
when you save: `version`, all of `ui` but `ui.web_shortcuts`, all of `timing`, `jira.token_command`,
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

#### Setting up

With no `.workflow.json` where the server works, Settings asks what
`workflow config init` asks instead of showing a form of defaults. First,
where the file goes: the repository, where it applies across it, or your home
directory, where it applies everywhere. Then Jira's address and your personal
access token, typed into a field that shows nothing of it; where the OS
keychain is wired (macOS), **Keep the token in your keychain, out of the file**
is checked, and unchecked the file keeps it, readable only by you. Last, a
Slack incoming webhook, saved unchecked; blank posts with your Slack user token
after `workflow slack login`. A blank address or webhook leaves that part out.

**Write ~/src/api/.workflow.json** — named for the file chosen — checks the
token with Jira first, saying *Checking with Jira…*. A check that does not
pass says why, beside the button, and writes nothing; **Write it anyway** keeps
the address and token unchecked. Once written, the server works with the file
as it does after a switch: the event stream reconnects, every section is read
again, and Settings shows the file with what was written said above it —
whom Jira knows the token as, that the keychain keeps it, and, for a file in a
repository git does not ignore, to add it to `.gitignore`. A file already
there is never written over. The token and the webhook go only to the server
on the loopback address, which takes them from no other page, and no answer
carries them back. Under `--dry-run` writing is held back, and the form says so.

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
**Remove cache…** and **Remove everything…** each ask first, in place, with
**Cancel** and **Remove**; removing everything says that people and group
associations will be asked again, and People and groups above is read again
to show them gone. What the removal did is said below the buttons, and the
listing is read again. A file another program holds open fails the removal, and
the area says why and reads the listing again, since a file that could not be
deleted after the rest were set aside may leave others gone. A removal waits
for a change to People and groups under way, and the other way round. Under
`--dry-run` the buttons are replaced by a sentence saying removing is held
back. It is the web's `workflow db-clean`
(see [Using workflow]({{< relref "/docs/usage#cleaning-the-local-data" >}})).

## What stays in the terminal

The web covers the loop's main line. These stay with the terminal interface, or
the command line, by decision rather than by oversight; most are written down
as ideas in
[FEATURES.md](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md)
or [UX.md](https://github.com/jacob-delgado/workflow/blob/main/UX.md):

- **Offering a task change at the loop's moments** — starting the issue's
  task when its branch is made or checked out, noting the pull request on it,
  completing it on a merge or a move to done; on the web those are the
  **Tasks** card's and the Tasks section's buttons, pressed when you choose.

## When a request fails

A failed request says why beside the button that made it, in words meant for
you, and the button is there to try again; a read that fails says why above
**Try again**. While a section reads, a line beginning *Reading* says what.
Behind that, the API answers a
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
| `GET /api/activity` | What you did from one day to another, or on the previous working day, nested by year, month, day and hour, with the Markdown to copy |
| `GET /api/repositories` | Where the server works — the directory, its repository and the path within, origin and the configuration files — and your favorite directories, each with what is there now |
| `PUT /api/repositories/here` | Switch the directory the server works in; refused while a write is in flight |
| `PUT` / `DELETE /api/repositories/favorites` | Keep a directory as a favorite, or forget it |
| `GET /api/directories` | The directories in one, to browse for one to switch to |
| `GET /api/tasks` | Your pending Taskwarrior tasks, most urgent first, waiting ones included, with the active context and whether a sync backend is set |

```sh
curl -s http://127.0.0.1:13579/api/review
```

`GET /api/repositories`'s answer needs no server: `workflow repositories
--json` prints the same object from the directory it runs in, and `workflow
summary --json` prints what `GET /api/activity` answers for the same period.

`POST /api/activity/post` posts the Summary: `from` and `to`, the period it
is of, `text`, its Markdown as `GET /api/activity` wrote it or as it was
edited, and `channel`, or none for the configured one or a webhook's own. The
server renders the Markdown for the service as it posts it and keeps nothing
of the post; it answers where the text went, in `destination`.

```sh
curl -s http://127.0.0.1:13579/api/activity | jq '{from, to, text}' |
  curl -s -H 'Content-Type: application/json' -d @- http://127.0.0.1:13579/api/activity/post
```

They are reads, so they answer under `--dry-run` too. When git or the forge
fails, the branch, changes and review reads answer a problem, so a script can
tell a failure from nothing to show; the event stream carries the same
problem beside the panel it emptied, in the snapshot's `problems`, and a CI
read that failed in the review's `ci_error`. The
tasks read answers `available: false` and why, never a problem, when there is
no Taskwarrior to ask; a Taskwarrior that is there but fails the read answers
a problem.

The Tasks section's writes are `POST`s beside that read: `/api/tasks` to add a
task, `/api/tasks/track` to track an issue, `/api/tasks/undo`,
`/api/tasks/sync`, and `start`, `stop`, `done`, `annotations` and `modify`
under `/api/tasks/{uuid}/`. Each answers the task list as it stands after the
write, and a task Taskwarrior changed nothing on answers `409`.

`POST /api/runs` with `{"kind": "pre_commit"}` — or `rebase`, `amend`, or
`fixup` with the `commit` to fix up — answers newline-delimited JSON as the
run goes: the run as it starts, a `line` for each line its program writes,
and the run as it ended, with its `state` and `outcome`. `DELETE
/api/runs/current` stops it.

```sh
curl -sN -H 'Content-Type: application/json' -d '{"kind":"pre_commit"}' \
  http://127.0.0.1:13579/api/runs
```

People and groups read `GET /api/people` and `GET /api/repo-groups`, and
write with `PUT` to each and `DELETE /api/people?owner=`; the choices come
from `GET /api/slack/members` and `GET /api/slack/groups`, which answer a
missing scope as a `200` naming it. `GET /api/announcement?channel=` checks
an owner not yet linked against that channel's members. `POST /api/announce`
takes `mentions`: `users`, the people the preview showed tagged, and
`groups`, the user groups checked. The linked owners it tags are read from
what is kept, never from the request, and a post whose linked owners are not
`users` is a `409`. It also takes `edited_text`, posted in place of the
composed announcement beside `text`, the one the edit began from, and
`when: ci_passes`, which holds it until the CI passes and answers `202`;
`DELETE /api/announce/queued` drops a held one, and each frame of
`GET /api/events` carries it as `queued_announcement`.

The issue writes sit under `/api/issues/{key}/`: `GET` and `POST transitions`
list the status changes with the fields each needs and make one,
`PUT assignee` assigns it, and `POST worklog` logs work on a Jira issue.
