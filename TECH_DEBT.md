# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-22". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts.

Checked against commit `817d323` on 2026-09-17. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **An audit.** Every non-test Go file, every script, every workflow and
   every page of docs was read in full, against the standard the project sets
   for itself in [CLAUDE.md](CLAUDE.md).
2. **An attempt to refute it.** Each finding was handed to a second pass that
   had not written it, with instructions to prove it wrong: read the callers,
   check the primary source, run an experiment. Of 126 claims, 98 held as
   written, 28 were corrected in some detail and none was refuted; the
   corrected versions are what is written here. Experiments ran against
   scratch copies and scratch repositories; nothing in this repository was
   changed.
3. **A third read.** The cited lines of every high and medium entry were read
   again while writing this file.
4. **Measurement.** `task test:cover` and `task cover:branch` were run at this
   commit for the test-suite section.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). Anything that rests on outside behavior nobody checked says so.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it appears under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost), which lists
the choices that were made on purpose and written down, and what they cost.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule for what it found: those findings went to the maintainer directly. The
entries below contain nothing that helps anyone misuse a credential, a
terminal or a release.

## The terminal interface

### DEBT-07 Six overlays, one state machine, written six times

Severity: low · Confidence: read

Done for the state machine: `sendState{sending bool; err error}`
(`internal/tui/sendstate.go`) is now the one shape, held as a `send` field on
every overlay that posts, applies, creates, links, opens or writes —
`statusPicker`, `commentPreview`, `branchCreator`, `commitComposer`,
`prComposer`, `slackPreview`, `issueLinker` and `hookgenOffer`. The error is
named one way (`send.err`) rather than three (`applyErr`, `err`, `problem`); each
failure applier now goes through the shared `send.failed(err)` transition (the
per-overlay extras it keeps — a form reset, a skipped fetch, the pane-level Slack
error — sit beside it); and `pinnedOutcome` draws the outcome from that one
value. The comment preview already survives an editor failure
(`commentEdited.apply`). Existing overlay tests, which drive each overlay's
failure, held green through the change and coverage did not move.

Also done: the three body editors now share one `textEdited` message
(`internal/tui/overlay.go`), routed to whichever overlay is `editable` — each of
`commitComposer`, `prComposer` and `slackPreview` takes its edit through an
`applyEdit` method rather than owning a near-identical `commitBodyEdited` /
`prBodyEdited` / `slackTextEdited` type and apply. `commentEdited` stays its own
message: it carries the issue and opens the preview from the issues pane rather
than updating an open overlay, so it is not the same shape.

### DEBT-12 One model that every message can change

Severity: low · Confidence: read

Done for help: `helpOpen` is gone. Help is now a `helpOverlay`
(`internal/tui/help.go`) like the pickers and composers — it renders the key
list it captures at open time, carries its own scroll so opening it no longer
disturbs the detail pane's, and marks itself `lightBordered` so it keeps the
light border an action overlay does not wear. `handleHelpKey` and its four
special-case sites are gone; `Update` routes it through the overlay seam like
everything else.

- What remains, low value: `Model` still keeps some feature state at the root
  (`runs`, `draft`, `prDraft`) rather than beside the code that owns it, and each
  pane's state is still changed from wherever a message reaches it. That is the
  judgment-call half the entry itself calls "not a rewrite"; left as noted.

### DEBT-13 Small things in the interface

Severity: low · Confidence: read

- DONE — `Model.branchIssue()` is the one place a branch name is read as an
  issue key; the five sites on `m.branch.branch.Name` call it. `"origin/"` is no
  longer trimmed by hand: `gitrepo.Branch.BaseName()` strips the remote (and does
  not hardcode origin), and the two sites that trimmed by hand call it.
- DONE — the run overlay no longer re-reads every line on every frame: it folds
  each line into the job state as the line arrives (`hooks.NextJob`) and keeps
  the parsed jobs on the overlay, and it caps the kept output at `maxRunLines`,
  so a chatty hook is no longer quadratic and cannot grow the model without end.
