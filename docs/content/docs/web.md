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
workflow --web            # serve http://127.0.0.1:7000
workflow --web --dry-run  # the same, read-only
```

Run it inside a repository, then open `http://127.0.0.1:7000`. It listens on
the loopback interface alone, on port 7000, and refuses a write from a page
served anywhere else, so a site open in another tab cannot drive it. `ctrl+c`
in the terminal stops it.

The release binaries and `task build` carry the web app inside them. A binary
built without it — `go install`, say — still serves the API, and its page says
the web interface is not built in.

**Read-only with `--dry-run`.** A banner under the header says so, and every
write is held back in the browser before it is sent, saying *Held back by
--dry-run: nothing was sent.* The server refuses every write as well, so
nothing changes even if a request gets past the page. Unlike the terminal, the
web does not narrate what a held-back write would have done.

## The page

The header holds the version, the theme and the stream's state. The rail down
the left holds the six sections; the one you are in, and its heading, take the
hue of the system it belongs to. Below a medium width the rail keeps its icons
alone; each still has its name, shown on hover and read by a screen reader.

**The theme** button cycles System, Light and Dark. System follows your
operating system; the choice is remembered in this browser.

**The stream** keeps the page current. The server reads the repository and
the services every five seconds and pushes what it finds, so nothing on the
page needs refreshing by hand. Its state is beside the theme:

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
with a **View** select for the views your configuration defines, a **Filter**
over the issues loaded so far, by key or summary, and **Load more** while the
view holds more. An issue with a local branch is marked *in flight*, with
**Check out** beside it when that branch is not the one checked out.

The selected issue's detail shows its type, priority, reporter and assignee, a
link to open it in Jira, its description and its comments, and its **work
story**: the branch, the changes and the pull request, each done or not yet,
then the announcement, which the story never marks done because the web keeps
no record of one — each step opening the section it belongs to. From the story,
**Start work** creates and checks out a branch named for the issue, and **Check
out this branch** switches to one it already has. Below a large width the list
sits over the detail rather than beside it.

### Branch

The checked-out branch, its base, its upstream and how far it is ahead or
behind, and the commits on it. **Push branch** publishes it, after a
confirmation.

Under **Working tree**, each changed file has its own **Stage** or
**Unstage**, and **Stage all** stages the rest. The commit form builds a
Conventional Commit from its type, scope, subject, body and a breaking-change
box, with the scope the terminal would suggest already filled in; **Commit
staged changes** commits, adds the `Refs:` trailer for the branch's issue, and
runs the repository's own hooks. A hook that refuses the commit says why.

### Review

The branch's pull request — its number, title, state, mergeability, approvals
and requested changes — and its CI checks. With none open, **Open a pull
request** composes one as `workflow pr` would and shows it as a form: the
title, the base, the reviewers, assignees and labels, the description, and
whether it is a draft. **Open pull request** pushes the branch first when it is
not published, then opens it. If the forge would not add every reviewer,
assignee or label, a warning says so.

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
sends it. Nothing is sent before that second press.

### Reviews

The pull requests on your forge that wait on your review, the longest-waiting
first: where each is, who asks, how long it has waited, whether it is a draft,
and how its CI stands, each with a link to open it and **Copy URL**. The section
asks the forge when you open it, unless it asked within the last minute, and
**Refresh** asks again.

### Settings

The configuration file in effect, in seven parts — Jira, the forge, messaging,
branches, commits, pull requests and the store — and **Save changes** writes it
back. A credential is shown masked and kept as it is unless you type a new one.
The parts the form does not show yet (`ui`, `timing`, `headers`, `views` and
`branch.prefixes`) are kept unchanged when you save.

## What stays in the terminal

The web covers the loop's main line. These stay with the terminal interface, or
the command line, by decision rather than by oversight; most are written down
as ideas in
[FEATURES.md](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md)
or [UX.md](https://github.com/jacob-delgado/workflow/blob/main/UX.md):

- **Re-running failed CI, merging, finishing a merged branch and editing an
  open pull request** — the terminal's `R`, `M`, `F` and `e`
  ([FEAT-79](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-79-review-actions-on-the-web)).
- **Moving an issue to any status but the review status, commenting,
  assigning and logging work** — the terminal's `t`, `c`, `a` and `w`
  ([FEAT-80](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-80-issue-writes-on-the-web)).
- **Editing an announcement before it is sent, and announcing once CI
  passes** — the terminal's `e` and `w` in the announcement preview.
- **Rebasing onto the base, amending, fixing up, reading a file's diff,
  choosing among pull request templates, creating a branch in a worktree,
  running the pre-commit hook on its own and generating a `lefthook.yml`** —
  the terminal alone.
- **Writing a first configuration file** — `workflow config init`; Settings
  edits a file that already exists.

## When a request fails

A failed request says why beside the button that made it, in words meant for
you, and the button is there to try again. Behind that, the API answers a
failed request with an RFC 9457 problem details object whose `code` a script
can rely on; only the loopback, same-origin and `--dry-run` guards, which
refuse a request before it reaches the API, answer in plain text. [Web API
errors]({{< relref "/docs/errors" >}}) lists the codes.
