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
what you have announced, the last issue list — under your platform's data
directory and never a secret; set `store.disabled` to `true` to keep nothing on
disk.

```sh
workflow            # open the interface
workflow --dry-run  # the same, with every write held back
workflow --web      # serve the loop in a browser instead
```

The browser's side of it has [a page of its own]({{< relref "/docs/web" >}}).

The steps a script or a shell prompt wants also run as commands, without the
interface — `workflow status`, `reviews`, `standup`, `branch`, `pr` and
`announce`. [Scripting]({{< relref "/docs/scripting" >}}) says what a script
can rely on from them: exit codes, which stream carries what, `--json`,
`--yes`, `--dry-run` and `--log`.

## The screen

```text
 ● Issue ─ ● Branch ─ ● Commits ─ ● Review ─ ○ Slack
┏━ 1 Issues ━━━━━━━━━━━━━━┓┌─ Issues ────────────────────────────────────────────────────┐
┃ ▸ ◐ PROJ-412 Fix token… ┃│ PROJ-412 Fix token redaction                                │
┃   ○ PROJ-388 Add retri… ┃│ Bug · In Progress                                           │
┃                         ┃│ reported by Ana Lopez                                       │
┃                         ┃│                                                             │
┃                         ┃│ Tokens reach the log.                                       │
┡━ 2 Branch ━━━━━━━━━━━━━━┩│                                                             │
│ fix/PROJ-412-fix-token… ││ Comments 1 of 1                                             │
│ pushed                  ││                                                             │
├─ 3 Commits ─────────────┤│ Ana Lopez · 2h ago                                          │
│ 1 of 1 staged           ││ Repro'd on 8.2.1                                            │
│ 1 commit on this branch ││                                                             │
├─ 4 Review ──────────────┤│                                                             │
│ #42 fix(config): redac… ││                                                             │
│ ● passed (1 of 1 finis… ││                                                             │
├─ 5 Slack ───────────────┤│                                                             │
│ #dev                    ││                                                             │
│ ○ nothing announced     ││                                                             │
├─ 6 Reviews ─────────────┤│                                                             │
│ 2 review requests wait… ││                                                             │
│                         ││                                                             │
└─────────────────────────┘└─────────────────────────────────────────────────────────────┘
 t change status • c comment • b branch for PROJ-412 • a assign • w log work • ? keys …
```

- **The top row** is how far along the loop the work is. `○` not started, `◐`
  in flight, `●` done, `✗` failed. It is derived, not recorded: the Issue stage
  is done once the branch names an issue, Review follows the pull request's CI.
  The last stage is named for your messaging service, as its pane is.
- **The rail** on the left is the six panes in one box, a light rule between
  them. The focused one is drawn with heavy rules and a bold title, and takes
  the most room; the rest keep a few rows each. The first five follow the
  work — the fifth is named for your messaging service, Slack above — and the
  sixth, Reviews, is the other side of it: the pull requests on your forge
  that wait on your review, the longest-waiting first.
- **The detail pane** on the right shows the focused pane in full. Pickers,
  composers and previews open here too, and take the keyboard until they close.
- **The bottom row** shows what the focused pane can do right now; it changes
  with the pane and with what that pane has loaded. On a narrow terminal the
  keys that do not fit are dropped whole and an ellipsis says so, but `?` is
  never among them: it lists every key.
- **A result** — a push sent, an announcement made, a change refused — appears
  on its own row above the keys, where it stays while you look around and
  clears when the next action starts.

Each pane fails on its own. A Jira that cannot be reached puts its reason in
the Issues pane, and the repository panes carry on.

The layout follows the terminal. Below 80 columns the rail and the detail pane
take turns rather than sharing the width; the detail pane drops its border once
it has fewer than 60 columns; below 24 rows the top row shrinks to a short form,
each stage its initial and glyph.

## Keys

`tab` and `shift+tab` move between panes, and `1`–`6` jump straight to one.
`j`/`k` or the arrow keys move within a list, and `J`/`K` or `pgdn`/`pgup`
scroll the detail pane. Each pane keeps its own place: come back to one and
its detail is scrolled where you left it, unless it shows another branch,
issue or pull request, which starts at the top, or its list reloaded while you
were away, which scrolls to keep the selection in sight. The table below holds
every key `?` lists, by where it works.

