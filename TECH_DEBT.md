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

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). This edition is read and measured.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it appears under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost), which lists
the choices that were made on purpose and written down, and what they cost.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule. This pass found a few; they were written to a private file for the
maintainer, not here, and nothing in this file says what they are. The
entries below contain nothing that helps anyone misuse a credential, a
terminal, a file on disk, the loopback server or a release.

## The command line

What is open here is the drift between the docs and the code they describe,
in the commands and in the clients and plumbing every command shares (the
interface and the web reach them too).

### DEBT-90 The docs site trails the code across usage, configuration, web and install

Severity: low · Confidence: read

The site's pages, the README and the docs index promise things the code does
not do or stay silent on things it does. `scripts/check-docs-drift.sh`
compares only the generated command reference, so no gate sees any of these
pages. UX-87 counts the same Settings undercount from the user's side.

The usage page:

- `docs/content/docs/usage.md:173` — "Pick up an issue" names three field
  cases (a fixed set, text, any other kind sent to Jira) where
  `fieldForm.textual` (`internal/tui/fields.go:80`) fills `FieldUser` and
  `FieldDate` as typed inputs and `fieldForm.multi` (`:90`) takes any number
  of a `FieldOptionList`'s options.
- `docs/content/docs/usage.md:185` — "Branch" says the branch "starts from
  origin's default branch, which the overlay names" with no word of the
  fetch (`branchCreator.create`, `internal/tui/branch.go:439`), the "fetched
  AGE" line (`branchCreator.start`, `:361`) or the offer after a failed
  fetch (`fetched.apply`, `internal/tui/branchresult.go:49`).
- `docs/content/docs/usage.md:192` — "Stage and commit" describes the list
  and the composer; the page never mentions a diff (`grep -ic diff` is 0)
  while `diffSection` (`internal/tui/diff.go:67`) draws the selected file's
  diff beneath the list.

A reader expects a webhook typo caught at init and it is saved unchecked,
then trusts a `doctor` check that never ran; sets `$GIT_EDITOR` and is told
another editor opens; in a subdirectory gets the repository's file after
reading that the current directory wins; reads "never overwrites" as a
guarantee the code does not make; looks for `workflow help` in the reference
and finds nothing; is warned off unreleased code they will not get and shown
a pin five releases old; follows the web page to the second Settings part
and finds a third; and meets a date input, a "could not fetch" offer and a
diff the usage page never mentions.

**One way to fix it.** One editing pass over the cited pages with the code's
own comment as the source of each sentence: say the webhook is saved
unchecked on the configuration page and in `config init`'s Short and Long,
then `task docs:gen`, and drop the `doctor` promise from the README and the
docs index; state the editor order as git's (`$GIT_EDITOR`, `$VISUAL`,
`$EDITOR`, else `vi`, `notepad` on Windows) on the usage page and in
`defaultEditor`'s comment; describe the walk to `.git` on the page and in
`Discover`'s comment; name the three store identifiers and the root-path
fallback on both pages; qualify "never overwrites" with the revision check's
window; state the third notify condition; name `help` as the one command
without a page; drop the "no releases yet" and "until the first tag"
sentences and refresh the pinned example; reorder the web page's Settings
list to the form's and name every carried key; add the field kinds, the
fetch and the diff to the usage page; and reword the `queryClient` comment
to `useSnapshotStore`.

**Done when.** `grep -c 'does the same for Slack'
docs/content/docs/configuration.md`, `grep -c 'no releases yet' README.md`,
`grep -ci 'until the first tag' README.md docs/content/docs/install.md` and
`grep -c setQueryData web/src/queryClient.ts` all print 0; `grep -n
GIT_EDITOR docs/content/docs/usage.md internal/editor/editor.go`, `grep -n
help docs/content/docs/reference/_index.md` and `grep -n '\.git'
docs/content/docs/configuration.md` each match a sentence that says what the
code does; the README and `docs/content/_index.md` no longer say `doctor`
checks a webhook; the web page's Settings list reads in `ConfigForm`'s
fieldset order; the usage page names user, date and multi-select fields, the
fetch and the diff; and `task docs:check` is green after `task docs:gen`.

### DEBT-105 The contributor documents restate counts and names the tree has moved past

Severity: low · Confidence: read

The documents a contributor and a later session read first restate numbers
and names the tree has moved past. No gate reads any of them.

- `CLAUDE.md:102`, `ARCHITECTURE.md:14` and `FEATURES.md:54` — each pairs
  `CGO_ENABLED` with the same count: "the release binaries cross-compile to
  five platforms", "so it cross-compiles to five platforms", "because the
  release cross-compiles to five platforms". `RELEASE_PLATFORMS`
  (`Taskfile.yml:73`) names three GOOS/GOARCH pairs, mirrored by the binary
  table in `.github/workflows/release.yml:108`, and `CONTRIBUTING.md:208`
  already says so: "macOS (arm64), Linux (amd64) and Windows (amd64)".
