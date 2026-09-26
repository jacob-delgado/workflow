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

What is open here is the clients and plumbing every command shares (the
interface and the web reach them too), the drift between the docs, the
comments and the code they describe, and a sweep that crosses every
surface — tests that prove nothing.

### DEBT-89 Comments and layout rows that no longer say what the code does

Severity: low · Confidence: read

Across the terminal, the command line, the clients, the plumbing, the web
server and the two layout maps, doc comments and layout rows describe an
earlier shape of the code. No linter reads a comment, so every one passes
the gate. The `wiring` package comment below still names the terminal as
the owner of the seams it fills, which `internal/seams` now declares for
all three surfaces.

The terminal:

- `internal/tui/branch.go:236` — `Model.branchIssue`'s comment calls itself
  "the one place the interface reads a branch name as an issue key" while
  `branchDetail` (`:147`) and `taskBranches`
  (`internal/tui/switchtask.go:63`) call `convention.IssueKey` too, and
  `Model.jiraIssue` (`internal/tui/issuelink.go:35`) reads the branch's Jira
  issue through `loop.JiraIssue`.
- `internal/tui/tui.go:182` — `Model.handleKey`'s comment orders overlay,
  help, global keys, pane; the switch (`:190`) has overlay,
  `filteringIssues`, global, and no help step.
- `internal/tui/overlay.go:15` — `overlay`'s comment says "Lip Gloss v1
  cannot layer one view over another" while the module requires
  `charm.land/lipgloss/v2`.
- `internal/tui/render.go:427` — "status describes the configuration…" sits
  atop `messagingLabel`'s comment block; `Model.status` (`:435`) has no
  comment.
- `internal/tui/keys.go:350` — `keyContexts`' comment says refresh,
  open-link and copy-link act on the Branch, Commits, Review and
  review-requests panes; the contexts (`:360`) give Branch and Commits only
  `actionRefresh`, and open and copy are answered on Issues
  (`handleIssuesKey`, `internal/tui/issuekeys.go:18`), Review
  (`handleReviewLink`, `internal/tui/review.go:467`) and Reviews
  (`handleReviewQueueKey`, `internal/tui/reviewqueue.go:188`).
- `internal/tui/keys.go:62` — `keyMap`'s comment on `openLink` and
  `copyLink`, "on the Issues and Review panes", omits the Reviews pane that
  `handleReviewQueueKey` (`internal/tui/reviewqueue.go:188`) handles.

The command line:

- `CLAUDE.md:27` — the layout row "thin main; wires cli.Execute and the exit
  status" leaves out the two terminal reads `terminalPrompt`
  (`cmd/workflow/main.go:37`) keeps there, beside the keychain and editor it
  takes from their own packages.

The clients:

- `internal/forge/pulls.go:237` — `FindPullRequest`'s comment sends a caller
  to `Opened` "rather than trusting found alone"; `Opened` (`:118`) is
  `Number != 0`, true for a merged pull, and `IsOpen` is meant.
- `internal/forge/pulls.go:92` — `IsOpen`'s comment says "(Opened, above,
  …)"; `Opened` is declared at `:118`, below.
- `CLAUDE.md:43` — the layout row for `internal/forge/` lists "remotes,
  tokens, pull requests, CI", not the issues (`AssignedIssues`,
  `internal/forge/issues.go:30`) or the templates
  (`internal/forge/templates.go`).
- `internal/jira/jira.go:6` — the package comment names search, read, move,
  comment, link and whoami, not `Assign` (`internal/jira/assignee.go:13`),
  `AddWorklog` (`internal/jira/worklog.go:33`) or `WikiFromMarkdown`
  (`internal/jira/wiki.go:22`); `ARCHITECTURE.md:163` repeats the list
  without `Assign` and `AddWorklog`.
- `internal/jira/search.go:135` — `wireIssue`'s comment says the fields
  "past Reporter ride only on the detail request"; `searchFields` (`:27`) is
  `summary,status,issuetype,priority`, so `Description` (`:148`) and
  `Reporter` are detail-only too.
- `internal/jira/detail.go:32` — `LinkedIssue` is "a parent or a subtask";
  `IssueLink.Issue` (`:45`) is a `LinkedIssue` too, as `wireLinked`'s
  comment (`internal/jira/search.go:96`) says.
- `internal/jira/wiki.go:14` — `boldSentinel`'s comment calls `"\x00"` "A
  caret-feed control byte"; it is NUL, and there is no caret-feed control.
