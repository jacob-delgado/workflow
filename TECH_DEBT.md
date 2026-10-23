# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It also holds [the trade-off
register](#the-trade-off-register): the costs chosen on purpose, and what
would reopen each. It is a record, not a plan.

Two readers are in mind: a contributor looking for something worth fixing, and
a later Claude Code session asked to pick up an entry by its ID.
[FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas; this file holds
the debts.

## How to write an entry

An entry is a `### DEBT-n <title>` section under a `##` heading for its area,
above the register, with the next unused DEBT ID, DEBT-331 onward: an ID is
never reused. One line under the title rates it, as
`Severity: low · Confidence: read · Size: S`, ending in `· Breaking` when that
applies, and four paragraphs follow: **Where.** names the functions and files
rather than lines, which drift; **Today.** says what is wrong and what it
costs; **Fix.** gives one way to fix it; and **Done when.** says how to tell it
is fixed. A path in backticks is named from the repository's root, or is a
file's name alone, and `task lint:techdebtpaths` fails one that no tracked file
or directory has.

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it, and the entry names the tool) or *read*
  (independent readings of the code agree).
- **Size** is S (an hour or two), M (a day) or L (several days, or a change
  across packages that CLAUDE.md asks to outline first).
- **Breaking** marks an entry whose fix changes a contract a user or a script
  depends on: a `--json` shape, the web API, or an exit status.
- **Done when** is observable, so a test or a command can assert it.

A DEBT entry is accidental debt. A cost chosen on purpose goes in [the
trade-off register](#the-trade-off-register) instead, which lists what each
costs and what would reopen it. A security problem goes in neither: report it
as [SECURITY.md](SECURITY.md) asks.

## The trade-off register

These were chosen on purpose. They are not debt; they are listed because each
one costs something a reader should know about. Each entry is a
`### TRADE-n <title>` section, under an ID that is never reused, with three
labeled paragraphs: **Decided.** (when, and why), **Cost.** and **Reopen when.** (the
event that would reopen it). The code it keeps carries a `Trade-off TRADE-n:`
comment naming it. An entry names the functions and files it keeps rather than
lines, which drift, and the register is in ID order. An audit or a review
drops a finding that restates an entry here unless that entry's reopen trigger
has fired. `scripts/check-tradeoffs.sh`, in `task lint`, fails an entry
missing its ID or one of its three fields, and a site comment or a
`scripts/gobco-allowlist.txt` line naming an ID the register does not hold.

### TRADE-3 The web is read-only under `--dry-run`

**Decided.** Under `--dry-run` the web is read-only. `refuseWritesInDryRun` in
`internal/webserver/edge/guard.go` answers every unsafe method 403 before a
handler runs. The browser reads `dry_run` from `getHealth`, shows a banner, and
holds each write in `web/src/api/client.ts` before it is sent, so the 403 is
only the backstop. One gate over every write is what guarantees nothing is
written; simulating each write would mean a second, per-endpoint narration
beside the terminal's. Recorded 2026-09-24, in #140.

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
still ahead has passed (`useTasks`). The stream's connection state
(`reconnecting`, `stale`) is shown, so a dropped stream is visible. A write's
answer wakes the stream, so the frame that shows what it did comes at once
rather than an interval later, and the page hands the answer on only once a
frame read after the write landed is on the page: the first whose count of
writes reaches the number the answer carries (the write gate in
`web/src/api/client.ts`, `nextFrame` in `web/src/api/snapshot.ts`). That wait
has no bound of its own, so a write never says it is done beside a page that
shows it undone while the stream is live; it ends early only when the stream
goes stale or closes. A configuration save checks the file's revision and
refuses a changed file with 409; a change landing between that check and the
write is not caught (`SaveLayers`, `internal/config/layers.go`), which is
accepted for one user's local file. A pull request opened here is put into the
stream's forge cache as its seed, which the reads of the forge under way as it
is made, and the frames waiting on them, do not replace; a read begun after it
does. So a frame whose git read came before a commit and an open, but which
reaches the forge cache only after the open's seed, starts a read for the old
head and replaces the seed with what the forge says of it (`forgeCache.land`,
`internal/webserver/streamcache.go`). Recorded 2026-09-24, in #140, and
2026-10-08, in #227, for the Tasks list's interval, 2026-10-21 for the write
gate's unbounded wait, and 2026-10-22 for the seed a later read of an older
head replaces.

**Cost.** A connection that stays open but stops sending frames looks live,
and leaves stale data with no refetch to fall back on. And every write's
answer is coupled to the slowest read the frame after it makes: a frame that
starts a due forge, tracker or assigned-keys read, or waits on the one a
write here dropped the cache for, holds the write's answer until that read
ends, up to `timing.request_timeout` (10 seconds unless set) per call when an
upstream is slow or unreachable. A local stage made then reads Staging… for
that long. And when a frame replaces the seed, a forge whose list lags behind
its opens finds no pull request at the new head either, so the page shows none
for the one just opened until the next forge read, `timing.ci_interval`
(twenty seconds unless set) later; the window to replace it is under a second,
since a frame reads the forge right after git.

**Reopen when.** A stalled-but-open stream is reported, a panel is reported
showing stale data that neither the stream, its own `staleTime` nor a save
reads again, or a write is reported held behind a slow forge or tracker read
for long enough to matter: then cap the wait at about a stream interval, or
have the server push a frame of git and Taskwarrior alone for a landing
rather than one that waits on held network reads. Or a pull request opened here
is reported vanishing from the page: then hold no read of the seed's branch at
another head unless a read of that head was under way as it was seeded.

### TRADE-5 The progress spine's per-system hue is color-only

**Decided.** On the loop track each stage's glyph, name and filled rule are
drawn in its system's hue (Jira, git, forge, messaging), inside
`spineView.stages` in `internal/tui/spine.go`. The hue only repeats what the
text says: the stage's name, or its initial when compact, names the system,
and the glyph's shape and the rule's weight carry its state. A stage still to
come is drawn faint rather than in its hue, so there its identity rests on the
name alone. The web's track under the header draws it the same way (`hueOf`,
`web/src/shell/LoopTrack.tsx`): a stage under way or done wears its system's
hue, a failed one the failure color, and one not begun the faint ink, its name
beside its mark and its state said in words for a screen reader. The hues are
part of UX.md's visual system, which the web's tokens share. Recorded
2026-09-24, in #140; revised 2026-10-14, when the spine became the track and
its reopen trigger, a change to the visual system, fired; and again that day,
to name the web's track, which UX.md's visual system gained in the same
refresh, so that edit belongs to this revision rather than reopening it.

**Cost.** A monochrome or color-blind reader loses one redundant channel
telling the systems apart, on the terminal's spine and the web's track alike,
and on a stage still to come there is no hue to lose: only its name tells
which system it is.

**Reopen when.** UX.md's visual system changes after 2026-10-14's refresh, or
an accessibility report names either track's hue or its faint stages.

### TRADE-8 The web's branch floor is v8's range-based count

**Decided.** The web's coverage gate is Vitest's v8 provider, with thresholds
in `web/vitest.config.ts` held at floor(measured) − 2. v8 counts a branch
covered once its code range has run, and no Vitest provider reports both
outcomes of each condition, so the web cannot have gobco's per-condition
floor. Recorded 2026-09-24, in #140.

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
server. Decided 2026-09-26, in #145.

**Cost.** The user's own remote host name can appear in a push's problem detail.

**Reopen when.** A report asks for host names to be taken out of a push's
detail, or a push's detail comes to be shown to, logged for, or stored for
anyone other than the user who pushed.

### TRADE-11 The editor seam stays in the terminal package

**Decided.** `Environment.Deps` returns `tui.Deps`
(`internal/wiring/wiring.go`), and `cli.WebDeps` hands its seam groups, which
package `seams` declares, to `webserver.Deps` whole. Four of its fields speak
Bubble Tea: `Editor` (`EditorDeps`, whose functions take and return Bubble Tea
messages and commands because `internal/editor` hands the terminal over
through `tea.ExecProcess`), `Forms` (`FormDeps`, `internal/tui/huhform.go`,
whose `Ask` takes a Huh form and returns a Bubble Tea command, since a form's
plain prompts take the terminal as the editor does), `After` and `Copy`. So
the bundle is the terminal's type rather than one in package `seams`. Decided
2026-09-25, in #144.

**Cost.** The wiring's output is named for one surface, and the web reaches
its seams through the terminal's struct; a second surface that wants the
editor or a form's prompts must import `internal/tui`.

**Reopen when.** A surface other than the terminal needs the editor, a form's
prompts, `After` or `Copy`; `internal/editor` stops returning a Bubble Tea
command; or `--web` needs to be wired without `internal/tui`. Package
`webserver` imports no `internal/tui` (`depguard`'s `webserver-not-terminal`
rule forbids it), but `cli.WebDeps` still builds its seams from the
`tui.Deps` the wiring returns, and hands it `tui.CheckKeys`, `tui.KeyActions`
and `tui.PaneSections` as functions.

### TRADE-13 Encoding the program's own types is taken not to fail

**Decided.** Every `json.Marshal`, `MarshalIndent` or `Unmarshal` on a value the
program builds keeps its `err != nil` arm, though none can be true: the values
hold only strings, integers, booleans, finite floats, slices and maps of the
same, or values just decoded from JSON, and a `config.Config`'s JSON always
fits `api.Config`. `encoding/json` refuses a float only when it is NaN or
infinite, and neither float encoded here can be: a task's `Urgency`
(`api.Task`, which the stream's snapshot carries to `sendEvent`) is decoded
from Taskwarrior's JSON, which has no way to write either, and a count in the
terminal's Settings (`internal/tui/settingsvalues.go`) is read back from the
configuration's JSON or converted from an integer. Each site carries a
`Trade-off TRADE-13:` comment, and `rg 'Trade-off TRADE-13' internal` lists
them: 15 comments, in config, forge, jira, messaging, slackauth, tui and
webserver, keeping the 17 arms `scripts/gobco-allowlist.txt` keeps under this
entry, since one comment on `seed` (`internal/tui/settingsvalues.go`) and one
on `configDTO` (`internal/webserver/config.go`) each keep two; lines are not
listed here because they drift. Decided 2026-09-26, in #146; re-decided
2026-10-14 by the maintainer, once those two floats were found to meet the
trigger's "gains a float" to the letter: a float that is always finite encodes
as surely as an integer, so the trigger now names one that is not.

**Cost.** Seventeen error arms no test runs. Because forge's
`Client.newRequest`, jira's `Client.newJSONRequest`, messaging's
`Client.postJSON` and `config.encodeValue` accept `any`, a caller passing a
NaN or infinite float, a channel or a failing marshaler would put an arm in
play with no test behind it. The configuration's own encoders, `encodeValue`,
`indented` and `decodeBuilt` (`internal/config`), panic on such a failure, as
`regexp.MustCompile` does on a pattern that cannot compile, rather than hand
their callers an error nothing could cause, so a failure there would stop the
program rather than be reported.

**Reopen when.** A type encoded at one of these sites gains a float that can
be NaN or infinite (one neither decoded from JSON nor converted from an
integer), a function, a channel, an interface-typed field or a `MarshalJSON`
of its own; `config.Config` and `api.Config` disagree on a field's type; or
an encoding failure is reported.

### TRADE-14 What the binary embeds is taken to load

**Decided.** `loadSpec` and `loadContract`
(`internal/webserver/edge/validator.go`) and `Handler`
(`internal/webserver/edge/edge.go`) keep five `err != nil` arms for an
embedded `api/openapi.yaml` that fails to load, validate or route, and
`Assets` (`internal/web/embed.go`) keeps a sixth, which panics, for an
`fs.Sub` that cannot root the embedded web app at its `dist` directory. Both
are compiled in (`api/embed.go`, and the `//go:embed all:dist` beside
`Assets`), so only a build-time defect trips them, and that defect fails every
web server test first; a seam to feed a broken spec or app would only re-test
kin-openapi and `io/fs`. Decided 2026-09-26, in #146, for the contract, and by
the maintainer on 2026-10-09 for the web app, as the condition coverage gate
fails an arm no test reaches.

**Cost.** Six arms no test runs; the start-up message for a broken contract,
and the panic for an app that cannot be rooted, are read, not checked.

**Reopen when.** The contract or the web app is read from anywhere but the
binary (a file, a flag, a download), or a broken contract or app reaches a
release.

### TRADE-16 Failures only a change between two calls can cause

Two calls follow one that has just read or made the same file, so they fail
only when the file system changes between the two, a race no test can hold
open without a seam. Opening the store at this build's schema version:
`removeDatabase` removes the files the open that just read the version held
(`internal/store/store.go`, checked in `remakeDatabase`), and `stamp` writes
the version into a file the open just made or read (checked in
`openCurrent`).

**Decided.** 2026-09-26, in #146.

**Cost.** Five error arms no test runs: `removeDatabase`'s own, a removal
that fails for any reason but the file being gone already; `stamp`'s own
check; `remakeDatabase`'s check of `removeDatabase`; and the two in
`openCurrent` that pass them on. What each says when the race is lost is read
rather than checked.

**Reopen when.** One of these failures is reported, or a change to the calls
lets a test fail the second without the first.

### TRADE-18 Two conditions a test reaches where the report cannot see it

Two conditions are reached by a test the condition report does not count.
`closeRequestLog` (`internal/cli/cmdkit/scriptable.go`) asks whether the
request log's file could be fully written, and says so on stderr when it could
not. `TestRequestLogWarnsOnceWhenItCouldNotBeWritten`
(`internal/cli/cmdkit/reqlog_test.go`) reaches it by logging to `/dev/full`,
every write to which fails as a full disk would; macOS has none, and no other
file can be made to refuse writes from a test, so a report measured there
lists it as seen one way, and CI's, measured on Linux, does not. `keysFrom`
(`internal/tui/tui.go`) asks whether the interface's input is a file and, if
so, whether that file is a terminal, and reads no keys from a file that is
none. `TestRunReadsKeysFromATerminalItIsHanded` (`internal/tui/run_test.go`)
hands `Run` a pseudo-terminal in the test process, so the file check is seen
both ways. Only `TestRunReadsNoKeysFromAFileThatIsNoTerminal`
(`internal/tui/run_unix_test.go`) hands it a file that is no terminal, in a
child process that has no terminal at all, as a test run from a developer's
shell does not, and gobco counts only the test process, so every report lists
the terminal check as never false.

**Decided.** 2026-09-26, in #146, for `closeRequestLog`, and in #237, on
2026-10-09, for `keysFrom`.

**Cost.** `task cover:branch` on macOS reads one arm fewer than CI does, and
every report lists `keysFrom`'s terminal check as never false, though a test
fails when either arm breaks; a reader of the report could take either for
untested.

**Reopen when.** CI measures condition coverage on another system, a test
stops reaching either condition, or gobco comes to count what a child process
runs.

### TRADE-19 A condition seen one way stays the report's worklist

`task cover:branch` fails on an arm no test reached of two kinds: a condition
no test evaluated, and the error arm of an error check, each unless a line of
`scripts/gobco-allowlist.txt` keeps it for a kept entry in this register. An
error check is one the gate can tell by its name, as CONTRIBUTING.md's *Tests*
defines it, where that rule is written once. Every other condition seen only
one way, a boolean, a comparison, any other `errors.Is` or an `ok`, fails
nothing: the report lists it on every run, by file, line and how many times it
was seen each way, and it stays out of this file and out of the allowlist.
CLAUDE.md reads that list as "a worklist of missing test cases, not a
percentage to chase", so a test for one is written when the code around it next
changes, and two floors keep the share seen from falling: `BRANCH_COVERAGE_MIN`
in `Taskfile.yml` for the module, and `scripts/gobco-floors.txt` for each
package it names.

Measured on Linux on 2026-10-12, as the dev user, not root, over the code as
the commit that last wrote these figures has it: of 6,176 conditions, 5,645
were seen both ways and 531 one way, and none never ran, so 11,821 of 12,352
arms were seen, 95.7% against `BRANCH_COVERAGE_MIN` 93. Of the 531, the
allowlist keeps 29 error arms for TRADE-13, TRADE-14, TRADE-16 and TRADE-36 (a
30th line, TRADE-18's, holds only on macOS), and the other 502 are the
worklist this entry keeps. Two runs of the same code can differ by an arm or
two: gobco's counters are plain increments, which a package's parallel tests
can lose (`scripts/gobco-report.sh` says so), and two runs of the same
`internal/config` code, this one and CI's, saw 500 and 501 of its 520 arms.
Since then CI's coverage comment, on the pull requests merged on 2026-10-14
(#268, #269 and #270), has read 95.6%. These figures are measured again on
2026-11-14, the day TRADE-42 is looked at again too.

**Decided.** 2026-09-26, in #146, and 2026-10-12, in #238, for which
conditions the gate fails: a condition seen one way that the gate does not
fail is left to the report.

**Cost.** A change that breaks one of the worklist's unseen arms fails no test,
and the floors notice only when enough of them add up to move a share; an
error check under a name the gate does not read, or one held in a variable
first, is read as a boolean. The allowlist names a kept arm by its number among
its function's conditions that read the same rather than by its line, so a
condition added above it that reads the same moves the number on, and the gate
fails until the line is changed to follow it, but for one change it cannot
see: when a test comes to reach a kept arm in the same change that adds an
untested condition reading the same above it in that function, the new arm
takes the kept one's number, the line keeps it, and the gate passes. Without
source lines the gate cannot tell the two apart, and
`scripts/gobco-report_test.sh` pins that case as one that passes.

**Reopen when.** The measured figure comes within one point of
`BRANCH_COVERAGE_MIN`, a defect ships through a condition the report listed
as seen only one way, or a kept arm is found traded for a new one as the cost
above describes. Whether or not one has, the entry is read again on
2026-11-14 against the figures measured that day.

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
issue's marks in the snapshot for a filter that runs in the browser.

**Cost.** A change to what a place means is made twice, against one shared
case file that fails whichever copy disagrees with it; a rule the file holds
no case for can still differ between the two.

**Reopen when.** The snapshot comes to carry each issue's marks for another
reason, or the two copies are found to disagree on a case the file does not
hold.

### TRADE-24 The combined tracker reads its settings once

**Decided.** Jira's client settings are read once, when the wiring is built:
its address, user and headers, and its token with where the token is read
from (`jira.token`, `jira.keychain`, `jira.token_env`,
`jira.token_command`). So is the combined tracker
(`internal/wiring/tracker.go`): whether the forge's issues join in, and the
JQL of the first view they lead, are fixed then. The web makes the forge and
messaging settings live on save through the wiring's controls, and reads
Jira's project, review status, views and Markdown comments from the
configuration at each use. Making the tracker's two live would mean always
building the combined tracker and threading another control through every
tracker seam, while Jira's client settings would still wait for the next
wiring. The forge-row count that offsets later pages is one per tracker,
shared by every tab paging that view. Decided 2026-10-01, in #164.

**Cost.** A save from the web's Settings that turns `issues.forge` on or
off waits for `workflow --web` to restart or switch directory, as one that
changes Jira's address, user, headers, token or where the token is read from
does, and until then the forge's issues stay with the first view's JQL as it
was: editing that JQL drops them from the first view, and putting another
view first leaves them leading the view that was first. The web's Jira
section, `configuration.md` and `web.md` say so. The terminal's Settings
reopens workflow to apply a save, asking first while a write is in flight or
reopening would lose something. Two tabs paging the first view while the
forge's open issues change can skip or repeat one Jira row on a later page.

**Reopen when.** Jira's client settings become live on save, or someone is
seen toggling `issues.forge` from Settings and expecting it to apply at once,
or editing the first view's JQL from Settings and expecting the forge's issues
to follow it.

### TRADE-25 Kept data has one schema and is never migrated

**Decided.** `kept.db` holds what the user decided and no session can see
again: owner links, repository groups and favorite directories
(`internal/store/kept.go`). It has one schema, `keptSchemaVersion`, and no
migrations. Its one bump, to 2, shipped in v0.6.0, and a user of v0.5.0's
`go install` or release binaries could upgrade across it; no kept table has
changed since `brew upgrade workflow`, which carries a Homebrew install from
one release to the next, arrived in v0.10.0, and the next change to one
decides a migration path first. A file at another version is never
discarded: it reads as empty and refuses writes with `ErrKeptSchemaDiffers`,
which names `workflow db-clean --all`. Decided 2026-10-01, in #165, on the
ground that workflow is installed fresh before 1.0; on 2026-10-14 the
maintainer moved the trigger from 1.0 to the next kept-table change, since
Homebrew users upgrade now.

**Cost.** Each change to a kept table leaves files from earlier builds unread:
tagging and favorites go blank until the user runs `workflow db-clean --all`
and answers again.

**Reopen when.** The next change to a kept table, the first a Homebrew user
upgrades across; a migration path is decided before that change ships, not
after.

### TRADE-26 The Slack directory is read whole, once a session

**Decided.** To offer a channel's members by name,
`internal/messaging/directory/directory.go` reads `users.list` whole (Slack's Tier 2,
up to `messaging.UserListPages`, 20 pages, about 4,000 people) and labels
members from it; past that cap it labels the channel's members one by one
through `users.info` (Tier 4). Every read is shared by everyone asking at
once, held for ten minutes or until a refresh, and never awaited under a lock.
A shared read runs to its end even if the one who began it leaves, and ends
with the command it runs in (`lifetime.Detached`): when the web server stops,
when a signal ends the command, or when workflow exits after the terminal's
last quit. A session started by switching directory (TRADE-34) reads afresh.
Decided 2026-10-01, in #165 and #166.

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

**Decided.** 2026-10-01, when the web gained commenting: Preview redraws on
every keystroke, and asking the server for each one would be a write under the
dry-run guard's rule (it refuses every non-GET) or a GET carrying the whole
comment in its URL. Converting in the browser shows exactly what will be sent
with no request at all. And 2026-10-05, in #174, for a forge issue's thread and
Preview, which the web's copy also draws, converting their Markdown to wiki
markup for `WikiText`: that reuses the drawing every Jira comment already has
rather than adding a Markdown renderer.

**Cost.** A change to how a Markdown construct converts is made twice, against
one shared case file that fails whichever copy disagrees with it; a construct
the file holds no case for can still convert differently. In a forge issue's
thread, Markdown the conversion does not know, such as a table, shows as text.

**Reopen when.** The server comes to render comments itself, or the two
copies are found to disagree on a construct the file does not hold.

### TRADE-30 A comment is written in a box drawn inside the interface

**Decided.** A Jira or forge comment is typed in the comment box
(`commentComposer`, `internal/tui/commentcomposer.go`), a Bubbles text area
with vim's normal and insert modes, rather than in `$EDITOR` as a commit body,
a pull request and an announcement are; ctrl+o still hands the draft to
`$EDITOR` and back. A comment is short and answers the issue beside it, which
a full-screen editor would hide, and the box needs no editor configured.
Normal mode knows only moving and entering insert mode (`h`, `l`, `j`, `k`,
`i`, `a`, `A`, `o`), so a key that types is never taken for a command.
Decided 2026-10-05, in #173: building a fuller editor would cost more than
the comments it serves. The web's comment box (`CommentComposer`,
`web/src/features/issues/CommentComposer.tsx`) is the browser's own text area
and was already on main when this was decided, so the trigger's second
surface is a second place in the terminal that wants a box of its own, and
the maintainer read it so on 2026-10-14: the composer is still the
terminal's one text area, so nothing has reopened this.

**Cost.** The box is a worse editor than the user's own: no operators, no
undo, no search, so vim's `dd` or `u` does nothing. Text that comes back from
`$EDITOR` or a paste passes the text area's filter, so a tab becomes four
spaces and a control character is dropped. It is one more widget to keep
working as Bubbles changes.

**Reopen when.** Users ask for editing commands the box lacks, a comment
written in `$EDITOR` is found posted wrong because of the filter, or a second
place in the terminal wants a box of its own.

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
would be a parser of its own to keep. Decided 2026-10-05, in #174: the bound
is a few lines and binds only on text no one reads formatted.

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
could disagree with the first. Decided 2026-10-05, in #175: a journal is a
feature with its own schema, not a fix. The web's calendar reads back each
month it shows the same way, to say how much was done on each day of it
(`web/src/features/summary/summaryApi.ts`), and is shown again within 30
seconds without a second read; a month that starts after today is not read.

**Cost.** A period is as slow to show as its slowest source, and a source that
is down shows nothing for it. What a source does not keep (a Jira comment on
an issue touched no other way, a GitLab event's repository, when a stopped
Taskwarrior task was started) the Summary cannot show. Nothing remembers a
Summary was posted, so the same one can be posted twice. The web's calendar
spends a read of every source on each month it shows, on top of the period's
own read: on GitHub, three searches of the 30 a minute allows, and on Jira
up to two pages. A source that caps what it reads may not read a busy month
whole, and then its days say "at least" so much was done, under a line saying
why.

**Reopen when.** A source the Summary needs keeps no history to read back,
reading a long period back proves too slow to be useful, a repeated post is
reported, or paging the calendar is seen to spend a forge's or Jira's rate
limit.

### TRADE-33 The calendar's dates are written twice

**Decided.** The Summary's dates (a day, a week or a month moved, a month's
last day, a whole month or year, a month as weeks from Monday, and a period
stepped by its own length) are worked out in `internal/activity/period.go` and
again in `web/src/features/summary/civilDate.ts`, both answering to one case
file, `internal/activity/testdata/civil_dates.json`. The month as weeks is the
layout both calendars draw: `Period.Weeks`, which the terminal's Day column
draws (`calendar.dayColumn`, `internal/tui/calendar.go`), and `monthWeeks`,
which the web's `MonthGrid` does, each held to the file's weeks cases. The
web's calendar moves on every key (`MonthGrid`'s arrows, Page Up and Page
Down, Home and End), so it cannot wait on a request for arithmetic. The server
still decides the default period and reads every period, so only the moves are
twinned, never which days were worked. Decided 2026-10-05, in #175: nearly all
of it is the Gregorian calendar, whose rules do not change, and the one rule
of workflow's own, that a whole month or year steps as one, is a few lines.
The browsers the web supports are the latest two versions of Chrome, Edge,
Firefox and desktop Safari, written as the `browserslist` in
`web/package.json`, which `npx browserslist` in `web/` resolves to versions;
decided 2026-10-14 by the maintainer, so the trigger below names something
that can be checked. Safari is desktop Safari alone, browserslist's `Safari`,
and never iOS Safari, `ios_saf`: the page is only ever served on 127.0.0.1,
to a browser on the machine `workflow --web` runs on.

**Cost.** A change to how a period steps is made twice, against one shared
case file that fails whichever copy disagrees with it; a mistake in one copy's
leap-year or month-end arithmetic on a date the file holds no case for still
passes that copy's tests.

**Reopen when.** The two copies are found to disagree, the step rule changes,
or `Temporal.PlainDate` is in every browser the web supports, so the browser's
copy can become the platform's. On 2026-10-14 the `caniuse-lite` the web
installs (1.0.30001815) has Temporal in both supported versions of Chrome,
Edge and Firefox, and in neither of Safari's, 26.6 and 27.

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
Decided 2026-10-05, in #176: rebinding live would be a second construction
path to keep in step.

**Cost.** A switch reads every pane again and drops what belonged to the
repository left (a commit message, a pull request or a forge issue's comment
being written, an announcement waiting for CI), so it asks first when any of
those would be lost, and refuses while an announcement or a change to a task
is in flight (`writeInFlight`, `internal/tui/reposwitch.go`); any other write
a pane sent goes on in the directory left, unshown, as TRADE-38 says. A
session started by a switch reads the Slack directory afresh (TRADE-26).

**Reopen when.** A switch is found slow enough to notice, or the terminal is
found left changed by one in practice.

### TRADE-35 Code only another system runs is never run in CI

**Decided.** Every CI job runs on `ubuntu-latest`. The Cross-compile job
(`cross` in `.github/workflows/ci.yml`) runs `task release:binaries` and
`task release:verify`, which build a binary for every `RELEASE_PLATFORMS`
target in `Taskfile.yml`, `darwin/arm64` and `windows/amd64` among them, and
the Lint job's `task lint` runs `task lint:crossvet`, which vets each of those
targets' build with its tests. So the code only a macOS or a Windows build
holds, test files and all, is compiled and vetted on every change, but no job
runs it. Decided 2026-10-09 by the maintainer, in #237, for the code only
Windows runs, and 2026-10-12, in #241, for the code only macOS runs: CI stays
on Linux, and the cost is recorded here. The vet was added on 2026-10-14, by
the maintainer's decision in the review of this register, as the part of the
cost a Linux runner can pay.

**Cost.** Five files are Windows-only, one is macOS-only, and four functions
have a branch or a value only Windows takes, each marked with this entry's ID:
`internal/filelock/lock_windows.go` (`//go:build windows`), the `LockFileEx`
lock `TryLock` takes, which keeps two sessions from writing the Slack
credentials at once; `internal/fileowner/owner_other.go` (`//go:build !unix`,
which among the release targets builds for Windows alone), whose `Of` knows no
owner, so the configuration's trust check refuses nothing there;
`internal/proc/pgroup/pgroup_other.go` (`//go:build !unix`), whose `Isolate`
isolates nothing, so a canceled streamed command's children can outlive it;
`internal/cli/cmdkit/environment_windows_test.go`, the test that a request
log named by a path Windows roots elsewhere (`\logs`, `D:logs`) is opened
where Windows reads it, which as a test file only `lint:crossvet` compiles,
and which stands beside the two clauses of `relativeToWorkingDir` in
`internal/cli/cmdkit/environment.go` it covers:
`filepath.VolumeName(path) == ""`, false only for a path naming a drive, and
`!rooted`, false only for a path rooted but not absolute, which elsewhere a
rooted path never is, so on Linux neither is ever false and no job sees
`--request-log` keep such a path off the working directory;
`internal/ptytest/open_other.go` (`//go:build !linux && !darwin`, which among
the release targets builds for Windows alone), whose `openPair` opens no
pseudo-terminal, so the tests that type at a terminal
(`internal/cli/prompt_terminal_test.go` and
`TestRunReadsKeysFromATerminalItIsHanded` in `internal/tui/run_test.go`) skip
there; `internal/ptytest/open_darwin.go` (built for macOS alone by its name),
whose `openPair` opens a pseudo-terminal as macOS makes one, granting and
unlocking it and naming it with `TIOCPTYGNAME`, so on macOS those same tests
type through code no job runs; the `runtime.GOOS == "windows"` branch of
`SharedMode` in `internal/config/save.go`, which reports no file shared; the
`goos == "windows"` branch of `defaultEditor` in `internal/editor/editor.go`,
which opens notepad where no editor is set and which no test reaches, since the
package only ever passes it `runtime.GOOS`; and the
`filepath.VolumeName(program)` that `probe` in
`internal/taskwarrior/taskwarrior.go` starts a detection run from, the root of
the drive the program is on, which is empty everywhere else, so no job sees a
probe run from anywhere but `/`. Only `lint:crossvet` compiles either of
ptytest's files, since only tests import the package. What the vet catches is a
change that no longer compiles for one of those systems, or that vet's
analyzers fault there; a regression that compiles and vets clean, a lock that
never holds or a branch that returns the wrong value, passes every job and
ships, unseen until a Windows user meets it, or, in `open_darwin.go`, until a
test fails or skips on a developer's Mac. The functions that take the platform
as an argument (`store.Dir`, `keychain.Storer`, `keychain.Open`,
`slackauth.Choose`, `taskwarrior.Candidates`, `wiring.BrowserCommand`,
`Environment.PlaceSlackCredentials` in `internal/wiring`,
`hooks.NewFailureScan` and `hooks.ExistingHooks`) are not part of the cost:
their tests pass `windows` or `darwin` on Linux.

**Reopen when.** A Windows- or macOS-specific bug is reported, code only
either system runs grows past the files and the branches named here, or the
project adds a feature only Windows has, such as the job object
`pgroup.Isolate` would need (FEAT-73 in `FEATURES.md`).

### TRADE-36 The stat of a file just opened is taken not to fail

**Decided.** `readTrusted` (`internal/config/layers.go`) asks `file.Stat()`
of the configuration file `readLayer` has just opened, so the owner and mode
it holds to `refuseUntrusted` are those of the file it goes on to read. A stat
by path would look the file up again, and a file swapped in between would be
checked in place of the one read. A stat of a descriptor the process holds
open fails only for a fault no test can cause without a seam to fake the file,
so its error arm keeps no test; it stays, so such a failure is reported rather
than read past. Decided 2026-10-09 by the maintainer, as the condition
coverage gate fails an arm no test reaches.

**Cost.** One arm no test runs: what a configuration file whose stat fails
says is read, not checked.

**Reopen when.** A stat failure is reported, or the trust check moves to a
path.

### TRADE-37 A web write outlasts its page

**Decided.** `worlds.serve` (`internal/webserver/world.go`) runs every write,
any request but a GET, HEAD or OPTIONS, under a context detached from its
request's (`worlds.outlivingItsPage`) that ends only when the server's
lifetime, the context `Handler` is given, does. A read stops when its page
goes; a write, a commit, a push, a post or a task started, goes on to its end,
so a page closed or reloaded mid-write never cuts one off halfway. It is what
`docs/content/docs/web.md` promises a run: it goes on to its end if the page
is closed. Decided 2026-10-09 by the maintainer, with each seam taking its
asker's context.

**Cost.** Closing a tab cannot call off a slow write; only Stop, for a run,
or stopping the server can.

**Reopen when.** A write is found to hang with no way to stop it.

### TRADE-38 The terminal stops its reads on quit, not on leaving a view

**Decided.** The terminal asks every system under one session context, set by
`Model.WithContext` (`internal/tui/options.go`), which `tui.Run` ends when the
program does. Quitting (`q`, or `ctrl+c` from anywhere) or switching directory
(TRADE-34) therefore stops every read the program left in flight, and no write,
but three pieces of work a read can begin: a shared read of the Slack directory
(TRADE-26) and the renewal of a Slack user token, which detach from their asker
through `lifetime.Detached` to the command's context, and the store's making
and stamping of its file (`internal/store`), which runs detached so a stamp
fails only as TRADE-16 says. Each runs to its own end, cut short no later than
the command's: once workflow exits after the quit, or when a signal ends the
command. The writes stay out of the session's end: `New` hands each write seam
the session's values but not its end (`outlasting`, `internal/tui/deps.go`,
over the wrappers in `internal/lifetime/outlasting.go`), and
`Model.WithLifetime`, beside `WithContext`, which `tui.Run` applies with the
context it is given, ends the writes with that context instead, and counts
each in flight, under a name that says what it does, in the
`lifetime.InFlight` that context carries.
The command line gives it `whileOpen`'s (`internal/cli/cli.go`), which ends
when SIGINT or SIGTERM cancels the command's context, until the command is
done. When the last interface quits, workflow waits for the writes still in
flight before it exits (`finishWrites`, `internal/cli/finishwrites.go`): it
says on stderr how many it is finishing, and exits once none has been in flight
for a tenth of a second, so a write that follows another (a post's record after
the post) is waited for too. Every quit settles so, even one that lands with
nothing in flight: between those two writes, or after the interface has started
a write's command and before that write has begun, where a quit would otherwise
exit under the write. A streamed write (a commit, a push, an amend, a fixup, a
rebase, a hook) counts until its program exits, and no longer: once a quit or a
switch leaves its output unread, the rest is read and dropped
(`releasedOnExit`, `internal/lifetime/outlasting.go`), so the program never
blocks on a full pipe. The wait gives up after `timing.request_timeout`, naming
each write it gave up on, and exits with status 1. A second `ctrl+c`, which
the terminal the interface left sends as SIGINT, or SIGTERM ends every write's
context and exits at once, with status 130. A switch does not wait: a write
sent before one goes on in the directory left, and the quit waits for it too.
Leaving a view cancels nothing: a late answer is dropped by its read's count or
view, and canceling per view would mean a cancel handle on every view's state.
Decided 2026-10-09 by the maintainer for reads, with each seam taking its
asker's context, and on 2026-10-12, in review, for writes: they stay out of the
session's end, and a quit waits for them, bounded.

