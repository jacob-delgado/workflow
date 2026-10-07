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
sections, misleading hints, blank selects, failed read and unremovable
credential; what the browser could borrow from the interface; the Branch
section's detached HEAD; keyboard focus; four states drawn as words with
no mark; links, stages and controls that tell less than their siblings;
and copy settled site by site.

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

### UX-106 Thirty-seven controls let keyboard focus fall to the page

Impact: low · Effort: small

**Today.** Thirty-seven controls set native `disabled` while their request
or run goes, which drops focus in Chromium and WebKit, and nothing re-takes
it. A keyboard user who is refused — a dirty tree, a branch that already
exists, the dry-run hold on every Start work press, a Taskwarrior refusal,
a Summary too long for its service — hears the reason and is left at the
top of the page, and one who presses Run pre-commit is left there for the
whole run. `useAsyncAction`'s catch sets the error and the state only
(`web/src/lib/useAsyncAction.ts:53`), so `onDone` never runs on a refusal
and the panel's `OutcomeLine` has nothing to follow. Counted from `grep -rn
"disabled={" web/src --include='*.tsx'`, tests and `aria-disabled`
excluded: 62 lines. Four name no request — a transition not yet chosen
(`web/src/features/issues/writes/StatusChangeForm.tsx:46`), an edited
description's Template (`web/src/features/review/OpenPullRequest.tsx:273`),
a fixup with no commit chosen
(`web/src/features/branch/HistoryActions.tsx:224`) and a group checkbox
under `--dry-run` (`web/src/features/settings/people/RepoGroups.tsx:74`) —
and three hand a prop down
(`web/src/features/settings/people/PeopleTable.tsx:100`, `:217`;
`web/src/features/messaging/TagPicker.tsx:74`), so 55 hold a running
control. Eighteen of those keep focus or never had it: the push
(`PushButton` re-focuses its opener on an error,
`web/src/features/branch/BranchPanel.tsx:180`) and Local data's two remove
openers (`web/src/features/settings/people/LocalData.tsx:182`); the
announcement preview's channel select, text box, Edit, Cancel, Announce
when CI passes, Announce now and group checkboxes (`AnnouncePreview`,
`web/src/features/messaging/AnnouncePreview.tsx:95`–`:183`;
`web/src/features/messaging/TagPicker.tsx:286`), whose send hands focus
back first (`PreviewStep`'s `send`,
`web/src/features/messaging/MessagingPanel.tsx:251`); seven that did not
have focus — the Cancels beside a send
(`web/src/features/review/OpenPullRequest.tsx:169`,
`web/src/lib/WriteForm.tsx:79`, `web/src/features/branch/HookSetup.tsx:147`,
`web/src/features/settings/people/PeopleTable.tsx:325`,
`web/src/features/branch/WorkingTree.tsx:398` in the discard confirm,
`web/src/features/summary/SummaryPost.tsx:129` in the Summary's preview)
and a code owner's Forget… held while the row's select saves
(`web/src/features/settings/people/PeopleTable.tsx:224`) — and
`WriteForm`'s fieldset (`web/src/lib/WriteForm.tsx:74`), whose fields the
send took focus from. The 37, by section, counted by line (`WriteForm`'s
one send serves ten forms):

- Issues: the list row's `RowCheckout`
  (`web/src/features/issues/IssuesPanel.tsx:528`); the story's
  `CheckoutButton` (`web/src/features/issues/WorkStory.tsx:353`) and
  `StartWorkButton` (`:385`), the path every dry-run press takes;
  `StartInWorktreeButton`
  (`web/src/features/issues/StartInWorktree.tsx:34`); and the send of
  every write form (`WriteForm`, `web/src/lib/WriteForm.tsx:82`): Change
  status, Assign and Log work here, the pull request's four writes and the
  Branch section's three looks below.
- Branch: `ChangeRow` (`web/src/features/branch/WorkingTree.tsx:145`),
  `StageAll` (`:303`), `UnstageAll` (`:338`) and the discard confirm's
  Discard (`:407`), which keeps its refusal in the confirm;
  `CommitForm`'s submit (`web/src/features/branch/CommitForm.tsx:97`);
  `LinkForm`'s Link (`web/src/features/branch/IssueLink.tsx:198`), while
  Unlink in the same file holds with `aria-disabled` (`:116`); Rebase
  onto, Run pre-commit, Amend last commit and Fix up a commit, each off
  while any run goes (`web/src/features/branch/HistoryActions.tsx:82`,
  `:145`, `:158`, `:169`); and Set up lefthook's two writes
  (`web/src/features/branch/HookSetup.tsx:152`, `:161`).
- Review: `OpenPullRequest`'s compose button
  (`web/src/features/review/OpenPullRequest.tsx:76`); `PullRequestForm`'s
  submit (`:172`), which stays up on a refused open; and `FollowUpOffer`'s
  button (`web/src/features/review/OpenedOutcome.tsx:85`).
- Messaging: `AnnounceControls`' Announce to
  (`web/src/features/messaging/MessagingPanel.tsx:171`); in the tag
  picker, an untagged owner's select and Not on Slack, held while a link
  saves (`web/src/features/messaging/TagPicker.tsx:235`, `:256`); and Stop
  waiting (`web/src/features/messaging/HeldAnnouncement.tsx:58`).
- Tasks: `TaskLineForm`'s submit
  (`web/src/features/tasks/TaskLineForm.tsx:80`); every `Verb` — Start,
  Stop and the rest (`web/src/features/tasks/TaskDetail.tsx:319`) — and the
  confirm's own button for Mark done…, Undo… and Sync… (`:305`); and
  `TrackIssue`'s Track in Taskwarrior
  (`web/src/features/tasks/IssueTasks.tsx:191`).
- Summary: the preview's Post
  (`web/src/features/summary/SummaryPost.tsx:138`), which keeps a refusal,
  a too-long Summary among them, in the preview.
- Settings: the configuration's Try again (`ConfigArea`,
  `web/src/features/settings/SettingsPanel.tsx:68`), its message in an
  `EmptyState` rather than an alert (UX-120), which hands focus to the
  form only once the read answers; Save (`SaveControls`, `:204`), which
  reports "Saved." through its own `role="status"` span (`:207`), a second
  state machine beside the shared `OutcomeLine`, so a mouse-clicked Save
  disables itself under focus and the span does not take it; the
  changed-since-read Reload (`ChangedSinceRead`, `:234`); a code owner's
  Slack select (`web/src/features/settings/people/PeopleTable.tsx:261`)
  and the forget confirm's Forget (`:334`); Save groups
  (`web/src/features/settings/people/RepoGroups.tsx:89`); and the first
  run's write and its Write it anyway
  (`web/src/features/settings/SetupForm.tsx:273`, `:278`).

The project already knows the rule, and keeps it in ten places: Load
more (`MoreIssues`, `web/src/features/issues/IssuesPanel.tsx:419`), the
review queue's Try again (`Queue`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:129`), the Try again
of every other failed read — the issue, Summary, Repositories and the
three Settings reads under the form (`Unread`,
`web/src/lib/Status.tsx:78`) — Show log (`CheckLog`,
`web/src/features/review/Checks.tsx:107`), Show diff
(`web/src/features/branch/WorkingTree.tsx:230`), Set up lefthook's read
(`web/src/features/branch/HookSetup.tsx:44`), Unlink, the comment's send
(`web/src/features/issues/CommentComposer.tsx:410`), every directory
switch's Switch (`ConfirmSwitch`,
`web/src/features/repositories/ConfirmSwitch.tsx:57`) and the Tasks
Refresh (`web/src/features/tasks/TasksPanel.tsx:214`) all hold with
`aria-disabled`; and `canHoldFocus` treats a disabled control as unable
to hold focus (`web/src/lib/Outcome.tsx:98`).