- `CLAUDE.md:75` — the `task lint` row's parenthetical lists twelve checks;
  the `lint` task (`Taskfile.yml:290`) runs sixteen sub-tasks, and the row
  omits `lint:packagesize` (`Taskfile.yml:301`), `lint:goversion`,
  `lint:goroutines` and `gen:verify`. `CLAUDE.md:157` says the package-size
  gate runs in `task lint`, contradicting the row in the same file.
- `docs/content/docs/contributing.md:50` — the `task lint` row names nine
  checks and omits `lint:markdown`, `lint:toml`, `lint:filelength`,
  `lint:packagesize`, `lint:goversion`, `lint:goroutines` and `gen:verify`.
- `CLAUDE.md:504` — the never-print-a-secret rule names `slack.token`, a key
  the decoder refuses: `ErrSlackRenamed` (`internal/config/config.go:50`)
  says the "slack" block was renamed to "messaging", and the field is
  `Messaging.Token` (`internal/config/config.go:110`, `json:"token"`).
- `CLAUDE.md:9` — the opening line names Slack alone where the same file's
  layout row (`CLAUDE.md:44`) names "Slack, Teams, Discord or a plain
  webhook".
- `SECURITY.md:61` — the in-scope list is "`cmd/`, `internal/`, `scripts/`,
  `build/`, and every file under `.github/workflows/`": no `web/`, no
  `api/`, no loopback server and no browser, though `web/README.md:4`
  describes the app "served locally by `workflow --web`".