- Vestigial: `Style.ASCII()` was called only by a test and is now deleted;
  `hookgenState.offered` was already gone. Left as they are: the
  `var _ help.KeyMap = keyMap{}` assertion documents a real conformance CLAUDE.md
  asks for, and the "FullHelp is every key" comment is now true by construction
  (`help_internal_test.go` proves it).
- Loads carry no sequence number, so the last answer to arrive wins.
  `detailLoaded` checks the key only (`internal/tui/detail.go:49`). Every
  request is bounded at ten seconds, which keeps the window narrow.

## The clients: forge, Jira and Slack

### DEBT-19 Smaller items in the clients

Severity: low · Confidence: read

Fixed: a 429 now reads as `httpx.ErrRateLimited` in all three clients — Jira no
longer calls it an unexpected status, the forge tells it apart from a 403
refusal, and Slack (both the auth check and the post path) no longer calls it a
rejected credential, so a rate-limited caller is told to wait rather than that
its credential is wrong. And every `Stringer` type now carries the static
assertion CLAUDE.md asks for (`AuthMode`, `SlackMode`, `Violation`, `Subject`,
`Kind`, `Token`, `Source`).

Also done: a 429 now names how long to wait when the server said so.
`httpx.RateLimited(header)` reads the `Retry-After` seconds and wraps
`ErrRateLimited` with the wait ("… (in 30s)"), so all three clients tell the
caller when to retry rather than only that it was rate limited. The header's
HTTP-date form, which these APIs do not use for a 429, and a missing value fall
back to the bare sentinel, so `errors.Is` still matches.

Also done: the config secrets are typed. `config.Secret` (as `forge.Token`
already was) carries `jira.token`, `slack.token`, `slack.webhook_url` and
`forge.token`, masking under every formatting verb — a `Config` printed with
`%+v` shows `****`, not the value — and taking an explicit `Reveal` to read.
`config.Redact` still masks the display paths; this stands behind it, so a
future raw `%v` cannot leak what Redact was never asked to hide. A test proves a
Secret, and a whole Config, mask when printed.

- Not here: adding a forge still edits several places; folding those into a
  `dialect` is FEAT-54, which keeps its own PR.

## Configuration, wiring and the command line

### DEBT-24 Nothing can be canceled, and no subprocess has a deadline

Severity: medium · Confidence: reproduced

- Evidence: `root.Execute()` (`internal/cli/cli.go:89`) gives every command
  `context.Background()`; production code contains no `ExecuteContext`,
  `NotifyContext`, `WithTimeout`, `WithCancel` or `WithDeadline`. HTTP has
  `RequestTimeout`; `git`, `gh` and `lefthook` have nothing. `build`
  (`internal/proc/start.go:137`) sets no process group, `Cancel` or
  `WaitDelay`. `Start` documents "Canceling ctx kills the program"
  (`internal/proc/start.go:49`), which cannot happen. A push gets
  `GIT_TERMINAL_PROMPT=0` (`internal/gitrepo/branch.go:238`); a commit gets no
  environment at all (`:222`).
- Cost: a hung `git status` or `gh auth token` leaves its pane on "loading…"
  forever. A streamed run ignores every key but `ctrl+c`, which quits the
  program; a parent built the same way left its child running with a parent
  process of 1, so a push can complete after the user "quit", and the commit
  message's temporary file is never removed. `GIT_TERMINAL_PROMPT` governs
  git's own prompts only: ssh and gpg open the terminal directly while the
  interface owns it (shown for ssh's documented behavior and a child's access
  to `/dev/tty`; gpg was not installed to try).