- `internal/config/config.go:4` — the package comment says `config` "loads
  the workflow configuration file"; `Save`, `SaveOver`, `RevisionOf`,
  `ParseRevision` and `SharedMode` are exported from
  `internal/config/save.go:108` onward, the `CLAUDE.md:29` row says
  "loading, redaction, validation", and the budget file's WHY
  (`scripts/package-size-budgets.txt:36`) says "load and save".

The plumbing:

- `internal/wiring/wiring.go:4` — the package comment "connects the terminal
  interface" to the clients, where each seam "the interface declares" meets
  its client, and the `CLAUDE.md:32` row "connects the interface's seams";
  both name one of three consumers, and the row above (`CLAUDE.md:31`) says
  `internal/seams` declares the seams for every surface: `connectAt`
  (`internal/cli/cli.go:404`) builds every command over `wiring.Deps`,
  `WebDeps` (`internal/cli/cli.go:306`) hands the same bundle to the web,
  and the row below (`CLAUDE.md:33`) already says `loop` is "for every
  surface".
- `internal/gitrepo/gitrepo.go:4` — the package comment says `gitrepo`
  "reads the git repository"; `Repository`'s own doc (`:28`) says reads and
  changes, and `Repository.Stage` (`internal/gitrepo/status.go:191`) is one
  of the writes.
- `internal/convention/convention.go:4` — the package comment names three
  concerns; `internal/convention/pullrequest.go` and
  `internal/convention/scopes.go` are two more, and the `CLAUDE.md:48` row
  already lists "pull request text".
- `internal/messaging/post.go:328` — `Announcement.Text`'s comment says
  "Every substituted value is escaped for Slack"; `markupFor` (`:267`)
  escapes per kind, and `keepText` (`:316`) not at all.
- `internal/messaging/messaging.go:44` — `ErrRejected` "reports a token
  Slack would not accept" and is returned for any kind's webhook 4xx by
  `deliver` (`internal/messaging/post.go:208`).

The web server:

- `internal/webserver/webserver.go:33` — `Deps`' comment says a nil read
  seam answers "with an empty result rather than an error";
  `server.GetIssue` (`internal/webserver/handlers.go:81`) answers 422 "no
  issue tracker is configured" for a nil `Issue`, and
  `server.GetAnnouncement` (`internal/webserver/announce.go:22`) and
  `server.GetPullRequestDraft` (`internal/webserver/pullrequest.go:26`)
  answer 409 for a nil `Branch` or `FindPull`, which `pullToAnnounce`
  (`internal/loop/announce.go:114`) reports as `ErrNoPullRequest`.
- `internal/webserver/webserver_test.go:25` — `errSeam`'s comment, "generic
  500 so the wire message carries no detail", predates `fault`
  (`internal/webserver/errors.go:91`), which classes by sentinel before the
  internal fallback.

A reader of `go doc`, of CLAUDE.md's layout table or of ARCHITECTURE.md is told
something the code beside it does not do, and acts on it: changes one call site
of three, treats a merged pull as open, or tries a layering the v2 upgrade
already allows. The cost is paid at the next change, when the comment is
trusted over the code.

**One way to fix it.** One pass, file by file, rewording each sentence to
what the code does now — or deleting the enumerations that go stale a verb
at a time.

**Done when.** `go doc` for `config`, `wiring`, `gitrepo`, `jira`,
`convention` and `forge.Client.FindPullRequest` reads as the code does; both
pane lists in `internal/tui/keys.go` match the
handlers; the `Deps` comment names the answers the handlers give; and each
of these prints nothing — `grep -n "the one place the interface reads"
internal/tui/branch.go`, `grep -n "cannot layer one view over another"
internal/tui/overlay.go`, `grep -n "checks Opened rather than trusting"
internal/forge/pulls.go`, `grep -n "caret-feed" internal/jira/wiki.go`,
`grep -n "status describes the configuration" internal/tui/render.go` and
`grep -n "so the wire message carries no detail"
internal/webserver/webserver_test.go`. The greps are a sample; every other
cited sentence is checked the same way, by grepping the phrase its bullet
quotes in the file it cites.

### DEBT-90 The docs site trails the code across usage, configuration, web and install

Severity: low · Confidence: read

The site's pages, the README and the docs index promise things the code does
not do or stay silent on things it does. `scripts/check-docs-drift.sh`
compares only the generated command reference, so no gate sees any of these
pages. UX-87 counts the same Settings undercount from the user's side.

The usage page:

- `docs/content/docs/usage.md:173` — "Pick up an issue" names three field
  cases (a fixed set, text, any other kind sent to Jira) where
  `fieldForm.textual` (`internal/tui/fields.go:79`) fills `FieldUser` and
  `FieldDate` as typed inputs and `fieldForm.multi` (`:90`) takes any number
  of a `FieldOptionList`'s options.
