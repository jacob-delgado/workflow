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

### UX-76 `commit.default_scope` can be edited in Settings and is never used

Impact: medium · Effort: small

**Today.** The interface's composer opens on the learned scope, else
`commit.default_scope` (`internal/tui/composer.go:131` `startingScope`). The web's
`CommitForm` hardcodes `scope: ''` (`web/src/features/branch/CommitForm.tsx:47`) — while
`commit.default_scope` is an editable field in the Settings form
(`SettingsPanel.tsx:184`). A setting the user can change with no visible
effect.

**Instead.** The snapshot's `changes` carries `suggested_scope` — the store's
last scope, else the configured default (the interface's own rule) — and
the form opens on it; the server records the scope after a commit, as the
interface does.

**Done when.** Editing `default_scope` in Settings pre-fills the next commit
form; a table test covers the store-then-config fallback.

### UX-77 Four of twelve writes succeed in silence

Impact: high · Effort: small

**Today.** `Pull request opened.` (`web/src/features/review/OpenedOutcome.tsx:21`), `Announced…`
(`web/src/features/messaging/MessagingPanel.tsx:139`) and `Saved.` (`SettingsPanel.tsx:323`) confirm,
and so do the link and the move offered after opening, each in a
`role="status"` line (`FollowUpOffer`, `web/src/features/review/OpenedOutcome.tsx:70`), and a
file's stage or unstage and Stage all, each in its own (`ChangeRow`,
`web/src/features/branch/WorkingTree.tsx:66`; `StageAll`, `:106`).
Commit (`web/src/features/branch/CommitForm.tsx:72`), push (`web/src/features/branch/BranchPanel.tsx:100`), check-out and
start-work say nothing: `useAsyncAction` ends in `done`
(`web/src/lib/useAsyncAction.ts:21`), but neither button reads it; the stated
rationale is that the snapshot is the confirmation (`web/src/lib/useAsyncAction.ts:9`),
but the stream re-pushes on a 5 s tick (`stream.go:20`), so a commit is
silent for up to five seconds. And two of the successes that do exist —
`Pull request opened.` and `Announced…` — are plain `<p>` elements, not live
regions: announced to nobody using a screen reader.

**Instead.** Every write ends in a `role="status"` line that keeps the
button's verb ("Pushed NAME", "Committed abc123 subject"); the five
hand-rolled copies adopt `useAsyncAction` and its `done` state (DEBT-63).

**Done when.** A table test over every write finds a status region for
each.

### UX-78 Focus is dropped after every action

Impact: high · Effort: small

**Today.** The only `.focus()` calls under `web/src` are the issue list's
Load more, which hands focus to the first issue a page adds
(`web/src/features/issues/IssuesPanel.tsx:145`), and the offers after
opening and the staging buttons, which hand it to what they said
(`web/src/features/review/OpenedOutcome.tsx:77`,
`web/src/features/branch/WorkingTree.tsx:145`). On success
`OpenPullRequest` unmounts the form for the outcome the panel shows above it
(`web/src/features/review/ReviewPanel.tsx:174`); `AnnounceControls` swaps its
form for a `<p>` (`web/src/features/messaging/MessagingPanel.tsx:137`);
`PushButton` swaps its idle and confirming states (`web/src/features/branch/BranchPanel.tsx:109`).
The focused button disappears and focus falls to `<body>`. Changing section
from the nav rail or a work-story row never moves focus to `<main>`, though
`tabIndex={-1}` is there for it (`web/src/shell/AppShell.tsx:68`).

**Instead.** Focus the outcome when a form closes; focus `<main>` on a
section change.

**Done when.** After a faked open succeeds, `document.activeElement` is the
outcome.

### UX-79 The fallback says what could not happen; the server sometimes says nothing

Impact: medium · Effort: small

**Today.** Server reasons are good where they exist (`the working tree has
uncommitted changes; commit or stash them before switching`, `internal/webserver/checkout.go:19`;
`nothing is staged to commit`, `internal/webserver/commit.go:21`). But the client fallbacks
are vague and near-apologetic — `The branch could not be checked out.`
(`web/src/features/issues/IssuesPanel.tsx:337`), `Work could not be started.` (`web/src/features/issues/WorkStory.tsx:267`),
`The push failed.` (`web/src/features/branch/BranchPanel.tsx:102`), `The commit could not be
created.` (`web/src/features/branch/CommitForm.tsx:76`) — and three handlers replace the tool's
reason with one of those sentences: `internal/webserver/checkout.go:44`, `internal/webserver/branchcreate.go:44`,
`internal/webserver/announce.go:53`. `writeResponseError` answers `something went wrong`
(`internal/webserver/errors.go:80`). A user gets a dead end with no next step.