**Cost.** A read for a view the user has left still spends its request to the
tracker, the forge or Taskwarrior. Quitting cannot call off a slow write: it
waits for it, up to `timing.request_timeout`. One that takes longer is given up
on and workflow exits under it: a write in workflow's own process (a post, a
Jira or forge change over HTTP, a settings save, a record in the store) ends
with the process, and a program a write runs (git, Taskwarrior, lefthook, `gh`,
`glab`) runs on until it next writes to the output workflow no longer reads,
when the broken pipe ends it: SIGPIPE does on Unix, while on Windows the write
fails instead, which git exits on, and a program that ignores the failure, as
`gh` or `glab` may, finishes. A signal during the wait does not wait for the
programs it stops either, nor for the work `Execute`'s comment names as going on
past its asker's context: a signal that exits while Settings is refreshing a
Slack user token on macOS, once Slack has taken the refresh and before the
keychain keeps the new pair, can lose the only pair that works, since Slack
spends the old one as it answers, and so can a wait given up on there, when the
keychain takes longer than the wait has left; `workflow slack login` then sets
the token up again. A write the interface chains on another's outcome once that
outcome reaches it (a commit's scope, remembered once the commit is shown) is
never begun after a quit. A write a pane sent just before a switch goes on in
the directory left, and its outcome is not shown; the switch refuses only while
an announcement or a change to a task is in flight.

**Reopen when.** Such reads are found to crowd a rate limit, or a write is
reported in practice left half made after workflow exits (a stale index lock, a
push whose tracking ref was not updated).

### TRADE-39 pre-push runs by hand without what git hands it

**Decided.** A run of a hook by hand (the terminal's `h`, the web's **Run a
hook**, `POST /api/runs` with kind `hook`) offers `pre-push` although git hands
it the remote's name and URL as arguments and the refs it pushes on standard
input, which a run by hand has none of. `handedByGit`
(`internal/hooks/githooks.go`) lists every other hook git hands arguments or
standard input as one only git runs, and leaves `pre-push` out. lefthook works
out what a push would carry from `@{push}` itself, falling back to the default
branch's merge base before the first push, so a run by hand checks what the
next push carries; `pre-push` is where a repository's slow checks live, which
the only other way to try is to push; and inventing git's arguments would set
off Git LFS's upload, which lefthook skips when it is given none. Decided
2026-10-13 by the maintainer, when `h` came to run any hook.

**Cost.** A `pre-push` job that reads `{1}`, `{2}` or `use_stdin` sees nothing
when run by hand, so it can pass or fail where a real push would not.

**Reopen when.** A repository's `pre-push` job is found reading git's arguments
or refs, or lefthook gains a way to run `pre-push` for a named remote without
pushing.

### TRADE-40 Every announcement reads its CI again as it goes

**Decided.** `loop.Deliver` (`internal/loop/announce.go`) reads the pull
request's CI through `ClearToAnnounce` before it posts, whatever a surface read
before it: the command line just before it asks, the terminal from its last
poll, the web from its preview or a frame an interval old. A failed check
refuses the post, and a CI that cannot be read refuses it too, since nothing
failing cannot be confirmed. Every path that announces (`workflow announce`,
the terminal's preview, the web's Announce now, a post held for CI) goes
through it, so none can let a failed check through on a stale reading. Decided
2026-10-13 by the maintainer, gating every announcement on "nothing failed".

**Cost.** One more forge request per announcement, two where a held post's
verdict read the CI just before; and a check that fails between the two reads
sends a held post back to waiting rather than out. A forge that cannot be
reached for that one read keeps an announcement from going that the reading a
moment before would have let through.

**Reopen when.** A forge's rate limit is found spent on these reads.

### TRADE-41 An armed announcement gives up on a pull request that reports no check for ten minutes

**Decided.** `loop.NoCIPatience` (`internal/loop/armed.go`) bounds how long
an announcement armed to go once CI passes waits for a pull request that
reports no check: ten minutes, counted from the last read that reported one,
or from arming. It then gives up, saying to announce it by hand, the same in
the terminal, on the web and under `workflow announce --when-green`. A
repository with no CI would otherwise have its pull request read every
`timing.ci_interval` for as long as the terminal or the server runs. Decided
2026-10-13 by the maintainer, with the automatic post.

**Cost.** A CI that registers its first check later than ten minutes after
the last one reported, or after arming, loses its automatic post; it is
announced by hand.

**Reopen when.** A CI is met that reports its first check later than that.

### TRADE-42 The formula is audited by whatever Homebrew the runner image ships

**Decided.** The Cross-compile job (`cross` in `.github/workflows/ci.yml`)
runs `scripts/release/check-formula.sh`, `brew audit --strict` in a throwaway
tap and in developer mode, on the formula
`scripts/release/package-manifests.sh` writes for every change, with the
Homebrew the `ubuntu-latest` image installs under `/home/linuxbrew`: put on
PATH by `brew shellenv`, never updated in the run, and pinned nowhere,
`mise.toml` included. No action sets it up, so the workflow gains none to
approve and pin. Decided 2026-10-14 by the maintainer, with the check, over a
pinned `Homebrew/actions/setup-homebrew` step; developer mode was added the
same day, at the maintainer's request.

**Cost.** The Homebrew that audits a change is unpinned, against the rule that
tool versions are exact and live in `mise.toml`: it moves when GitHub rebuilds
the image, without anyone here deciding it. A rule a newer Homebrew adds can
turn an unrelated pull request's Cross-compile red until
`package-manifests.sh` is changed to keep it. Each run also needs the network:
brew downloads its formula API from formulae.brew.sh and, on a runner without
them, installs the gems its audit and style cops run on from rubygems.org, so
either one unreachable turns the job red too; brew's own error, above the
script's hint, then names the network rather than the formula. The audit runs
on Linux, where the formula's `if OS.mac?` is false, so the macOS url and
sha256 get only the RuboCop cops that read the file's text, not the audit's
checks of the url and checksum the formula loads with: the blind spot TRADE-35
keeps for code only macOS runs. Developer mode is what turns a deprecation
the formula calls into a failure rather than a silent pass, but it fails the
audit on one in brew's own code just the same, which only a newer Homebrew
fixes. The audit loads the formula without running its install or test block,
so a call deprecated or changed there shows only once someone installs it.
And `task check` does not run it, since it needs brew: a developer runs the
script by hand where Homebrew is installed, or leaves it to CI.

**Reopen when.** The audit fails a pull request over a Homebrew change more
than rarely, or a pinned `Homebrew/actions/setup-homebrew` becomes worth
approving as a new action. The audit had run for about a day when this was
written, and no Cross-compile job failed in the 80 CI runs up to 2026-10-14,
so "more than rarely" is judged on 2026-11-14, over the month of runs since,
the day TRADE-19's figures are measured again too.

### TRADE-43 A wait on a terminal prints its artifact on stderr

**Decided.** While a slow command waits with a person watching, its live
line is an inline Bubble Tea program at the foot of stderr
(`Notes.Watched`, `internal/cli/console/live.go`), and everything written
to stderr meanwhile is printed above it through that program
(`console.Stream`). When stdout is the same terminal, `artifactBeside`
(`internal/cli/cmdkit/prompt.go`) prints the command's artifact above the
line too, through the program and so on stderr, rather than on stdout under
the line's feet, where each would overwrite the other. With stdout piped or
redirected, the artifact goes to stdout untouched. Decided 2026-10-14 by the
maintainer, with the live line: the wait runs as the program's first
command, so no goroutine of this module's own keeps the line turning.

**Cost.** During a wait — `doctor --online`'s report, today — the artifact a
person sees at the terminal reaches it on stderr: a program reading a
terminal's stdout, such as an `expect` script on a pseudo-terminal, finds
it on the other stream, and `2>/dev/null` at a terminal hides it with the
line. A blank line printed above the line is a space, since the program
prints nothing for an empty one. While the line is kept, the cursor is
hidden and bracketed paste is on; both are restored when it ends, but a
process killed outright leaves them as they were.

**Reopen when.** A script is found reading a terminal's stdout during a
wait, or the goroutine gate allows a ticker to keep the line, which would
let the artifact stay on stdout.

### TRADE-44 Fang shapes the help, capitals and all

**Decided.** `execute` (`internal/cli/cli.go`) runs the command tree through
`fang.Execute` (`charm.land/fang/v2`), which draws the help, the usage and
the version, in the terminal's own colors from `ansiScheme`. Fang sends its
titles through `strings.ToUpper` and capitalizes the first word of each
flag's and command's help, and neither can be turned off; and it links four
man-page libraries, `muesli/mango-cobra` and `muesli/roff` directly and
`muesli/mango` and `muesli/mango-pflag` through `mango-cobra`, for a `man`
command that `fang.WithoutManpage()` leaves out. Decided 2026-10-14 by the
maintainer, with the CLI's look, accepting both.

**Cost.** The help reads "USAGE", "FLAGS" and "Hold back every write…",
where the flag help in the code and the reference generated from the same
tree (`docs/content/docs/reference/`) read in lower case; and a flag's
placeholder, such as `FILE`, shows in the help between backticks rather than
after the flag. Fang pads the help's lines to its width with spaces, and
reads the color profile, the terminal's width and its background from the
process itself (`os.Environ`, `os.Stdout`, `os.Stdin`) rather than from the
`Environment` every command is handed: the help's color answers to the
shell's `TERM` and `NO_COLOR`, as colorprofile reads them, and not to a
test's, so the test that the help is colored on a terminal sets `TERM` on
its process and runs alone. On a terminal, a help or an error first asks the
terminal for its background, through stdin and then stdout; a terminal that
answers neither that nor the device-attributes request beside it holds the
command up to two seconds each time. The four man-page libraries are compiled
into a binary that never builds a man page.

**Reopen when.** Fang lets its titles and first words keep their case, a man
page is wanted, or a terminal that does not answer the background query is
found slowing an error.

### TRADE-45 The track keeps five stages, with CI as the fill of Review's rule

**Decided.** The loop track draws the five stages `progress.Stages` derives —
Issue, Branch, Commits, Review and the messaging service's — with no stage of
its own for CI: the pull request and its CI are one forge stage, Review, and
CI's progress is how far the rule after Review is filled, its checks finished
over its checks (`spineView.fillOf`, `internal/tui/spine.go`). Nothing in
`internal/progress` or the API changes, so `workflow status` and the web read
the same five stages. Decided 2026-10-14 by the maintainer, with the track.

**Cost.** A pull request opened with CI not yet started and one whose CI is
under way both draw `◐ Review`; only the rule's fill tells them apart, and on
GitLab, which reports a pipeline as a whole rather than check by check, the
rule stays open until the pipeline passes.

**Reopen when.** A user asks to see the pull request and its CI as separate
steps, or GitLab's checks are read one by one.

### TRADE-46 A form's text fields are a Huh field of our own

**Decided.** The terminal's multi-field forms are Huh forms embedded in an
overlay, with Huh's own groups, selects and accessible prompts, but each field
that takes typed text is `quietinput.Input` (`internal/tui/quietinput`), a
`huh.Field` of our own over a text input whose cursor never blinks. Huh's
`Input` blinks and offers no way not to: each focus and each key schedules a
blink that waits about half a second of real time. A running program pays
nothing for that, since each blink cancels the one before, but a test that
settles after every key pays it in full: with Huh's inputs in the status
change's text fields alone the terminal's tests took about 56 seconds rather
than 34, and the first run's forms, typed a URL and a token at a time, took
their tests from about 8 seconds to 3 and a half minutes. Decided 2026-10-14,
with Huh, after measuring that cost. Later that day the pull request form's
base came to need suggestions, and a token's and a webhook's fields a mask;
each was added to `quietinput.Input` (`Suggesting`, `Secret`), so no form
came to need what only `huh.Input` offers, and the maintainer read the trigger
so: neither reopened this.

**Cost.** `quietinput.Input` does for one line what `huh.Input` does — its
title, placeholder, prompt, check, keys, theme, width, suggestions and mask —
so a fix or a feature Huh adds to its input, such as a character limit, is not
ours until it is added here. Its accessible prompt is Huh's own input, so that
one stays Huh's.

**Reopen when.** Huh lets an input's cursor be static, or a form needs what
only `huh.Input` offers. That is where extending `quietinput.Input` stops
being worth it: a feature that is an option of the text input beneath, such as a
character limit, is still added here, but a field whose title, description,
placeholder or suggestions follow another field's value, as Huh's
`TitleFunc`, `DescriptionFunc`, `PlaceholderFunc` and `SuggestionsFunc`
re-evaluate on each change, would copy Huh's machinery for watching what they
bind to, and that is the cost this entry does not take on.

### TRADE-47 An accessible form takes the whole terminal while it asks

**Decided.** With `ui.accessible` on, or the `ACCESSIBLE` environment variable
set, the terminal does not draw a multi-field form over the screen: it hands
the terminal to Huh's accessible prompts, through `tea.Exec` the way the
editor takes it (`promptedForm` in `internal/tui/huhform.go`), and they ask
one plain question at a time, which a screen reader reads as it would any
command's output. The interface comes back once the prompts are answered.
Decided 2026-10-14 by the maintainer, with Huh.

**Cost.** While the prompts ask, the panes, the spine and the footer are gone:
nothing else on screen can be looked at or done until the form is answered or
its prompts end, and prompts that end early leave the change unsent, said
under the picker when the interface returns.

**Reopen when.** The interface gains a screen-reader mode of its own (UX-154),
or a user of the prompts asks to see the screen while answering.

### TRADE-48 The web page's face carries latin alone

**Decided.** The `--web` page is set in Recursive
(`@fontsource-variable/recursive`), and `web/src/index.css` declares one
`@font-face`, the package's latin file with the weight and MONO axes
(`recursive-latin-mono-normal.woff2`, 72,560 bytes), swapped in as it arrives.
The latin-extended and Vietnamese files, and the CASL, CRSV and slant axes, are
left out. Decided 2026-10-14 by the maintainer, in the UX refresh's web pull
request, over carrying every subset the package ships.

**Cost.** Text outside the latin range a forge, a tracker or a person writes —
a name in Cyrillic, a title in Greek, a Vietnamese comment, even a Polish ł —
draws in the platform's own face beside Recursive, glyph by glyph, so one line
can mix two faces. The binary also carries the font: 72.6 kB more in every
build, whether or not the page is ever served. Until the face arrives, the
first frame draws in the fallback and then swaps.

**Reopen when.** Someone reads the page in a language the latin file does not
cover often enough that the mixed faces matter, or the package ships one
variable file covering more scripts at a size near this one.

### TRADE-49 The directory picker draws at most 200 rows, with no virtual list

**Decided.** A listing holds at most `workdirs.ListLimit`, a thousand
directories, and each surface draws at most 200 of the ones matching what was
typed, then says how many more match and to keep typing: the terminal's
directory picker by `rowsShownMost` (`internal/tui/dirrows.go`), the web's
directory dialog by `rowCap`
(`web/src/features/repositories/DirectoryDialog.tsx`). The cap is how many rows
a surface draws, not something a listing is, so it stays out of the API, and
each surface keeps its own constant; one file,
`testdata/twins/directory_rows.json`, holds the 200 both answer to. Each
surface's tests decode it strictly and draw past it, so a cap that differs
from the file's fails that surface's own tests. The thousand is the server's:
`GET /api/directories` reports it as `truncated_to` on a listing cut short,
and the dialog's "First N shown" line reads it from there; the terminal's own
line names `workdirs.ListLimit`. A listing cut short at the thousand is asked
for again with the name typed. No virtualized list, and no library for one
such as TanStack Virtual, was adopted. Decided 2026-10-14 by the maintainer,
with the picker; the shared file was the maintainer's call the day after, in
place of this entry alone holding the two 200s to each other.

**Cost.** In a directory of more than 200 that match, the rest cannot be
scrolled to: they are reached only by typing more of a name, or by a path
typed whole. The cap is written in each surface's constant and in the shared
file, which the tests hold together, and in prose that nothing holds to it:
`docs/content/docs/usage.md`, `docs/content/docs/web.md`, `web/CLAUDE.md` and
this entry. A change to it is made in every one.

**Reopen when.** Someone needs to scroll past the first 200 rather than type,
or a listing's cap is raised enough that typing to narrow stops being quick.

### TRADE-50 Base UI's ScrollArea and Select stay off the page

**Decided.** The `--web` page draws its dialogs, its command palette and
directory picker, Settings' sections and the announcement's tag picker with
Base UI (`@base-ui/react`), whose Dialog, Autocomplete, Tabs and Combobox it
takes through `web/src/lib/baseui` alone, under a `CSPProvider` with
`disableStyleElements`: the content policy (`internal/webserver/edge/guard.go`,
`default-src 'self'`, no nonce) refuses the one `<style>` element Base UI
writes, which only ScrollArea and Select (with `alignItemWithTrigger`) do, to
hide a native scrollbar. So neither part is used, and every choice from a
fixed list stays a native `<select>`; a choice narrowed by typing, as the tag
picker offers a channel's members, is the Combobox, and a choice of section is
Tabs, neither of which writes one. Decided 2026-10-14 by the maintainer, in the
UX refresh's web pull request, over a per-request nonce in the policy.

**Cost.** A choice in a form is the browser's own `<select>`: its open list is
drawn by the platform, not in the page's theme or face, and cannot carry an
icon, a hint or a group's heading as a Base UI Select could. A long list
scrolls with the platform's scrollbar rather than a drawn one. And a later
Base UI release that writes a `<style>` would be refused by the policy without
a sound, so the server-backed specs, under the policy `workflow --web` itself
sends, fail on any refusal while they open each part:
`web/e2e/server/page.spec.ts` opens the palette, an Autocomplete in a Dialog,
and the shortcut sheet, a Dialog; `web/e2e/server/policy.spec.ts` walks every
one of Settings' Tabs and opens the tag picker's Combobox with its list of
members shown, answering the reads that fill both itself, since the fixture
has no configuration, Slack or forge, and opens the directory picker's
Autocomplete, in its Dialog, three times over the fixture's own directories:
once listing the directories above the repository, once on the repository,
whose listing holds none, and once naming a directory that is not there,
whose list is left empty. Both run on Chromium, Firefox and WebKit. WebKit's
own shadow tree for each native `<select>` writes a style attribute the policy
allows by its hash alone (TRADE-55). A refusal from a state those specs never
reach, such as the tag picker's list with no member matching, or from a part
the page takes up later, is still heard by no spec.

**Reopen when.** A choice needs more than a native `<select>` can draw, or the
server gains a per-request nonce that the page could hand to `CSPProvider`.

### TRADE-51 The web checks a few values the server checks again

**Decided.** The `--web` Settings refuses a few settings beside themselves
before a save is sent (`web/src/features/settings/settingsRules.ts`): a Jira
address that is not http or https, carries a login, or would send the token
over http off this machine; a branch template without `{key}`; a timing that
is not a positive Go duration; an issue trailer of more than one word or with
a colon; and a count below zero. Each says again what `internal/config`
refuses at load. A status change does the same for a field the change needs
and was left empty (`emptyOf`,
`web/src/features/issues/writes/StatusChangeForm.tsx`), refused beside the
field before the change is sent, as the server refuses it
(`internal/jira/fields.go`): a text or single-option field with
`jira.ErrNeedsValue`, a list with nothing chosen with `jira.ErrNeedsChoice`.
The server stays the authority: a save it refuses as not valid names each
setting in the problem's `fields`, which Settings marks the same way, and a
status change it refuses keeps its refusal beside the form. Decided
2026-10-14 by the maintainer, in the UX refresh's Settings pull request, and
widened the same day to the status change's check, which the refresh's
inline refusals added, with no shared case file for either: the rules are
few, and the server checks each again.

**Cost.** Two copies of these checks, in Go and in TypeScript, which can
drift: a rule loosened on the server alone is still refused on the page until
its twin follows, and one tightened there is caught only by the server's
refusal after the save or the change is sent. Settings words a refusal its
own way, not the server's. The status change copies the server's words,
`needs a value` for a text or single-option field and `needs at least one`
for a list (`emptyRefusal`), so a change to `jira.ErrNeedsValue` or
`jira.ErrNeedsChoice` leaves the page's copy behind.

**Reopen when.** A client rule refuses a value the server accepts, or the
server can be asked to check a configuration without saving it, which would
let Settings ask rather than copy.

### TRADE-52 The directory matcher is written twice

The directory picker narrows a listing to the names holding what was typed,
letter by letter in order and in any case, those starting with it first, in
the terminal (`workdirs.Matching`, `internal/workdirs/match.go`) and in the
web (`matching`, `web/src/features/repositories/dirFilter.ts`), each as the
name is typed. Both copies read their cases from one file,
`testdata/twins/directory_match.json`, and each decodes it strictly, so a case
one side would not read fails rather than passing unread.

**Decided.** 2026-10-14 by the maintainer, in the tech-debt pull request that
found the two copies disagreeing on İ: the web's dialog narrows on every
keystroke, and asking the server to rank the names would be a request per
letter for a listing the page already holds, while the terminal matches
without the API at all.

**Cost.** A change to how a name matches is made twice, against one shared
case file that fails whichever copy disagrees with it; a name the file holds no
case for can still match differently. The two lower letters through different
tables, Go's and the browser's, so a letter one of them knows and the other
does not yet lowers on one surface alone.

**Reopen when.** The server comes to rank a listing's names for another
reason, or the two copies are found to disagree on a name the file does not
hold.

### TRADE-53 Two linters are pinned outside mise, each whole tree by hash

yamllint is pinned by `build/yamllint/requirements.txt`, which names it and
every package it needs at run time, each by version and by the sha256 of every
wheel PyPI holds for it, under `--require-hashes` and `--only-binary :all:`.
`scripts/pin-yamllint.sh` writes it (`task deps:yamllint`), taking each
dependency at its newest final release whose every file is a week old.
markdownlint-cli2 is pinned by `build/markdownlint/package-lock.json`, which
npm resolves from `build/markdownlint/package.json` under the week its
`.npmrc` asks for (`task deps:markdownlint`), every package by its sha512.
`task tools` installs yamllint into a virtual environment and markdownlint-cli2
with `npm ci`, both under `tmp/tools`, and links each into `tmp/tools/bin`,
which `mise.toml`'s `[env]` puts on PATH; the build container installs both
from the same files; and `scripts/check-dependency-age.sh` dates each pin a
pull request adds, a PyPI file by its own upload. mise is the exception's
reason, named in `mise.toml`'s header and in CLAUDE.md's rule that tool
versions live there.

**Decided.** 2026-10-17 by the maintainer, in the pull request that pins the
two trees, once a fresh install was seen taking argparse 3.0.3 for
markdownlint-cli2 two days after its release. A `mise.toml` pin named each
linter alone, so every install, a developer's, CI's, the image's and the dev
container's, resolved pyyaml, pathspec and markdownlint-cli2's 86 packages
afresh, with no hash and outside the age gate. mise 2026.10.8 could not hold
the trees itself: an exact pin skips its own age filter, `uv tool install`
wrote corrupted constraint hashes, pip refuses the bare pin mise passes it
under `--require-hashes`, its npm installer ignored npm's age settings, and
its own lockfile needs uv, a new tool to approve. Committed lock files need
nothing the repository does not already run: pip, the npm the pinned node
brings, curl and jq.

**Cost.** `mise install` alone no longer installs every linter: `task setup`
or `task tools` must run too, and after a pull that changes a pin file the
installed copy is stale until `task tools` runs again. Each checkout and
linked worktree has its own `tmp/tools/bin`, so a new one has neither linter
until `task tools` runs in it. yamllint needs a `python3` from 3.10 to 3.14
with venv on the host and on CI's runner, a prerequisite no pin holds:
yamllint needs 3.10, pyyaml has no wheel past CPython 3.14, and pip builds
none. The dev container adds Debian's python3-venv, which its image lacks.
CI's Lint job and both release gates add the step and its PATH line
themselves. The requirements file is a pip manifest GitHub's dependency graph
reads, so dependency review sees yamllint, licensed GPL-3.0-or-later, against
its GPL deny list, and `.github/workflows/dependency-review.yml` allows
`pkg:pypi/yamllint` by name: a lint tool, never linked into or shipped with
the binary. For the same reason the release's SBOM leaves both pin files and
`tmp/` out (`build/syft.yaml`), and a tool pinned by a file of its own under
`build/` joins that list. `npm ci` takes what the lock names at any age, so
the age gate, which runs on a pull request, is the lock's only check of age.
Dependabot moves the lock but not the requirements file, whose bumps are `task
deps:yamllint` by hand. The generator reads no version range a requirement
names, and pip refusing the tree at install is the only check of one; a hand
edit that drops `--only-binary` is caught only in review. The lock holds
braces 3.0.3, which markdownlint-cli2 reaches through micromatch to expand the
repository's own globs, and GHSA-vfj7-8cjw-p6xm (stack exhaustion through
deeply nested patterns) covers every braces release, so
`.github/workflows/dependency-review.yml` allows that advisory by name, and
its Dependabot alert is dismissed as a tolerable risk; the same braces was
installed, unseen, before the lock. markdownlint reaches katex through
micromark-extension-math, whose every release asks for katex ^0.16.0, which
GHSA-238p-pmpm-9mq7 covers and 0.18.2 fixes, so
`build/markdownlint/package.json` overrides it with 0.18.11. markdownlint
imports katex along with the math syntax it parses but never renders math,
which the advisory needs, so the override changes no code markdownlint calls.