- Done for the root context: `Execute` now builds a `signal.NotifyContext` and
  runs the tree with `ExecuteContext`, so SIGINT and SIGTERM cancel the context
  every command runs under — a hung `doctor --online` or a slow `git status`
  stops on the first Ctrl+C rather than a second. A test drives the internal
  `execute` with a canceled context and proves a command's git subprocess is
  canceled (without `ExecuteContext` it ran under `context.Background` and
  ignored it). The interface runs the terminal in raw mode, where Ctrl+C is a key
  not a signal, so this does not fight Bubble Tea.
Done for the process group, the original done-when: `proc.Start` now puts a
streamed child in its own process group (`grouped`, build-tagged —
`group_unix.go` sets `Setpgid` and, on cancel, `SIGKILL`s the whole group;
`group_other.go` is a no-op where a Unix session does not apply). A test cancels
a streamed run and proves the grandchild it spawned is killed with it rather than
reparented to init and left running — the leak where a push finished after the
user quit. `Run` and `Capture`, quick reads that spawn nothing, are left alone.

Done for the deadline: `proc.Run` is bounded by `DefaultRunTimeout` (30s), so a
hung quick read — a `git status` on a dead mount, a `gh` call to a host that
never answers — recovers rather than leaving a pane loading. `RunWithin` exposes
the bound for a read whose limit differs; streamed `Start` and piped `Capture`,
which can run long, are untouched.

Done for the stop key: while a command streams, `s` stops it. `proc.Start` now
gives each run its own cancelable context — a child of the caller's, so one run
is ended alone — and exposes it as `Output.Stop`; the run overlay keeps that
handle and, on the key, kills the process group and shows the run as stopped
rather than failed. A test drives a hung hook, presses the key, and proves the
run's own Stop was called.

- What remains: the Windows twin of the process group, a job object, cannot be
  validated from here (see DEBT-32).

## Git, hooks, conventions and processes

### DEBT-32 Windows is a release target the code has not met

Severity: low · Confidence: read

Three code fixes are done, each parameterized by GOOS so both branches test from
one machine: `ExistingHooks(dir, goos)` counts a plain file as runnable on
Windows, where Go reports no executable bit, so the hook-generation feature is no
longer dead there; the editor falls back to `notepad` on Windows rather than
`vi`, which it usually lacks; and `Failures(lines, goos)` now reads
`C:\src\main.go:12` as a whole path on Windows — the file:line pattern allows one
drive-letter prefix there, and only there, so a Unix `a:b.go` is not mistaken for
a drive. None was run on Windows, but every branch has a test.

- What remains: the done-when — the test suite running on Windows in CI — is
  deliberately left off. A Windows leg is the discovery tool for whatever else
  the platform breaks, and it must be watched and iterated on a Windows runner
  rather than added blind where it would sit red.

### DEBT-34 Domain values travel as bare strings

Severity: low · Confidence: read

Done for the status category: `jira.StatusCategory` is now a type with
`CategoryNew`/`CategoryIndeterminate`/`CategoryDone` constants, carried on
`jira.Issue` and `jira.Transition`, and the glyph map is keyed by it — so a
mistyped category is a build error, and the `exhaustive` map check (DEBT-46)
guards the glyph table against a new category drawing nothing.

Done for the issue key too: `jira.Key` is now a named type carried on
`jira.Issue.Key` and taken by every issue method — `Issue`, `AddComment`,
`LinkPullRequest`, `BrowseURL`, `Transitions`, `ApplyTransition` — and by the
`tui.JiraDeps` seam, so `Comment(issueKey, text)` and `LinkPullRequest`'s three
adjacent strings can no longer be passed swapped: the compiler rejects it. The
key stays typed across `jira`, `tui`, `wiring` and `cli`.

- Where it deliberately stops: `convention` keeps `string`. `IssueKey` also
  returns forge issue numbers, and typing it `jira.Key` would make the stateless
  `convention` package depend on `jira`; the branch-derived string becomes a
  `jira.Key` at the one tui boundary (`branchIssue`) instead. `BranchName`'s
  arguments (a convention concern) and `GitHook.Name` (a hooks one) are separate
  and stay as they are.

