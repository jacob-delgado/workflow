# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up UX-62". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `e5156e3` on 2026-10-06: the
`claude/wizardly-babbage-lptyxi-ux2` branch, where the Summary's post on
every surface, the web's answer to the terminal's keys and both
interfaces' first-run setup shipped on top of the `docs/ux-refresh`
edition at `cac1adf`. This edition also tracked the fixes made on the
branch since: a setup that refuses a link and keeps no address that is
not one, a Summary refused when too long for its service, escaped titles,
one setup at a time, the web's keys held during a confirm and kept off a
Mac's Control+K, and the terminal telling what was never set up as
guidance rather than as a failure; an entry they closed is gone, and one
they narrowed says only what is left. Every pointer was read again at that
commit. Line numbers drift, so every pointer also names the symbol it
means.

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
   the failure voice (`errorSentence`, `internal/tui/failure.go:90`); the
   web says it in a `role="alert"` line beside what failed. A service that
   is not set up is not a failure: every surface says what is missing and
   how to set it up, beside the not-started mark, as guidance rather than
   an alarm (`loop.NotSetUp` decides which is which, once, and
   `loop.SetUpAdvice` words it, `internal/loop/summary.go:76`, `:87`).
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
| The ready, merged or CI-red message | **Announce** … **Announced to** |
| The Summary, sent to the team | **Post** … **Posted to** |
| Narrowing by typing | **Search** (`/` in the terminal) |
| Narrowing by a checklist | **Filter** (`f` in the terminal) |
| Ordering | **Sort** (`O` in the terminal) |
| Finishing a Taskwarrior task | **Mark done** … **Marked done** |
| The local stores | **Local data**, **Remove** … **Removed** |
| Reading again after a failure | **Try again** |
| Reading | **Reading …** |
| `esc` | **discard** when it drops work, **close** when it keeps it, **cancel** for an unsent form, **back** for a step inside a flow, **stay** at a guard, **skip** for an offer after a done act (the constants beside `everywhereKeys`, `internal/tui/keys.go`) |

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
| List, search and filter issues | — (FEAT-78) | Issues pane: `/`, `f`, `v`, `ctrl+n` | Issues: View, Search, Filter, Load more |
| Read an issue and its comments | — (FEAT-78) | the detail | the detail |
| Comment | `comment KEY`, the text on stdin | `c`, a composer with vim modes | the comment composer, Markdown |
| Change its status | moves to `jira.review_status` inside `pr` | `t` | Change status, with the transition's fields |
| Assign; log work | — (FEAT-78) | `a`; `w` | Assign; Log work |
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
| Run pre-commit; set up lefthook | not for scripts | `h`; `g` | Run pre-commit; Set up lefthook |
| **Review** | | | |
| Open a pull request | `pr [--json]` | `n` | Open a pull request |
| Link it on the issue, move the issue | `pr` asks both | offered after `n` | Link it on KEY, Move KEY to STATUS |
| Choose its template | `pr` takes the first (UX-62) | `ctrl+t` in the composer | Template |
| Edit it | not for scripts | `e` | Edit pull request |
| Read the checks and a job's log | `status` shows CI | `c`, then `l` | the CI checks, Show log |
| Re-run CI; merge; finish | not for scripts; finish — (FEAT-83) | `R`; `M`; `F` | Re-run failed checks; Merge; Finish the branch |
| Where the work stands | `status [DIR…] --json` | the top row | the work story, the header |
| **Messaging** | | | |
| Announce | `announce` | `p` | Announce to SERVICE |
| Announce when CI passes | — (FEAT-65) | `w` in the preview | Announce when CI passes |
| Edit the announcement first | not for scripts | `e` in the preview | Edit, in the preview |
| Know it was announced | `announce` says so, and asks again | "● announced" on the Messaging rail | the section says so |
| People and groups | not for scripts | `P` | Settings: People, Groups |
| Post the summary | `summary --post [--yes]` | Summary pane: `p`, after a preview | Summary: Post…, after a preview |
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
| Switch the directory | not for scripts | `enter`, `g` | Switch, Switch here |
| Favorites | listed by `repositories` | `f` | Add to favorites, Remove |
| List the worktrees | `repositories [--json]` | the Repositories pane | Worktrees |
| **Settings and local data** | | | |
| Set up from nothing | `config init`, `slack login` | `enter` on the no-file screen, or `S` on the Repositories pane | Settings, with no file |
| Read the configuration | `config show` | Settings: `S` on the Repositories pane | Settings |
| Change it | edit the file | Settings, `ctrl+s` | Settings, Save changes |
| Check the setup | `doctor [--online]` | a failure names `workflow doctor` | a failure names `workflow doctor` |
| Remove the local data | `db-clean` | Local data: `L` on the Repositories pane | Settings: Local data, Remove |
| Every key or command | `--help` | `?` | `?`, and ⌘K or Ctrl+K (outside a Mac's text field) to find an action by name |
| Hold back every write | `--dry-run` | `--dry-run` | `--web --dry-run` |

## The promises the interface makes

The terminal states its own rules, in its docs and in its code. This table
is the shortest summary of how far the screen keeps them, re-counted at
`e5156e3`.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "`?` lists every key" | `docs/content/docs/usage.md:89` | **Yes, by construction, and a test enumerates every placement.** Help is generated from the bindings (`helpBuilder.place`, `internal/tui/keys.go:164`, rendered in two columns split where they balance, `balancedSplit`, `internal/tui/help.go:150`): 101 placements — 91 `builder.bind` and 10 `builder.bindShown` calls — in 12 groups (`groupMoving` to `groupEverywhere`, `internal/tui/keys.go:99`), on 100 lines, since `cycle-type-right` has no help of its own and rides `cycle-type-left`'s line (`internal/tui/keys.go:345`). `TestHelpListsEveryPlacedBinding` (`internal/tui/help_test.go:292`) reads `?` back and holds it to a table of every placement. |
| "the one way the interface says something broke" | `wording`, `internal/tui/failure.go:51` | **Yes: every site that renders an error's text.** Each is told through `errorSentence` (`internal/tui/failure.go:90`) and drawn by one of eight helpers — those that draw a mark choose it, and its style, through `voice` (`internal/tui/failure.go:430`), the footer's two failure notices through `markOf` (`:418`), which asks the same `loop.NotSetUp`: `failureBlock` at 20 sites, `pinnedOutcome` 16, `failureLine` 24, `failureSummary` 8, `unreadRow` 3, `noticedFailure` 12, `noticedFailureLedBy` 3 and `noticedGuidance` 4 (plain, since red means something broke). They are also the one way the terminal says something was never set up: as guidance, with `○` and no red, in `loop.SetUpAdvice`'s words (`setUp`, `internal/tui/failure.go:145`), as principle 4 asks. |
| "Nothing outward facing is sent without" a last look | `commentPreview`, `internal/tui/comment.go:54` | **Yes, under principle 2.** Every write that leaves the machine or cannot be taken back waits on a preview or a confirmation, most through `lastLook` (`internal/tui/overlay.go:229`): every write to Jira, the forge and the messaging service, every push, Taskwarrior's sync, mark done and undo, forgetting a person and every directory switch. The reversible local toggles principle 2 names act at once. |
| "a refused change must never go unseen" | `statusPicker`, `internal/tui/picker.go:321` | **Yes: 19 of 19.** Every overlay that sends a request refuses every key while it is in flight and keeps a refusal where it happened until `esc`: `branchCreator`, `branchLinker`, `branchPicker`, `commentPreview`, `finishPreview`, `hookgenOffer`, `issueLinker`, `issueWrite`, `lastLook`, `mergePicker`, `messagingPreview`, `peopleOverlay`, `prComposer`, `prEditor`, `settingsForm`, `setupForm` (`internal/tui/setupform.go:391`, while it writes or checks), `statusPicker` (with its field form), `summaryPost` and `taskLine`. |
| "Each pane fails on its own" | `docs/content/docs/usage.md:97` | **Yes.** `Init` (`internal/tui/tui.go:193`) batches eight loads, and the Summary and Repositories panes read on first focus (Init reads Repositories too when it starts focused there, `internal/tui/tui.go:201`); each pane holds and renders its own load's error, the Summary per source, and says of a service never set up how to set it up. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:16` | **Yes.** `unicodeGlyphs` and `asciiGlyphs` differ in shape (`internal/tui/glyphs.go:33`, `:45`); `NO_COLOR` keeps bold and faint. Two residues: the progress spine's five system hues are color-only, mitigated by each system's name or initial, and `◐` means both "partly staged" and "announces when CI passes" (see the visual system). |

To re-count rather than trust these numbers: `grep -o '\bfailureBlock('
internal/tui/*.go`, tests excluded, and the same for each helper, less
what `failure.go` itself holds — each helper's definition, the `Model`
wrappers and the calls one helper makes into another there
(`pinnedOutcome`'s `failureBlock`, `noticedFailure`'s
`noticedFailureLedBy`); `grep -n 'send\.sending\|sending\.sending'
internal/tui/*.go` for the overlays that guard a send, keeping the types
whose key handler returns on it; and the `builder.bind(` and
`builder.bindShown(` calls in `internal/tui/keys.go` for the help.

## The command line

What is open here is the flags the scriptable commands lack and the JSON
a script cannot join or time.

### UX-62 Flags the scriptable commands are missing

Impact: low · Effort: medium

**Today.** No command declares a single shorthand — there is no `VarP(`
call in `internal/cli` — so `-n`, `-y`, `-j` do not exist; `status` emits
`●◐✗○` (`statusGlyph`, `internal/cli/status.go:522`) with ASCII selectable
only through `ui.ascii` in the file, no `--plain`; `pr` declares only
`--yes` and `--json` (`newPRCmd`, `internal/cli/pr.go:58`, the flags at
`:84` and `:86`), so no draft, base, reviewer, title or body flag;
`announce` has no `--channel` (the channel comes from `messaging.channel`
alone); `pr` names no template, so it takes the repository's first
(`chosenTemplate`, `internal/loop/pull.go:185`, given an empty name at
`:191`), where the interface cycles them (`ctrl+t`,
`internal/tui/keys.go:337`) and the web's pull request form offers a
Template select (`TemplateChoice`,
`web/src/features/review/OpenPullRequest.tsx:249`).

**Instead.** Shorthands for the three common flags; `--plain` on `status`;
`--draft`, `--base`, `--reviewer`, `--channel`, `--template` where the seam
already carries the value.

**Done when.** Each flag has a test that it reaches the seam.

### UX-92 The scriptable output lacks a unique label and a timestamp

Impact: low · Effort: small

**Today.** A script reading the JSON has no stable key to join on and no
time to compare. (`pr --json` has shipped, printing what was opened and
the offers that followed.) UX-62 keeps the missing flags.

- `internal/cli/status.go:229` (`repoLabel`): the label is the base name,
  and "." is returned as itself (`:231`), so `status .` labels the row "."
  (`TestStatusAcrossLabelsTheCurrentDirectory`,
  `internal/cli/status_test.go:96`, pins that prefix) and `status ~/a/api
  ~/b/api` gives two rows the same `repository`, the only key
  `docs/content/docs/scripting.md:147` (under "JSON") offers.
- `internal/cli/reviews.go:151` (`reviewReport.Age`): a string, filled by
  `renderReviewsJSON` (`:156`) through `humanizeAge` (`:130`), which
  rounds 25 h and 47 h alike to "1d"; `ReviewRequest.OpenedAt`
  (`internal/forge/pulls.go:194`) holds the time and never reaches the
  JSON. `docs/content/docs/scripting.md:163` documents `"age": "3d"` as
  the shape, so `jq 'map(select(.age > "2d"))'` compares strings.

**Instead.** Label a `status` row by the cleaned absolute path's base
name, falling back to the full path when two arguments would share a
label, and carry the given path in a second `path` field; add `opened_at`
(RFC3339 UTC, from `OpenedAt`) beside `age` and document it.

**Done when.** `status .` in a repository labels the row by the
directory's name and `status a/api b/api --json` yields two distinct
`repository` values; `TestReviewsAsJSONReportsEachOldestFirstWithItsAge`
(`internal/cli/reviews_test.go:163`) also parses `opened_at` back into the
time it seeded.

## The terminal interface

What is open here is a screen-reader mode, the alternate screen and a fixed
delay, and undo.

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint
(`UI.DrawColor`, `internal/config/ui.go:61`; `Model.WithoutColor`,
`internal/tui/tui.go:162`); `ui.ascii` swaps glyphs and borders
(`asciiGlyphs`, `internal/tui/glyphs.go:45`); escapes in server text are
neutralized. But there is no screen-reader mode; the alternate screen is
unconditional (`Model.View`, `internal/tui/render.go:38`, `view.AltScreen
= true`), so nothing the interface prints survives quitting; `ui.color`
has no `always` for a piped terminal that does support color; and the
150 ms detail delay (`detailDelay`, `internal/tui/detail.go:20`) is fixed.
A terminal at 80 by 24 gets the full-screen layout at its tightest,
and nothing offers a plain mode that prints a line at a time
instead; the command line is the only other way in.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`, which `Config.validateUI` (`internal/config/ui.go:69`) and the
`UIConfig.color` enum (`api/openapi.yaml:4796`) refuse until they list it;
`ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (the commit, `commitComposer.handleKey`,
`internal/tui/composer.go:257`; the pull request, `prComposer.handleKey`,
`internal/tui/prcomposer.go:351`), a dirty tree blocks a switch instead of
stashing (`errDirtyTree`, `internal/tui/switchtask.go:25`), and quit is
guarded while an announcement waits (`quitGuard`,
`internal/tui/messaging.go:67`). But a posted comment, an applied
transition, a merge and the `git branch -D` in finish
(`finishPreview.commands`, `internal/tui/finish.go:76`) have no undo, and
the interface never says which acts are reversible: the finish preview
lists the three commands and says nothing of what `-D` costs
(`finishPreview.view`, `internal/tui/finish.go:82`), and a comment's notice
is "commented on KEY" (`commentPosted.apply`,
`internal/tui/comment.go:144`).

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries the
issue's link so the comment can be edited where it lives. Undo itself,
over a log of what workflow did, is FEAT-87's.

**Done when.** Each irreversible act's preview or notice says so.

## The web

What is open here is a stream that marks no change; Settings' unseen
sections, misleading hints, blank selects and unremovable credential; what
the browser could borrow from the interface; the Branch section's detached
HEAD; and four states drawn as words with no mark.

Every pointer here was checked again at `e5156e3`. A screenshot named
by its file (`1440-dark-tasks.png`, say) is from a pass on 2026-10-05 over
every section at 640, 1024 and 1440 px in both themes, of the mock build,
and like the 2026-09-24 audit's is not in the repository; one that shows a
control drawn since — a field, a button, a row's facts — shows it as it
was before `web/src/lib` drew it.

### UX-86 Nothing marks a change the stream just made

Impact: low · Effort: medium

**Today.** `StreamStatus` shows `Connecting` / `Live` / `Reconnecting` /
`Out of date` (`web/src/shell/StreamStatus.tsx:7`), but nothing says when
the last snapshot arrived — the store keeps `receivedAt`
(`web/src/api/snapshot.ts:28`, set on every frame at `:78` and `:100`) and
nothing draws it — and a panel that changed because CI settled looks
exactly like one that re-rendered. There is no toast, and no "CI passed"
moment on the web where the interface rings the terminal
(`ciFinishNotice`, `internal/tui/review.go:155`).

**Instead.** A "updated 3 s ago" beside the pill, read from `receivedAt`;
a brief highlight on the row a snapshot changed; a status line when CI
settles, honoring `ui.notify` (`internal/config/ui.go:35`).

**Done when.** A snapshot that flips CI to passed produces a status
region saying so.

### UX-87 Settings can edit nine sections and carry six it cannot show

Impact: low · Effort: medium

**Today.** The form seeds itself with the whole `Config` (`ConfigForm`,
`web/src/features/settings/SettingsPanel.tsx:118`, its comment at `:119`
naming what rides along), so `ui` beyond `web_shortcuts`, `timing`,
`jira.headers`, `jira.views`, `branch.prefixes` and `messaging.channels`
(`internal/config/config.go:124`) survive a save unchanged — and cannot
be edited. The nine fieldsets it draws
(`web/src/features/settings/SettingsPanel.tsx:181` to `:189`) are Jira,
messaging, the forge, commits, branches, pull requests, the store,
Taskwarrior and the keyboard, whose one field is `ui.web_shortcuts`.

**Instead.** Fieldsets for the six, with `views`, `prefixes` and
`channels` as editable lists.

**Done when.** A view added in the browser appears in the interface's `v`
cycle, and a channel added there is offered in the announcement preview.

### UX-88 What the browser could borrow from the interface

Impact: low · Effort: small

**Today.** The web now reads a file's diff, amends and fixes up, runs
pre-commit, edits an open pull request and chooses among the repository's
pull request templates, as the interface does. The interface still jumps
to a failure in `$EDITOR` (`openFailure`, `internal/tui/run.go:382`),
which the web has no equivalent of. Four
smaller things the terminal shows are absent on the web too, none of them
among what `docs/content/docs/web.md` says stays in the terminal.

- A renamed file shows only its new path: `ChangeRow` renders
  `change.path` alone (`web/src/features/branch/WorkingTree.tsx:137`),
  though `changesDTO` sends `OriginalPath`
  (`internal/webserver/dto.go:181`) and `original_path` is read nowhere in
  `web/src` outside the generated client; the terminal's `changeRows`
  draws old → new (`internal/tui/commits.go:153`, the arrow at `:159`).
- The commit subject has no length against `commit.subject_limit`:
  `MessageFields`' Subject is a bare `<Input required placeholder>`
  (`web/src/features/branch/CommitForm.tsx:175`) though `subject_limit` is
  on the wire (`CommitConfig`,
  `web/src/api/generated/types.gen.ts:1817`); the terminal's
  `commitComposer.view` shows "n/limit" as typed
  (`internal/tui/composer.go:152`, the count at `:154`). The limit is met
  only as a 422 after the click.
- The Review section has no Copy URL: the title link is the only handle
  on the pull request (`PullRequestSummary`,
  `web/src/features/review/ReviewPanel.tsx:100`), where the queue's
  `CopyURL` one section over has a tested clipboard outcome
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:382`) and the
  terminal's `reviewKeys` offer `linkKeys` on the pull request
  (`internal/tui/review.go:417`).
- The Search text survives a view change: `IssueBrowser` keeps `filter`
  in local state nothing resets
  (`web/src/features/issues/IssuesPanel.tsx:68`) — the places picked
  beside it belong to the view they were picked in (`:75`), the text does
  not — and `ViewSelect`'s `onChange` calls `setView` alone
  (`web/src/features/issues/IssueListControls.tsx:73`), where the
  terminal's `nextIssueView` seeds a fresh list, search and all
  (`internal/tui/views.go:49`); choosing a view with "proj-12" still in
  the search shows "No loaded issue matches the filter." — the user asked
  for a view, not a narrowed one.

**Instead.** In rough order of value: `original_path` drawn before the
path with an arrow; a muted "n/limit" hint under the subject counting the
assembled header; `CopyURL` beside the title with the same "Copied the URL
of #128." outcome; clearing the search when the view changes.

**Done when.** Each lands with a role/name test; a `WorkingTree` test with
a renamed change finds both paths in the row; a `CommitForm` test with
`subject_limit: 20` finds the count text change as the subject is typed; a
`ReviewPanel` test clicks "Copy URL to #128" and reads the URL back from
the clipboard; a test types a search, selects another view, and finds the
searchbox named Search empty once the new frame lands.

### UX-107 The web's detached HEAD offers no way out

Impact: low · Effort: small

**Today.** With HEAD detached, the Branch section heads itself "Detached
HEAD at abcdef1" and then draws the same Base, Upstream and Tracking list,
Commits and working tree as on a branch, with nothing saying what to do
next — and the controls below the list, Link an issue and Rebase onto,
are withheld too (`web/src/features/branch/BranchPanel.tsx:104`, `:105`;
`canRebase`, `web/src/features/branch/HistoryActions.tsx:24`). The
terminal says it:
"Check out a branch, or press b to start one for the selected issue."

- `BranchSummary`, `web/src/features/branch/BranchPanel.tsx:71`: the
  detached heading (`:73`), followed by the branch's own list (`:90`).
- `BranchPanel`, `web/src/features/branch/BranchPanel.tsx:20`: renders
  `BranchSummary`, `Commits` and `WorkingTree` alike for a branch and a
  detached HEAD.
- `Model.branchDetail`, `internal/tui/branch.go:121`: the terminal's
  sentence (`:133`), which names the bound key.

**Instead.** Under the detached heading, one sentence pointing at Issues,
where Switch branch and Start work live.

**Done when.** A `BranchPanel` test with `detached: true` finds a line
naming Issues under the heading.

### UX-108 Three Settings hints that hide where a value comes from or applies

Impact: low · Effort: small

**Today.** Three Settings fields say nothing, or the wrong thing, about
their value. Beside UX-87's six carried unseen, these are values carried
and misdescribed.

- `JiraFieldset`'s Token hint is "Leave as-is to keep the stored token."
  (`web/src/features/settings/fieldsets/JiraFieldset.tsx:18`; on screen in
  `1440-light-settings.png`), over a field that is empty after the
  recommended macOS setup: `keepTokenSafe`
  (`internal/cli/config_cmd.go:360`) hands the token to `setup.Keep`
  (`internal/setup/setup.go:179`), which clears `jira.Token` and sets
  `TokenCommand` (`:189`) — as both interfaces' first-run setup does. A user who types a token to "fix" it writes a
  secret into the file, and "The file's own `token` wins when set"
  (`docs/content/docs/configuration.md:273`, under "Keeping tokens out of
  the file") — what `config init` worked to avoid.
- Announcement is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:50`), though
  `Messaging.Announcement` is a Slack-only template with seven
  placeholders (`internal/config/config.go:125`), which the fields table
  describes in one line (`docs/content/docs/configuration.md:124`). A
  Teams user edits it and sees no change; a Slack user has no placeholder
  list on screen.
