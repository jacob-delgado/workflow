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

### DEBT-10 Click math mirrors the view, by hand

Severity: low · Confidence: read

- Evidence: the wrapped-jobs click bug is fixed — `commandRun.click` now skips
  every row a wrapped jobs line took, not one. What remains is tidiness: each
  click handler still recomputes its view's layout from constants
  (`internal/tui/run.go`, `internal/tui/picker.go`, `internal/tui/commits.go`),
  where the issues pane already asks a shared `rowAt`.
- Cost: a change to one of those views can silently break its click handler
  again, the way the jobs line did.
- Remedy: one `selection` value with `moved`, `window` and `rowAt`, returned
  by the view so the click handler asks it.
- Done when: the picker, run and commits click handlers contain no layout
  constants.

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
- The pane set is written down in six places: `internal/tui/panes.go`,
  `key.WithKeys("1","2","3","4","5")` (`internal/tui/keys.go:58`),
  `pane(msg.String()[0] - '1')` (`internal/tui/tui.go:196`), the help groups,
  a second list of names in `internal/tui/spine.go:54`, and the literal
  "(4 Review)" in `internal/tui/slack.go:113`.
- DONE — the run overlay no longer re-reads every line on every frame: it folds
  each line into the job state as the line arrives (`hooks.NextJob`) and keeps
  the parsed jobs on the overlay, and it caps the kept output at `maxRunLines`,
  so a chatty hook is no longer quadratic and cannot grow the model without end.
- Vestigial: `Style.ASCII()` was called only by a test and is now deleted;
  `hookgenState.offered` was already gone. Left as they are: the
  `var _ help.KeyMap = keyMap{}` assertion documents a real conformance CLAUDE.md
  asks for, and the "FullHelp is every key" comment is now true by construction
  (`help_internal_test.go` proves it).
- "1 files staged" (`internal/tui/composer.go:155`) is pinned by
  `internal/tui/composer_test.go:50`. `Breaking: false` is hardcoded at
  `internal/tui/composer.go:92`.
- Loads carry no sequence number, so the last answer to arrive wins.
  `detailLoaded` checks the key only (`internal/tui/detail.go:49`). Every
  request is bounded at ten seconds, which keeps the window narrow.

## The clients: forge, Jira and Slack

### DEBT-17 Three HTTP clients copied by hand

Severity: low · Confidence: read

Done for the security-critical part, the done-when: `CheckRedirect` is written
once. `internal/httpx` (standard library only) holds the `Doer` seam, the
`ErrRedirected` sentinel and the redirect-refusing `Client`; the Jira, forge and
Slack clients alias `Doer`/`ErrRedirected` to it and their `HTTPClient` delegates
to `httpx.Client`, so the redirect policy — the piece where "one of them forgot
the mask" is the standing warning — has a single copy that a test guards.

Also done: the transport-error message reads one way. The duplicated `cause`
(Jira) and `withoutURL` (Slack) helpers are gone — `httpx.Cause` strips
net/http's `*url.Error` down to its inner error in one place, and all four
transport-error sites use it, so the forge and Slack's identity check no longer
echo the request URL the other two already dropped. `httpx_test.go` guards it.

- Left per service, deliberately: `bodyLimit` differs (16 MB for Jira and the
  forge, 1 MB for Slack), and status mapping genuinely differs between them.
- What remains, lower value: a refused redirect still reads "could not reach …"
  though the server did answer with a redirect the client declined. The cause is
  named (the error wraps `ErrRedirected`, which a caller tells apart with
  `errors.Is`), so this is a wording nicety rather than a wrong signal, and
  settling it cleanly means unifying three clients' distinct unreachable
  sentinels — kept as a deliberate minor choice. The `call[T]`-style exchange
  helper the forge has and Jira repeats is a further refactor on its own.

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

- What remains, lower value: the config secrets are still bare strings;
  `config.Redact` masks them wherever a configuration is shown, so a typed
  masking secret (as `forge.Token` already is) would be a defense-in-depth
  against a future raw `%v`, not a present leak.
- Not here: adding a forge still edits several places; folding those into a
  `dialect` is FEAT-54, which keeps its own PR.

