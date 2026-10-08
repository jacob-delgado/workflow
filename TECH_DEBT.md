# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.

Two readers are in mind: a contributor looking for something worth fixing, and
a later Claude Code session asked to pick up an entry by its ID. Each entry
says what is wrong, where, what it costs, one way to fix it, and how to tell
when it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the
ideas; this file holds the debts.

This is the pre-1.0 audit edition: a full read of every surface at commit
`b9ab000` on 2026-10-07, ahead of 1.0. The entries below are to be paid down
in one PR before 1.0, the pre-1.0 paydown PR, and an entry is deleted from
this file in the commit that pays it. Numbering continues at DEBT-162 and an
ID is never reused: the 2026-09-24 edition's entries, DEBT-64, DEBT-65,
DEBT-71 and DEBT-75 to DEBT-161, were paid down in #141, #144, #145 and #146.

Checked against commit `b9ab000` on 2026-10-07. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **Every area read in full, by independent readers.** The command line
   (`internal/cli`, `cmd/`), the terminal interface (`internal/tui`), the web
   (`web/src`, `web/e2e`, `internal/webserver`, `api/openapi.yaml`), every
   other package under `internal/`, the gate scripts, the CI workflows and the
   docs were each read whole, with each unit's tests beside it. The standard
   was the one the project sets itself in [CLAUDE.md](CLAUDE.md), Effective Go
   and Google's Go style guide, the clig.dev guidelines, current React and
   TypeScript practice, and WCAG 2.1 AA for the web. A test whose Assert would
   pass whatever its Act did counts as a finding.
2. **Measurement, once, at `b9ab000`.** File lengths from
   `scripts/check-file-length.sh --list`; package budgets from
   `scripts/check-package-size.sh --list`; condition coverage from `task
   cover:branch` (gobco, 91.4% against a floor of 91%, with its per-condition
   worklist); unreachable code from `deadcode`; and the inventory of every
   `//nolint` and `eslint-disable` in the tree. An entry that cites a number
   cites the tool it came from, and its confidence reads *measured*.
3. **Every finding handed to a skeptic.** Each finding went to one independent
   reader whose task was to refute it; a medium or high one went to two, with
   a third to break a tie. A finding was dropped unless it survived, and a
   severity or a citation the skeptics corrected was written as corrected.
   Findings that described one problem were then merged, keeping the richest
   location and fix, and a problem found in several areas became one entry
   under [Across the surfaces](#across-the-surfaces).
4. **Every finding matched against the register.** A finding that only
   restated a trade-off whose reopen trigger had not fired was dropped.
5. **The register re-judged, entry by entry.** Each trade-off was kept, with
   its three paragraphs rewritten to what is true at `b9ab000`, or found worth
   paying down, or found with its reopen trigger fired. Each one paid down or
   fired has an entry below whose title starts "Pay down TRADE-n"; the
   register entry stays as it was until that entry is paid, and is deleted
   then.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness). This
  edition found nothing high.
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (independent readings of the code
  agree). This edition is read and measured.
- **Size** is S (an hour or two), M (a day) or L (several days, or a change
  across packages that CLAUDE.md asks to outline first).
- **Breaking** marks an entry whose fix changes a contract a user or a script
  depends on: a `--json` shape, the web API, or an exit status.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it is in [the trade-off
register](#the-trade-off-register), which lists the choices made on purpose,
what each costs, and what would reopen it.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule. This pass found 35; they are handled privately with the maintainer, and
nothing in this file says what they are. The entries below contain nothing
that helps anyone misuse a credential, a terminal, a file on disk, the
loopback server or a release.

## The command line

### DEBT-175 Pay down TRADE-18: commands take their working directory, home and environment from the caller

Severity: low · Confidence: read · Size: L

**Where.** `loadFromEnvironment` and `configHome` (`internal/cli/cli.go:402`,
`:418`), `inputFor` (`cli.go:251`, `NO_COLOR`), `whereInit`
(`internal/cli/config_cmd.go:141`), `connectLeniently`
(`internal/cli/connect.go:62`), `reportRepository`
(`internal/cli/doctor.go:157`), `repositoryFactsFor`
(`internal/cli/doctor_json.go:111`), `completeAssignedIssues`
(`internal/cli/scriptable.go:177`), `cleanLocalData`
(`internal/cli/dbclean_cmd.go:185`); `run` (`internal/cli/cli_test.go:24`),
`internal/cli/removed_workdir_test.go`.

**Today.** `Execute` takes arguments and streams but no environment, so
commands call `os.Getwd`, `os.UserHomeDir`, `os.Getenv` and `store.DefaultDir`
themselves; six `os.Getwd` calls word the same failure. TRADE-18 keeps six of
those Getwd arms reachable only on Linux. The same habit makes the cli tests
change directory and set the environment, so they cannot run in parallel:
`cli_test.go` says so, and `internal/cli` is the slowest package in `task
check` (about 38 s under `-race`). CLAUDE.md's D principle cites
`config.Load(workDir, homeDir)` as the pattern, which the CLI does not follow.

**Fix.** Add an environment value (working directory as `func() (string,
error)`, home, `Getenv`, state directory) to what `Execute` and
`NewRootCmdOver` take, built from the OS in `cmd/workflow/main.go`, and read
it through one `workingDir(cmd)` helper that wraps the failure once. Rewrite
`removed_workdir_test.go` to inject a failing working directory, in parallel,
with no `t.Chdir` and no macOS skip. Shrink TRADE-18 to `closeRequestLog`
alone, cited by name, and remove the six TRADE-18 site comments.

**Done when.** `rg 'os.Getwd' internal/cli` finds only the production default,
the cli tests call `t.Parallel()`, `task cover:branch` on macOS lists none of
the six arms, and `go test ./internal/cli` wall time drops.

## The terminal interface

### DEBT-187 Every overlay repeats the close, step and scroll key code, and the job log skips the page keys

Severity: low · Confidence: read · Size: M

**Where.** `checkList.handleKey` and `jobLogView.handleKey`
(`internal/tui/checks.go:119`, `:388`), `helpOverlay.handleKey`
(`internal/tui/help.go:86`), `handleFormKey` (`internal/tui/fields.go:192`),
and the other overlays; `overlayKeys` (`internal/tui/overlay.go:63`).

**Today.** Fourteen overlays match the cursor keys by hand and thirty end with
`m.overlay = c`. The help overlay pages by half a page and is scrollable and
steppable; the job log, the overlay most likely to be long, handles only close
and the cursor keys, so pgup and pgdn do nothing there and its footer never
offers them.

**Fix.** Make `jobLogView` scrollable and steppable like the help overlay;
extract a helper for the shared close and cursor arms only where it does not
need per-overlay hooks.

**Done when.** A test pages a long job log with pgdn and sees it move, and the
job log's footer offers the scroll keys.

### DEBT-188 Every overlay stores its own copy of the marks and styles, and `lookAt` is bypassed

Severity: low · Confidence: read · Size: M

**Where.** `overlay.view` (`internal/tui/overlay.go:27`), `lookAt`
(`internal/tui/taskactions.go:231`), `previewPush` and `previewRebase`
(`internal/tui/branch.go:233`, `:245`), `previewRerun`
(`internal/tui/checks.go:201`), `askToRemove`
(`internal/tui/localdata.go:213`).

**Today.** `view(width, rows)` gets no rendering context, so 25 overlay types
carry the marks and styles, filled in 31 places with `marks: m.marks, styles:
m.styles`. `lookAt` exists to fill a last look's, but four sites build theirs
by hand.

**Fix.** Pass a small render context to `view` and drop the fields; until
then, move `lookAt` to `overlay.go` and use it at every last-look site.

**Done when.** `grep -c 'marks: m.marks, styles: m.styles' internal/tui/*.go`
is 0, or limited to overlays that are not last looks.

### DEBT-189 `keyContexts` mirrors dispatch by hand, so the key check can miss a conflict

Severity: low · Confidence: read · Size: M

**Where.** `keyContexts` and `keyContext.covers`
(`internal/tui/keycheck.go:153`, `:135`), the Repositories context (`:177`).

**Today.** `CheckKeys` decides conflicts from help groups plus a hand-kept
`alsoLive` list of actions a pane handles outside its group. Nothing derives
the list from the handlers, so a pane that starts answering another group's
key lets a conflicting `ui.keys` override pass. The Repositories context
already lists up and down, which its groups cover.

**Fix.** Build the contexts from each pane's list of offers
(`internal/tui/panes.go`); until then drop the redundant entries and add a
test that every binding a handler matches is covered by its context.

**Done when.** A test fails when a handler answers an action missing from its
key context.

### DEBT-190 `Model` is a god type: 431 methods, with every pane's behavior on one receiver

Severity: low · Confidence: measured · Size: L

**Where.** `Model` (`internal/tui/tui.go:36`), `settled`
(`internal/tui/taskoffers.go:22`), `branchLoaded.apply`
(`internal/tui/branch.go:39`), `taskIssueLine` and `branchAndPull`
(`internal/tui/tasks.go:415`, `:428`).

**Today.** `grep -c '^func (m Model)' internal/tui/*.go` sums to 431 (tasks.go
29, detail.go 28, taskactions.go 23, review.go and commits.go 22 each), over
about 35 fields. Pane states such as `tasksState` and `reviewQueueState` are
mostly plain data whose behavior lives on `Model`, so any pane reads and
writes any other's fields, and core plumbing (`settled`, which every Update
route ends in) sits in a feature file. A reader cannot tell what a pane
depends on. Value receivers and one package are deliberate and stay.

