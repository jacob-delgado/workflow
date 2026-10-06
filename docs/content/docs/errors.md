---
title: "Web API errors"
weight: 25
---

# Web API errors

The `workflow --web` API answers a failed request with an
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details object, sent
as `application/problem+json`. (The loopback, same-origin and dry-run **guards**
refuse a request as plain text *before* it reaches the API, so those few
responses are not problem details.)

```json
{
  "type": "https://jacob-delgado.github.io/workflow/docs/errors/#not-found",
  "title": "Not found",
  "status": 404,
  "detail": "issue PROJ-412 was not found",
  "code": "not_found"
}
```

- **`type`** — a stable URI naming the problem; it points at the matching section
  below.
- **`title`** — a short, fixed summary of the problem type.
- **`status`** — the HTTP status code.
- **`detail`** — what went wrong this time, safe to show. It is curated: it never
  carries a secret (tokens are redacted before an error is formed) or an internal
  host. A write that is refused may include the git or forge's own reason so you
  can act on it; a read failure and an unreachable upstream stay generic.
- **`code`** — a stable, machine-readable reason, for a client to switch on rather
  than parsing prose.

The codes below are the whole set.

## Bad request

Status 400. The request could not be understood — a malformed body, a query
or path parameter that did not fit the contract (a task write whose uuid is not
one, say), or a configuration save whose `If-Match` is not in the form of the
`ETag` a read of the configuration returns.

## Not found

Status 404. The addressed resource does not exist: most often an issue that is
not in the tracker, or a `view` the configuration does not name — the issue list
and the event stream both refuse a view they do not know rather than answering
with the default one. Staging and unstaging answer it too for a path the working
tree does not list as changed: they move a change the server read, never a path
of the caller's own. Opening a pull request answers it for a repository the
forge will not show the token. A check's log answers it for an id no
check of the current pull request's CI lists now, or when the branch has no
pull request — the log is read only for a check the forge lists. A path the
API does not serve answers it too.

## Method not allowed

Status 405. The path is one the API serves, but not with the request's method
— a `POST` to a read, say. The `Allow` header lists the methods the path
answers, and so does the detail. The API answers only the methods its contract
declares, so a `HEAD` is refused here as well.

## Conflict

Status 409. The request cannot be applied to the current state — a server not
running in a git repository, for a request that cannot go ahead without reading
it (the command line exits 4 there; the event stream shows the branch, changes
and review empty instead), a working tree with uncommitted changes, a branch
that already exists, nothing staged to commit, no pull request to announce or
to link, no branch checked out (a detached `HEAD`) to link to an issue or to
unlink from one, an announcement that changed since the page previewed it (CI
turned red, the pull request merged — nothing is posted; preview it again), an
announcement whose linked code owners are not the people its preview showed
tagged (a link changed in Settings or a terminal since — nothing is posted;
preview it again), an
issue the checked-out branch does not name, a move to the review status that Jira
does not offer or wants fields filled for (the terminal interface's status
picker asks for them), or a configuration file that changed since Settings read
it — edited on disk, or saved from another tab — which a save refuses rather
than overwrite, short of an edit landing between the save's check and its write
([Web]({{< relref "/docs/web" >}}) names that window). Taskwarrior answers it
too, for a task it changed nothing on — already started, not started, or no
longer pending — and for an undo with nothing to undo. Removing the local data
in Settings answers it for a database file that could not be removed, as one
another program holds open can be on Windows. Every file is set aside before
any is removed, so a file that cannot be set aside leaves them all in place;
one that cannot be deleted after that may leave others already gone, so the
area reads the listing again to show what is left. Close other workflow
sessions and remove again (the command line's `workflow db-clean` exits 4
there).

A git run (`POST /api/runs`) answers it while another run goes, and when
there is nothing to rebase, amend or fix up; editing, merging, finishing and
re-running answer it when the branch's pull request is not at the point each
needs — open, ready to merge, merged, or with failed CI — read afresh; and an
announcement answers it for a moment already announced, from here, the
terminal or `workflow announce`.

## Unprocessable

