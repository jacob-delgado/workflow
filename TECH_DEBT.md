# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-57". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts. An earlier edition of this file was retired once
every entry in it was done; this one starts fresh, and its numbering
continues where that one stopped, so an ID is never reused.

Checked against commit `5e69cb8` on 2026-09-22 (PR #125's tip, merged to main). Line numbers drift, so every pointer
also names the symbol it means.

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
   report written at this commit (`task cover:branch`); the file lengths
   from `git ls-files` and `wc`; the budget standings from
   `scripts/check-package-size.sh --list`.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). This edition is read and measured; only DEBT-72, added after
  it, was reproduced, in the build container.
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

### DEBT-55 Four source files and five test files sit past the 500-line soft target

Severity: low · Confidence: measured

`scripts/check-file-length.sh --list` flags four source files past the
500-line soft target — `internal/forge/github.go` (551),
`internal/gitrepo/branch.go` (532), `internal/tui/prcomposer.go` (523) and
`internal/tui/messaging.go` (525) — and five test files
(`internal/messaging/post_test.go` 738, `internal/tui/messaging_test.go`
612, `internal/webserver/pullrequest_test.go` 601,
`internal/tui/composer_test.go` 565, `internal/jira/detail_test.go` 501).
None is over the 800 hard ceiling. The first edition of this entry missed
the two source files outside `internal/tui`. Its headline file,
`internal/tui/review.go` at 727 lines, is paid: the merge picker moved to
`internal/tui/merge.go` and the re-run to `internal/tui/checks.go`,
leaving it at 451.

**What it costs.** `scripts/check-file-length.sh` warns on every run, so the
warning has stopped meaning anything.

**One way to fix it.** Split each by the concern it carries, as
`review.go` was — a new file in `internal/tui` carries its budget bump in
the same commit.

**Done when.** `check-file-length.sh --list` flags nothing `soft`.

### DEBT-57 The same overlay shapes, written fourteen, five and three times

Severity: low · Confidence: read

- Fourteen "keep the overlay open with the reason" appliers of the same
  `overlay.(T)` / `send.failed` / reassign shape:
  `internal/tui/branchresult.go:109`, `internal/tui/issuewrite.go:194`,
  `internal/tui/issuelink.go:98`, `internal/tui/preditor.go:162`,
  `internal/tui/prcomposer.go:487`, `internal/tui/hookgen.go:153`,
  `internal/tui/switchtask.go:228`, `internal/tui/comment.go:173`,
  `internal/tui/messaging.go:503`, `internal/tui/checks.go:227`,
  `internal/tui/merge.go:86`, `internal/tui/finish.go:141`,
  `internal/tui/picker.go:114`, `internal/tui/comment.go:71`.
- Five list-picker bodies with identical `up`/`down`/`confirm`/`esc` and a
  `window`-scrolled `rows`: `internal/tui/picker.go:225`, `internal/tui/picker.go:423`,
  `internal/tui/switchtask.go:119`, `internal/tui/checks.go:68`, `internal/tui/run.go:257`.
- Three focus-guarded "re-clamp the shared scroll after a shrinking reload"
  blocks: `internal/tui/commits.go:50`, `internal/tui/reviewqueue.go:52`, plus `internal/tui/commits.go:249`
  `followChange` / `internal/tui/reviewqueue.go:213`.
- Two `onFieldNav` + `*CanComplete` pairs (`scopesuggest.go:17`,
  `internal/tui/prcomposer.go:301`) and two blur-all-then-focus-one switches
  (`internal/tui/composer.go:297`, `internal/tui/prcomposer.go:322`).

The rule of three is met several times over. A helper that pins a failure
in whichever overlay asked would serve the first group, and a generic picker
would remove the second.

**Done when.** One picker type renders the five lists; the appliers share a
helper.

### DEBT-58 One scroll offset for six panes

Severity: medium · Confidence: read