| Where | Key | Does |
| --- | --- | --- |
| Moving around | `tab` / `shift+tab` | Next pane, previous pane |
| | `1`–`6` | Jump to a pane |
| | `j`/`k` or `↓`/`↑` | Move within a list |
| | `J`/`K` or `pgdn`/`pgup` | Scroll the detail pane |
| 1 Issues | `t` | Change the selected issue's status |
| | `c` | Comment on it |
| | `a` | Assign it |
| | `w` | Log work on it |
| | `b` | Start a branch for it, or a branch for no issue when none is selected |
| | `o` / `y` | Open the issue in the browser, or copy its URL |
| | `/` | Filter the list as you type; `enter` keeps the filter, `esc` clears it |
| | `v` | Switch which issue list is shown |
| | `ctrl+n` | Load the next page of the list |
| | `r` | Search again |
| | `enter` / `esc` | Below 80 columns, read the selected issue in full, then go back to the list |
| 2 Branch | `b` | Start a branch |
| | `s` | Switch to another issue's branch |
| | `u` | Rebase the branch onto its base, after a last look |
| | `P` | Push a branch that has unpushed commits, after a last look |
| | `r` | Read the repository again |
| 3 Commits | `space` | Stage or unstage the selected file |
| | `a` | Stage every file |
| | `c` | Commit what is staged |
| | `A` | Amend the last unpushed commit with what is staged, after a preview |
| | `f` | Record what is staged as a `fixup!` of an unpushed commit you pick |
| | `h` | Run the pre-commit hook now |
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
| 5, your service | `p` | Preview the announcement of the pull request |
| 6 Reviews | `o` / `y` | Open the selected request in the browser, or copy its URL |
| | `r` | Ask the forge again |
| A composer or preview | `tab` / `shift+tab` | Next field, previous field |
| | `←` / `→` | Change the commit's type; in the announcement preview, change the channel |
| | `ctrl+o` | Write the commit's body, or the pull request's description, in your editor |
| | `ctrl+b` | Mark the commit a breaking change |
| | `ctrl+t` | Use the repository's next pull request template |
| | `ctrl+r` | Open the pull request as a draft, or not |
| | `e` | Edit a comment or an announcement in your editor before it is sent |
| | `space` | Pick an option in a field that takes several |
| | `v` | Keep every existing hook whole as a script, in the lefthook offer |
| | `ctrl+w` | In the branch creator, create the branch in a new git worktree rather than switching to it |
| | `w` | In the announcement preview, announce once CI passes |
| While a command runs | `s` | Stop it |
| | `r` | Run it again, once it has ended |
| | `o` | Show its full output, or every place a failed hook reported |
| Everywhere | `enter` | Do what the bottom row names |
| | `esc` | Close without doing it |
| | `m` | Turn mouse capture off or on, for this session |
| | `?` | Every key |
| | `q` | Quit (`ctrl+c` works even with a preview open) |

Every key here but the pane numbers, `1`–`6`, can be rebound with `ui.keys`;
see [Configuration]({{< relref "/docs/configuration" >}}).

## The loop

### Pick up an issue

The Issues pane lists what is assigned to you and not done, most recently
updated first. The detail pane shows the selected issue's description and
comments. If the current branch names an issue, that issue is selected when
workflow opens, so you land back where you left off.

**Change status** (`t`) lists the transitions Jira's workflow offers from the
issue's status. A transition that needs fields filled in says which. Choosing
it asks for them: a field with a fixed set of values gets a picker, a text field
gets an input. A field of any other kind is named with a pointer to Jira's own
screen, because guessing at it would send something you did not choose.

**Comment** (`c`) opens your editor. The comment is shown back to you before it
is posted: `enter` posts it, `e` edits it again, `esc` discards it.

**Without Jira**, when `jira.base_url` is empty, your forge's issues are the
tracker: the pane lists the open issues assigned to you on the repository's
GitHub or GitLab project. A branch for one is named by its number rather than
a key, as in `feat/42-fix-typo`, and that number is how workflow finds the
issue again. **Change status** (`t`) offers only Close. Comment, assign and
log work (`c`, `a`, `w`), linking the pull request on the issue, the
`jira.views` issue lists (`v`), and opening or copying the issue's URL
(`o` / `y`) do not apply. `workflow doctor` names the tracker in effect.

### Branch

`b` proposes a name from the selected issue: `fix/` for a bug and `feat/` for
anything else, then the key and the summary, as in
`fix/PROJ-412-token-redaction`. Edit it freely; a name git would refuse says so
as you type. The branch starts from origin's default branch, which the overlay
names, and is not set to track it, so it reads as unpushed until it is.
`ctrl+w` creates it in a new git worktree beside the repository instead of
switching to it.

### Stage and commit

The Commits pane lists changed files, one per row. Staging is by whole file.