**Instead.** Hold each running control with `aria-disabled` (guarding
`onClick` or `onChange` as `Unread`'s Try again does) so it keeps focus
through a refusal and a run, and say Settings' save result through
`useOutcome` and `OutcomeLine` as the other panels do, keeping the
changed-since-read alert separate.

**Done when.** A test presses each of the thirty-seven against a refused
request, or a run, and finds `document.activeElement` still on it once
the refusal or the run's end is shown; a `SettingsPanel` test clicks Save
with the mouse, awaits "Saved.", and finds `document.activeElement` on the
status line, not `document.body`; the grep above finds no `disabled={`
that names a request's running state.

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

### UX-120 Settings draws a failed configuration read as a resting state

Impact: low · Effort: small

**Today.** When the configuration cannot be read, `ConfigArea` puts the
reason inside `<EmptyState>`
(`web/src/features/settings/SettingsPanel.tsx:63`), as muted text with no
`role="alert"` (`:65`); `EmptyState` is the dashed, centered,
`text-muted-foreground` box a panel shows when it has nothing yet
(`web/src/shell/EmptyState.tsx:6`). Every other failed read now says its
reason in `text-destructive`: through `Unread`
(`web/src/lib/Status.tsx:72`) for the issue, the Summary, Repositories and
the three Settings reads directly beneath this one; through `ReadFailure`
(`:34`) for the Issues list, the branch, the working tree and the Review
section; and, in the review queue, through its own `role="alert"` line
(`Queue`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:122`). Red is the failure color and nothing else, and here a
failure is not red: a 1024 px dark screenshot of the production build's
empty Settings section, taken in the 2026-09-24 audit, showed gray "The
configuration could not be loaded." centered in the dashed box, the same
drawing as "Connecting to workflow…" (`web/src/shell/SectionPanel.tsx:42`),
while its empty Reviews twin showed the queue's failure red and
left-aligned; a screen reader hears nothing, since no live region carries
it. A Try again is right there, held with native `disabled` (UX-106), and
a configuration that fails to read is outside the daily loop, which is
why this is low.

**Instead.** Say the failure through `Unread`, as the three reads below
it do: a `role="alert"` line in `text-destructive` with Try again beside
it, outside the `EmptyState`.

**Done when.** A `SettingsPanel` test with a failing configuration read
finds the reason by role alert; a screenshot shows it in the failure color.

### UX-122 Web copy settles plurals, case, periods and state words site by site

Impact: low · Effort: small

**Today.** The same act, state or moment reads differently depending on
where the eye lands — list versus story, one `dl` row versus the next,
header versus content, browser versus terminal.

- The web has no plural helper, where the terminal's `plural` counts in
  words (`internal/tui/render.go:508`): `changesDetail` says "{n} file(s)
  to commit" (`web/src/features/issues/WorkStory.tsx:175`; "3 file(s) to
  commit" in `1440-dark-issues.png`), Set up lefthook writes "script(s)"
  and "hook(s)" (`web/src/features/branch/HookSetup.tsx:78`, `:106`,
  `:120`), and `filterOutcome` "{shown} of {loaded} loaded issues match."
  (`web/src/features/issues/IssuesPanel.tsx:463`), so "1 … match."
  disagrees in number; meanwhile three sites count correctly inline, each
  its own way (`queueSummary`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:183`; the task count,
  `web/src/features/tasks/TasksPanel.tsx:275`; the comment's characters,
  `web/src/features/issues/CommentComposer.tsx:407`).
- `loadOutcome` ends "4 of 5 loaded" without a period
  (`web/src/features/issues/IssuesPanel.tsx:445`) and "All 5 loaded." with
  one (`:448`).
- Nine placeholders split four lowercase to four sentence case and one
  path: "what the change does, in the imperative" (`MessageFields`,
  `web/src/features/branch/CommitForm.tsx:178`), "comma-separated
  usernames or org/team", "comma-separated usernames" and
  "comma-separated labels" (`ProposalFields`,
  `web/src/features/review/OpenPullRequest.tsx:218`, `:223`, `:228`), against
  "Key or summary" (`web/src/features/issues/IssueListControls.tsx:29`),
  "Text, +tag, issue or #id"
  (`web/src/features/tasks/TaskListControls.tsx:72`), "Write a
  comment…" (`web/src/features/issues/CommentComposer.tsx:181`) and the
  palette's "Find an action in SECTION"
  (`web/src/features/keyboard/CommandPalette.tsx:104`).
- The list row's Switch branch says "Switching…" while it runs
  (`RowCheckout`, `web/src/features/issues/IssuesPanel.tsx:534`) but keeps
  its `aria-label` "Switch branch for KEY" (`:527`), so a keyboard user's
  visible word is not in the button's accessible name, where a file's
  Stage words its label by the same state
  (`web/src/features/branch/WorkingTree.tsx:144`).
- The never-done Announce stage is "Not announced" off HEAD
  (`notStartedStages`, `web/src/features/issues/WorkStory.tsx:81`;
  `offHeadStages`, `:95`) and an imperative on it: `announceDetail` reads
  "Announce to {channel}" (`:169`), on a button that opens the messaging
  section — "Announce to #dev-workflow" in `1440-dark-issues.png`.
- A missing base is "—" (`BranchSummary`,
  `web/src/features/branch/BranchPanel.tsx:92`) beside a missing upstream
  "none" (`:94`); Repositories says "None" for a missing origin
  (`web/src/features/repositories/WorkingIn.tsx:43`).
- `SectionPanel` says "Connecting to workflow…" while the snapshot is null
  (`web/src/shell/SectionPanel.tsx:42`), never reading the stream's
  status, while `useEventStream` sets `reconnecting` on every
  `EventSource` error, one before any open included
  (`web/src/api/snapshot.ts:117`); so the header says "Reconnecting" over
  every section's "Connecting to workflow…" at once, where
  `docs/content/docs/web.md:68`, under "The page", defines Reconnecting as
  "the connection dropped" and Connecting (`:66`) as no first update yet.
- The empty review queue is "Nothing is waiting on your review." on the
  web (`queueSummary`, read only by a screen reader,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:177`; `Requests`,
  `:246`) and "No pull requests are waiting on your review." in the
  terminal's detail (`reviewQueueDetail`,
  `internal/tui/reviewqueue.go:172`) and on the command line
  (`renderReviews`, `internal/cli/reviews.go:105`).
- Past a month `waited` writes the date as running text, "Aug 15, 2026"
  (`writtenDate`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:365`),
  under a comment promising "the terminal's words" (`:342`), where the
  terminal's `age` prints `time.DateOnly` (`internal/tui/detail.go:484`).
- The configuration's failed read is "could not be loaded" (`ConfigArea`,
  `web/src/features/settings/SettingsPanel.tsx:65`) and its failed reload
  "could not be read again" (`ConfigForm`, `:164`) in the same file, while
  the three reads below it say "could not be read"
  (`web/src/features/settings/people/PeopleTable.tsx:37`).