- Channel is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:49`), though
  `Messaging.Channel` "applies to a Slack user token only; a webhook
  carries its own channel" (`internal/config/config.go:119`). A webhook
  user sees an editable channel that does nothing.

**Instead.** When `token_command` or `token_env` is set, replace the token
hint with "Taken from token_command: VALUE" (or the variable's name),
neither a secret; a hint on Announcement naming the placeholders, that it
applies to Slack only, and that empty keeps the built-in message; a hint
on Channel: "With a Slack user token; a webhook posts to its own channel."

**Done when.** A `SettingsPanel` test seeding `jira.token_command` finds
the Token textbox described by text naming that command;
`getByRole('textbox', { name: 'Announcement', description: /\{author\}/
})` and `getByRole('textbox', { name: 'Channel', description: /user token/
})` resolve.

### UX-109 Three Review rows and the queue's Draft are states with no mark

Impact: low · Effort: small

**Today.** A file's staged state now carries a `StateMark` before its
word (`ChangeRow`, `web/src/features/branch/WorkingTree.tsx`), as every
state on the web should. Four states are still plain words: the Review
section's State, Mergeable and Changes requested rows
(`PullRequestSummary` and `ReviewRows`,
`web/src/features/review/ReviewPanel.tsx`) and the queue's "Draft"
among a row's facts (`RequestRow`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx`). The words keep
them accessible, so this is consistency inside the system, not a change
to it.