**Reopen when.** mise can install a Python or npm tool from a lock of its
whole tree, with hashes and within its age filter, using no tool the
repository does not already pin; or uv is adopted for another reason, which
would let mise's own lockfile hold the trees; or either linter is replaced by
one released as a single binary. The braces allowance goes as soon as a braces
release fixes GHSA-vfj7-8cjw-p6xm and the lock can take it, or
markdownlint-cli2 stops reaching braces. The katex override goes once
micromark-extension-math takes a fixed katex, or markdownlint stops reaching it.

### TRADE-54 A cloud session's gate runs zizmor offline and leaves vuln to CI

`scripts/gate-as-dev.sh` runs the gate in a Claude Code cloud session as the
non-root user, so the session can catch before a push what CI would. Its
`hand_over` runs zizmor with `ZIZMOR_OFFLINE=true`, and, while `vuln.go.dev`
cannot be reached, its `check_steps` leaves `vuln` out of `task check`'s
steps. The SessionStart hook, `.claude/hooks/session-start.sh`, sets
`ZIZMOR_OFFLINE=true` for the session's own commands too, so the pre-commit
hook and `task lint`, run as root with the session's GitHub token in their
environment, do the same.

**Decided.** 2026-10-16, with the local gate the maintainer asked cloud
sessions to run before a push. zizmor runs its online audits whenever a
GitHub token is in its environment, and they list each third-party action's
tags from github.com, which refuses a cloud session's token with a 401. The
gate user is handed no token, so its zizmor would run offline anyway;
`ZIZMOR_OFFLINE=true` keeps it so whatever its environment comes to hold.
And the session's network refuses `vuln.go.dev`, where govulncheck reads its
database. Either would fail a local run for a reason no change can fix.