**Fix.** Pane by pane, move behavior that reads only one pane's state onto its
state type, with the few cross-pane facts as arguments, leaving `Model` to
route and compose; move `settled` beside `Update`. Do not split the package.

**Done when.** The method count falls well below 431 (for example under 250),
and the Tasks, Reviews and Summary renderers have state-type receivers.

### DEBT-191 Fifteen files in `internal/tui` are past the 500-line soft target, one near 800

Severity: low · Confidence: measured · Size: M

**Where.** `taskactions.go` (738), `tasks.go` (695), `setupform.go` (665),
`people.go` (576), `reposwitch.go` (535), `branch.go` (520), `failure.go`
(519), `render.go` (510), `commits.go` (510), `settingsfields.go` (508),
`tagging.go` (506), `review.go` (503), `picker.go` (502), with `detail.go`
(494) and `keys.go` (491) close; all under `internal/tui/`.

**Today.** Most carry more than one concern: `branch.go` holds the whole
`branchCreator` overlay; `failure.go` a 220-line error-wording catalog between
`sendState` and the renderers; `detail.go` the Issues list's search, paging
and footer and the shared `age()`; `picker.go` an unrelated `fixupPicker`;
`reposwitch.go` the `dirPrompt`; `taskactions.go` the `taskLine` overlay and
issue tracking. `taskactions.go` is 62 lines from the 800-line ceiling that
fails the gate, so the next Tasks feature forces an unplanned split. The
package holds 64 files under its cohesive ceiling of 80.

**Fix.** Split file per concern inside the package: `branchcreator.go`,
`errorwords.go`, `issuesearch.go`, `fixup.go`, `dirprompt.go`, `taskline.go`,
`tasktrack.go`, `footer.go`, `setupsteps.go`, under the package's cohesive
ceiling.

**Done when.** `scripts/check-file-length.sh --list` flags none of these files.

### DEBT-192 The frame keeps two copies of the border glyphs, and the ASCII focus corners disagree

Severity: low · Confidence: read · Size: S

**Where.** `glyphs` (`internal/tui/frame/frame.go:54`, with a
`//nolint:exhaustive` at `:55`), `railSide`, `railHorizontal`, `railCorners`
(`:227`, `:241`, `:305`), `top` and `ruleLine` (`:144`, `:296`).

**Today.** `Render` reads glyphs from a map that leaves Light out (hence the
nolint); `Rail` spells the same characters again as switches on two bools;
`top` and `ruleLine` are one function. In ASCII a focused `Render` box has `#`
corners and a focused rail pane `+` corners with `#` sides.

**Fix.** One `borders` value per style (Light in the map, nolint gone), the
rail functions taking a `borders` value, and `top` calling `ruleLine`.

**Done when.** The nolint is gone and a frame test asserts a focused ASCII
rail and a focused `Render` share corners.

### DEBT-193 `layout`'s exported functions take four interchangeable ints

Severity: low · Confidence: read · Size: S

**Where.** `Compute`, `ComputeWithNotice`, `NoticeKeepsFocus`
(`internal/tui/layout/layout.go:74`, `:102`, `:212`).

**Today.** Each takes `(width, height, railPanes, focused int)`, so a call
with width and height, or panes and focus, swapped compiles and fails only at
run time.

**Fix.** Take a `Terminal{Width, Height}` and a `Rail{Panes, Focused}`.

**Done when.** The exported functions take named struct values and their call
sites in `internal/tui` use them.

### DEBT-194 The central applier list claims every message but names 55 of 76

Severity: low · Confidence: read · Size: S

**Where.** The assertion block (`internal/tui/overlay.go:106`).

**Today.** It is headed "Every message the interface waits for is an applier."
but lists 55 types; 21 message types with an `apply` method are missing (among
them `announcesLoaded`, `branchLinked`, `dirLooked`, `reviewersRead`,
`tasksLoaded`), each already asserted beside its definition, and five are
asserted in both places.

**Fix.** Delete the central block and keep one `var _ applier = T{}` beside
each definition.

**Done when.** Every type with `apply(Model)` has exactly one assertion beside
it, and `overlay.go` holds no partial list.

### DEBT-196 Pay down TRADE-6: one field ring for the composers, the calendar and the pane switch

Severity: low · Confidence: read · Size: S

**Where.** `commitComposer.onFieldNav` and `scopeCanComplete`
(`internal/tui/scopesuggest.go:18`, `:30`), `prComposer.onFieldNav` and
`baseCanComplete` (`internal/tui/prcomposer.go:380`, `:396`), the calendar's
field keys (`internal/tui/calendar.go:229`), the type cycle
(`internal/tui/composer.go:289`), the pane switch (`internal/tui/tui.go:294`).

**Today.** TRADE-6's trigger has fired: the Summary's calendar binds both
next-field and previous-field and cycles a focus index. The wrap arithmetic
`(index + n - 1) % n` is now written five times, the two `onFieldNav` twins
have diverged (one handles previous-field outside it), and the two
`*CanComplete` checks are one function over different inputs.

**Fix.** Add a small value type, `ring{at, size}` with `next()` and `prev()`,
used by both composers, the calendar, the type cycle and the pane switch;
replace the two checks with one `completesOnTab(textinput.Model) bool`; give
both composers the same `onFieldNav`. Paying this closes TRADE-6: delete the
entry and its two site comments.

**Done when.** No `- 1) %` remains in `internal/tui` outside the helper, the
composer, calendar and pane-switch tests pass unchanged, and TRADE-6 is gone.

### DEBT-197 Pay down TRADE-12: `tui.Run` takes its input from the caller

Severity: low · Confidence: measured · Size: S

**Where.** `Run` (`internal/tui/tui.go:178`), the `RunInterface` type and its
call (`internal/cli/cli.go:177`, `:251`), the stand-in in
`internal/cli/root_test.go:56`.

**Today.** `Run` takes an output writer but not its input, so Bubble Tea reads
`os.Stdin`, and the CLI passes `cmd.OutOrStdout()` but never
`cmd.InOrStdin()`, against Cobra's own idiom. That is the only reason no test
can drive `Run`, and the reason gobco reports `tui.go:182` as never evaluated.

**Fix.** Take `in io.Reader` and pass `tea.WithInput(in)`; have the CLI pass
`cmd.InOrStdin()`. Test a canceled context (the error wraps `context.Canceled`
and starts "running the interface") and a quit key on an empty model. Paying
this closes TRADE-12: delete the entry and its site comment.

**Done when.** gobco no longer lists `tui.go:182` and `task check` is green.

## The web

### DEBT-201 Pay down TRADE-10: the server ships the loop's stages

Severity: medium · Confidence: read · Size: M

**Where.** `progress.Stages` (`internal/progress/progress.go:118`),
`onHeadStages` (`web/src/features/issues/WorkStory.tsx:122`),
`web/src/features/issues/WorkStory.stages.test.tsx`, the snapshot builder in
`internal/webserver`, `Snapshot` in `api/openapi.yaml`.

**Today.** The stage rules are written in Go and again in TypeScript, and the
copies have drifted in shape: Go derives five stages (Issue, Branch, Commits,
Review, the service) and the web four, with no Issue stage, which is
TRADE-10's "one surface shows a stage the other does not". Branch is decided
differently (Go reads `OnFeatureBranch`, the web `branch.name !== ''`), and
uncommitted changes make Commits in flight only in Go. The paired cases cover
the review, changes-done and announce rules only. The server already holds
every input.

**Fix.** Add a required `stages` array to `Snapshot` (name, system, state),
run `task gen` and `yarn gen`, and fill it from `progress.Stages` in the
snapshot builder. Have `onHeadStages` read it, keeping only per-issue
presentation in the web. Move the rule cases into webserver snapshot tests,
and keep a web test that renders whatever stages arrive. Paying this closes
TRADE-10: delete the entry and its site comments (`progress.go:14`,
`WorkStory.tsx:37`).

**Done when.** The web derives no stage state, `rg 'reviewReached' web/src`
finds nothing, and `task check` is green.

### DEBT-216 The forge's kind is inferred from the display noun "merge request"

Severity: low · Confidence: read · Size: S

**Where.** `useTrackerName`
(`web/src/features/issues/IssueDetailPanel.tsx:315`), `gitLabNoun`
(`web/src/features/issues/CommentComposer.tsx:384`), `Health` in
`api/openapi.yaml` (`:3660`).

