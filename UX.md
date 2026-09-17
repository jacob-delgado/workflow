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
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:43` | Mostly. |
| "a refused change must never go unseen" | `internal/tui/picker.go:183` | In one overlay of seven. UX-35 |
| "Each pane fails on its own" | `docs/content/docs/usage.md:58` | Yes, and it is the best thing about the first run. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:14` | Yes. It reads in monochrome. |

## The first run

### UX-01 At the default terminal size, "see detail" points at nothing

Impact: high · Effort: small

- Today: below 90 columns the rail and the detail pane take turns, and the
  Issues pane draws only its list (`internal/tui/render.go:95`,
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

## Issues

## Branch

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

## Review

## Slack

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