## The test suite

Snapshot at this commit: statement coverage 98.1% against a floor of 95, and
condition coverage 96.4% (1642 of 1704 arms) against a floor of 93. Both
gates print an available ratchet (to 96 and to 94). The weakest packages by
condition coverage are `internal/cli` at 75.7% and `internal/wiring` at 81.6%.
These numbers are a dated reading, not a floor; the floors live in
`Taskfile.yml`.

### DEBT-35 No test runs the root command — DONE

`tui.Run` is now injected into the command tree through `newRootCmd(prompt,
run)` (`internal/cli/cli.go`), so `cli_dryrun_internal_test.go` runs the bare
`workflow` RunE with a fake runner and asserts the model it built shows "DRY
RUN" with the flag and not without. Removing `if dryRun { model.WithDryRun() }`
now fails the suite; gobco sees both arms of `dryRun`.

### DEBT-37 The shared fake world was 27 lines from the file-length gate — DONE

Severity: low · Confidence: measured

The done-when is met: no `internal/tui` test file is within 50 lines of the
500-line limit. The shared fixture is split — `world_test.go` keeps the `world`
struct and its fakes, and the harness that drives the interface (`drain`,
`within`, `live`, `typing`, `click`, the screen assertions) moves to
`harness_test.go` — so the next `tui.Deps` seam no longer pushes the largest file
in the repository past the gate mid-feature. The six other files that sat at
450–485 lines each shed a trailing concern into its own sibling file
(`screen_reload_test.go`, `composer_scope_test.go`, and the like).

### DEBT-38 Smaller items in the tests

Severity: low · Confidence: read

Fixed: there is now an advisory `task fuzz` that discovers every Fuzz function
and gives each a short `-fuzztime` run, so a target that would find a new input
gets the chance the seed-only gate never gave it. The "1 files staged"
pluralization defect is gone (`composer.go` renders `plural(...)`), so no test
pins it any more.

- What remains, low value: `patience` and `within`/`drain`
  (`internal/tui/world_test.go`) still wait on wall-clock time and drop a command
  that overruns, which a slow runner under `-race` can turn into a confusing
  failure. Reworking the Bubble Tea drain to settle on quiescence rather than a
  timer is a delicate test-harness change on its own.
- Three wiring tests skip when lefthook is absent
  (`internal/wiring/hooks_test.go`), which it is in the build container — tied to
  DEBT-41.

## Build, CI, scripts and release

### DEBT-41 The build container is never built, and is not the same gate

Severity: low · Confidence: read

Done for the done-when: `.github/workflows/container.yml` builds `build/Dockerfile`
and runs `task container:check` (the full gate inside the image) weekly and on
demand, so a drift in the Dockerfile — a dead download URL, a base image that
moved, a version that no longer resolves — is caught on a Monday rather than at a
release. It is scheduled and dispatch-only, so it gates no pull request; the
first run is what reveals whatever the container has drifted into, which this
machine cannot exercise (no container runtime here).

- What remains, tied to that first run: `scripts/tool-versions.sh` still passes
  only 14 of the pins and omits lefthook, shellcheck, hugo-extended, cloc and
  deadcode; shellcheck, jq and node come from apt rather than the pins; and the
  curl'd tools are not checksum-verified. Making the gate inside the container
  the same gate — same tool versions, verified downloads — is what the weekly run
  will surface and drive, on a machine that can build the image.

### DEBT-42 mise itself floats in CI — DONE

Every `jdx/mise-action` use now pins `version: 2026.9.3` (the seven `ci.yml`
uses, plus `release.yml` and `pages.yml`), so the mise binary — which carries
the tool registry — no longer floats to the latest release, which for the
uncached release job was younger than the seven-day gate allows anything else
to be. `min_version` in `mise.toml` is raised to `2026.9.3` to match, with a
note that mise cannot pin its own binary from there and the workflows and
devcontainer must travel with it.