`c` opens the commit composer. The subject is built from its parts so it is
always a well-formed [Conventional Commit](https://www.conventionalcommits.org/):
`←`/`→` choose the type, `tab` moves to the scope and the description, and a
ruler counts against the 72-character limit. `ctrl+b` marks it a breaking
change, and `ctrl+o` writes the body in your editor. A `Refs:` trailer naming
the issue is added unless the body already has one.

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

`h` in the Commits pane runs the pre-commit hook on its own, without
committing. lefthook skips a pre-commit job when nothing is staged, as a commit
would, so stage something first.

### Open the pull request

`n` in the Review pane opens the composer:

- **The title** is the branch's oldest commit subject, which on a branch of
  Conventional Commits already reads as one.
- **The description** is the repository's pull request template, found where
  GitHub or GitLab looks for it. `ctrl+t` moves between several. With no
  template it lists the branch's commits. Either way it links the branch's
  issue.
- `ctrl+o` edits the description in your editor, `ctrl+r` marks it a draft,
  and `tab` moves through the base branch, the reviewers, the assignees and the
  labels.

`enter` pushes the branch first if it is not pushed — pre-push hooks stream just
as commit hooks do — and opens the pull request only if the push succeeded.

The Review pane then follows CI, asking every twenty seconds while checks run.
`c` lists the checks and opens the selected one's page. `R` re-runs the failed
ones, and `u` on the Branch pane rebases the branch onto its base; like a push,
each first shows a last look naming what it acts on, and does nothing until
`enter`. `e` edits the pull request's title and description. Once it is
green and approved, `M` previews the merge methods the repository permits and
merges by the one you choose; once it has merged, `F` previews the three git
commands that finish the branch — switch to the base, catch it up, delete the
branch — and runs them on `enter`. `n` stays on offer after a merge, as
`workflow pr` allows, so commits made on the branch since can go up in a new
pull request.

### Announce it

`p` in the messaging pane — named for your service, as the top row's last stage
is — previews the announcement:

```text
jacob opened a pull request: <https://github.com/…/pull/42|fix(config): redact tokens>
<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction
```

`e` edits it in your editor, `enter` announces it now, and `w` announces it
once CI passes; with a bot token and more than one channel to choose from,
`←`/`→` change the channel. An announcement waiting for CI is dropped, saying
so, if CI fails. Announcing now replaces one that is waiting, so the channel
never reads it twice.

An announcement waits for the pull request it was written for, and no other.
Switch to another branch while it waits, or replace the pull request, and it is
dropped, saying so, rather than sent for something you never previewed. A pull
request is announced once at each moment — ready for review, CI red, merged —
and the messaging pane and the top row say whether the one on screen has been.
The store remembers what was announced, so a later session does not offer the
same announcement again.

## Dry run

`workflow --dry-run` reads everything as usual and writes nothing. Every action
that would change something — a status change, a comment, a branch, staging, a
commit, a push, a pull request, an announcement, a generated `lefthook.yml` — says
what it would have done instead. The top row starts with `DRY RUN` while it is
on. It opens no
[store]({{< relref "/docs/configuration#what-is-kept-between-sessions" >}})
either, so it starts without the cached issue list, your last commit scope and
what was announced before.

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

Comments, commit bodies, pull request descriptions and announcements are
written in `$VISUAL`, else `$EDITOR`, else `vi`. Everything below the scissors
line (a `>8` cut mark) is help and is not kept. An editor that has to be told
to wait needs saying so: `EDITOR="code --wait"`.

Opening a hook failure at its line works for vi, Vim, Neovim, nano, Emacs,
micro, Kakoune, mg, VS Code, VSCodium, Cursor, Helix, Sublime Text and Zed.
Other editors open the file at the top.

## Mouse and ASCII

A click focuses a pane; a click on a row of the focused pane selects it, and the
wheel scrolls. Mouse capture stops your terminal's own click-and-drag selection,
so `m` turns it off for the session and `ui.mouse` turns it off for good.

`ui.ascii` draws borders and glyphs in plain ASCII, for a font that does not
have them. See [Configuration]({{< relref "/docs/configuration" >}}).

## Limits

- **A push cannot ask for a password.** The interface owns the terminal, so git
  is told not to prompt. Push over SSH, or over HTTPS with a credential helper.
- **Staging is by whole file.** Staging hunks is lazygit's whole project; use
  it, or `git add -p`, alongside.
- **Nothing outlives the session.** An announcement waiting for CI is not sent
  if you quit first.
- **`w` needs checks to wait for.** Where the pull request has no CI at all,
  the preview does not offer it and pressing it does nothing, and an
  announcement already waiting keeps waiting if CI stops reporting any
  checks; announce with `enter` instead.
- **A branch is worked on under the name it shows.** git allows characters in a
  branch's name that cannot be drawn as they are, such as one with no width or
  one that reverses the text after it. A branch, upstream or base named with
  one is refused, and the Branch pane says which.
- **Each announcement is its own message.** A pull request's later
  announcements are posted beside the first rather than in its thread;
  replying in the thread is an idea on the list,
  [FEAT-81](https://github.com/jacob-delgado/workflow/blob/main/FEATURES.md#feat-81-reply-in-the-announcements-own-thread).
