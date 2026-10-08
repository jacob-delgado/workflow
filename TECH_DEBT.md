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
   location and fix, and a problem found in several areas became one entry
   under [Across the surfaces](#across-the-surfaces).
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

## The command line

### DEBT-162 Guided `config init` and `slack login` fail with a raw ioctl error when stdin is piped

Severity: low · Confidence: read · Size: S

**Where.** `terminalPrompt` (`cmd/workflow/main.go:37`), `runGuidedInit`
(`internal/cli/config_cmd.go:280`), `collectJira`
(`internal/cli/config_cmd.go:322`), `askFor` and `askForLogin`
(`internal/cli/slack_cmd.go`).

**Today.** `Prompt.Line` reads through a `bufio.Reader` over stdin, but
`Prompt.Secret` calls `term.ReadPassword` on the raw descriptor with no
`term.IsTerminal` check. When stdin is a pipe holding a non-blank first
answer, `Line` succeeds and `ReadPassword` then fails with ENOTTY
("inappropriate ioctl for device"). That error is neither `io.EOF` nor
`errNoTerminal`, so `runGuidedInit` and `askFor` return it raw and it falls
into no exit family, though `docs/content/docs/scripting.md` promises exit 2
and the `--template` guidance. Bytes the buffered reader has already read past
the first line are also never seen by the raw-descriptor read. Scripts get the
wrong exit family and a cryptic message.

**Fix.** In `terminalPrompt.Secret`, check `term.IsTerminal` first and answer
`io.EOF`, which both callers already map to the guidance.

**Done when.** `printf 'https://jira.example\n' | workflow config init; echo
$?` prints the `--template` guidance and 2, and the same holds for `workflow
slack login`.

### DEBT-163 `slack login` writes the client ID before the refresh proves it

Severity: low · Confidence: read · Size: S

**Where.** `slackLogin` (`internal/cli/slack_cmd.go:82`), `nameSlackApp`
(`:164`), `keepFirstToken` (`:182`).

**Today.** `slackLogin` calls `nameSlackApp`, which saves `messaging.kind` and
`messaging.client_id` through `config.SaveLayers`, and only then refreshes in
`keepFirstToken`. A refused or unreachable refresh fails the command with the
file already rewritten. Running `slack login` again repairs a first login, but
a failed re-login with a mistyped client ID replaces a working `client_id`
while the old tokens stay, so later refreshes fail until the user logs in
again. Nothing needs this order: `SlackRefresher`
(`internal/wiring/messaging.go:168`) takes the client ID directly.

**Fix.** Refresh with the typed client ID first, and write the configuration
only after `store.Keep` succeeds.

**Done when.** A test whose fake Slack answers `invalid_grant` leaves
`.workflow.json` byte-identical and exits 3.

### DEBT-164 `slack login`'s requests bypass the `--log` request log

Severity: low · Confidence: read · Size: S

**Where.** `keepFirstToken` (`internal/cli/slack_cmd.go:182`), the `slack
login` RunE (`:69`), `onlineDoer` (`internal/cli/doctor_credentials.go:113`).

**Today.** The command opens the `--log` file through `connect`, but
`keepFirstToken` builds its transport with a bare `onlineDoer(cfg)`, so
neither the refresh nor `auth.test` is recorded. `config init` and `doctor`
wrap theirs with `RequestLog.Wrap`, and the root flag promises an outline of
each request. A bug report for a failed login has none.

**Fix.** Pass the connection's request log into `slackLogin` and wrap the
transport with `Wrap("slack", …)`.

**Done when.** A test running `workflow --log F slack login` against a fake
Slack finds the refresh and `auth.test` outlines in F.

### DEBT-165 Text output ignores write errors; only `--json` fails on a closed or full stdout

Severity: low · Confidence: read · Size: M

**Where.** `renderStatusLine` (`internal/cli/status.go:463`), `renderReviews`
(`internal/cli/reviews.go:110`), `field` (`internal/cli/doctor.go:356`),
`writeRow` (`internal/cli/repositories.go:125`), `runSummary`
(`internal/cli/summary.go:152`), `listDataFiles`
(`internal/cli/dbclean_cmd.go:135`); `encodeJSON`
(`internal/cli/scriptable.go:162`).

**Today.** `encodeJSON` returns write failures, and `output_refused_test.go`
pins that for `doctor --json`. Every prose artifact is written with
`fmt.Fprint*` and its error dropped (errcheck excludes them, reasoning that
the failed channel cannot report itself, which does not hold for stdout when
stderr and the exit status remain). `workflow status > /dev/full` exits 0 with
nothing written, while the `--json` path exits 1, so a script cannot trust
exit 0.

**Fix.** Render each command's text into a `strings.Builder` and return the
error of one final write, as `encodeJSON` does.

**Done when.** A test running `status`, `reviews` and `doctor` without
`--json` into the refusing writer expects `errOutputClosed` and exit 1.

### DEBT-166 `status DIR...` labels by base name, so two repositories of one name look alike

Severity: low · Confidence: read · Size: S

**Where.** `repoLabel` (`internal/cli/status.go:243`), `statusesOf`,
`unreadDirectories`, `statusesJSON` (`:503`), `repoStatus` (`:491`).

**Today.** `repoLabel` returns `filepath.Base(dir)`, and the line, the joined
errors, the stderr notes and the JSON's `repository` all use it. `workflow
status ~/work/api ~/oss/api` prints two rows labeled `api`, and "api: not a
git repository" does not say which. The JSON keeps argument order but carries
no directory.

**Fix.** Label with the path as given (home-relative, as `repositories` does)
when base names collide, and add a `dir` field to the JSON.

**Done when.** A test over two directories with one base name shows distinct
labels and JSON entries with their directories.

### DEBT-167 `reviews --json` invents its own shape and drops `opened_at`

Severity: low · Confidence: read · Size: S · Breaking

**Where.** `reviewReport` and `renderReviewsJSON`
(`internal/cli/reviews.go:151`, `:163`), `ReviewRequest`
(`internal/api/models.gen.go:1920`), `docs/content/docs/scripting.md:162`.

**Today.** `repositories --json` and `summary --json` print the web API's
objects on purpose. `reviews --json` prints `reviewReport` instead, whose
`age` is a humanized string ("3d") in place of the API's RFC 3339 `opened_at`,
with a differently typed `ci`. Scripts cannot compute an exact age, and the
same queue has two JSON contracts to document.

**Fix.** Encode `[]api.ReviewRequest` through the web's mapping, keep `age`
for the prose line only, and update scripting.md.

**Done when.** `workflow reviews --json | jq '.[0].opened_at'` returns an RFC
3339 time and the shape matches `GET /api/reviews` items.

### DEBT-168 `doctor --json` builds credential results by scraping the prose report

Severity: low · Confidence: read · Size: M

**Where.** `captureCheck` (`internal/cli/doctor_json.go:183`), the credential
checks in `internal/cli/doctor_credentials.go` (`checkMessaging`,
`credentialMissing`, `credentialUnchecked`).

**Today.** Each credential check writes a fixed `"  %-10s ..."` prose line;
`captureCheck` runs it into a buffer and makes the JSON `detail` by trimming
the service name off the front. Labels and keys agree today only by convention
(the messaging label and key are two separate `strings.ToLower` calls), so
renaming a label or adding a line silently corrupts `detail`. The reuse exists
so both reports mask alike, which one shared model would keep.

**Fix.** Have each check return a `credentialLine{Service, Status, Detail}`,
formatted by the prose report and encoded by the JSON one.

**Done when.** `doctor_json.go` holds no `strings.TrimPrefix`, and a test
changing a check's label leaves `doctor --json` unchanged.

### DEBT-169 `doctor`'s prose and JSON gather repository facts separately, and the JSON loses why

Severity: low · Confidence: read · Size: S

**Where.** `reportRepository` (`internal/cli/doctor.go:157`),
`noRepositoryReason` (`:185`), `repositoryFactsFor`
(`internal/cli/doctor_json.go:111`).

**Today.** Both call `os.Getwd` and `gitrepo.Describe` and map the result
themselves. Prose says why a repository is absent (git missing, not a work
tree, a working directory that cannot be read); JSON sets `inside_work_tree:
false` with no reason. Tooling and config facts were unified so the two
reports cannot disagree; this section was not.

**Fix.** One `repositoryFacts` gatherer with a `problem` field, rendered by
both reports.

**Done when.** `doctor --json` with git off PATH shows `repository.problem`
naming git, and `reportRepository` renders from the same struct.

### DEBT-170 `status.go` is past the 500-line soft target with several concerns

Severity: low · Confidence: measured · Size: M

**Where.** `internal/cli/status.go` (569 lines): `newStatusCmd` (`:48`),
`statusAcross` and `statusesOf` (`:111`, `:184`), `gather` (`:304`),
`serviceNotes` (`:350`), `renderStatusLine` (`:442`), `statusReport` and
`repoStatus` (`:467`, `:491`), the glyph and word maps (`:536`).

**Today.** One file holds the command, the fan-out over directories, fact
gathering, the services' note wording, the line renderer, two JSON shapes and
the glyph maps: several reasons to change. `internal/cli` is at its budget
(20/20).

**Fix.** Split by concern (`status_gather.go`, `status_render.go`) with a
budget row, or under the cohesive budget DEBT-238 proposes.

**Done when.** `scripts/check-file-length.sh --list` no longer flags
`internal/cli/status.go`.

### DEBT-171 Several command functions run 40 to 50 lines

Severity: low · Confidence: read · Size: M

**Where.** `runAnnounce` (`internal/cli/announce.go:155`, 47 lines),
`runGuidedInit` (`internal/cli/config_cmd.go:255`, 48), `NewRootCmdOver`
(`internal/cli/cli.go:183`, 48).

**Today.** Each does several things: `runAnnounce` checks the transport,
composes, handles repeats, previews, tags, confirms, delivers and reports.
They pass `funlen` only because its limit (60 lines, 40 statements in
`.golangci.yml`) is far above CLAUDE.md's ~25-line aim.

**Fix.** Extract named steps such as `previewAnnouncement`,
`deliverAnnouncement`, `guidedAnswers`, `writeGuided` and `rootFlags`.

**Done when.** Each of the three is at most about 25 lines and the existing
tests pass.

### DEBT-172 `credentialOutcome` borrows the exit-status type as a matcher

Severity: low · Confidence: read · Size: S

**Where.** `credentialOutcome` (`internal/cli/doctor_credentials.go:349`),
`exitFamily` and `holds` (`internal/cli/scriptable.go:338`).

**Today.** It builds two `exitFamily` values with no status only to call
`holds()` as "is this error any of these". The code reads as if an exit status
were chosen there, and couples doctor's verdict to the exit table's internals.

**Fix.** Extract `isAny(err error, targets ...error) bool`, used by `holds`
and `credentialOutcome`.

**Done when.** `credentialOutcome` no longer constructs an `exitFamily`.

### DEBT-173 An avoidable `//nolint:nilerr` in `credentialFacts`

Severity: low · Confidence: read · Size: S

**Where.** `credentialFacts` (`internal/cli/doctor_json.go:161`),
`runDoctorJSON` (`:82`), `runDoctor` (`internal/cli/doctor.go:147`).

**Today.** `credentialFacts` checks `run.loadErr` and returns nil, which
`nilerr` flags, though `runDoctorJSON` already reports the load error.
`runDoctor` shows the shape that needs no suppression: it returns early on a
load error.

**Fix.** Call `credentialFacts` only in `runDoctorJSON`'s no-load-error branch
and drop the check and the nolint.

**Done when.** The nolint inventory no longer lists `doctor_json.go:161`, and
the doctor JSON tests pass.

### DEBT-174 `db-clean`, `config init`, `config show` and `slack login` show no examples

Severity: low · Confidence: read · Size: S

**Where.** `internal/cli/dbclean_cmd.go` (the command at `:29`),
`internal/cli/config_cmd.go:66`, `:120`, `internal/cli/slack_cmd.go:54`;
`TestScriptableCommandsLeadTheirHelpWithExamples`
(`internal/cli/help_examples_test.go:12`).

**Today.** clig.dev asks for examples in help, and nine commands carry them.
`db-clean` (a removing write with `--yes` and `--all`) and `config show` (made
to pipe into jq) are scriptable and have none, nor do the two interactive
commands, so their `--help` and reference pages show no worked invocation.

**Fix.** Add `examples(...)` to each and have the test walk every leaf command
of the root.

**Done when.** The help test iterates every leaf command and passes.

### DEBT-175 Pay down TRADE-18: commands take their working directory, home and environment from the caller

Severity: low · Confidence: read · Size: L

**Where.** `loadFromEnvironment` and `configHome` (`internal/cli/cli.go:402`,
`:418`), `inputFor` (`cli.go:251`, `NO_COLOR`), `whereInit`
(`internal/cli/config_cmd.go:141`), `connectLeniently`
(`internal/cli/connect.go:62`), `reportRepository`
(`internal/cli/doctor.go:157`), `repositoryFactsFor`
(`internal/cli/doctor_json.go:111`), `completeAssignedIssues`
(`internal/cli/scriptable.go:177`), `cleanLocalData`
(`internal/cli/dbclean_cmd.go:185`); `run` (`internal/cli/cli_test.go:24`),
`internal/cli/removed_workdir_test.go`.

**Today.** `Execute` takes arguments and streams but no environment, so
commands call `os.Getwd`, `os.UserHomeDir`, `os.Getenv` and `store.DefaultDir`
themselves; six `os.Getwd` calls word the same failure. TRADE-18 keeps six of
those Getwd arms reachable only on Linux. The same habit makes the cli tests
change directory and set the environment, so they cannot run in parallel:
`cli_test.go` says so, and `internal/cli` is the slowest package in `task
check` (about 38 s under `-race`). CLAUDE.md's D principle cites
`config.Load(workDir, homeDir)` as the pattern, which the CLI does not follow.

**Fix.** Add an environment value (working directory as `func() (string,
error)`, home, `Getenv`, state directory) to what `Execute` and
`NewRootCmdOver` take, built from the OS in `cmd/workflow/main.go`, and read
it through one `workingDir(cmd)` helper that wraps the failure once. Rewrite
`removed_workdir_test.go` to inject a failing working directory, in parallel,
with no `t.Chdir` and no macOS skip. Shrink TRADE-18 to `closeRequestLog`
alone, cited by name, and remove the six TRADE-18 site comments.

**Done when.** `rg 'os.Getwd' internal/cli` finds only the production default,
the cli tests call `t.Parallel()`, `task cover:branch` on macOS lists none of
the six arms, and `go test ./internal/cli` wall time drops.

## The terminal interface

### DEBT-176 An announcement can be posted twice when a preview is open as a queued post fires

Severity: medium · Confidence: read · Size: S

**Where.** `Model.canPost` (`internal/tui/messaging.go:245`), `postQueued`
(`:347`), `messagingPreview.handleKey`
(`internal/tui/messagingpreview.go:95`), `Model.sendToMessaging` (`:191`),
`messagingPosted.apply` (`:227`).

**Today.** `canPost` refuses while a post is sending but not while one waits
for CI, so the user can open a new preview over a queued post. When CI passes,
`postQueued` sends the queued post while that preview stays open; its
`handleKey` checks only its own `send.sending`, so enter posts the same
announcement again, and `loop.Deliver` does not deduplicate. When the queued
post answers, `messagingPosted` (which carries no opened count) closes
whichever preview is open on success, dropping its edits, or pins the queued
post's error into it on failure. The team channel reads the announcement
twice.

**Fix.** Carry the opened count in `messagingPosted` and act on the overlay
only through `beneath[messagingPreview](m, opened)`, and have the preview
refuse to post while `m.messaging.send.sending` is set.

**Done when.** A test queues a post with `w`, opens a second preview, lets CI
pass and presses enter: the world sees one post, and the preview stays open
until esc.

### DEBT-177 The success notice names the default channel, not the one posted to

Severity: medium · Confidence: read · Size: S

**Where.** `messagingPosted.apply` (`internal/tui/messagingpreview.go:240`),
`sendToMessaging` (`:191`), `summaryPosted`
(`internal/tui/summarypost.go:177`).

**Today.** The notice reads "announced to " plus `cfg.Messaging.Target()`, the
default channel, but a post can go to a channel cycled in the preview or held
with a queued post, and `messagingPosted` carries no destination. The dry-run
and queued notices use the destination; `summaryPosted` carries `to` and gets
it right. After choosing `#team-b`, the footer says the team was told in
`#dev-workflow`.

**Fix.** Carry the destination in `messagingPosted`, set from
`p.destination()` or the pending channel, and use it in the notice.

**Done when.** `TestTheChannelCanBeChangedBeforePosting` also asserts the
screen reads "announced to " and the chosen channel.

### DEBT-178 Two forms draw their send failure on one clipped line, beside a shared helper that wraps it

Severity: medium · Confidence: read · Size: S

**Where.** `pinnedOutcome` and `failureLine` (`internal/tui/failure.go:510`,
`:473`), `issueWrite.outcome` (`internal/tui/issuewrite.go:127`, with a
`//nolint:mnd` at `:120`), `branchLinker.outcome`
(`internal/tui/branchlink.go:107`); `TestAnOverlayShowsAFailureFully`
(`internal/tui/overlay_outcome_test.go`).

**Today.** Nine overlays draw a refusal under their title through
`pinnedOutcome`, which wraps it with `failureBlock`. The assign, log-work and
link-branch forms instead append the one-row `failureLine` at the bottom,
which `frame.fit` truncates at the box width, so a long Jira or forge refusal
cannot be read. The overlay-outcome test's table leaves out these forms, and
the nolint exists only to size this hand-made layout.

**Fix.** Use `pinnedOutcome` in both, adding the form's own problem as a line
after it, add the forms to the outcome test, and drop the nolint.

**Done when.** A test with a long, multi-line refusal on the assign form shows
the whole reason, and the nolint at `issuewrite.go:120` is gone.

### DEBT-179 Footer key lists and key handlers check the same conditions separately, and disagree

Severity: low · Confidence: read · Size: M

**Where.** `branchKeys`, `canSwitchTask`, `handleBranchKey` and `branchOffer`
(`internal/tui/branch.go:189`, `:216`, `:264`, `:271`), `commitsKeys` and
`handleCommitsKey` (`internal/tui/commits.go:196`, `:247`),
`outsideRepository` (`branch.go:126`).

**Today.** Each pane works out its live keys twice, once for the footer and
once to dispatch. Outside a repository the footer offers nothing, yet `s`
still opens the branch picker (its check reads only the seams, which wiring
always sets) and `h` still runs `lefthook run pre-commit`. The test for that
case checks only the footer. Each new verb is added in two or three places.

**Fix.** One offer list per pane (binding, `can()`, action), from which the
footer filters and the handler dispatches, as `branchOffer` already half does;
apply it to Branch, Commits and Issues.

**Done when.** A test outside a repository presses `s` on Branch and `h` on
Commits and sees no overlay and no hook run.

### DEBT-180 Blocking git, SQLite and file reads run inside `Update`

Severity: low · Confidence: read · Size: M

**Where.** `openCommitComposer` (`internal/tui/composer.go:99`, through
`withScopeSuggestions` and `RecentSubjects`), `startingScope` (`:138`,
`Store.LastScope`), the commit's done callback (`:393`, `loop.RememberScope`),
`issuesLoaded.apply` and `cacheIssues` (`internal/tui/issues.go:33`, `:49`),
`proposePullRequest` (`internal/tui/prcomposer.go:123`, `RemoteBranches` at
`:141` and `Templates` at `:143`).

**Today.** Opening the commit composer runs `git log` and a SQLite read on the
key press; a finished commit and every issue page write to SQLite from
`Update`; opening the pull request composer runs `git for-each-ref` and reads
template files. The reviewers and title-issue reads beside them are commands,
and `Init`'s comment says loads run in a command so a slow service never
freezes the screen. With the store's 5 s busy timeout, a lock held by `--web`
or another session freezes the interface on `c` for up to 5 s. (DEBT-311 is
about the store's error-less write seams.)

**Fix.** Open both composers at once and fill their suggestions, scope and
template from commands that answer with the opened count, as `reviewersRead`
does; make the issue-cache and scope writes commands.

**Done when.** A test whose `RecentSubjects` and `Templates` fakes block still
draws each composer at once, and no Store or Git seam is called directly in an
`apply` or a `handleKey`.

### DEBT-181 A stale go-to-directory answer acts on a prompt reopened since

Severity: low · Confidence: read · Size: S

**Where.** `dirPrompt.handleKey` (`internal/tui/reposwitch.go:372`),
`dirLooked.apply` (`:416`), `dirCompleted.apply` (`:483`).

**Today.** esc closes the prompt while a path is being checked, and the
comment expects the answer to find the prompt gone. `dirLooked.apply` only
checks that some `dirPrompt` is open, so open, type A, enter, esc, reopen,
type B lands A's answer in the new prompt: A's refusal pinned under B, or a
switch confirmation for a directory the user abandoned. `dirCompleted.apply`
guards the same case by comparing the typed value.

**Fix.** Carry the typed text or an opened count in `dirLooked` and drop an
answer that no longer matches.

**Done when.** A test that parks the first look, escapes, reopens and types
another path, then releases the first look, still shows "Go to a directory"
and no switch confirmation.

### DEBT-182 Tab completion writes the sanitized directory name into the path

Severity: low · Confidence: read · Size: S

**Where.** `dirPrompt.complete` (`internal/tui/reposwitch.go:513`),
`dirCompleted.apply` (`:483`), `workdirs.List`
(`internal/workdirs/workdirs.go:52`).

**Today.** Completion passes every entry name through `sanitize.Line` and then
puts those names into the input and the shared prefix. A directory whose name
holds a control or bidi character completes to a path that does not exist, and
enter fails with "not there" for no visible reason.

**Fix.** Keep the raw names for the value and the prefix, and sanitize only
what is drawn, including the input's own rendering.

**Done when.** A test listing a subdirectory with a sanitized rune completes
and looks up the exact on-disk path.

### DEBT-183 A dry-run commit loses its message; a dry-run comment keeps it

Severity: low · Confidence: read · Size: S

**Where.** `commitComposer.commit` (`internal/tui/composer.go:382`), the live
path (`:386`), esc (`:261`), `commentPreview.post`
(`internal/tui/comment.go:113`).

**Today.** Under `--dry-run`, enter closes the composer with "dry run: would
commit …" without saving the draft, so `c` reopens it empty (or with an older
draft). The live path and esc keep the draft, and a dry-run comment keeps its
own.

**Fix.** Set `m.draft = c.draft()` before the dry-run notice.

**Done when.** A dry-run test composes, presses enter, presses `c` and finds
the subject still there.

### DEBT-184 Forge issue keys lose their `#` in the assign and link messages

Severity: low · Confidence: read · Size: S

**Where.** `assignAction.done` and `.would` (`internal/tui/issuewrite.go:33`),
`issueWrite.view` (`:121`), `branchLinker.link`
(`internal/tui/branchlink.go:247`), `branchLinked.said` (`:318`), `shownKey`
(`internal/tui/issues.go:295`).

**Today.** `shownKey` writes a forge number as `#42`, and the unlink messages
use it, but the assign form's title, its "assigned 42 to …", the dry-run link
notice and "linked branch to 42" write `string(key)`, so one issue reads `#42`
and then `42`.

**Fix.** Use `shownKey` wherever a key is shown, and `string(key)` only for
what a seam is sent.

**Done when.** A test assigning a forge issue and linking a branch to `#42`
reads `#42` in each notice.

### DEBT-185 Help pads keys with `%-10s`, so a ten-character key runs into its description

Severity: low · Confidence: read · Size: S

**Where.** `helpColumn` (`internal/tui/help.go:216`), `composerKeys`
(`internal/tui/keys.go:349`), `asciiGlyphs.sideways`
(`internal/tui/glyphs.go:51`).

**Today.** With `ui.ascii` the type-cycle key shows as `left/right`, exactly
ten characters, so the help reads "left/rightchange type". Any `ui.keys`
override of ten or more characters does the same, and `%-10s` counts runes,
not cells.

**Fix.** Size the column from the widest shown key with `lipgloss.Width` and
always leave one space.

**Done when.** A test with `ui.ascii` on shows "left/right" followed by a
space and "change type".

### DEBT-186 `editor.Parse` cuts a draft at the scissors text even mid-line

Severity: low · Confidence: read · Size: S

**Where.** `Parse` and `Scissors` (`internal/editor/editor.go:127`).

**Today.** `strings.Cut(raw, Scissors)` ends the text at any occurrence,
though the doc says "from this line down" and git honors the scissors only as
a whole line. A body quoting git's scissors line mid-line is silently cut.

**Fix.** Stop at the first line equal to `Scissors` after trimming trailing
spaces.

**Done when.** An editor test with the scissors mid-line before the real one
keeps the first line whole.

### DEBT-187 Every overlay repeats the close, step and scroll key code, and the job log skips the page keys

Severity: low · Confidence: read · Size: M

**Where.** `checkList.handleKey` and `jobLogView.handleKey`
(`internal/tui/checks.go:119`, `:388`), `helpOverlay.handleKey`
(`internal/tui/help.go:86`), `handleFormKey` (`internal/tui/fields.go:192`),
and the other overlays; `overlayKeys` (`internal/tui/overlay.go:63`).

**Today.** Fourteen overlays match the cursor keys by hand and thirty end with
`m.overlay = c`. The help overlay pages by half a page and is scrollable and
steppable; the job log, the overlay most likely to be long, handles only close
and the cursor keys, so pgup and pgdn do nothing there and its footer never
offers them.

**Fix.** Make `jobLogView` scrollable and steppable like the help overlay;
extract a helper for the shared close and cursor arms only where it does not
need per-overlay hooks.

**Done when.** A test pages a long job log with pgdn and sees it move, and the
job log's footer offers the scroll keys.

### DEBT-188 Every overlay stores its own copy of the marks and styles, and `lookAt` is bypassed

Severity: low · Confidence: read · Size: M

**Where.** `overlay.view` (`internal/tui/overlay.go:27`), `lookAt`
(`internal/tui/taskactions.go:231`), `previewPush` and `previewRebase`
(`internal/tui/branch.go:233`, `:245`), `previewRerun`
(`internal/tui/checks.go:201`), `askToRemove`
(`internal/tui/localdata.go:213`).

**Today.** `view(width, rows)` gets no rendering context, so 25 overlay types
carry the marks and styles, filled in 31 places with `marks: m.marks, styles:
m.styles`. `lookAt` exists to fill a last look's, but four sites build theirs
by hand.

**Fix.** Pass a small render context to `view` and drop the fields; until
then, move `lookAt` to `overlay.go` and use it at every last-look site.

**Done when.** `grep -c 'marks: m.marks, styles: m.styles' internal/tui/*.go`
is 0, or limited to overlays that are not last looks.

### DEBT-189 `keyContexts` mirrors dispatch by hand, so the key check can miss a conflict

Severity: low · Confidence: read · Size: M

**Where.** `keyContexts` and `keyContext.covers`
(`internal/tui/keycheck.go:153`, `:135`), the Repositories context (`:177`).

**Today.** `CheckKeys` decides conflicts from help groups plus a hand-kept
`alsoLive` list of actions a pane handles outside its group. Nothing derives
the list from the handlers, so a pane that starts answering another group's
key lets a conflicting `ui.keys` override pass. The Repositories context
already lists up and down, which its groups cover.

**Fix.** Build the contexts from each pane's offer list (DEBT-179); until then
drop the redundant entries and add a test that every binding a handler matches
is covered by its context.

**Done when.** A test fails when a handler answers an action missing from its
key context.

### DEBT-190 `Model` is a god type: 431 methods, with every pane's behavior on one receiver

Severity: low · Confidence: measured · Size: L

**Where.** `Model` (`internal/tui/tui.go:36`), `settled`
(`internal/tui/taskoffers.go:22`), `branchLoaded.apply`
(`internal/tui/branch.go:39`), `taskIssueLine` and `branchAndPull`
(`internal/tui/tasks.go:415`, `:428`).

**Today.** `grep -c '^func (m Model)' internal/tui/*.go` sums to 431 (tasks.go
29, detail.go 28, taskactions.go 23, review.go and commits.go 22 each), over
about 35 fields. Pane states such as `tasksState` and `reviewQueueState` are
mostly plain data whose behavior lives on `Model`, so any pane reads and
writes any other's fields, and core plumbing (`settled`, which every Update
route ends in) sits in a feature file. A reader cannot tell what a pane
depends on. Value receivers and one package are deliberate and stay.