**Instead.** A `StateMark` before each: open in flight, merged done,
closed not started; mergeable done, conflicting failed; changes
requested failed; a draft not started.

**Done when.** `ReviewPanel` and `ReviewQueuePanel` screenshots show a
shape before each of the four words.

### UX-119 A credential cannot be removed from Settings

Impact: low · Effort: small

**Today.** `keepSecret` treats an emptied secret field as "keep the stored
value" (`internal/config/redact.go:185`) and `KeepStored` applies it to
all six secrets and every Jira header value (`:157`) for every save
through `config.SaveEdit` (`internal/config/save.go:369`) — the web's and,
now, the terminal's Settings alike — so neither has a way to clear
`jira.token`, `messaging.client_secret`, `messaging.refresh_token`,
`messaging.webhook_url` or `forge.token`: clearing one in the form and
saving keeps it, and moving from a user token to a webhook leaves the old
client secret and refresh token in the file, to be removed by hand. The
keep is documented — `updateConfig` says an empty or masked secret field
keeps the stored secret (`api/openapi.yaml:1066`), and the web page says
under "Settings" that a credential is "kept as it is unless you type a new
one" (`docs/content/docs/web.md:491`) — but no clear is offered anywhere.
UX-87 covers sections the form cannot show, not clearing a secret.

