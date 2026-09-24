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
every entry in it was done; this one starts fresh, and its numbering
continues where that one stopped, so an ID is never reused.

Checked against commit `f7b671f` on 2026-09-24 (main after the surface
review, PRs #128–#133); an entry a later change touched was checked again
in that change. Line numbers drift, so every pointer also names the symbol
it means.

## How this was produced

1. **Three reads, from code only.** The command line (`internal/cli`,
   `cmd/`), the terminal interface (`internal/tui`) and the web (`web/src`,
   `internal/webserver`, `api/openapi.yaml`) were each read in full against
   the standard the project sets for itself in [CLAUDE.md](CLAUDE.md), with
   the repository's gates, budgets, coverage report and dependency files
   read alongside. Nothing was run interactively; the screens are known
   from the source and from the golden output the tests hold.
2. **A second read of every medium and high entry.** The cited lines were
   read again while writing this file; a claim that could not be pointed at
   a line was dropped or marked *read* rather than *measured*.
3. **Measurement.** The condition-coverage figures come from the gobco
   report (`task cover:branch`), written at this commit or, where an entry
   says it was re-measured, at that point; the file lengths from
   `git ls-files` and `wc`; the budget standings from
   `scripts/check-package-size.sh --list`.

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
rule. This pass found none: the loopback bind, the `Host` and `Origin` guards,
the dry-run gate, the fixed configuration path, the `0600` request log and
the redaction paths all hold. The entries below contain nothing that helps
anyone misuse a credential, a terminal or a release.

## The terminal interface

No open entry of its own. What it carries on purpose — the two composers'
field handling written twice, and the spine's color-only hue — is under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost).

## The web

No open entry of its own. What the web's gates still lack — an end-to-end
run that drives a write — is DEBT-65, with the other gates below.

## The gates, the build and the tests

### DEBT-64 The condition-coverage worklist: 237 one-sided conditions, and 9 never evaluated

Severity: low · Confidence: measured

Re-measured after the store, forge-wiring and root-command tests (`task
cover:branch` on macOS, floor 91 %): 3,821 of 4,076 arms, 93.7 %. Of 2,038
conditions, 237 were observed only one way — 72 of them an `err != nil`
never seen true. By package: `internal/tui` 88, `internal/cli` 39,
`internal/forge` 31, `internal/webserver` 16, `internal/wiring` 12,
`internal/jira` 12, `internal/config` 11, `internal/messaging` 8,
`internal/store` 5, `internal/tui/frame` 5, and ten across `buildinfo`,
`editor` and `hooks` (two each) and `convention`, `gitrepo`, `httpx` and
`proc` (one each).

The store is under ten. Its five are `sql.Open` in `Store.open`
(`internal/store/store.go:148`), which fails only for an unregistered
driver, and four that need SQLite to fail partway through a statement:
`BeginTx` and `Commit` in `Store.CacheIssues` (`internal/store/cache.go:110`,
`:121`), and `rows.Err` in `readCachedIssues` (`internal/store/cache.go:88`)
and `Store.Announces` (`internal/store/announce.go:80`).

Nine conditions were never evaluated. Five are a test away:

- `internal/cli/doctor_json.go:237` — `credentialStatus`'s `errUnreachable`
  case: no `doctor --json --online` test has a credential check fail.
- `internal/tui/messaging.go:90` and `:92` — `quitGuard.handleKey`'s confirm
  and stay: `TestQuittingWithAQueuedPostAsksFirst` opens the guard but
  presses neither enter (quit) nor esc (stay) in it.
- `internal/tui/prcreate.go:134` — `pullCreated.apply`'s `named` case: every
  test that opens a pull request on an issue's branch wires
  `Jira.LinkPullRequest`.
- `internal/wiring/wiring.go:232` — `fetchOrigin`, git failing to start: no
  wiring test fetches.

Four no black-box test reaches without changing the code:

- `internal/tui/tui.go:142` and `:149` — inside `tui.Run`, which needs a real
  terminal.
- `internal/wiring/wiring.go:144` — `browserCommand`'s `"windows"` case,
  evaluated only where `runtime.GOOS` is not `"darwin"`: CI's Linux run reaches
  it, a Mac never does.
- `internal/wiring/forgecli.go:59` — `forgeProgram`'s `forge.KindUnknown`
  case, which `exhaustive` requires but `connectForge` never passes, since
  `Repo.APIBase` refuses an unknown forge first.

**What it costs.** Branch coverage reads 93.7 %, 2.7 points above the 91 %
floor, which is the ratchet's own slack, so an untested error path in the
next feature no longer fails the gate on someone else's pull request. The
cost now is the ratchet: floor(93.7) − 2 is today's 91, so
`BRANCH_COVERAGE_MIN` cannot rise until the report reads 94.0 % — 3,830
arms, 9 more than today, since the gate rounds to one decimal first.

**One way to fix it.** The five reachable sites above, one test each; then
the report is the worklist, most of it in `internal/tui`, `internal/cli` and
`internal/forge`.

**Done when.** `task cover:branch` names no never-evaluated condition but
the four above, and reads 94.0 % or more, so `BRANCH_COVERAGE_MIN` ratchets
to 92.

### DEBT-65 The web's e2e drives no write

Severity: medium · Confidence: read

`task check` (`Taskfile.yml:502`) runs the web's lint, client-drift check and
unit tests beside the Go gates, but not its end-to-end suite. That suite is
six specs (`web/e2e/a11y.spec.ts`, `web/e2e/layout.spec.ts`,
`web/e2e/panes.spec.ts`, `web/e2e/screens.spec.ts`,
`web/e2e/smoke.spec.ts`, `web/e2e/theme.spec.ts`), outside `task check`
(CI's `e2e` job and `yarn test:e2e` run it), with no `workflow --web`
backend — acknowledged at `.github/workflows/ci.yml:100` ("No backend": the
specs answer the API themselves, or read a VITE_MOCK build's fixtures) — so
no test drives any of the twelve write actions end to end.

**One way to fix it.** The e2e job starts `workflow --web` against a fixture
repository so one spec can commit, push and open a pull request.

**Done when.** One Playwright spec performs a write against a running server.

### DEBT-71 `wiring` returns `tui.Deps`, though three surfaces consume the seams

Severity: low · Confidence: read

`wiring.Deps` (`internal/wiring/wiring.go:72`) returns `tui.Deps`, so the
wiring package imports the terminal interface; the CLI's `WebDeps`
(`internal/cli/cli.go:284`) then narrows that bundle for the web server.
The seams are not the terminal's — they are the loop's. That import no
longer stands in the way of shared composition: `internal/loop` takes each
seam as a plain argument (`loop.PullSeams`, `loop.AnnounceSeams`) and never
needed `wiring`. What is left is narrower: a seam only the CLI or the web
needs must still be declared on `tui.Deps`, as a `RecentCommits` for
`standup` would be — which is why `standup` reads its commits from the
repository directly instead (`internal/cli/standup.go:98`).

**One way to fix it.** Move the bundles that depend only on leaf types —
`JiraDeps`, `GitDeps`, `ForgeDeps`, `MessagingDeps`, `HookDeps` — to a
package all three surfaces import, with type aliases left in `tui`.
`StoreDeps` carries `tui.AnnouncedPost`, so it could move only with that
type, and `EditorDeps` carries Bubble Tea's `tea.Msg` and `tea.Cmd`, so
`wiring` would still return `tui.Deps`.

**Done when.** The seam bundles are declared where all three surfaces can
import them without importing each other. Deferred — YAGNI until a seam the
terminal does not use has to be added to `tui.Deps`.

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
  does not list answers to the default, which `internal/forge` fills
  exactly, so its next file is a first entry with its WHY. The cost is
  that any change adding a file there must carry its budget row in the
  same commit or fail `task check`.
- **Seven test files stay past the 500-line soft target**, all under the
  800 ceiling (`scripts/check-file-length.sh --list`). They were left whole
  on purpose when the source files past the target were split by concern;
  each holds the cases of one behavior. Announcing:
  `internal/messaging/post_test.go` (763, the post to each service and the
  announcement's text) and `internal/tui/messaging_test.go` (612, the
  terminal's Messaging pane). Opening a pull request:
  `internal/webserver/pullrequest_test.go` (601, the web's draft and open).
  Staging, committing and pushing: `internal/tui/composer_test.go` (568,
  the terminal's commit composer), `internal/webserver/staging_test.go`
  (533, the web's stage and unstage) and
  `web/src/features/branch/BranchPanel.test.tsx` (510, the web's commit and
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
- **`staleTime: Infinity`** (`web/src/queryClient.ts:12`) with the event
  stream as the sole freshness source. Correct for a pushed snapshot; the
  cost is that a stalled stream leaves stale data with no refetch to fall
  back on. Three queries set their own `staleTime`. The issue detail's and
  the review queue's are a minute (`web/src/features/issues/issueApi.ts:23`,
  `web/src/features/reviewqueue/reviewQueueApi.ts:23`): the stream carries
  only the list's slim issues and never the queue, so each is read again
  when reopened after a minute, and the queue's Refresh reads it at once.
  The configuration's is 0 (`web/src/features/settings/configApi.ts:65`):
  the file can change on disk, which no event reports, so Settings and the
  commit form read it again each time they open. A save that still meets a
  change it has not seen is refused (409) and nothing is written; Settings
  offers **Reload**.
- **The progress spine's per-system hue is color-only**
  (`internal/tui/spine.go:68`), mitigated by the stage name, or its initial
  when compact (`internal/tui/spine.go:51`). Part of the visual system UX.md
  says should not change; the cost is one channel the monochrome reader does
  not get.
- **The two composers' field handling is written twice.** The commit and
  pull request composers each pair an `onFieldNav` with a `*CanComplete`
  check (`commitComposer.onFieldNav`, `internal/tui/scopesuggest.go:17`;
  `prComposer.onFieldNav`, `internal/tui/prcomposer.go:304`), and each blurs
  every field before focusing one (`commitComposer.focusOn`,
  `internal/tui/composer.go:297`; `prComposer.focusOn`,
  `internal/tui/prcomposer.go:325`). Two is not yet the rule of three, so
  they stay apart until a third composer needs them. The cost is that a
  change to field navigation is made twice, and a third composer must copy
  the pairs or extract them then.
- **A condition-coverage skip list of two** (`UNANALYZABLE`,
  `scripts/gobco-report.sh:82`): gobco ignores build tags, so it cannot read
  a package whose files come in tagged twins, and `internal/proc/pgroup` and
  `internal/web` are named there with that reason beside them. Every other
  package is read, and one that becomes unreadable without being named fails
  the gate rather than shrinking the number. The cost is that the two named
  packages' conditions go unmeasured — platform glue and an embed stub, with
  no branch worth the count — and that the next tagged twin must join them.
- **The web's branch floor is v8's range-based count**
  (`web/vitest.config.ts:43` `thresholds`), not a gobco-style per-condition
  one: v8 marks a branch covered once its range of code has run, and never
  asks which way each operand of a condition went. The cost is that an
  `a && b` only ever seen with `b` true still passes the web's floor, where
  the Go side's `task cover:branch` would name it.
