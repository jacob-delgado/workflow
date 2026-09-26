# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-64". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts. An earlier edition of this file was retired once
every entry in it was done; this one carries the three entries that outlived
the debt paydown and the findings of a full read of every surface, and its
numbering continues where the earlier edition stopped, so an ID is never
reused.

Checked against commit `f05ae9f` on 2026-09-24 (main after the debt
paydown, PRs #134 and #136, and a Dependabot bump); an entry a later change
touched was checked again in that change. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **Every surface and every package, read from source.** The command
   line (`internal/cli`, `cmd/`), the terminal interface (`internal/tui`)
   and the web (`web/src`, `internal/webserver`, `api/openapi.yaml`), and
   every package under `internal/` beside them, were read in full against
   the standard the project sets for itself in [CLAUDE.md](CLAUDE.md), the
   clig.dev guidelines, the promises the interface makes in
   [UX.md](UX.md) and the web's accessibility floor, with each unit's tests
   read beside it, so a test whose Assert would pass whatever the Act did
   counts as a finding too.
2. **The web, read from the screen.** The mock build was run under
   Playwright and every section screenshotted in both themes at 640, 1024
   and 1440 px, with the production build's empty and no-API states beside
   them, so the web's look was judged from what it draws rather than from
   the source alone.
3. **Measurement, once, at this commit.** The condition-coverage figures
   come from `task cover:branch`; the file lengths from
   `scripts/check-file-length.sh --list`; the budget standings from
   `scripts/check-package-size.sh --list`; the line counts from `task
   cloc`; the web's numbers from the v8 summary and `knip`; the Go side's
   from `deadcode`; and the docs drift from `task docs:check`. An entry
   that cites a number cites the tool it came from.
4. **Every finding refuted before it was written.** Each was handed to an
   independent reader to disprove, and to a second one when rated medium or
   high; a claim that could not be pointed at a line was dropped. Every
   cited line was read again as its entry was written.
5. **Every finding matched against the register.** The register below came
   after this edition's reading, so from the next audit on, each finding is
   compared with [the trade-off register](#the-trade-off-register) before
   it is written, and dropped when it restates an entry whose reopen
   trigger has not fired.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). This edition is read and measured.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it is in
[the trade-off register](#the-trade-off-register), which lists the choices
made on purpose, what each costs, and what would reopen it.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule. This pass found a few; they were written to a private file for the
maintainer, not here, and nothing in this file says what they are. The
entries below contain nothing that helps anyone misuse a credential, a
terminal, a file on disk, the loopback server or a release.

## The command line

Nothing is open here.

## The terminal interface

Nothing is open here: the interface announces and drafts a pull request
through `loop`, as the command line and the web do, and remembers what it
announced through `loop.Deliver`, as the command line does (the web does not
yet: FEAT-84). What it carries on purpose — the spine's color-only hue,
the two composers' field handling written twice, and the editor seam kept
in `internal/tui` — is in [the register](#the-trade-off-register) as
TRADE-5, TRADE-6 and TRADE-11.

## The web

Nothing is open here. What the web carries on purpose — read-only under
`--dry-run`, queries the stream keeps fresh, the host names git prints kept
in a refused push's detail, and the stage rules written again in
TypeScript — is in [the register](#the-trade-off-register) as TRADE-3,
TRADE-4, TRADE-9 and TRADE-10.

## The gates, the build and the tests

What is open here is the coverage worklist and the metric behind it. The
clicked steps the web's axe scans and Tab walks once missed — the pull
request form, the push confirmation, the announcement preview and a refused
write — are scanned in both themes and walked at every width, and a
server-backed run stages, commits and pushes through a running
`workflow --web`. What the gates carry on purpose — every declared budget
at its count, seven long test files, gobco's skip list of two and the web's
range-based branch floor — is in [the register](#the-trade-off-register)
as TRADE-1, TRADE-2, TRADE-7 and TRADE-8.

### DEBT-64 The condition-coverage worklist: 432 one-sided conditions, and 7 never evaluated

Severity: low · Confidence: measured

Re-measured at this commit (`task cover:branch` on macOS, floor 89 %, 23
packages measured), with gobco counting every operand of an `&&` or `||` as
a condition of its own: 4,873 of 5,338 arms, 91.3 %. Of 2,669 conditions,
432 were observed only one way — 81 of them an `err != nil` never seen
true. By package: `internal/tui` 172, `internal/cli` 48, `internal/forge`
41, `internal/webserver` 25, `internal/config` 17, `internal/jira` 16,
`internal/testshape` 15, `internal/messaging` 14, `internal/wiring` 14,
`internal/gitrepo` 13, `internal/hooks` 12, `internal/store` 11,
`internal/tui/frame` 10, `internal/convention` 8, `internal/editor` 6,
`internal/buildinfo` 5, and five across `sanitize` (two) and `httpx`,
`proc` and `tui/layout` (one each).

The store has eleven. Six are failures no test causes: `sql.Open` in
`Store.open` (`internal/store/store.go:192`) and in `Store.openAsItIs`
(`:221`), which fails only for an unregistered driver, and four that need
SQLite to fail partway through a statement: `BeginTx` and `Commit` in
`Store.CacheIssues` (`internal/store/cache.go:109`, `:120`), and `rows.Err`
in `readCachedIssues` (`internal/store/cache.go:87`) and `Store.Announces`
(`internal/store/announce.go:80`). The other five are
the do-nothing guards' second operands, never seen true: `repo == ""` in
`Store.RecordAnnounce` and `Store.Announces`
(`internal/store/announce.go:24`, `:50`), `instance == ""` in
`Store.CachedIssues` and `Store.CacheIssues` (`internal/store/cache.go:31`,
`:98`), and `s.dir == ""` in `Store.off` (`internal/store/store.go:149`).

Seven conditions were never evaluated. Four are a test away:

- `internal/tui/messaging.go:83` and `:85` — `quitGuard.handleKey`'s confirm
  and stay: `TestQuittingWithAQueuedPostAsksFirst` opens the guard but
  presses neither enter (quit) nor esc (stay) in it.
- `internal/tui/prcreate.go:134` — `pullCreated.apply`'s `named` case, a
  Jira issue with no link seam: every test that opens a pull request on a
  Jira issue's branch wires `Jira.LinkPullRequest`.
- `internal/wiring/wiring.go:225` — `streamToEnd`, git failing to start: no
  wiring test fetches or pulls without git on `PATH`.

Two more came into view once each operand counted, and each is a test
away too:

- `internal/messaging/post.go:335` — `Announcement.Text`'s `a.Kind ==
  config.KindSlack`: each test that renders a template leaves `Kind` empty,
  so the `||` never reads it.
- `internal/tui/issuekeys.go:136` — `extendFilterWith`'s `msg.Code ==
  tea.KeySpace`: no filter test types a key without text, so `text == ""`
  never lets the `&&` read it.

One no black-box test reaches without changing the code:

- `internal/tui/tui.go:143` — `tui.Run`'s error return, which needs a real
  terminal.

**What it costs.** Condition coverage reads 91.3 %, 2.3 points above the
89 % floor, which is the ratchet's own slack, so an untested error path in
the next feature no longer fails the gate on someone else's pull request.
The cost now is the ratchet: floor(91.3) − 2 is today's 89, so
`BRANCH_COVERAGE_MIN` cannot rise until the report reads 92.0 % — 4,909
arms, 36 more than today, since the gate rounds to one decimal first.

**One way to fix it.** The reachable sites above, one test each; then the
report is the worklist, most of it in `internal/tui`, `internal/cli` and
`internal/forge`.

**Done when.** `task cover:branch` names no never-evaluated condition but
the one above, and reads 92.0 % or more, so `BRANCH_COVERAGE_MIN` ratchets
to 90.

## The trade-off register

These were chosen on purpose. They are not debt; they are listed because
each one costs something a reader should know about. Each entry carries an
ID that is never reused, when it was decided, what it costs, and the event
that would reopen it, and the code it keeps carries a `Trade-off TRADE-n:`
comment naming it. An audit or a review drops a finding that restates an
entry here unless that entry's reopen trigger has fired.
`scripts/check-tradeoffs.sh`, in `task lint`, fails an entry missing its ID
or one of its three fields, and a site comment naming an ID the register
does not hold.

### TRADE-1 Every package with a declared file budget sits exactly at it

The numbers are in `scripts/package-size-budgets.txt`, and
`scripts/check-package-size.sh --list` shows the standings. That is the
gate working as designed, since a budget left above its count fails too:
the next file in any of them is a decision (a split, or a bump with the WHY
rewritten and a history row), not an accident. A directory the file does
not list answers to the default, which `internal/forge` and `web/src/shell`
fill exactly, so the next file in either is a first entry with its WHY.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** Any change adding a file there must carry its budget row in the
same commit or fail `task check`.

**Reopen when.** One directory's budget rises three times in rows of
`scripts/package-size-budget-history.md` dated after 2026-09-25, with no
row lowering it between them.

### TRADE-2 Seven test files stay past the 500-line soft target

All seven are under the 800 ceiling (`scripts/check-file-length.sh
--list`). They were left whole on purpose when the source files past the
target were split by concern; each holds the cases of one behavior.
Announcing: `internal/messaging/post_test.go` (766, the post to each
service and the announcement's text) and `internal/tui/messaging_test.go`
(614, the terminal's Messaging pane). Opening a pull request:
`internal/webserver/pullrequest_test.go` (585, the web's draft and open).
Staging, committing and pushing: `internal/tui/composer_test.go` (568, the
terminal's commit composer), `internal/webserver/staging_test.go` (533, the
web's stage and unstage) and `web/src/features/branch/BranchPanel.test.tsx`
(513, the web's commit and push). Reading and writing an issue:
`internal/jira/detail_test.go` (501, Jira's issue read, comment and pull
request link). The counts are pinned: a commit that changes the length of
one of these files updates its count here.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** `scripts/check-file-length.sh` still warns on every run, so a
source file newly past the target is one more line among seven a reader has
learned to skim.

**Reopen when.** One of the seven passes 700 lines, or grows at all once
past it (`post_test.go` is at 766), or starts holding the cases of a second
behavior.

### TRADE-3 The web is read-only under `--dry-run`

`refuseWritesInDryRun` (`internal/webserver/guard.go:40`) documents it:
every unsafe method answers 403 at one gate, where the terminal simulates
each write and narrates it. The browser reads `dry_run` from `getHealth`,
says so in a banner, and holds every write before sending it
(`web/src/api/client.ts:25`), so the server's 403 is only the backstop. The
blanket refusal itself is the intended design.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** A web write under dry run is refused outright rather than
simulated: the browser cannot show what the write would have done, as the
terminal's narration does.

**Reopen when.** A dry-run preview of a web write is asked for, in a
FEATURES.md entry or a user's report.

### TRADE-4 `staleTime: Infinity`, with the event stream as the sole freshness source

`web/src/queryClient.ts:17`. Correct for a pushed snapshot. Three queries
set their own `staleTime`. The issue detail's and the review queue's are a
minute (`web/src/features/issues/issueApi.ts:23`,
`web/src/features/reviewqueue/reviewQueueApi.ts:23`): the stream carries
only the list's slim issues and never the queue, so each is read again when
reopened after a minute, and the queue's Refresh reads it at once. The
configuration's is 0 (`web/src/features/settings/configApi.ts:65`): the
file can change on disk, which no event reports, so Settings reads it again
each time it opens. The commit form's types come from the snapshot and,
like commit validation, follow the configuration the server last read or
saved, so an edit made on disk reaches them once Settings is opened or a
save lands. A save that finds the file changed since Settings read it is
refused (409) and nothing is written; Settings offers **Reload**. A change
landing between that check and the write is not caught (`SaveOver`,
`internal/config/save.go:120`).

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.
That an edit made on disk reaches the commit form only once Settings opens
or a save lands was accepted on 2026-09-26 in #145.

**Cost.** A stalled stream leaves stale data with no refetch to fall back
on.

**Reopen when.** A stalled stream is reported, or a panel is reported
showing stale data that neither the stream, its own `staleTime` nor a save
reads again.

### TRADE-5 The progress spine's per-system hue is color-only

The hue map in `Model.stages` (`internal/tui/spine.go:71`) is mitigated by
the stage name, or its initial when compact (`initial`,
`internal/tui/spine.go:51`). It is part of the visual system UX.md says
should not change.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** One channel the monochrome reader does not get.

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

### TRADE-7 A condition-coverage skip list of two

`UNANALYZABLE` (`scripts/gobco-report.sh:87`): gobco ignores build tags, so
it cannot read a package whose files come in tagged twins, and
`internal/proc/pgroup` and `internal/web` are named there with that reason
beside them. Every other package is read, and one that becomes unreadable
without being named fails the gate rather than shrinking the number.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** The two named packages' conditions go unmeasured — platform glue
and an embed stub, with no branch worth the count — and the next tagged
twin must join them.

**Reopen when.** gobco reads build tags, or a third package whose files
come in tagged twins appears.

### TRADE-8 The web's branch floor is v8's range-based count

`thresholds` in `web/vitest.config.ts:45` is not a gobco-style
per-condition floor: v8 marks a branch covered once its range of code has
run, and never asks which way each operand of a condition went.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** An `a && b` only ever seen with `b` true still passes the web's
floor.

**Reopen when.** The web's coverage tool can report per-condition counts
the gate could read, or a web defect ships through a condition only ever
seen one way.

### TRADE-9 A refused push's detail keeps the host names git prints

When a push from the web fails, its problem detail keeps git's own lines,
so the reason between them survives (`withoutAddresses`,
`internal/webserver/push.go:93`). URLs (`scheme://…`) and scp-style
`user@host:path` addresses in them are replaced with `<address>`; a bare
host name git prints — in `Could not resolve host`, `connect to host` or
`To host:path` — is kept. The detail is shown on the page of the user who
pushed, which the server serves on the loopback only.

**Decided.** 2026-09-26, in #145.

**Cost.** A host name from git's output can appear in a push's problem
detail.

**Reopen when.** A report asks for host names to be taken out of a push's
detail, or a push's detail comes to be shown to anyone other than the user
who pushed.

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

Every seam bundle the surfaces share lives in `internal/seams`, but
`tui.EditorDeps` (`internal/tui/deps.go:62`) stays in `internal/tui`
because its functions take and return Bubble Tea's messages and commands,
so `wiring.Deps` still returns a `tui.Deps`.

**Decided.** 2026-09-25, in #144 (DEBT-71).

**Cost.** A second surface that wants the editor seam must import the
terminal package or declare its own, and the wiring's bundle is the
terminal's type rather than a surface-neutral one.

**Reopen when.** A second surface needs the editor seam, or its functions
stop taking Bubble Tea types.