### DEBT-43 The gate needs jq and node — now pinned in `mise.toml`

Severity: low · Confidence: read

Fixed for a clean machine: `mise.toml` now pins `node` (the runtime the `npm:`
tools need, which mise does not add on its own) and `jq` (the JSON arithmetic in
`scripts/gobco-report.sh` and `scripts/coverage-summary.sh`), so `mise install &&
task check` no longer depends on whatever `jq`/`node` happen to be installed.

- What remains: the build container still `apt install`s jq and nodejs
  (`build/Dockerfile`) rather than provisioning the pinned versions through
  `scripts/tool-versions.sh`, so the "clean container" leg of the done-when
  belongs to DEBT-41, which also has to teach `tool-versions.sh` to pass them.

### DEBT-44 A gate can still pass having measured only part

Severity: low · Confidence: reproduced

The two clear halves are fixed: `check-file-length.sh` now captures the tracked
list up front, so a `git ls-files` that fails — outside a repository, say — stops
the gate instead of escaping `set -e` and passing on nothing measured
(`scripts/check-file-length_test.sh` reproduces the old pass and guards the fix);
and both coverage floors are required arguments rather than defaulting to 0 or 70
when a caller drops one.

Also fixed: `gobco-report.sh` now lists every package in the module, not only
those with tests, and requires a package with no tests to be named in a
`NO_TESTS` allowlist with its reason — the three thin `cmd/` mains are there. A
new package with neither tests nor an entry fails the gate rather than dropping
out of the "every package" total unseen.

- Deliberate divergence, left as is: the file-length gate measures tracked files
  only, where the license check and testshape also see untracked ones. Its header
  argues the case (a scratch file cannot fail the gate; a new file counts once it
  is `git add`ed), so this is a decision to revisit, not a defect to fix blind.

### DEBT-46 Smaller items in the tooling

Severity: low · Confidence: reproduced

Fixed: `_typos.toml` now sets `locale = "en-us"`; `check-commit-message.sh`
enforces the 72-character subject limit and no trailing period, and strips the
`git commit -v` diff before matching; `.golangci.yml` now runs `exhaustive` with
`check: [switch, map]`, so a new `forge.CIState` or `hooks.JobState` value that a
glyph map forgot fails the lint (the maps that are partial by design carry a
`//nolint:exhaustive` naming why); Dependabot now covers `docs/go.mod` and the
devcontainer's image and features; and the coverage-comment job is guarded to
same-repository pull requests, so a fork's read-only token no longer fails it.

- What remains, each its own reason to defer: `build/Dockerfile` gives
  `GO_VERSION` a default written outside `mise.toml`, and provisions jq, node and
  shellcheck from apt rather than the pins — both belong to DEBT-41. The
  release-tag script pushes the tag before `release.yml` runs `task check`, so a
  red gate could leave a tag with no release; reordering that is the maintainer's
  release path and cannot be exercised without a real release. The coverage,
  file-length-adjacent and docs-drift gate scripts still have no tests of their
  own failure paths. And a scratch `.go` under the gitignored `tmp/` still joins
  `go test ./...`, which is guidance (keep Go scratch elsewhere), not a gate to
  add.

## Docs and configuration drift

### DEBT-49 Smaller drift — DONE

Severity: low · Confidence: read

- The usage guide's per-pane Keys table now lists the five wired bindings it
  omitted — switch view (`v`), switch task (`s`), rebase (`u`), worktree
  (`ctrl+w`) and post-when-green (`w`). `?` is complete by construction:
  `help_internal_test.go` asserts the help groups hold every binding the keyMap
  declares.
- `CONTRIBUTING.md` and `docs/content/docs/contributing.md` no longer enumerate
  the tools — they point at `mise.toml` as the list, so neither can drift from it
  or from each other.

The gobco "reads every package" comments, the version and platform lists, the
scissors line and the CLAUDE.md layout block were corrected earlier.

## Deliberate trade-offs that carry a cost