**Instead.** Accept an explicit clear — a `null` for the secret fields in
the contract, or a per-field remove control — that writes an empty value.

**Done when.** A test sends `jira.token` as `null` and the saved file holds
no token.

### UX-124 Two Settings selects draw blank for the value in effect

Impact: low · Effort: small

**Today.** `pull_request.title_source` and `messaging.kind` may be the
empty string, and the server reads them as `commit` and Slack, and the
spec allows it. But the Title source and Service selects offer no option
for `""`, so a configuration holding it shows an empty control in either
theme, and once a value is picked there is no way back to the default.
Every Settings screenshot (`1440-light-settings.png` among them) shows
Title source empty under "Pull request", the only control on the form
with no visible value. A blank select saves back `""` harmlessly and
Settings is opened rarely, which is why this is low.

- `PullRequestFieldset` offers `commit` and `issue` only
  (`web/src/features/settings/fieldsets/PullRequestAndStoreFieldsets.tsx:16`),
  and `MessagingFieldset`'s choices begin at `slack`
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:16`);
  `SelectField` registers the `<Select>` with the value as-is
  (`web/src/features/settings/fieldsets/Field.tsx:119`), so `""` matches
  no option and shows blank. `ForgeFieldset` handles the same case with an
  explicit `['', 'Auto-detect']`
  (`web/src/features/settings/fieldsets/ForgeFieldset.tsx:13`).
- Neither `Default` (`internal/config/config.go:195`) nor `Template`
  (`:207`), which `config init` writes, sets a `PullRequest`, so
  `title_source` is `""` in every file workflow writes. `Template` sets
  `Kind: KindSlack` (`:217`), but the guided setup starts from `Default`
  (`setup.Beneath`, `internal/setup/setup.go:197`), whose messaging has
  no kind, and `withWebhook` leaves it so when the webhook question is left
  blank for the user token (`internal/setup/setup.go:137`) — on the command
  line and in both interfaces' first run alike — so that path writes kind
  `""` and the Service select draws empty while the messaging section's rail
  label and heading say "Slack" — two surfaces disagreeing about one file.
- `PullRequest.TitleSource` documents `commit` as the default
  (`internal/config/pullrequest.go:19`) and its tag is
  `json:"title_source"` without `omitempty` (`:21`),
  `validatePullRequest` accepts `""` (`:27`), and `encode`'s
  `MarshalIndent` of the whole struct writes the empty value
  (`indented`, `internal/config/save.go:174`).
- `MessagingConfig`'s `kind` `enum` is `["", slack, teams, discord,
  webhook]` (`api/openapi.yaml:4735`), so the spec allows what the select
  cannot show; the mock the screenshots show has `title_source: ''`
  (`web/src/dev/mockConfig.ts:58`).

**Instead.** A first choice `['', "The branch's oldest commit
(default)"]` and `['', 'Slack (default)']` as `ForgeFieldset` does for its
empty kind — or normalize `""` onto the default when seeding the form and
let the save write it.

**Done when.** A `SettingsPanel` test seeding `title_source: ''` finds the
Title source combobox with a selected option naming the oldest-commit
default, and one seeding `messaging.kind: ''` finds the Service combobox
with a selected option named for Slack.

## The visual system

What is there is a real system, and a good one for a terminal: five
systems' hues — Jira blue, git yellow, the forge green, chat magenta and
Taskwarrior cyan (`newStyles`, `internal/tui/glyphs.go:114`; Taskwarrior's
marks the active task at the spine's end, `internal/tui/spine.go:122`) —
and red for failure alone, all taken from the terminal's own palette as
ANSI indices (`internal/tui/glyphs.go:100`) so the user's theme decides
the shades; shape for state (`○ ◐ ● ✗`, or `o * # x` in ASCII,
`internal/tui/glyphs.go:35`, `:47`); border weight for focus. None of that
should change.

The shapes now mark more than a status, and stay on one axis while they
do: `○` not begun, `◐` under way or partway, `●` done, `✗` broke. `◐`
means both "partly staged" on the Commits pane (`Model.stageGlyph`,
`internal/tui/commits.go:170`) and "announces when CI passes" on the
Messaging pane (`Model.messagingState`, `internal/tui/messaging.go:142`);
that is not a conflict, since each is the halfway point of its own
progression and words stand beside it. The rule a new use must keep: a
state glyph says how far something has got, never anything else. A
checkbox therefore has its own shapes, `[x]` and `[ ]` in both glyph
sets (`glyphs.checkbox`, `internal/tui/glyphs.go`). Three marks are not states
and take shapes of their own: `★` (`^` in ASCII) for a favorite
directory, `‹›` (`<>` in ASCII) for the value under a cursor — the
calendar's and the commit type's (`internal/tui/calendar.go:117`,
`internal/tui/composer.go:216`) — and `[]` for the value another calendar
column stands on (`internal/tui/calendar.go:119`). The diff is the one documented exception to
the hue rule: an added line is drawn in the forge's green and a removed
one in red (`Model.markDiffLine`, `internal/tui/diff.go:111`, `:113`;
`styles`, `internal/tui/glyphs.go:90`), where the `+` and `-` git leaves
in place carry the meaning by shape (`internal/tui/diff.go:68`). Border
weight is the one rule the screen loses at its commonest size: at 80 by
24 the detail draws borderless, and the rail's heavy rules around the
focused pane carry the focus instead.

The web now speaks it too. The five systems' hues are tokens in both
themes (`web/src/index.css:60`), held at least 30° of OKLCH hue from the
status lights, the periwinkle control accent and each other, and at 4.5:1
as text, by `web/src/tokens.test.ts`; that separation moves two off the
terminal's families — the forge is teal, clear of the success light's
green, and Taskwarrior violet. They mark whose a thing is — the active
rail icon, each section's heading, the work story's stages and an issue's
local branch — and never how it stands. Every state is drawn by its shape
through one `StateMark` (`web/src/shell/StateMark.tsx:36`), hidden from
assistive tech beside its words. Type, space and corners each have one
scale (`web/src/index.css:164`), headings are set in sentence case — the
web lint refuses an `uppercase` class (`web/eslint.config.js:38`) — and
monospace is for code alone. Periwinkle stays the one control accent, and
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
1024 and 1440 px in both themes (`web/e2e/layout.spec.ts:19`): nothing
scrolls sideways, nor the page down, Tab reaches every control, each in
view as it takes focus, and axe finds nothing. It holds the steps a click
opens to the same widths — the pull request form, the push and discard
confirmations, the announcement and Summary previews, the forget and
remove confirmations, a refused write and the first run's setup.

## Across the surfaces

Nothing is open here at the moment. What each surface can do is the
table at the head of this file, and the names the three surfaces share are
the Vocabulary table's beside it; a sentence one surface words differently
from another, or that assumes GitHub or the terminal, is an entry here.

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
(`Model.noticed`, `internal/tui/overlay.go:181`) — sits among them.

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
`internal/tui/panes.go:36`), once written up as five in FEATURES.md, six
before the Tasks pane joined it, seven before the Summary and eight
before Repositories, *is* the decision as built. UX-154's lines mode
draws the same nine panes one at a time, on request, rather than
reopening the rail; web.md's "What stays in the terminal" lists
choices for now, not settled decisions.
