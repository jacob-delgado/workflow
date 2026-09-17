# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up ". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done.

Checked against commit `817d323` on 2026-09-17. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

Two passes, and one gap.

1. **A read of the whole surface.** Every key binding, string, empty state,
   loading state and error state in `internal/tui` and `internal/cli` was
   cataloged. A second, independent pass then tried to refute each claim by
   rendering the real model in throwaway tests. Of 33 claims, 27 held as
   written, 6 were corrected in a detail, and none was refuted.
2. **Driving the real program.** The built binary was run in a terminal
   multiplexer at 120×36, 100×20, 89×30, 80×24, 80×12, 59×20 and 24×6, in
   three scenes: outside a repository, in a fresh repository, and in a
   repository with commits, changed files, a local `origin` and an old-style
   `.git/hooks/pre-commit`. The colors on screen were read from the raw escape
   sequences. Entries marked **seen live** come from these runs.
3. **The gap.** Every run was unconfigured, with no credential reachable. The
   configured path (issues listed, a pull request followed, a post sent) was
   never watched live. Those screens are known from the code and from the
   screen tests only.

## How to read an entry

- **Impact** and **Effort** are estimates. Effort is small (a day or less),
  medium (a few days) or large (a week or more).
- **Today** is what happens now, with the evidence.
- **Instead** is one proposal. There are usually others.
- **Done when** is observable, so a screen test can assert it.

The sections follow the panes, in the order the work goes, with the first run
before them and the things every pane shares after them.

## The promises the interface makes

The interface states its own rules, in its docs and in its code. They are
good rules. Most of what follows is a place where the screen does not keep
one of them yet, which makes this table the shortest summary of the file.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "a key it does not show does nothing" | `docs/content/docs/usage.md:55` | No. Ten keys work unseen. UX-32 |
| "`?` lists every key" | `docs/content/docs/usage.md:56` | No. Eleven bindings are missing. UX-33 |
| "the one way the interface says something broke" | `failure`, `internal/tui/render.go:272` | In 7 places of about 20. UX-40 |
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:43` | Mostly. UX-13, UX-23 |
| "a refused change must never go unseen" | `internal/tui/picker.go:183` | In one overlay of seven. UX-35 |
| "Each pane fails on its own" | `docs/content/docs/usage.md:58` | Yes, and it is the best thing about the first run. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:14` | Yes. It reads in monochrome. |

## The first run

### UX-01 At the default terminal size, "see detail" points at nothing

Impact: high · Effort: small

- Today: below 90 columns the rail and the detail pane take turns, and the
  Issues pane draws only its list (`internal/tui/render.go:92`,
  `internal/tui/panes.go:65`). A new user
  in an 80×24 terminal, the size most terminals open at, sees this and
  nothing else (seen live):

  ```text
   ● Issue ─ ● Branch ─ ● Commits ─ ○ Review ─ ○ Slack
  ┌─ Issues ───────────────────────────────────────────────────────────────┐
  │ ✗ failed · see detail                                                  │
  │                                                                        │
  └────────────────────────────────────────────────────────────────────────┘
   r refresh • ? keys • tab next pane • 1-5 jump to pane • q quit
  ```

  There is no detail to see. The reason ("no jira.token is configured") and
  the pointer to `workflow config init` exist only at 90 columns and up. No
  key reaches them. An issue's description and comments are unreachable at
  this width too.
- Instead: in the collapsed layout, draw the detail whenever the list has
  nothing to choose from, and give the Issues pane a key (`enter`, with `esc`
  back) to look at the selected issue.

  ```text
   ● Issue ─ ● Branch ─ ● Commits ─ ○ Review ─ ○ Slack
  ┌─ 1 Issues ─────────────────────────────────────────────────────────────┐
  │ ✗ Jira is not set up yet.                                              │
  │                                                                        │
  │ Run `workflow config init`, add your Jira token to the file it         │
  │ writes, then `workflow doctor` to check it.                            │
  └────────────────────────────────────────────────────────────────────────┘
   r try again • ? keys • tab next pane • 1-5 jump to pane • q quit
  ```

- Touches: `internal/tui/render.go` (`detailContent`), `internal/tui/panes.go`
  (`narrow`), `internal/tui/detail.go`.
