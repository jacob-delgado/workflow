# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up" an entry by its ID. Each entry
says what happens today, what could happen instead, where the change would
land, and how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `38cfba25` on 2026-10-14, main after the UX
refresh: every pointer was found again there and its line moved to where
its symbol now stands, the promises' counts were taken again by the recipe
under their table, and what stays open across the surfaces was listed
afresh. The entries and the rest of the prose were last audited in full at
`e5156e3` on 2026-10-06, the `claude/wizardly-babbage-lptyxi-ux2` branch
where the Summary's post on every surface, the web's answer to the
terminal's keys and both interfaces' first-run setup shipped; the commits
since then edited the sections they changed as they landed, the refresh's
among them (the track, the visual system, the dialogs, the directory
picker and Settings). An entry they closed is gone, and one they narrowed
says only what is left. Line numbers drift, so every pointer also names the
symbol it means.

## How this was produced

The first edition was one pass over every surface and every package, from
the source and, for the web, from the screen, at `f05ae9f`. This edition
re-read it after nine milestones changed the screens it describes.

1. **Three surface audits.** Every command and flag in `internal/cli`, every
   pane, key and overlay in `internal/tui`, and every section, control and
   operation in `web/src`, `internal/webserver` and `api/openapi.yaml` were
   inventoried, held against [clig.dev](https://clig.dev), the principles
   below and the promises the terminal makes, and every open entry was
   marked open, partly fixed or shipped from the code.
2. **The screens were looked at.** The web's populated mock ran under
   Playwright (`web/e2e/screens.spec.ts`), every section in both themes at
   640, 1024 and 1440 px; the terminal ran with `--dry-run` in tmux at
   80x24, where its narrowest layout shows. A claim about how a screen
   looks names what it was read from.
3. **Every finding refuted before it was written.** Each count and pointer
   was read again as its entry was written, an independent reader tried to
   refute each new entry, and a claim that could not be pointed at a line
   was dropped.

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

## Principles

The rules every entry is judged by. Each is stated once here; an entry that
breaks one names it.

1. **One name per action, kept through its flow.** The control, its busy
   label and its done notice share a verb: Switch, Switching…, Switched to
   `~/src/api`. Each surface keeps one notice style: the terminal's notices
   are lowercase fragments with no period, the web's are sentences, and the
   command line's are sentences on stderr.
2. **Confirm what leaves the machine or cannot be undone.** A push, a post,
   a merge, a Taskwarrior sync, marking a task done, undoing in
   Taskwarrior, forgetting a person and switching the directory each wait
   on a last look. A reversible local toggle acts at once: stage and
   unstage, start and stop a task, add and remove a favorite, tag someone
   in an announcement, link an owner to a person.
3. **One key per verb, on every pane.** The same verb has the same key
   wherever it appears (`/` search, `f` filter, `O` sort, `r` refresh, `o`
   open, `y` copy), and no binding takes a key a text field uses to edit
   while that field has the focus.
4. **A failure is never drawn as empty or as rest.** The terminal says it in
   the failure voice (`errorSentence`, `internal/tui/errorwords.go:67`); the
   web says it in a `role="alert"` line beside what failed. A service that
   is not set up is not a failure: every surface says what is missing and
   how to set it up, beside the not-started mark, as guidance rather than
   an alarm (`loop.NotSetUp` decides which is which, once, and
   `loop.SetUpAdvice` words it, `internal/loop/setup.go:21`, `:32`).
5. **Shape carries state, not color.** A glyph's shape, a word or a weight
   tells the state; a hue only repeats it.
6. **The terminal and the web are equals.** Each can do what the other
   can, in its own idiom. The command line does what is useful once or in a
   script, with `--json`, and leaves browsing to the other two.
7. **The forge's own nouns.** A merge request on GitLab, `!` before its
   number; a pull request and `#` on GitHub; never one assumed for both.

## Vocabulary

One name per concept, so the three surfaces read as one tool. All three
use these words; a new control or notice takes its word from here, and a
concept that needs a new word adds a row.

| Concept | Name |
| --- | --- |
| Making a branch for an issue | **Start work** (the command stays `branch`; its prompt and help say "start work") |
| The same, in a new worktree | **Start work in a new worktree** |
| Checking out a git branch | **Switch branch** … **Switching…** … **Switched to NAME** |
| Changing the directory workflow works in | **Switch** … **Switched to** |
| The review queue | **Reviews**, headed "Waiting on your review" |
| The ready or merged message | **Announce** … **Announced to** |
| The Summary, sent to the team | **Post** … **Posted to** |
| Narrowing by typing | **Search** (`/` in the terminal) |
| Narrowing by a checklist | **Filter** (`f` in the terminal) |
| Ordering | **Sort** (`O` in the terminal) |
| Finishing a Taskwarrior task | **Mark done** … **Marked done** |
| The local stores | **Local data**, **Remove** … **Removed** |
| Reading again after a failure | **Try again** |
| Reading | **Reading …** |
| `esc` | **discard** when it drops work, **close** when it keeps it, **cancel** for an unsent form, **back** for a step inside a flow, **stay** at a guard, **skip** for an offer after a done act (the constants beside `everyPaneKeys`, `internal/tui/keys.go`) |

## What each surface can do

One row per capability, in the order the work goes. A cell names the
command, key or control; "—" names the entry or feature that would fill
it. "Not for scripts" marks a deliberate gap: the command line leaves
browsing, a choice a person makes as they go, and what git, lefthook, the
forge's own CLI or Taskwarrior already do on their own command line, to
them.

| Capability | Command line | Terminal | Web |
| --- | --- | --- | --- |
| **Issue** | | | |
| List, search and filter issues | `issues [--view NAME] [--json]`; search and filter not for scripts | Issues pane: `/`, `f`, `v`, `ctrl+n` | Issues: View, Search, Filter, Load more |
| Read an issue and its comments | `issue KEY [--json]` | the detail | the detail |
| Comment | `comment KEY`, the text on stdin | `c`, a composer with vim modes | the comment composer, Markdown |
| Change its status | `transition KEY STATUS`, a move asking no fields; `pr` offers `jira.review_status` | `t` | Change status, with the transition's fields |
| Assign; log work | `assign KEY USERNAME`; `worklog KEY DURATION` | `a`; `w` | Assign; Log work |
| Track it in Taskwarrior | not for scripts | `T` | Track in Taskwarrior |
| **Branch** | | | |
| Start work | `branch KEY [--fetch]` | `b` | Start work |
| Start work in a new worktree | `branch KEY --worktree` | `b`, then `ctrl+g` | Start work in a new worktree |
| Switch branch | not for scripts | `s` | Switch branch |
| Link an issue to the branch | not for scripts | `i` | Link an issue |
| Unlink it | not for scripts | `i`, then `u` | Unlink |
| Push | inside `pr` | `P` | Push branch |
| Rebase onto the base | not for scripts | `u` | Rebase onto BASE |
| **Commits** | | | |
| Stage, unstage, stage all, unstage all | not for scripts | `space`, `a`, `U` | Stage, Unstage, Stage all, Unstage all |
| Discard a file's changes | not for scripts | `x`, after a last look | Discard…, after a confirm |
| Read a file's diff | not for scripts | the selected file's diff | Show diff |
| Commit | not for scripts | `c` | Commit staged changes |
| Amend; fix up | not for scripts | `A`; `f` | Amend last commit; Fix up a commit |
| Run a hook; set up lefthook | not for scripts | `h`; `g` | Run a hook; Set up lefthook |
| **Review** | | | |
| Open a pull request | `pr [--title --body --draft --base --reviewer --announce --json]` | `n` | Open a pull request |
| Link it on the issue, move the issue | `pr` asks both | offered after `n` | Link it on KEY, Move KEY to STATUS |
| Choose its template | `pr --template NAME` | `ctrl+t` in the composer | Template |
| Edit it | not for scripts | `e` | Edit pull request |
| Read the checks and a job's log | `status` shows CI | `c`, then `l` | the CI checks, Show log |
| Re-run CI; merge; finish | not for scripts; `finish` | `R`; `M`; `F` | Re-run failed checks; Merge; Finish the branch |
| Where the work stands | `status [DIR…] [--json or --plain]` | the top row | the work story, the header |
| **Messaging** | | | |
| Announce | `announce [--channel]` | `p` | Announce to SERVICE |
| Announce when CI passes | `announce --when-green`, `pr --announce` | `w` in the preview; armed as `n` opens one | Announce when CI passes |
| Stop waiting for CI | `ctrl+c` while it waits | `x` on the messaging pane | Stop waiting |
| Edit the announcement first | not for scripts | `e` in the preview | Edit, in the preview |
| Know it was announced | `announce` says so, and asks again | "● announced" on the Messaging rail | the section says so |
| People and groups | not for scripts | `P` | Settings: People, Groups |
| Post the summary | `summary --post [--yes --channel]` | Summary pane: `p`, after a preview | Summary: Post…, after a preview |
| **Reviews** | | | |
| List what waits on your review | `reviews --json --sort` | Reviews pane: `O`, `f` | Reviews: Sort, Filter |
| **Tasks** | | | |
| Add, annotate, modify, start, stop, mark done, undo, sync | not for scripts | Tasks pane: `a`, `A`, `e`, `s`, `d`, `u`, `S` | Tasks: Add, Annotate, Modify, Start, Stop, Mark done…, Undo…, Sync… |
| Sort, search, filter | not for scripts | `O`, `/`, `f` | Sort, Search, Filter |
| **Summary** | | | |
| Read what you did in a period | `summary [--from --to] [--json]` | Summary pane: `[`, `]`, `t`, `c` | Summary: Earlier, Later, Today, the calendar |
| Copy it as Markdown | `summary` (to stdout) | `Y` | Copy as Markdown |
| **Repositories** | | | |
| Where workflow works, and what configures it | `doctor`, `config show` | Repositories pane, the top row's place | Repositories, the header's place |
| Switch the directory | not for scripts | `enter`, `g` | Switch; Go to a directory… (`g`), then Switch to … |
| Favorites | listed by `repositories` | `f` | Add to favorites, Remove |
| List the worktrees | `repositories [--json]` | the Repositories pane | Worktrees |
| **Settings and local data** | | | |
| Set up from nothing | `config init`, `slack login` | `enter` on the no-file screen, or `S` on the Repositories pane | Settings, with no file |
| Read the configuration | `config show` | Settings: `S` on the Repositories pane | Settings |
| Remove a stored credential or header | edit the file | Settings: `D`, after a last look | Settings: Remove…, after a confirm |
| Change it, every section and list | edit the file | Settings, `ctrl+s` | Settings, Save changes |
| Check the setup | `doctor [--online]` | a failure names `workflow doctor` | a failure names `workflow doctor` |
| Remove the local data | `db-clean` | Local data: `L` on the Repositories pane | Settings: Local data, Remove |
| Every key or command | `--help` | `?` | `?`, and ⌘K or Ctrl+K (outside a Mac's text field) to find an action by name |
| Hold back every write | `--dry-run` (`-n`) | `--dry-run` | `--web --dry-run` |

## The promises the interface makes

The terminal states its own rules, in its docs and in its code. This table
is the shortest summary of how far the screen keeps them, re-counted at
`38cfba25`.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "`?` lists every key" | `docs/content/docs/usage.md:109` | **Yes, by construction, and a test enumerates every placement.** Help is generated from the bindings (`helpBuilder.place`, `internal/tui/keys.go:174`, rendered in two columns split where they balance, `balancedSplit`, `internal/tui/help.go:166`): 104 placements — 94 `builder.bind` and 10 `builder.bindShown` calls — in 15 groups (`groupMoving` to `groupPanesAndOverlays`, `internal/tui/keys.go:108`), on 103 lines, since `cycle-type-right` has no help of its own and rides `cycle-type-left`'s line (`internal/tui/keys.go:271`). `TestHelpListsEveryPlacedBinding` (`internal/tui/help_test.go:309`) reads `?` back and holds it to a table of every placement. |
| "the one way the interface says something broke" | `wording`, `internal/tui/errorwords.go:28` | **Yes: every site that renders an error's text.** Each is told through `errorSentence` (`internal/tui/errorwords.go:67`) and drawn by one of nine helpers — those that draw a mark choose it, and its style, through `voice` (`internal/tui/failure.go:79`), the footer's two failure notices through `markOf` (`:67`), which asks the same `loop.NotSetUp`: `failureBlock` at 22 sites, `pinnedOutcome` 18, `pinnedProblem` 2, `failureLine` 19, `failureSummary` 9, `unreadRow` 3, `noticedFailure` 15, `noticedFailureLedBy` 3 and `noticedGuidance` 7 (plain, since red means something broke). They are also the one way the terminal says something was never set up: as guidance, with `○` and no red, in `loop.SetUpAdvice`'s words (`setUp`, `internal/tui/errorwords.go:129`), as principle 4 asks. |
| "Nothing outward facing is sent without" a last look | `commentPreview`, `internal/tui/comment.go:53` | **Yes, under principle 2.** Every write that leaves the machine or cannot be taken back waits on a preview or a confirmation, most through `lastLook` (`internal/tui/overlay.go:220`): every write to Jira, the forge and the messaging service, every push, Taskwarrior's sync, mark done and undo, forgetting a person and every directory switch. The reversible local toggles principle 2 names act at once. |
| "a refused change must never go unseen" | `statusPicker`, `internal/tui/picker.go:328` | **Yes: 19 of 19.** Every overlay that sends a request refuses every key while it is in flight and keeps a refusal where it happened until `esc`: `branchCreator`, `branchLinker`, `branchPicker`, `commentPreview`, `finishPreview`, `hookgenOffer`, `issueLinker`, `issueWrite`, `lastLook`, `mergePicker`, `messagingPreview`, `peopleOverlay`, `prComposer`, `prEditor`, `settingsForm` (each refused setting marked under its own row until that setting is edited or a save is sent, and named below a form open over the rows while that form hides them), `setupForm` (`internal/tui/setupform.go:264`, while it writes or checks), `statusPicker` (with its field form), `summaryPost` and `taskLine`. |
| "Each pane fails on its own" | `docs/content/docs/usage.md:117` | **Yes.** `Init` (`internal/tui/tui.go:214`) batches eight loads, and the Summary and Repositories panes read on first focus (Init reads Repositories too when it starts focused there, `internal/tui/tui.go:224`); each pane holds and renders its own load's error, the Summary per source, and says of a service never set up how to set it up. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:19` | **Yes.** `unicodeGlyphs` and `asciiGlyphs` differ in shape (`internal/tui/glyphs.go:48`, `:65`); `NO_COLOR` keeps bold and faint. Two residues: the progress spine's five system hues are color-only, mitigated by each system's name or initial, and `◐` means both "partly staged" and "announces when CI passes" (see the visual system). |

To re-count rather than trust these numbers: `grep -o '\bfailureBlock('
internal/tui/*.go`, tests excluded, and the same for each helper, less
what `failure.go` itself holds — each helper's definition, the `Model`
wrappers and the calls one helper makes into another there
(`pinnedOutcome`'s and `pinnedProblem`'s `failureBlock`,
`noticedFailure`'s `noticedFailureLedBy`); `grep -n 'send\.sending\|sending\.sending'
internal/tui/*.go` for the overlays that guard a send, keeping the types
whose key handler returns on it; and the `builder.bind(` and
`builder.bindShown(` calls in `internal/tui/keys.go` and
`internal/tui/panekeys.go` for the help.

## The command line

Nothing is open here at the moment.

## The terminal interface

What is open here is a screen-reader mode, the alternate screen and a fixed
delay, and undo.

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint
(`UI.DrawColor`, `internal/config/ui.go:113`; `Model.WithoutColor`,
`internal/tui/options.go:54`); `ui.ascii` swaps glyphs and borders
(`asciiGlyphs`, `internal/tui/glyphs.go:65`); escapes in server text are
neutralized. But there is no screen-reader mode; the alternate screen is
unconditional (`Model.View`, `internal/tui/render.go:38`, `view.AltScreen
= true`), so nothing the interface prints survives quitting; `ui.color`
has no `always` for a piped terminal that does support color; and the
150 ms detail delay (`detailDelay`, `internal/tui/detail.go:20`) is fixed.
A terminal at 80 by 24 gets the full-screen layout at its tightest,
and nothing offers a plain mode that prints a line at a time
instead; the command line is the only other way in.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`, which `Config.validateUI` (`internal/config/ui.go:134`) and the
`UIConfig.color` enum (`api/openapi.yaml:5461`) refuse until they list it;
`ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (the commit, `commitComposer.handleKey`,
`internal/tui/composer.go:241`; the pull request, `prComposer.handleKey`,
`internal/tui/prcomposer.go:294`), a dirty tree blocks a switch instead of
stashing (`loop.ErrDirtyTree`, `internal/loop/guards.go:18`), and quit is
guarded while an announcement waits (`quitGuard`,
`internal/tui/messagingheld.go`). But a posted comment, an applied
transition, a merge and the `git branch -D` in finish
(`finishPreview.commands`, `internal/tui/finish.go:94`) have no undo, and
the interface never says which acts are reversible: the finish preview
lists the three commands and says nothing of what `-D` costs
(`finishPreview.view`, `internal/tui/finish.go:100`), and a comment's notice
is "commented on KEY" (`commentPosted.apply`,
`internal/tui/comment.go:140`).

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries the
issue's link so the comment can be edited where it lives. Undo itself,
over a log of what workflow did, is FEAT-87's.

**Done when.** Each irreversible act's preview or notice says so.

## The web

Nothing is open here at the moment.

## The visual system

What is there is a real system, and a good one for a terminal: five
systems' hues — Jira blue, git yellow, the forge green, chat magenta and
Taskwarrior cyan (`newStyles`, `internal/tui/glyphs.go:148`; Taskwarrior's
marks the active task at the spine's end, `internal/tui/spine.go:276`) —
and red for failure alone, all taken from the terminal's own palette as
ANSI indices (`internal/tui/glyphs.go:134`) so the user's theme decides
the shades; shape for state (`○ ◐ ● ✗`, or `o * # x` in ASCII,
`State.Glyph`, `internal/progress/progress.go:33`); border weight for focus. None of that
should change.

The shapes now mark more than a status, and stay on one axis while they
do: `○` not begun, `◐` under way or partway, `●` done, `✗` broke. `◐`
means both "partly staged" on the Commits pane (`stageGlyph`,
`internal/tui/commits.go:174`) and "announces when CI passes" on the
Messaging pane (`messagingState.standing`, beside `queuedPost.waitingOn` in
`internal/tui/messagingheld.go`);
that is not a conflict, since each is the halfway point of its own
progression and words stand beside it. The rule a new use must keep: a
state glyph says how far something has got, never anything else. A
checkbox therefore has its own shapes, `[x]` and `[ ]` in both glyph
sets (`checkbox`, `internal/tui/glyphs.go`). Three marks are not states
and take shapes of their own: `★` (`^` in ASCII) for a favorite
directory, `‹›` (`<>` in ASCII) for the value under a cursor — the
calendar's and the commit type's (`internal/tui/calendar.go:115`,
`internal/tui/composer.go:203`), its closing `›` also standing between a
section and a setting in it at Settings' foot, as Jira › Project
(`setting.named`, `internal/tui/settingsrefusals.go`) — and `[]` for the
value another calendar column stands on (`internal/tui/calendar.go:117`).
The diff is the one documented exception to the hue rule: an added line
is drawn in the forge's green and a removed one in red (`markDiffLine`,
`internal/tui/diff.go:111`, `:113`; `styles`, `internal/tui/glyphs.go:121`),
where the `+` and `-` git leaves in place carry the meaning by shape
(`internal/tui/diff.go:70`). Border
weight is the one rule the screen loses at its commonest size: at 80 by
24 the detail draws borderless, and the rail's heavy rules around the
focused pane carry the focus instead.

The top row is the one bold element, and draws the loop as a track
(`spineView.draw`, `internal/tui/spine.go`): each stage's glyph and name in
its system's hue, then a rule filled `━` in that hue once the stage is done,
open `─` and faint while it is to come, and after Review filled by CI's
share of checks finished (TRADE-45). Stages still to come are faint, so a
stage's identity rests on its name (TRADE-5). The rules share the room
left, up to eight columns each; the place gives way first, then the active
task's description, then the stage names for initials, which below 60
columns or 24 rows the row uses from the start, and ASCII draws them `=`
and `-`. Motion is deliberate and rare (`internal/tui/motion.go`): the
in-flight mark of a read turns `◐◓◑◒` (`|/-\` in ASCII, `progress.Turn`)
while the read is out, a stage's rule fills on a critically damped spring
when the stage completes, and the Review pane's bar of checks springs
toward the share finished — each on frames through `Deps.After`, only
while something moves, never while the first screen fills in, and never
under `ui.motion: reduce`, which holds the mark on its still glyph, `◐` or
`*`.

The web now speaks it too. The five systems' hues are tokens in both
themes (`web/src/index.css:113`), held at least 30° of OKLCH hue from the
status lights, the periwinkle control accent and each other, and at 4.5:1
as text, by `web/src/tokens.test.ts`; that separation moves two off the
terminal's families — the forge is teal, clear of the success light's
green, and Taskwarrior violet. They mark whose a thing is — the active
rail icon, each section's heading, the work story's stages, the loop's
track under the header (`web/src/shell/LoopTrack.tsx`) and an issue's
local branch — and never how it stands. The track is the terminal's spine
drawn for the page: five stages, each a mark and a name over a rule on the
`--track` token that fills in its system's hue as the stage gets done, the
Review rule by CI's share of checks; the fill is a transform under
`motion-safe`, the page's one moving part. Every state is drawn by its shape
through one `StateMark` (`web/src/lib/StateMark.tsx`), hidden from
assistive tech beside its words. Type, space and corners each have one
scale (`web/src/index.css`), headings are set in sentence case — the
web lint refuses an `uppercase` class (`web/eslint.config.js:41`) — and
the face is Recursive, latin alone (TRADE-48): proportional, `MONO` 0, for
prose, and `MONO` 1, every letter one width, for code alone. The dialogs —
the command palette and the `?` sheet — are Base UI's Dialog and
Autocomplete, drawn through `web/src/lib/baseui`, over a backdrop of the
page's own ground; Settings' sections are its Tabs, and a choice narrowed by
typing — whom a code owner is on Slack — its Combobox. A field a form refuses
is marked invalid, its edge in the failure color, and described by why, from
`Field.tsx`'s one `error`. Periwinkle stays the one control accent, and
the middle dot the separator. Each part of that is drawn in one place in
`web/src/lib`: a field (`Field.tsx`) and a button (`Button.tsx`) in one
padding and size scale, each held with `aria-disabled` while its request
runs so it keeps the focus, a row's facts apart, with the dot between them
hidden from assistive tech (`Meta.tsx`), a date in one locale
(`dates.ts`), a link that opens a new tab, saying so (`NewTabLink.tsx`),
and a read in flight or a failure (`Status.tsx`) as a
`role="status"` line beginning "Reading" or a `role="alert"` line in the
failure color.

It follows the window, too. Below `md` the rail keeps its icons alone, each
name kept for assistive tech and shown on hover; below `lg` the issue list
sits over its detail, and at every width it scrolls in its own pane, over a
line saying how many it holds. The header holds still while the content
scrolls beneath it, and a word wider than the content breaks rather than
scroll it sideways. `web/e2e/layout.spec.ts` holds every section to 640,
1024 and 1440 px in both themes (`web/e2e/layout.spec.ts:22`): nothing
scrolls sideways, nor the page down, Tab reaches every control, each in
view as it takes focus, and axe finds nothing. It holds the steps a click
opens to the same widths — the pull request form, the push and discard
confirmations, the announcement and Summary previews, the forget and
remove confirmations, a refused write and the first run's setup.

## Across the surfaces

No entry is open here. What each surface can do is the table at the head
of this file, and the names the three surfaces share are the Vocabulary
table's beside it; a sentence one surface words differently from another,
or that assumes GitHub or the terminal, is an entry here.

Three differences between the terminal and the web stay open by decision,
each recorded where it is kept:

- **A dry run.** Under `--dry-run` the terminal narrates each write it
  holds back; the web refuses every write before it is sent and cannot
  show what one would have done (TRADE-3).
- **A Jira setting saved.** The terminal's Settings reopens the interface
  with the file saved when no announcement or task change is in flight and
  nothing would be lost; otherwise it asks first, and staying keeps the
  save for when workflow next opens. With no directory to reopen in, it
  says the save waits for the next start. On the web a change to Jira's
  address, token, views or `issues.forge` applies once `workflow --web`
  starts again or switches directory (TRADE-24).
- **A task change at the loop's moments.** The terminal offers to start
  the issue's task when its branch is made or checked out, note the pull
  request on it, and complete it on a merge or a move to done; the web
  leaves each to the Tasks card's and the Tasks section's buttons, pressed
  when you choose (web.md's "What stays in the terminal").

The rule the last looks follow was chosen by the maintainer: **confirm
what leaves the machine or cannot be undone; a reversible local toggle
acts at once.** The parity rule beside it: the terminal and the web cover
the loop, each for browsing; the command line covers one-shot and scripted
work, with `--json`, and is not a browser.

## New ideas

An idea this edition adds that no surface has begun: a mode a screen
reader can follow. One more is a feature
rather than a change to how the interface is used, so it lives in
[FEATURES.md](FEATURES.md) and is only pointed at here:

- **FEAT-87 What workflow did, and taking it back** — a session's log of
  every write, with undo where the system allows it; the feature UX-68
  stops short of.

### UX-154 A screen-reader and plain mode

Impact: low · Effort: large

**Today.** UX-64 asks for the alternate screen to be optional; a screen
reader needs more than that. The interface redraws one full screen of
boxes each frame (`view.AltScreen = true`, `internal/tui/render.go:38`),
so a reader re-reads the rail, the borders and the footer at every
change, and the one line that says what just happened — the notice
(`Model.noticed`, `internal/tui/overlay.go:172`) — sits among them.
Forms are the one part already read well: with `ui.accessible` or
`ACCESSIBLE`, a status change's fields are asked as Huh's plain prompts,
one question at a time (`promptedForm`, `internal/tui/huhform.go`), but
the screen around them is unchanged.

**Instead.** `ui.screen: lines` (and `--plain` for one run): no
alternate screen, no rail, no boxes; the interface prints, and each
change is one appended line.

- **What prints.** On start, where you work and the progress row in
  words ("Issue done, Branch done, Commits in flight, Review not
  started"); then a line per notice, failure and settled read ("CI
  passed on #42", "3 review requests waiting"), each once, printed above
  the input line as Bubble Tea's print command does outside the
  alternate screen.
- **Reading a pane.** The digit keys still choose a pane, and enter
  prints its detail as plain text, glyphs spelled out ("in flight", not
  `◐`), the way `ui.ascii` swaps them now.
- **Overlays as questions.** A picker prints its choices numbered and
  takes a number; a preview prints what it will send, then "Send? enter
  sends, esc cancels" — the command line's `writePrompt` shape.
- **Keys unchanged.** The same bindings and `ui.keys`, and `?` prints the
  help, one key to a line.

**Done when.** A screen test in lines mode sees no box-drawing character
in any frame, and after a fake CI turns green finds one new line "CI
passed on #42" and no redrawn rail; a test in lines mode presses `t`,
`2`, enter and finds the issue moved, with each step's question on a line
of its own.

## Ideas that would reopen a settled decision

None this edition. The nine-pane rail (`paneCount`,
`internal/tui/panes.go:38`), once written up as five in FEATURES.md, six
before the Tasks pane joined it, seven before the Summary and eight
before Repositories, *is* the decision as built. UX-154's lines mode
draws the same nine panes one at a time, on request, rather than
reopening the rail; web.md's "What stays in the terminal" lists
choices for now, not settled decisions.
