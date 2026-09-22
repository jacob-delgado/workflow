# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-50". Each entry says
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
  code agree). This edition is read and measured; nothing was reproduced
  live.
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

## The composition of the loop

### DEBT-50 The loop is composed three times, once per surface

Severity: high · Confidence: read

`internal/webserver/webserver.go:6` describes the web server as "another
consumer of the wiring, not a second implementation", and for the *clients*
that holds: its `Deps` (`webserver.go:32`) are the same plain function seams
the terminal interface declares. It does not hold for what the surfaces
*do* with those seams. `internal/wiring` (six files) constructs clients and
nothing else, so each surface composes the loop for itself:

- Branch naming — three identical `convention.NewBranchNaming(cfg.Branch.
  Template, cfg.Branch.DefaultPrefix, cfg.Branch.Prefixes,
  cfg.Branch.SlugLimit)` calls: `internal/tui/branch.go:296` `branchNaming`,
  `internal/cli/branch.go:78`, `internal/webserver/branchcreate.go:83`.
- Composing a pull request — `internal/cli/pr.go:222` `composePR` and
  `internal/webserver/pullrequest.go:127` `draftFor` are near line-for-line
  copies; the terminal has its own in `internal/tui/prcomposer.go:119`.
- `ensurePushed` — the same function, the same name, in two packages:
  `internal/cli/pr.go:241` and `internal/webserver/pullrequest.go:171`.
- The announcement and its moment — built three times:
  `internal/tui/messaging.go:137` `announcement` / `:125` `announceMoment`,
  `internal/cli/announce.go:158` `composeAnnouncement` / `:180`
  `announceMoment`, `internal/webserver/announce.go:68` / `:102`.
- The GitLab noun — three copies of `if kind == KindGitLab { "merge
  request" }`: `internal/tui/review.go:49`, `internal/cli/announce.go:194`
  `forgeNoun`, `internal/webserver/announce.go:145` `noun`.
- The guards — `internal/webserver/checkout.go:16` and `internal/webserver/commit.go:18` each
  carry a comment saying they are "the same guard the terminal interface
  applies" (`internal/tui/switchtask.go:88`, the composer), copied rather
  than shared.

**What it costs.** Every parity gap between the surfaces is a copy that one
of them lacks: the review-status offer after a pull request exists in the
terminal (`internal/tui/picker.go:199`) and the CLI (`internal/cli/pr.go:161`)
and not on the web, because there is no one place to put it. Each new action
added to a surface becomes a fourth copy, and a fix to the composition (a
changed moment rule, a new trailer) has to be made three times or diverges.

**One way to fix it.** A new leaf package — `internal/loop` — that imports
only `config`, `convention`, `forge`, `gitrepo`, `jira`, `messaging` and
`proc`, and is imported by `cli`, `tui` and `webserver`. It cannot live in
`wiring` (which imports `tui` for `tui.Deps`, so `tui` importing it back is a
cycle), in `tui` (the web server must not import the terminal), or in
`convention` (`config` imports it, so it can never take a `config.Config`).
Two of the duplicates have better homes than a new package: the noun as a
method on `forge.Kind` (the open/closed spelling CLAUDE.md asks for) and the
naming as `config.Branch.Naming()` (four of its own fields). A `depguard`
rule holds the direction. The CLI's and the web server's copies deleting
cleanly, with their tests unchanged, is the proof the layer is right.

**Done when.** `grep -rn 'NewBranchNaming(' internal/{cli,tui,webserver}`
and `grep -rn '"merge request"' internal/{cli,tui,webserver}` both print
nothing; `ensurePushed`, `composeAnnouncement` and `announceMoment` each
exist in exactly one package; every existing `cli_test`, `webserver_test`
and `tui` test still passes with its golden output unchanged.

### DEBT-51 The CLI wires itself seven times, and `--log` reaches none of them

Severity: medium · Confidence: read