- Done when: at 80×24 with no configuration the reason and the next step are
  on screen, and with issues listed `enter` shows the selected issue.

### UX-02 Keep the two-pane layout at 80 columns

Impact: high · Effort: small

- Today: `collapseBelow = 90` (`internal/tui/layout/layout.go`). The layout
  the usage guide draws, rail beside detail, is never seen in a default
  terminal. The rail's minimum is 24 columns, which would leave 56 for the
  detail at 80.
- Instead: collapse below 80, not below 90, and check the result against real
  issue text before settling the number.
- Touches: `internal/tui/layout/layout.go`, its tests, the breakpoints named
  in `docs/content/docs/usage.md`.
- Done when: an 80×24 terminal shows the rail and the detail side by side.

### UX-03 One sentence for "you are not set up", said the same way everywhere

Impact: medium · Effort: small

- Today: what the user is told depends on where they are standing.

  | Situation | Names `config init` | Names `doctor` |
  | --- | --- | --- |
  | No file, in the interface | yes | no |
  | Incomplete file, in the interface | no | yes |
  | Unreadable file, in the interface | no | no |
  | `workflow config show`, no file | no | no |
  | `workflow doctor`, no file | yes | yes |

  (`Model.status`, `internal/tui/render.go`; `runConfigShow`,
  `internal/cli/config_cmd.go`.)
- Instead: one block of copy with both steps in order, used by all five. For
  an unreadable file: what is wrong, on which line, and that
  `workflow config init --force` starts over.
- Touches: `internal/tui/render.go`, `internal/cli/config_cmd.go`,
  `internal/cli/doctor.go`.
- Done when: all five say the same two steps in the same words.

### UX-04 Let every pane say what it is missing

Impact: high · Effort: medium

- Today: the configuration summary is drawn in one place, the Issues detail,
  and only when the search failed or nothing is selected
  (`issueDetailView`, `internal/tui/detail.go`). Once issues are listed,
  "incomplete: slack.webhook_url" can never be seen. The Slack pane says
  `(not set)`, with no label and no advice, and still offers `p`, which fails
  with "no slack.token or slack.webhook_url is configured" after the message
  has been written (seen live for the pane; `canPost`,
  `internal/tui/slack.go`).
- Instead: each pane owns its unconfigured state, and an action that cannot
  work is not offered.

  ```text
  ┌─ Slack ────────────────────────────────────────────────────────────────┐
  │ Slack is not set up.                                                   │
  │                                                                        │
  │ Add slack.webhook_url (or slack.token and slack.channel) to            │
  │ ~/.workflow.json. `workflow doctor --online` checks it.                │
  └────────────────────────────────────────────────────────────────────────┘
  ```

- Touches: `internal/tui/slack.go`, `internal/tui/review.go`,
  `internal/tui/render.go`.
- Done when: with Slack unset, the pane names the keys to set and the file to
  set them in, and the bottom row does not offer `p`.

### UX-05 Outside a repository, say so once

Impact: medium · Effort: small

- Today (seen live): the Branch pane says it could not read the branch, with
  git's "not a git repository" under it, and still offers `b`, which fails
  with git's own words, cut off at the pane's edge.
  The Commits pane offers `h run pre-commit` and prints
  "reading the status of /a/long/path: git: exit status 128: fatal: not a git
  repository (or any of the parent directories): .git". The Review pane says
  "on no feature branch". Three panes describe one fact four ways, and two of
  them offer actions that cannot work.