**Cost.** A dependency with a newly published advisory passes the local gate
and fails first in CI's Scan job, which runs `vuln`. zizmor costs nothing
against CI: CI's Lint job hands zizmor no token, so it runs offline there too.
Its online audits (known-vulnerable-actions, impostor-commit, ref-confusion,
stale-action-refs and ref-version-mismatch) run only where a developer's own
environment holds `GH_TOKEN`, `GITHUB_TOKEN` or `ZIZMOR_GITHUB_TOKEN`.

**Reopen when.** The cloud environment's network policy allows `vuln.go.dev`
(the gate then runs `vuln` again on its own), or CI's zizmor is handed a
token, so the local run no longer matches CI's.

### TRADE-55 The content policy allows WebKit's style for a select's slot

**Decided.** The content policy (`contentPolicy`,
`internal/webserver/edge/guard.go`) allows one inline style attribute, and
only by its hash: `style-src-attr 'unsafe-hashes'` with the SHA-256 of
`text-overflow:inherit`. WebKit, building the shadow tree of its own that
every native `<select>` draws from (`didAddUserAgentShadowRoot`, in WebKit's
`HTMLSelectElement`), sets that style attribute on the tree's button slot
before attaching the slot to the tree, so WebKit does not yet see the
attribute as its own tree's (`isInUserAgentShadowTree` is false) and judges
it by the page's policy. Under `default-src 'self'` alone that refused it.
In Playwright's WebKit 27.2, the build CI runs, the two tests of the
`server-webkit` project that listen for a refusal heard two each and failed;
its other two listen for none. WebKit's main branch sets the attribute the
same way, once for each select, so a Safari built from it would log a
console error per select, though no shipping Safari was tried. The refusal
likely changes nothing on screen: the button slot holds only a `<button>`
written as a select's first child, which no select on the page has, and
WebKit's own comment there says a select drawn natively does not use the
slots. The native `<select>` stays, since TRADE-50 keeps it, so the policy
takes the one string WebKit writes rather than refuse it. Decided 2026-10-15
by the maintainer, in the pull request that runs the server-backed page spec
on Firefox and WebKit, whose WebKit run heard the refusals.