**Instead.** A `plural(count, noun)` in `web/src/lib/utils.ts` for the
four `(s)` sites and the three inline ones; one form for both load-count
states; one case for every placeholder; the row's `aria-label` worded by
its state, as a file's Stage is; one Announce phrasing beginning "Not
announced" that names the channel; one placeholder word for absence in
every `dl` row, in the muted foreground; `SectionPanel` wording its wait
from the same status table as `StreamStatus`, or the stream staying
"Connecting" until its first open; one empty-queue sentence on all three
surfaces, and one form for a date past a month on both interfaces;
"could not be read" in Settings.

**Done when.** `WorkStory`, `HookSetup` and `IssuesPanel` tests read "1
file to commit", "3 files to commit", "1 hook" and "1 of 2 loaded issues
matches."; `grep -rn "(s)" web/src --include='*.tsx'` finds
nothing outside generated code; every `placeholder=` under `web/src` is in
one case; the paging test asserts one load-count form; a test pressing the
row's Switch branch with a pending promise finds a button named
/switching/i; a grep for "could not be loaded" in
`web/src/features/settings` returns nothing; the three-case Announce test
asserts a phrase beginning "Not announced"; a test with base `''` and
upstream `''` finds the same placeholder in both `dd` cells; a
`SectionPanel` test with status `reconnecting` and no snapshot finds
header and content agree; `ReviewQueuePanel` tests assert the terminal's
empty-queue sentence and its date form past a month, with the comment and
code agreeing.