**Today.** Both compare `health.forge_noun`, a word meant for display, with
"merge request" to decide whether the forge is GitLab; anything else is named
GitHub. A wording change or a third forge mislabels the tracker link and the
GitLab quick-action hints.

**Fix.** Add a `forge_kind` enum to `Health`, regenerate, and read it in both
places.

**Done when.** No `'merge request'` literal remains in `web/src` outside tests.

## The gates, the build and the tests

### DEBT-239 Pay down TRADE-19: the gate fails a never-run condition or an unseen error arm

Severity: medium · Confidence: measured · Size: L

**Where.** `scripts/gobco-report.sh` (`:208`), `BRANCH_COVERAGE_MIN`
(`Taskfile.yml:78`), TRADE-19; among the gaps:
`internal/wiring/tracker.go:107` (`jiraOnly`),
`internal/wiring/messaging.go:198` to `:202` (`asMessagingError`),
`internal/wiring/forge.go:98`, `internal/config/save.go:120`, `:217`, `:222`,
`internal/setup/setup.go:164`, `internal/tui/branch.go:457`,
`internal/tui/composer.go:307`, `internal/tui/fields.go:217`,
`internal/tui/commentcomposer.go:70`, `internal/store/kept.go` (`:63` to
`:234`).

**Today.** TRADE-19's triggers have fired: the measured share is 91.4% against
a floor of 91%, already lowered from 92 in #166, and its premise is false. It
says 280 conditions are seen one way, "none of them an `err != nil` or a
condition that never ran"; the report at `b9ab000` lists 967, 262 of them `err
!= nil` and 73 never evaluated (36 in `internal/tui`, about 45 conditions in
`internal/wiring` against the four it records). Among them: the guard that
keeps a Jira-only write off a forge key, the classification of Slack
credential failures, the paste handlers of the branch creator, the commit
composer and a transition's text field, and replacing a comment draft. The
worklist-only policy gives no signal when a change adds an untested error arm,
so the share erodes one arm at a time.

**Fix.** Have `gobco-report.sh` fail on any never-evaluated condition and any
`err != nil` never seen true, unless its file and function are in a checked-in
`scripts/gobco-allowlist.txt`, one line per arm a TRADE entry keeps, with its
ID (checked by `check-tradeoffs.sh`). Seed it from the arms TRADE-13,
TRADE-14, TRADE-16 and TRADE-18 keep after their paydowns, and write tests for
the rest, starting with the never-evaluated ones. Raise `BRANCH_COVERAGE_MIN`
to floor(measured) − 2 once paid. Rewrite TRADE-19 to cover only
boolean-domain conditions seen one way, with counts the gate prints rather
than prose. CLAUDE.md's "a worklist of missing test cases, not a percentage to
chase" then names the allowlist as the exception list.

**Done when.** The gate fails a new untested `err != nil` arm, and the
measured share is at least two points above the floor.

### DEBT-243 Concurrency tests synchronize on wall-clock time

Severity: low · Confidence: read · Size: M

**Where.** `patience` and `drainPast` (`internal/tui/harness_test.go:108`,
`:113`), `internal/webserver/runs_test.go:737`,
`internal/webserver/stream_test.go:245`,
`internal/webserver/announcequeue_test.go:333`,
`TestConcurrentReadsOfAChannelShareOneSetOfRequests`
(`internal/wiring/slackdirectory_test.go:474`).

**Today.** `drainPast` drops any command not answered within 200 ms, assuming
only a held seam is that slow. A run test sleeps 50 ms hoping a stage claimed
the index; a stream test sleeps 40 ms hoping a frame follows a save; the
single-flight test sleeps 50 ms hoping the second reader joined, and passes
against a cache-after-completion implementation when it did not. The module is
on Go 1.27 and no test uses `testing/synctest`; `internal/webserver`'s tests
take 213 s under `-race`.

**Fix.** Have the fakes signal on channels (a held seam reports it is
blocking; the directory fake signals the second arrival), drop only commands
blocked on an armed hold, and use `synctest` bubbles for stream and queue
timing.

**Done when.** `time.Sleep` in these packages' tests is only seam-internal
hold time, and `go test -race -count=20` passes for them.

## The docs

### DEBT-278 The trade-off register's text no longer matches the code or itself

Severity: low · Confidence: read · Size: S

**Where.** TRADE-18 in this file; the register's ordering (TRADE-28 between
TRADE-23 and TRADE-24); `scripts/check-tradeoffs.sh`.

**Today.** The entries this edition keeps were rewritten to `b9ab000`, but one
left until its paydown still cites what the code no longer has. TRADE-18 names
`targetDir` (now `whereInit`), places `connectLeniently` in `cli.go` (now
`connect.go`) and `repositoryFactsFor` in `doctor_json.go` (now `doctor.go`),
counts `reportRepository`, which no longer reads the working directory, among
seven conditions that are now six, and cites lines that have all moved.
`check-tradeoffs.sh` checks IDs, not the files and lines an entry cites, so
nothing catches this.

**Fix.** When each paydown above lands its entry is deleted; until then cite
symbols, not lines, and sort the register by ID. Optionally have
`check-tradeoffs.sh` check that every `file:line` an entry cites holds a
matching site comment.

**Done when.** No entry names a function that does not exist, and the register
is in ID order.

### DEBT-279 The docs describe the dry run's store three contradictory ways

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/usage.md:862`, `:508`, `:760`,
`docs/content/docs/configuration.md:999`, CLAUDE.md's store rules;
`openInterface` (`internal/cli/cli.go:345`), `keptReads`
(`internal/tui/dryrun.go`), `readKeptAsItIs` (`internal/store/kept.go:108`),
`version_test.go:142`.

**Today.** usage.md's Dry run section says it "opens no store", and
configuration.md that neither surface opens `kept.db` "at all"; the code binds
owner links, groups, last groups and favorites to the read-only store, as
usage.md's own lines 508 and 760 say. configuration.md names `workflow.db-wal`
and `-shm` as what a `kept.db` read leaves. CLAUDE.md says the read-only open
"neither checks, stamps nor changes" either file, but a `kept.db` at another
version is checked and read as empty, while `workflow.db` is read as it is on
purpose (pinned by a test). Nothing is written in any case.

**Fix.** Rewrite both docs passages to match: the interface reads `kept.db`
read-only when it exists, opens no cache, and never makes or changes either
file; name the companion files after the file read. In CLAUDE.md say: "A
`--dry-run` store's read-only open reads `workflow.db` as it is and a
`kept.db` at another version as empty, never stamps or changes either file,
and never makes one that is missing."

**Done when.** `grep -rn 'opens no store\|opens it at all' docs/content` finds
nothing and the three passages agree with `openInterface`.

### DEBT-280 CLAUDE.md's layout omits `internal/taskwarrior` and `internal/slackauth`, and kept favorites

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md's layout block and first paragraph, its `kept.db`
sentence, `internal/config/store.go:6`,
`docs/content/docs/configuration.md:147`, ARCHITECTURE.md's package table
(`:166`).

**Today.** Every directory under `internal/` is in the layout but these two;
`slackauth` holds a credential (the Slack user token), so it is invisible to
someone looking for where secrets live, and Taskwarrior is a fourth
integration the summary does not name. The `kept.db` sentence leaves out
favorite directories, and `config.Store`'s doc and the `store.disabled` row
say disabling the store drops only conveniences, though it also drops owner
links, groups and favorites. Nothing checks the layout against the
directories.

**Fix.** Add `internal/taskwarrior/   finding Taskwarrior and reading and
changing its tasks` and `internal/slackauth/   the Slack user token:
refreshing, storing, locking`, name Taskwarrior in the first paragraph, and
add favorites to the `kept.db` sentence; reword `config.Store`'s doc and the
`store.disabled` row; add a small check, in `task lint`, that every Go package
directory is named in the layout.

**Done when.** The check passes and `grep favorite CLAUDE.md` matches the
`kept.db` sentence.

### DEBT-281 README gives nine panes the keys 1 to 7, lists seven web sections, and keeps writes in the terminal

Severity: low · Confidence: read · Size: S

**Where.** `README.md:239`, `:256`, `:258`; `web/src/shell/sections.ts:22`,
`docs/content/docs/web.md:42`, `:147`, `:248`, `:586`.

**Today.** README says nine panes but keys `1`–`7` (the TUI binds 1 to 9),
lists seven web sections (there are nine, with Summary and Repositories), and
says re-running CI, merging, finishing a branch and most issue writes stay in
the terminal, where web.md documents them all; only the task offers stay
terminal-only.

**Fix.** Write `1`–`9`, list the nine sections, and replace the sentence with
the one remaining gap or a link to web.md.

**Done when.** README has no "`1`–`7`" and no "seven sections", and its
terminal-only sentence matches web.md.

### DEBT-282 Status, install and release prose is copied across four documents and has drifted

Severity: low · Confidence: read · Size: M