- `docs/content/docs/usage.md:185` — "Branch" says the branch "starts from
  origin's default branch, which the overlay names" with no word of the
  fetch (`branchCreator.create`, `internal/tui/branch.go:442`), the "fetched
  AGE" line (`branchCreator.start`, `:364`) or the offer after a failed
  fetch (`fetched.apply`, `internal/tui/branchresult.go:49`).
- `docs/content/docs/usage.md:192` — "Stage and commit" describes the list
  and the composer; the page never mentions a diff (`grep -ic diff` is 0)
  while `diffSection` (`internal/tui/diff.go:67`) draws the selected file's
  diff beneath the list.
- `docs/content/docs/usage.md:225` — "The title is the branch's oldest
  commit subject" is true only when `pull_request.title_source`
  (`PullRequest.TitleSource`, `internal/config/pullrequest.go:21`) is the
  default; the key is validated (`:24`) and on no docs page.
- `docs/content/docs/usage.md:313` — "Editor" says "`$VISUAL`, else
  `$EDITOR`, else `vi`"; `chosen` (`internal/editor/editor.go:85`) consults
  `GIT_EDITOR` first and `defaultEditor` (`:97`) returns `notepad` on
  Windows, while `defaultEditor`'s own comment (`:94`) omits `$GIT_EDITOR`
  too.