**Instead.** `checkout` and `branchcreate` pass git's own reason through
the classified problem mapping; `announce` classifies by the messaging
sentinels and never forwards the text (a messaging error can name the
webhook URL); every fallback names a next step.

**Done when.** A checkout refused by git shows git's reason in the alert; a
test proves the announce error never carries the webhook.

### UX-80 Dead-end empty states, and four ways to say "connecting"

Impact: low · Effort: small

**Today.** Good: `Select an issue to see its detail.`, `Open a pull request
first — there is nothing to announce yet.`, `{service} is not configured.
Add a token or webhook in Settings.` Dead ends: `This directory is not a
Git repository.` (`web/src/features/branch/BranchPanel.tsx:22`) and `The configuration could not be
loaded.` (`SettingsPanel.tsx:17`) offer nothing to do. And the one condition
"no snapshot yet" reads `Connecting to the tracker…`, `Connecting to the
workspace…`, `Connecting to the forge…` and `Connecting…` in four panels
(`web/src/features/issues/IssuesPanel.tsx:19`, `web/src/features/branch/BranchPanel.tsx:14`, `web/src/features/review/ReviewPanel.tsx:37`,
`web/src/features/messaging/MessagingPanel.tsx:13`).

**Instead.** One connecting line, in the shell; a retry on the config
error; the not-a-repository state says what a repository would give it.

**Done when.** One string for connecting; the settings error has a Retry
button.

### UX-81 Disabled by dimming, and no rule for reduced motion

Impact: medium · Effort: small

**Today.** Every disabled button uses `disabled:opacity-60` — fifteen
places (`web/src/features/issues/IssuesPanel.tsx:349`, `web/src/features/issues/WorkStory.tsx:248`, `:278`,
`web/src/features/branch/BranchPanel.tsx:138`, `web/src/features/branch/CommitForm.tsx:130`,
`web/src/features/branch/WorkingTree.tsx:176` — the staging buttons' shared
class — `web/src/features/review/ReviewPanel.tsx:206`, `:345`,
`:352`, `web/src/features/review/OpenedOutcome.tsx:90`,
`web/src/features/messaging/MessagingPanel.tsx:170`, `:216`, `:231`, `:239`,
`SettingsPanel.tsx:318`) — which CLAUDE.md's accessibility rule names as the
thing not to do (opacity dims text below the contrast floor) and which axe
does not catch on disabled controls. The app has only two `transition-colors`
and no animation, but no `prefers-reduced-motion` rule either; the a11y spec
forces `reducedMotion: 'reduce'` to stabilize its scan, not because the app
honors it.

**Instead.** A `disabled:` color treatment on the token scale; `motion-safe:`
on the two transitions and one reduced-motion rule in `index.css`.

**Done when.** `grep -c opacity-60 web/src` is 0; axe passes both themes.

### UX-82 A bad stream frame disappears without a trace

Impact: medium · Effort: small

**Today.** A frame that fails `zSnapshot.safeParse` is dropped
(`web/src/api/snapshot.ts:63`); the last good snapshot stays on screen and
`StreamStatus` still says `Live`. A field added on the server without
regenerating the client makes every frame vanish, indistinguishably from a
quiet repository (DEBT-67).

**Instead.** A dropped frame sets the stream status to `stale` with the
reason, shown in the same pill.

**Done when.** Feeding the store a schema-mismatched frame shows `stale` in
`StreamStatus`.

### UX-83 The web has no Reviews section

Impact: medium · Effort: medium

**Today.** The review queue — pull requests waiting on you — is a pane in
the interface (`internal/tui/reviewqueue.go`) and a command (`workflow
reviews --json`, `internal/cli/reviews.go`), both over `Forge.ReviewRequests`.
The web's `web/src/shell/sections.ts:6` has no such section, and the contract has no
operation for it.