**Cost.** The policy carries a hash a reader has to take on trust, with the
string it stands for written only in `guard.go`'s comment, in this entry and
in `TestThePolicyAllowsNoInlineStyleButTheOneWebKitGivesASelect`
(`internal/webserver/edge/guard_test.go`), which holds `style-src-attr` to
exactly that hash and the rest of the policy to what it was. Under
`'unsafe-hashes'` a style attribute with that value applies wherever it is
written, by the page as well as by WebKit, though `text-overflow:inherit` is
all it can ever say. The allowance follows WebKit's string exactly: a release
that writes it otherwise, with a space or another property, is refused again,
and the page spec fails on `server-webkit` until the hash follows. An engine
that applies no hash to a style attribute refuses it as before.

**Reopen when.** A WebKit release no longer writes that attribute on a slot
not yet attached: it writes the attribute only after attaching the slot; it
sets the property through `setInlineStyleProperty` instead, as its list-box
slot already does, which writes no attribute for the policy to judge; or it
sets no such style at all. Or the page stops drawing a native `<select>`,
which reopening TRADE-50 would mean. Then the hash and `'unsafe-hashes'`
leave the policy.

### TRADE-56 The docs' images are regenerated by hand and compared with nothing

**Decided.** 2026-10-19 by the maintainer, in the pull request that adds them:
the docs show six screenshots of the `--web` page, a filmed walkthrough of it
and a recording of the terminal, committed among the site's assets, and
`task docs:media` (`scripts/docs-media.sh`) writes them all again: Playwright
screenshots the `--web` page the demo serves (`cmd/demo -web`,
`web/e2e/docs/media.spec.ts`) and films the loop on it
(`web/e2e/docs/walkthrough.spec.ts`), which ffmpeg encodes into an animated
WebP, and VHS films `docs/media/terminal.tape` over the demo's terminal
(`cmd/demo`). Nothing runs it but a person, and nothing compares what it writes
with what is committed. An image's bytes depend on the machine's fonts, its
Chromium and its ffmpeg, so a comparison would fail everywhere but on the
machine that wrote the image; the page's state marks fall outside its own face
(TRADE-48) and come from whatever font the system has. The script checks only
that the run wrote each image and kept it within its budget, 300 KiB a PNG, 2
MiB the GIF and 4 MiB the WebP, whose lossless film of the whole loop is about
3.2 MiB, that the GIF runs as long as its tape and the WebP as its film, with no
frame lost where the film moves, which a busy machine's recordings do not, and
none so brief, 10 ms or less, that a browser holds it for 100 ms, that each
frame the WebP holds is the film's own as Chromium decodes it, which a lossy
encoding's leftover blocks are not, that the machine is not busy when it starts,
and that what the images show was committed when it ran, and the reviewer of the
commit that changes them looks at them. Chromium and ffmpeg are the machine's,
unpinned; VHS and ttyd are mise's pins or, where mise cannot install them, built
by `go install` and downloaded. mise checks VHS against its release's checksums,
but its pin of ttyd, `aqua:tsl0922/ttyd`, has no checksum to check against, so
`mise install`, on a developer's Linux machine or in
`.devcontainer/postCreate.sh`, downloads ttyd unchecked; the release workflows
leave both out with `MISE_DISABLE_TOOLS`. So the script checks every ttyd it
runs, mise's included, against the SHA-256 in `docs/media/tool-checksums.txt`
before running it, and downloads its own copy when PATH has none.