The configuration page (the Fields table's missing rows are DEBT-91):

- `docs/content/docs/configuration.md:37` — "does the same for Slack" claims
  `config init` checks the webhook; `newConfigInitCmd`'s Short
  (`internal/cli/config_cmd.go:58`, "asking for and checking each
  credential") and Long (`:59`, "check each one") say the same, reproduced
  at `docs/content/docs/reference/workflow_config_init.md:10` and `:14`,
  while `collectMessaging` (`internal/cli/config_cmd.go:308`) prints "saved
  (a webhook cannot be checked without posting)".
- `docs/content/docs/configuration.md:44` — "looks for .workflow.json in the
  current directory first, then in your home directory", as does
  `Discover`'s comment (`internal/config/load.go:18`, "searching workDir
  first and then homeDir"), where `Discover` (`:26`) calls `nearest`, which
  walks up to the directory holding `.git` (`:37`).
- `docs/content/docs/configuration.md:383` — "While it is on and no
  `timing.ci_interval` is set, CI is polled every three minutes" gives two
  of `pollInterval`'s three conditions (`internal/tui/review.go:195`): no
  announcement may be waiting either.
- `docs/content/docs/configuration.md:482` — the store is "keyed only by a
  repository's host and path and by a hash of your Jira URL", and
  `ARCHITECTURE.md:256` says the repository key is the remote's parsed host
  and path; `migrate` (`internal/store/store.go:258`) keys the cache by
  `(instance, view)`, where `Model.cacheIssues`
  (`internal/tui/issues.go:53`) passes the view's JQL text, and `repoKey`
  (`internal/wiring/wiring.go:385`) falls back to `where.Root` when there is
  no remote or it does not parse.

The README and the docs index:

- The README (line 45) and `docs/content/_index.md:29` — "`workflow doctor
  --online` asks Jira, your messaging service and your forge whether each
  credential actually works"; `checkMessaging`'s comment
  (`internal/cli/doctor_credentials.go:197`) says a webhook is uncheckable,
  `ErrWebhookUncheckable` is reported unchecked (`:220`), `credentialStatus`
  names that `unchecked` (`internal/cli/doctor_json.go:202`), and
  `TestDoctorOnlineSaysAWebhookCannotBeChecked`
  (`internal/cli/online_test.go:153`) pins "cannot be checked".
- The README's line 262 — "There are no releases yet." while nine tags
  exist, v0.3.0 the latest; the README's line 66 and
  `docs/content/docs/install.md:37` still say that until the first tag
  exists `@latest` resolves to `main`, and the pinned example (`:34`, and
  the README's line 65) is `@v0.0.5`, five releases behind.

The web page:

- `docs/content/docs/web.md:126` — Settings lists its seven parts as Jira,
  the forge, messaging, branches, commits, pull requests and the store;
  `ConfigForm` (`web/src/features/settings/SettingsPanel.tsx:127`) renders
  Jira, Messaging, Forge, Commit, Branch, Pull request, Store.
- `docs/content/docs/web.md:129` — "The parts the form does not show yet
  (`ui`, `timing`, `headers`, `views` and `branch.prefixes`)" names five of
  ten carried keys, as UX-87 in `UX.md` ("carry five it cannot show") and
  `ConfigForm`'s comment (`web/src/features/settings/SettingsPanel.tsx:67`)
  do; the whole `Config` seeds the form and rides back, and `token_command`
  and `token_env` (`api/openapi.yaml:1399`; messaging's at `:1434`) and
  `channels` (`:1440`; `Messaging.Channels`,
  `internal/config/config.go:125`) are in the schema and registered by no
  fieldset.
- `docs/content/docs/web.md:131` — "a save never overwrites a change it has
  not seen" is stronger than `SaveOver` makes it: its comment
  (`internal/config/save.go:117`) says the check (`:121`) and the write
  (`:130`) are not one step, and the `staleTime: Infinity` trade-off in this
  file repeats the page's phrasing.
- `web/src/queryClient.ts:4` — the `queryClient` comment says "the stream's
  snapshots update it through setQueryData"; the stream handler in
  `useEventStream` (`web/src/api/snapshot.ts:71`) writes `useSnapshotStore`,
  and the only `setQueryData` callers are `useReloadConfig`
  (`web/src/features/settings/configApi.ts:129`) and the save (`:111`).

The reference index:

- `docs/content/docs/reference/_index.md:9` — "Every command and flag,
  generated from the command tree itself", and "The same text is available
  offline" (`:13`); `run` in `cmd/docsgen/main.go:88` says cobra's `help`
  command gets no page, `docs/content/docs/scripting.md:38` documents `help`
  as a command with its own exit behavior, and
  `scripts/check-docs-drift.sh:30` copies the hand-written index around the
  comparison, so the gate cannot see the claim.

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

### DEBT-91 The configuration page's Fields table omits thirteen keys the code reads

Severity: low · Confidence: read

Thirteen keys the loader reads, validates or writes — `version`, `jira.project`,
`jira.review_status`, `forge.cli`, `messaging.channels`, `ui.comments_shown`,
`timing.request_timeout`, `timing.ci_interval`, `branch.slug_limit`,
`commit.types`, `commit.subject_limit`, `commit.refs_trailer` and
`pull_request.title_source` — have no row in the Fields table of
`docs/content/docs/configuration.md:58`, the `timing` and `pull_request`
sections are never named, and the sample file at the top of the page omits the
`version` key `config init` writes first. Counted by grepping each key on the
page: 0 hits each, `ci_interval` once in prose. The page says an unknown key is
an error, then omits thirteen it accepts. A user who meets "Review status" or
"Use the forge CLI" in the browser, the channel cycle in the terminal, the
72-character ruler or the `Refs:` trailer in the composer, or `"version": "1"`
at the top of a file `config init` just wrote, opens the reference and finds
nothing; `forge.cli`, the setting that reaches a forge behind SSO, is documented
in the README (line 191) and `docs/content/docs/scripting.md:51` but has no row
on the configuration page; usage.md presents the limit, the trailer and the
oldest-commit title as fixed when each has a key. `doctor` and the Settings form
honor every one, and no gate sees the page. The web side of the same gap is
UX-87.

- `docs/content/docs/configuration.md:58` — the Fields table the thirteen
  rows are missing from.
- `docs/content/docs/configuration.md:10` — the sample file under
  "Configuration" lists `jira`, `messaging`, `forge` and `ui`; no
  `version`.
- `internal/config/config.go:168` — `Config.Version`, written first by
  `config init` and validated; 0 hits on the page.
- `internal/config/config.go:84` — `Jira.Project`, the branch-name key
  guard; 0 hits.
- `internal/config/config.go:94` — `Jira.ReviewStatus`, which drives the
  post-open transition offer; named in errors.md and the `pr` reference
  only.
- `internal/config/config.go:125` — `Messaging.Channels`, the channel
  cycle's source; 0 hits.
- `internal/config/config.go:155` — `Forge.CLI`, which routes forge calls
  through `gh` or `glab`; on the site only at
  `docs/content/docs/scripting.md:51`.
- `internal/config/ui.go:37` — `UI.CommentsShown`; 0 hits.
- `internal/config/timing.go:21` — `Timing.RequestTimeout`; 0 hits, and no
  `timing` row at all.
- `internal/config/timing.go:25` — `Timing.CIInterval`, named once in
  prose at `docs/content/docs/configuration.md:383` ("The interface:
  mouse, ASCII and color") as if already introduced; its twenty-second
  default is given only beside the web page's stream
  (`docs/content/docs/web.md:48`) and in the event stream's description in
  `api/openapi.yaml`, and its format nowhere.
- `internal/config/branch.go:37` — `Branch.SlugLimit`, validated at `:55`;
  0 hits, and the Branch names section lists the other three fields only.
- `internal/config/commit.go:29` — `Commit.Types`, validated at load; 0
  hits.
- `internal/config/commit.go:32` — `Commit.SubjectLimit`, default 72; 0
  hits.
- `internal/config/commit.go:35` — `Commit.RefsTrailer`, default `Refs`; 0
  hits.
- `internal/config/pullrequest.go:21` — `PullRequest.TitleSource`, refused
  when unknown at load; the whole `pull_request` block is absent from the
  page.
- `docs/content/docs/usage.md:197` — "Stage and commit": "the 72-character
  limit" stated as fixed though `commit.subject_limit` changes it.
- `docs/content/docs/usage.md:198` — "A `Refs:` trailer" stated as fixed
  though `commit.refs_trailer` relabels it.
- `docs/content/docs/usage.md:225` — "Open the pull request": "The title
  is the branch's oldest commit subject" stated as fixed though
  `pull_request.title_source` can switch it to the issue.
- `docs/content/docs/usage.md:270` — "Announce it" promises a channel
  choice "with more than one channel to choose from" that the
  configuration page never says how to set up.
- `docs/content/docs/install.md:14` — "What it needs" lists `gh` only, as an
  optional place to find a GitHub token; `glab`, which `forge.cli` needs on
  GitLab, is not listed.

**One way to fix it.** One row per key in the Fields table with the struct
field's own comment as the text and the default stated (ten seconds,
twenty seconds, 48, 72, `Refs`, `commit`); `"version": "1"` in the sample
file; `timing` and `pull_request` named as sections; usage.md's sentences
on the subject limit, the trailer and the title qualified with "by
default"; and `glab` beside `gh` in install.md's needs list.

**Done when.** For each of the thirteen keys
`grep -c '<key>' docs/content/docs/configuration.md` returns at least 1,
`grep -c '"version"' docs/content/docs/configuration.md` returns at least
1, `task docs:check` stays green, and `grep -n glab
docs/content/docs/install.md` prints a line.

### DEBT-92 Tests in Go, the scripts and the web that prove nothing

Severity: low · Confidence: read

Across `internal/tui`, `internal/cli`, `internal/forge`, `internal/config`,
`internal/store`, the release and coverage scripts and the web's vitest
suite, tests named for a rule would pass with the rule gone. `testshape`
checks only that a failure call is reachable and v8's range count cannot see
a weak assertion, so every one clears the gate.

The terminal:

- `internal/tui/notify_test.go:120` —
  `TestNotifyPollsOnALongerBeatWithNoIntervalSet` asserts only "running" and
  no ring; `horizon` (`internal/tui/harness_test.go:27`) is one second, so
  any beat over a second is indistinguishable from `notifyPollInterval`
  (`internal/tui/review.go:189`).
- `internal/tui/merge_test.go:468` — `TestTheMergePreviewShowsItIsMerging`
  presses `j` in flight and asserts only "merging"; without the
  `p.send.sending` guard in `mergePicker.handleKey`
  (`internal/tui/merge.go:178`), `j` would move the selection and the word
  would still show.
- `internal/tui/finish_test.go:226` —
  `TestTheFinishPreviewShowsItIsFinishing` presses `x`, a key
  `finishPreview.handleKey` (`internal/tui/finish.go:103`) ignores anyway,
  and asserts only "finishing".
- `internal/tui/preditor_test.go:179` — `TestTheEditorShowsItIsSaving`
  presses `x` and asserts only "saving"; without the guard in
  `prEditor.handleKey` (`internal/tui/preditor.go:83`), `x` lands in the
  title (`:94`) unseen.
- `internal/tui/edges_test.go:27` — `TestNothingInterruptsAWriteBeingSent`,
  the table the three guards should join, covers a branch, a pull request,
  an announcement, a configuration and a re-run.

The command line:

- `internal/cli/branch_test.go:289` — `TestBranchReportsAFailedCreate`
  asserts `err == nil` only; so do `TestBranchReportsAnUnreadableRepository`
  (`:306`), leaving the documented exit 4 unpinned, and
  `TestBranchReportsAnUnreachableTracker` (`:322`), whose fixture answers
  500 (`:315`) — a rejection, exit 1, since `statusError`
  (`internal/jira/jira.go:253`) makes a 500 `ErrUnexpectedStatus`, never
  `ErrUnreachable` — so "Unreachable" in its name is wrong.
- `internal/cli/scriptable_test.go:25` — `TestBranchCommandNeedsATracker`
  asserts `err == nil` only and cannot tell the exit-3 refusal from any
  other failure; `TestPRCommandReadsTheBranch` (`:36`) and
  `TestStandupOutsideARepositoryReportsSo`
  (`internal/cli/standup_test.go:80`) do the same, while
  `TestAnnounceCommandNeedsMessaging` (`internal/cli/scriptable_test.go:46`)
  shows the file's own stronger shape, `wantExit`
  (`internal/cli/exitstatus_test.go:30`) is the helper the six could call,
  and `TestRequestLogReportsAFileItCannotOpen`
  (`internal/cli/reqlog_test.go:40`) explains why a bare error check proves
  nothing.

The clients:

- `internal/forge/reviews_test.go:185` —
  `TestReviewRequestsForAnUnknownForge` accepts any error and never names
  `ErrUnknownForge`; `TestForgeIssueMethodsRejectAnUnknownForge`
  (`internal/forge/issues_test.go:232`) does the same in each of three
  subtests.
- `internal/config/version_test.go:24` —
  `TestLoadAcceptsTheCurrentVersionAndRejectsAnUnknownOne` runs two `Load`s
  under one Act (`:24`, `:25`) and checks
  `strings.Contains(unknownErr.Error(), "99")` (`:36`) rather than
  `errors.Is`, so `ErrUnknownVersion` (`internal/config/config.go:45`) is
  exported, wrapped and asserted by nothing.
- `internal/config/load_test.go:169` —
  `TestDiscoverIgnoresAnEmptyDirectory`'s comment describes a machine with
  no home directory, but the Act (`:170`) passes `""` as workDir, so
  `nearest("")` never enters its loop, and the `dir == ""` guard in `fileIn`
  (`internal/config/load.go:85`) the test means to cover was, per the gobco
  report, 66 times false and never true.

The scripts:

- `scripts/release/push-release-tag_test.sh:32` — the `gh` stub answers any
  `gh api` call with `GH_STUB_PULL` (`:33`) and ignores `--jq`, so the label
  filter in `pr_number` (`scripts/release/push-release-tag.sh:91`) is
  evaluated by no test, and the "no pull request labeled" case
  (`scripts/release/push-release-tag_test.sh:118`) passes because the stub
  printed nothing, not because the filter selected nothing.
- `scripts/coverage-summary_test.sh:69` — the summary-shape case asserts the
  substrings `"statements"` and `"branch"` only; the `stats` fixture (`:51`)
  yields 50 %, which nothing compares, so the arms arithmetic in
  `scripts/coverage-summary.sh:41` is unprotected.
- `scripts/gobco-report_test.sh:23` — the suite's only case is the no-floor
  argument; the untested-package refusal (`unaccounted`,
  `scripts/gobco-report.sh:181`) and the no-statistics refusal (`summary`,
  `:246`) are exercised only in their passing direction.

The web:

- `web/src/features/review/ReviewPanel.tsx:275` — `PullRequestForm`'s "The
  branch is not pushed yet; opening will push it first." is asserted
  nowhere; the mocked draft in
  `web/src/features/review/ReviewPanel.test.tsx:20` sets `needs_push: true`
  for every case.
- `web/src/features/issues/IssueDetailPanel.test.tsx:311` — "a Retry refused
  again beside an issue already read keeps its focus" names a browser
  outcome jsdom cannot observe: `expect(document.activeElement).toBe(retry)`
  (`:338`) passes because jsdom does not move focus off a disabled control,
  while `IssueUnread`'s `disabled={retrying}`
  (`web/src/features/issues/IssueDetailPanel.tsx:81`) drops it in Chromium.
- `web/src/features/issues/IssueListControls.test.tsx:124` — "offers no view
  select when the views cannot be read" waits only for a request, and
  `queryByRole('combobox')` (`:126`) is null while pending too, since
  `ViewSelect` (`web/src/features/issues/IssueListControls.tsx:50`) returns
  null for an empty list either way.
- `web/src/features/review/ReviewPanel.test.tsx:92` —
  `expect(screen.getByText('build')).toBeTruthy()` matches a span as readily
  as a link, so the `check.url === ''` branch in `PullRequestSummary`
  (`web/src/features/review/ReviewPanel.tsx:120`) is exercised by the one
  fixture with a URL (`web/src/features/review/ReviewPanel.test.tsx:79`) and
  never checked by role or href.
- `web/src/features/writes.test.tsx:326` — the writes table discards the
  `Request[]` that `fakeApi` (`web/src/test/fakeApi.ts:14`) returns for
  exactly this, so `breaking: fields.breaking` in `CommitForm`
  (`web/src/features/branch/CommitForm.tsx:61`) can be dropped —
  `CommitRequest.breaking` is optional
  (`web/src/api/generated/types.gen.ts:114`), so it compiles — and no test
  fails.

The store:

- `internal/store/store.go:70` — `timestamp`, the RFC3339 rule, has no test
  behind it; `Store.CachedIssues` (`internal/store/cache.go:43`) scans
  `cached_at` (`:46`) into a variable nothing reads.
- `internal/store/store.go:57` — `dsnPragmas`' `foreign_keys(1)` and the `ON
  DELETE CASCADE` on `cached_issue` (`:271`) are exercised by nothing:
  `writeCachedIssues` (`internal/store/cache.go:142`) deletes the children
  itself, so the cascade guards nothing.
- `internal/store/store_test.go:79` — `TestScopesAreKeptPerRepository`
  discards `RecordScope`'s error and asserts only that another repository
  reads nothing (`:85`); `TestTheCacheIsKeptPerInstanceAndView`
  (`internal/store/cache_test.go:85`) discards `CacheIssues`' error and
  asserts only that another view and instance read nothing (`:92`), so a
  `RecordScope` or `CacheIssues` that writes nothing passes.

The regressions these tests exist to catch pass the suite green: a dropped
in-flight guard on the merge picker, the finish preview or the pull request
editor; `branch` exiting 1 where the contract says 4; a lost
`ErrUnknownForge` or `ErrUnknownVersion` wrap; a release-label filter typo
found at the next release; a `timestamp()` that writes `now.String()`; a
commit sent without its `!` or body; the "opening will push it first" note
deleted.

**One way to fix it.** Sharpen each Assert to what its name claims:
`wantExit` or `errors.Is` with the family or sentinel meant, and rename the
500 case "rejected"; a recording timer for the notify beat; the three
missing guards as cases of `TestNothingInterruptsAWriteBeingSent`;
`aria-disabled` on Retry or a Playwright focus case; the refused read
awaited before asserting the select is absent; the check asserted by role
and href; the recorded requests read for their bodies; the `gh` stub running
the script's own `--jq` over a fixture of pulls; exact JSON from
`scripts/coverage-summary.sh`; a stub gobco for the gate's refusals;
raw-file reads that parse each `_at`, a cascade a test makes fire, and each
Arrange's error fatal.

**Done when.** Each named mutation fails a test: setting
`notifyPollInterval` to 20 seconds; removing `case p.send.sending` from
`mergePicker.handleKey`, `finishPreview.handleKey` and `prEditor.handleKey`;
changing `branch`'s non-repository exit from 4; returning a different
sentinel for `KindUnknown` from `ReviewRequests` or the issue methods;
removing `select(any(.labels[]; …))` from
`scripts/release/push-release-tag.sh`; changing `($conditions * 2)` to
`$conditions` in `scripts/coverage-summary.sh`; deleting a name from
`NO_TESTS` in `scripts/gobco-report.sh`; changing `timestamp()` to
`now.String()`, removing `foreign_keys(1)` from `dsnPragmas`, or making
`RecordScope` or `CacheIssues` return nil without writing; rendering a
select while `useViews` is in error; replacing the check anchor in
`web/src/features/review/ReviewPanel.tsx` with a span; deleting `breaking:
fields.breaking` from `web/src/features/branch/CommitForm.tsx`; and deleting
the "opening will push it first" paragraph.

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

Nothing is open here. What the web's gates still lack — an end-to-end run
that drives a write — is DEBT-65, with the other gates below; what the web
carries on purpose — read-only under `--dry-run`, and queries the stream
keeps fresh — is under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost).

## The gates, the build and the tests

What is open here is the coverage worklist and the metric behind it, gates
whose printed sentence claims more than their check measures, CI jobs and
triggers that do not do what their comments say, and tests named or shaped
for something other than what they prove.

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
`Store.CacheIssues` (`internal/store/cache.go:110`, `:121`), and `rows.Err`
in `readCachedIssues` (`internal/store/cache.go:88`) and `Store.Announces`
(`internal/store/announce.go:80`). The other five are
the do-nothing guards' second operands, never seen true: `repo == ""` in
`Store.RecordAnnounce` and `Store.Announces`
(`internal/store/announce.go:24`, `:50`), `instance == ""` in
`Store.CachedIssues` and `Store.CacheIssues` (`internal/store/cache.go:32`,
`:99`), and `s.dir == ""` in `Store.off` (`internal/store/store.go:149`).

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

### DEBT-65 The web's e2e drives no write

Severity: medium · Confidence: read

`task check` (`Taskfile.yml:502`) runs the web's lint, client-drift check and
unit tests beside the Go gates, but not its end-to-end suite. That suite is
six specs (`web/e2e/a11y.spec.ts`, `web/e2e/layout.spec.ts`,
`web/e2e/panes.spec.ts`, `web/e2e/screens.spec.ts`,
`web/e2e/smoke.spec.ts`, `web/e2e/theme.spec.ts`), outside `task check`
(CI's `e2e` job and `yarn test:e2e` run it), with no `workflow --web`
backend — acknowledged at `.github/workflows/ci.yml:101` ("No backend": the
specs answer the API themselves, or read a VITE_MOCK build's fixtures) — so
no test drives any of the twelve write actions end to end.

**One way to fix it.** The e2e job starts `workflow --web` against a fixture
repository so one spec can commit, push and open a pull request.

**Done when.** One Playwright spec performs a write against a running server.

### DEBT-148 Four clicked steps in the web are never scanned or walked

Severity: medium · Confidence: read

The axe scans reach the issue detail, the opened pull's offers and a staged file
after a click, and the Tab walk runs once, on each section as it opens. Four
steps a user reaches by clicking — the pull request form, the push confirmation,
the announcement preview and a write's refusal — are scanned and walked in
neither theme, and the hermetic Settings scan settles on the heading rather than
on the read's outcome. `CLAUDE.md:240` promises axe across every section in both
themes and every control Tab reaches in view at three widths; for the clicked
steps only jsx-a11y's static rules apply. DEBT-65 (a write against a real
server) does not cover this: it is the runtime floor's reach, not the backend's.

- `web/e2e/a11y.spec.ts:102` — the populated-sections test clicks each of
  `populatedSectionNames` and scans at once; the populated snapshot's
  `review` has `found: true` (`web/src/dev/mockSnapshot.ts:108`), so the
  pull branch is taken and `PullRequestForm` never mounts there.
- `web/e2e/a11y.spec.ts:265` — the offers test clicks "Open a pull request"
  and then "Open pull request" with no scan between compose and submit, and
  `scan` runs (`web/e2e/a11y.spec.ts:274`) after `OpenPullRequest`
  (`web/src/features/review/ReviewPanel.tsx:166`) has returned null on
  `open.state === 'done'`, so the form is gone.
- `web/e2e/a11y.spec.ts:22` — `settled` returns the level-1 heading for
  every section but Reviews, and the heading is drawn regardless of panel
  state; the hermetic loop (`web/e2e/a11y.spec.ts:72`) scans as soon as it
  is visible, before the config read fails to Retry, so it may land on
  `SettingsPanel`'s "Loading the configuration…" placeholder
  (`web/src/features/settings/SettingsPanel.tsx:30`).
- `web/e2e/layout.spec.ts:180` — the every-section-fits test runs
  `openSection` then `walkTabOrder` once per section with no click between.
- `web/src/features/branch/stagingApi.ts:13` — `stageFile` under `VITE_MOCK`
  returns before the SDK, so no write can fail and no `role=alert` refusal
  appears.
- `web/src/features/review/ReviewPanel.tsx:265` — `PullRequestForm`'s
  `aria-label="Open a …"` form, with its seven labeled fields, is scanned
  and walked in neither build.
- `web/src/features/branch/BranchPanel.tsx:172` — `PushConfirm`'s
  `role="group"` with `aria-labelledby` sits behind `confirming`, which no
  spec sets.
- `web/src/features/messaging/MessagingPanel.tsx:231` — `AnnouncePreview`'s
  `role="group"` sits behind `preview.state === 'done'`, which no spec
  reaches.

So `PushConfirm`'s `aria-labelledby` resolving, the form's label
associations, and the focus order of a form that opens inside the scrolling
pane — exactly where a focused control is clipped at 640 px — can regress
green. The hermetic Settings scan is timing-dependent; what it can hide is
one `EmptyState` with a Retry button.

**One way to fix it.** In `web/e2e/a11y.spec.ts`, scan with the pull request
form, the push confirmation and the announcement preview open, and with one
write routed to a 500 so its alert is on screen, in both themes; in
`web/e2e/layout.spec.ts`, walk again after opening each step and add a
hermetic pull-request-form case at 640 px; settle hermetic Settings on Retry
the way Reviews settles on its list.

**Done when.** `web/e2e/a11y.spec.ts` scans a page on which the "Open a pull
request" form, the push confirmation group, the "Announcement preview" group
and a `role=alert` refusal are each visible, in both themes;
`web/e2e/layout.spec.ts` reports Push, Cancel, Channel, Announce now and the
form's fields among the controls reached, each at least 99 % in view, at 640
px; and the hermetic Settings scan waits on the Retry button, so a
deliberate delay in the config read does not change what axe reports on.

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
- **`staleTime: Infinity`** (`web/src/queryClient.ts:12`) with the event
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
  opened or a save lands. A save that still meets a change it has not seen
  is refused (409) and nothing is written; Settings offers **Reload**.
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