`m.scroll` (`internal/tui/tui.go:51`) is a single offset shared by every
pane, reset on focus (`internal/tui/tui.go:269` `focusOn`). It is the reason for the
focus-guarded re-clamps in DEBT-57, and the reason `pickChange`
(`internal/tui/commits.go:255`) and `pickReview` (`internal/tui/reviewqueue.go:220`) must add
`m.scroll` to a clicked line while `pickIssue` (`internal/tui/detail.go:320`) must not —
three click paths that disagree about the same number.

**What it costs.** Switching panes loses the scroll position; every
scroll-aware change has to remember which pane the one offset currently
belongs to.

**One way to fix it.** A scroll per pane, held on the pane's state.

**Done when.** `focusOn` no longer touches a scroll; leaving and returning
to a pane restores its position (a screen test).

### DEBT-59 A generation counter stands in for cancellation, and a race is left on purpose

Severity: low · Confidence: read

`reviewState.generation` (`internal/tui/review.go:32`) exists solely to
stop a superseded CI polling chain from applying — a workaround for having
no way to cancel the earlier chain. `detailLoaded.apply`
(`internal/tui/detail.go:51`) documents a last-writer-wins race between two
in-flight reads of the same issue and consciously declines to fix it. Both
are honest about what they are; both are the kind of thing the next
concurrency change trips on.

**Done when.** Polling chains carry a context that the next generation
cancels; the detail read is keyed so a stale answer is dropped.

## The web

### DEBT-62 One frontend function is 293 lines, and nothing measures a function's length

Severity: medium · Confidence: measured

`web/eslint.config.js:80` sets `complexity`, `max-params`, `max-depth` and
`max-nested-callbacks` but no `max-lines-per-function`. The result:
`ConfigForm` (`web/src/features/settings/SettingsPanel.tsx:40`) is 293
lines, `PullRequestForm` (`web/src/features/review/ReviewPanel.tsx:233`)
125 and `CommitForm` (`web/src/features/branch/CommitForm.tsx:39`) 126. Files
are measured — `scripts/check-file-length.sh` holds `.ts` and `.tsx` to the
500/800 targets, and the longest, `web/src/features/issues/IssuesPanel.tsx`,
is 359 lines — but a function can grow to fill one with nothing to say so.

**One way to fix it.** Add `max-lines-per-function` to eslint at a number
the split `ConfigForm` meets; split `ConfigForm` by fieldset.

**Done when.** `yarn lint` fails a 300-line component.

## The gates, the build and the tests

### DEBT-64 The condition-coverage worklist: 324 one-sided conditions, and `internal/store` is the worst

Severity: medium · Confidence: measured

At this commit the gobco report (`task cover:branch`, floor 91 %) shows 324
of 1,960 conditions observed only one way. By package: `internal/tui`
112/753, `internal/cli` 60/194, `internal/forge` 31/184, **`internal/store`
29/43** — nearly every `err != nil` in `announce.go` and `cache.go` is never
true — `internal/webserver` 25/162, `internal/wiring` 21/69. Nine conditions
were never evaluated at all: six in `internal/cli/cli.go` (`:177`, `:178`,
`:221`, `:226`, `:251` — the `--web` and dry-run branches of the root
command) and three in `internal/wiring/forge.go` (`:47`, `:56`, `:95`).

**What it costs.** The floor holds at 91.3 %, three tenths above the line,
so the next feature with an untested error path fails the gate on someone
else's PR. The store's error paths are exactly the ones a locked or corrupt
database would take.

**One way to fix it.** The report is the worklist: a store test against a
read-only or pre-corrupted database file; a `--web` root-command test; the
three forge-wiring conditions.

**Done when.** `internal/store` is under 10 one-sided conditions and no
condition in the module is never-evaluated.

### DEBT-65 The web has no condition gate, and its e2e drives no write

Severity: medium · Confidence: read