**Where.** `README.md:23` and `docs/content/_index.md:12` (the Status lists),
the taglines (`README.md:20`, `docs/content/_index.md:8`,
`CONTRIBUTING.md:4`), the platforms (`CONTRIBUTING.md:223`,
`docs/content/docs/install.md:67`, `RELEASE_PLATFORMS` in `Taskfile.yml:94`),
`CONTRIBUTING.md:161`, `docs/content/docs/contributing.md:14`.

**Today.** The site's Status list lacks the forge-issues, summary and reviews
bullets and describes `config init` differently; the taglines drop the plain
webhook; the platforms are written by hand beside their gate's source;
CONTRIBUTING's release step omits the SBOM `install.md` and `release.yml`
name; and CONTRIBUTING says a subject "under 72" where the hook allows 72.

**Fix.** Make `_index.md` the one Status source with README linking to it;
name the platforms by linking to the release; keep setup in CONTRIBUTING with
the site linking to it; say "at most 72".

**Done when.** Each fact appears in one hand-written place and no second copy
of the Status bullets exists.

### DEBT-283 Reference pages carry release history the CHANGELOG already keeps

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/usage.md:242`,
`docs/content/docs/configuration.md:666`, `:531`; `CHANGELOG.md:201`, `:370`.

**Today.** The pages say `9` "is new with the Repositories pane" and a map
that "worked before … now stops workflow from starting", and that "before, an
unknown GitLab reviewer … stopped the merge request". The rule is stated
beside each, and the history is in the CHANGELOG.

**Fix.** Delete the history clauses and keep the rules.

**Done when.** `grep -rn 'worked before\|before, an unknown\|is new with'
docs/content` finds nothing.

### DEBT-284 configuration.md promises an earlier-build workspace migration the kept schema cannot hold

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/configuration.md:978`, `ownersSchema` and
`OwnerLinks` (`internal/store/owners.go:88`, `:129`), `keptSchemaVersion`
(`internal/store/kept.go:33`).

**Today.** The docs say links kept by an earlier build belong to no workspace
and an owner it marked not on Slack stays so everywhere. Every `slack_team` is
`NOT NULL` and part of its key, reads match the current workspace only, and an
earlier file reads as empty (TRADE-25), so neither can happen.

**Fix.** Delete the two sentences; the next paragraph already says an earlier
file reads as empty until `workflow db-clean --all`.

**Done when.** configuration.md has no "earlier build" sentence about
workspaces.

### DEBT-285 The key tables and command lists have small errors nothing checks

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/web.md:107` and `:346`,
`docs/content/docs/usage.md:127` (the key table, no `l`) and `:26` (the
command list), `docs/content/docs/scripting.md:63` (the family table),
`docs/content/docs/web.md:612` ("Five reads"); `showLog`
(`internal/tui/keys.go:346`), `internal/tui/keys_docs_test.go:23`, the
`ProblemCode` enum.

**Today.** web.md names the terminal's issue filter `p` (it is `f`) and the
review sort `s` (it is `O`); usage.md's table claims every key `?` lists but
has no `l` (show log), and its scriptable commands omit `repositories` and
`comment`; scripting.md's table of web families omits `too_long`,
`fetch_failed` and `check_failed`; web.md says "five reads" over a table of
ten requests, two of them writes. Only configuration.md's action list is
tested against the bindings.

**Fix.** Correct each; extend `keys_docs_test.go` to check usage.md's table
and add a test that every `ProblemCode` appears in scripting.md's table;
replace the command list with a link to Scripting.

**Done when.** A test fails when a bound action's default key or a problem
code is missing from its table, and the web.md passages agree with their
tables.

### DEBT-286 Gate texts say "two build-tagged twins", the golangci comments misname what is off, and the container docs misplace the release build

Severity: low · Confidence: read · Size: S

**Where.** `scripts/gobco-report.sh:44`, `:53`, `Taskfile.yml:27`, `:35`;
`.golangci.yml:18`, `CONTRIBUTING.md:124`;
`.github/workflows/container.yml:3`, `build/Dockerfile:1`,
`.github/workflows/release.yml:55`.

**Today.** `UNANALYZABLE` names one package, yet gobco-report's header and the
Taskfile speak of two. `.golangci.yml` says the deprecated linters are
"superseded by the _v5 linters enabled above" where both generations are
disabled, and CONTRIBUTING says every linter is enabled though five are off.
The container workflow says the release binaries are built in the container;
`release.yml` builds them on the runner with mise.

**Fix.** Name the one package; reword the golangci comment and CONTRIBUTING
("every linter but the few `.golangci.yml` disables, each with its reason");
say the container runs the gate on demand and weekly.

**Done when.** `grep -rn 'two build-tagged\|Two packages are there'
Taskfile.yml scripts` finds nothing, and no file claims the release is built
in the container.

### DEBT-287 configuration.md says any `NO_COLOR` value turns color off, but an empty one does not

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/configuration.md:625`, `DrawColor`
(`internal/config/ui.go:59`).

**Today.** `DrawColor` keeps color for `NO_COLOR=` (as no-color.org asks),
while the doc and the function's comment say "any value".

**Fix.** Say "a non-empty value" in both.

**Done when.** The doc and the comment say non-empty.

### DEBT-288 The `loop` package doc names the wrong import set

Severity: low · Confidence: read · Size: S

**Where.** The package doc (`internal/loop/loop.go:12`), the depguard rule
(`.golangci.yml:58`).

**Today.** It lists seven packages; loop also imports `activity`, `codeowners`
and `taskwarrior`, all allowed by depguard.

**Fix.** Point the doc at the depguard rule rather than restating the list.

**Done when.** The doc lists no packages, or matches the rule.

## Across the surfaces

### DEBT-295 The forge's kind and group seams are fixed at start-up though forge settings are live

Severity: low · Confidence: read · Size: M

**Where.** `forgeDeps` (`internal/wiring/forge.go:74`), `groupMembersSeam` and
`isGroupSeam` (`:147`, `:169`), `useForgeSettings` (`:349`), `WebDeps`
(`internal/cli/web.go:93`), `branchOwners`
(`internal/webserver/people.go:227`).

**Today.** `forgeDeps` decides the kind once and binds `Kind`, `GroupMembers`
and `IsGroup` (nil unless GitLab) from it, while a web Settings save replaces
the live settings for every connect. Switching to GitLab in Settings leaves
`IsGroup` nil, so top-level groups are tagged as people until restart; the
group cache is keyed by name only and survives a host change.
`Controls.UseForgeSettings` documents the opposite.

**Fix.** Decide the kind inside each closure from the live settings, as
`gitDeps` does, and key the cache by host and kind.

**Done when.** A wiring test starts with GitHub, applies GitLab settings and
sees `IsGroup` reach the fake GitLab endpoint.

### DEBT-307 The CLI imports the web server and the terminal package for logic every surface shares

Severity: low · Confidence: read · Size: L

**Where.** `notSetUpNote` (`internal/cli/status.go:372`), `runSummary`
(`internal/cli/summary.go:142`, `:182`), `internal/cli/repositories.go:53`,
`internal/cli/scriptable.go:382`, `internal/cli/doctor_requirements.go:42`;
`FaultDetail`, `ActivityReport` and `PostLength`
(`internal/webserver/activity.go:282`, `:149`, `:46`), `RepositoriesView`
(`internal/webserver/repositories.go:169`), `CheckKeys`
(`internal/tui/keycheck.go:48`).