**Fix.** Pane by pane, move behavior that reads only one pane's state onto its
state type, with the few cross-pane facts as arguments, leaving `Model` to
route and compose; move `settled` beside `Update`. Do not split the package.

**Done when.** The method count falls well below 431 (for example under 250),
and the Tasks, Reviews and Summary renderers have state-type receivers.

### DEBT-191 Fifteen files in `internal/tui` are past the 500-line soft target, one near 800

Severity: low · Confidence: measured · Size: M

**Where.** `taskactions.go` (738), `tasks.go` (695), `setupform.go` (665),
`people.go` (576), `reposwitch.go` (535), `branch.go` (520), `failure.go`
(519), `render.go` (510), `commits.go` (510), `settingsfields.go` (508),
`tagging.go` (506), `review.go` (503), `picker.go` (502), with `detail.go`
(494) and `keys.go` (491) close; all under `internal/tui/`.

**Today.** Most carry more than one concern: `branch.go` holds the whole
`branchCreator` overlay; `failure.go` a 220-line error-wording catalog between
`sendState` and the renderers; `detail.go` the Issues list's search, paging
and footer and the shared `age()`; `picker.go` an unrelated `fixupPicker`;
`reposwitch.go` the `dirPrompt`; `taskactions.go` the `taskLine` overlay and
issue tracking. `taskactions.go` is 62 lines from the 800-line ceiling that
fails the gate, so the next Tasks feature forces an unplanned split. The
package is at its budget (64/64).

**Fix.** Split file per concern inside the package: `branchcreator.go`,
`errorwords.go`, `issuesearch.go`, `fixup.go`, `dirprompt.go`, `taskline.go`,
`tasktrack.go`, `footer.go`, `setupsteps.go`, under the cohesive budget
DEBT-238 proposes.

**Done when.** `scripts/check-file-length.sh --list` flags none of these files.

### DEBT-192 The frame keeps two copies of the border glyphs, and the ASCII focus corners disagree

Severity: low · Confidence: read · Size: S

**Where.** `glyphs` (`internal/tui/frame/frame.go:54`, with a
`//nolint:exhaustive` at `:55`), `railSide`, `railHorizontal`, `railCorners`
(`:227`, `:241`, `:305`), `top` and `ruleLine` (`:144`, `:296`).

**Today.** `Render` reads glyphs from a map that leaves Light out (hence the
nolint); `Rail` spells the same characters again as switches on two bools;
`top` and `ruleLine` are one function. In ASCII a focused `Render` box has `#`
corners and a focused rail pane `+` corners with `#` sides.

**Fix.** One `borders` value per style (Light in the map, nolint gone), the
rail functions taking a `borders` value, and `top` calling `ruleLine`.

**Done when.** The nolint is gone and a frame test asserts a focused ASCII
rail and a focused `Render` share corners.

### DEBT-193 `layout`'s exported functions take four interchangeable ints

Severity: low · Confidence: read · Size: S

**Where.** `Compute`, `ComputeWithNotice`, `NoticeKeepsFocus`
(`internal/tui/layout/layout.go:74`, `:102`, `:212`).

**Today.** Each takes `(width, height, railPanes, focused int)`, so a call
with width and height, or panes and focus, swapped compiles and fails only at
run time.

**Fix.** Take a `Terminal{Width, Height}` and a `Rail{Panes, Focused}`.

**Done when.** The exported functions take named struct values and their call
sites in `internal/tui` use them.

### DEBT-194 The central applier list claims every message but names 55 of 76

Severity: low · Confidence: read · Size: S

**Where.** The assertion block (`internal/tui/overlay.go:106`).

**Today.** It is headed "Every message the interface waits for is an applier."
but lists 55 types; 21 message types with an `apply` method are missing (among
them `announcesLoaded`, `branchLinked`, `dirLooked`, `reviewersRead`,
`tasksLoaded`), each already asserted beside its definition, and five are
asserted in both places.

**Fix.** Delete the central block and keep one `var _ applier = T{}` beside
each definition.

**Done when.** Every type with `apply(Model)` has exactly one assertion beside
it, and `overlay.go` holds no partial list.

### DEBT-195 Stale-answer and data-loss guards in the terminal are never exercised

Severity: low · Confidence: measured · Size: M

**Where.** `summaryAnswered.apply` and `summaryRested.apply`
(`internal/tui/summary.go:58`, `:80`), `runFinished.apply`
(`internal/tui/run.go:163`), `writeInFlight` and `lostOnLeaving`
(`internal/tui/reposwitch.go:253` to `:275`), `handleTaskFilterKey` and
`withoutTaskFilter` (`internal/tui/tasklist.go:391` to `:409`).

**Today.** gobco lists each guard seen only one way: a Summary answer for a
period already left (270 times false, never true), a stopped run reported as
stopped, each warning before a directory switch (an announcement sending, a
task write in flight, a body-only commit draft, a post waiting for CI;
`prDraft.edited` never evaluated), and enter, up, backspace and leaving in the
Tasks filter. A regression in any of these passes the suite.

**Fix.** Black-box tests: step the period while a source is parked, then
release it; stop a run and deliver its killed exit; switch with each kind of
unsaved work and assert the last look names it; type a Tasks filter, then
enter, backspace and leave.

**Done when.** `task cover:branch` no longer lists those lines as seen one way.

### DEBT-196 Pay down TRADE-6: one field ring for the composers, the calendar and the pane switch

Severity: low · Confidence: read · Size: S

**Where.** `commitComposer.onFieldNav` and `scopeCanComplete`
(`internal/tui/scopesuggest.go:18`, `:30`), `prComposer.onFieldNav` and
`baseCanComplete` (`internal/tui/prcomposer.go:380`, `:396`), the calendar's
field keys (`internal/tui/calendar.go:229`), the type cycle
(`internal/tui/composer.go:289`), the pane switch (`internal/tui/tui.go:294`).

**Today.** TRADE-6's trigger has fired: the Summary's calendar binds both
next-field and previous-field and cycles a focus index. The wrap arithmetic
`(index + n - 1) % n` is now written five times, the two `onFieldNav` twins
have diverged (one handles previous-field outside it), and the two
`*CanComplete` checks are one function over different inputs.

**Fix.** Add a small value type, `ring{at, size}` with `next()` and `prev()`,
used by both composers, the calendar, the type cycle and the pane switch;
replace the two checks with one `completesOnTab(textinput.Model) bool`; give
both composers the same `onFieldNav`. Paying this closes TRADE-6: delete the
entry and its two site comments.

**Done when.** No `- 1) %` remains in `internal/tui` outside the helper, the
composer, calendar and pane-switch tests pass unchanged, and TRADE-6 is gone.

### DEBT-197 Pay down TRADE-12: `tui.Run` takes its input from the caller

Severity: low · Confidence: measured · Size: S

**Where.** `Run` (`internal/tui/tui.go:178`), the `RunInterface` type and its
call (`internal/cli/cli.go:177`, `:251`), the stand-in in
`internal/cli/root_test.go:56`.

**Today.** `Run` takes an output writer but not its input, so Bubble Tea reads
`os.Stdin`, and the CLI passes `cmd.OutOrStdout()` but never
`cmd.InOrStdin()`, against Cobra's own idiom. That is the only reason no test
can drive `Run`, and the reason gobco reports `tui.go:182` as never evaluated.

**Fix.** Take `in io.Reader` and pass `tea.WithInput(in)`; have the CLI pass
`cmd.InOrStdin()`. Test a canceled context (the error wraps `context.Canceled`
and starts "running the interface") and a quit key on an empty model. Paying
this closes TRADE-12: delete the entry and its site comment.

**Done when.** gobco no longer lists `tui.go:182` and `task check` is green.

## The web

### DEBT-200 The mockup is an `if VITE_MOCK` branch copied into 26 production modules

Severity: medium · Confidence: read · Size: L

**Where.** `useEventStream` (`web/src/api/snapshot.ts:96`), `loadHealth`
(`web/src/api/health.ts:58`), `Refusal` (`web/src/api/apiError.ts:4`),
`pushBranch` (`web/src/features/branch/pushApi.ts:10`), `readPeople`
(`web/src/features/settings/people/peopleApi.ts:17`), the API modules of every
feature (127 lines by `grep VITE_MOCK`),
`web/src/shell/SectionPanel.test.tsx:31`, `web/vitest.config.ts:36`.

**Today.** Each API function opens with `if (import.meta.env.VITE_MOCK ===
'true') { … await import('@/dev/…') }`, so every new endpoint carries a
second, mock-only body; `health.ts` writes its mock inline; `Refusal` is
thrown only by `src/dev/mockSlack.ts` outside its subclass; `configApi.ts:70`
changes the caching policy under the mock. Tests switch production code into
mock mode with `vi.stubEnv` and import `src/dev` fixtures (21 files) beside
`src/test/fixtures.ts`, while `src/dev` is excluded from coverage. The
fixtures are typed (`mockSnapshot: Snapshot`) and stripped from production
builds, so nothing reaches users; the cost is shotgun surgery and tests that
can pass through the mock arm.

**Fix.** Install the mockup once, at startup: under `VITE_MOCK`, `main.tsx`
dynamically imports a `src/dev/mockServer.ts` that answers the generated
client's routes the way `src/test/fakeApi.ts` does. Delete the per-function
branches, the inline health mock and `Refusal` (the mock answers problem+json
like the server), and move shared fixtures to `src/test`.

**Done when.** `grep -rln VITE_MOCK web/src` outside `main.tsx`, `src/dev` and
tests is empty, and `task web:mockup` still opens every section.

### DEBT-201 Pay down TRADE-10: the server ships the loop's stages

Severity: medium · Confidence: read · Size: M

**Where.** `progress.Stages` (`internal/progress/progress.go:118`),
`onHeadStages` (`web/src/features/issues/WorkStory.tsx:122`),
`web/src/features/issues/WorkStory.stages.test.tsx`, the snapshot builder in
`internal/webserver`, `Snapshot` in `api/openapi.yaml`.

**Today.** The stage rules are written in Go and again in TypeScript, and the
copies have drifted in shape: Go derives five stages (Issue, Branch, Commits,
Review, the service) and the web four, with no Issue stage, which is
TRADE-10's "one surface shows a stage the other does not". Branch is decided
differently (Go reads `OnFeatureBranch`, the web `branch.name !== ''`), and
uncommitted changes make Commits in flight only in Go. The paired cases cover
the review, changes-done and announce rules only. The server already holds
every input.

**Fix.** Add a required `stages` array to `Snapshot` (name, system, state),
run `task gen` and `yarn gen`, and fill it from `progress.Stages` in the
snapshot builder. Have `onHeadStages` read it, keeping only per-issue
presentation in the web. Move the rule cases into webserver snapshot tests,
and keep a web test that renders whatever stages arrive. Paying this closes
TRADE-10: delete the entry and its site comments (`progress.go:14`,
`WorkStory.tsx:37`).

**Done when.** The web derives no stage state, `rg 'reviewReached' web/src`
finds nothing, and `task check` is green.

### DEBT-202 The e2e "reachable and clean" assertions are hand-copied about seventeen times and have drifted

Severity: medium · Confidence: read · Size: M

**Where.** `walkTabOrder`, `sidewaysScrollers`, `pageScrolls` and
`axeViolations` (`web/e2e/tabwalk.ts:139` to `:203`), `heldToTheLayout`
(`web/e2e/keyboard/keyboard.spec.ts:47`), the blocks in
`issues/comments.spec.ts`, `issues/worktree.spec.ts`, `issues/writes.spec.ts`,
`messaging/held.spec.ts`, `review/writes.spec.ts`, `summary/calendar.spec.ts`,
`repositories/picker.spec.ts`, `layout.spec.ts:583` and `:609`.

**Today.** Every surface spec writes its own sideways, page-scroll, missed,
hidden and axe expectations. Seven specs never check the page scroll CLAUDE.md
requires ("must not scroll the page"), with nothing saying why; layout.spec's
pull request form and refused-write tests skip axe and run in one theme;
keyboard.spec has a third form. The layout floor is enforced unevenly per
surface without anyone having decided so, and a new spec copies whichever
block it starts from.

**Fix.** Export `expectReachableAndClean(page, {reaches?, passedBy?})` that
runs all five checks, and use it everywhere.

**Done when.** One definition of the block remains under `web/e2e`, and every
surface spec calls the helper.

### DEBT-204 A permanently closed default stream reads "Reconnecting" forever