### UX-123 Forge links, story stages and two controls tell less than their siblings

Impact: low · Effort: small

**Today.** jsx-a11y strict and axe A/AA pass, since each control has some
name; what each says at rest, or in its name, is less than its neighbors
say.

- In the Review section `PullRequestSummary`'s title link opens the forge
  in a new tab (`target="_blank"`,
  `web/src/features/review/ReviewPanel.tsx:124`) with `{pull.title}` as
  its whole accessible name (`:128`) and only `underline-offset-4
  hover:underline` for a class (`:126`): no `text-primary`, no icon, no
  new-tab note, no focus-visible ring. The Issue row's link (`IssueRow`,
  `:179`) and each CI check's name (`CheckRow`,
  `web/src/features/review/Checks.tsx:59`, the link at `:69`) are the
  same, and a check without a URL is bare text (`:67`) that looks
  identical. The
  Summary's activity links (`ActivityLine`,
  `web/src/features/summary/ActivityList.tsx:114`) are underlined and
  ringed but carry no new-tab note either. On the Review screenshots
  (`1440-dark-review.png`) the heading "#128 fix: redact tokens…" and the
  check rows look like static text; a focused forge link falls back to
  the browser's default outline; a screen-reader user activating any of
  them is moved to a new tab unwarned. The page's other three outbound
  links carry all of it: the queue's Open (`RequestRow`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:325`), Open in Jira
  (`IssuePeople`, `web/src/features/issues/IssueDetailPanel.tsx:129`) and
  the task's issue link (`web/src/features/tasks/TaskDetail.tsx:135`),
  each with the `ExternalLink` icon and the sr-only "(opens in a new
  tab)".