**Cost.** The images go stale without a word: a pane, a section or the
demo's story changes, and they show the old screen until someone runs the
task. Each run writes every image again, on another machine every pixel, so
it adds the whole set, about 5 MiB, to the repository's history for good,
and a reviewer tells a changed screen from a changed font only by looking.
A tampered ttyd 1.7.7 release asset would reach every Linux checkout's
`mise install` unnoticed, stopped only when `task docs:media` hashes it.

**Reopen when.** A recording environment is pinned whole, its fonts,
Chromium and ffmpeg with it, as the build container could be, so two runs
write the same bytes and CI could check them for drift; or the history the
images add makes a clone slow enough to notice; or a change to the interface
ships while the docs' images still show the screen before it; or aqua's
registry declares a checksum for tsl0922/ttyd, so mise checks it as it checks
VHS (`mise tool ttyd --json` shows a non-empty `security`).

### TRADE-57 The demo world and the terminal's test world are two fakes of one world

**Decided.** 2026-10-19 by the maintainer, in the pull request that records
the terminal for the docs: the recording runs the real interface over a world
of its own, `internal/demo`, a made-up day's work whose seams keep state, so
staging moves a change, a commit adds one, a push sets the upstream, opening a
pull request keeps it and its CI passes on the demo's clock. The terminal's
tests keep their own world, `internal/tui/world_test.go` and its siblings,
which answers what each test sets and records every call for the test to read
back. Moving that world out for the demo to share would have meant exporting
its fields and rewriting most of the package's test files, and would tie what
a test expects to what the docs show.

**Cost.** One world is faked twice. A seam that changes shape is changed in
both, and the compiler names each place. Every image the docs show comes from
the demo, the terminal's recording and the web's screenshots alike, but the
demo's story is told a third time by the web's mock (`web/src/dev`), which the
web's own tests and `task web:mockup` keep, and nothing holds the mock to the
demo. The demo's first moment is written twice, in Go (`epoch` in
`internal/demo/world.go`) and as the clock the web's screenshots and its
walkthrough both hold (`now` in `web/e2e/docs/capture.ts`), and only an
image's ages and timers show the two apart. Only the demo's own tests, which
drive both interfaces over it, keep it from rotting when a pane or a section
changes what it reads.

**Reopen when.** A seam change has to be made in both worlds for a reason
other than its shape, the two drift far enough that a reader of one is misled
by the other, or the maintainer schedules the follow-up that has the test
world take its fixture data from `internal/demo`.
