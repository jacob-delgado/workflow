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
```

The steps a script or a shell prompt wants also run as commands, without the
interface — `workflow status`, `reviews`, `standup`, `branch`, `pr` and
`announce`. [Scripting]({{< relref "/docs/scripting" >}}) says what a script
can rely on from them: exit codes, which stream carries what, `--json`,
`--yes`, `--dry-run` and `--log`.

## The screen

```text
 ● Issue ─ ● Branch ─ ● Commits ─ ● Review ─ ○ Slack
┏━ 1 Issues ━━━━━━━━━━━━━━┓┌─ Issues ──────────────────────────────────────────┐
┃ ▸ ◐ PROJ-412 Fix token… ┃│ PROJ-412 Fix token redaction                      │
┃   ○ PROJ-388 Add retri… ┃│ Bug · In Progress                                 │
┃                         ┃│ reported by Ana Lopez                             │
┡━ 2 Branch ━━━━━━━━━━━━━━┩│                                                   │
│ fix/PROJ-412-fix-token… ││ Tokens reach the log.                             │
│ pushed                  ││                                                   │
├─ 3 Commits ─────────────┤│ Comments 1 of 1                                   │
│ 1 staged · 1 changed    ││                                                   │
│ 1 commit on this branch ││ Ana Lopez · 2h ago                                │
├─ 4 Review ──────────────┤│ Repro'd on 8.2.1                                  │
│ #42 fix(config): redac… ││                                                   │
│ ● passed (1 of 1 finis… ││                                                   │
├─ 5 Slack ───────────────┤│                                                   │
│ #dev                    ││                                                   │
│ ○ nothing posted        ││                                                   │
└─────────────────────────┘└───────────────────────────────────────────────────┘
 t change status • c comment • b branch for PROJ-412 • a assign • ? keys …
```

- **The top row** is how far along the loop the work is. `○` not started, `◐`
  in flight, `●` done, `✗` failed. It is derived, not recorded: the Issue stage
  is done once the branch names an issue, Review follows the pull request's CI.
- **The rail** on the left is the five panes in one box, a light rule between
  them. The focused one is drawn with heavy rules and a bold title, and takes
  the most room; the rest keep a few rows each.
- **The detail pane** on the right shows the focused pane in full. Pickers,
  composers and previews open here too, and take the keyboard until they close.
- **The bottom row** shows what the focused pane can do right now; it changes
  with the pane and with what that pane has loaded. On a narrow terminal the
  keys that do not fit are dropped whole and an ellipsis says so, but `?` is
  never among them: it lists every key.
- **A result** — a push sent, a post made, a change refused — appears on its
  own row above the keys, where it stays while you look around and clears when
  the next action starts.

Each pane fails on its own. A Jira that cannot be reached puts its reason in
the Issues pane, and the repository panes carry on.

The layout follows the terminal. Below 80 columns the rail and the detail pane
take turns rather than sharing the width; below 60 the borders go as well; below
24 rows the top row shrinks to a short form, each stage its initial and glyph.

## Keys

`tab` and `shift+tab` move between panes, and `1`–`5` jump straight to one.
`j`/`k` or the arrow keys move within a list, and `J`/`K` or `pgdn`/`pgup`
scroll the detail pane.

| Where | Key | Does |
| --- | --- | --- |
| 1 Issues | `t` | Change the selected issue's status |
| | `c` | Comment on it |
| | `a` | Assign it |
| | `w` | Log work on it |
| | `b` | Start a branch for it |
| | `/` | Filter the list as you type; `enter` keeps the filter, `esc` clears it |
| | `v` | Switch which issue list is shown |
| | `r` | Search again |
| | `enter` / `esc` | Below 80 columns, read the selected issue in full, then go back to the list |
| 2 Branch | `b` | Start a branch |
| | `s` | Switch to another issue's branch |
| | `u` | Rebase the branch onto its base, once a last look is confirmed |
| | `P` | Push a branch that has unpushed commits |
| | `r` | Read the repository again |
| 3 Commits | `space` | Stage or unstage the selected file |
| | `a` | Stage every file |
| | `c` | Commit what is staged |
| | `h` | Run the pre-commit hook now |
| | `g` | Set up lefthook for hooks it does not manage |
| 4 Review | `n` | Open a pull or merge request |
| | `R` | Re-run failed CI, once a last look at the pull request is confirmed |
| | `r` | Look for the pull request and its CI again |
| 5 Slack | `p` | Preview the post announcing the pull request |
| New branch | `ctrl+w` | Create it in a new git worktree rather than switching to it |
| Slack preview | `w` | Post automatically once CI passes |
| Anywhere | `m` | Turn mouse capture off or on, for this session |
| | `?` | Every key |
| | `q` | Quit (`ctrl+c` works even with a preview open) |

Inside a picker, composer or preview, `enter` does the thing it names in the
bottom row and `esc` closes without doing it.

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

### Branch

`b` proposes a name from the selected issue: `fix/` for a bug and `feat/` for
anything else, then the key and the summary, as in
`fix/PROJ-412-token-redaction`. Edit it freely; a name git would refuse says so
as you type. The branch starts from origin's default branch, which the overlay
names, and is not set to track it, so it reads as unpushed until it is.

### Stage and commit

The Commits pane lists changed files, one per row. Staging is by whole file.

`c` opens the commit composer. The subject is built from its parts so it is
always a well-formed [Conventional Commit](https://www.conventionalcommits.org/):
`←`/`→` choose the type, `tab` moves to the scope and the description, and a
ruler counts against the 72-character limit. `ctrl+e` writes the body in your
editor. A `Refs:` trailer naming the issue is added unless the body already has
one.

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
- `ctrl+e` edits the description in your editor, `ctrl+d` marks it a draft, and
  `tab` moves to the base branch.

`enter` pushes the branch first if it is not pushed — pre-push hooks stream just
as commit hooks do — and opens the pull request only if the push succeeded.

The Review pane then follows CI, asking every twenty seconds while checks run.

### Tell the team

`p` in the Slack pane previews the announcement:

```text
jacob opened a pull request: <https://github.com/…/pull/42|fix(config): redact tokens>
<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction
```

`e` edits it in your editor, `enter` posts it now, and `w` posts it once CI
passes. A post waiting for CI is dropped, saying so, if CI fails. Posting now
replaces a post that is waiting, so the channel never reads it twice.

A post waits for the pull request it was written for, and no other. Switch to
another branch while it waits, or replace the pull request, and it is dropped,
saying so, rather than sent for something you never previewed. Each pull request
is announced once in a session, and the Slack pane and the top row say whether
the one on screen has been.

## Dry run

`workflow --dry-run` reads everything as usual and writes nothing. Every action
that would change something — a status change, a comment, a branch, staging, a
commit, a push, a pull request, a Slack post, a generated `lefthook.yml` — says
what it would have done instead. The top row starts with `DRY RUN` while it is
on.

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
hooks as `.git/hooks/*.old`. An existing `lefthook.yml` is never overwritten.
The offer only appears when `lefthook` is installed.

## Editor

Comments, commit bodies, pull request descriptions and Slack messages are
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
- **Nothing outlives the session.** A Slack post waiting for CI is not sent if
  you quit first.
- **`w` waits for checks that exist.** In a repository with no CI at all it
  keeps waiting; post with `enter` instead.
- **A branch is worked on under the name it shows.** git allows characters in a
  branch's name that cannot be drawn as they are, such as one with no width or
  one that reverses the text after it. A branch, upstream or base named with
  one is refused, and the Branch pane says which.
- **Each Slack post is its own message.** With nowhere to keep a message's
  timestamp, later posts cannot thread under the first.