- Each work-story stage is a button that calls `setSection` (`WorkStory`,
  `web/src/features/issues/WorkStory.tsx:318`), yet it is styled only with
  `hover:bg-accent` and a focus ring (`:323`) and its sr-only span carries
  the state alone (`:326`): nothing at rest or in its name says it
  navigates, though `docs/content/docs/web.md:122` promises under "Issues"
  each "step opening the section it belongs to", so the story reads as a
  plain timeline in `1440-dark-issues.png`.
- The Settings `<form>` has `onSubmit` and a `className` only
  (`ConfigForm`, `web/src/features/settings/SettingsPanel.tsx:175`), no
  `aria-label` or `aria-labelledby`, so it is not a form landmark while
  the commit and pull request forms are, and
  `getByRole('form', { name: /settings/i })` cannot resolve the site's
  largest form.
- `ThemeToggle`, icon-only at every width, carries its name in
  `aria-label` alone (`web/src/shell/ThemeToggle.tsx:24`) with no `title`,
  where `NavRail`'s buttons show theirs on hover (`title={name}`,
  `web/src/shell/NavRail.tsx:33`); a mouse resting on the toggle shows
  nothing.

**Instead.** Draw the title, issue and check-name links as the queue draws
Open — `text-primary`, the `ExternalLink` icon, the sr-only new-tab note
and the focus-visible ring — so a check with a URL differs visibly from
one without, and give the activity links the note; mark each stage as a
control at rest inside the system (the title in `text-primary`, or a
trailing chevron in the muted foreground) and give it an sr-only suffix or
`aria-describedby` naming the section it opens; `aria-labelledby` on the
Settings form pointing at the section's h1; a `title` on the toggle equal
to the choice it announces ("Theme: System").

**Done when.** `web/src/features/review/ReviewPanel.test.tsx` finds the
title link, the issue link and each check with a URL by role link with a
name ending "(opens in a new tab)" and `target="_blank"`, and a check
without a URL as text; a `WorkStory` test finds each stage by role button
with a name or description matching /opens (branch|review|messaging)/i
and a screenshot of the Issues section shows a rest-state mark on each
stage; `screen.getByRole('form', { name: /settings/i })` resolves in
`web/src/features/settings/SettingsPanel.test.tsx`; a `ThemeToggle` test
asserts the button's `title` names the current choice and changes with
it.

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
padding and size scale, a row's facts apart, with the dot between them
hidden from assistive tech (`Meta.tsx`), a date in one locale
(`dates.ts`), and a read in flight or a failure (`Status.tsx`) as a
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