**Instead.** `GET /api/reviews` over the same seam, polled by the query
client (it is a cross-repository forge search, not a snapshot field), and a
sixth section listing number, title, repository, requester, CI and age with
open/copy links.

**Done when.** The web lists the same requests the CLI prints, by role and
name.

### UX-84 The web's visual system is not the product's

Impact: medium · Effort: large

**Today.** The interface's system — five hues for five systems (Jira blue,
git yellow, the forge green, chat magenta; red for failure and nothing
else) and state carried by the *shape* of `○ ◐ ● ✗` (`internal/tui/glyphs.go:16`,
`:96-127`; the spine at `internal/tui/spine.go:68`) — is stated, and the previous edition
says none of it should change. The web carries none of it across: one
periwinkle accent `#8b93f8` chosen "clear of the green/amber/red the status
lights own" (`web/src/index.css:24`) plus a three-color CI language; Jira is
not blue, git is not yellow, the forge is not green anywhere; the `NavRail`
icons are all muted (`web/src/shell/NavRail.tsx:30`). State marks are the web's own:
`StageMarker` (`web/src/features/issues/WorkStory.tsx:291`) invents three, and `ciDot`
(`web/src/features/review/ReviewPanel.tsx:20`) and `StreamStatus` (`StreamStatus.tsx:4`) are
color-only dots of one shape (mitigated by a text label beside each). And
the page shows the template tells the interface avoids: nine `uppercase`
eyebrow headings from seven class strings (`SettingsPanel.tsx:343`,
`web/src/features/branch/BranchPanel.tsx:67`, `web/src/features/branch/WorkingTree.tsx:15`, `web/src/features/issues/IssueDetailPanel.tsx:8`
— the work story, description and comments share it — `web/src/features/review/ReviewPanel.tsx:105`,
`web/src/features/messaging/MessagingPanel.tsx:41`, `:61`) as the *only* heading treatment; no type or spacing tokens (raw `text-2xl`
… `text-xs`, `gap-8` … `gap-0.5` per component); one radius on everything
(`rounded-md` ×28). To its credit: no shadows, no gradients, no `→`, and a
real color-token system with hand-picked contrast (`index.css:9-96`).

**Instead.** The four system hues as tokens in both themes, at AA
contrast, carrying identity (the active nav icon, section headings); one
`StateMark` component that draws `○ ◐ ● ✗` for CI, the stream and the work
story (label kept, mark `aria-hidden`); sentence-case headings on a
`--text-*`/`--space-*` scale; the radius scale actually used. The periwinkle
accent can stay for interactive controls — it was chosen on purpose — or be
replaced; that is the maintainer's call. The interface's own system does
not change.

**Done when.** No `uppercase` heading remains; every state has a distinct
shape; both themes pass axe.

### UX-85 The layout has no breakpoints

Impact: medium · Effort: medium

**Today.** There is not one `sm:`, `md:`, `lg:` or `xl:` utility in any
`.tsx` under `web/src`. A `w-20` rail (`web/src/shell/NavRail.tsx:14`) beside a `w-80
shrink-0` issues list (`web/src/features/issues/IssuesPanel.tsx:102`) and a `flex-1` detail squeezes
the detail to nothing near 640 px; definition lists use fixed first columns
(`grid-cols-[6rem_1fr]` `web/src/features/branch/BranchPanel.tsx:49`, `[8rem_1fr]`
`web/src/features/messaging/MessagingPanel.tsx:31`, `[9rem_1fr]` `web/src/features/review/ReviewPanel.tsx:91`); panels cap at
`max-w-2xl` and never reflow; the issues `<ul>` is not scrollable, so a long
list scrolls the page. The viewport meta tag is present (`index.html:5`) and
nothing responds to it. (From code; no viewport was rendered.)

**Instead.** The rail collapses to icons under `md`; list and detail stack
under `lg`; the grids and the caps go fluid; the list scrolls inside its
panel.

**Done when.** A Playwright spec at 640, 1024 and 1440 px finds no horizontal
scroll and every control reachable.

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
(`SettingsPanel.tsx:26`) so `ui`, `timing`, `headers`, `views` and
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
would give the web's five sections keyboard reach.

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
no place in the terminal where the system is not applied; the one open
question is the web's, and it is UX-84.

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
