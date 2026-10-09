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
   location and fix, and a problem found in several areas became one entry,
   in a section of its own for the problems across the surfaces.
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

## The gates, the build and the tests

### DEBT-239 Pay down TRADE-19: the gate fails a never-run condition or an unseen error arm

Severity: medium · Confidence: measured · Size: L

**Where.** `scripts/gobco-report.sh` (the TRADE-19 site, where it prints
gobco's per-condition worklist), `BRANCH_COVERAGE_MIN` (`Taskfile.yml`),
TRADE-19; among the gaps: `jiraOnly` (`internal/wiring/tracker.go`),
`asMessagingError` (`internal/wiring/messaging.go`), `ask`'s failed connection
(`internal/wiring/forge.go`), `Create` and `createPrivate`
(`internal/config/save.go`), `Keepable` (`internal/setup/setup.go`),
`branchCreator.pasted` (`internal/tui/branchcreator.go`),
`commitComposer.pasted` (`internal/tui/composer.go`), `statusPicker.pasted`
(`internal/tui/fields.go`), `commentDrafts.keeping`
(`internal/tui/commentcomposer.go`), and `keptWithin` through
`pruneSlackEntities` (`internal/store/kept.go`).

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
stays. TRADE-10, TRADE-23 and TRADE-29, the rules the web wrote a second
time, were paid down when the server took them over, and their entries are
gone. TRADE-6 was paid down when the composers, the calendar and the pane
switch came to move around one field ring, and TRADE-12 when the terminal
interface came to take its input from the caller, so a test drives it; their
entries are gone. TRADE-18 was paid down to one condition, when every command
came to take its working directory from its caller, then widened to a second,
when the interface came to read keys from the terminal past a piped input, and
stays. TRADE-19 is to be paid down by the entry above whose title names it,
and stays here until that entry is paid. In the pre-1.0 paydown every entry
that stays was checked against the code again: each names the functions and
files it keeps rather than lines, which drift, and the register is in ID
order.

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
0, or `directoryHold`), and the one that changes with no event to say so, the
Tasks list, sets a `refetchInterval` that reads it again once the earliest wait
still ahead has passed (`useTasks`). The stream's connection state (`reconnecting`,
`stale`) is shown, so a dropped stream is visible. A configuration save checks
the file's revision and refuses a changed file with 409; a change landing
between that check and the write is not caught (`SaveLayers`,
`internal/config/layers.go`), which is accepted for one user's local file.
Recorded on 2026-09-24 in the audit (#140), kept on 2026-10-07 in the pre-1.0
audit, and given the Tasks list's interval when the server took over its rules
(DEBT-328).

**Cost.** A connection that stays open but stops sending frames looks live,
and leaves stale data with no refetch to fall back on.

**Reopen when.** A stalled-but-open stream is reported, or a panel is reported
showing stale data that neither the stream, its own `staleTime` nor a save
reads again.

### TRADE-5 The progress spine's per-system hue is color-only

**Decided.** On the progress spine each stage's glyph is drawn in its system's
hue (Jira, git, forge, messaging), inside `spineView.stages` in
`internal/tui/spine.go`. The hue only repeats what the text says: the stage's
name, or its initial when compact, names the system, and the glyph's shape
carries its state. The hues are part of UX.md's visual system, which the web's
tokens share. Recorded on 2026-09-24 in the audit (#140), and kept on
2026-10-07 in the pre-1.0 audit.

**Cost.** A monochrome or color-blind reader loses one redundant channel
telling the systems apart.

**Reopen when.** UX.md's visual system changes, or an accessibility report
names the spine's hue.

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

### TRADE-11 The editor seam stays in the terminal package

**Decided.** `Environment.Deps` returns `tui.Deps`
(`internal/wiring/wiring.go`), and `cli.WebDeps` hands its seam groups, which
package `seams` declares, to `webserver.Deps` whole. Three of its fields speak
Bubble Tea: `Editor` (`EditorDeps`, whose functions take and return Bubble Tea
messages and commands because `internal/editor` hands the terminal over
through `tea.ExecProcess`), `After` and `Copy`. So the bundle is the
terminal's type rather than one in package `seams`. Decided 2026-09-25 in #144
(DEBT-71), kept on 2026-10-07 in the pre-1.0 audit, and rewritten when the web
server came to take the seam groups whole in the pre-1.0 paydown.

**Cost.** The wiring's output is named for one surface, and the web reaches
its seams through the terminal's struct; a second surface that wants the
editor must import `internal/tui`.

**Reopen when.** A surface other than the terminal needs the editor, `After`
or `Copy`; `internal/editor` stops returning a Bubble Tea command; or `--web`
needs to be wired without `internal/tui`. Package `webserver` imports no
`internal/tui` (`depguard`'s `webserver-not-terminal` rule forbids it), but
`cli.WebDeps` still builds its seams from the `tui.Deps` the wiring returns,
and hands it `tui.CheckKeys` and `tui.KeyActions` as functions.

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

**Cost.** About sixteen error arms no test runs. Because forge's
`Client.newRequest`, jira's `Client.newJSONRequest`, messaging's
`Client.postJSON` and `config.encodeValue` accept `any`, a caller passing a
float, a channel or a failing marshaler would put an arm in play with no test
behind it.

**Reopen when.** A type encoded at one of these sites gains a float, a
function, a channel, an interface-typed field or a `MarshalJSON` of its own;
`config.Config` and `api.Config` disagree on a field's type; or an encoding
failure is reported.

### TRADE-14 The embedded OpenAPI contract is taken to load

**Decided.** `loadSpec` and `loadContract` (`internal/webserver/validator.go`)
and `Handler` (`internal/webserver/webserver.go`) keep five `err != nil` arms
for an embedded `api/openapi.yaml` that fails to load, validate or route. The
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

### TRADE-18 Two conditions a test reaches where the report cannot see it

Two conditions are reached by a test the condition report does not count.
`closeRequestLog` (`internal/cli/scriptable.go`) asks whether the request
log's file could be fully written, and says so on stderr when it could not.
`TestRequestLogWarnsOnceWhenItCouldNotBeWritten`
(`internal/cli/reqlog_test.go`) reaches it by logging to `/dev/full`, every
write to which fails as a full disk would; macOS has none, and no other file
can be made to refuse writes from a test, so a report measured there lists it
as seen one way, and CI's, measured on Linux, does not. `keysFrom`
(`internal/tui/tui.go`) asks whether the interface's input is a file that is
no terminal, from which it reads no keys.
`TestRunReadsNoKeysFromAFileThatIsNoTerminal`
(`internal/tui/run_unix_test.go`) reaches it in a child process that has no
terminal at all, as a test run from a developer's shell does not, and gobco
counts only the test process, so every report lists that the input is a file
as never true and whether it is a terminal as never asked.

**Decided.** 2026-09-26, in #146; narrowed to `closeRequestLog` alone in the
pre-1.0 paydown, when every command came to read its working directory from
the environment its caller hands it, so a test reaches the six `os.Getwd`
arms this entry also kept on every system; and widened to `keysFrom` in the
same paydown, when the interface came to read keys from the terminal when its
input is a file that is none (DEBT-197).

**Cost.** `task cover:branch` on macOS reads one arm fewer than CI does, and
every report lists `keysFrom`'s condition as seen one way, though a test fails
when either arm breaks; a reader of the report could take either for
untested.

**Reopen when.** CI measures condition coverage on another system, a test
stops reaching either condition, or gobco comes to count what a child process
runs.

### TRADE-19 The rest of the condition report stays gobco's worklist

Besides the conditions the entries above keep, `task cover:branch` lists 280
seen only one way, none of them an `err != nil` or a condition that never
ran. Six of them still check an error, spelled another way: the `err == nil`
after a pull request's reviews read (`githubReviewState`,
`internal/forge/github.go`), GitLab's approvals read and re-run
(`gitlabReviewState` and `gitlabRerun`, `internal/forge/gitlab.go`), a
link's parse (`isWebURL`, `internal/messaging/announcement.go`) and a
further page of issues (`issueList.settle`, `internal/tui/issues.go`), none
of them ever seen false, and the `errors.Is` asking whether the web's issue
read failed for want of a repository (`server.GetIssue`,
`internal/webserver/handlers.go`), never seen true. They stay out of this
file: the report is their list, printed on every run, and CLAUDE.md already
reads it as "a worklist of missing test cases, not a percentage to chase",
so a test for one is written when the code around it next changes, and
`BRANCH_COVERAGE_MIN` keeps the share from falling. By package, measured on
macOS on 2026-09-27: `internal/tui` 152, `internal/forge` 19,
`internal/testshape` 13, `internal/gitrepo` 11, `internal/config` and
`internal/tui/frame` 10 each, `internal/cli` and `internal/messaging` 9
each, `internal/convention` and `internal/hooks` 8 each,
`internal/webserver` 6, `internal/buildinfo`, `internal/editor` and
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

### TRADE-35 Windows-only code is built in CI but never run

**Decided.** Every CI job runs on `ubuntu-latest`. The Cross-compile job
(`cross` in `.github/workflows/ci.yml`) runs `task release:binaries` and
`task release:verify`, which build a binary for every `RELEASE_PLATFORMS`
target in `Taskfile.yml`, `windows/amd64` among them, so the code only a
Windows build holds is compiled on every change, but no job runs it. Decided
2026-10-09 by the maintainer in the pre-1.0 paydown (#232): CI stays on
Linux, and the cost is recorded here.

**Cost.** Four files are Windows-only, and one function has a branch only
Windows takes, each marked with this entry's ID:
`internal/filelock/lock_windows.go` (`//go:build windows`), the `LockFileEx`
lock `TryLock` takes, which keeps two sessions from writing the Slack
credentials at once; `internal/fileowner/owner_other.go` (`//go:build
!unix`, which among the release targets builds for Windows alone), whose `Of`
knows no owner, so the configuration's trust check refuses nothing there;
`internal/proc/pgroup/pgroup_other.go` (`//go:build !unix`), whose `Isolate`
isolates nothing, so a canceled streamed command's children can outlive it;
`internal/cli/environment_windows_test.go`, the test that `--log` reads a path
Windows roots elsewhere (`\logs`, `D:logs`) where Windows does, which as a
test file no job even compiles; and the `runtime.GOOS == "windows"` branch of
`SharedMode` in `internal/config/save.go`, which reports no file shared. A
regression in any of them builds, passes every job and ships, unseen until a
Windows user meets it. The functions that take the platform as an argument
(`store.Dir`, `keychain.Storer`, `taskwarrior.Candidates`, the editor's
default, `wiring.BrowserCommand`, `hooks.NewFailureScan` and
`hooks.ExistingHooks`) are not part of the cost: their tests pass `windows`
on Linux.

**Reopen when.** A Windows-specific bug is reported, Windows-only code grows
past the files and the branch named here, or the project adds a feature only
Windows has, such as the job object `pgroup.Isolate` would need (FEAT-73 in
`FEATURES.md`).