What is open here is GitHub- and terminal-shaped sentences, and
sentences that disagree. What each surface can
do is the table at the head of this file, and the names the three
surfaces share are the Vocabulary table's beside it.

The rule the last looks follow was chosen by the maintainer: **confirm
what leaves the machine or cannot be undone; a reversible local toggle
acts at once.** The parity rule beside it: the terminal and the web cover
the loop, each for browsing; the command line covers one-shot and scripted
work, with `--json`, and is not a browser.

### UX-126 Sentences that assume GitHub or the terminal, told elsewhere

Impact: low · Effort: small

**Today.** The no-token hint is fixed: `Resolve` wraps
`forge.ErrNoToken` with `Sources(kind, host)`
(`internal/forge/token.go:164`, `Sources` at `:252`), and the terminal
keeps that error in its own words (`forgeErrors`,
`internal/tui/failure.go:204`), so `doctor` and the interface now name the
same variable and tool for a GitLab host. The rest is still GitHub's or
the terminal's: a GitLab user reads "merge request" and `!7` on one line
and "pull request" and `#7` on the next, and a script is told to press a
key it does not have.

- `errNoCommitsToOpen`, `internal/cli/pr.go:26`, and `errPullAlreadyOpen`,
  `:30`: fixed "pull request" sentences that `composeRefusal` returns
  (`:347`, `:351`), while the same command's question uses
  `seams.Kind.Noun()` (`:211`) and its success line `Kind.Sigil()`
  (`:262`).
- `errNoPullRequest`, `internal/cli/announce.go:25`: fixed "pull request",
  wrapped at `:158`, though `announceSeams` carries `Kind` (`:37`) and
  uses it at `:168`.
- `renderReviews`, `internal/cli/reviews.go:103`: "No pull requests are
  waiting on your review." (`:105`) with no forge kind in reach —
  `reviewsSeams` (`:26`) carries none; `reviewLine`, `:118`, writes `#%d`
  before every number (`:124`).
- `docs/content/docs/scripting.md:94`, the `reviews` row of the stdout and
  stderr table: quotes that sentence verbatim, so the row moves with it.
- `Model.reviewQueueDetail`, `internal/tui/reviewqueue.go:163`: "pull
  requests" whatever the forge (`:166`, `:172`); `Model.reviewRows`,
  `:214`, writes `" #"` (`:220`) where the Review pane uses
  `m.vocab.sigil` (`internal/tui/review.go:270`).
- `prBodyHelp`, `internal/tui/prcomposer.go:31`: a fixed "Write the pull
  request description above this line", handed to `$EDITOR` by the
  composer (`:465`) and the editor (`internal/tui/preditor.go:123`).
- `rejectionReason`, `internal/messaging/post.go:205`: three sentences
  end "then press enter to try again" (`:212`, `:214`, `:215`) inside a
  domain error. `runAnnounce` wraps it with `%w`
  (`internal/cli/announce.go:189`), as does `postSummary`
  (`internal/cli/summary.go:208`), and `main`
  (`cmd/workflow/main.go:26`) prints it on stderr, key and all — against
  the interface's own rule, in the doc comment on `wording`
  (`internal/tui/failure.go:59`), that a full form names no key to press.
  The web, for its part, drops the reason and says "post from a
  terminal to see its reason" (`messagingFaults`,
  `internal/webserver/errors.go:351`).

**Instead.** Carry the forge `Kind` on `reviewsSeams` as `prSeams` and
`announceSeams` already do, and word every fixed "pull request" and `#`
through `Kind.Noun()` / `Kind.Sigil()` on the command
line and `m.vocab` in the terminal and the editor help, keeping the
sentinels for `errors.Is`. Keep `rejectionReason` to the fix
(`join #dev`) and leave the key out — the overlay's footer already offers enter
— and, once the sentence names no key, let the web's detail carry it
rather than sending the user to a terminal. Change the `reviews` row of
scripting.md with it.

