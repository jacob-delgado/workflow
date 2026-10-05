# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to pick up an entry by its ID. Each
entry says what is wrong, where, what it costs, one way to fix it, and how
to tell when it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold
the ideas; this file holds the debts.

Every entry of the 2026-09-24 audit edition, a full read of every surface
at `f05ae9f` — DEBT-64, DEBT-65, DEBT-71 and DEBT-75 to DEBT-161 — was paid
down in PRs #141, #144, #145 and #146. What the file holds now is the
method that edition followed, how to read an entry, one short section per
surface saying nothing is open there, and the trade-off register: the costs
chosen on purpose, which a later audit checks each finding against first
(step 5 below). Numbering continues at DEBT-162, so an ID is never reused.

Checked against commit `29fad1b7` on 2026-09-27 — main once #145 merged —
with #146's commits on top; an entry a later change touches is checked
again in that change. Line numbers drift, so every pointer also names the
symbol it means.

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
3. **Measurement, once, at the audited commit.** The condition-coverage
   figures come from `task cover:branch`; the file lengths from
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

Nothing is open here. The clicked steps the web's axe scans and Tab walks
once missed — the pull request form, the push confirmation, the
announcement preview and a refused write — are scanned in both themes and
walked at every width, a server-backed run stages, commits and pushes
through a running `workflow --web`, and every Go condition runs and every
Go `err != nil` is seen both ways in a test, but for those the register
keeps. What the gates carry on purpose — every declared budget at its
count, seven long test files, gobco's skip list of two, the web's
range-based branch floor, the conditions no black-box test reaches and the
rest of gobco's report left as its worklist — is in
[the register](#the-trade-off-register) as TRADE-1, TRADE-2, TRADE-7,
TRADE-8 and TRADE-12 to TRADE-19.

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

TRADE-27, a top-level GitLab group linking to Slack like a person, was
closed in #166: a bare CODEOWNERS name is now asked of GitLab when tags
are composed, and a group links to a Slack user group. A bare name decided
as a person before GitLab was asked, as every one was, is asked about
again as a team once GitLab knows it as a group; one GitLab cannot be asked
about stays what it was decided as. Its ID is not reused.

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
The detail's and the queue's minute became `freshFor`'s 30 seconds on
2026-09-30, to match the terminal's refresh on switch.

**Cost.** Any change adding a file there must carry its budget row in the
same commit or fail `task check`.

**Reopen when.** One directory's budget rises three times in rows of
`scripts/package-size-budget-history.md` dated after 2026-09-25, with no
row lowering it between them.

**Revisited.** 2026-10-05, in #170: the trigger had fired for
`internal/tui` (45 → 50 in #164 and #165), and seven more files are
planned there — `keycheck.go`, then the Tasks listing, the comment
composer, the Summary pane and its calendar, and the Repositories pane and
its directory prompt. Each is a pane or an overlay of the one interface
CLAUDE.md says must not be split to chase a number, so each rises with its
own row and WHY rather than reopening how the package is budgeted.

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

`web/src/queryClient.ts:17`. Correct for a pushed snapshot. Four queries
set their own `staleTime`. The issue detail's and the review queue's are
`freshFor`, 30 seconds (`web/src/queryClient.ts`, used in
`web/src/features/issues/issueApi.ts` and
`web/src/features/reviewqueue/reviewQueueApi.ts`), the same span after which
the terminal reads a pane again when it is switched to: the stream carries
only the list's slim issues and never the queue, so each is read again when
reopened after it, and the queue's Refresh reads it at once. The tasks' is 0
(`web/src/features/tasks/tasksApi.ts`): Taskwarrior is local and has no
rate limit, so the section reads it each time it opens. The
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

### TRADE-7 A condition-coverage skip list of one

`UNANALYZABLE` (`scripts/gobco-report.sh:78`): gobco ignores build tags, so
it cannot read a package whose files come in tagged twins, and
`internal/proc/pgroup` is named there with that reason beside it. Every
other package is read, and one that becomes unreadable without being named
fails the gate rather than shrinking the number.

**Decided.** Recorded on 2026-09-24 in the audit (#140), and kept on
2026-09-25 when the debt paydown reopened none of the recorded trade-offs.

**Cost.** The named package's conditions go unmeasured — platform glue,
with no branch worth the count — and the next tagged twin must join it.
`internal/web` left the list on 2026-09-30, when its embed stopped being
build-tagged.

**Reopen when.** gobco reads build tags, or a second package whose files
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

Ten `err != nil` checks follow an `encoding/json` call on a type the
program defines, and none is ever true: `json.Marshal` fails only on a
channel, a function, a complex number, a NaN or infinite float, a map it
cannot key, a cycle or a marshaler of its own that fails, and these types
hold none of them; and the JSON a `config.Config` encodes to always fits
`api.Config`, so decoding it into the web's shape cannot fail either. The
sites: `Client.newRequest` (`internal/forge/client.go:189`),
`Client.newJSONRequest` (`internal/jira/jira.go:135`), `Client.postJSON`
(`internal/messaging/post.go:169`), `write`
(`internal/config/save.go:142`), `configDTO`'s encode and decode
(`internal/webserver/config.go:259`, `:266`) and its two callers,
`server.GetConfig` (`:38`) and `server.writeOver` (`:132`), `fromDTO`
(`:278`), and `writeSnapshot` (`internal/webserver/stream.go:115`). Each
check stays, since the project returns an error rather than dropping it.

**Decided.** 2026-09-26, in #146.

**Cost.** Ten error arms no test runs. A field added later that can fail
to encode would put one of them in play with no test behind it.

**Reopen when.** One of these types gains a float, an interface-typed
field or a marshaler of its own, `config.Config` and `api.Config` stop
agreeing on a field's type, or an encoding failure is reported.

### TRADE-14 The embedded OpenAPI contract is taken to load

The web server checks each request against `api/openapi.yaml`, compiled
into the binary, and five conditions ask whether that contract loads,
validates and routes: `loadSpec` (`internal/webserver/validator.go:33`,
`:38`), `validate` (`:55`, `:62`) and `Handler`
(`internal/webserver/webserver.go:192`). Only a contract broken at build
time fails them, and then every web server test fails with it, so no test
can run against a broken one.

**Decided.** 2026-09-26, in #146.

**Cost.** Five error arms no test runs: the start-up message for a broken
contract is read by whoever broke the build, never checked by a test.

**Reopen when.** The contract is read from anywhere but the binary, such
as a file or a flag, or a broken contract reaches a release.

### TRADE-15 A driver the store imports is taken to be registered

Three conditions fail only for a database driver that is not registered:
`sql.Open` in `openDatabase`, its check in `openCurrent`, and `sql.Open`
in `Store.openAsItIs` (`internal/store/store.go:277`, `:230`, `:368`). The package imports its own, so
no exported call lets a test cause either, and a seam added only for the
test would be code kept for the test's sake.

**Decided.** 2026-09-26, in #146.

**Cost.** Three error arms no test runs, so what a store says when it cannot
open its driver is read rather than checked.

**Reopen when.** One of these failures is reported with a message that
does not say what failed, the store comes to choose its driver at run
time, or a seam over the driver arrives for another reason.

### TRADE-16 Failures only a change between two calls can cause

Eleven conditions follow a call that has just read or made the same
thing, so they fail only when the file system or the context changes
between the two, a race no test can hold open without a seam. Saving the
configuration through a link: `followDanglingLink` reads a link `os.Lstat`
has just found (`internal/config/save.go:218`), and `linkDestination`'s
two reads follow what the system has just resolved (`:234`, `:245`).
Caching the issue list: `BeginTx` in `Store.CacheIssues`
(`internal/store/cache.go:111`) takes no lock, so it fails only when the
context ends between the schema step, which used it, and the transaction.
Opening the store at this build's schema version: `holdsTables` lists the
schema the connection just loaded (`internal/store/store.go:308`, checked
in `openCurrent`, `:246`); `removeDatabase` removes files the open that
just read the version held (`:347`, checked in `remakeDatabase`, `:318`,
and again in `openCurrent`, `:256`, where a driver not registered would
fail the reopen too); and `stamp` writes to a file the open just made or
read (`:332`, checked in `openCurrent`, `:262`).

**Decided.** 2026-09-26, in #146.

**Cost.** Eleven error arms no test runs, so what each says when the race
is lost is read rather than checked.

**Reopen when.** One of these failures is reported, or a change to the
calls lets a test fail the second without the first.

### TRADE-17 The keychain the web's Settings fills is never seen filled

One Slack path is only ever seen failing: the web's Settings placing typed
secrets in the macOS keychain (`placeSlackCredentials`,
`internal/wiring/messaging.go`). It runs only on macOS and writes the
real keychain, which no test may. The refresh it makes is tested against
a local server in `internal/slackauth`, and the web server's side of a
Settings save against a fake placement. `workflow doctor --online`
accepting a user token and `workflow slack login` keeping what Slack
gives back are tested against a fake Slack, which `WORKFLOW_SLACK_API`
points them at.

**Decided.** 2026-09-26, in #146; widened on 2026-09-30 when the Slack bot
token gave way to the rotating user token and `workflow slack login`;
narrowed on 2026-10-01, in #166, once `WORKFLOW_SLACK_API` let tests
stand a fake in for Slack.

**Cost.** What a Settings save leaves in the keychain once Slack accepts
the secrets is never checked end to end.

**Reopen when.** The keychain can be pointed at a store a test owns, or
a Settings save on macOS is reported to keep the wrong secrets.

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

### TRADE-20 The embedded web app is taken to be rooted at dist

Two conditions fail only when the embedded bundle cannot be opened at its
directory: `fs.Sub` in `Assets` (`internal/web/embed.go:21`) and its check
in `WebServerAt` (`internal/cli/cli.go:315`). `fs.Sub` fails only for an
invalid path, the path is the constant `"dist"`, and the embed fails the
build when that directory is missing, so no test can reach either arm.

**Decided.** 2026-09-30, when every build came to embed the committed web
app so `go install` carries it.

**Cost.** Two error arms no test runs: what the server says for an app it
cannot open is read rather than checked.

**Reopen when.** The app is read from anywhere but the binary, or its root
becomes something other than a constant.

### TRADE-21 The place rules are written twice

The Issues list narrows to places — a Jira status, or one of the marks in
flight, task active, tracked and task done — in the terminal
(`internal/tui/issueplaces.go`) and in the web
(`web/src/features/issues/issuePlaces.ts`), each working out an issue's
marks and which places admit it from what that surface already holds. The
two copies are pinned by twin-named test cases in
`internal/tui/issueplaces_test.go` and
`web/src/features/issues/issuePlaces.test.ts`.

**Decided.** 2026-09-30, when both surfaces gained the place filter: the
marks are read from data each surface already has, the branches and the
linked tasks, and moving the rule to the server would mean sending each
issue's marks in the snapshot for a filter that runs in the browser.

**Cost.** A change to what a place means is made twice, and a change made
to one copy alone passes that copy's tests.

**Reopen when.** The snapshot comes to carry each issue's marks for another
reason, or the two copies are found to disagree.

### TRADE-22 A stale refresh lock can be taken over twice

The Slack refresh lock (`internal/slackauth/lock.go`) is a file made with
`O_CREATE|O_EXCL`, and one older than a minute is taken as left behind by
a process that ended mid-refresh and is removed. Two processes that both
find it stale at the same moment can both remove it, and the second's
remove can take the fresh lock the first just made, so both refresh.

**Decided.** 2026-09-30, in #162: the race needs a lock left behind by a
crash and two workflows waiting on it in the same instant, and its harm is
bounded — the second refresh spends a refresh token already spent, Slack
refuses it, and nothing is saved, so the pair the first kept stands and
the next post uses it. An operating-system lock (`flock`, `LockFileEx`)
would end the race and the minute's wait, at the cost of a lock written
per platform.

**Cost.** In that rare case one post fails, saying the refresh was refused,
and a lock left behind blocks every refresh for up to a minute.

**Reopen when.** A refused refresh is traced to two refreshes at once, or
the lock wait is seen to block a post.

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
(`web/src/features/issues/wiki/wikiFromMarkdown.ts`). The two are pinned
by twin-named cases in `internal/jira/wiki_test.go` and
`web/src/features/issues/wiki/wikiFromMarkdown.test.ts`.

**Decided.** 2026-10-01, when the web gained commenting: Preview redraws
on every keystroke, and asking the server for each one would be a write
under the dry-run guard's rule (it refuses every non-GET) or a GET carrying
the whole comment in its URL. Converting in the browser shows exactly what
will be sent with no request at all.

**Cost.** A change to how a Markdown construct converts is made twice, and
a change made to one copy alone passes that copy's tests.

**Reopen when.** The server comes to render comments itself, or the two
copies are found to disagree.

### TRADE-24 The combined tracker reads its settings once

With Jira configured and `issues.forge` on, the Issues list draws on both
trackers through `combinedTracker` (`internal/wiring/tracker.go`). Whether
the forge's issues join in, and which view they lead (the first in
`jira.views`), are read when the wiring is built, and the count of forge
rows that led a first page is one per tracker, shared by every surface and
tab that pages that view.

**Decided.** 2026-10-01, in #164: Jira's own settings — its address, token
and views as the tracker sees them — are already read once, at wiring, and
a live tracker would need a control the web's Settings save calls, as
`UseForgeSettings` is for the forge, threaded through every tracker seam.
The web's lists resolve a view's JQL from the configuration in effect, so a
renamed or reordered first view still lists Jira's issues; only the forge's
rows are left off it until workflow restarts.

**Cost.** A Settings save that turns `issues.forge` on or off, or changes
the first view, takes effect only after a restart of `workflow --web`; and
two tabs paging the first view while the forge's open issues change can
skip or repeat a Jira row on a later page.

**Reopen when.** Jira's settings are made live on save, or someone is seen
to toggle `issues.forge` from Settings and expect it at once.

### TRADE-25 Kept data has one schema and is never migrated

Whom a forge owner is on Slack, and which Slack groups a repository tags,
are decisions the user made once and no session can see again. They live
in `kept.db` beside the cache (`internal/store/kept.go`). Like the cache,
its schema has one version, `keptSchemaVersion`, stamped into the file
when it is made; unlike the cache, a file at another version is never
discarded.

**Decided.** 2026-10-01, in #165, as forward-only migrations; replaced on
2026-10-01 by one schema with no migrations, since workflow is only ever
installed fresh. Discarding the file would make the "asked once" promise
false behind the user's back, so a file at another version is left as it
is: it reads as empty and refuses writes with `ErrKeptSchemaDiffers`,
which names `workflow db-clean --all`.

**Cost.** A change to a kept table leaves every file made before it
unread: tagging stops, and nothing can be decided, until the user runs
`workflow db-clean --all` and every owner and group is asked about again.

**Reopen when.** workflow is installed where kept data must outlive a
schema change, or a kept table needs to change at all.

### TRADE-26 The Slack directory is read whole, once a session

Tagging a code owner means picking them from the announcement channel's
members, by the name Slack shows for them. `conversations.members` gives
only IDs, so `internal/wiring/slackdirectory.go` reads `users.list` whole
and labels the members from it, and holds every directory read for ten
minutes or until the user refreshes. A workspace past
`messaging.UserListPages` pages of `users.list` is not listed whole: its
channel's members are labeled one by one through `users.info`, each held
the same way.

**Decided.** 2026-10-01, in #165: one paged `users.list` is a handful of
requests for most workspaces, where a `users.info` per member would be one
request per person every time a channel is shown. Holding the reads for the
session lets the preview open again at once, and ten minutes bounds how
stale a newly joined member or a renamed group can be. Revisited
2026-10-01, in #166: the reopen trigger — a directory too large to read
whole — is met by the `users.info` fallback, so it is no longer a reason
to reopen. `users.list` is Slack's Tier 2 (about 20 requests a minute), so
the fallback starts after 20 pages (4,000 people), before the list would
meet the rate limit; `users.info` is Tier 4 and bounded by the channel's
size. Reads share one request however many ask at once, and none holds a
lock while Slack answers.

**Cost.** A workspace under the cap holds every user's name in memory for
the session, and pays up to 20 `users.list` pages on its first read. A
workspace over it pays those 20 pages once per ten minutes to learn it is
too large, then one `users.info` per channel member, so a large channel's
first read is slow. A channel large enough to meet Tier 4 waits as long as
Slack's Retry-After asks, up to a minute in all, and past that fails the
read, keeping who was labeled, so the next read goes on from there; there
is no pacing ahead of the limit. Someone who joins the channel within the
ten minutes is not offered until a refresh.

**Reopen when.** Slack's rate limits are met in practice below the cap, or
a large channel's first read through `users.info` is too slow to wait for:
lower `UserListPages`, or label members concurrently within Tier 4.

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

A comment is typed in the comment box (`internal/tui/commentcomposer.go`), a
text area with vim's normal and insert modes, rather than in `$EDITOR` as a
commit body, a pull request and an announcement are. ctrl+o still hands the
draft to `$EDITOR` and back.

**Decided.** 2026-10-05, in #173: a comment is short and written beside
the issue it answers, which a full-screen editor hides; the box keeps the
issue in view and needs no editor configured. It understands only moving
and entering insert mode (`h`, `l`, `j`, `k`, `i`, `a`, `A`, `o`), so a
key that types is never mistaken for a command.

**Cost.** The box is a worse editor than the user's own: no operators,
no undo, no search. A user who expects vim's `dd` or `u` finds nothing
happens, and the text area is one more thing to keep working as Bubbles
changes. Everything that enters the box passes the text area's own filter:
a tab becomes four spaces and a control character is dropped, so a comment
written in `$EDITOR` with tabs is posted with spaces.

**Reopen when.** Users ask for editing commands the box lacks, or a second
surface wants a box of its own.