Severity: low · Confidence: read · Size: S

**Where.** The error listener in `useEventStream`
(`web/src/api/snapshot.ts:131`), `web/src/api/snapshot.test.tsx:272`,
`StreamStatus` (`web/src/shell/StreamStatus.tsx:15`), `useHealth`
(`web/src/api/health.ts:46`).

**Today.** When the browser closes the default stream for good (a non-stream
answer, such as another process now on the port or a 5xx before the upgrade),
the hook special-cases a closed source only for a named view and otherwise
sets "reconnecting", which a test pins. Nothing reopens it, so the page says
it is reconnecting while dead. A network failure, the common case, still
retries and is correctly "reconnecting".

**Fix.** On a closed source for the default view, set a terminal state whose
reason says to reload, or reopen with backoff by re-running the effect.

**Done when.** After a refused default stream, the status is no longer
"reconnecting" and names a reload, or a new source opens after the backoff.

### DEBT-205 `useAsyncAction.reset` does not cancel an in-flight run

Severity: low · Confidence: read · Size: S

**Where.** `run` and `reset` (`web/src/lib/useAsyncAction.ts:41`, `:60`),
`AskFirst`'s cancel (`web/src/features/tasks/TaskDetail.tsx:288`).

**Today.** `run` sets the result, `done` and calls `onDone` whenever the
action settles; `reset` clears only error and code. Cancelling `AskFirst`
mid-run (its Cancel is not held) returns the hook to `done` when the write
lands, and the teller announces it after the user backed out. The write did
happen, so the announcement is true; the hook's state is not. No unit test
covers the hook.

**Fix.** Hold Cancel while running in `AskFirst`; in the hook, keep a run
counter and have `reset` clear message and result, so a late settlement
updates state only for the current run.

**Done when.** A new `web/src/lib/useAsyncAction.test.ts` covering reset
during a run passes.

### DEBT-214 Seven confirm steps repeat one shape by hand, and have drifted

Severity: low · Confidence: read · Size: M

**Where.** `AskFirst` (`web/src/features/tasks/TaskDetail.tsx:335`),
`RemoveConfirm` (`web/src/features/settings/people/LocalData.tsx:284`),
`ForgetConfirm` (`web/src/features/settings/people/PeopleTable.tsx:305`),
`DiscardConfirm` (`web/src/features/branch/WorkingTree.tsx:411`),
`PushConfirm` (`web/src/features/branch/BranchPanel.tsx:239`), `ConfirmSwitch`
(`web/src/features/repositories/ConfirmSwitch.tsx:25`), `RemoveQuestion`
(`web/src/features/settings/fieldsets/CredentialRemoval.tsx:83`),
`SummaryPost` (`web/src/features/summary/SummaryPost.tsx:71`).

**Today.** Each writes focus-on-mount, `useHoldShortcuts`, a labeled group
with `tabIndex=-1`, the question, a muted cost line, an optional refusal and a
Cancel and primary pair. Only `DiscardConfirm` draws a focus ring;
`PushConfirm` hard-codes its label id while others use `useId`; two use
`aria-label` and the rest `aria-labelledby`; some run the write inside the
step and some hand it back; `ConfirmSwitch` is a section with a heading and
holds its button with `aria-disabled`. A change to the confirm pattern is made
seven times.

**Fix.** Extract a `LastLook` component into `web/src/lib` that owns focus,
the shortcut hold, the label via `useId`, the ring, the refusal alert and the
held buttons, with an optional `run` for the steps that write in place.

**Done when.** `useHoldShortcuts()` appears in features only in overlays that
are not confirm steps, and the existing confirm tests pass unchanged.

### DEBT-215 Ten buttons re-implement `Button`'s `held` with a raw `aria-disabled` and a hand guard

Severity: low · Confidence: read · Size: S

**Where.** `Button` (`web/src/lib/Button.tsx:31`, `:55`), `Unread`
(`web/src/lib/Status.tsx:78`), `web/src/features/tasks/TasksPanel.tsx:215`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:129`,
`web/src/features/review/Checks.tsx:105`,
`web/src/features/repositories/ConfirmSwitch.tsx:57`,
`web/src/features/branch/WorkingTree.tsx:274`, `HookSetup.tsx:45`,
`IssueLink.tsx:116`, `web/src/features/issues/IssuesPanel.tsx:427`,
`CommentComposer.tsx:410`; `heldNotDisabled` (`web/eslint.config.js:49`).

**Today.** `held` sets `aria-disabled` and swallows clicks while a run goes,
and about thirty sites use it; these pass `aria-disabled={running}` and repeat
`if (!running)` in onClick, or rely on a guard further away. Every site is
guarded today, but each new control copies whichever pattern it starts from,
and the lint rule that steers toward `held` only catches `disabled`.

**Fix.** Replace each with `held={…}` and extend `heldNotDisabled` to flag
`aria-disabled` on `Button`.

**Done when.** No `Button` in `web/src` passes `aria-disabled` for a running
state, and `yarn lint` fails on a reintroduced `<Button
aria-disabled={busy}>`.

### DEBT-216 The forge's kind is inferred from the display noun "merge request"

Severity: low · Confidence: read · Size: S

**Where.** `useTrackerName`
(`web/src/features/issues/IssueDetailPanel.tsx:315`), `gitLabNoun`
(`web/src/features/issues/CommentComposer.tsx:384`), `Health` in
`api/openapi.yaml` (`:3660`).

**Today.** Both compare `health.forge_noun`, a word meant for display, with
"merge request" to decide whether the forge is GitLab; anything else is named
GitHub. A wording change or a third forge mislabels the tracker link and the
GitLab quick-action hints.

**Fix.** Add a `forge_kind` enum to `Health`, regenerate, and read it in both
places.

**Done when.** No `'merge request'` literal remains in `web/src` outside tests.

### DEBT-217 Three hand-written "time ago" formatters with different rules

Severity: low · Confidence: read · Size: S

**Where.** `waited` (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:343`),
`sinceWritten` (`web/src/features/issues/IssueDetailPanel.tsx:280`),
`narrowAgo` (`web/src/lib/dates.ts:63`), the duration constants in each and in
`web/src/features/tasks/taskWords.ts:4`.

