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
| "a key it does not show does nothing" | `docs/content/docs/usage.md:55` | No. Ten keys work unseen. |
| "`?` lists every key" | `docs/content/docs/usage.md:56` | No. Eleven bindings are missing. UX-33 |
| "the one way the interface says something broke" | `failure`, `internal/tui/render.go:277` | In 7 places of about 20. |
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:43` | Mostly. |
| "a refused change must never go unseen" | `internal/tui/picker.go:184` | In one overlay of seven. |
| "Each pane fails on its own" | `docs/content/docs/usage.md:58` | Yes, and it is the best thing about the first run. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:14` | Yes. It reads in monochrome. |

## The first run

## Issues

## Branch

## Commits

## Review

## Slack

## Across the interface

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

## The visual system

What is there is a real system, and a good one for a terminal: five hues for
five systems (Jira blue, git yellow, the forge green, Slack magenta), all
taken from the terminal's own palette so the user's theme decides the shades;
shape for state (`○ ◐ ● ✗`); border weight for focus; red for failure and
nothing else. None of that should change. The entries below are places where
the system is not applied, or where two of its channels disagree.

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
