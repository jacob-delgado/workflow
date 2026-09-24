# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up UX-61". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `5e69cb8` on 2026-09-22 (PR #125's tip, merged to main). Line numbers drift, so every pointer
also names the symbol it means.

## How this was produced

One pass, from code only, over three surfaces.

1. **A read of every surface.** Every command, flag and message in
   `internal/cli`; every key binding, overlay, empty state, loading state
   and error state in `internal/tui`; and every panel, button, string and
   endpoint call in `web/src` and `internal/webserver`, read against three
   yardsticks — the [clig.dev](https://clig.dev) guidelines for the command
   line, the promises the terminal interface makes about itself (the table
   below, re-verified), and, for the web, the accessibility floor the gates
   already enforce plus a designer's read of whether the page has an
   identity of its own or a template's.
2. **Nothing was run.** Unlike the previous edition, no binary was driven in
   a terminal and no browser was opened. The screens are known from the
   source and from the golden output the screen tests hold. Where that
   matters — a claim about how something *looks* rather than what the code
   does — the entry says "from code".
3. **A second read while writing.** Every cited line was read again as its
   entry was written; a claim that could not be pointed at a line was
   dropped.

## How to read an entry

- **Impact** and **Effort** are estimates. Effort is small (a day or less),
  medium (a few days) or large (a week or more).
- **Today** is what happens now, with the evidence.
- **Instead** is one proposal. There are usually others.
- **Done when** is observable, so a screen test can assert it.

The sections follow the surfaces and, within the terminal, the panes in the
order the work goes. Entries are not ranked. An entry that is also a debt
points at its [TECH_DEBT.md](TECH_DEBT.md) twin; one that is really a new
feature lives in [FEATURES.md](FEATURES.md) and is only pointed at from here.

## The promises the interface makes

The interface states its own rules, in its docs and in its code. They are
good rules. This table is the shortest summary of how far the screen keeps
them, re-counted at this commit.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "a key it does not show does nothing" | No longer stated anywhere; the sentence the previous edition cited at `docs/content/docs/usage.md:55` is gone. Folds into the next row. | — |
| "`?` lists every key" | `docs/content/docs/usage.md:63` | **Yes, by construction, and a test enumerates every placement.** Help is generated from the bindings (`internal/tui/keys.go:138` `helpBuilder.place`, rendered at `internal/tui/render.go:219`): 57 of 57, on 56 lines, since `cycle-type-right` rides `cycle-type-left`'s line (`internal/tui/render.go:233` skips a binding with no help of its own). `TestHelpListsEveryPlacedBinding` (`internal/tui/help_test.go:211`) reads `?` back and holds it to a table of every placement, group by group, and each action moved to a free key must be listed on its own line in its group. |
| "the one way the interface says something broke" | `wording`, `internal/tui/failure.go:63` | **Yes: every site that renders an error's text.** Each tells it through `errorSentence` (`internal/tui/failure.go:100`), which words every sentinel the seams return briefly and in full: 6 panes, the configuration screen, 11 `pinnedOutcome` overlays and the 2 details that repeat a summary row's failure beneath it through `failureBlock` (`internal/tui/failure.go:395`), 12 one-failure rows through `failureLine`, 4 rail and summary rows through `failureSummary`, and 11 failure notices through `noticedFailure`, drawn in the failure style, the merge's and the re-run's led by what failed so a clipped row still names it; a run's headline names the step a failure status stopped, and a pull request opened without every reviewer says why in brief. The 3 guidance notices stay plain, since red means something broke. The width-one glyph-only marks — a failed check, job, stage or review CI — the three rails that point at their detail, and the branch creator's fixed "could not fetch" line carry no error text to word. |
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:64` | **Yes: 18 of 18.** Every outward act waits on a preview or a confirmation; the push, `R`'s re-run of CI and `u`'s rebase share one last look, `lastLook` (`internal/tui/overlay.go:139`). |
| "a refused change must never go unseen" | `internal/tui/picker.go:269` | **Yes: 14 of 14.** Every overlay that sends a request guards it while in flight (the previous edition counted 1 of 7), and every one keeps a refusal in the overlay, where it happened, until `esc` — the merge and finish previews last, through `pinnedOutcome` (`internal/tui/failure.go:413`). |
| "Each pane fails on its own" | `docs/content/docs/usage.md:69` | Yes. `internal/tui/tui.go:157` batches six loads; each pane holds and renders its own error. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:16` | Yes. `unicodeGlyphs` and `asciiGlyphs` differ in shape (`glyphs.go:31`, `:43`); `NO_COLOR` keeps bold and faint (`internal/tui/tui.go:139`). One residue: the progress spine's per-system hue is color-only, mitigated by the name or its initial. |

## The command line

### UX-61 A slow command is silent while it works

Impact: low · Effort: medium

**Today.** No spinner, no elapsed time, no "checking…". `doctor --online`
makes three round trips in silence (`reportCredentials`,
`internal/cli/doctor.go:129`);
`standup` fires up to fifteen forge requests plus a Jira search
(`gatherPulls`, `internal/cli/standup.go:192`); `status DIR…` visits each directory in
series (`statusesOf`, `internal/cli/status.go:165`). The only trace is `--log`,
which outlines each request in a file for a bug report and shows the person
waiting nothing.

**Instead.** A one-line "checking Jira…" on stderr when stderr is a
terminal, replaced in place; nothing when it is not.

**Done when.** A test with a terminal-flagged stderr sees the line; one
without does not.

### UX-62 Flags the scriptable commands are missing

Impact: low · Effort: medium

**Today.** No command declares a single shorthand — there is no `VarP(`
call in `internal/cli` — so `-n`, `-y`, `-j` do not exist; `status` emits
`●◐✗○` (`statusGlyph`, `internal/cli/status.go:404`) with ASCII selectable only through
`ui.ascii` in the file, no `--plain`; `standup` has no `--json`; `pr` has no
draft, base, reviewer, title or body flag; `announce` has no `--channel`
(the channel comes from `messaging.channel` alone); both `pr` and the web
take the first repository template only (`firstTemplate`, `internal/loop/pull.go:136`),
where the interface cycles them (`ctrl+t`).

**Instead.** Shorthands for the three common flags; `--plain` on `status`;
`--json` on `standup`; `--draft`, `--base`, `--reviewer`, `--channel`,
`--template` where the seam already carries the value.

**Done when.** Each flag has a test that it reaches the seam.

## The terminal interface

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint (`internal/tui/tui.go:139`);
`ui.ascii` swaps glyphs and borders (`glyphs.go:43`); escapes in server text
are neutralized. But there is no screen-reader mode; the alternate screen
is unconditional (`internal/tui/render.go:38` `view.AltScreen = true`), so nothing the
interface prints survives quitting; `ui.color` has no `always` for a piped
terminal that does support color; and the 150 ms detail delay
(`internal/tui/detail.go:22`) is fixed.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`; `ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (commit `internal/tui/composer.go:254`, pull request
`internal/tui/prcomposer.go:275`), a dirty tree blocks a switch instead of stashing, and
quit is guarded while an announcement waits. But a posted comment, an applied
transition, a merge and the `branch -D` in finish have no undo, and the
interface never says which acts are reversible.

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries its
URL so it can be edited where it lives.

**Done when.** Each irreversible act's preview or notice says so.

### UX-69 Vim habits stop at `j`/`k`

Impact: low · Effort: small

**Today.** `up/k`, `down/j`, `pgup/K`, `pgdn/J` (`internal/tui/keys.go:204`). No `h`/`l`
(`←`/`→` are cycle-type and cycle-channel only), no `g`/`G` to jump to the
ends of a list or the detail.

**Instead.** `g`/`G` on the lists and the detail; `h`/`l` where a pane has a
horizontal axis.

**Done when.** `G` on the Issues list selects the last loaded issue.

## The web

### UX-86 Nothing marks a change the stream just made

Impact: low · Effort: medium

**Today.** `StreamStatus` shows `Connecting` / `Live` / `Reconnecting`, but
nothing says when the last snapshot arrived, and a panel that changed
because CI settled looks exactly like one that re-rendered. There is no
toast, and no "CI passed" moment on the web where the interface rings the
terminal (`internal/tui/review.go:140`).

**Instead.** A "updated 3 s ago" beside the pill; a brief highlight on the
row a snapshot changed; a status line when CI settles, honoring
`ui.notify`.

**Done when.** A snapshot that flips CI to passed produces a status
region saying so.

### UX-87 Settings can edit seven sections and carry five it cannot show

Impact: low · Effort: medium

**Today.** The form seeds itself with the whole `Config`
(`web/src/features/settings/SettingsPanel.tsx:52`) so `ui`, `timing`, `headers`, `views` and
`branch.prefixes` survive a save unchanged — and cannot be edited. There is
no guided, credential-checking flow like `workflow config init`; the web
edits an existing file only.

**Instead.** Fieldsets for the five, with `views` and `prefixes` as
editable lists; a first-run flow that checks the Jira token as `config
init` does.

**Done when.** A view added in the browser appears in the interface's `v`
cycle.

### UX-88 Ideas the interface has that the browser could borrow

Impact: low · Effort: medium

**Today.** The interface shows a per-file diff under the changes list
(`internal/tui/diff.go:29`), amends (`A`) and fixups (`f`), lists CI checks and jumps to
a failure in `$EDITOR` (`internal/tui/checks.go:41`, `internal/tui/run.go:368`), edits an open pull
request (`preditor.go`), and cycles the repository's pull-request templates
(`ctrl+t`). None has a web equivalent, and the web takes the first template
only (`firstTemplate`, `internal/loop/pull.go:136`). A `?` shortcut sheet, which the interface has,
would give the web's six sections keyboard reach.

**Instead.** In rough order of value: a template select on the pull-request
form; a checks list with links; a diff view; `?`.

**Done when.** Each lands with a role/name test; the template select shows
every repository template.

## The visual system

What is there is a real system, and a good one for a terminal: five hues
for five systems (Jira blue, git yellow, the forge green, chat magenta),
all taken from the terminal's own palette so the user's theme decides the
shades; shape for state (`○ ◐ ● ✗`); border weight for focus; red for
failure and nothing else. None of that should change. This edition found
no place in the terminal where the system is not applied.

The web now speaks it too. The four systems' hues are tokens in both
themes (`web/src/index.css:62`), in the terminal's hue families but held at
least 30° of OKLCH hue from the status lights, the periwinkle control
accent and each other, and at 4.5:1 as text, by `web/src/tokens.test.ts`;
they mark whose a thing is — the active rail icon, each section's heading,
the work story's stages and an issue's local branch — and never how it
stands. Every state is drawn by its shape through one `StateMark`
(`web/src/shell/StateMark.tsx:33`), hidden from assistive tech beside its
words. Type, space and corners each have one scale
(`web/src/index.css:162`), headings are set in sentence case — the web lint
refuses an `uppercase` class — and monospace is for code alone. Periwinkle
stays the one control accent, and the middle dot the separator.

It follows the window, too. Below `md` the rail keeps its icons alone, each
name kept for assistive tech and shown on hover; below `lg` the issue list
sits over its detail, and at every width it scrolls in its own pane, over a
line saying how many it holds. The header holds still while the content
scrolls beneath it, and a word wider than the content breaks rather than
scroll it sideways. `web/e2e/layout.spec.ts` holds every section to 640,
1024 and 1440 px in both themes: nothing scrolls sideways, nor the page
down, Tab reaches every control, each in view as it takes focus, and axe
finds nothing.

## Across the surfaces

### UX-89 Worktrees and a fresh base on the command line and the web

Impact: low · Effort: medium

**Today.** The interface's branch creator fetches `origin` first and offers
to branch from what you have when the fetch fails (`internal/tui/branch.go:464`), and
`ctrl+w` creates the branch in a worktree beside the repository
(`internal/tui/branch.go:404`). `workflow branch` and `POST /api/branches` do neither:
no fetch, no worktree.

**Instead.** `--worktree` and `--fetch` on `branch`; a worktree toggle on
the web's start-work flow; both through the shared composition layer,
`internal/loop`.

**Done when.** `workflow branch KEY --worktree` creates a directory beside
the repository and says where.

## Ideas that would reopen a settled decision

None this edition. One note for the record: `FEATURES.md:52` states the
settled decision as "Five panes down the left"; the code has six
(`internal/tui/panes.go:27`), and has since the Reviews pane landed. That is
stale text to correct (TECH_DEBT.md DEBT-69), not a decision to reopen —
the six-pane rail *is* the decision as built.