**Today.** The CLI's not-set-up wording, its summary and repositories reports
and the summary's length come from the web server package, and its keymap
check from the TUI. Sharing the shape and the host-free wording is intended
(the `--json` output is the API's object); where it lives is the debt:
surface-neutral wording and report building sit in an HTTP package already at
its budget. (The `tui.Deps` bundle is TRADE-11.)

**Fix.** Move `FaultDetail`'s wording and the report builders beside
`internal/api` in a small report package both import, and `CheckKeys` with its
sentinels beside `config.UI`.

**Done when.** `internal/cli` imports `internal/webserver` only for serving
`--web`.

### DEBT-308 `WebDeps` copies about sixty seams one by one from `tui.Deps` to `webserver.Deps`

Severity: low · Confidence: read · Size: L

**Where.** `WebDeps` and the `with*` helpers (`internal/cli/web.go:70`,
`:135`), `webserver.Deps` (`internal/webserver/webserver.go:46`),
`TestWebDepsHandsTheServerEverySeam` and
`TestWebDepsHandsTheServerTheLenientSearch`
(`internal/cli/webdeps_test.go:16`, `:68`).

**Today.** Each new seam is an edit in three places, split into six helpers
only to pass funlen. The test catches a nil seam, not two same-typed seams
wired crosswise (Stage, Unstage and Discard; Branches and RemoteBranches;
Commit, Push, Rebase and RunHook), so each such pair needs its own test, as
the searches did.

**Fix.** Have `webserver.Deps` hold the same seam groups `tui.Deps` uses,
narrowed per group where interface segregation asks, or build both bundles
from one source in wiring.

**Done when.** `WebDeps` is under 25 lines and a seam added to a group needs
no edit in `internal/cli`.

### DEBT-323 Pay down TRADE-23: the server describes each review request's facets

Severity: low · Confidence: read · Size: M

**Where.** `internal/tui/reviewfacets.go:41`,
`web/src/features/reviewqueue/reviewFacets.ts:28`, `ReviewRequest` and
`ReviewQueue` in `api/openapi.yaml` (`:4119`), the reviews handler.

**Today.** Every facet input is a plain field of a request the server already
returns, so describing facets once per queue read costs nothing; TRADE-23's
objection is to filtering on the server, not describing. The facet rules
(labels, the CI order, draft before ready, sorting by name) are written twice,
and can already disagree: Go sorts by UTF-8 bytes and JavaScript by UTF-16
units. The twin pin is three TypeScript tests against nineteen Go ones.

**Fix.** Move the rules to an exported domain home (`FacetsOf`, `Label`,
`Offered`) the TUI calls; add `facets` to `ReviewRequest` and `facet_order` to
`ReviewQueue`, regenerate, and fill them in the handler; cut the TypeScript to
counting, toggling and admitting. Move ordering and label cases to Go. Paying
this closes TRADE-23: delete the entry and both site comments.

**Done when.** No facet word, order or label is spelled in `web/src`, and
`task check` and the web unit tests are green.

### DEBT-328 Pay down TRADE-29: the server ranks and describes each task

Severity: low · Confidence: read · Size: M

**Where.** `ByUrgency` to `ByPriority` and `Task.State`
(`internal/taskwarrior/order.go:72`, `:40`), `Task.Facets`, `Choices`,
`offeredFacets` and `Task.mentions` (`internal/taskwarrior/narrow.go`),
`orderedTasks` and `stateOf` (`web/src/features/tasks/taskOrder.ts:47`,
`:35`), `taskFacetChoices`, `matchesNarrowing` and `taskFacetLabel`
(`web/src/features/tasks/taskFacets.ts`), `Task` and `TaskList` in
`api/openapi.yaml` (`:4227`, `:4146`), `internal/webserver/tasks.go`.

**Today.** Six orders, the state as the list words it, five facet kinds with
their labels and offered order, and the typed-text match are written in Go and
again in TypeScript, about 360 lines on the web. TRADE-29's objection is to
asking the server for each order, not to the server describing each task: every
input is a field of a task the server already sends, and the read and all ten
task writes answer with the same list type, so ranks shipped with it apply at
once with no request and nothing to race. The copies already differ: Go has an
`unknown` state the web's `TaskState` lacks, and Go's `strings.ToLower` and
JavaScript's `toLowerCase` fold some letters differently (U+0130 becomes one
code point in Go and two in JavaScript), so the same typed text can match
different tasks.