Status 422. The request was understood but cannot be carried out as asked — an
invalid configuration body (a `ui.keys` map the terminal interface would
refuse to start on among them), a configuration file on disk that no longer reads
as valid (the configuration in effect stands), a request for an issue when no
tracker is configured, no `jira.review_status` to move an issue to, a change
Jira refused, a comment with no text or with no tracker to post it to, a
comment the forge turned down (its own reason, such as a body too long, is in
the detail), a comment on a repository whose forge cannot be told or whose
origin names no repository, a file git would not stage or unstage, or a branch git would not
switch to or create (git's own words stay off the wire, since a fetch it
makes on the way can name the remote; the detail says how to see them), or a
branch's link to an issue that git could not write to, or remove from, the
repository's configuration. Linking a branch answers it too for a key that
names no issue — neither a Jira key like `PROJ-7` nor a forge number like `#42`
— and linking or unlinking does for a server that cannot link a branch at
all. A check's log answers it for a check the forge keeps no log for: a
commit status, or a check run from an app other than GitHub Actions. A push
that ran and failed is answered here with git's own output, since the reason —
a ref the remote rejected, a hook's refusal — is in it, with the remote's URL
or `user@host:path` address taken out (a bare host git prints — a remote
written `host:path`, or the host a connection error names — can remain); one
that could not start says to push from a terminal to see why.
A Jira token that is not configured, that its command or variable did not give,
or that Jira did not accept, a `jira.base_url` that is not a usable address,
and one with no Jira API behind it are answered here too, pointing at
`workflow doctor` rather than naming the address. So are a forge token that was not found or that the forge did not
accept, a `forge.kind` set without its `forge.host`, a forge address with no
forge API behind it, and a request the forge refused. A token the forge turned
down is told in the words the terminal uses: which forge, the scope it asks
for, and the forge's own reason. A pull request the forge turned down is answered here with
the forge's own reason, while a reviewer or assignee GitLab does not know
never stops one opening; any other open that fails is answered as a read's failure is, under
the code its cause belongs to. An announcement the messaging service refused,
or one that could not be sent because messaging has no credential (none set, or
a token its command or variable did not give) or its webhook is not https, is
answered here too — never with the service's own error, which can name the
webhook. So is a change to your Taskwarrior tasks when there is no Taskwarrior
to make it — turned off by `taskwarrior.disabled`, not installed, a task
program that is not Taskwarrior (go-task, most likely), one older than 3.5.0,
one never run, or one whose taskrc has a malformed line — as is an empty task
line or note, or an issue whose tracker key is not one word. A line Taskwarrior
refused is answered here too, in Taskwarrior's own words less where its data is
and any line naming a server; they say what in the line it could not take. A
sync with no backend named in the taskrc is answered here, and so is a sync
that failed, in fixed words: Taskwarrior's name the sync server, so the detail
says to run `task sync` in a terminal to see them.
The Local data area in Settings answers it when there is no directory to keep
the store in (no home directory is set), for a server wired with no store, and
for a removal that finds something other than the store's own plain file — a
symlink or a directory — where a database file belongs, which it refuses
without removing anything (`workflow db-clean` exits 1 there).
People and groups answers it when the configuration in effect has no Slack
user token to read the directory with (a webhook, say, or a token Slack holds
no credential for), or no store to keep people in; for a link that names both a
Slack ID and "not on Slack", or neither; for a Slack ID of the wrong kind for
the owner (a team links to a user group, a person to a user) or one the
channel's members or the workspace's user groups do not hold (a person is
checked against the channel the link names, the configured one when it names
none); for a new repository group, never one already saved, that the
workspace's user groups do not hold; for a token
that lacks a scope the read needs, which the detail names; and for kept data
a build of workflow with another schema wrote, which is left as it is. Nothing is written
then. (A Slack directory read on its own answers a missing scope with a 200
naming it, so the preview can still post.) An announcement answers it for
mentions that name a user group the announcement did not offer, or that it
was given where it tags no one — only a ready-for-review announcement with a
Slack user token in the configuration in effect at the post tags, so a switch
to a webhook in Settings after the preview lands here — and posts nothing.

## Precondition required

Status 428. A configuration save named no revision to write over: it carried no
`If-Match` header with the `ETag` its read returned. Settings always sends one,
so from the browser this is a fault in the page; reload it, then save again.

## Unreachable

Status 502. An upstream service — Jira, the Git forge or the messaging
service — could not be reached, asked to wait because it is limiting
requests (a forge's refusal whose headers ask for a wait among them),
answered with a redirect (refused, so a credential goes nowhere else), or,
for the forge or the messaging service, answered with a status it does not
document; or Taskwarrior did not answer in the time it is given. A failed
check's log answers it too when the forge sent the log to an address that is
not https or redirected it nowhere, or when the storage it sent the log to did
not hand it over — a signed address that expired, or a log since deleted —
which is the storage's failure, not the token's. The request was well formed;
try again once the service is back.

## Fetch failed

Status 502. Start work fetches origin first, so the new branch starts from
what origin holds now, and the fetch failed: the network, origin's address or
your credential for it. Nothing was made. The page offers to branch from what
you have, which asks again without the fetch (`"fetch": false`); `git fetch`
in a terminal shows git's own reason, which the detail leaves out because it
names origin.

## Internal

Status 500. An unexpected failure the caller cannot act on — among them a
read of the repository that git could not answer, when the request cannot go
ahead without it (a write that has already landed answers all the same, naming
the branch as far as it can, and the event stream shows that panel empty). The
detail stays generic on purpose, and says to try again and to run `workflow
doctor` if it keeps failing; the cause is not in the response but on the
standard error of the `workflow --web` that answered, one `workflow web:` line
per failure, a cause of several lines joined by semicolons, with every
credential the configuration holds masked.