## Configuration, wiring and the command line

### DEBT-21 The forge connection is built twice, and remembered when it fails

Severity: low · Confidence: read

Fixed: `onceConnected` (`internal/wiring/wiring.go`) caches a connection only
once it succeeds and retries after a failure, so `gh auth login` in another
terminal is found on the next attempt rather than only on a restart — the
"Done when" is met. The duplicated `forge.Resolver` literal is now the shared
`wiring.ForgeResolver`, which both `connectForge` and `checkForge` call, so the
two cannot drift in where they look for a credential.

- What remains: the preamble around the resolver — parse the remote, apply the
  configured kind, find the API base — is still shaped the same in both, but the
  two report failure differently (doctor prints a line per step; wiring returns
  wrapped errors), so a single `connect` would have to thread that divergence.
  Left until a third caller earns it. The success arm of the interface's own
  `connect()` sites is DEBT-36.

### DEBT-23 The file format has no version and rejects what it does not know

Severity: low · Confidence: reproduced

Held, deliberately, against CLAUDE.md's YAGNI rule ("no backwards-compat shims
for unreleased code"). The remedy — a version field plus a rename table that maps
an old key to the one that replaced it — is machinery for renames that have not
happened: nothing is released, so no key has ever been renamed, and the table
would be empty. The current behavior already names the unknown key
(`invalid .workflow.json: … json: unknown field "url"`), which is enough to
correct a typo. When the first real rename lands (FEAT-51 territory), the version
field and the table for that specific rename are worth adding — with the rename
to point at, not before.

- Done when: the first rename exists and its old key produces a message naming
  the new one.

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

- What remains, each its own change: a default deadline in `proc.Run` (risky —
  a legitimate slow clone or hook must not be killed, so it needs a generous,
  per-command bound rather than one blanket number); the Windows twin of the
  process group, a job object, which cannot be validated from here (see
  DEBT-32); and a "stop" key in the run overlay, which needs a per-run cancelable
  context threaded through the `tui.Deps` seam contract rather than the shared
  root the seams capture now.

## Git, hooks, conventions and processes

### DEBT-30 `origin` is spelled out in eight places

Severity: low · Confidence: read

Two of the three remedy items are done. `git fetch` runs before branching
(`gitrepo.FetchCommand`, wired through `fetchOrigin`), and the base's age is
shown ("from BASE, fetched X ago" via `Branch.BaseUpdated`/`Model.baseAge`). And
the word `origin` now appears once in production code: `gitrepo.DefaultRemote`,
which the base fallback, the fetch and push commands, the origin-URL read and the
interface's remote checks all reference — `Branch.BaseName` strips whatever
remote a base carries rather than assuming origin.

- What remains: nothing consults `remote.pushDefault`, so a repository whose push
  default is not origin, or a fork, still assumes origin. That changes observable
  push and base behavior and cannot be exercised against a live remote in a unit
  test, so it wants a seam and a RED test of its own. See FEAT-12 and FEAT-15.

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

### DEBT-33 Smaller items in the local packages

Severity: low · Confidence: read

The reproduced bugs here are fixed: the issue-key match no longer reads `UTF-8`,
`SHA-256`, `CVE-2024` or `ISO-8601` as a key (`standardAbbreviations` in
`internal/convention/convention.go` skips them); `Message` and `PullRequestBody`
find an existing reference by whole word, not substring, and keep an appended
`Refs:` in the same trailer block as `Co-authored-by:`; `ReadBranch` reverses the
whole range before capping, so a branch past 200 commits still titles its pull
request from its first commit; the dead `ReadConfig`/`wireHook`/`Runner` and
`File.Executable` are gone; and a `@` branch name is refused as `@`, not as empty.

The issue-key upgrade is done: `convention.IssueKey(text, project)` takes the
project as a parameter — a pure argument, so `convention` stays a stateless
package — and reads a branch as an issue only when its project matches the
configured `jira.project`, falling back to the shape guard when none is set. The
five call sites pass `jira.project` from the configuration.