**Fix.** Add to `Task` a required `state` (the list's word, from `Task.State`),
`facets` (kind, value, label, from `Task.Facets`), `ranks` (one integer per
order, the task's place in `ByUrgency` to `ByPriority`) and `searchable` (the
fields text matches, lower-cased by Go); add `facet_order` to `TaskList`, the
offered facets from `offeredFacets`; run `task gen` and `yarn gen`, and fill
them where the handler builds the list. Cut the TypeScript to sorting by
`ranks[order]`, counting and toggling facets, and an `includes` over
`searchable`. Since a wait passing changes a task's state with no event,
invalidate the tasks query at the earliest `wait` still ahead. Move the order
and facet cases to Go. Paying this closes TRADE-29: delete the entry and its
four site comments.

**Done when.** `rg 'priorityRank|naturalOrder|stateRank' web/src` finds
nothing, no facet label is spelled in `web/src/features/tasks`, and `task
check` and the web unit tests are green.

## Rules this audit changes

### DEBT-324 Declare compiled regular expressions at package level

Severity: low · Confidence: read · Size: M

**Where.** CLAUDE.md, *Design principles*, O — Open/closed; 41 non-test
`regexp.MustCompile` calls under `internal/`, among them `trailerLine`
(`internal/convention/convention.go:398`, called per body line), `issueKey`
(`:55`), `setOption` and `errexitOption` (`internal/hooks/generate.go`, per
script line), `summaryLine` (`internal/hooks/output.go:87`, per hook output
line), `versionPattern`, `createdTask`, `revertedOperations`
(`internal/taskwarrior/taskwarrior.go:315`, `write.go:19`, `:25`),
`pseudoVersion` (`internal/buildinfo/buildinfo.go:46`).

**Today.** The Open/closed note says `gochecknoglobals` makes a package-level
lookup map a build failure, and authors extended that to regexps, wrapping
each in a `func x() *regexp.Regexp` compiled on every use, some in per-line
loops. The pinned gochecknoglobals allows `regexp.MustCompile` globals, and
Effective Go and Google's style both declare them once.

**Fix.** Add to the Open/closed note: "A compiled regular expression is the
exception the linter itself makes: declare it once as a package-level `var
name = regexp.MustCompile(...)` rather than compiling it in a helper on every
call." Move every fixed pattern to a package-level var, keep the func form
only for patterns built from run-time input, and hoist those out of loops.

**Done when.** `grep -rn 'func .*\*regexp.Regexp' internal | grep -v _test`
returns only parameterized builders and `golangci-lint run` is clean.

### DEBT-325 Name the e2e suites in the definition of done

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Before declaring any task done* and the accessibility
paragraph; `check` (`Taskfile.yml:578`).

**Today.** CLAUDE.md says the axe scan and the layout spec fail on violations
and that `task check` must pass before work is done, but `task check` leaves
out `yarn test:e2e` on purpose (a browser, a port), and no task wraps it. A
web change can be called done locally with the accessibility floor never run;
CI catches it after.

**Fix.** Add `task e2e` wrapping `yarn test:e2e`, and change the rule to: "run
`task check`; for a change under `web/` or `internal/webserver` also run `task
e2e`, since `task check` leaves the Playwright suites out."

**Done when.** CLAUDE.md names the e2e task in the definition of done and the
task exists.

### DEBT-326 Mark the goroutine smell as gated

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Code smells*, Go-specific, "Premature
goroutines/channels"; `scripts/check-goroutines.sh`, `lint:goroutines`
(`Taskfile.yml:458`).

**Today.** The catalog marks a smell a linter catches, and lists this one as
judgment only, though a gate fails any `go` statement outside `internal/proc`
and the tests; a session writing a goroutine elsewhere learns the rule when
the build fails.

**Fix.** Change the bullet to: "*Premature goroutines/channels* — concurrency
with no measured need → simple synchronous code first; a bare `go` statement
is allowed only in `internal/proc` and tests (**gate**
`scripts/check-goroutines.sh`, `task lint:goroutines`)."

**Done when.** CLAUDE.md names `check-goroutines.sh` beside the smell.

### DEBT-327 Land Boy Scout improvements as their own commit

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Design principles*, Boy Scout Rule, and *Cross-cutting
conventions*, Commits.

**Today.** The Boy Scout Rule makes an improvement in every touched file part
of a feature or fix commit's definition of done, while the commit rule asks
for one logical change per commit and main merges by rebase, so each commit
lands as written. Read together, a fix commit carries unrelated cleanups, and
the bug-fix commit CLAUDE.md calls "the most valuable artifact" is harder to
read.

**Fix.** Append to the Boy Scout bullet: "Land the improvement as its own
commit (`refactor:`, `style:` or `chore:`) on the same branch, before or after
the feature or fix commit, so each commit stays one logical change."

**Done when.** CLAUDE.md says where the improvement goes and the two rules no
longer conflict.

## The trade-off register

These were chosen on purpose. They are not debt; they are listed because each
one costs something a reader should know about. Each entry carries an ID that
is never reused, when it was decided, what it costs, and the event that would
reopen it, and the code it keeps carries a `Trade-off TRADE-n:` comment naming
it. An audit or a review drops a finding that restates an entry here unless
that entry's reopen trigger has fired. `scripts/check-tradeoffs.sh`, in `task
lint`, fails an entry missing its ID or one of its three fields, and a site
comment naming an ID the register does not hold.

The pre-1.0 audit re-judged every entry on 2026-10-07. TRADE-3, TRADE-4,
TRADE-5, TRADE-8, TRADE-9, TRADE-11, TRADE-13, TRADE-14, TRADE-24, TRADE-25,
TRADE-26 and TRADE-30 to TRADE-34 were kept and rewritten to what is true at
`b9ab000`. TRADE-16 was paid down to the two calls it now names, and
TRADE-21 and TRADE-28 to two copies held to one shared case file, and each
stays. TRADE-6, TRADE-10, TRADE-12, TRADE-18, TRADE-19, TRADE-23 and TRADE-29
are to be paid down by the entries above whose titles name them, and each
stays here, as it was, until its entry is paid.

TRADE-27, a top-level GitLab group linking to Slack like a person, was closed
in #166: a bare CODEOWNERS name is now asked of GitLab when tags are composed,
and a group links to a Slack user group. A bare name decided as a person
before GitLab was asked, as every one was, is asked about again as a team once
GitLab knows it as a group; one GitLab cannot be asked about stays what it was
decided as. Its ID is not reused.

### TRADE-3 The web is read-only under `--dry-run`

**Decided.** Under `--dry-run` the web is read-only. `refuseWritesInDryRun` in
`internal/webserver/guard.go` answers every unsafe method 403 before a handler
runs. The browser reads `dry_run` from `getHealth`, shows a banner, and holds
each write in `web/src/api/client.ts` before it is sent, so the 403 is only
the backstop. One gate over every write is what guarantees nothing is written;
simulating each write would mean a second, per-endpoint narration beside the
terminal's. Recorded on 2026-09-24 in the audit (#140), and kept on 2026-10-07
in the pre-1.0 audit.

**Cost.** A web write under dry run is refused, not simulated, so the page
cannot show what the write would have done the way the terminal's narration
does.

**Reopen when.** A dry-run preview of a web write is asked for, in a
FEATURES.md entry or a user's report.

### TRADE-4 `staleTime: Infinity`, with the event stream as the sole freshness source

**Decided.** The query client defaults to `staleTime: Infinity` and no refetch
on focus (`web/src/queryClient.ts`). The event stream pushes the cockpit's
state into `useSnapshotStore`, so a cached read needs no refetch of its own; a
read the stream does not carry sets its own `staleTime` beside it (`freshFor`,
0, or `directoryHold`). The stream's connection state (`reconnecting`,
`stale`) is shown, so a dropped stream is visible. A configuration save checks
the file's revision and refuses a changed file with 409; a change landing
between that check and the write is not caught (`SaveOver`,
`internal/config/save.go`), which is accepted for one user's local file.
Recorded on 2026-09-24 in the audit (#140), and kept on 2026-10-07 in the
pre-1.0 audit.

**Cost.** A connection that stays open but stops sending frames looks live,
and leaves stale data with no refetch to fall back on.

**Reopen when.** A stalled-but-open stream is reported, or a panel is reported
showing stale data that neither the stream, its own `staleTime` nor a save
reads again.

### TRADE-5 The progress spine's per-system hue is color-only

**Decided.** On the progress spine each stage's glyph is drawn in its system's
hue (Jira, git, forge, messaging), inside `Model.stages` in
`internal/tui/spine.go`. The hue only repeats what the text says: the stage's
name, or its initial when compact, names the system, and the glyph's shape
carries its state. The hues are part of UX.md's visual system, which the web's
tokens share. Recorded on 2026-09-24 in the audit (#140), and kept on
2026-10-07 in the pre-1.0 audit.

**Cost.** A monochrome or color-blind reader loses one redundant channel
telling the systems apart.

**Reopen when.** UX.md's visual system changes, or an accessibility report
names the spine's hue.

### TRADE-6 The two composers' field handling is written twice

The commit and pull request composers each pair an `onFieldNav` with a
`*CanComplete` check (`commitComposer.onFieldNav`,
`internal/tui/scopesuggest.go:20`; `prComposer.onFieldNav`,
`internal/tui/prcomposer.go:370`), and each blurs every field before
focusing one (`commitComposer.focusOn`, `internal/tui/composer.go:297`;
`prComposer.focusOn`, `internal/tui/prcomposer.go:391`). Two is not yet the
rule of three, so they stay apart until a third composer needs them.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** A change to field navigation is made twice, and a third composer
must copy the pairs or extract them then.

**Reopen when.** A third overlay binds the next-field and previous-field
keys (`keys.nextField`, `keys.prevField`), which makes it a third composer.

### TRADE-8 The web's branch floor is v8's range-based count

**Decided.** The web's coverage gate is Vitest's v8 provider, with thresholds
in `web/vitest.config.ts` held at floor(measured) − 2. v8 counts a branch
covered once its code range has run, and no Vitest provider reports both
outcomes of each condition, so the web cannot have gobco's per-condition
floor. Recorded on 2026-09-24 in the audit (#140), and kept on 2026-10-07 in
the pre-1.0 audit.

**Cost.** An `a && b` only ever seen with `b` true still passes the web's
branch floor.

**Reopen when.** A web coverage tool reports per-condition counts the gate
could read, or a web defect ships through a condition only ever seen one way.

### TRADE-9 A refused push's detail keeps the host names git prints

**Decided.** When a push from the web fails, its problem detail keeps git's
own lines so the reason survives (`withoutAddresses`,
`internal/webserver/push.go`). Every URL, including any userinfo or token in
it, and every scp-style `user@host:path` is replaced with `<address>`, so no
credential in a remote reaches the page. A bare host name git prints, in
`Could not resolve host`, `connect to host` or `To host:path`, is kept: it is
the user's own remote and what they need to act on. The detail is answered
only to the page that pushed, on the loopback-bound, same-origin-guarded
server. Decided 2026-09-26 in #145, and kept on 2026-10-07 in the pre-1.0
audit.

**Cost.** The user's own remote host name can appear in a push's problem detail.

**Reopen when.** A report asks for host names to be taken out of a push's
detail, or a push's detail comes to be shown to, logged for, or stored for
anyone other than the user who pushed.

### TRADE-10 The loop's stage rules are written twice

Go's `internal/progress` (`progress.Stages`, read by the terminal's spine
and `workflow status`) and the web's work story
(`web/src/features/issues/WorkStory.tsx`, the TypeScript port) each derive
the stages. `web/src/features/issues/WorkStory.stages.test.tsx` pins the
same rules: a case there that has a twin in
`internal/progress/progress_test.go` carries its name. A CI not read counts
as none in Go, and a draft has no Go twin, since `progress.Work` carries no
draft.

**Decided.** 2026-09-26, in #145 (DEBT-139).

**Cost.** A change to a stage rule is made twice, and the two agree only as
far as the paired cases reach.

**Reopen when.** The snapshot is asked to carry the stages, or the two
drift: a rule in `progress.Stages` changes with no matching change to
`WorkStory.tsx`, a pair of same-named cases disagrees about a stage's
state, or one surface shows a stage the other does not.

### TRADE-11 The editor seam stays in the terminal package

**Decided.** `wiring.Deps` returns `tui.Deps` (`internal/wiring/wiring.go`),
and `cli.WebDeps` narrows it to `webserver.Deps`. Three of its fields speak
Bubble Tea: `Editor` (`EditorDeps`, whose functions take and return Bubble Tea
messages and commands because `internal/editor` hands the terminal over
through `tea.ExecProcess`), `After` and `Copy`. So the bundle is the
terminal's type rather than one in package `seams`. Decided 2026-09-25 in #144
(DEBT-71), and kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** The wiring's output is named for one surface, and the web reaches
its seams through the terminal's struct; a second surface that wants the
editor must import `internal/tui`.

**Reopen when.** A surface other than the terminal needs the editor, `After`
or `Copy`; `internal/editor` stops returning a Bubble Tea command; or the web
server needs to be built without importing `internal/tui` (today it also needs
`tui.CheckKeys`).

### TRADE-12 The terminal program's own start-up error goes untested

`tui.Run` (`internal/tui/tui.go:146`) starts the Bubble Tea program and
returns its error, the one condition `task cover:branch` reports as never
evaluated. No test calls `Run`: in a developer's terminal the program would
take over the terminal the tests run in, and with none it fails to open
one, so a test could only ever see it fail. What the interface draws and
does is tested through the `Model` that `Run` is given.

**Decided.** 2026-09-26, in #146.

**Cost.** The wording of a start-up failure ("running the interface: …")
is never checked, and the report keeps one never-evaluated condition.

**Reopen when.** `Run` comes to take its program or its input from the
caller for another reason, so a test could drive it, or a start-up failure
is reported with a message that does not say what went wrong.

### TRADE-13 Encoding the program's own types is taken not to fail

**Decided.** Every `json.Marshal`, `MarshalIndent` or `Unmarshal` on a value
the program builds keeps its `err != nil` arm, though none can be true: the
values hold only strings, integers, booleans, slices and maps of the same, or
values just decoded from JSON, and a `config.Config`'s JSON always fits
`api.Config`. Each site carries a `Trade-off TRADE-13:` comment, and `rg
'TRADE-13' internal` lists them (16 at `b9ab000`, in config, forge, jira,
messaging, slackauth, tui and webserver); lines are not listed here because
they drift. Decided 2026-09-26 in #146, and kept on 2026-10-07 in the pre-1.0
audit.

**Cost.** About sixteen error arms no test runs. Because `forge.newRequest`,
`jira.newJSONRequest`, `messaging.postJSON` and `config.encodeValue` accept
`any`, a caller passing a float, a channel or a failing marshaler would put an
arm in play with no test behind it.

**Reopen when.** A type encoded at one of these sites gains a float, a
function, a channel, an interface-typed field or a `MarshalJSON` of its own;
`config.Config` and `api.Config` disagree on a field's type; or an encoding
failure is reported.

### TRADE-14 The embedded OpenAPI contract is taken to load

**Decided.** `loadSpec` and `validate` (`internal/webserver/validator.go`) and
`Handler` (`internal/webserver/webserver.go`) keep five `err != nil` arms for
an embedded `api/openapi.yaml` that fails to load, validate or route. The
document is compiled in (`api/embed.go`), so only a build-time defect trips
them, and that defect fails every web server test first; a seam to feed a
broken spec would only re-test kin-openapi. Decided 2026-09-26 in #146, and
kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** Five arms no test runs; the start-up message for a broken contract
is read, not checked.

**Reopen when.** The contract is read from anywhere but the binary (a file, a
flag, a download), or a broken contract reaches a release.

### TRADE-16 Failures only a change between two calls can cause

Two calls follow one that has just read or made the same file, so they fail
only when the file system changes between the two, a race no test can hold
open without a seam. Opening the store at this build's schema version:
`removeDatabase` removes the files the open that just read the version held
(`internal/store/store.go`, checked in `remakeDatabase`), and `stamp` writes
the version into a file the open just made or read (checked in
`openCurrent`).

**Decided.** 2026-09-26, in #146; narrowed to these two calls in the pre-1.0
paydown, which made one call of each other pair.

**Cost.** Two error arms no test runs, so what each says when the race is
lost is read rather than checked.

**Reopen when.** One of these failures is reported, or a change to the calls
lets a test fail the second without the first.

### TRADE-18 Conditions only Linux's tests reach

Seven conditions are reached by tests that skip on macOS, so a report
measured there lists them as seen one way, and CI's, measured on Linux,
does not. Six ask whether `os.Getwd` failed, which it does on Linux once
the working directory is removed but not on macOS, whose `getcwd` still
names it: `connectLeniently` and `loadFromEnvironment`
(`internal/cli/cli.go:423`, `:477`), `targetDir`
(`internal/cli/config_cmd.go:140`), `reportRepository`
(`internal/cli/doctor.go:136`), `repositoryFactsFor`
(`internal/cli/doctor_json.go:110`) and `completeAssignedIssues`
(`internal/cli/scriptable.go:179`), each reached by
`internal/cli/removed_workdir_test.go`. The seventh, `closeRequestLog`
(`internal/cli/scriptable.go:113`), is reached through `/dev/full`, which
macOS lacks, by `TestRequestLogWarnsOnceWhenItCouldNotBeWritten`
(`internal/cli/reqlog_test.go`).

**Decided.** 2026-09-26, in #146.

**Cost.** `task cover:branch` on macOS reads seven arms fewer than CI does,
and lists seven conditions a reader there could take for untested.

**Reopen when.** CI measures condition coverage on another system, or
either test stops reaching its conditions on Linux.

### TRADE-19 The rest of the condition report stays gobco's worklist

Besides the conditions the entries above keep, `task cover:branch` lists
280 seen only one way, none of them an `err != nil` or a condition that
never ran. Six of them still check an error, spelled another way: the
`err == nil` after a pull request's reviews read
(`internal/forge/github.go:262`), GitLab's approvals read and re-run
(`internal/forge/gitlab.go:106`, `:411`), a link's parse
(`internal/messaging/post.go:344`) and a further page of issues
(`internal/tui/issues.go:123`), none of them ever seen false, and the
`errors.Is` asking whether the web's issue read failed for want of a
repository (`internal/webserver/handlers.go:88`), never seen true. They
stay out of this file: the report is their list, printed on every run,
and CLAUDE.md already reads it as "a worklist of missing test cases, not
a percentage to chase", so a test for one is written when the code around
it next changes, and `BRANCH_COVERAGE_MIN` keeps the share from falling.
By package, measured on macOS on 2026-09-27: `internal/tui` 152,
`internal/forge` 19, `internal/testshape` 13, `internal/gitrepo` 11,
`internal/config` and `internal/tui/frame` 10 each, `internal/cli` and
`internal/messaging` 9 each, `internal/convention` and `internal/hooks` 8
each, `internal/webserver` 6, `internal/buildinfo`, `internal/editor` and
`internal/store` 5 each, `internal/wiring` 4, `internal/jira` 3,
`internal/sanitize` 2 and `internal/tui/layout` 1.

**Decided.** 2026-09-26, in #146, which gave every condition that never
ran and every `err != nil` seen one way a test or an entry above, and left
the rest to the report.

**Cost.** A change that breaks one of those conditions' unseen arms fails
no test, and the floor notices only when enough of them add up to move
the share.

**Reopen when.** The measured figure comes within one point of
`BRANCH_COVERAGE_MIN`, or a defect ships through a condition the report
listed as seen only one way.

### TRADE-21 The place rules are written twice

The Issues list narrows to places — a Jira status, or one of the marks in
flight, task active, tracked, task done and forge issue — in the terminal
(`internal/places/places.go`) and in the web
(`web/src/features/issues/issuePlaces.ts`), each working out an issue's marks
and which places admit it from what that surface already holds. Both copies
read their cases from one file, `testdata/twins/places.json`, and each decodes
it strictly, so a case one side would not read fails rather than passing
unread.

**Decided.** 2026-09-30, when both surfaces gained the place filter: the
marks are read from data each surface already has, the branches and the
linked tasks, and moving the rule to the server would mean sending each
issue's marks in the snapshot for a filter that runs in the browser. Kept in
the pre-1.0 paydown, which held both copies to one shared case file.

**Cost.** A change to what a place means is made twice, against one shared
case file that fails whichever copy disagrees with it; a rule the file holds
no case for can still differ between the two.

**Reopen when.** The snapshot comes to carry each issue's marks for another
reason, or the two copies are found to disagree on a case the file does not
hold.

### TRADE-23 The review facets are written twice

The review queue narrows by facet — its repository, how its CI stands,
draft or ready, and who asks — in the terminal
(`internal/tui/reviewfacets.go`) and in the web
(`web/src/features/reviewqueue/reviewFacets.ts`), each working out a
request's facets, the values on offer with their counts, and which picked
values admit a request. The two copies are pinned by twin-named test cases
in `internal/tui/reviewfacets_test.go` and
`web/src/features/reviewqueue/reviewFacets.test.ts`.

**Decided.** 2026-10-01, when both surfaces gained the reviews filter, on
TRADE-21's reasoning: every facet is read from the request each surface
already holds, and a filter the server ran would cost a read of the forge's
queue, under its rate limits, for each change of a filter that runs in the
browser and the terminal over what is loaded.

**Cost.** A change to what a facet means, or to the order its values are
offered in, is made twice, and a change made to one copy alone passes that
copy's tests.

**Reopen when.** The API comes to filter the queue for another reason, or
the two copies are found to disagree.

### TRADE-28 Markdown is turned into wiki markup twice

With `jira.markdown_comments` on, a comment written as Markdown is posted
as Jira's wiki markup by `jira.WikiFromMarkdown` (`internal/jira/wiki.go`),
and the web's comment Preview draws the same conversion from its own copy
(`web/src/features/issues/wiki/wikiFromMarkdown.ts`). Both copies read their
cases from one file, `internal/jira/testdata/wiki_from_markdown.json`, and
each decodes it strictly.

**Decided.** 2026-10-01, when the web gained commenting: Preview redraws
on every keystroke, and asking the server for each one would be a write
under the dry-run guard's rule (it refuses every non-GET) or a GET carrying
the whole comment in its URL. Converting in the browser shows exactly what
will be sent with no request at all. Kept in the pre-1.0 paydown, which held
both copies to one shared case file.

**Cost.** A change to how a Markdown construct converts is made twice,
against one shared case file that fails whichever copy disagrees with it; a
construct the file holds no case for can still convert differently.

**Reopen when.** The server comes to render comments itself, or the two
copies are found to disagree on a construct the file does not hold.

**Revisited.** 2026-10-05, in #174: the web's copy now also draws a forge
issue's thread and Preview, converting its Markdown to wiki markup for
`WikiText` to draw. That reuses the drawing every Jira comment already has
rather than adding a Markdown renderer, at the cost of Markdown the
conversion does not know, such as a table, showing as text.

### TRADE-24 The combined tracker reads its settings once

**Decided.** Jira's settings (address, token, views) are read once, when the
wiring is built, and the combined tracker (`internal/wiring/tracker.go`)
follows them: whether the forge's issues join in, and which view they lead,
are fixed then. The web makes only the forge and messaging settings live on
save. Making these two live would mean always building the combined tracker
and threading another control through every tracker seam, while the rest of
Jira's settings would still wait for a restart. The forge-row count that
offsets later pages is one per tracker, shared by every tab paging that view.
Decided 2026-10-01 in #164, and kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** A Settings save that turns `issues.forge` on or off, or changes the
first view, applies only after `workflow --web` restarts; both Settings
surfaces and the configuration reference should say so, and do not yet for
`issues.forge`. Two tabs paging the first view while the forge's open issues
change can skip or repeat one Jira row on a later page.

**Reopen when.** Jira's settings become live on save, or someone is seen
toggling `issues.forge` from Settings and expecting it to apply at once.

### TRADE-25 Kept data has one schema and is never migrated

**Decided.** `kept.db` holds what the user decided and no session can see
again: owner links, repository groups and favorite directories
(`internal/store/kept.go`). It has one schema, `keptSchemaVersion`, and no
migrations, because before 1.0 workflow is installed fresh. A file at another
version is never discarded: it reads as empty and refuses writes with
`ErrKeptSchemaDiffers`, which names `workflow db-clean --all`. Decided
2026-10-01 in #165, held through the bump to version 2 in #176, and kept on
2026-10-07 in the pre-1.0 audit.

**Cost.** Each change to a kept table leaves files from earlier builds unread:
tagging and favorites go blank until the user runs `workflow db-clean --all`
and answers again.

**Reopen when.** The first change to a kept table after a release users
upgrade across (1.0 onward); a migration path is needed before that change
ships, not after.

### TRADE-26 The Slack directory is read whole, once a session

**Decided.** To offer a channel's members by name,
`internal/messaging/directory/directory.go` reads `users.list` whole (Slack's Tier 2,
up to `messaging.UserListPages`, 20 pages, about 4,000 people) and labels
members from it; past that cap it labels the channel's members one by one
through `users.info` (Tier 4). Every read is shared by everyone asking at
once, held for ten minutes or until a refresh, and never awaited under a lock.
A session started by switching directory (TRADE-34) reads afresh. Decided
2026-10-01 in #165 and #166, and kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** A workspace under the cap holds every user's name in memory for the
session and pays up to 20 pages on its first read. One over the cap pays those
20 pages every ten minutes to learn it is too large, then one `users.info` per
member, so a large channel's first read is slow. A Tier 4 limit is waited out
for at most a minute in all, then the read fails while keeping who was
labeled. A member who joins within the ten minutes is not offered until a
refresh.

**Reopen when.** Slack's rate limits are hit below the cap, or a large
channel's first read through `users.info` is too slow to wait for; then lower
`UserListPages`, or label members concurrently within Tier 4.

### TRADE-29 The Tasks list's orders and narrowing are written twice

The Tasks list sorts by urgency, state, id, tag, issue or priority, and is
narrowed by facet and typed text, in the terminal
(`internal/taskwarrior/order.go`, `narrow.go`) and in the browser
(`web/src/features/tasks/taskOrder.ts`, `taskFacets.ts`). Each pair is
pinned by twin-named cases in its tests.

**Decided.** 2026-10-05, in #172: the web holds the whole list already,
and an order changed in a select should apply at once. Asking the server
for each order would carry it through the read and all ten task writes,
each of which answers with the list, and race the answers when the order
changes again before one lands. Taskwarrior's own report sort was no
alternative: state as the list words it and natural issue-key order are
not keys it has.

**Cost.** A change to how an order breaks a tie, how a value ranks, or what
typed text matches is made twice, and a change to one copy alone passes
that copy's tests.

**Reopen when.** The server comes to order the list itself, or the two
copies are found to disagree.

### TRADE-30 A comment is written in a box drawn inside the interface

**Decided.** A Jira or forge comment is typed in the comment box
(`commentComposer`, `internal/tui/commentcomposer.go`), a Bubbles text area
with vim's normal and insert modes, rather than in `$EDITOR` as a commit body,
a pull request and an announcement are; ctrl+o still hands the draft to
`$EDITOR` and back. A comment is short and answers the issue beside it, which
a full-screen editor would hide, and the box needs no editor configured.
Normal mode knows only moving and entering insert mode (`h`, `l`, `j`, `k`,
`i`, `a`, `A`, `o`), so a key that types is never taken for a command.
Decided 2026-10-05 in #173, and kept on 2026-10-07 in the pre-1.0 audit:
building a fuller editor would cost more than the comments it serves.

**Cost.** The box is a worse editor than the user's own: no operators, no
undo, no search, so vim's `dd` or `u` does nothing. Text that comes back from
`$EDITOR` or a paste passes the text area's filter, so a tab becomes four
spaces and a control character is dropped. It is one more widget to keep
working as Bubbles changes.

**Reopen when.** Users ask for editing commands the box lacks, a comment
written in `$EDITOR` is found posted wrong because of the filter, or a second
surface wants a box of its own.

### TRADE-31 A comment too large to parse safely is shown unformatted

**Decided.** The web draws a comment through `CommentBody`
(`web/src/features/issues/wiki/WikiText.tsx`). A body over
`largestParsedBody` (65,536 characters) or with a line over 1,000 is shown as
plain text, its markup unread, and `beyondParsing`
(`web/src/features/issues/IssueDetailPanel.tsx`) does the same for each older
comment once the newer ones in the thread add up to that size. Forge threads
are written by anyone who can comment, and several markup patterns (a link's
label, a bare link, Markdown's links and underscores) rescan from every opener
that never closes, so a long run of them is quadratic and stalls the page.
Bounding the input keeps every pattern as it is; a linear scanner for each
would be a parser of its own to keep. Decided 2026-10-05 in #174, and kept on
2026-10-07 in the pre-1.0 audit: the bound is a few lines and binds only on
text no one reads formatted.

**Cost.** A real comment past either bound, such as a pasted log, loses its
formatting and its links are text; on a long, busy thread the older comments
do too. 65,536 is GitHub's own limit on a comment, so there only the line
length and the thread's total bind; GitLab allows far longer bodies.

**Reopen when.** A legitimate comment is found drawn plain, or the markup
comes to be parsed by something linear.

### TRADE-32 The Summary is read back each time, not journaled

**Decided.** The Summary (`internal/tui/summary.go`, and the web's section
through `internal/webserver/activity.go`) is read again from git,
Taskwarrior, Jira and the forge whenever a period is shown; nothing about what
was done is written to the store, and a posted Summary is not recorded, as an
announcement is (`loop.PostSummary`). Every source already keeps what it
knows, with its own times, and a journal of workflow's own would see only what
was done through workflow, missing a commit made in a shell or an issue moved
in Jira's own page. Reading back keeps the store free of a second copy that
could disagree with the first. Decided 2026-10-05 in #175, and kept on
2026-10-07 in the pre-1.0 audit: a journal is a feature with its own schema,
not a fix.

**Cost.** A period is as slow to show as its slowest source, and a source that
is down shows nothing for it. What a source does not keep (a Jira comment on
an issue touched no other way, a GitLab event's repository, when a stopped
Taskwarrior task was started) the Summary cannot show. Nothing remembers a
Summary was posted, so the same one can be posted twice.

**Reopen when.** A source the Summary needs keeps no history to read back,
reading a long period back proves too slow to be useful, or a repeated post is
reported.

### TRADE-33 The calendar's dates are written twice

**Decided.** The Summary's dates (a day, a week or a month moved, a month's
last day, a whole month or year, a month as weeks from Monday, and a period
stepped by its own length) are worked out in `internal/activity/period.go` and
again in `web/src/features/summary/civilDate.ts`, both answering to one case
file, `internal/activity/testdata/civil_dates.json`. The web's calendar moves on every
key (`MonthGrid`'s arrows, Page Up and Page Down, Home and End), so it cannot
wait on a request for arithmetic. The server still decides the default period
and reads every period, so only the moves are twinned, never which days were
worked. Decided 2026-10-05 in #175, and kept on 2026-10-07 in the pre-1.0
audit: nearly all of it is the Gregorian calendar, whose rules do not change,
and the one rule of workflow's own, that a whole month or year steps as one,
is a few lines.

**Cost.** A change to how a period steps is made twice, against one shared
case file that fails whichever copy disagrees with it; a mistake in one copy's
leap-year or month-end arithmetic on a date the file holds no case for still
passes that copy's tests.

**Reopen when.** The two copies are found to disagree, the step rule changes,
or `Temporal.PlainDate` is in every browser the web supports, so the browser's
copy can become the platform's.

### TRADE-34 Switching directory starts the interface again

**Decided.** The Repositories pane's switch ends the running program and
`runInterfaces` (`internal/cli/cli.go`) starts a fresh one in the directory
chosen, wired as the first was; `tui.Next` (`internal/tui/reposwitch.go`)
carries only the session's own choices: comments being written on Jira
issues, how the Tasks list is seen, and the Summary's period. A save in
Settings or a first run's setup reopens the same way, so the configuration
saved is the one the next program is wired with. Every seam (the repository,
its forge, the configuration that applies, the store's keys) is bound to a
directory when the interface is built, and rebinding them in a running
program would mean every pane and overlay noticing its world change under it;
starting again gets the answer starting there would, by the same code.
Decided 2026-10-05 in #176, and kept on 2026-10-07 in the pre-1.0 audit:
rebinding live would be a second construction path to keep in step.

**Cost.** A switch reads every pane again and drops what belonged to the
repository left (a commit message, a pull request or a forge issue's comment
being written, an announcement waiting for CI), so it asks first when any of
those would be lost and refuses while a write is in flight. A session started
by a switch reads the Slack directory afresh (TRADE-26).

**Reopen when.** A switch is found slow enough to notice, or the terminal is
found left changed by one in practice.
