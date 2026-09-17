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
| "`?` lists every key" | `docs/content/docs/usage.md:56` | No. Eleven bindings are missing. |
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

## The visual system

What is there is a real system, and a good one for a terminal: five hues for
five systems (Jira blue, git yellow, the forge green, Slack magenta), all
taken from the terminal's own palette so the user's theme decides the shades;
shape for state (`○ ◐ ● ✗`); border weight for focus; red for failure and
nothing else. None of that should change. The entries below are places where
the system is not applied, or where two of its channels disagree.

## The command line