- `web/README.md:13` — "Running the cockpit takes two shells", with `task
  dev` and `task web:mockup` mentioned nowhere; the comment above `dev`
  (`Taskfile.yml:104`) names `web` and `web:ui` as the two shells, directly
  above the one-shell `dev` task (`Taskfile.yml:111`, "Run the whole cockpit
  in one shell"), and `web:ui`'s `desc` (`Taskfile.yml:145`) still says
  "Shell 2". `web:mockup` (`Taskfile.yml:130`, "Serve the web UI against
  mock data in one shell") is undocumented in the README.

A session reading CLAUDE.md learns a platform count the gate does not hold,
a lint list that omits four gates it will trip on, and a secret key it
cannot find in the code; a contributor tripped by `lint:goversion` or
`lint:packagesize` finds no mention on the published page; a reporter can
read SECURITY.md literally and not report a hole in the React client or the
contract; a frontend contributor opens two terminals and never learns about
`task dev` or the mock mode.

**One way to fix it.** Replace each count with a pointer at its source
(`RELEASE_PLATFORMS`, `task --list`) or the full list, at every site in one
commit; name `jira.token`, `messaging.token`, `messaging.webhook_url` and
`forge.token` — or "any `Secret` field" — in the secret rule and the four
services in the opening line; add `web/` and `api/` to SECURITY.md's scope
with the loopback server and the browser named; lead `web/README.md` with
`task dev`, add one line for `task web:mockup`, and move the two-shell
comment to the `web` and `web:ui` pair it describes.

**Done when.** `grep -n 'five platforms' CLAUDE.md ARCHITECTURE.md` prints
nothing, the `CGO_ENABLED=0` bullet in `FEATURES.md` names no platform
count (the phrase wraps there, so grep it with `-z` or read it), and `grep
-n 'slack.token' CLAUDE.md` prints nothing; every
sub-task under `Taskfile.yml`'s `lint` is named in, or referenced by,
CLAUDE.md's and `docs/content/docs/contributing.md`'s `task lint` rows;
CLAUDE.md's opening line names the services its `internal/messaging` row
names; `grep -n 'web/' SECURITY.md` matches inside the in-scope list; `grep
-n 'task dev' web/README.md` and `grep -n 'web:mockup' web/README.md` match;
and `grep -n 'two shells\|Shell 2' Taskfile.yml` prints nothing above the
`dev` task.

## The terminal interface

Nothing is open here: the interface announces and drafts a pull request
through `loop`, as the command line and the web do, and remembers what it
announced through `loop.Deliver`, as the command line does (the web does not
yet: FEAT-84). What it carries on purpose — the two composers' field
handling written twice, and the spine's color-only hue — is under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost).

## The web

Nothing is open here. What the web carries on purpose — read-only under
`--dry-run`, and queries the stream keeps fresh — is under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost).

## The gates, the build and the tests

What is open here is the coverage worklist and the metric behind it. The
clicked steps the web's axe scans and Tab walks once missed — the pull
request form, the push confirmation, the announcement preview and a refused
write — are scanned in both themes and walked at every width, and a
server-backed run stages, commits and pushes through a running
`workflow --web`.

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

## Deliberate trade-offs that carry a cost

These were chosen on purpose and are written down at their sites. They are
not debt; they are listed because each one costs something a reader should
know about.

- **Every package with a declared file budget sits exactly at it** — the
  numbers are in `scripts/package-size-budgets.txt`, and
  `scripts/check-package-size.sh --list` shows the standings. That is the
  gate working as designed, since a budget left above its count fails too:
  the next file in any of them is a decision (a split, or a bump with the
  WHY rewritten and a history row), not an accident. A directory the file
  does not list answers to the default, which `internal/forge` and
  `web/src/shell` fill exactly, so the next file in either is a first
  entry with its WHY. The cost is that any change adding a file there
  must carry its budget row in the same commit or fail `task check`.
- **Seven test files stay past the 500-line soft target**, all under the
  800 ceiling (`scripts/check-file-length.sh --list`). They were left whole
  on purpose when the source files past the target were split by concern;
  each holds the cases of one behavior. Announcing:
  `internal/messaging/post_test.go` (766, the post to each service and the
  announcement's text) and `internal/tui/messaging_test.go` (614, the
  terminal's Messaging pane). Opening a pull request:
  `internal/webserver/pullrequest_test.go` (585, the web's draft and open).
  Staging, committing and pushing: `internal/tui/composer_test.go` (568,
  the terminal's commit composer), `internal/webserver/staging_test.go`
  (533, the web's stage and unstage) and
  `web/src/features/branch/BranchPanel.test.tsx` (513, the web's commit and
  push). Reading and writing an issue: `internal/jira/detail_test.go` (501,
  Jira's issue read, comment and pull request link). The cost is that
  `scripts/check-file-length.sh` still warns on every run, so a source file
  newly past the target is one more line among seven a reader has learned
  to skim.
- **The web is read-only under `--dry-run`** (`internal/webserver/guard.go:40`
  documents it): every unsafe method answers 403 at one gate, where the
  terminal simulates each write and narrates it. The browser reads
  `dry_run` from `getHealth`, says so in a banner, and holds every write
  before sending it (`web/src/api/client.ts:25`), so the server's 403 is
  only the backstop. The cost is that a web write under dry run is refused
  outright rather than simulated: the browser cannot show what the write
  would have done, as the terminal's narration does. The blanket refusal
  itself is the intended design.
- **`staleTime: Infinity`** (`web/src/queryClient.ts:14`) with the event
  stream as the sole freshness source. Correct for a pushed snapshot; the
  cost is that a stalled stream leaves stale data with no refetch to fall
  back on. Three queries set their own `staleTime`. The issue detail's and
  the review queue's are a minute (`web/src/features/issues/issueApi.ts:23`,
  `web/src/features/reviewqueue/reviewQueueApi.ts:23`): the stream carries
  only the list's slim issues and never the queue, so each is read again
  when reopened after a minute, and the queue's Refresh reads it at once.
  The configuration's is 0 (`web/src/features/settings/configApi.ts:65`):
  the file can change on disk, which no event reports, so Settings reads it
  again each time it opens. The commit form's types come from the snapshot
  and, like commit validation, follow the configuration the server last
  read or saved, so an edit made on disk reaches them once Settings is
  opened or a save lands. A save that finds the file changed since Settings
  read it is refused (409) and nothing is written; Settings offers
  **Reload**. A change landing between that check and the write is not
  caught (`SaveOver`, `internal/config/save.go:114`).
- **The progress spine's per-system hue is color-only**
  (`internal/tui/spine.go:69`), mitigated by the stage name, or its initial
  when compact (`internal/tui/spine.go:51`). Part of the visual system UX.md
  says should not change; the cost is one channel the monochrome reader does
  not get.
- **The two composers' field handling is written twice.** The commit and
  pull request composers each pair an `onFieldNav` with a `*CanComplete`
  check (`commitComposer.onFieldNav`, `internal/tui/scopesuggest.go:17`;
  `prComposer.onFieldNav`, `internal/tui/prcomposer.go:367`), and each blurs
  every field before focusing one (`commitComposer.focusOn`,
  `internal/tui/composer.go:297`; `prComposer.focusOn`,
  `internal/tui/prcomposer.go:388`). Two is not yet the rule of three, so
  they stay apart until a third composer needs them. The cost is that a
  change to field navigation is made twice, and a third composer must copy
  the pairs or extract them then.
- **A condition-coverage skip list of two** (`UNANALYZABLE`,
  `scripts/gobco-report.sh:85`): gobco ignores build tags, so it cannot read
  a package whose files come in tagged twins, and `internal/proc/pgroup` and
  `internal/web` are named there with that reason beside them. Every other
  package is read, and one that becomes unreadable without being named fails
  the gate rather than shrinking the number. The cost is that the two named
  packages' conditions go unmeasured — platform glue and an embed stub, with
  no branch worth the count — and that the next tagged twin must join them.
- **The web's branch floor is v8's range-based count**
  (`web/vitest.config.ts:45` `thresholds`), not a gobco-style per-condition
  one: v8 marks a branch covered once its range of code has run, and never
  asks which way each operand of a condition went. The cost is that an
  `a && b` only ever seen with `b` true still passes the web's floor.