The preamble `os.Getwd → os.UserHomeDir → config.Load → wiring.Locate →
wiring.Deps(ctx, cfg, where, nil)` is written out in
`internal/cli/reviews.go:59` `runReviewsCommand`, `internal/cli/branch.go:63`
`runBranchCommand`, `pr.go:82` `runPRCommand`, `internal/cli/status.go:117` `seamsFor`,
`standup.go:75` `runStandupCommand`, `internal/cli/announce.go:80` `runAnnounceCommand`
and `scriptable.go:71` `completeAssignedIssues`, with an eighth variant in
the root's `RunE` (`cli.go:161`). The copies do not agree: `branch`, `pr`,
`standup` and `announce` fail on a `Getwd` error while `reviews` discards it
(`reviews.go:59`). Every subcommand passes `nil` for the request log, so the
`--log` facility the root's help advertises for bug reports (`cli.go:195`)
is unavailable to any scriptable command; and the root's `--dry-run`
(`cli.go:193`) and the write commands' `--dry-run` (`scriptable.go:29`) are
two unrelated flags with different help text, neither persistent, so
`workflow --dry-run pr` is an unknown-flag error.

**What it costs.** A change to how the CLI connects — a new seam, a timeout,
a log — is a seven-place edit, and the seven have already drifted.

**One way to fix it.** One `connect(cmd) (cfg, deps, where, closeLog, err)`
that every subcommand calls; `--dry-run` and `--log` declared once as
persistent root flags, with `writeOptions.addFlags` (`scriptable.go:28`)
keeping only `--yes`.

**Done when.** `wiring.Deps(` is called from one function in `internal/cli`;
`workflow --log FILE status` appends a request line to `FILE`;
`workflow --dry-run pr` is accepted.

### DEBT-52 `status` builds its seams and then goes around them

Severity: low · Confidence: read

`seamsFor` (`internal/cli/status.go:117`) builds the full `deps` bundle and
then ignores `deps.Git.Branch` and `deps.Git.Changes`, constructing a second
`gitrepo.At(proc.Run, where.Root)` and calling `ReadBranch` and `Status`
directly (`internal/cli/status.go:122-131`) — although `GitDeps.Branch` and
`GitDeps.Changes` exist (`internal/tui/deps.go:74`) and `pr`, `announce` and
`branch` all go through them. `standup` does the same (`standup.go:87`),
but there it is forced: `RecentCommits` and `LocalBranches` have no
`GitDeps` equivalent.

**What it costs.** A test that fakes the git seam does not reach `status`;
and `status` skips whatever the seam adds (the non-interactive
`GIT_TERMINAL_PROMPT=0` runner, a future timeout).

**One way to fix it.** `status` reads the branch and the changes through
`deps.Git`; `GitDeps` gains `RecentCommits` and `LocalBranches` so `standup`
can too.

**Done when.** `gitrepo.At(` appears in `internal/cli` only inside the
wiring preamble (or not at all, once DEBT-51 lands).

### DEBT-53 Three ways to write JSON

Severity: low · Confidence: read