**Done when.** On a GitLab remote, tests of `pr`'s two refusals,
`announce`'s refusal, `reviews` (the empty-queue note on stderr, `!`
before each number on stdout), the Reviews pane and the composer's
`ctrl+o` help all see "merge request" and `!` and never "pull request" or
`#`; an `announce`
test whose fake Slack answers `not_in_channel` sees the fix on stderr and
no "press enter"; `docs/content/docs/scripting.md:94` matches the new
`reviews` note.

### UX-130 Four sentences that disagree with a neighbor or a sibling surface

Impact: low · Effort: small

**Today.** Four things are said two or three ways.

- `ErrDirtyTree`, `internal/loop/guards.go:17`: "the working tree has
  uncommitted changes"; `errDirtyTree`, `internal/tui/switchtask.go:25`,
  is a second sentence ("uncommitted changes — commit or stash them
  before switching branches"), and `errDirtyTree`,
  `internal/webserver/checkout.go:19`, a third ("…; commit or stash them
  before switching") — one guard, three sentences, so a user who meets the
  refusal in the browser and then in the terminal reads two, and a change
  to the guidance is made in three places.
- `announceTarget`, `internal/cli/announce.go:199`: "the configured
  SERVICE channel" for a webhook and for a bot with no channel, used by
  `runAnnounce` for the `to` line (`:176`), the dry-run line
  (`announcePrompt`, `:226`) and the done notice (`:192`) — where
  `Messaging.Target`, `internal/config/config.go:284`, says "(no channel
  set)" (`:287`) and "the channel its webhook is bound to" (`:292`), which
  the interface's notice uses (`internal/tui/messagingpreview.go:240`),
  and the web's announce says "Announced to SERVICE." for a webhook
  (`web/src/features/messaging/MessagingPanel.tsx:321`); `summary --post`,
  on the same command line, already says `Target()`'s words
  (`postSummary`, `internal/cli/summary.go:188`). With a user
  token and no channel the command line's preview claims a channel that
  does not exist, and the post then fails.
- `placeholder`, `internal/tui/fields.go:72`: returns `jira.DateLayout`
  (`:74`), Go's reference date `2006-01-02`
  (`internal/jira/fields.go:32`), as the hint, while `jira.ErrNeedsDate`
  (`internal/jira/fields.go:20`) says "must be a date like 2026-09-21" —
  the hint reads as a stale date rather than a shape.
- `Model.messagingDetail`, `internal/tui/messaging.go:160`: "SERVICE is
  not set up" and "to ~/" + `config.FileName` (`:162`), with no not-started
  mark beside it, where `loop.SetUpAdvice`'s
  `messaging.ErrNoCredential` wording (`internal/loop/summary.go:123`)
  words the same condition as "messaging has no credential", names
  `workflow slack login` as well, and names no file, beside the not-started
  mark every other not-set-up cause gets; `FileName`'s comment
  (`internal/config/config.go:15`) says the name serves both search
  locations.

**Instead.** Let `loop.ErrDirtyTree` carry the guidance once ("…; commit
or stash them before switching") and have both surfaces render it
through their failure voice, dropping the two local sentinels; drop
`announceTarget` for `seams.Messaging.Target()` on the `to` line, the
dry-run line and the done notice; one example in `placeholder` and
`jira.ErrNeedsDate` (from the fake-able clock, or a plain `YYYY-MM-DD` in
both); `config.FileName`
without the `~/`, letting the `messaging.ErrNoCredential` wording serve
both places as `loop.SetUpAdvice`'s guidance.

**Done when.** `grep -rn 'commit or stash'` finds one string outside
tests; `TestAnnounceDryRunComposesTheReadyMoment`
(`internal/cli/announce_test.go:51`) sees `Target()`'s wording and `grep
-rn announceTarget internal/cli` finds nothing;
`TestATransitionFillsAUserDateAndSeveralVersions` and
`TestADateFieldRefusesWhatIsNotADate` (`internal/tui/fields_test.go:41`,
`:80`) agree on one example; `TestTheSlackPaneNamesWhatItNeedsWhenUnset`
(`internal/tui/messaging_test.go:531`) refuses `~/` and the pane and the
failure wording name the same settings.

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