`task check` (`Taskfile.yml:477`) runs the web's lint, client-drift check and
unit tests beside the Go gates, but what those tests are held to is thinner.
`web/vitest.config.ts:35` sets `thresholds: { lines: 85, branches: 85 }`
under the v8 provider — statement branches, not gobco-style per-condition
coverage, and nine points below the Go statement floor of 94. The e2e suite
is four specs (`web/e2e/a11y.spec.ts`, `web/e2e/screens.spec.ts`,
`web/e2e/smoke.spec.ts`, `web/e2e/theme.spec.ts`), outside `task check`
(CI's `e2e` job and `yarn test:e2e` run it), with no `workflow --web`
backend — acknowledged at `.github/workflows/ci.yml:100` ("No backend": the
specs answer the API themselves, or read a VITE_MOCK build's fixtures) — so
no test drives any of the twelve write actions end to end.

**One way to fix it.** The e2e job starts `workflow --web` against a fixture
repository so one spec can commit, push and open a pull request; the web's
branch floor becomes per-condition.

**Done when.** One Playwright spec performs a write against a running
server, and the web's coverage floor measures each condition both ways.

### DEBT-67 The event stream lives outside both generators

Severity: medium · Confidence: read

`GET /api/events` is registered by hand (`internal/webserver/webserver.go:156`
— "a streaming response the strict, one-response-object interface cannot
express") and the browser consumes it with a raw `new EventSource`
(`web/src/api/snapshot.ts:62`); the generated `streamEvents`
(`web/src/api/generated/sdk.gen.ts:283`) is never called. The only thing
keeping the payload honest is the runtime `zSnapshot.safeParse`
(`web/src/api/snapshot.ts:69`). A frame that fails it now marks the stream
*Out of date*, with why, rather than vanishing (`web/src/api/snapshot.ts:73`),
but nothing checks that the frames the server writes are ones the client
reads — and `zSnapshot` strips a key it does not know rather than failing, so
a `Snapshot` field added in Go without regenerating the client is dropped in
silence.

**One way to fix it.** Assert the SSE frame shape with a test that decodes a
server-produced frame with the client's schema, losslessly.

**Done when.** A Go test holds `stream.go`'s frames in a file a client test
feeds through `zSnapshot` and the stream hook.

### DEBT-68 Dependency posture worth knowing

Severity: low · Confidence: read

The generated TypeScript tree is owned by a pre-1.0 generator,
`@hey-api/openapi-ts ^0.99.0` (`web/package.json`), whose minor releases
change output; `@hey-api/client-fetch` is in `dependencies` but listed under
knip's `ignoreDependencies` because only the generated tree imports it; a
`resolutions: { "js-yaml": "^4.3.2" }` override carries no comment saying
why; and `typescript ~6.0.3` is the only patch-pinned dependency, with no
note on what a minor bump breaks. On the Go side `charmbracelet/ultraviolet`
is an indirect pseudo-version.

**Done when.** Each pin and override carries a one-line reason beside it,
or is removed.

### DEBT-69 Docs that lag the code

Severity: low · Confidence: read

- `CLAUDE.md:35` lists `internal/slack/`; the package was renamed to
  `internal/messaging` in `d229dc1`, and the layout block also omits
  `api/`, `internal/api`, `internal/keychain`, `internal/progress`,
  `internal/buildinfo`, `internal/store`, `internal/web`,
  `internal/webserver`, `internal/tui/frame` and `internal/tui/layout`.
- Residual "Slack" after the rename: the root command's `Short`
  (`internal/cli/cli.go:171`), the help group `groupReviewSlack`
  (`internal/tui/keys.go:88`), `FEATURES.md:30`, `web/index.html:9`.
- `docs/content/docs/usage.md:56` says "the five panes" and `:78` says
  "`1`–`5`"; `internal/tui/panes.go:27` has `paneCount = 6` and the jump
  binding shows `1-6` (`internal/tui/keys.go:202`); the screen mock at `docs/content/docs/usage.md:30-49`
  omits the Reviews pane. `FEATURES.md:52`'s settled-decision text says
  "Five panes" for the same reason — stale text, not a decision reopened.
- `usage.md` never mentions `--web`, and neither does `README.md`; the only
  user-facing references are the flag's one line in the generated
  reference, the errors page, and the problem-code and streams sections of
  `docs/content/docs/scripting.md`. There is no page naming the web's
  sections, its stream, its theme or which actions it supports.
- `FEATURES.md:12` is pinned to `817d323`, twenty-odd commits back; FEAT-26
  (`:118`) and FEAT-31 (`:143`) carry inline `Done:` notes instead of the
  removal the standing rule asks for.

**Done when.** `grep -n 'internal/slack' CLAUDE.md` prints nothing;
`usage.md` has a web page and says six.

### DEBT-70 The web server's sanitize exemption is not written down

Severity: low · Confidence: read

CLAUDE.md's database standard asks that stored text be sanitized "again on
the way out through `internal/sanitize` at the seam that renders it".
`internal/webserver` makes no call into `internal/sanitize`. That is
defensible — the web's rendering seam is React, which escapes text nodes,
and `dangerouslySetInnerHTML`/`innerHTML` are banned by
`web/eslint.config.js:11` — but the exemption is stated nowhere, so the next
reader either adds a redundant sanitize pass or wonders whether one was
forgotten.

**Done when.** One sentence beside the RFC 9457 convention in CLAUDE.md
says why the web server does not sanitize on the way out, and what would
change that.

### DEBT-71 `wiring` returns `tui.Deps`, though three surfaces consume the seams

Severity: low · Confidence: read

`wiring.Deps` (`internal/wiring/wiring.go:72`) returns `tui.Deps`, so the
wiring package imports the terminal interface; the CLI's `WebDeps`
(`internal/cli/cli.go:276`) then narrows that bundle for the web server.
The seams are not the terminal's — they are the loop's. That import no
longer stands in the way of shared composition: `internal/loop` takes each
seam as a plain argument (`loop.PullSeams`, `loop.AnnounceSeams`) and never
needed `wiring`. What is left is narrower: a seam only the CLI or the web
needs must still be declared on `tui.Deps`, as a `RecentCommits` for
`standup` would be — which is why `standup` reads its commits from the
repository directly instead (`internal/cli/standup.go:97`).

**One way to fix it.** Move the bundles that depend only on leaf types —
`JiraDeps`, `GitDeps`, `ForgeDeps`, `MessagingDeps`, `HookDeps` — to a
package all three surfaces import, with type aliases left in `tui`.
`StoreDeps` carries `tui.AnnouncedPost`, so it could move only with that
type, and `EditorDeps` carries Bubble Tea's `tea.Msg` and `tea.Cmd`, so
`wiring` would still return `tui.Deps`.

**Done when.** The seam bundles are declared where all three surfaces can
import them without importing each other. Deferred — YAGNI until a seam the
terminal does not use has to be added to `tui.Deps`.

### DEBT-72 `task container:check` is red for reasons of its own

Severity: medium · Confidence: reproduced

`task container:check` (`Taskfile.yml:510`) runs `task check` inside the
build container, and that run fails whatever the change under test:

- **Root reads what the test locked.** `internal/testshape/run_test.go:118`
  `TestRunReportsADirectoryItCannotList` sets its temporary directory to
  mode `0o100` and expects `Run` to fail listing it. `build/Dockerfile`
  declares no `USER`, so the container runs the gate as root, root lists
  the directory whatever its mode, and the test fails. Reproduced during
  the surface review's Phase 0, on a `git archive a3e773d` tree.
- **A cancel test that failed once.**
  `internal/proc/group_unix_test.go:20`
  `TestStartKillsTheGrandchildWhenTheRunIsCanceled` failed once under
  `-race` in the same container run and passed when re-run. It has not
  failed since. One reading, *read* rather than proven: `docker run` without
  `--init` leaves `task` as PID 1, which does not reap the re-parented
  grandchild, and `kill(pid, 0)` succeeds on a zombie until `gone`'s
  three-second poll gives up.
- **The scheduled run never got that far.** The Container workflow's only
  run so far, 35620452149 on main (2026-09-21, at `133c166`), failed in
  `container:build`, in the Dockerfile step that downloads and extracts
  zizmor: it unpacked with `--strip-components=1 --wildcards '*/zizmor'`,
  though the release archive holds `zizmor` at its root. `b4c4484` (on main
  since 2026-09-22) changed that line to extract `zizmor` directly
  (`build/Dockerfile:168`). No scheduled run has happened since, so that the
  build step now passes in CI is *read*, not measured.

**What it costs.** The weekly Container workflow is the one check that the
container the release is built in can run the gate, and it is red. Once its
build step passes it stays red on the testshape failure, so a real
regression inside the container would hide behind a known one.

**One way to fix it.** Skip the permission test when `os.Geteuid() == 0`,
saying why in the skip message, or run the container as a non-root user (a
`USER` in `build/Dockerfile`, or `--user "$(id -u):$(id -g)"` in
`container:check`); add `--init` to the `docker run` if the cancel test
fails again.

**Done when.** `task container:check` exits 0 on a clean checkout, and the
next scheduled Container run is green.

## Deliberate trade-offs that carry a cost

These were chosen on purpose and are written down at their sites. They are
not debt; they are listed because each one costs something a reader should
know about.

- **Four packages sit exactly at their file budget** — `internal/tui`
  38/38, `internal/webserver` 15/15, `internal/cli` 13/13,
  `internal/config` 13/13 (`scripts/package-size-budgets.txt`). That is the
  gate working as designed: the next file in any of them is a decision (a
  split, or a bump with the WHY rewritten), not an accident. The cost is
  that any change adding a file there must carry its budget row in the same
  commit or fail `task check`.
- **The web is read-only under `--dry-run`** (`internal/webserver/guard.go:40`
  documents it): every unsafe method answers 403 at one gate, where the
  terminal simulates each write and narrates it. The browser reads
  `dry_run` from `getHealth`, says so in a banner, and holds every write
  before sending it (`web/src/api/client.ts:24`), so the server's 403 is
  only the backstop. The cost is that a web write under dry run is refused
  outright rather than simulated: the browser cannot show what the write
  would have done, as the terminal's narration does. The blanket refusal
  itself is the intended design.
- **`staleTime: Infinity`** (`web/src/queryClient.ts:8`) with the event
  stream as the sole freshness source. Correct for a pushed snapshot; the
  cost is that the config query never refetches on its own and a stalled
  stream leaves stale data with no refetch to fall back on. The issue
  detail is the one query with a finite `staleTime`
  (`web/src/features/issues/issueApi.ts:23`): the stream carries only the
  list's slim issues, so a reopened issue is read again after a minute.
- **The progress spine's per-system hue is color-only** (`internal/tui/spine.go:68`),
  mitigated by the stage name, or its initial when compact (`internal/tui/spine.go:51`).
  Part of the visual system UX.md says should not change; the cost is one
  channel the monochrome reader does not get.
- **A condition-coverage skip list of two** (`UNANALYZABLE`,
  `scripts/gobco-report.sh:82`): gobco ignores build tags, so it cannot read
  a package whose files come in tagged twins, and `internal/proc/pgroup` and
  `internal/web` are named there with that reason beside them. Every other
  package is read, and one that becomes unreadable without being named fails
  the gate rather than shrinking the number. The cost is that the two named
  packages' conditions go unmeasured — platform glue and an embed stub, with
  no branch worth the count — and that the next tagged twin must join them.