- Instead: the three repository panes share one plain sentence ("Not inside a
  git repository. Start `workflow` from one to use this pane.") and offer no
  repository keys. The Issues pane carries on, as the design intends.
- Touches: `internal/tui/branch.go`, `internal/tui/commits.go`,
  `internal/tui/review.go`.
- Done when: outside a repository no pane offers `b`, `P`, `h`, `c` or `n`.

### UX-06 Do not open on the lefthook offer

Impact: medium · Effort: small

- Today (seen live): in a repository with a hook in `.git/hooks` and lefthook
  installed, the first thing a new user sees is a full-pane offer to write a
  `lefthook.yml` and run `lefthook install`. They have not yet seen the
  interface they started. At 80×24 the offer is the whole screen. `esc` skips
  it and nothing brings it back in that session.
- Instead: open on the normal screen. Say once, in the Commits pane, that a
  hook is not managed by lefthook and which key shows the offer. Keep that
  key.
- Touches: `internal/tui/hookgen.go`, `internal/tui/commits.go`,
  `internal/tui/tui.go` (`Init`).
- Done when: the first screen is the five panes, and the offer can be opened
  and reopened from the Commits pane.

## Issues

## Branch

### UX-13 Give a push the same last look as every other write

Impact: medium · Effort: small

- Today: `P` pushes on a single key press (`handleBranchKey`,
  `internal/tui/branch.go`). A comment, a status change, a branch, a commit, a
  pull request and a Slack post are all shown before they are sent. A push is
  the one outward write that is not.
- Instead: the run overlay opens first, showing "push
  fix/PROJ-412-token-redaction to origin", and `enter` starts it. This is a
  judgment call: lazygit also pushes on `P` alone. The argument for changing
  it is the interface's own rule, not the convention.
- Touches: `internal/tui/run.go` (`startPush`), `internal/tui/branch.go`.
- Done when: `P` shows what will be pushed and where, and `esc` sends
  nothing.

### UX-15 Say how old the base is

Impact: medium · Effort: small

- Today: the overlay says "from origin/main". Nothing fetches, so that means
  "from wherever origin/main was when you last fetched", and the screen does
  not say when that was.
- Instead: "from origin/main, fetched 3 days ago". FEAT-12 in
  [FEATURES.md](FEATURES.md) goes further and fetches.
- Touches: `internal/gitrepo/branch.go`, `internal/tui/branch.go`.
- Done when: the overlay shows the age of the base.

## Commits

### UX-17 Put the heavy border where the cursor is

Impact: medium · Effort: small

- Today (seen live): focus is shown by a heavy border, and in the Commits
  pane the heavy border is around two summary lines in the rail while the
  cursor, the list and everything the keys act on are in the light-bordered
  detail pane. In the Issues pane the list is in the rail, so border and
  cursor agree. The eye is sent to the wrong box in one pane and the right box
  in the next.
- Instead: when a pane's list lives in the detail, the detail takes the heavy
  border, as an overlay already does. Border weight stays the focus signal;
  this only changes which box carries it.
- Touches: `internal/tui/render.go` (`View`, `detailContent`),
  `internal/tui/panes.go`.
- Done when: in every pane the heavy border surrounds the row marker.

### UX-20 Lead a failed run with what failed

Impact: medium · Effort: small

- Today (seen live): the most prominent line of a failed commit is
  `✗ git: exit status 1`. The line that explains it
  (`.git/hooks/pre-commit: line 3: go: command not found`) is below, and when
  the output has `file:line` places in it, the raw output is replaced by that
  list and cannot be brought back (`commandRun.view`,
  `internal/tui/run.go`). There is no scrolling back through a long run.
- Instead: "✗ the commit was refused by the pre-commit hook" as the headline,
  a key that switches between the places and the full output, and scrolling
  in both.
- Touches: `internal/tui/run.go`.
- Done when: the headline names the step that failed, and the full output of
  a finished run can always be read.

## Review

### UX-23 Do not say "the forge did not answer" when it was never asked

Impact: medium · Effort: small

- Today (seen live): with a remote that is not a forge address the rail says
  "✗ the forge did not answer" and the detail adds "✗ reading origin: not a
  repository remote". The same rail line covers a missing token and an
  unrecognized host. In none of those was a request sent. `n` is still
  offered, the composer opens, and `enter` **pushes the branch** before
  failing with the error that was already on screen (`canOpenPullRequest`,
  `internal/tui/review.go`; `prComposer.open`,
  `internal/tui/prcomposer.go`).
- Instead: a rail line per cause ("no token for github.com", "origin is not
  GitHub or GitLab", "could not reach github.com"), each with its next step
  in the detail, and no `n` until a pull request could actually be opened.
- Touches: `internal/tui/review.go`, `internal/wiring/wiring.go`
  (`connectForge` returns distinct sentinels already).
- Done when: with no token, `n` is not offered and nothing is pushed.

### UX-25 Call it a merge request on GitLab

Impact: low · Effort: small

- Today: the interface says "pull request" and `#42` everywhere, on GitLab
  too. The README and the docs say "pull or merge request".
- Instead: the noun and the sigil follow the forge: "merge request", `!42`.
- Touches: `internal/tui/review.go`, `internal/tui/prcomposer.go`,
  `internal/tui/keys.go`, `internal/slack/post.go` (the announcement text).
- Done when: on a GitLab remote no string says "pull request".

### UX-26 Keep what was written in the pull request composer

Impact: high · Effort: medium

- Today: three ways to lose it, all reproduced in tests.
  - `ctrl+t` replaces the description with the next template even when there
    is one template, so a description written in the editor is wiped by a key
    whose label is "next template".
  - When `enter` has to push first and the push fails, `esc` on the failed
    run discards the title, the description and the draft flag. `r` keeps
    them. The commit composer keeps its draft in exactly this case.
  - `esc` in the composer keeps nothing.
- Instead: keep a draft per branch for the session, as the commit composer
  does; never re-template a description that has been edited without asking;
  and do not show "ctrl+t next template" when there is no next template
  (seen live with "no template in this repository" on the same screen).
- Touches: `internal/tui/prcomposer.go`, `internal/tui/run.go`.
- Done when: a failed push followed by `esc` and `n` reopens the composer
  with what was typed.

### UX-28 Show that CI is being watched

Impact: medium · Effort: small

- Today: CI is asked about every twenty seconds while it runs. The pane shows
  a state and no time, so "running (2 of 5 finished)" looks the same whether
  it was read a second ago or the forge stopped answering ten minutes ago.
  After `r`, the state drops back to "checking…" even for the same pull
  request. In a repository with no CI, `w post when CI passes` waits forever,
  which the usage guide lists as a limit and the screen does not mention.
- Instead: "running (2 of 5 finished) · checked 14:02". When the forge
  reports no checks, do not offer `w`, and say why.
- Touches: `internal/tui/review.go`, `internal/tui/slack.go`.
- Done when: the CI line carries the time it was last read, and `w` is absent
  when nothing reports CI.

## Slack

### UX-30 Let a queued post be seen, withdrawn and mourned

Impact: medium · Effort: small

- Today: a queued post can be replaced but not withdrawn. `q` quits with one
  waiting and says nothing, although the post is lost. When CI fails, "✗ CI
  failed, so nothing was posted to Slack" appears in the bottom row and the
  next key press clears it; the pane then reads "○ nothing posted", as if
  nothing had been asked.
- Instead:

  ```text
  ┌─ Slack ────────────────────────────────────────────────────────────────┐
  │ to     #dev-workflow                                                   │
  │ state  ✗ not posted: CI failed at 14:02                                │
  │                                                                        │
  │ p posts it anyway.                                                     │
  └────────────────────────────────────────────────────────────────────────┘
  ```

  A key withdraws a queued post. `q` with one waiting asks first: "A post is
  waiting for CI and will be lost. enter quit • esc stay".
- Touches: `internal/tui/slack.go`, `internal/tui/tui.go`
  (`handleGlobalKey`).
- Done when: a dropped post leaves a line in the pane until the next post,
  and quitting with a queued post asks once.

## Across the interface

### UX-32 Make the bottom row keep its promise, or change the promise

Impact: medium · Effort: small

- Today: the usage guide says "a key it does not show does nothing". These
  work without being shown: `r` in the Issues pane when an issue is selected,
  `r` in the Branch and Commits panes, `j`/`k`, `J`/`K`, `pgup`/`pgdn`,
  `shift+tab` and `m`. Pressing `r` in each pane was confirmed to reload.
- Instead: show `r` wherever it works, since it is a pane action. Then say
  what the row really is: "the bottom row shows what this pane can do right
  now; `?` has the keys for moving around."
- Touches: `internal/tui/detail.go` (`issuesKeys`), `internal/tui/branch.go`
  (`branchKeys`), `internal/tui/commits.go` (`commitsKeys`),
  `docs/content/docs/usage.md`, `README.md`.
- Done when: every pane action that works is in the bottom row, and the docs
  describe the row as it is.

### UX-33 A help screen that knows where you are

Impact: medium · Effort: medium

- Today: `?` claims to list every key. It lists 27 and omits the ones that
  live in overlays: `e`, `ctrl+e`, `ctrl+t`, `ctrl+d`, `v`, `r` for "run
  again", the field keys, `←`/`→` and `ctrl+c`. It files `w` under "Review
  and Slack" though `w` works only inside the Slack preview, and `enter
  apply` and `esc close` under "Everywhere" though they do nothing on the main
  screen. `?` itself does not work inside an overlay, which is where the
  missing keys are. At 120×36 the list is longer than the pane, the last rows
  fall off the bottom, and nothing says there is more (seen live).
- Instead: group by place ("In a composer", "In a preview", "While a command
  runs"), lay the groups out in two columns, show a "more below" mark when
  clipped, and open context help with a key that a text field does not need.
- Touches: `internal/tui/keys.go` (`FullHelp`, `helpGroups`),
  `internal/tui/render.go` (`helpView`).
- Done when: every binding in `newKeyMap` appears in the help, and a test
  fails when a new binding is left out.

### UX-34 Let a message stay long enough to be read

Impact: high · Effort: medium

- Today: every result ("● opened #42 https://…", "● posted to #dev", "✗ CI
  failed, so nothing was posted") is one line that replaces the key hints,
  is cut at the terminal's width with no mark, and is cleared by the next key
  press of any kind (`Model.footer`, `internal/tui/render.go`;
  `handleKey`, `internal/tui/tui.go`). Pressing `j` to look around erases the
  only record of what just happened. `m` changes mouse capture and shows
  nothing at all.
- Instead: give notices their own row above the hints when there is height
  for it. Clear one when the next action starts, not on the next key. Write
  anything that matters later (a dropped post, a failed push) into its pane
  as well. `m` says what it did: "mouse off: your terminal selects text
  again".
- Touches: `internal/tui/render.go`, `internal/tui/tui.go`,
  `internal/tui/overlay.go` (`noticed`), `internal/tui/layout/layout.go`.
- Done when: a notice survives `j`, `k` and `tab`, and the key hints stay
  visible beside it.

### UX-35 Pin every outcome where it cannot be pushed off screen

Impact: high · Effort: medium

- Today: an overlay draws its result line ("posting…", "✗ …") after its body,
  unwrapped. Two things follow. A long reason is cut with an ellipsis (seen
  live: "✗ creating branch x: git: exit status 128: fatal: not a git
  repository (or any …"). And when the body is taller than the pane, the
  result is below the fold with no way to scroll to it: a failed lefthook
  install at 80×24 with two hooks showed no error at all. The status picker
  reserves room for its outcome, with the comment "a refused change must
  never go unseen". The other six overlays do not.
- Instead: every overlay draws its state directly under its title, as the run
  overlay already does, wrapped to the pane's width.
- Touches: `internal/tui/comment.go`, `internal/tui/branch.go`,
  `internal/tui/prcomposer.go`, `internal/tui/slack.go`,
  `internal/tui/hookgen.go`, `internal/tui/composer.go`.
- Done when: at 80×24, a failure in any overlay is fully visible.

### UX-36 Rewrite errors as what happened, why, and what to do

Impact: high · Effort: medium

- Today: most errors are a Go error chain printed as is. They are accurate
  and they read like a stack trace.

  | Today | Instead |
  | --- | --- |
  | `issues: no jira.token is configured` | Jira is not set up. Add `jira.token` to `.workflow.json`. |
  | `✗ the forge did not answer` (no token) | No GitHub token found. Run `gh auth login`, or set `$GITHUB_TOKEN`. |
  | `✗ git: exit status 1` | The commit was refused by the pre-commit hook. |
  | `reading the status of /long/path: git: exit status 128: fatal: not a git repository (or any of the parent directories): .git` | Not inside a git repository. |
  | `could not reach the server at https://jira…: context deadline exceeded` | Jira did not answer within 10 seconds. Check the VPN, then press `r`. |
  | `the credential was not accepted: not_in_channel` | Slack refused the post: the bot is not in the channel. |

- Instead: a sentence in the interface's voice first, the raw text under it
  in the faint style for bug reports. The errors already carry sentinels
  (`ErrUnreachable`, `ErrNoToken`, `ErrRejected`); only one of them is ever
  tested for outside its own package, in `internal/cli/doctor.go`.
- Touches: a small mapping beside `Model.failure` in
  `internal/tui/render.go`; the sentinel sets in `internal/jira`,
  `internal/forge`, `internal/slack`.
- Done when: each sentinel has a sentence, and an unknown error still shows
  its raw text.

### UX-37 Show that something is happening

Impact: medium · Effort: small

- Today: loading is a word ("loading…", "looking…", "checking…"). A refresh
  shows nothing at all: the old content stays until the new answer replaces
  it, so `r` on a slow Jira looks like a key that did nothing. Only a running
  command moves. Every request is bounded at ten seconds, and nothing on
  screen says a request is in flight or for how long.
- Instead: put the in-flight glyph in the pane's title while any load for
  that pane is outstanding (`┏━ 1 Issues ◐ ━━`). It costs no timer. The
  Commits detail should also stop saying "nothing changed" while its rail
  still says "loading…" (`commitsDetail`, `internal/tui/commits.go`).
- Touches: `internal/tui/render.go`, each pane's state (`loaded` flags),
  `internal/tui/commits.go`.
- Done when: pressing `r` changes the pane's title until the answer arrives.

### UX-39 A compact progress row that still names its stages

Impact: low · Effort: small

- Today (seen live): below 24 rows the progress row becomes `[●●●○○]`. Which
  stage failed is carried by position and hue alone, which is the one place
  the "shape, not color" rule runs out.
- Instead: `I● B● C● R✗ S○`. Nine more columns, and it reads in monochrome.
  In the collapsed layout, also keep the pane's number in its title
  (`┌─ 3 Commits ─`), since `1`–`5` is how you get anywhere and the rail that
  showed the numbers is gone.
- Touches: `internal/tui/spine.go`, `internal/tui/render.go`.
- Done when: the compact row can be read without color, and a collapsed
  pane's title shows its number.

## The visual system

What is there is a real system, and a good one for a terminal: five hues for
five systems (Jira blue, git yellow, the forge green, Slack magenta), all
taken from the terminal's own palette so the user's theme decides the shades;
shape for state (`○ ◐ ● ✗`); border weight for focus; red for failure and
nothing else. None of that should change. The entries below are places where
the system is not applied, or where two of its channels disagree.

### UX-40 Make red mean broken everywhere, and let it win

Impact: medium · Effort: small

- Today: `failure` is "the one way the interface says something broke", and
  it is used in seven places. About thirteen others draw `✗` and the error in
  the default color: every rail ("✗ failed · see detail", "✗ not a git
  repository", "✗ the forge did not answer"), every overlay, and both CI
  lines. Read from the raw escape codes on a live screen: the rail's failure
  lines carry no color at all. In the progress row, hue belongs to the system,
  so a failed Review stage is drawn as a **green** `✗`. Green with a cross is
  the one pairing where the two channels say opposite things to most readers.
- Instead: `✗` is red wherever it appears, including the progress row, where
  the label keeps the system's hue and the glyph takes the failure color.
  Overlays are handed the styles as well as the glyphs, which is why they
  cannot call `failure` today.
- Touches: `internal/tui/render.go`, `internal/tui/spine.go`,
  `internal/tui/overlay.go` and each overlay's `view`.
- Done when: a test over the raw output finds no `✗` outside a red run.

### UX-41 Stop the red from leaking

Impact: medium · Effort: small

- Today (seen live): an error longer than the pane is styled and then
  wrapped, so the color starts on the first row and its reset lands on the
  last. Everything between is tinted: the pane's right border, the rail's
  borders on the rows beside it, and the next pane's title. On the
  not-a-repository screen the red ran from the error across three rows to the
  words "2 Branch". This is the first thing a new user sees in a terminal
  wide enough to show it.
- Instead: wrap first, then style, so each row closes its own color.
- Touches: `internal/tui/commits.go`, `internal/tui/detail.go`,
  `internal/tui/review.go`, `internal/tui/render.go` (`wrap`).
- Done when: every rendered row that opens a color closes it.

### UX-42 Bring the bottom row into the palette, and up in contrast

Impact: high · Effort: small

- Today: the bottom row is the interface's teaching surface, and it is the
  dimmest thing on the screen. It is drawn by the help component with its
  default styles (`help.New()`, `internal/tui/render.go`), which are fixed
  grays chosen by probing the background: `#626262` for keys and `#4A4A4A`
  for descriptions on a dark terminal. Against black that is a contrast of
  about 3.4:1 and 2.4:1, where 4.5:1 is the usual floor for text. It is also
  the only color on screen that does not come from the user's palette.
- Instead: keys in the default foreground and bold, descriptions in the faint
  style the labels already use. Both inherit the theme.
- Touches: `internal/tui/render.go` (`footer`), `internal/tui/glyphs.go`
  (`styles`).
- Done when: the bottom row uses only the terminal's own foreground, bold and
  faint.

### UX-43 Do not whisper the instructions

Impact: low · Effort: small

- Today (seen live): empty-state lines such as "on no feature branch" and
  "○ nothing posted" are drawn faint, the same as field labels. The lines
  that tell a new user what to do next are the hardest to read.
- Instead: faint is for labels ("base", "upstream", "CI"). A sentence
  addressed to the user is drawn at normal weight.
- Touches: `internal/tui/review.go`, `internal/tui/slack.go`,
  `internal/tui/branch.go`.
- Done when: no full sentence is drawn faint.

### UX-44 Keep a cursor under `NO_COLOR`

Impact: medium · Effort: small

- Today (seen live): `NO_COLOR=1` is honored, by the terminal library and not
  by this code, and it removes every attribute, not only color. The screen
  had no escape sequences at all. That includes the reverse-video block that
  is the text cursor, so every text field has no visible cursor, and bold and
  faint go too, which flattens labels into values. People who set `NO_COLOR`
  are asking for no color. They are not asking for no cursor.
- Instead: under `NO_COLOR`, keep bold, faint and reverse, and drop only
  hues. Add a `ui.color` setting for "never", so the choice does not depend on
  an environment variable alone.
- Touches: `internal/tui/glyphs.go` (`newStyles`), `internal/tui/tui.go`
  (`Run`), `internal/config/config.go`.
- Done when: with `NO_COLOR=1` a text field shows its cursor and no hue is
  drawn.

### UX-46 Give the rail its rows back

Impact: low · Effort: medium

- Today: each of the five rail panes has its own box. At 36 rows, ten rows
  are top and bottom borders, for panes that mostly hold two lines.
- Instead: one box for the rail with a light rule between panes, and the
  heavy weight on the focused pane's two rules. Four rows return to the
  focused pane.

  ```text
  ┌─ 1 Issues ──────────────┐
  │ ◐ PROJ-412 Fix token…   │
  │ ○ PROJ-388 Add retri…   │
  ┢━ 2 Branch ━━━━━━━━━━━━━━┪
  ┃ fix/PROJ-412-fix-token… ┃
  ┃ not pushed yet          ┃
  ┡━ 3 Commits ━━━━━━━━━━━━━┩
  │ 1 of 3 staged           │
  │ 1 commit on this branch │
  ├─ 4 Review ──────────────┤
  ```

  The joining characters need an ASCII form too, which is most of the work.
- Touches: `internal/tui/frame/frame.go`, `internal/tui/render.go`,
  `internal/tui/layout/layout.go`.
- Done when: the rail draws one shared rule between panes in both glyph sets.

## The command line

## Would reopen a settled decision

### UX-49 A second signal for focus

Impact: low · Effort: small

- Reopens: focus is shown by the weight of a border, not by color.
- Why: heavy and light box-drawing differ by one pixel of stroke in many
  fonts, and the difference is the only thing that says where the keys go.
- A version that fits: a bold title on the focused pane. It is weight, not
  color, and it survives `NO_COLOR` once UX-44 is done.
- Done when: the focused pane can be found at a glance in a font whose heavy
  box characters look like its light ones.