**Today.** Each redeclares minute, hour and day and words elapsed time its own
way (terminal-style "5m ago" switching to a date after 30 days, Intl "5
minutes ago" switching after 7, narrow Intl), and only `waited` guards the
zero time.

**Fix.** One `ago(elapsed, {style, dateAfter})` in `dates.ts`, exporting the
constants, with the terminal's wording kept as a style.

**Done when.** `const minute = 60_000` appears only in `dates.ts` outside
tests, and `dates.test.tsx` covers the shared formatter.

### DEBT-218 "Every date the web writes is written here" is false; dates are formatted four ways

Severity: low · Confidence: read · Size: S

**Where.** `writtenDate`, `civilNoon` and `locale` (`web/src/lib/dates.ts:1`,
`:82`, `:6`), `waitsUntilWords` (`web/src/features/tasks/taskWords.ts:113`),
`noon` and `written` (`web/src/features/summary/civilDate.ts:19`),
`web/src/features/summary/PeriodPicker.tsx:44`, `counted`
(`web/src/features/summary/SummaryPost.tsx:153`).

**Today.** `taskWords` hand-rolls a local YYYY-MM-DD; `civilDate.ts`
re-implements `civilNoon` and `writtenDay` (TRADE-33 covers its Go twin, not
this TypeScript copy); `PeriodPicker` pads its own date; `SummaryPost`
hard-codes `'en-US'` beside the private locale. `civilNoon`'s `?? 1` fallbacks
do not catch NaN, so `writtenDay('bad')` throws a RangeError.

**Fix.** Have `taskWords` call `writtenDate`, export `civilNoon` and `written`
from `dates.ts` for `civilDate.ts`, export the locale or a count formatter for
`SummaryPost`, and validate in `civilNoon`.

**Done when.** `padStart(2` and `'en-US'` appear in `web/src` only in
`lib/dates.ts`.

### DEBT-219 The stored-theme rules are written three times and only the key is pinned

Severity: low · Confidence: read · Size: S

**Where.** The pre-paint script (`web/index.html:14`), `readStoredChoice`
(`web/src/shell/themeStore.ts:13`), `useApplyTheme`
(`web/src/shell/useApplyTheme.ts:13`), `web/src/shell/theme.test.tsx:76`.

**Today.** Which stored values are valid and how "system" resolves live in all
three; the test only checks that `index.html` reads the same key. A theme
choice added to the store but not to the script resolves wrongly before paint
with no test noticing. `readStoredChoice` is exported only for a test.

**Fix.** Run the pre-paint script in a test against each stored value and
compare with `useApplyTheme`'s result; test the restore path through
`vi.resetModules` and stop exporting `readStoredChoice`.

**Done when.** A test feeds `light`, `dark`, `system` and an unknown value to
both and asserts the same `data-theme`.

### DEBT-220 `IssuesPanel.tsx` is past 500 lines and drills ten props through `ListAndDetail`

Severity: low · Confidence: measured · Size: M

**Where.** `IssueBrowser`, `ListAndDetailProps` and `ListAndDetail`
(`web/src/features/issues/IssuesPanel.tsx:65`, `:141`, `:164`),
`groupBranchesByKey` (`:525`).

**Today.** At 583 lines it holds the browser, narrowing, paging, arrival
focus, rows, scroll-into-view, branch grouping and the row checkout.
`ListAndDetail` takes ten props and reads three. Branch membership is worked
out twice per render.

**Fix.** Move narrowing to `issueNarrowing.ts`, rows to `IssueRows.tsx`,
paging and arrival focus to `IssuePaging.tsx`, and compute branch grouping
once.

**Done when.** The file is not flagged soft and `ListAndDetailProps` has five
fields or fewer.

### DEBT-221 `shell` and the features import each other, and lib reaches up into shell

Severity: low · Confidence: read · Size: M

**Where.** `UiState` (`web/src/shell/uiStore.ts:2`, `:26`), `Status`
(`web/src/lib/Status.tsx:3`), `StateMark` and `EmptyState`
(`web/src/shell/StateMark.tsx:35`, `EmptyState.tsx:6`),
`web/.dependency-cruiser.cjs`.

**Today.** `StateMark` and `EmptyState` are generic primitives, yet they live
in shell, so `lib/Status.tsx` imports upward from it. No dependency-cruiser
rule keeps lib a leaf, so the layering is held by habit. (The type-only
feature imports in `uiStore.ts` keep sections' state across visits on
purpose.)

**Fix.** Move `StateMark` and `EmptyState` to `web/src/lib` with a budget row,
and add a `lib-is-a-leaf` rule (lib imports neither shell nor features).

**Done when.** `yarn lint:deps` passes with the new rule.

### DEBT-222 The production bundle is one 681 kB chunk and the build warns on every run

Severity: low · Confidence: measured · Size: S

**Where.** `build` (`web/vite.config.ts:13`), `SectionPanel`
(`web/src/shell/SectionPanel.tsx:2`), the comment at
`web/src/api/snapshot.ts:94`.

**Today.** Every panel, the zod schemas and the icons land in one entry chunk
(681.64 kB, 200.58 kB gzip), and Vite prints its chunk-size warning in every
`task check`, teaching readers to skip build output. The comment calls the
mock "code-split" where it is constant-folded away.

**Fix.** Lazy-load the section panels with `React.lazy` and a Suspense
fallback, or set a justified `chunkSizeWarningLimit`; correct the comment.

**Done when.** `task web:build` prints no chunk-size warning.

### DEBT-223 Stale and garbled comments in shell and lib

Severity: low · Confidence: read · Size: S

**Where.** `choiceMeta` (`web/src/shell/ThemeToggle.tsx:4`), `sectionMeta`
(`web/src/shell/sections.ts:15`), `WriteFormFrameProps`
(`web/src/lib/WriteForm.tsx:35`), `web/src/api/apiError.ts:14`.

**Today.** ThemeToggle says the map is built "rather than as a module
constant" above a module constant; sections.ts carries a run-on sentence left
from an insert; WriteForm's props are named for a component renamed since;
apiError.ts holds a merged, overlong comment line.

**Fix.** Correct the ThemeToggle sentence, rewrap the other two comments, and
rename the interface `WriteFormProps`.

**Done when.** The four sites read correctly and `grep WriteFormFrameProps
web/src` is empty.

### DEBT-224 Most feature tests run under TanStack's defaults, not the app's query policy

Severity: low · Confidence: read · Size: S

**Where.** `renderWithClient` and `appQueryClient`
(`web/src/test/renderWithClient.tsx:8`, `:17`), `web/src/queryClient.ts:13`.

**Today.** `renderWithClient`, used by 49 test files, builds a client with
only `retry: false`, so those tests run with `staleTime: 0` and refetch on
focus, a policy production never uses (TRADE-4). Six files opt into
`appQueryClient`. A test can pass because of a remount read production would
never make.

**Fix.** Build `renderWithClient`'s client through `appQueryClient()`, with an
explicit opt-out for a test that needs library defaults.

**Done when.** `renderWithClient` uses `appQueryClient()` and the web unit
tests pass.

### DEBT-225 Two `eslint-disable` lines carry no inline reason, and nothing requires one

Severity: low · Confidence: measured · Size: S

**Where.** `web/src/features/keyboard/CommandPalette.tsx:163`,
`web/src/features/keyboard/ModalDialog.tsx:48`, `web/eslint.config.js`.

**Today.** Five of the seven feature disables give their reason after `--`, as
the config's own comment describes the escape hatch; these two put it in the
comment above. No rule requires a reason (unused disables already fail through
`reportUnusedDisableDirectives`).

**Fix.** Move both reasons onto the directive and enable
`@eslint-community/eslint-comments/require-description` (a new dev dependency,
to be approved).

**Done when.** `yarn lint` fails on a fixture disable with no `--` reason.

### DEBT-226 Eleven e2e files copy the whole stream frame to vary a field or two

Severity: low · Confidence: read · Size: M

**Where.** `issuesSnapshot` (`web/e2e/a11y.spec.ts:231`), `pagedSnapshot`
(`web/e2e/layout.spec.ts:420`), `web/e2e/branchlink.spec.ts:24`, `withTasks`
(`web/e2e/tasks.spec.ts:37`), `web/e2e/issues/comments.spec.ts:21`,
`worktree.spec.ts:20`, `writes.spec.ts:20`,
`web/e2e/messaging/held.spec.ts:28`, `tags.spec.ts:27`,
`web/e2e/review/writes.spec.ts:21`, `web/e2e/branch/runs.spec.ts:10`.

**Today.** Each spells every field of the frame. `satisfies Snapshot` catches
a missing field, so drift is not silent, but each new field is an eleven-file
edit, and the noise hides what each test varies. `src/dev/mockSnapshot.ts`
exists and no spec builds from it.

**Fix.** An `e2e/fixtures.ts` with `emptySnapshot()` and
`snapshotWith(overrides)`, typed with `satisfies Snapshot`.

**Done when.** `grep -rn "commit_types: \['feat', 'fix'\]" web/e2e` returns
one line.

### DEBT-227 `a11y.spec.ts` and `layout.spec.ts` each keep `confirmSteps`, `pullDraft` and an axe scan

Severity: low · Confidence: read · Size: S

**Where.** `scan`, `confirmSteps`, `pullDraft`, `sectionNames`
(`web/e2e/a11y.spec.ts:61`, `:171`, `:342`, `:17`), `confirmSteps` and
`pullDraft` (`web/e2e/layout.spec.ts:176`, `:557`), `axeViolations`
(`web/e2e/tabwalk.ts:203`).

**Today.** `scan()` repeats `axeViolations` and its summary line nine times;
`confirmSteps` is written twice and has drifted: only layout's has the
credential removal, so that confirmation is never axe-scanned in either theme.
`pullDraft` is the same in both, and a11y keeps its own section list.

**Fix.** Share `confirmSteps` and `pullDraft` from one fixtures module, use
`axeViolations`, and derive the section list from `cockpit.ts`.

**Done when.** Each is defined once under `web/e2e` and the credential removal
is scanned in both themes.

### DEBT-228 Problem-detail fixtures in the e2e specs are untyped, and some are not RFC 9457 shapes

Severity: low · Confidence: read · Size: S

**Where.** `noFile` (`web/e2e/cockpit.ts:58`), `web/e2e/a11y.spec.ts:48`,
`:492`, `noSuchIssue` (`web/e2e/branchlink.spec.ts:49`), `refused`
(`web/e2e/tasks.spec.ts:81`), `web/e2e/issues/worktree.spec.ts:119`,
`web/e2e/summary/calendar.spec.ts:82`, `web/e2e/layout.spec.ts:615`; `problem`
(`internal/webserver/errors.go:41`).

**Today.** Each spec writes its problem bodies by hand and none is checked
against `Problem`. branchlink anchors `#not_found` where the server writes
`#not-found`, tasks omits `type`, calendar uses `about:blank`, and layout
fulfills a bare 500. The client reads only detail, title and code, so no test
passes wrongly today; the specs exercise shapes the server never sends.

**Fix.** One `problem(code, status, detail)` builder typed `satisfies Problem`
that derives `type` as the server does.

**Done when.** `application/problem+json` appears under `web/e2e` only in the
builder.

### DEBT-229 The repositories fake answers every switch as `~/src/web`, and a test asserts it

Severity: low · Confidence: read · Size: S

**Where.** `opensRepositories` (`web/e2e/repositories/picker.spec.ts:19`), the
worktree test's assert (`:69`), `ConfirmSwitch`
(`web/src/features/repositories/ConfirmSwitch.tsx:34`).

**Today.** The fake echoes the asked directory but always answers `shown:
'~/src/web'`, so switching to `~/src/api-review` asserts "Switched to
~/src/web.", a reply the server never sends. CLAUDE.md's L principle calls a
fake that cuts corners broken.

**Fix.** Map the asked directory to its shown path from the mock list, and
assert "Switched to ~/src/api-review."

**Done when.** The fake has no hard-coded shown path and the test asserts the
api-review message.

### DEBT-230 Three e2e tests would pass if their Act did nothing

Severity: low · Confidence: read · Size: S

**Where.** "a year picked some way back still offers the years since"
(`web/e2e/summary/calendar.spec.ts:102`), "an issue opens at the top of its
pane at 1440 px" (`web/e2e/panes.spec.ts:72`), "switching back to Reviews
within 30 seconds reads nothing again" (`web/e2e/reviews.spec.ts:81`).

**Today.** The year test only asserts 2026 is offered, which it is before the
select runs. The pane test scrolls `article.parentElement` and never checks
the scroll happened, so a markup change makes it vacuous. The reviews test
proves no second read with `waitForLoadState('networkidle')`, which resolves
at once after the first load, so a refetch with `freshFor` at 0 can land after
the assert.

**Fix.** Assert the picked year and an earlier offered year; assert in the
Arrange that the heading is out of view; bound the negative with `expect.poll`
over the read count or a route that fails on a second hit.

**Done when.** Removing each Act makes its test fail, and `web/e2e` holds no
`networkidle`.

### DEBT-231 The section sweeps run nine independent scenarios as one test

Severity: low · Confidence: read · Size: S

**Where.** `web/e2e/layout.spec.ts:19` (`test.slow()` at `:27`), `:97`
(`:103`), `web/e2e/a11y.spec.ts:68`, `:146`, `web/e2e/screens.spec.ts:33`.

**Today.** Each loops over the sections inside one test body, each pass its
own Act and Assert, and needs `test.slow()` for the budget. The first failing
section hides the rest, and CLAUDE.md asks for one Act per test.

**Fix.** Generate one test per section, as the confirm-step tables already do.

**Done when.** No `test.slow()` remains in `web/e2e`, and each section's
layout and axe result is its own test.

### DEBT-232 The hermetic a11y sweep scans sections before their reads settle

Severity: low · Confidence: read · Size: S

**Where.** `settled` (`web/e2e/a11y.spec.ts:41`), the routes (`:77`),
`readings` and `openSection` (`web/e2e/cockpit.ts:109`, `:118`).

**Today.** `settled()` hard-codes which sections read and waits only for the
first "Try again". Settings also reads people, groups and local data, which
the test does not route, so they retry with backoff while axe scans whatever
mix of "Reading…" is on screen. `cockpit.ts` already waits for no Reading
status.

**Fix.** Route every `/api` read in the test and open each section with
`openSection`.

**Done when.** `settled()` is deleted and the sweep uses `openSection`.

### DEBT-233 Nine places click the Sections rail by hand instead of `openSection`

Severity: low · Confidence: read · Size: S

**Where.** `web/e2e/messaging/held.spec.ts:84`, `messaging/tags.spec.ts:129`,
`settings/localdata.spec.ts:54`, `settings/people.spec.ts:66`,
`tasks.spec.ts:254`, `a11y.spec.ts:398`, `:474`,
`summary/calendar.spec.ts:22`, `repositories/picker.spec.ts:28`; `openSection`
(`web/e2e/cockpit.ts:118`).

**Today.** Settle waits are skipped inconsistently, `opensPreview` is the same
in two messaging specs, and calendar.spec's unscoped, non-exact
`getByRole('button', {name: 'Summary'})` would match any button containing the
word. A rail rename touches nine files.

**Fix.** Use `openSection` everywhere, with a variant that does not wait on
reads where a failing read is the point, and share `opensPreview`.

**Done when.** The Sections navigation locator appears only in `cockpit.ts`
and the rail's own layout tests.

### DEBT-234 The keyboard trap walk re-implements the Tab walk with narrower rules

Severity: low · Confidence: read · Size: S

**Where.** `trappedLap` (`web/e2e/keyboard/keyboard.spec.ts:15`), `tabStops`
and `focusedStop` (`web/e2e/tabwalk.ts:18`, `:48`), `ShortcutSheet`
(`web/src/features/keyboard/ShortcutSheet.tsx:53`).

**Today.** It counts stops with a narrower selector and checks in-view against
the window only, while the shared walk clips by scroll ancestors. The shortcut
sheet scrolls inside its dialog, so a control scrolled out of the dialog's box
still counts as in view.

**Fix.** Give `walkTabOrder` a `within` option reusing `tabStops` and
`focusedStop`, and use it for the trap.

**Done when.** keyboard.spec.ts runs no `querySelectorAll` of its own.

### DEBT-235 `layout.spec.ts` and `a11y.spec.ts` are past 500 lines, holding surface tests that have folders

Severity: low · Confidence: measured · Size: M

**Where.** `web/e2e/layout.spec.ts` (633 lines: the issue paging test `:451`,
the 320 px branch test `:531`, the pull request form `:583`, the refused write
`:610`), `web/e2e/a11y.spec.ts` (525: issue list and detail `:291`, the form
`:355`, offers `:377`, working tree `:464`, refused write `:506`).

**Today.** Beside the cross-section sweeps, each holds hermetic tests of one
surface whose folder (`issues/`, `review/`, `branch/`) already exists, so
coverage of a surface is split between places.

**Fix.** Move the surface tests into their folders, leaving the two files as
the sweeps.

**Done when.** `scripts/check-file-length.sh --list` shows both under 500.

### DEBT-236 The `web/e2e` root is at its budget with single-surface specs left outside their folders

Severity: low · Confidence: measured · Size: S

**Where.** `web/e2e/branchlink.spec.ts`, `reviews.spec.ts`, `tasks.spec.ts`,
`streams` (`web/e2e/tabwalk.ts:164`).

**Today.** The root sits at 12/12. `branchlink.spec.ts` belongs in the
existing `branch/`; `reviews.spec.ts` and `tasks.spec.ts` are single-surface
specs; `tabwalk.ts` also carries `streams`, `sidewaysScrollers`, `pageScrolls`
and `axeViolations`, which are not about the Tab walk. CLAUDE.md asks an
end-to-end surface to break into per-surface folders.

**Fix.** Move `branchlink` to `branch/`, add `reviews/` (or settle the
review/reviews naming) and `tasks/`, and move the stream and fixture helpers
to a fixtures module.

**Done when.** The root holds only cross-section specs and helpers and is
under its budget.

## The gates, the build and the tests

### DEBT-238 Pay down TRADE-1: cohesive packages get a ceiling instead of zero headroom

Severity: medium · Confidence: measured · Size: M

**Where.** `scripts/package-size-budgets.txt` (site comment at `:27`),
`scripts/check-package-size.sh`, `scripts/package-size-budget-history.md`,
CLAUDE.md's "Package & directory size".

**Today.** TRADE-1's trigger, one budget rising three times with no lowering
row, has fired many times over: 21 `internal/tui` rows since 2026-09-25 (45 to
64), the last two after the 2026-10-05 note that kept the entry, while
`internal/webserver` went 20 to 31, `internal/cli` 15 to 20 and
`internal/forge` 12 to 15. Every row gives the same WHY ("the same
responsibility … not a second reason to change"), so for the file-per-pane,
file-per-endpoint and file-per-command packages CLAUDE.md says must not be
split, the zero-headroom budget is a ritual that has never led to a split. Six
more directories sit at the default 12/12 (`web/e2e`,
`web/src/features/branch`, `issues`, `settings/fieldsets`, `tasks`,
`web/src/shell`). TRADE-1's text also says `internal/forge` fills the default,
which it no longer does.

**Fix.** Give the budgets file a second kind of entry for groupings that are
file-per-concern by design (`internal/tui 80 cohesive`, `internal/webserver 40
cohesive`, `internal/cli 28 cohesive`): it fails only past its ceiling, is
exempt from the budget-above-count ratchet, and carries one WHY naming its
spelling rule. Teach `check-package-size.sh` the kind and print headroom in
`--list`; keep zero headroom for every other entry; a ceiling raise still
needs a history row. Change CLAUDE.md so "zero-headroom both ways" applies to
ratcheted entries and says what makes a grouping cohesive. Paying this closes
TRADE-1: delete the entry and its site comment.

**Done when.** `task lint` passes, a 65th file in `internal/tui` needs no
budget edit, a 13th file in `web/src/shell` still fails with the split-or-bump
message, and `--list` prints cohesive headroom.

### DEBT-239 Pay down TRADE-19: the gate fails a never-run condition or an unseen error arm

Severity: medium · Confidence: measured · Size: L

**Where.** `scripts/gobco-report.sh` (`:208`), `BRANCH_COVERAGE_MIN`
(`Taskfile.yml:78`), TRADE-19; among the gaps:
`internal/wiring/tracker.go:107` (`jiraOnly`),
`internal/wiring/messaging.go:198` to `:202` (`asMessagingError`),
`internal/wiring/forge.go:98`, `internal/config/save.go:120`, `:217`, `:222`,
`internal/setup/setup.go:164`, `internal/tui/branch.go:457`,
`internal/tui/composer.go:307`, `internal/tui/fields.go:217`,
`internal/tui/commentcomposer.go:70`, `internal/store/kept.go` (`:63` to
`:234`).

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

### DEBT-240 Hermetic e2e runs proxy every unrouted `/api` call to a developer's real `workflow --web`

Severity: medium · Confidence: read · Size: S

**Where.** `server.proxy` (`web/vite.config.ts:27`), the `webServer` entries
(`web/playwright.config.ts:46`, `:56`), `web/e2e/smoke.spec.ts:3`,
`web/e2e/a11y.spec.ts:77`.

**Today.** `vite preview` inherits `server.proxy`, so on the hermetic projects
any `/api` request a spec does not route (`smoke` routes nothing; others route
a few endpoints) goes to `127.0.0.1:13579`, the default port of `workflow
--web`. The loopback guard passes it (the proxy rewrites Host, and a
same-origin GET carries no Origin). The config's "This e2e run has no backend"
is false whenever a developer's server is up: the run reads their real issues
and repository, becomes non-deterministic, and retained traces record the
data. CI has no server there.

**Fix.** Give `vite preview` an empty proxy (`preview: { proxy: {} }`), or
register a catch-all `**/api/**` route answering a 404 problem in a shared
fixture each spec's routes override.

**Done when.** With `workflow --web` on 13579, `yarn test:e2e` makes no
request to it, and a test asserts an unrouted read gets the fixture's 404.

### DEBT-241 Pay down TRADE-2: split the test files past 700 lines and stop pinning counts in prose

Severity: low · Confidence: measured · Size: M

**Where.** `scripts/check-file-length.sh` (site comment at `:46`), TRADE-2;
`internal/messaging/post_test.go` (768), `internal/webserver/runs_test.go`
(769), `web/src/features/review/ReviewPanel.test.tsx` (710),
`internal/codeowners/codeowners_test.go` (707),
`web/src/features/issues/WorkStory.test.tsx` (702).

**Today.** TRADE-2's trigger has fired: `post_test.go`, given at 766, is 768,
and it holds a second behavior (the announcement's text and templating,
`TestAnnouncementText` to `TestAConfiguredTemplateStillEscapesAHostileValue`).
Most pinned counts are stale (`composer_test.go` 595 not 568,
`BranchPanel.test.tsx` 573 not 513), `jira/detail_test.go` is now under 500,
and the premise of seven skimmable files is gone: 59 files are past the soft
target, 38 of them tests, five past 700, none of those five but `post_test.go`
in the entry. Unlisted examples include `internal/cli/root_test.go`,
`internal/tui/help_test.go`, `tasks_test.go`,
`internal/convention/convention_test.go` and
`internal/gitrepo/branch_test.go`. The warning is noise.

**Fix.** Move the announcement-text tests out of `post_test.go` into
`announcement_test.go` (and optionally the webhook tests into
`webhook_test.go`); split the other four past 700 by the behavior they group,
with shared fixtures in a sibling helper file. Lower the hard ceiling for test
files to 700 in `check-file-length.sh` so it cannot drift again. Paying this
closes TRADE-2: delete the entry and its site comment.

**Done when.** No file in `check-file-length.sh --list` exceeds 700, `task
check` passes, and TRADE-2 is gone.

### DEBT-242 The Tab walk never reports focus landing on an undrawn, zero-size control

Severity: low · Confidence: read · Size: S

**Where.** `drawnOnly` (`web/e2e/tabwalk.ts:28`), `focusedStop` (`:89`),
`walkTabOrder` (`:149`).

**Today.** For a focused element of zero width or height, `shown` is 0/0, NaN,
and `NaN < inView` is false, so the stop is never reported hidden; a stop
`drawnOnly` filtered out gets index −1 and is silently ignored. A tabbable 0×0
control passes the layout guard CLAUDE.md calls enforced. (Index −1 alone
cannot mean hidden: the skip link and the page itself legitimately have it.)

**Fix.** Treat a non-finite `shown` as 0 and report the stop by its words.

**Done when.** A fixture with a tabbable 0×0 button makes the walk's `hidden`
non-empty.

### DEBT-243 Concurrency tests synchronize on wall-clock time

Severity: low · Confidence: read · Size: M

**Where.** `patience` and `drainPast` (`internal/tui/harness_test.go:108`,
`:113`), `internal/webserver/runs_test.go:737`,
`internal/webserver/stream_test.go:245`,
`internal/webserver/announcequeue_test.go:333`,
`TestConcurrentReadsOfAChannelShareOneSetOfRequests`
(`internal/wiring/slackdirectory_test.go:474`).

**Today.** `drainPast` drops any command not answered within 200 ms, assuming
only a held seam is that slow. A run test sleeps 50 ms hoping a stage claimed
the index; a stream test sleeps 40 ms hoping a frame follows a save; the
single-flight test sleeps 50 ms hoping the second reader joined, and passes
against a cache-after-completion implementation when it did not. The module is
on Go 1.27 and no test uses `testing/synctest`; `internal/webserver`'s tests
take 213 s under `-race`.

**Fix.** Have the fakes signal on channels (a held seam reports it is
blocking; the directory fake signals the second arrival), drop only commands
blocked on an armed hold, and use `synctest` bubbles for stream and queue
timing.

**Done when.** `time.Sleep` in these packages' tests is only seam-internal
hold time, and `go test -race -count=20` passes for them.

### DEBT-244 `cmd/docsgen` deletes and writes files with no tests

Severity: low · Confidence: measured · Size: S

**Where.** `run`, `removeGeneratedPages`, `frontMatter`, `hugoLink`
(`cmd/docsgen/main.go`).

**Today.** Coverage is 0.0%. `removeGeneratedPages` deletes every `.md`
holding the generated notice, and its comment rests a mistyped destination's
safety on that guard, which nothing tests (the drift check writes into a fresh
directory). `run` reads `os.Args` directly. The front matter and links are
exercised end to end by `task docs:check`.

**Fix.** Move the logic into an internal package called from a thin main, as
`cmd/testshape` does, with `run(args, stdout, stderr)` and black-box tests of
the removal guard.

**Done when.** A test proves a hand-written `.md` survives and a generated one
is removed.

### DEBT-245 Guards that stop a second send while one is in flight are never tested

Severity: low · Confidence: measured · Size: S

**Where.** `internal/tui/issuewrite.go:130`, `:143`, `:153`, `:170`,
`internal/tui/issuelink.go:73`, `:83`, `internal/tui/branchlink.go:109` to
`:143`, `jobLogView.handleKey` (`internal/tui/checks.go:390`), `logRead.apply`
(`:332`); `hold` (`internal/tui/harness_test.go:203`).

**Today.** gobco lists `send.sending` as never true in the assign, work-log,
Jira-link and branch-link forms, so no test presses enter or pastes while one
of those writes is in flight; dropping the guard would send a Jira write twice
and fail nothing. Esc from a job log (202 times false) and a failed log read
are also untested.

**Fix.** Use `hold` to keep each write seam from answering, press enter again
and assert one call; add tests for esc from a job log and a failing `JobLog`.

**Done when.** gobco reports both outcomes for these conditions.

### DEBT-246 `gobco-report`'s own test never checks that coverage below the floor fails

Severity: low · Confidence: read · Size: S

**Where.** The cases in `scripts/gobco-report_test.sh:83`, the floor check
(`scripts/gobco-report.sh:273`), `unexpected` (`:218`),
`require_current_gobco` (`:116`).

**Today.** The four cases cover a missing floor, no statistics, all untested
packages accounted for (100% against 50) and an unlisted package. No case
measures below the floor, so deleting the floor check passes; nor are a
failing gobco, the `UNANALYZABLE` skip or a gobco built by another Go covered.

**Fix.** Add cases for stats below the floor, a stub gobco that exits 1, a
listed unanalyzable package and a stub `go version -m` reporting another Go.

**Done when.** The suite fails when the floor check is deleted.

### DEBT-247 The goroutine gate cannot see `sync.WaitGroup.Go`, and one already runs in wiring

Severity: low · Confidence: read · Size: S

**Where.** The pattern (`scripts/check-goroutines.sh:53`), `eachAtOnce`
(`internal/wiring/repositories.go:238`).

**Today.** The gate greps for the `go` keyword, so `wg.Go(func(){…})` passes.
`eachAtOnce` reads repositories side by side that way, outside
`internal/proc`, and the gate reports none. It is bounded and correct, but the
stated rule (concurrency only in `tea.Cmd` and `internal/proc`) no longer
holds and the gate cannot tell.

**Fix.** Catch `.Go(func` (or a type-aware forbidigo rule on
`(*sync.WaitGroup).Go`), and record `eachAtOnce` as an allowed exception or
move it behind `internal/proc`.

**Done when.** A fixture using `wg.Go` outside `internal/proc` fails the gate,
and the wiring use carries a written exception.

### DEBT-248 `ci-gate` passes any skipped job, not just the one meant to skip

Severity: low · Confidence: read · Size: S

**Where.** `ci-gate` (`.github/workflows/ci.yml:471`, the filter at `:490`).

**Today.** The jq filter accepts `skipped` for every job; only `commit-lint`
skips by design. An `if:` added to, or mistyped on, any other job turns the
single required check green with that job never run.

**Fix.** Accept `skipped` only for `commit-lint` by name.

**Done when.** Adding `if: false` to the lint job makes `ci-gate` fail.

### DEBT-249 yamllint runs without `--strict`, and its warnings hide truncated API descriptions

Severity: low · Confidence: measured · Size: S

**Where.** `lint:yaml` (`Taskfile.yml:392`), the hook (`lefthook.yml:72`),
`api/openapi.yaml:1560`, `:2895`, `:2908`, `:4086`.

**Today.** `yamllint .` exits 0 on warnings, and four "missing starting space
in comment" warnings pass every run. They mark a real defect: each line is a
plain scalar ending "without its #.", which YAML reads as a comment, so the
description ends "with or without its" and the truncation has reached
`models.gen.go` and `types.gen.ts`.

**Fix.** Quote those four descriptions and run `yamllint --strict`.

**Done when.** `task lint:yaml` fails on a warning, and the generated
descriptions end with "#.".

### DEBT-250 The generated-code drift checks ignore new untracked files

Severity: low · Confidence: read · Size: S

**Where.** `gen:verify` (`Taskfile.yml:501`), `gen:check`
(`web/package.json:11`), `web:dist:check` (`Taskfile.yml:219`).

**Today.** Both run `git diff`, which ignores untracked files. When a hey-api
upgrade or config change makes `openapi-ts` emit a new file, the check passes
with it uncommitted and later CI steps compile against the regenerated tree.
`web:dist:check` already handles this with `git ls-files --others`.
(oapi-codegen writes fixed files, so the Go side has almost no trigger.)

**Fix.** Add an untracked-files check, or use `git status --porcelain` over
the generated directory, in both.

**Done when.** Regenerating with an extra output file left uncommitted fails
`gen:check`.

### DEBT-251 A markdownlint version bump skips the Markdown lint on its own pull request

Severity: low · Confidence: read · Size: S

**Where.** The pattern (`scripts/markdown-changed.sh:25`), the pin
(`mise.toml:60`), the Lint job (`.github/workflows/ci.yml:34`).

**Today.** The lint runs on a pull request only when docs, `*.md` or the lint
config changed. A pull request that bumps markdownlint-cli2 in `mise.toml`,
changing what the lint accepts, skips it, and the new rules first run on main.

**Fix.** Also match `^mise\.toml$`, with a case in `markdown-changed_test.sh`.

**Done when.** The test has a case where a `mise.toml`-only change exits 0.

### DEBT-252 The pre-commit golangci-lint compares against `HEAD~`, one commit wider than it says

Severity: low · Confidence: read · Size: S

**Where.** The `golangci-lint` command (`lefthook.yml:38`).

**Today.** In pre-commit the commit is not made yet, so what it changes is the
diff from HEAD; `--new-from-rev=HEAD~` also lints every line of the previous
commit, so an issue already in HEAD fails an unrelated commit. The fallback
reasoning is also wrong: on the second commit `HEAD~` is missing and the hook
lints the whole tree.

**Fix.** Use `--new-from-rev=HEAD`, falling back when `git rev-parse --verify
--quiet HEAD` fails, and fix the comment.

**Done when.** With a lint issue committed in HEAD and an unrelated clean
staged change, the hook passes.

### DEBT-253 The commit-message gate does not check the 72-column body wrap CLAUDE.md asks for

Severity: low · Confidence: read · Size: S

**Where.** `scripts/check-commit-message.sh:28` to `:100`.

**Today.** It checks the subject's pattern, length and period and the breaking
markers, never a body line, so long bodies pass lefthook and CI though
CLAUDE.md asks for a body "wrapped at 72", and every commit lands on main as
written.

**Fix.** Refuse prose body lines over 72 characters, except URLs, trailers and
indented or fenced blocks.

**Done when.** The test has a case where a 90-character prose body line fails.

### DEBT-254 No CI job sets `timeout-minutes`, and several workflows lack a concurrency group

Severity: low · Confidence: read · Size: S

**Where.** Every job in `.github/workflows/*.yml`; `codeql.yml`,
`container.yml`, `release-please.yml`, `release.yml`.

**Today.** A hung Playwright server, gobco run or container gate holds a
runner for the six-hour default. Those four workflows have no `concurrency:`;
release races are narrow (a hand tag push plus a dispatch), so the cost is
mostly runner minutes.

**Fix.** Set `timeout-minutes` per job, and add a non-cancelling release group
to the release workflows and a cancelling group to CodeQL.

**Done when.** `grep -L timeout-minutes .github/workflows/*.yml` prints
nothing and zizmor and actionlint stay clean.

### DEBT-255 Dependabot's docker entry cannot update the build image's `FROM` line

Severity: low · Confidence: read · Size: S

**Where.** The `/build` entry (`.github/dependabot.yml:71`), `FROM
golang:${GO_VERSION}-trixie@sha256:…` and its comment (`build/Dockerfile:21`,
`:25`).

**Today.** Dependabot does not substitute build ARGs, so it cannot parse the
`FROM` reference, and no Dependabot PR has ever touched the Dockerfile, though
the comment says Dependabot updates the line. A digest bump without
`mise.toml` and `GO_VERSION` would fail the check after `FROM` anyway.

**Fix.** Remove the entry and say in the Dockerfile that the digest moves by
hand with a Go bump, guarded by `check-go-version.sh`.

**Done when.** The `/build` entry is gone and the Dockerfile comment matches.

### DEBT-256 `postCreate` skips the hooks in a worktree, and the devcontainer image floats

Severity: low · Confidence: read · Size: S

**Where.** `.devcontainer/postCreate.sh:94`,
`.devcontainer/devcontainer.json:3`.

**Today.** In a worktree or submodule `.git` is a file, so `-d` is false and
lefthook is never installed, with a misleading "No .git directory yet". The
base image and the docker-in-docker feature float on tags, while the build
container pins by digest.

**Fix.** Test with `git -C "$workspace" rev-parse --git-dir`, and pin the
image and feature by digest for Dependabot's devcontainers ecosystem to move.

**Done when.** postCreate installs hooks in a `git worktree add` checkout and
the image carries an `@sha256` digest.

### DEBT-257 `task fmt` does not format the web

Severity: low · Confidence: read · Size: S

**Where.** `fmt` (`Taskfile.yml:478`), `fmt` and `lint` scripts
(`web/package.json:12`, `:14`).

**Today.** CLAUDE.md lists `task fmt` as formatting everything, but it skips
prettier, while `yarn lint` fails on `prettier --check`; a developer who ran
`task fmt` still fails `task check` on web formatting.

**Fix.** Add a `web:fmt` task running `corepack yarn fmt` to `fmt`.

**Done when.** After `task fmt` on a misformatted `.tsx`, `task web:lint`
passes its prettier step.

### DEBT-258 CI's Web job re-implements the web tasks, with a second copy of the bundle check

Severity: low · Confidence: read · Size: S

**Where.** The `web` job (`.github/workflows/ci.yml:107`, the bundle check at
`:146`), `web:install` to `web:dist:check` (`Taskfile.yml:181` to `:247`).

**Today.** Every other job calls `task`. This one runs `corepack enable`,
`yarn` steps and its own copy of `web:dist:check`, and the copies differ (bare
`yarn` against `corepack yarn`, no git-environment scrub), so a new web lint
step is made in two places.

**Fix.** Run `task web:gen:check web:lint web:test web:dist:check`, passing
the extra reporters through `CLI_ARGS`, and delete the inline check.

**Done when.** The Web job's steps are `task` calls and `ci.yml` holds no `git
diff --quiet -- ../internal/web/dist`.

### DEBT-259 The Go package roots are written out in three places

Severity: low · Confidence: read · Size: S

**Where.** `GO_PKGS` (`Taskfile.yml:10`), `unit-go` (`lefthook.yml:135`),
`go_roots` (`scripts/gobco-report.sh:162`).

**Today.** `./cmd/... ./internal/... ./api/...` is written three times, each
with its own comment about the stray Go package in `web/node_modules`, and the
pre-push hook repeats `task test` rather than calling it. A missed copy
silently leaves a package out of the pre-push tests or the condition gate.
`test-summary.sh` already takes the roots as arguments.

**Fix.** Pass `GO_PKGS` to `gobco-report.sh` (the explicit package list moving
to a flag) and have `unit-go` run `task test`.

**Done when.** No copy of the roots remains in `lefthook.yml` or `scripts/`.

### DEBT-260 Every script test re-implements one harness, and `test:scripts` runs each twice

Severity: low · Confidence: read · Size: M

**Where.** `scripts/*_test.sh` and `scripts/release/*_test.sh` (21 files;
`expect` in `check-goroutines_test.sh:23`), `test:scripts`
(`Taskfile.yml:263`), `scripts/hook-environment_test.sh:32`.

**Today.** Each declares its own counters, an `expect` (14 copies), a work
directory and trap, the closing summary, and in nine the git-environment
scrub. `test:scripts` lists every test by hand, and
`hook-environment_test.sh`, on that list, runs every other test again under a
hook's environment, so each runs twice per `task test`, the second time
silently.

**Fix.** A `scripts/lib/testing.sh` with setup, expect-exit, expect-output and
finish, sourced by each; have `test:scripts` run the hook-environment test,
which already runs every test once.

**Done when.** No `^failures=0` remains in a script test, and each script test
runs once per `task test:scripts`.

### DEBT-261 CI repeats work and installs the whole toolchain in every job

Severity: low · Confidence: read · Size: M

**Where.** `summary` (`.github/workflows/ci.yml:259`), `build` (`:90`),
`e2e-server` (`:233`), `cross` (`:283`), every `jdx/mise-action` step,
`scripts/test-summary.sh`.

**Today.** The advisory summary reruns the Go tests with `-race`, vitest with
coverage and Playwright, though those jobs upload their counts; `task build`
runs in build and e2e-server, and the web bundle in four jobs; every mise step
installs all of `mise.toml` (Go, golangci-lint, hugo, gobco, gitleaks, node)
even in node-only jobs. The summary is not on the required path, so the cost
is runner minutes and slower installs.

**Fix.** Build the summary from the uploaded artifacts with `needs:`, drop the
build job or have it upload `bin/`, and pass `install_args` to install only
each job's tools.

**Done when.** The summary job runs no test suite, and the node-only jobs
install only node.

### DEBT-262 Fifteen `//nolint:tagliatelle` lines in `internal/jira` could be one exclusion

Severity: low · Confidence: measured · Size: S

**Where.** `internal/jira/fields.go:133`, `:144`, `activity.go:152`, `:162`,
`transitions.go:48`, `search.go:120`, `:121`, `:147`, `:156`, `worklog.go:19`,
`:35`, `answer.go:33`, `detail.go:70`, `jira.go:65`, `worklog_test.go:31`;
`tagliatelle` settings (`.golangci.yml:169`).

**Today.** Every Jira wire struct repeats "Jira's field name on the wire". The
snake_case rule exists for `.workflow.json`; Jira's REST is camelCase by
contract and the package holds no JSON of ours, so the exception is the whole
package, spelled field by field.

**Fix.** Add an exclusion rule for `path: internal/jira/` and `linters:
[tagliatelle]` with the reason once, and delete the directives.

**Done when.** `grep -rc 'nolint:tagliatelle' internal/jira` prints 0 and
`task lint:go` passes.

### DEBT-263 The forge's error paths for activity, job logs and writes are never exercised

Severity: low · Confidence: measured · Size: M

**Where.** `githubActivity`, `githubReviewed`, `gitlabActivity`
(`internal/forge/activity.go:96` to `:193`), `logResponse`
(`internal/forge/cijobs.go:145`), `JobLog` (`:100`), `send`
(`internal/forge/client.go:189`), `githubRequestReviewers`
(`internal/forge/github.go:411`).

**Today.** gobco lists every `err != nil` in the activity reads as never true,
a forge 4xx or 5xx on a log's first request never seen, a missing token in
`JobLog` and `send` never seen, and the one-by-one reviewer fallback never
entered on an unexpected status. A regression in classifying those failures
goes unnoticed.

**Fix.** Table cases answering 401, 403, 404 and 500 for activity on both
forges and a log's first request, and Merge and Rerun with no token.

**Done when.** gobco's worklist drops those lines.

### DEBT-264 Credential conditions in `internal/config` are never seen true by its tests

Severity: low · Confidence: measured · Size: S

**Where.** `hasToken` (`internal/config/config.go:252`),
`HoldsUserTokenSecrets` and `hasUserToken` (`:323`, `:329`), `Layers`
(`internal/config/layers.go:62`), `CreateLayers` (`:166`).

**Today.** A `token_env`-only Jira, a messaging block holding only a refresh
or access token, the `Layers()` path fallback and the whole of `CreateLayers`
(its refusal over a changed file included) are never seen by a test, though
they decide the auth mode, which file holds Slack secrets and whether a
credential write is refused.

**Fix.** Table cases for each, and a test that `CreateLayers` over a changed
home file returns `ErrChangedOnDisk`.

**Done when.** `task cover:branch` no longer lists those conditions.

### DEBT-265 Three forge test helpers each rebuild one recording fake server

Severity: low · Confidence: read · Size: S

**Where.** `forgeAnswering` (`internal/forge/pulls_test.go:51`),
`scriptedForge` (`internal/forge/bestreviewers_test.go:34`),
`forgeConversation` (`internal/forge/prpeople_test.go:41`), `serveForge` and
`answerJSON` (`client_test.go:33`, `:43`), `clientOn` (`refusal_test.go:27`).

**Today.** Each starts a server, decodes the body, records method, path, query
and body and answers; they differ only in how the answer is chosen. Recording
a header means editing three to five copies.

**Fix.** One `recordingForge(t, answer func(recorded) (int, string))`, with
the others as answer funcs.

**Done when.** The forge tests hold one server helper that decodes request
bodies.

### DEBT-266 The git-environment scrub list is hand-copied into two `TestMain`s

Severity: low · Confidence: read · Size: S

**Where.** `TestMain` (`internal/wiring/main_test.go:24`,
`internal/cli/main_test.go:28`), `git` helpers
(`internal/wiring/wiring_test.go:51`, `internal/cli/doctor_test.go:140`).

**Today.** Both carry the fifteen names of `git rev-parse --local-env-vars`
and copies of the same hook-environment tests, and the two `git` helpers have
already drifted. A git version adding a variable must be fixed in each copy; a
missed one lets a test write into the developer's repository from a hook.

**Fix.** An internal test-support package holding the list and helper (as
`internal/rlimit` is), or read the list at run time.

**Done when.** One definition of the list exists and both `TestMain`s call it.

### DEBT-267 The gitrepo test helper `with` mutates the fixture it is given

Severity: low · Confidence: read · Size: S

**Where.** `with` (`internal/gitrepo/gitrepo_test.go:80`).

**Today.** `maps.Copy` writes into the caller's map; it works only because
every caller passes a fresh one. A shared fixture would leak answers between
parallel tests.

**Fix.** Clone first: `merged := maps.Clone(replies); maps.Copy(merged,
changes)`.

**Done when.** `with` does not write to its argument and `go test -race
./internal/gitrepo` passes.

### DEBT-268 testshape misnames versioned imports, so a library call can count as a failing helper

Severity: low · Confidence: read · Size: S

**Where.** `importNames` (`internal/testshape/scope.go:249`), `selectorFails`
(`internal/testshape/failures.go:234`), `methodOutcome`
(`internal/testshape/receivers.go:56`).

**Today.** An unaliased `go.yaml.in/yaml/v3` is recorded as `v3`, so
`yaml.X(t)` falls through to matching test-package methods by name; a
same-named asserting method makes an Assert that only calls a library count as
reaching a failure. No test hits it today.

**Fix.** Drop a trailing `/vN` (and gopkg.in's `.vN`) before taking the base.

**Done when.** A fixture calling an unaliased `/v2` package's `Check(t)`
beside an unrelated asserting `Check` reports assert-without-failure.

### DEBT-269 `eslint-plugin-jsx-a11y` runs under ESLint 10, outside its declared peer range

Severity: low · Confidence: measured · Size: S

**Where.** `web/package.json:50`, `:52`, `jsxA11y.flatConfigs.strict`
(`web/eslint.config.js:107`).

**Today.** Yarn reports the plugin's peer range (`^3 … ^9`) and ESLint 10.11.0
do not overlap, and `@axe-core/playwright`'s `playwright-core` peer is
undeclared; both warn on every install. The rules still fire today (an `img`
with no alt fails lint), so the risk is an unsupported pairing nothing proves.

**Fix.** Add a `packageExtensions` entry with a written reason, plus a lint
fixture that proves the strict rules fire; dedupe `@typescript-eslint/utils`.

**Done when.** `corepack yarn explain peer-requirements` lists no ✘ for these,
and a bad fixture fails `yarn lint`.

### DEBT-270 Web coverage thresholds for branches and functions sit at floor(measured)

Severity: low · Confidence: measured · Size: S

**Where.** `coverage.thresholds` (`web/vitest.config.ts:38`).

**Today.** The comment says floor(measured) − 2; lines and statements follow
it (96), but branches (94, measured 94.03%) and functions (98, measured
98.54%) have no tolerance, so an incidental refactor fails the gate.

**Fix.** Set branches to 92 and functions to 96, or state the exception and why.

**Done when.** Each threshold equals floor(measured) − 2 or the comment names
the exception.

### DEBT-271 The web lint has no exhaustive-switch or React key rules

Severity: low · Confidence: read · Size: S

**Where.** `web/eslint.config.js:99`, `web/tsconfig.app.json:21`, `Shape`
(`web/src/shell/StateMark.tsx:48`), `web/src/lib/Meta.tsx:15`.

**Today.** Go fails an incomplete switch through `exhaustive`; the web enables
neither `switch-exhaustiveness-check` nor `noImplicitReturns`, so a new
`MarkState` makes `Shape` silently draw nothing. No React rules plugin checks
keys, and index keys appear in several components.

**Fix.** Enable `switch-exhaustiveness-check` and `noImplicitReturns`; propose
`@eslint-react/eslint-plugin` (a new dev dependency, to be approved) for key
rules.

**Done when.** Removing a case from `Shape` fails `yarn lint`.

### DEBT-272 Vitest's globals, alias and setup are loose

Severity: low · Confidence: read · Size: S

**Where.** `types` (`web/tsconfig.app.json:7`), `web/vite.config.ts:21`,
`web/vitest.config.ts:6`, `web/tsconfig.app.json:12`, `afterEach`
(`web/src/test-setup.ts:15`, `:74`).

**Today.** `vitest/globals` types cover production source, so a stray
`expect(...)` in a component compiles and lints. The `@` alias and React
plugin are declared in three configs that ask to be kept in step. `test-setup`
resets five stores through `getInitialState()` but two with literals that
restate their shapes, so a new store or field leaks between tests until
someone edits the list.

**Fix.** Split a test project that adds the globals; build `vitest.config.ts`
with `mergeConfig` over the Vite config and read paths from tsconfig; reset
every store through `getInitialState()`, or a zustand test helper that records
stores.

**Done when.** `expect(1)` in `App.tsx` fails `tsc -b`, `'@'` is declared
once, and `test-setup.ts` holds no literal store state.

### DEBT-273 Vitest re-creates jsdom per file

Severity: low · Confidence: measured · Size: S

**Where.** `test` (`web/vitest.config.ts:13`).

**Today.** The run reports jsdom created 111 times, 82.56 s of tracked worker
time (about 20 s of wall time on four cores), and Vitest itself suggests
`vmThreads` or `isolate: false`.

**Fix.** Try `pool: 'vmThreads'`, checking the prototype and global patches in
`test-setup.ts` under it.

**Done when.** The run's jsdom setup time falls well below 30 s of tracked
time with every test green.

### DEBT-274 An unused `@types` package in devDependencies

Severity: low · Confidence: read · Size: S

**Where.** `@types/eslint-plugin-jsx-a11y` (`web/package.json:42`).

**Today.** Its only consumer, `eslint.config.js`, is plain JavaScript no
tsconfig checks, so the types are never read; knip pairs `@types/x` with `x`
and does not flag it.

**Fix.** Remove it, or type-check the config as `eslint.config.ts`.

**Done when.** `yarn lint` passes without it.

### DEBT-275 `apiError.test.ts` packs five scenarios into one Act and Assert

Severity: low · Confidence: read · Size: S

**Where.** `web/src/api/apiError.test.ts:3`, `problemCode`
(`web/src/api/apiError.ts:49`).

**Today.** One test runs five `apiErrorMessage` cases back to back, against
CLAUDE.md's one Act per test; the first failure hides the rest. `problemCode`
has no direct test.

**Fix.** A `test.each` table, and one for `problemCode`.

**Done when.** The file uses `test.each` and covers `problemCode`.

### DEBT-276 No lint for the e2e specs: raw locators, `networkidle`, non-retrying asserts and Arrange-Act-Assert pass

Severity: low · Confidence: read · Size: M

**Where.** The e2e block (`web/eslint.config.js:172`),
`web/e2e/review/rows.spec.ts:22`, `tasks.spec.ts:161`, `:191`, `:280`,
`issues/comments.spec.ts:120`, `reviews.spec.ts:98`,
`review/writes.spec.ts:104`, `branch/runs.spec.ts:121`, `screens.spec.ts:33`;
`cmd/testshape`.

**Today.** The specs answer only to the four black-box patterns the units do,
so `locator('dd')`, `locator('svg')`, `waitForLoadState('networkidle')` and
`expect(await x.innerText())` pass, and a `[data-…]` selector is not caught.
`cmd/testshape` reads only Go, so in TypeScript the Arrange-Act-Assert markers
hold by habit: two flows assert a precondition inside an unlabeled Arrange
where CLAUDE.md asks labeled steps.

**Fix.** Add `eslint-plugin-playwright` (a new dev dependency, to be approved)
with `no-raw-locators`, `no-networkidle`, `prefer-web-first-assertions` and
`no-wait-for-timeout`, or extend `no-restricted-syntax` to ban CSS strings in
`.locator(`; add a small AST check for the markers, or say in CLAUDE.md the
marker rule is gated in Go only; relabel the two flows.

**Done when.** `yarn lint` fails on a new `page.locator('svg')` in `web/e2e`
and on a spec Assert with no `expect`.

### DEBT-277 Package-size gate advice cites features that do not exist here

Severity: low · Confidence: read · Size: S

**Where.** The split advice (`scripts/check-package-size.sh:196`).

**Today.** It says "features/genres came out of features/lists exactly that
way"; neither has ever existed in this repository.

**Fix.** Cite a real split from `scripts/package-size-budget-history.md`, or
drop the example.

**Done when.** `grep -n genres scripts/check-package-size.sh` returns nothing.

## The docs

### DEBT-278 The trade-off register's text no longer matches the code or itself

Severity: low · Confidence: read · Size: S

**Where.** TRADE-1, TRADE-2, TRADE-13, TRADE-15, TRADE-16, TRADE-18 and
TRADE-20 in this file; the register's ordering (TRADE-28 between TRADE-23 and
TRADE-24); `scripts/check-tradeoffs.sh`.

**Today.** The entries this edition keeps were rewritten to `b9ab000`, but the
ones left until their paydown still cite what the code no longer has: TRADE-18
names `targetDir` (now `whereInit`) and places `connectLeniently` in `cli.go`;
TRADE-20 places its check at `cli.go:315`; TRADE-15 lists two of three
`sql.Open` sites (`openKept` is missing); TRADE-16's eleven lines have all
moved; TRADE-1 says `internal/forge` fills the default and ends its Decided
paragraph with a sentence about TRADE-4's `freshFor`. `check-tradeoffs.sh`
checks IDs, not the files and lines an entry cites, so nothing catches this.

**Fix.** When each paydown above lands its entry is deleted; until then cite
symbols, not lines, and sort the register by ID. Optionally have
`check-tradeoffs.sh` check that every `file:line` an entry cites holds a
matching site comment.

**Done when.** No entry names a function that does not exist, and the register
is in ID order.

### DEBT-279 The docs describe the dry run's store three contradictory ways

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/usage.md:862`, `:508`, `:760`,
`docs/content/docs/configuration.md:999`, CLAUDE.md's store rules;
`openInterface` (`internal/cli/cli.go:345`), `keptReads`
(`internal/tui/dryrun.go`), `readKeptAsItIs` (`internal/store/kept.go:108`),
`version_test.go:142`.

**Today.** usage.md's Dry run section says it "opens no store", and
configuration.md that neither surface opens `kept.db` "at all"; the code binds
owner links, groups, last groups and favorites to the read-only store, as
usage.md's own lines 508 and 760 say. configuration.md names `workflow.db-wal`
and `-shm` as what a `kept.db` read leaves. CLAUDE.md says the read-only open
"neither checks, stamps nor changes" either file, but a `kept.db` at another
version is checked and read as empty, while `workflow.db` is read as it is on
purpose (pinned by a test). Nothing is written in any case.

**Fix.** Rewrite both docs passages to match: the interface reads `kept.db`
read-only when it exists, opens no cache, and never makes or changes either
file; name the companion files after the file read. In CLAUDE.md say: "A
`--dry-run` store's read-only open reads `workflow.db` as it is and a
`kept.db` at another version as empty, never stamps or changes either file,
and never makes one that is missing."

**Done when.** `grep -rn 'opens no store\|opens it at all' docs/content` finds
nothing and the three passages agree with `openInterface`.

### DEBT-280 CLAUDE.md's layout omits `internal/taskwarrior` and `internal/slackauth`, and kept favorites

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md's layout block and first paragraph, its `kept.db`
sentence, `internal/config/store.go:6`,
`docs/content/docs/configuration.md:147`, ARCHITECTURE.md's package table
(`:166`).

**Today.** Every directory under `internal/` is in the layout but these two;
`slackauth` holds a credential (the Slack user token), so it is invisible to
someone looking for where secrets live, and Taskwarrior is a fourth
integration the summary does not name. The `kept.db` sentence leaves out
favorite directories, and `config.Store`'s doc and the `store.disabled` row
say disabling the store drops only conveniences, though it also drops owner
links, groups and favorites. Nothing checks the layout against the
directories.

**Fix.** Add `internal/taskwarrior/   finding Taskwarrior and reading and
changing its tasks` and `internal/slackauth/   the Slack user token:
refreshing, storing, locking`, name Taskwarrior in the first paragraph, and
add favorites to the `kept.db` sentence; reword `config.Store`'s doc and the
`store.disabled` row; add a small check, in `task lint`, that every Go package
directory is named in the layout.

**Done when.** The check passes and `grep favorite CLAUDE.md` matches the
`kept.db` sentence.

### DEBT-281 README gives nine panes the keys 1 to 7, lists seven web sections, and keeps writes in the terminal

Severity: low · Confidence: read · Size: S

**Where.** `README.md:239`, `:256`, `:258`; `web/src/shell/sections.ts:22`,
`docs/content/docs/web.md:42`, `:147`, `:248`, `:586`.

**Today.** README says nine panes but keys `1`–`7` (the TUI binds 1 to 9),
lists seven web sections (there are nine, with Summary and Repositories), and
says re-running CI, merging, finishing a branch and most issue writes stay in
the terminal, where web.md documents them all; only the task offers stay
terminal-only.

**Fix.** Write `1`–`9`, list the nine sections, and replace the sentence with
the one remaining gap or a link to web.md.

**Done when.** README has no "`1`–`7`" and no "seven sections", and its
terminal-only sentence matches web.md.

### DEBT-282 Status, install and release prose is copied across four documents and has drifted

Severity: low · Confidence: read · Size: M

**Where.** `README.md:23` and `docs/content/_index.md:12` (the Status lists),
the taglines (`README.md:20`, `docs/content/_index.md:8`,
`CONTRIBUTING.md:4`), the platforms (`CONTRIBUTING.md:223`,
`docs/content/docs/install.md:67`, `RELEASE_PLATFORMS` in `Taskfile.yml:94`),
`CONTRIBUTING.md:161`, `docs/content/docs/contributing.md:14`.

**Today.** The site's Status list lacks the forge-issues, summary and reviews
bullets and describes `config init` differently; the taglines drop the plain
webhook; the platforms are written by hand beside their gate's source;
CONTRIBUTING's release step omits the SBOM `install.md` and `release.yml`
name; and CONTRIBUTING says a subject "under 72" where the hook allows 72.

**Fix.** Make `_index.md` the one Status source with README linking to it;
name the platforms by linking to the release; keep setup in CONTRIBUTING with
the site linking to it; say "at most 72".

**Done when.** Each fact appears in one hand-written place and no second copy
of the Status bullets exists.

### DEBT-283 Reference pages carry release history the CHANGELOG already keeps

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/usage.md:242`,
`docs/content/docs/configuration.md:666`, `:531`; `CHANGELOG.md:201`, `:370`.

**Today.** The pages say `9` "is new with the Repositories pane" and a map
that "worked before … now stops workflow from starting", and that "before, an
unknown GitLab reviewer … stopped the merge request". The rule is stated
beside each, and the history is in the CHANGELOG.

**Fix.** Delete the history clauses and keep the rules.

**Done when.** `grep -rn 'worked before\|before, an unknown\|is new with'
docs/content` finds nothing.

### DEBT-284 configuration.md promises an earlier-build workspace migration the kept schema cannot hold

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/configuration.md:978`, `ownersSchema` and
`OwnerLinks` (`internal/store/owners.go:88`, `:129`), `keptSchemaVersion`
(`internal/store/kept.go:33`).

**Today.** The docs say links kept by an earlier build belong to no workspace
and an owner it marked not on Slack stays so everywhere. Every `slack_team` is
`NOT NULL` and part of its key, reads match the current workspace only, and an
earlier file reads as empty (TRADE-25), so neither can happen.

**Fix.** Delete the two sentences; the next paragraph already says an earlier
file reads as empty until `workflow db-clean --all`.

**Done when.** configuration.md has no "earlier build" sentence about
workspaces.

### DEBT-285 The key tables and command lists have small errors nothing checks

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/web.md:107` and `:346`,
`docs/content/docs/usage.md:127` (the key table, no `l`) and `:26` (the
command list), `docs/content/docs/scripting.md:63` (the family table),
`docs/content/docs/web.md:612` ("Five reads"); `showLog`
(`internal/tui/keys.go:346`), `internal/tui/keys_docs_test.go:23`, the
`ProblemCode` enum.

**Today.** web.md names the terminal's issue filter `p` (it is `f`) and the
review sort `s` (it is `O`); usage.md's table claims every key `?` lists but
has no `l` (show log), and its scriptable commands omit `repositories` and
`comment`; scripting.md's table of web families omits `too_long`,
`fetch_failed` and `check_failed`; web.md says "five reads" over a table of
ten requests, two of them writes. Only configuration.md's action list is
tested against the bindings.

**Fix.** Correct each; extend `keys_docs_test.go` to check usage.md's table
and add a test that every `ProblemCode` appears in scripting.md's table;
replace the command list with a link to Scripting.

**Done when.** A test fails when a bound action's default key or a problem
code is missing from its table, and the web.md passages agree with their
tables.

### DEBT-286 Gate texts say "two build-tagged twins", the golangci comments misname what is off, and the container docs misplace the release build

Severity: low · Confidence: read · Size: S

**Where.** `scripts/gobco-report.sh:44`, `:53`, `Taskfile.yml:27`, `:35`;
`.golangci.yml:18`, `CONTRIBUTING.md:124`;
`.github/workflows/container.yml:3`, `build/Dockerfile:1`,
`.github/workflows/release.yml:55`.

**Today.** `UNANALYZABLE` names one package, yet gobco-report's header and the
Taskfile speak of two. `.golangci.yml` says the deprecated linters are
"superseded by the _v5 linters enabled above" where both generations are
disabled, and CONTRIBUTING says every linter is enabled though five are off.
The container workflow says the release binaries are built in the container;
`release.yml` builds them on the runner with mise.

**Fix.** Name the one package; reword the golangci comment and CONTRIBUTING
("every linter but the few `.golangci.yml` disables, each with its reason");
say the container runs the gate on demand and weekly.

**Done when.** `grep -rn 'two build-tagged\|Two packages are there'
Taskfile.yml scripts` finds nothing, and no file claims the release is built
in the container.

### DEBT-287 configuration.md says any `NO_COLOR` value turns color off, but an empty one does not

Severity: low · Confidence: read · Size: S

**Where.** `docs/content/docs/configuration.md:625`, `DrawColor`
(`internal/config/ui.go:59`).

**Today.** `DrawColor` keeps color for `NO_COLOR=` (as no-color.org asks),
while the doc and the function's comment say "any value".

**Fix.** Say "a non-empty value" in both.

**Done when.** The doc and the comment say non-empty.

### DEBT-288 The `loop` package doc names the wrong import set

Severity: low · Confidence: read · Size: S

**Where.** The package doc (`internal/loop/loop.go:12`), the depguard rule
(`.golangci.yml:58`).

**Today.** It lists seven packages; loop also imports `activity`, `codeowners`
and `taskwarrior`, all allowed by depguard.

**Fix.** Point the doc at the depguard rule rather than restating the list.

**Done when.** The doc lists no packages, or matches the rule.

## Across the surfaces

### DEBT-289 `Guide.Write` stores the token in the keychain before the file write that can still fail

Severity: medium · Confidence: read · Size: S

**Where.** `Guide.Write` (`internal/setup/guide.go:72`, `Keep` at `:88`),
`Keep` (`internal/setup/setup.go:179`), `storeLine`
(`internal/keychain/keychain.go:172`), `checkTyped`
(`internal/webserver/setup.go:146`), the TUI's setup form
(`internal/tui/setupform.go:571`).

**Today.** `Write` keeps the token (`add-generic-password -U`, replacing any
`workflow-jira` item) before `Beneath` and `Create`, either of which can still
fail, so a failed setup can leave the keychain changed with no file pointing
at it. It keeps whenever the base URL is set, even with an empty token: a
blank token kept unchecked (both setup surfaces allow it) stores "" over an
existing keychain token, which another configuration may rely on. macOS only,
first-run setup only.

**Fix.** Refuse `Keep` for an empty token with a sentinel, and run `Beneath`
before `Keep`, or delete the item when `Create` fails.

**Done when.** Setup tests show `Write` with an empty token never calls the
store, and a `Write` whose `Create` fails leaves it uncalled.

### DEBT-290 A streamed run never ends while any descendant holds the output pipe

Severity: medium · Confidence: read · Size: M

**Where.** `Start` and `deliver` (`internal/proc/start.go:89`, `:110`,
`:137`), `killGrace` and `Isolate` (`internal/proc/pgroup/pgroup_unix.go:17`,
`:32`).

**Today.** `Start` hands the child an `os.Pipe` write end, so exec makes no
pipes of its own and `WaitDelay` has nothing to close; the reader calls `Wait`
only after `deliver` reads to EOF. A hook that backgrounds a process holding
stdout (`server &`, or a daemon that called setsid and escaped the group kill)
keeps the pipe open after the child exits: `Lines` never closes, `Wait` blocks
forever, the run shows as running in the TUI or web, and a goroutine and
descriptor leak per run. `killGrace`'s comment claims the opposite.

**Fix.** Wait for the child's exit in its own goroutine; once it exits, give
`deliver` the grace period and then close the reader. Fix the comment.

**Done when.** A Unix test whose helper backgrounds a `sleep 60` holding
stdout and exits 0 sees `Lines` close and `Wait` return within seconds, under
`-race`.

### DEBT-291 Draining a streamed program is copied five times, each treating its lines differently

Severity: medium · Confidence: read · Size: S

**Where.** `Push` (`internal/loop/push.go:57`), `streamToEnd`
(`internal/wiring/wiring.go:276`), `installLefthook` (`:435`), `runCommit`
(`internal/webserver/commit.go:152`), `streamRun`
(`internal/webserver/runs.go:278`), `pushFailure` (`internal/cli/pr.go:395`).

**Today.** Five places range over `output.Lines`, wait and build an error from
the lines, and each one prepares the lines in its own way. How a caller sees a
program's output depends on which copy it picked, and a sixth caller will
copy whichever it finds first.

**Fix.** Add `proc.Output.Drain() ([]string, error)` returning the prepared
lines and the exit error, used by every drain-to-slice caller.

**Done when.** No `for line := range output.Lines` remains in `internal/loop`
or `internal/wiring`, and one test of `Drain` covers what every caller
receives.

### DEBT-292 `readPages` stops at its cap without saying so, and `Activity.Truncated` ignores it

Severity: medium · Confidence: read · Size: M

**Where.** `readPages` (`internal/forge/client.go:371`), `gitlabActivity` and
`githubActivity` (`internal/forge/activity.go:181`, `:96`), `githubSearch`
(`internal/forge/github.go:176`), `githubPages`
(`internal/forge/githubci.go:183`), `gitlabIssues`
(`internal/forge/gitlab.go:227`), `gitlabGroupMembers`
(`internal/forge/members.go:94`), `Activity` (`activity.go:48`).

**Today.** `readPages` returns what it read after 20 pages with no error and
no flag. GitLab's activity reads `/events` (every event kind) through it
ascending, so a busy period drops its newest events while `Truncated`,
documented as "whether there was more than was read" and drawn by the Summary
as "had more than this shows", stays false. GitHub's opened and merged
searches cap at 1,000 unreported. `githubPages` counts for itself; the other
listings cut silently. A Summary under-reports and says it is complete.

**Fix.** Have `readPages` return a truncated flag, thread it into `Truncated`
on both forges, and use it in place of `githubPages`'s counting.

**Done when.** A `gitlabActivity` test serving 21 full pages gets `Truncated
== true`, and a GitHub search past 1,000 does too.

### DEBT-293 Pay down TRADE-17: the keychain placement takes its platform and runner as arguments

Severity: medium · Confidence: measured · Size: M

**Where.** `SlackStore` and `placeSlackCredentials`
(`internal/wiring/messaging.go:160`, `:224`), `keptUnlessTyped`,
`keychain.Open` (`internal/keychain/keychain.go:68`), `slackauth.Choose`.

**Today.** TRADE-17's trigger is already true: `keychain.Open` takes the
platform, a runner, a user lookup and `getenv`, and `slackauth.Choose` takes
the platform; only the wiring hard-codes `runtime.GOOS` and `proc.Capture`. So
gobco reports every condition from `messaging.go:226` to `:264` never
evaluated, including `keptUnlessTyped`, which takes a blank typed secret from
the keychain. This is credential code CLAUDE.md asks to cover above the floor.

**Fix.** Thread the platform and runner through `SlackStore`,
`placeSlackCredentials` and the exported path that reaches it, with production
passing `runtime.GOOS` and `proc.Capture`. Test with `darwin`, a fake runner
and a fake Slack: the returned configuration has the secrets cleared, the
runner stored the refreshed pair, a blank typed refresh token is taken from
the fake keychain, a refusal maps to `messaging.ErrRejected`, and no secret
reaches an error unmasked. Paying this closes TRADE-17: delete the entry and
its site comment.

**Done when.** gobco lists none of `messaging.go:226` to `:264` as never
evaluated.

### DEBT-294 A Jira-only tracker sends bare forge numbers to Jira for assign and transition

Severity: low · Confidence: read · Size: S

**Where.** `jiraDeps` (`internal/wiring/jira.go:28`), `readJiraIssue` and
friends (`:125`), `trackerDeps` (`internal/wiring/forgeissues.go:40`),
`AssignIssue`, `TransitionIssue`, `ChangeStatus`
(`internal/webserver/issuewrite.go:385`, `:107`, `:259`), `combinedTracker`
(`internal/wiring/tracker.go:33`).

**Today.** `jiraDeps` refuses a key with no project part for reads and
comments, because Jira reads a bare number as an issue id; its transition and
assign seams have no guard, and a Jira-only tracker returns `jiraDeps`
unwrapped. A hand-made web request for `42` changes Jira issue id 42, someone
else's issue. (The web's log-work and link paths already refuse such keys
upstream, and the combined tracker routes by shape.)

**Fix.** Route every key-taking `jiraDeps` seam through one guard that returns
`jira.ErrNotFound`.

**Done when.** A wiring test with a Jira-only config asserts assign and
transition on "42" return `ErrNotFound` and send no request.

### DEBT-295 The forge's kind and group seams are fixed at start-up though forge settings are live

Severity: low · Confidence: read · Size: M

**Where.** `forgeDeps` (`internal/wiring/forge.go:74`), `groupMembersSeam` and
`isGroupSeam` (`:147`, `:169`), `useForgeSettings` (`:349`), `WebDeps`
(`internal/cli/web.go:93`), `branchOwners`
(`internal/webserver/people.go:227`).

**Today.** `forgeDeps` decides the kind once and binds `Kind`, `GroupMembers`
and `IsGroup` (nil unless GitLab) from it, while a web Settings save replaces
the live settings for every connect. Switching to GitLab in Settings leaves
`IsGroup` nil, so top-level groups are tagged as people until restart; the
group cache is keyed by name only and survives a host change.
`Controls.UseForgeSettings` documents the opposite.

**Fix.** Decide the kind inside each closure from the live settings, as
`gitDeps` does, and key the cache by host and kind.

**Done when.** A wiring test starts with GitHub, applies GitLab settings and
sees `IsGroup` reach the fake GitLab endpoint.

### DEBT-296 Linking a branch writes the sanitized pull request description back to the forge

Severity: low · Confidence: read · Size: M

**Where.** `branchLinker.choose` and `link` (`internal/tui/branchlink.go:231`,
`:252`), `namePullIssue` and the preview
(`internal/webserver/branchlink.go:166`, `:104`), `Client.exchange`
(`internal/forge/client.go:278`), `sanitize.JSON`.

**Today.** Both surfaces build the new description from the body as read,
which `sanitize.JSON` has already neutralized (controls and bidi marks
replaced, every CR dropped), and send it and the title back. Linking one issue
rewrites the author's whole description, silently: bidi marks in right-to-left
text become U+FFFD and CRLF bodies become LF.

**Fix.** Have the edit seam re-read the raw body, or apply "insert the issue
line" on the forge side, keeping the sanitized copy for display.

**Done when.** A test with a body containing U+202E and CRLF sends it back
unchanged but for the issue line.

### DEBT-297 A job log is read whole under the ten-second client timeout

Severity: low · Confidence: read · Size: M

**Where.** `JobLog`, `logResponse`, `followLog`, `tailOf`, `lastBytes`
(`internal/forge/cijobs.go:95`, `:134`, `:184`, `:206`, `:228`),
`httpx.Client` (`internal/httpx/httpx.go:117`), `RequestTimeout`
(`internal/wiring/wiring.go:43`).

**Today.** The log is read to EOF on purpose, through a client whose timeout
covers the body, so a log of tens of megabytes on an ordinary link fails with
"Client.Timeout exceeded" exactly where its tail matters. Memory stays bounded
and `timing.request_timeout` is a workaround.

**Fix.** Request only the tail with a `Range` header, or give log reads a
client with a per-read deadline instead of a whole-request timeout.

**Done when.** A test serving a slow 50 MB log through a one-second client
returns its last lines.

### DEBT-298 Response handling is written seven times across the clients, and the copies disagree

Severity: low · Confidence: read · Size: M

**Where.** `Client.exchange` (`internal/jira/jira.go:170`), `Client.exchange`
(`internal/forge/client.go:254`), `Client.send` and `deliver`
(`internal/messaging/messaging.go:231`, `internal/messaging/post.go:246`),
`Refresher.ask` (`internal/slackauth/refresh.go:98`), the `bodyLimit` and
`reasonLimit` constants.

**Today.** Each reads a capped body by hand. The copies drift: the forge
refuses a non-JSON answer with an actionable message (`mustBeJSON`) and Jira
does not, so a 200 HTML login page reads as "invalid character '<'";
`slackauth.ask` reads no status, so a 429 surfaces as a refused refresh or
"not JSON" rather than `httpx.ErrRateLimited`. (Each service's own
classification and its distinct sentinels are intended.)

**Fix.** An `httpx.Read(response, limit)` that detects overflow (DEBT-299), a
Content-Type check for Jira, and status and 429 handling in `slackauth.ask`.

**Done when.** A Jira test answering 200 HTML gets the actionable message, and
slackauth's 429 test yields `httpx.ErrRateLimited`.

### DEBT-299 An answer past the body limit is silently cut and reads as a JSON syntax error

Severity: low · Confidence: read · Size: S

**Where.** `internal/jira/jira.go:182`, `internal/forge/client.go:271`,
`internal/messaging/messaging.go:246`, `internal/messaging/post.go:253`.

**Today.** `io.ReadAll(io.LimitReader(body, limit))` truncates without saying
so, and decoding then reports "unexpected end of JSON input", so the user is
told the server sent bad JSON rather than that the answer was too large.
Slack's 1 MiB cap is the likeliest to be met.

**Fix.** Read limit + 1 bytes and return an `ErrAnswerTooLarge` sentinel on
overflow, in the shared reader.

**Done when.** A test serving limit + 1 bytes of valid JSON gets
`errors.Is(err, ErrAnswerTooLarge)`.

### DEBT-300 Messaging's `ErrRejected` blames the credential for any 4xx, including a bad message

Severity: low · Confidence: read · Size: S

**Where.** `ErrRejected` (`internal/messaging/messaging.go:47`), `deliver`
(`internal/messaging/post.go:263`), `configurationErrors`
(`internal/cli/scriptable.go:381`).

**Today.** Every webhook 4xx becomes "the credential was not accepted", so a
Discord 400 for a message too long or a Teams 400 for a bad payload exits with
the configuration family and advises rotating a working webhook. Slack's own
path keeps the two apart.

**Fix.** Map 401, 403 and Slack's credential codes to `ErrRejected`, and other
4xx to `ErrPostRefused` with the capped reason.

**Done when.** A webhook test answering 400 gets `ErrPostRefused`, not
`ErrRejected`.

### DEBT-301 A failed group-member read is reported as "no such user"

Severity: low · Confidence: read · Size: S

**Where.** `gitlabReviewers.addGroupNamedAsUser`
(`internal/forge/members.go:155`), `IsGroup` (`:74`), `addTeam` (`:167`).

**Today.** Any error from the group read, including a rate limit, a 403 or a
cancel, is recorded as `ErrNoUser`; `IsGroup` and `addTeam` keep the real
error. A transient failure sends the user hunting a typo.

**Fix.** Use `ErrNoUser` only for `ErrNoAPI`, and the error itself otherwise.

**Done when.** A test whose group read answers 429 records
`httpx.ErrRateLimited` in the missed-people chain.

### DEBT-302 Discard and Unstage treat any failed HEAD probe as an unborn branch

Severity: low · Confidence: read · Size: S

**Where.** `Unstage` (`internal/gitrepo/status.go:208`), `discardArgs`
(`:243`), `readFailure` (`internal/gitrepo/gitrepo.go:64`).

**Today.** Both run `git rev-parse --verify --quiet HEAD` and on any error
take the no-commit-yet path, `git rm --force` or `git rm --cached`. A probe
that times out (the 30 s per-command bound) while the next command succeeds
makes Discard of a committed file delete it and stage the deletion, instead of
restoring it. The probe is written twice and does not use `readFailure`.

**Fix.** One `hasHead(ctx) (bool, error)` that answers an error for a timeout
or a missing git and false only for git's "no such ref".

**Done when.** A test whose HEAD probe answers `proc.ErrTimedOut` sees Discard
return an error and run no `rm`.

### DEBT-303 `proc.Run` hides a caller's cancel that `Capture` reports, and `Failure` re-parses its own message

Severity: low · Confidence: read · Size: S

**Where.** `runWithin` and `captureWithin` (`internal/proc/proc.go:72`,
`:118`), `Failure` (`:157`),
`TestRunWithinLeavesATighterParentDeadlineUnclaimed`
(`internal/proc/proc_test.go:38`), `ExitStatus` handling
(`internal/cli/cli.go:146`), the taskwarrior callers
(`internal/taskwarrior/client.go:204`).

**Today.** `captureWithin` returns `context.Cause` once the context is done;
`runWithin` wraps exec's "signal: killed", so `errors.Is(err,
context.Canceled)` is false for every git read, and the CLI patches it back at
the top. Both format `"%s: %w: %s"` and `Failure` recovers stderr with
`strings.Cut` on the message, so taskwarrior's refusal classification depends
on a format string in another file. The Run test passes for any error.

**Fix.** Have `runWithin` call `captureWithin`, and return a typed
`ExitError{Program, Code, Stderr}` that `Failure` reads with `errors.As`;
tighten the test to `context.DeadlineExceeded`.

**Done when.** A test asserts `RunWithin` answers a caller's cancel as
`context.Canceled`, and `Failure` holds no `strings.Cut`.

### DEBT-304 Hook generation leaves directories behind, drops set options and skips hooks that mention lefthook

Severity: low · Confidence: read · Size: M

**Where.** `Write` and `remove` (`internal/hooks/generate.go:387`, `:397`),
`filesIn` and `TestAWriteCutShortLeavesNoFileBehind`
(`internal/hooks/write_unix_test.go:43`, `:66`), `plainCommands`, `setOption`
and `errexitOption` (`generate.go:254`, `:283`, `:291`), `ExistingHooks`
(`:87`).

**Today.** A write cut short removes its files but not the `.lefthook/<hook>`
directories, and the test meant to prove "nothing behind" skips directories.
Conversion accepts any `set -flags` line and discards it, so `set -ef`
(noglob) or `set -en` (noexec) converts and the glob expands or commands now
run; gobco confirms that case is untested. And a hand-written hook that merely
mentions "lefthook" (a comment, an `npx lefthook` step) is treated as
lefthook's own shim and left out of the generated configuration.

**Fix.** Record and remove the directories `MkdirAll` made, and list them in
the test; accept only errexit-compatible set lines (`e`, `u`, `x`); recognize
lefthook's shim by its header or `call_lefthook`, not the word.

**Done when.** The cut-short test asserts an empty directory, `set -ef` and
`set -en` scripts stay scripts, and a hook with only a lefthook comment
appears in `ExistingHooks`.

### DEBT-305 A relative `XDG_STATE_HOME` puts the store under the working directory, which db-clean refuses

Severity: low · Confidence: read · Size: S

**Where.** `Dir` (`internal/store/dir.go:24`, `:39`), `Clean`
(`internal/store/clean.go:222`).

**Today.** `Dir` joins `XDG_STATE_HOME` and `AppData` without checking they
are absolute (the XDG spec says to ignore a relative value), so the store is
made relative to wherever workflow started, possibly inside a working tree,
and `db-clean` refuses it as relative.

**Fix.** Ignore a non-absolute value and fall back to the home-based default.

**Done when.** `Dir("linux", "/home/u", XDG_STATE_HOME="state")` returns
`/home/u/.local/state/workflow`.

### DEBT-306 The Jira key shape is written three times, and the web's copy disagrees

Severity: low · Confidence: read · Size: S

**Where.** `issueKey` and `wholeJiraKey`
(`internal/convention/convention.go:56`, `:171`), `forgeNumber` (`:199`),
`forgeIssueNumber` (`internal/convention/pullrequest.go:120`), `jiraKey`
(`web/src/features/repositories/useFollowSwitch.ts:8`).

**Today.** Go spells `[A-Z][A-Z0-9_]+-[1-9][0-9]*` twice; the web's
`^[A-Z][A-Z0-9_]*-\d+$` accepts `A-1` and `PROJ-01`. Within convention,
`forgeNumber` requires no leading zero while `forgeIssueNumber` accepts any
digits, so `0` would be worded "Closes #0". Every caller parses through the
strict rule first, so no user reaches the difference today.

**Fix.** Build both Go regexps from one constant, keep one forge-number
predicate, and expose the selected issue's tracker in the API so the web stops
re-deriving it.

**Done when.** One Go constant and no TypeScript copy of the key shape remain,
and `PullRequestBody` with key "0" no longer writes "Closes #0".

### DEBT-307 The CLI imports the web server and the terminal package for logic every surface shares

Severity: low · Confidence: read · Size: L

**Where.** `notSetUpNote` (`internal/cli/status.go:372`), `runSummary`
(`internal/cli/summary.go:142`, `:182`), `internal/cli/repositories.go:53`,
`internal/cli/scriptable.go:382`, `internal/cli/doctor_requirements.go:42`;
`FaultDetail`, `ActivityReport` and `PostLength`
(`internal/webserver/activity.go:282`, `:149`, `:46`), `RepositoriesView`
(`internal/webserver/repositories.go:169`), `CheckKeys`
(`internal/tui/keycheck.go:48`).

**Today.** The CLI's not-set-up wording, its summary and repositories reports
and the summary's length come from the web server package, and its keymap
check from the TUI. Sharing the shape and the host-free wording is intended
(the `--json` output is the API's object); where it lives is the debt:
surface-neutral wording and report building sit in an HTTP package already at
its budget. (The `tui.Deps` bundle is TRADE-11.)

**Fix.** Move `FaultDetail`'s wording and the report builders beside
`internal/api` in a small report package both import, and `CheckKeys` with its
sentinels beside `config.UI`.

**Done when.** `internal/cli` imports `internal/webserver` only for serving
`--web`.

### DEBT-308 `WebDeps` copies about sixty seams one by one from `tui.Deps` to `webserver.Deps`

Severity: low · Confidence: read · Size: L

**Where.** `WebDeps` and the `with*` helpers (`internal/cli/web.go:70`,
`:135`), `webserver.Deps` (`internal/webserver/webserver.go:46`),
`TestWebDepsHandsTheServerEverySeam` and
`TestWebDepsHandsTheServerTheLenientSearch`
(`internal/cli/webdeps_test.go:16`, `:68`).

**Today.** Each new seam is an edit in three places, split into six helpers
only to pass funlen. The test catches a nil seam, not two same-typed seams
wired crosswise (Stage, Unstage and Discard; Branches and RemoteBranches;
Commit, Push, Rebase and RunHook), so each such pair needs its own test, as
the searches did.

**Fix.** Have `webserver.Deps` hold the same seam groups `tui.Deps` uses,
narrowed per group where interface segregation asks, or build both bundles
from one source in wiring.

**Done when.** `WebDeps` is under 25 lines and a seam added to a group needs
no edit in `internal/cli`.

### DEBT-309 One concept, several spellings: CI words, glyphs, finish commands, sizes and settings

Severity: low · Confidence: read · Size: M

**Where.** `statusGlyph` and `ciWord` (`internal/cli/status.go:536`, `:562`),
`unicodeGlyphs` and `asciiGlyphs` (`internal/tui/glyphs.go:35`), `ciWord`
(`internal/tui/reviewfacets.go:56`), `ciState`
(`internal/webserver/dto.go:258`); `finishPreview.commands`
(`internal/tui/finish.go:78`), `web/src/features/review/PullActions.tsx:360`,
`FinishBranch` (`internal/gitrepo/branching.go:119`); `humanBytes`
(`internal/tui/localdata.go:135`, `internal/cli/dbclean_cmd.go:154`,
`web/src/features/settings/people/LocalData.tsx:111`) and the removal wording
(`internal/tui/localdata.go:206`); `settingsForm.settings`
(`internal/tui/settingsfields.go:81`) and
`web/src/features/settings/fieldsets/*.tsx`.

**Today.** The CI-state words are mapped three times and the stage glyphs
twice, though the CLI's line claims to mirror the spine. The finish preview's
commands, which a destructive `-D` is confirmed against, are written in the
TUI and the web apart from what gitrepo runs. Byte sizes and the removal
consequences are written in the TUI, the CLI and the web. The terminal's
Settings form copies the web's labels and hints by hand (already drifting),
restates owners' defaults as prose ("0 keeps 72", "empty keeps 20s") and
addresses settings by dotted strings the compiler cannot check.

**Fix.** A `Word()` on the CI state and a `Glyph(ascii)` on the progress
state; `gitrepo.FinishCommands(base, branch)` used by the runner and the TUI
and served to the web; one `humanBytes` and consequence wording in
`internal/store`; a test that every settings path resolves to a
`config.Config` json tag, with hints built from the owning constants (one
shared settings table is the larger option).

**Done when.** Non-test Go holds one `◐` literal and one CI-word map,
`ff-only` appears in `internal/tui` and `web/src` only through the shared
source, one `humanBytes` exists in Go, and the settings-path test passes.

### DEBT-310 The wiring's adapters and types are written several ways

Severity: low · Confidence: read · Size: M

**Where.** `askJira` and `tellJira` (`internal/wiring/jira.go:100`),
`askTaskwarrior` and `tellTaskwarrior` (`internal/wiring/tasks.go:95`), the
fourteen `connect()` arms and the funlen helpers of
`internal/wiring/forge.go`; `SlackTarget` and `OwnerLink`
(`internal/loop/tags.go:18`, `:27`; `internal/messaging/directory.go:91`;
`internal/store/owners.go:49`), `fromStoreLinks` to `toStoreTargets`
(`internal/wiring/kept.go:116`), `asLoopTargets`
(`internal/wiring/slackdirectory.go:420`); `SlackDirectory`
(`internal/wiring/slackdirectory.go:39`).

**Today.** Jira and Taskwarrior each define the same generic "get the client,
then call" pair, and `forge.go` hand-rolls it fourteen times, with four
helpers whose only reason is "keep forgeDeps within its length".
`loop.SlackTarget` is identical to `messaging.SlackTarget`, and loop already
imports messaging; the `OwnerLink` copies are converted field by field.
`SlackDirectory`, 455 lines of single-flight, expiry and rate-limit patience,
is behavior rather than binding. (The store's own plain-string types are a
deliberate boundary and stay.)

**Fix.** One `ask[C, T]` and `tell[C]` in wiring used by all three; make
`loop.SlackTarget` an alias of messaging's and delete `asLoopTargets`; move
`SlackDirectory` to a `messaging/directory` subpackage (messaging itself would
cycle through loop).

**Done when.** `askJira` and `askTaskwarrior` are gone, `forge.go` holds at
most two hand-rolled connects, and `internal/wiring` has no `SlackDirectory`
type.

### DEBT-311 Store write seams return no error, and the announcement memory trusts its moments

Severity: low · Confidence: read · Size: M

**Where.** `RecordScope`, `RecordAnnounce`, `CacheIssues`
(`internal/seams/seams.go:213`), `storeDeps` (`internal/wiring/wiring.go:340`
to `:381`), `LastGroups` (`internal/wiring/kept.go:40`),
`AnnounceMemory.Holds`.

**Today.** The seams have no error result, so every failure is dropped with `_
=`, including the announcement record: on a full disk or locked file an
announcement is not remembered and the next session posts again with no
warning. A disabled store no-ops by design; a failing one looks the same. The
stored moment is cast without validation (harmless, since it can only fail to
match).

**Fix.** Give the write seams an error result that surfaces can note, and drop
rows with an unknown moment on read.

**Done when.** `grep '_ = kept\.' internal/wiring` returns nothing, and a test
shows an out-of-range moment row is not returned.

### DEBT-312 Loop's contracts are uneven: a nil post panics, setup advice lives in the Summary, and Merge's doc is wrong

Severity: low · Confidence: read · Size: S

**Where.** `PostSummary` (`internal/loop/summary.go:405`), `Deliver` and
`Push` (`internal/loop/announce.go:218`, `internal/loop/push.go:48`),
`NotSetUp`, `SetUpAdvice` and `setUpCauses` (`internal/loop/summary.go:76`,
`:87`, `:107`), `Merge` (`internal/activity/item.go:135`), `CommitsRead`
(`internal/loop/summary.go:51`).

**Today.** `Deliver` and `Push` refuse a nil seam with a sentinel;
`PostSummary` calls it and panics, so each surface keeps its own guard. The
advice every surface uses to word missing setup lives in the Summary's file
and is tested in `setup_test.go` with no `setup.go`. `Merge` documents
deduplicating a commit shared by two repositories, which `CommitsRead` already
does by full hash; Merge's own check by ref can never match across
repositories.

**Fix.** Return a sentinel from `PostSummary` for a nil post; move the setup
advice to `loop/setup.go`; drop Merge's dedupe and its doc claim.

**Done when.** `PostSummary(nil, …)` returns the sentinel, `summary.go` holds
only Summary reads and posting, and Merge's doc matches it.

### DEBT-313 The forge's GitHub and GitLab code repeats small rules

Severity: low · Confidence: read · Size: M

**Where.** `addUser`, `gitlabKnownIDs`, `gitlabResolveAssignees`
(`internal/forge/members.go:142`, `:239`, `:266`), `strings.Cut(RepositoryURL,
"/repos/")` (`internal/forge/github.go:138`, `internal/forge/activity.go:88`,
`:150`), `githubReviewed` (`activity.go:150`), `runState` and `runFailed`
(`internal/forge/ci.go:104`, `internal/forge/githubci.go:174`), `pattern`
(`internal/codeowners/match.go:17`, `:124`), `github.go` (539 lines).

**Today.** A GitLab username is resolved to its id three times (the
slice-shaped helper serves one single-name caller; gobco shows its error paths
never run). The repository is cut out of GitHub's `repository_url` three
times, and `githubReviewed` rebuilds the pull path by hand and reads one page
of 100 reviews, missing any past it. The passing-conclusion set is declared
twice with a comment promising they agree. The codeowners pattern carries
fields for each dialect, chosen by a non-empty string rather than the dialect.
`github.go` holds pulls, issues, the review queue, reviewers and merge.

**Fix.** One `gitlabUserID`; one repository accessor and `githubPullReviews`
in `githubReviewed`; one `passingConclusion`; one matcher per dialect; split
`githubpeople.go` and `githubissues.go`.

**Done when.** `members.go` resolves ids in one place, one `strings.Cut` on
`RepositoryURL` remains, a test with 101 reviews counts the last, `"neutral"`
appears once outside tests, and `github.go` is not flagged soft.

### DEBT-314 `post.go` holds both transport and rendering, and the Markdown escape lists must stay inverse

Severity: low · Confidence: read · Size: S

**Where.** `Post` to `deliver` and `Moment` to `slackEscape`
(`internal/messaging/post.go:87`, `:271`), `markdownEscape` (`post.go:362`),
`markdownUnescape` (`internal/messaging/markdown.go:102`), `markdownEscaper`
(`internal/activity/text.go:88`).

**Today.** The second half of `post.go` is the announcement's rendering, which
`markdown.go` shares, and its tests are the bulk of `post_test.go` (DEBT-241).
Three hand-kept tables list the same thirteen Markdown metacharacters, two
escaping and one unescaping across packages; a character added to one leaves
stray backslashes in Slack and webhook summaries, and no test ties them.

**Fix.** Move `Announcement` and the markup helpers to `announcement.go`;
derive the tables from one list and add a round-trip test.

**Done when.** `post.go` is transport only, and a test asserts unescape of
escape returns its input for every metacharacter.

### DEBT-315 Unused and unreachable code in config, messaging and taskwarrior

Severity: low · Confidence: measured · Size: S

**Where.** `LoadFile`, `LoadFileAt`, `parseFile`
(`internal/config/load.go:100`, `:115`, `:135`), `RevisionOf`, `SaveOver`
(`internal/config/save.go:72`, `:133`), `ErrChangedOnDisk`'s doc
(`save.go:20`); `Client.Workspace` (`internal/messaging/messaging.go:165`);
`Task.State` (`internal/taskwarrior/order.go:51`), `Facet.Label` and `noneOf`
(`internal/taskwarrior/narrow.go:52`, `:65`).

**Today.** `deadcode` reports the five config functions and `Client.Workspace`
unreachable outside tests; the config ones carry four nolints and tests that
pin dead behavior, and `ErrChangedOnDisk` is still called "SaveOver's
refusal". `State`'s `case Waiting` can never run, and `noneOf` indexes a fixed
array by an exported int, panicking for an unknown kind.

**Fix.** Delete them, moving any valuable config cases onto `SaveLayers`; drop
the dead case; make `noneOf` a switch returning "" for an unknown kind.

**Done when.** `deadcode ./...` reports nothing in these packages, and
`Facet{Kind: 9}.Label()` does not panic.

### DEBT-316 `priorityRank` needs three `//nolint:mnd`, and the H, M, L order is written twice

Severity: low · Confidence: measured · Size: S

**Where.** `priorityRank` (`internal/taskwarrior/order.go:161`),
`offeredFacets` (`internal/taskwarrior/narrow.go:236`).

**Today.** Ranks 2, 4 and 3 are literals behind three suppressions, and the
order is also a literal slice in `offeredFacets`.

**Fix.** One ordered list, ranked by `slices.Index` with named ranks for other
and none, used by both.

**Done when.** `grep nolint internal/taskwarrior/order.go` returns nothing and
the order and narrow tests pass.

### DEBT-317 Small inconsistencies in gitrepo, workdirs, the forge's token and db-clean

Severity: low · Confidence: read · Size: S

**Where.** `gitProgram` (`internal/gitrepo/branch.go:23`) against the literals
in `gitrepo.go`, `branch.go`, `status.go` and `branching.go:26`; `List`
(`internal/workdirs/workdirs.go:52`); `ReachForge`
(`internal/wiring/forge.go:437`); `databases` (`internal/store/clean.go:81`).

**Today.** `gitProgram` is "the program every command here runs", yet a dozen
calls pass `"git"`. `List` stats every entry and checks `.git` in each
directory before cutting to its limit. With `forge.cli` on, `ReachForge` still
resolves a real token (possibly running `gh auth token`) that the CLI
transport never uses. db-clean's table summary restates the schema by hand and
already misses `repo_choice`.

**Fix.** Use `gitProgram` everywhere; filter by name and `DirEntry.Type()`
first and check `.git` only for kept entries; resolve only without the CLI
transport; add a test that every `CREATE TABLE` has a summary entry.

**Done when.** No `"git"` program literal remains in `internal/gitrepo`, a
10k-entry benchmark makes O(limit) `.git` checks, a `forge.cli` test runs no
token command, and a seeded `repo_choice` shows in the summary.

### DEBT-318 The messaging package's `users.info` lookup and `Grant.Lacks` have no test of their own

Severity: low · Confidence: measured · Size: S

**Where.** `Client.User` (`internal/messaging/directory.go:280`),
`Grant.Lacks` (`internal/messaging/messaging.go:102`).

**Today.** gobco lists every condition in both as never evaluated by the
package's tests. `User` is the large-workspace fallback whose checks for an
untaggable or mismatched user guard who can be tagged; the mismatch and
deleted-user branches are tested nowhere.

**Fix.** Black-box tests: `users.info` answering a bot, a deleted user and
another id, and `auth.test` with and without scopes.

**Done when.** gobco no longer lists those conditions.

### DEBT-319 Pay down TRADE-15: the store opens its driver without a lookup that can fail

Severity: low · Confidence: read · Size: S

**Where.** `openDatabase` (`internal/store/store.go:306`), `openAsItIs`
(`:434`), `openKept` (`internal/store/kept.go:132`), the DSN constants
(`store.go:72`, `kept.go:25`).

**Today.** `sql.Open("sqlite", dsn)` is written three times, past the rule of
three, and the register names two (with a stale third line). Each keeps an
error arm only a missing driver registration can reach. The writing opens
build plain-path DSNs while `openAsItIs` already escapes its path into a
`file:` URI.

**Fix.** One `connect(dsn) *sql.DB` returning `sql.OpenDB` over a small
`driver.Connector` that opens through `(&sqlite.Driver{}).Open`, with `var _
driver.Connector = connector{}`; build all three DSNs as escaped `file:` URIs
through one helper, with a test first that a store directory containing `?`
keeps its file inside that directory and foreign keys on. Paying this closes
TRADE-15: delete the entry and its site comments.

**Done when.** `rg 'sql.Open\(' internal/store` finds nothing and `task check`
is green.

### DEBT-320 Pay down TRADE-16: one call where two made a race, and one transaction helper

Severity: low · Confidence: measured · Size: M

**Where.** `readVersion` and `holdsTables` (`internal/store/store.go:374`),
`CacheIssues` (`internal/store/cache.go:108`), `keptWithin` and
`makeKeptSchema` (`internal/store/kept.go:62`, `:181`), `followDanglingLink`
and `linkDestination` (`internal/config/save.go:250`, `:277`), `stamp` and
`removeDatabase` (`store.go:402`, `:415`).

**Today.** Most of TRADE-16's arms exist because two calls are made where one
would do: `holdsTables` re-queries the schema `readVersion` just loaded;
begin, rollback and commit are written three times; `followDanglingLink` stats
and then reads the link. gobco also lists many untested `kept.go` arms no
entry covers.

**Fix.** Read the version and whether tables exist in one statement; add
`inTransaction(ctx, db, write)` used by all three, with a test that a canceled
context fails a kept write; read the link first, treating not-exist or EINVAL
as "the path itself". Shrink TRADE-16 to `stamp` and `removeDatabase`, cited
by name.

**Done when.** gobco no longer lists `cache.go:111`, the `holdsTables` arm or
the two link arms, and TRADE-16 names only `stamp` and `removeDatabase`.

### DEBT-321 Pay down TRADE-21 and TRADE-28: twin rules read one shared case file

Severity: low · Confidence: read · Size: M

**Where.** `internal/tui/issueplaces.go`,
`web/src/features/issues/issuePlaces.ts` and their tests;
`jira.WikiFromMarkdown` (`internal/jira/wiki.go`),
`web/src/features/issues/wiki/wikiFromMarkdown.ts`, `fenceLine`, and their
tests.

**Today.** Both trade-offs keep rules in Go and TypeScript, for good reasons
(marks come from reads the browser makes separately; Preview converts on every
keystroke), but their stated cost, "a change made to one copy alone passes
that copy's tests", is cheap to remove. The cases are written twice in
different shapes, and nothing checks the twins hold the same data. The wiki
converters already differ where the languages do: Go's `TrimSpace` trims
U+0085 and JavaScript's `trim` U+FEFF, so a fence line ending in either gets a
different `{code:lang}`. With TRADE-29 and TRADE-33 the pattern is past the
rule of three.

**Fix.** A JSON case corpus per rule set (`testdata/twins/places.json`,
`internal/jira/testdata/wiki_from_markdown.json`), read by a Go test and by
the TypeScript test through `readFileSync`, replacing both hand tables; move
the place rules into a small exported package so the Go test is black-box; add
the two fence cases and align the trims. Rewrite each entry with its cost as
"made twice, against one shared case file that fails whichever copy
disagrees"; reuse the loader for TRADE-33 (TRADE-29 is paid by DEBT-328).

**Done when.** Editing either copy alone fails its own suite against the
shared corpus, and `task check` and the web unit tests are green.

### DEBT-322 Pay down TRADE-22: the Slack refresh lock is an operating-system lock

Severity: low · Confidence: read · Size: M

**Where.** `tryLock`, `lockStale` and the unlock
(`internal/slackauth/lock.go:67`, `:72`), `Source.Token`
(`internal/slackauth/source.go:20`, `:43`), `go.mod`,
`scripts/gobco-report.sh`.

**Today.** Beside the race TRADE-22 records, the unlock is a bare
`os.Remove(path)`: a holder that runs past a minute (a keychain write waiting
on a prompt) has its lock taken over as stale, and its unlock then deletes the
next holder's, so a third process refreshes alongside and spends a single-use
refresh token. A lock left by a crash blocks every refresh for a minute while
waiters give up after ten seconds. `Source`'s doc ("two processes never spend
the same refresh token") overclaims.

**Fix.** Hold an advisory lock on an open file: `lock_unix.go` with
`syscall.Flock` and `lock_windows.go` with `windows.LockFileEx`, polled until
the wait ends; unlock by closing, never removing. Delete `lockStale`, the
takeover and its test, and add tests that a lock file left by no live holder
is taken at once and a holder past a minute keeps its lock. Promote
`golang.org/x/sys`, already required indirectly, to a direct requirement, and
name the build-tagged twin in `UNANALYZABLE` if gobco cannot read it. Paying
this closes TRADE-22: delete the entry and its site comment.

**Done when.** Cross-compiling to every `RELEASE_PLATFORMS` target succeeds,
TRADE-22 is gone, and `task check` is green.

### DEBT-323 Pay down TRADE-23: the server describes each review request's facets

Severity: low · Confidence: read · Size: M

**Where.** `internal/tui/reviewfacets.go:41`,
`web/src/features/reviewqueue/reviewFacets.ts:28`, `ReviewRequest` and
`ReviewQueue` in `api/openapi.yaml` (`:4119`), the reviews handler.

**Today.** Every facet input is a plain field of a request the server already
returns, so describing facets once per queue read costs nothing; TRADE-23's
objection is to filtering on the server, not describing. The facet rules
(labels, the CI order, draft before ready, sorting by name) are written twice,
and can already disagree: Go sorts by UTF-8 bytes and JavaScript by UTF-16
units. The twin pin is three TypeScript tests against nineteen Go ones.

**Fix.** Move the rules to an exported domain home (`FacetsOf`, `Label`,
`Offered`) the TUI calls; add `facets` to `ReviewRequest` and `facet_order` to
`ReviewQueue`, regenerate, and fill them in the handler; cut the TypeScript to
counting, toggling and admitting. Move ordering and label cases to Go. Paying
this closes TRADE-23: delete the entry and both site comments.

**Done when.** No facet word, order or label is spelled in `web/src`, and
`task check` and the web unit tests are green.

### DEBT-328 Pay down TRADE-29: the server ranks and describes each task

Severity: low · Confidence: read · Size: M

**Where.** `ByUrgency` to `ByPriority` and `Task.State`
(`internal/taskwarrior/order.go:72`, `:40`), `Task.Facets`, `Choices`,
`offeredFacets` and `Task.mentions` (`internal/taskwarrior/narrow.go`),
`orderedTasks` and `stateOf` (`web/src/features/tasks/taskOrder.ts:47`,
`:35`), `taskFacetChoices`, `matchesNarrowing` and `taskFacetLabel`
(`web/src/features/tasks/taskFacets.ts`), `Task` and `TaskList` in
`api/openapi.yaml` (`:4227`, `:4146`), `internal/webserver/tasks.go`.

**Today.** Six orders, the state as the list words it, five facet kinds with
their labels and offered order, and the typed-text match are written in Go and
again in TypeScript, about 360 lines on the web. TRADE-29's objection is to
asking the server for each order, not to the server describing each task: every
input is a field of a task the server already sends, and the read and all ten
task writes answer with the same list type, so ranks shipped with it apply at
once with no request and nothing to race. The copies already differ: Go has an
`unknown` state the web's `TaskState` lacks, and Go's `strings.ToLower` and
JavaScript's `toLowerCase` fold some letters differently (U+0130 becomes one
code point in Go and two in JavaScript), so the same typed text can match
different tasks.

**Fix.** Add to `Task` a required `state` (the list's word, from `Task.State`),
`facets` (kind, value, label, from `Task.Facets`), `ranks` (one integer per
order, the task's place in `ByUrgency` to `ByPriority`) and `searchable` (the
fields text matches, lower-cased by Go); add `facet_order` to `TaskList`, the
offered facets from `offeredFacets`; run `task gen` and `yarn gen`, and fill
them where the handler builds the list. Cut the TypeScript to sorting by
`ranks[order]`, counting and toggling facets, and an `includes` over
`searchable`. Since a wait passing changes a task's state with no event,
invalidate the tasks query at the earliest `wait` still ahead. Move the order
and facet cases to Go. Paying this closes TRADE-29: delete the entry and its
four site comments, and drop TRADE-29 from DEBT-321's fix.

**Done when.** `rg 'priorityRank|naturalOrder|stateRank' web/src` finds
nothing, no facet label is spelled in `web/src/features/tasks`, and `task
check` and the web unit tests are green.

## Rules this audit changes

### DEBT-324 Declare compiled regular expressions at package level

Severity: low · Confidence: read · Size: M

**Where.** CLAUDE.md, *Design principles*, O — Open/closed; 41 non-test
`regexp.MustCompile` calls under `internal/`, among them `trailerLine`
(`internal/convention/convention.go:398`, called per body line), `issueKey`
(`:55`), `setOption` and `errexitOption` (`internal/hooks/generate.go`, per
script line), `summaryLine` (`internal/hooks/output.go:87`, per hook output
line), `versionPattern`, `createdTask`, `revertedOperations`
(`internal/taskwarrior/taskwarrior.go:315`, `write.go:19`, `:25`),
`pseudoVersion` (`internal/buildinfo/buildinfo.go:46`).

**Today.** The Open/closed note says `gochecknoglobals` makes a package-level
lookup map a build failure, and authors extended that to regexps, wrapping
each in a `func x() *regexp.Regexp` compiled on every use, some in per-line
loops. The pinned gochecknoglobals allows `regexp.MustCompile` globals, and
Effective Go and Google's style both declare them once.

**Fix.** Add to the Open/closed note: "A compiled regular expression is the
exception the linter itself makes: declare it once as a package-level `var
name = regexp.MustCompile(...)` rather than compiling it in a helper on every
call." Move every fixed pattern to a package-level var, keep the func form
only for patterns built from run-time input, and hoist those out of loops.

**Done when.** `grep -rn 'func .*\*regexp.Regexp' internal | grep -v _test`
returns only parameterized builders and `golangci-lint run` is clean.

### DEBT-325 Name the e2e suites in the definition of done

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Before declaring any task done* and the accessibility
paragraph; `check` (`Taskfile.yml:578`).

**Today.** CLAUDE.md says the axe scan and the layout spec fail on violations
and that `task check` must pass before work is done, but `task check` leaves
out `yarn test:e2e` on purpose (a browser, a port), and no task wraps it. A
web change can be called done locally with the accessibility floor never run;
CI catches it after.

**Fix.** Add `task e2e` wrapping `yarn test:e2e`, and change the rule to: "run
`task check`; for a change under `web/` or `internal/webserver` also run `task
e2e`, since `task check` leaves the Playwright suites out."

**Done when.** CLAUDE.md names the e2e task in the definition of done and the
task exists.

### DEBT-326 Mark the goroutine smell as gated

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Code smells*, Go-specific, "Premature
goroutines/channels"; `scripts/check-goroutines.sh`, `lint:goroutines`
(`Taskfile.yml:458`).

**Today.** The catalog marks a smell a linter catches, and lists this one as
judgment only, though a gate fails any `go` statement outside `internal/proc`
and the tests; a session writing a goroutine elsewhere learns the rule when
the build fails.

**Fix.** Change the bullet to: "*Premature goroutines/channels* — concurrency
with no measured need → simple synchronous code first; a bare `go` statement
is allowed only in `internal/proc` and tests (**gate**
`scripts/check-goroutines.sh`, `task lint:goroutines`)." Widen the gate as
DEBT-247 asks.

**Done when.** CLAUDE.md names `check-goroutines.sh` beside the smell.

### DEBT-327 Land Boy Scout improvements as their own commit

Severity: low · Confidence: read · Size: S

**Where.** CLAUDE.md, *Design principles*, Boy Scout Rule, and *Cross-cutting
conventions*, Commits.

**Today.** The Boy Scout Rule makes an improvement in every touched file part
of a feature or fix commit's definition of done, while the commit rule asks
for one logical change per commit and main merges by rebase, so each commit
lands as written. Read together, a fix commit carries unrelated cleanups, and
the bug-fix commit CLAUDE.md calls "the most valuable artifact" is harder to
read.

**Fix.** Append to the Boy Scout bullet: "Land the improvement as its own
commit (`refactor:`, `style:` or `chore:`) on the same branch, before or after
the feature or fix commit, so each commit stays one logical change."

**Done when.** CLAUDE.md says where the improvement goes and the two rules no
longer conflict.

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
TRADE-5, TRADE-7, TRADE-8, TRADE-9, TRADE-11, TRADE-13, TRADE-14, TRADE-24,
TRADE-25, TRADE-26 and TRADE-30 to TRADE-34 were kept and rewritten to what is
true at `b9ab000`. TRADE-1, TRADE-2, TRADE-6, TRADE-10, TRADE-12, TRADE-15 to
TRADE-19, TRADE-21 to TRADE-23, TRADE-28 and TRADE-29 are to be paid down by
the entries above whose titles name them, and each stays here, as it was,
until its entry is paid.

TRADE-27, a top-level GitLab group linking to Slack like a person, was closed
in #166: a bare CODEOWNERS name is now asked of GitLab when tags are composed,
and a group links to a Slack user group. A bare name decided as a person
before GitLab was asked, as every one was, is asked about again as a team once
GitLab knows it as a group; one GitLab cannot be asked about stays what it was
decided as. Its ID is not reused.

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
0, or `directoryHold`). The stream's connection state (`reconnecting`,
`stale`) is shown, so a dropped stream is visible. A configuration save checks
the file's revision and refuses a changed file with 409; a change landing
between that check and the write is not caught (`SaveOver`,
`internal/config/save.go`), which is accepted for one user's local file.
Recorded on 2026-09-24 in the audit (#140), and kept on 2026-10-07 in the
pre-1.0 audit.

**Cost.** A connection that stays open but stops sending frames looks live,
and leaves stale data with no refetch to fall back on.

**Reopen when.** A stalled-but-open stream is reported, or a panel is reported
showing stale data that neither the stream, its own `staleTime` nor a save
reads again.

### TRADE-5 The progress spine's per-system hue is color-only

**Decided.** On the progress spine each stage's glyph is drawn in its system's
hue (Jira, git, forge, messaging), inside `Model.stages` in
`internal/tui/spine.go`. The hue only repeats what the text says: the stage's
name, or its initial when compact, names the system, and the glyph's shape
carries its state. The hues are part of UX.md's visual system, which the web's
tokens share. Recorded on 2026-09-24 in the audit (#140), and kept on
2026-10-07 in the pre-1.0 audit.

**Cost.** A monochrome or color-blind reader loses one redundant channel
telling the systems apart.

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

**Decided.** `scripts/gobco-report.sh` names `internal/proc/pgroup` in
`UNANALYZABLE`. gobco ignores build tags, so it fails on the redeclared
`Isolate` in `pgroup_unix.go` and `pgroup_other.go`, and the Unix half's
syscalls cannot compile elsewhere, so the twin cannot be folded into one file.
The package holds only that glue, so `internal/proc` keeps its condition
coverage, and `proc.Start`'s grandchild-kill test exercises the Unix path end
to end. A package that becomes unreadable without being named fails the gate.
Recorded on 2026-09-24 in the audit (#140), and kept on 2026-10-07 in the
pre-1.0 audit.

**Cost.** pgroup's three `Cancel` conditions go unmeasured by gobco, and the
next package of tagged twins must join the list.

**Reopen when.** gobco reads build tags, or a second package whose files come
in tagged twins appears.

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

**Decided.** `wiring.Deps` returns `tui.Deps` (`internal/wiring/wiring.go`),
and `cli.WebDeps` narrows it to `webserver.Deps`. Three of its fields speak
Bubble Tea: `Editor` (`EditorDeps`, whose functions take and return Bubble Tea
messages and commands because `internal/editor` hands the terminal over
through `tea.ExecProcess`), `After` and `Copy`. So the bundle is the
terminal's type rather than one in package `seams`. Decided 2026-09-25 in #144
(DEBT-71), and kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** The wiring's output is named for one surface, and the web reaches
its seams through the terminal's struct; a second surface that wants the
editor must import `internal/tui`.

**Reopen when.** A surface other than the terminal needs the editor, `After`
or `Copy`; `internal/editor` stops returning a Bubble Tea command; or the web
server needs to be built without importing `internal/tui` (today it also needs
`tui.CheckKeys`).

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

**Decided.** Every `json.Marshal`, `MarshalIndent` or `Unmarshal` on a value
the program builds keeps its `err != nil` arm, though none can be true: the
values hold only strings, integers, booleans, slices and maps of the same, or
values just decoded from JSON, and a `config.Config`'s JSON always fits
`api.Config`. Each site carries a `Trade-off TRADE-13:` comment, and `rg
'TRADE-13' internal` lists them (16 at `b9ab000`, in config, forge, jira,
messaging, slackauth, tui and webserver); lines are not listed here because
they drift. Decided 2026-09-26 in #146, and kept on 2026-10-07 in the pre-1.0
audit.

**Cost.** About sixteen error arms no test runs. Because `forge.newRequest`,
`jira.newJSONRequest`, `messaging.postJSON` and `config.encodeValue` accept
`any`, a caller passing a float, a channel or a failing marshaler would put an
arm in play with no test behind it.

**Reopen when.** A type encoded at one of these sites gains a float, a
function, a channel, an interface-typed field or a `MarshalJSON` of its own;
`config.Config` and `api.Config` disagree on a field's type; or an encoding
failure is reported.

### TRADE-14 The embedded OpenAPI contract is taken to load

**Decided.** `loadSpec` and `validate` (`internal/webserver/validator.go`) and
`Handler` (`internal/webserver/webserver.go`) keep five `err != nil` arms for
an embedded `api/openapi.yaml` that fails to load, validate or route. The
document is compiled in (`api/embed.go`), so only a build-time defect trips
them, and that defect fails every web server test first; a seam to feed a
broken spec would only re-test kin-openapi. Decided 2026-09-26 in #146, and
kept on 2026-10-07 in the pre-1.0 audit.

**Cost.** Five arms no test runs; the start-up message for a broken contract
is read, not checked.

**Reopen when.** The contract is read from anywhere but the binary (a file, a
flag, a download), or a broken contract reaches a release.

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

**Revisited.** 2026-10-05, in #174: the web's copy now also draws a forge
issue's thread and Preview, converting its Markdown to wiki markup for
`WikiText` to draw. That reuses the drawing every Jira comment already has
rather than adding a Markdown renderer, at the cost of Markdown the
conversion does not know, such as a table, showing as text.

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
`internal/wiring/slackdirectory.go` reads `users.list` whole (Slack's Tier 2,
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
again in `web/src/features/summary/civilDate.ts`, pinned by twin-named cases
in `period_test.go` and `civilDate.test.ts`. The web's calendar moves on every
key (`MonthGrid`'s arrows, Page Up and Page Down, Home and End), so it cannot
wait on a request for arithmetic. The server still decides the default period
and reads every period, so only the moves are twinned, never which days were
worked. Decided 2026-10-05 in #175, and kept on 2026-10-07 in the pre-1.0
audit: nearly all of it is the Gregorian calendar, whose rules do not change,
and the one rule of workflow's own, that a whole month or year steps as one,
is a few lines.

**Cost.** A change to how a period steps is made twice, and a mistake in one
copy's leap-year or month-end arithmetic passes that copy's own cases until
DEBT-321's shared case file pins both.

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
