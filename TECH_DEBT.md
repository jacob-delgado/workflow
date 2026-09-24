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

No open entry of its own. What it carries on purpose — the two composers'
field handling written twice, and the spine's color-only hue — is under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost).

## The web

No open entry of its own. What the web's gates still lack — a condition
gate, and an end-to-end run that drives a write — is DEBT-65, with the other
gates below.

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
`web/vitest.config.ts:38` sets `thresholds: { lines: 85, branches: 85 }`
under the v8 provider — statement branches, not gobco-style per-condition
coverage, and nine points below the Go statement floor of 94. The e2e suite
is six specs (`web/e2e/a11y.spec.ts`, `web/e2e/layout.spec.ts`,
`web/e2e/panes.spec.ts`, `web/e2e/screens.spec.ts`,
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

### DEBT-71 `wiring` returns `tui.Deps`, though three surfaces consume the seams

Severity: low · Confidence: read

`wiring.Deps` (`internal/wiring/wiring.go:72`) returns `tui.Deps`, so the
wiring package imports the terminal interface; the CLI's `WebDeps`
(`internal/cli/cli.go:277`) then narrows that bundle for the web server.
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

### DEBT-72 `task container:check` is red for reasons of its own

Severity: medium · Confidence: reproduced

`task container:check` (`Taskfile.yml:510`) runs `task check` inside the
build container, and that run fails whatever the change under test:

- **Root reads what the test locked.** `internal/testshape/run_test.go:118`
  `TestRunReportsADirectoryItCannotList` sets its temporary directory to
  mode `0o100` and expects `Run` to fail listing it. `build/Dockerfile`
  declares no `USER`, so the container runs the gate as root, root lists
  the directory whatever its mode, and the test fails. Reproduced in the
  build container on 2026-09-22.
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

- **Five packages sit exactly at their file budget** — `internal/tui`
  41/41, `internal/webserver` 17/17, `internal/cli` 13/13,
  `internal/config` 13/13 (`scripts/package-size-budgets.txt`), and
  `internal/forge` 12/12, at the default the gate applies to a directory the
  file does not list. That is the gate working as designed: the next file in
  any of them is a decision (a split, or a bump with the WHY rewritten, or
  for `internal/forge` a first entry with its WHY), not an accident. The
  cost is that any change adding a file there must carry its budget row in
  the same commit or fail `task check`.
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