These were chosen, and the reason is written down in the code or the docs.
They are listed so the cost is visible, not so they get "fixed".

- **Every redirect is refused**, in all three clients, to keep credentials
  from following one. The cost is DEBT-17's unhelpful message.
- **`gh` failures are swallowed** when resolving a token — a signed-out or
  absent `gh` is a miss, not an error, so the next source is tried. `doctor`
  now tells the two apart: when `gh` is on `PATH` but yielded no token it names
  the host it is not signed in to rather than printing the generic list of
  sources.
- **There is no `glab` step**, because it reports its token as prose.
- **Convention rules are constants**: the eleven commit types, the 72 and 48
  character limits, `fix/` and `feat/`, the `Refs:` trailer, the title from
  the oldest commit. A team with other conventions has no setting. FEAT-14.
- **The editor setting is never given to a shell**, so shell syntax in it does
  not run; and `core.editor` is not read, which would have the editor package
  run git. `$GIT_EDITOR`, `$VISUAL` and `$EDITOR` are consulted in git's order,
  and an editor whose path contains a space is kept whole.
- **lefthook's decorative output is scraped**, with no check of the installed
  version. A change degrades to "no job rows", not to a wrong answer.
- **A line over 1 MiB ends output capture** for that run; the exit status is
  still reported.
- **Interface seams may be nil**, which costs 29 nil checks in production
  code for the benefit of partial test fakes. One path has no check
  (`startPush` calls `m.deps.Git.Push`), which today's wiring always sets.
- **Map literals stand in for switches** so that gobco has no uncoverable
  arm. The cost is in DEBT-46.
- **The layout breakpoints are constants, not settings**: the 80- and 60-column
  widths at which the interface collapses the rail and drops the border. The
  timing that changes behavior (request timeout, CI interval) and the comments
  shown are settings; these two are a rendering detail, not a knob. UX-02.
- **Test files are exempt from the complexity linters**, and
  `internal/cli` and `internal/wiring` run their tests serially.
- **gobco runs without `-race`, one package at a time**, so `task check` runs
  the suite twice.
- **The license header's year is a fixed string.** From 2027 a new file still
  says 2026, and a contributor's own copyright line is refused. A fixed
  first-publication year is common practice; it should be written down as the
  policy.
- **Dependabot's cooldown exempts `actions/*` and `github/*`.** CLAUDE.md's
  age-gate rule names only the CVE exception.

## Checked and found clean

Recorded so the next audit can spend its time elsewhere.

- Every response body is closed and read through a limit, and every client is
  built with a timeout.
- Server text is sanitized before it is decoded or shown, at every site in
  the three clients.
- No request or parse error echoes a URL that could hold a credential; the
  webhook URL never enters an error; Jira refuses userinfo in its base URL
  before sending anything.
- Slack announcement text is escaped; issue keys, repository paths and commit
  hashes are path-escaped.
- Configuration: unknown keys are rejected, defaults survive a partial file,
  and "local replaces home, never merged" is implemented as documented.
- Sentinel errors are static and wrapped with `%w` throughout; nothing builds
  an error at the point of failure.
- Every `//nolint` directive was read; each one's stated reason holds.
- The porcelain `-z` status parser handles spaces, quotes, newlines and
  renames. Detached heads, unborn branches and worktrees are handled. No
  parsing depends on the locale.
- `ValidateBranchName` agrees with `git check-ref-format` except for the name
  `HEAD`.
- Stale-response guards are correct for issue detail, transitions, pull
  request lookup, CI checks and the three run messages.
- All 40 `uses:` lines in the workflows are pinned to a full commit hash, and
  workflow permissions are least-privilege.
- `go.mod` has no `replace`, and every direct dependency is used.
- The generated command reference is checked for drift in both directions.
- Every shell script sets `set -euo pipefail` and quotes its expansions; none
  disables a shellcheck rule inline.
- Across 48 commits, no subject is over 72 characters and none ends with a
  period.