Done for the data clump: the `(ctx, run Runner, dir string)` triple that
repeated on twelve gitrepo functions is now carried by a `gitrepo.Repository`
value — `gitrepo.At(run, dir)` returns the handle, and `Describe`, `ReadBranch`,
`Status`, `Stage`, `Unstage`, `CreateBranch`, `LocalBranches`, `Checkout`,
`WorktreeAdd`, `HooksDir`, `RecentCommits` and `CheckIgnored` are methods on it.
The wiring builds one `repo` and hangs the seams off it; the `Status`/`Stage`
"dir must be the root" requirement, which was silent, is now written on `At`. The
four `proc.Command` builders stay free functions: they take only `dir`, so they
were never part of the triple.

- Left as they are, YAGNI over microseconds: the `regexp.MustCompile` calls sit
  inside functions, and `Output.Wait` (`internal/proc/start.go`) blocks on a
  second call that no caller makes.

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

### DEBT-36 The forge's success path is never exercised outside its package

Severity: low · Confidence: measured

Done for the interface's forge seams: `forgeDepsFrom` (`internal/wiring/forge.go`)
takes the `connect` as a parameter, and `forgedeps_internal_test.go` drives
FindPullRequest, CreatePullRequest, CheckStatus, ReviewRequests and Author with a
client pointed at a fake GitHub, so the success arm of each connect guard is now
exercised — only failures were before. `config init --global` is tested too
(`config_init_guided_test.go`).

- What remains: `doctor`'s `askForge` and `checkSlack` still build real clients
  against real addresses (`internal/cli/doctor.go`), so their success arm is only
  reached against a live forge/Slack. Injecting the API base and `Doer` there is
  the remaining work, tied to DEBT-17's shared transport.

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

- **The issue list is one query, capped at 50, with no paging.** The pane
  says "showing N of M", so the cut is not silent. FEAT-02, FEAT-03.
- **Every redirect is refused**, in all three clients, to keep credentials
  from following one. The cost is DEBT-17's unhelpful message.
- **`gh` failures are swallowed** when resolving a token, so `doctor` cannot
  say "`gh` is installed but not signed in to this host".
- **There is no `glab` step**, because it reports its token as prose.
- **Configuration is looked for in the current directory, then home.** From
  a subdirectory of a repository, the repository's own file is skipped for
  the one at home, and `config init` writes to wherever it was run. FEAT-44.
- **Convention rules are constants**: the eleven commit types, the 72 and 48
  character limits, `fix/` and `feat/`, the `Refs:` trailer, the title from
  the oldest commit. A team with other conventions has no setting. FEAT-14.
- **`$EDITOR` is split on spaces and never given to a shell**, so an editor
  whose path contains a space fails, and `GIT_EDITOR` and `core.editor` are
  ignored.
- **Failure locations need a file extension**, which misses `Dockerfile:3`,
  though hadolint is in this repository's own gate. FEAT-25.
- **lefthook's decorative output is scraped**, with no check of the installed
  version. A change degrades to "no job rows", not to a wrong answer.
- **A line over 1 MiB ends output capture** for that run; the exit status is
  still reported.
- **Interface seams may be nil**, which costs 29 nil checks in production
  code for the benefit of partial test fakes. One path has no check
  (`startPush` calls `m.deps.Git.Push`), which today's wiring always sets.
- **Map literals stand in for switches** so that gobco has no uncoverable
  arm. The cost is in DEBT-46.
- **Timing and layout are named constants, not settings**: the ten-second
  request timeout, the twenty-second CI interval, the 90- and 60-column
  breakpoints, five comments shown. FEAT-47, UX-02.
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

- `os/exec` is imported only in `internal/proc`.
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
- No file is over 500 lines, there is no package-level mutable state, and
  there are no goroutines outside `tea.Cmd` and `sync.OnceValues`.
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
- The Go version agrees across `go.mod`, `mise.toml`, the Dockerfile, the docs
  module and the README badge. `go.mod` has no `replace`, and every direct
  dependency is used.
- The generated command reference is checked for drift in both directions.
- Every shell script sets `set -euo pipefail` and quotes its expansions; none
  disables a shellcheck rule inline.
- Across 48 commits, no subject is over 72 characters and none ends with a
  period.