`encodeJSON` (`internal/cli/status.go:305`, commented "the one place a
command encodes it") and `encodeReport` (`internal/cli/doctor_json.go:109`)
are the same four lines with different error wording, and `runConfigShow`
uses a third path, `json.MarshalIndent` (`internal/cli/config_cmd.go:288`).

**Done when.** One encoder, called from all three.

## The command line's tests

### DEBT-54 The CLI test harness cannot see which stream a line went to

Severity: medium · Confidence: read

`run` and `runGuided` return `stdout.String() + stderr.String()`
(`internal/cli/cli_test.go:51`). No test in `internal/cli` can tell a
message that moved from stdout to stderr, or the reverse, so the stream
discipline clig.dev asks for — the artifact on stdout, commentary on stderr
— is unenforced. Today the gitignore warning (`config_cmd.go:279`), the
decline notices and `dry run: would …` lines (`scriptable.go:46`, `:61`),
the no-configuration guidance (`:90`) and the web server's banner
(`cli.go:255`) all go to stdout, and the harness would pass either way.

**What it costs.** `workflow config show | jq .` fails on the `# <path>`
header; a script that captures stdout gets prose mixed into its data; and
fixing any of it cannot be pinned by a test until the harness changes.

**One way to fix it.** The harness returns both streams; each command's
tests say which one they expect a line on.

**Done when.** A test asserts `json.Unmarshal(stdout)` succeeds for
`config show`, and another asserts the decline notice is on stderr.

## The terminal interface

### DEBT-55 `review.go` is the Review pane, its state, its vocabulary, its polling, rerun, merge and the merge picker

Severity: medium · Confidence: measured

`internal/tui/review.go` is 701 lines, the largest non-test file in the
repository, holding `reviewState`, the forge vocabulary, the CI polling
chain, the pane's keys and rendering, `rerunChecks`, `canMerge …
mergeReason` and the whole `mergePicker` overlay (`:538-701`). `messaging.go`
and `prcomposer.go` are 525 each. Six test files are also over the 500-line
soft target (`internal/messaging/post_test.go` 738, `internal/tui/
messaging_test.go` 596, `composer_test.go` 566, `world_test.go` 565,
`internal/webserver/pullrequest_test.go` 527, `internal/jira/detail_test.go`
501). None is over the 800 hard ceiling.

**What it costs.** `scripts/check-file-length.sh` warns on every run, so the
warning has stopped meaning anything; and the merge picker cannot be read
or changed without the polling chain in the same window.

**One way to fix it.** Move `canMerge … mergePicker` to `merge.go` — the
same split `finish.go` already made — which is the one honest reason to
raise `internal/tui`'s budget from 38 to 39 (the WHY rewritten and a row
appended to `scripts/package-size-budget-history.md` in the same commit).

**Done when.** `review.go` is under 500 lines; `check-file-length.sh --list`
no longer flags it.

### DEBT-56 Three overlays keep their own "in flight" flag instead of `sendState`

Severity: medium · Confidence: read

`sendState` (`internal/tui/sendstate.go:10`) exists so that "in flight, then
failed with this" is named once, and eleven overlays use it. Three still
carry their own booleans: `branchPicker.sending` and `switchErr`
(`switchtask.go:78`), `finishPreview.finishing` (`finish.go:65`) and
`mergePicker.merging` (`review.go:636`). Those three are also the ones that
skip `pinnedOutcome` (`render.go:457`), and two of them are the two that
close on a refusal and demote it to a one-line notice — `mergeRequested.
apply` (`review.go:597`) and `finished.apply` (`finish.go:139`) — which is
why the interface's promise that "a refused change must never go unseen"
holds in 11 overlays of 13, not 13.

**One way to fix it.** The three adopt `sendState` and `pinnedOutcome`;
the refusal stays in the overlay with its reason.

**Done when.** `grep -nE '(sending|merging|finishing)\s+bool'
internal/tui/*.go` matches only `sendstate.go`.

### DEBT-57 The same overlay shapes, written nine, five and three times

Severity: low · Confidence: read

- Nine "keep the overlay open with the reason" appliers of the same
  `overlay.(T)` / `send.failed` / reassign shape: `branchresult.go:109`,
  `issuewrite.go:194`, `issuelink.go:98`, `preditor.go:162`,
  `prcomposer.go:489`, `hookgen.go:153`, `switchtask.go:228`,
  `comment.go:169`, `internal/tui/messaging.go:502`.
- Five list-picker bodies with identical `up`/`down`/`confirm`/`esc` and a
  `window`-scrolled `rows`: `picker.go:238`, `picker.go:436`,
  `switchtask.go:119`, `checks.go:64`, `internal/tui/run.go:255`.
- Three focus-guarded "re-clamp the shared scroll after a shrinking reload"
  blocks: `commits.go:50`, `reviewqueue.go:52`, plus `commits.go:255`
  `followChange` / `reviewqueue.go:213`.
- Two `onFieldNav` + `*CanComplete` pairs (`scopesuggest.go:17`,
  `prcomposer.go:304`) and two blur-all-then-focus-one switches
  (`composer.go:294`, `prcomposer.go:325`).

The rule of three is met several times over. DEBT-56 and the failure-voice
work in UX.md reduce the first group as a side effect; a generic picker
would remove the second.

**Done when.** One picker type renders the five lists; the appliers share a
helper.

### DEBT-58 One scroll offset for six panes

Severity: medium · Confidence: read

`m.scroll` (`internal/tui/tui.go:51`) is a single offset shared by every
pane, reset on focus (`tui.go:269` `focusOn`). It is the reason for the
focus-guarded re-clamps in DEBT-57, and the reason `pickChange`
(`commits.go:261`) and `pickReview` (`reviewqueue.go:220`) must add
`m.scroll` to a clicked line while `pickIssue` (`internal/tui/detail.go:260`) must not —
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
(`internal/tui/detail.go:50`) documents a last-writer-wins race between two
in-flight reads of the same issue and consciously declines to fix it. Both
are honest about what they are; both are the kind of thing the next
concurrency change trips on.

**Done when.** Polling chains carry a context that the next generation
cancels; the detail read is keyed so a stale answer is dropped.

### DEBT-60 The world tests wait a fixed 400 ms for the model to settle

Severity: medium · Confidence: read

`patience` (`internal/tui/world_test.go:34`) caps draining the model's
commands at 400 ms of wall clock, with a comment explaining that quiescence
detection is not available. Under `-race`, and on a loaded CI runner, a
slow drain reads as a missing message.

**What it costs.** A flake that is not a defect, on the suite the coverage
floor depends on, that gets slower to reproduce the more the frames change.

**One way to fix it.** A harness that knows when the model has no command
outstanding — counting the commands it hands out and the messages that come
back — so the test waits for a count, not a clock.

**Done when.** `patience` is gone and the world tests pass 50 runs under
`-race -count=50`.

## The web

### DEBT-62 One frontend function is 304 lines, and nothing measures a function's length

Severity: medium · Confidence: measured

`web/eslint.config.js:80` sets `complexity`, `max-params`, `max-depth` and
`max-nested-callbacks` but no `max-lines-per-function`. The result:
`ConfigForm` (`web/src/features/settings/SettingsPanel.tsx:24`) is 304
lines, `PullRequestForm` (`web/src/features/review/ReviewPanel.tsx:214`)
124 and `CommitForm` (`web/src/features/branch/CommitForm.tsx:36`) 105. Files
are measured — `scripts/check-file-length.sh` holds `.ts` and `.tsx` to the
500/800 targets, and the longest, `SettingsPanel.tsx`, is 371 lines — but a
function can grow to fill one with nothing to say so.

**One way to fix it.** Add `max-lines-per-function` to eslint at a number
the split `ConfigForm` meets; split `ConfigForm` by fieldset.

**Done when.** `yarn lint` fails a 300-line component.

### DEBT-63 The async-write state machine is written six times

Severity: medium · Confidence: read

`useAsyncAction` (`web/src/features/issues/useAsyncAction.ts:11`) names the
`idle → running → idle | error` machine once, and is used by check-out and
start-work. Five components hand-roll the same machine instead:
`PushButton` (`web/src/features/branch/BranchPanel.tsx:125`), `CommitForm`
(`CommitForm.tsx:58`), `AnnounceControls`
(`web/src/features/messaging/MessagingPanel.tsx:109`), `OpenPullRequest`
(`web/src/features/review/ReviewPanel.tsx:114`) and `ConfigForm`
(`SettingsPanel.tsx:33`). Alongside: `splitList` (`ReviewPanel.tsx:204`),
`trimmedList` (`internal/webserver/pullrequest.go:205`) and an inline third
copy (`SettingsPanel.tsx:203`) all trim a comma-separated list; and
`SettingsPanel.errorMessage` (`SettingsPanel.tsx:331`) is now a one-line
wrapper over `apiErrorMessage` that stays exported only so its own test can
repeat four of the five assertions in `web/src/api/apiError.test.ts`.

**What it costs.** A success state (which four of the seven writes lack —
see UX.md) has to be added in six places; the six already differ in how
they clear an error.

**One way to fix it.** `useAsyncAction` grows a `done` state and a
message; the five adopt it; `splitList` moves to `web/src/api` and the
server's `trimmedList` stays (they are on different sides of the wire).

**Done when.** `set('running')` appears in one file under `web/src`.

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
is three specs (`web/e2e/a11y.spec.ts`, `smoke.spec.ts`, `theme.spec.ts`),
outside `task check` (CI's `e2e` job and `yarn test:e2e` run it), with no
`workflow --web` backend — acknowledged at `.github/workflows/ci.yml:101`
("Hermetic — no backend") — so no test drives any of the seven write
actions end to end.

**One way to fix it.** The e2e job starts `workflow --web` against a fixture
repository so one spec can commit, push and open a pull request; the web's
branch floor becomes per-condition.

**Done when.** One Playwright spec performs a write against a running
server, and the web's coverage floor measures each condition both ways.

### DEBT-67 The event stream lives outside both generators, and a bad frame vanishes silently

Severity: medium · Confidence: read

`GET /api/events` is registered by hand (`internal/webserver/webserver.go:117`
— "a streaming response the strict, one-response-object interface cannot
express") and the browser consumes it with a raw `new EventSource`
(`web/src/api/snapshot.ts:39`); the generated `streamEvents`
(`web/src/api/generated/sdk.gen.ts:231`) is never called. The only thing
keeping the payload honest is the runtime `zSnapshot.safeParse`
(`snapshot.ts:55`), and a frame that fails it is dropped (`snapshot.ts:52`)
with the last good snapshot left on screen and no signal to the user. A
`Snapshot` field added in Go without regenerating the client therefore
makes every frame vanish. Beside it, `resolveJQL`
(`internal/webserver/handlers.go:211`) falls through to `views[0]` for an
unknown view name, so a typo in `?view=` silently returns the default view.

**One way to fix it.** A parse failure sets the stream status to *stale*
with the reason; an unknown view answers `not_found`; and the SSE frame
shape is asserted by a test that decodes a server-produced frame with the
client's schema.

**Done when.** A schema-mismatched frame shows in `StreamStatus`; `GET
/api/issues?view=nope` is 404; a Go test feeds `stream.go`'s frame through
`zSnapshot`.

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

- `CLAUDE.md:34` lists `internal/slack/`; the package was renamed to
  `internal/messaging` in `d229dc1`, and the layout block also omits
  `api/`, `internal/api`, `internal/keychain`, `internal/progress`,
  `internal/buildinfo`, `internal/store`, `internal/web`,
  `internal/webserver`, `internal/tui/frame` and `internal/tui/layout`.
- Residual "Slack" after the rename: the root command's `Short`
  (`internal/cli/cli.go:154`), the help group `groupReviewSlack`
  (`internal/tui/keys.go:80`), `FEATURES.md:30`, `web/index.html:9`.
- `docs/content/docs/usage.md:49` says "the five panes" and `:71` says
  "`1`–`5`"; `internal/tui/panes.go:27` has `paneCount = 6` and the jump
  binding shows `1-6` (`keys.go:198`); the screen mock at `usage.md:24-43`
  omits the Reviews pane. `FEATURES.md:52`'s settled-decision text says
  "Five panes" for the same reason — stale text, not a decision reopened.
- `usage.md` never mentions `--web`, and neither does `README.md`; the only
  user-facing references are the flag's one line in the generated
  reference and the errors page. There is no page naming the web's
  sections, its stream, its theme or which actions it supports.
- The generated reference omits `--version`, `help` and `completion`
  because `cmd/docsgen/main.go` never calls cobra's
  `InitDefaultVersionFlag`/`InitDefaultCompletionCmd`.
- `FEATURES.md:12` is pinned to `817d323`, twenty-odd commits back; FEAT-26
  (`:118`) and FEAT-31 (`:143`) carry inline `Done:` notes instead of the
  removal the standing rule asks for.

**Done when.** `grep -n 'internal/slack' CLAUDE.md` prints nothing; the
reference lists `--version`; `usage.md` has a web page and says six.

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
wiring package imports the terminal interface; the CLI's `webDeps`
(`internal/cli/cli.go:263`) then narrows that bundle for the web server.
The seams are not the terminal's — they are the loop's — and the import is
what rules `wiring` out as a home for shared composition (DEBT-50). It costs
nothing today; it will cost the first time the web needs a seam the
terminal does not declare.

**Done when.** The seam bundle is declared where all three surfaces can
import it without importing each other (YAGNI until DEBT-50 or a new web
seam forces it).

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
  terminal simulates each write and narrates it. The cost is that the
  browser cannot tell it is in that mode — `getHealth.dry_run` exists on
  the wire and is never fetched — which UX.md records as the gap; the
  blanket refusal itself is the intended design.
- **`staleTime: Infinity`** (`web/src/queryClient.ts:8`) with the event
  stream as the sole freshness source. Correct for a pushed snapshot; the
  cost is that the config query never refetches on its own and a stalled
  stream leaves stale data with no refetch to fall back on.
- **The progress spine's per-system hue is color-only** (`internal/tui/spine.go:68`),
  mitigated by the stage name, or its initial when compact (`spine.go:51`).
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
