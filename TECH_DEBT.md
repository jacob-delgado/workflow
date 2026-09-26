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
comments and the code they describe, and three sweeps that cross every
surface — tests that prove nothing, facts written twice and arms no input
reaches.

### DEBT-71 `wiring` returns `tui.Deps`, though three surfaces consume the seams

Severity: low · Confidence: read

`wiring.Deps` (`internal/wiring/wiring.go:82`) returns `tui.Deps`, so the
wiring package imports the terminal interface; the CLI's `WebDeps`
(`internal/cli/cli.go:292`) then narrows that bundle for the web server.
The seams are not the terminal's — they are the loop's. That import no
longer stands in the way of shared composition: `internal/loop` takes each
seam as a plain argument (`loop.PullSeams`, `loop.AnnounceSeams`) and never
needed `wiring`. What is left is narrower: a seam only the CLI or the web
needs must still be declared on `tui.Deps`, as a `RecentCommits` for
`standup` would be — which is why `standup` reads its commits from the
repository directly instead (`internal/cli/standup.go:103`).

**One way to fix it.** Move the bundles that depend only on leaf types —
`JiraDeps`, `GitDeps`, `ForgeDeps`, `MessagingDeps`, `HookDeps` — to a
package all three surfaces import, with type aliases left in `tui`.
`StoreDeps` declares what it remembers of announcements as
`loop.Announced`, which every surface already imports, so it can move with
them; `EditorDeps` carries Bubble Tea's `tea.Msg` and `tea.Cmd`, so
`wiring` would still return `tui.Deps`.

**Done when.** The seam bundles are declared where all three surfaces can
import them without importing each other. Deferred — YAGNI until a seam the
terminal does not use has to be added to `tui.Deps`.

### DEBT-89 Comments and layout rows that no longer say what the code does

Severity: low · Confidence: read

Across the terminal, the command line, the clients, the plumbing, the web
server and the two layout maps, doc comments and layout rows describe an
earlier shape of the code. No linter reads a comment, so every one passes
the gate. The `wiring` package comment
below is the same drift DEBT-71 records at the type level.

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
- `CLAUDE.md:42` — the layout row for `internal/forge/` lists "remotes,
  tokens, pull requests, CI", not the issues (`AssignedIssues`,
  `internal/forge/issues.go:30`) or the templates
  (`internal/forge/templates.go`).
- `internal/jira/jira.go:6` — the package comment names search, read, move,
  comment, link and whoami, not `Assign` (`internal/jira/assignee.go:13`),
  `AddWorklog` (`internal/jira/worklog.go:33`) or `WikiFromMarkdown`
  (`internal/jira/wiki.go:22`); `ARCHITECTURE.md:152` repeats the list
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
  interface" to the clients, and the `CLAUDE.md:31` row "connects the
  interface's seams", name one of three consumers: `connectAt`
  (`internal/cli/cli.go:403`) builds every command over `wiring.Deps`,
  `WebDeps` (`internal/cli/cli.go:305`) hands the same bundle to the web,
  and the row below (`CLAUDE.md:32`) already says `loop` is "for every
  surface".
- `internal/gitrepo/gitrepo.go:4` — the package comment says `gitrepo`
  "reads the git repository"; `Repository`'s own doc (`:28`) says reads and
  changes, and `Repository.Stage` (`internal/gitrepo/status.go:191`) is one
  of the writes.
- `internal/convention/convention.go:4` — the package comment names three
  concerns; `internal/convention/pullrequest.go` and
  `internal/convention/scopes.go` are two more, and the `CLAUDE.md:47` row
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
  `ARCHITECTURE.md:245` says the repository key is the remote's parsed host
  and path; `migrate` (`internal/store/store.go:258`) keys the cache by
  `(instance, view)`, where `Model.cacheIssues`
  (`internal/tui/issues.go:53`) passes the view's JQL text, and `repoKey`
  (`internal/wiring/wiring.go:384`) falls back to `where.Root` when there is
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
  `internal/config/config.go:121`) are in the schema and registered by no
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
- `internal/config/config.go:164` — `Config.Version`, written first by
  `config init` and validated; 0 hits on the page.
- `internal/config/config.go:80` — `Jira.Project`, the branch-name key
  guard; 0 hits.
- `internal/config/config.go:90` — `Jira.ReviewStatus`, which drives the
  post-open transition offer; named in errors.md and the `pr` reference
  only.
- `internal/config/config.go:121` — `Messaging.Channels`, the channel
  cycle's source; 0 hits.
- `internal/config/config.go:151` — `Forge.CLI`, which routes forge calls
  through `gh` or `glab`; on the site only at
  `docs/content/docs/scripting.md:51`.
- `internal/config/ui.go:37` — `UI.CommentsShown`; 0 hits.
- `internal/config/timing.go:21` — `Timing.RequestTimeout`; 0 hits, and no
  `timing` row at all.
- `internal/config/timing.go:24` — `Timing.CIInterval`, named once in
  prose at `docs/content/docs/configuration.md:383` ("The interface:
  mouse, ASCII and color") as if already introduced; its format and
  twenty-second default are nowhere.
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

- `internal/tui/hookgen_test.go:222` — `TestTheOfferIsMadeOnlyWhenItHelps`
  calls `refuseScreen` on the start screen, where
  `TestTheLefthookOfferOpensFromTheCommitsPaneNotAtStart` (`:78`) shows the
  offer never opens at start; it cannot see the `msg.configured` check in
  `hooksFound.apply` (`internal/tui/hookgen.go:47`).
- `internal/tui/commits_test.go:84` —
  `TestTheCommitsDetailWaitsForTheStatus` calls `requireScreen` for
  "loading" on the whole screen; `commitsRail`
  (`internal/tui/commits.go:104`) says it in the rail whether or not the
  detail is open.
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
  filter in `pr_number` (`scripts/release/push-release-tag.sh:71`) is
  evaluated by no test, and the "no pull request labeled" case
  (`scripts/release/push-release-tag_test.sh:96`) passes because the stub
  printed nothing, not because the filter selected nothing.
- `scripts/coverage-summary_test.sh:69` — the summary-shape case asserts the
  substrings `"statements"` and `"branch"` only; the `stats` fixture (`:51`)
  yields 50 %, which nothing compares, so the arms arithmetic in
  `scripts/coverage-summary.sh:41` is unprotected.
- `scripts/gobco-report_test.sh:23` — the suite's only case is the no-floor
  argument; the untested-package refusal (`unaccounted`,
  `scripts/gobco-report.sh:176`) and the no-statistics refusal (`summary`,
  `:241`) are exercised only in their passing direction.

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
  (`web/src/features/branch/CommitForm.tsx:75`) can be dropped —
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
500 case "rejected"; the focused pane's title and first body row rather than
the whole screen; a recording timer for the notify beat; the three missing
guards as cases of `TestNothingInterruptsAWriteBeingSent`; `aria-disabled`
on Retry or a Playwright focus case; the refused read awaited before
asserting the select is absent; the check asserted by role and href; the
recorded requests read for their bodies; the `gh` stub running the script's
own `--jq` over a fixture of pulls; exact JSON from
`scripts/coverage-summary.sh`; a stub gobco for the gate's refusals;
raw-file reads that parse each `_at`, a cascade a test makes fire, and each
Arrange's error fatal.

**Done when.** Each named mutation fails a test: deleting the
`if msg.configured` branch from `hooksFound.apply`; setting
`notifyPollInterval` to 20 seconds; removing `case p.send.sending` from
`mergePicker.handleKey`,
`finishPreview.handleKey` and `prEditor.handleKey`; changing `branch`'s
non-repository exit from 4; returning a different sentinel for `KindUnknown`
from `ReviewRequests` or the issue methods; removing `select(any(.labels[];
…))` from `scripts/release/push-release-tag.sh`; changing `($conditions *
2)` to `$conditions` in `scripts/coverage-summary.sh`; deleting a name from
`NO_TESTS` in `scripts/gobco-report.sh`; changing `timestamp()` to
`now.String()`, removing `foreign_keys(1)` from `dsnPragmas`, or making
`RecordScope` or `CacheIssues` return nil without writing; rendering a
select while `useViews` is in error; replacing the check anchor in
`web/src/features/review/ReviewPanel.tsx` with a span; deleting `breaking:
fields.breaking` from `web/src/features/branch/CommitForm.tsx`; and deleting
the "opening will push it first" paragraph.

### DEBT-93 Facts written in two places with nothing holding the copies together

Severity: low · Confidence: read

Facts the code needs in more than one place are written in each, with
nothing keeping the copies equal. CLAUDE.md names the smell and the rule of
three; no linter or knip rule sees any of it.

- `internal/tui/branch.go:22` — `notInRepository` is a second wording of
  `gitrepo.ErrNotARepository`, under a comment (`:21`) saying the fact is
  "stated one way"; `programErrors` (`internal/tui/failure.go:283`) holds
  the first.
- `internal/tui/composer.go:378` — `Model.recordScope` tests `scope != ""`
  on the raw `c.scope.Value()` (`:143`), so `' '` is recorded;
  `server.rememberScope` (`internal/webserver/commit.go:174`) trims first,
  and `ValidateScope` (`internal/convention/convention.go:65`) accepts a
  whitespace-only scope.
- `internal/tui/run.go:222` — `commandRun.failureHeadline` is a map keyed by
  run-title literals, "the fixup was refused" (`:225`) untested, while six
  sites type the titles: `commitComposer.commit`
  (`internal/tui/composer.go:364`, "git commit"), the amend
  (`internal/tui/commits.go:384`), the fixup (`:399`), `preCommit` (`:22`),
  the push (`internal/tui/run.go:398`) and the rebase (`:418`).
- `internal/tui/spine.go:68` — `Model.stages` hard-codes five hues in stage
  order and indexes them (`:74`) by the position of `Stages`' result
  (`internal/progress/progress.go:66`), the one place the stages are
  derived.
- `internal/forge/remote.go:153` — `kindOf` knows `github.com` only, so a
  `.ghe.com` host is `KindUnknown`; `githubsOwn`
  (`internal/forge/host.go:40`) and `githubAPIBase`
  (`internal/forge/remote.go:186`) both know the `.ghe.com` rule, and
  `checkForge` (`internal/cli/doctor_credentials.go:121`) tells such a
  tenant to "set forge.kind and forge.host" for a host the code could
  classify.
- `internal/config/ui.go:46` — the rebindable action names are listed
  in `UI.Keys`' comment, again under "Rebinding keys"
  (`docs/content/docs/configuration.md:415`), and bound in `CheckKeys`
  (`internal/tui/keys.go:302`), which binds `jump-to-pane` too but refuses
  to move it; no test holds the three to each other.
- `internal/config/config.go:349` — `Config.Problems`' sentence
  "jira.base_url is not an absolute http or https URL" is
  `ErrInvalidBaseURL`'s text verbatim (`internal/jira/jira.go:44`), and the
  rule behind it is written twice: `absoluteWebURL`
  (`internal/config/config.go:364`) accepts userinfo that `usable`
  (`internal/jira/jira.go:164`) refuses first (`:160`).
- `.github/workflows/release-please.yml:67` — "Provision the toolchain for
  the gate" and "Verify the gate before tagging" (`:79`) each restate the
  release subject with `startsWith(…, 'chore(main): release ')`, "Push tag
  for a merged release PR" (`:89`) runs the script with no `if:`, and
  `version` in `scripts/release/push-release-tag.sh:46` decides the same
  question with its own `sed` regex.
- `web/e2e/a11y.spec.ts:141` — `issuesSnapshot` and `pagedSnapshot`
  (`web/e2e/layout.spec.ts:351`) are untyped literals of the empty snapshot
  shape that `makeSnapshot` (`web/src/test/fixtures.ts:23`) builds typed.
- `web/src/features/branch/CommitForm.tsx:12` — `defaultCommitTypes` copies
  the eleven Go types (`internal/convention/commit.go:41`) in order, and
  `useCommitTypes` (`web/src/features/branch/CommitForm.tsx:131`) falls back
  to it when `config.commit.types` is empty; `server.commitConvention`
  (`internal/webserver/commit.go:73`) resolves the same empty list through
  `convention.NewCommitConvention`, so the server's default is never sent,
  and no test compares the two.
- `web/src/shell/themeStore.ts:11` — `storageKey` is `'workflow-theme'` with
  a "Keep the two in step" comment and is not exported; `web/index.html:25`
  spells it again in the pre-paint script, `web/src/shell/theme.test.tsx:21`
  pins the store's copy only, and the e2e theme case
  (`web/e2e/theme.spec.ts:12`) stores `'system'`, which resolves as an
  unread key does.
- `web/src/features/settings/SettingsPanel.tsx:59` — `secondaryButton`
  carries `text-foreground` where every other copy has `text-sm`; `control`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:27`) lacks the
  `disabled:` classes; and the same string is inline in `AnnouncePreview`
  (`web/src/features/messaging/MessagingPanel.tsx:261`), `PushButton`
  (`web/src/features/branch/BranchPanel.tsx:144`), `PullRequestForm`
  (`web/src/features/review/ReviewPanel.tsx:284`), `CheckoutButton`
  (`web/src/features/issues/WorkStory.tsx:267`), `FollowUpOffer`
  (`web/src/features/review/OpenedOutcome.tsx:88`) and `IssueUnread`
  (`web/src/features/issues/IssueDetailPanel.tsx:83`), with the primary's
  inline in `CommitForm` (`web/src/features/branch/CommitForm.tsx:108`).
  The primary button's three sizes are UX-113; one `Button` component (or
  one primary and one secondary class) closes both.
- `internal/cli/cli.go:391` — `connectLeniently` discards `os.UserHomeDir`'s
  error under a "not a failure" comment (`:390`); `loadFromEnvironment`
  (`:477`), `statusesOf` (`internal/cli/status.go:161`) and
  `completeAssignedIssues` (`internal/cli/scriptable.go:133`) repeat the
  discard and the comment, and `targetDir`
  (`internal/cli/config_cmd.go:128`) is the one caller that must keep the
  error.
- `internal/convention/commit.go:24` — `commitType` is `^[a-z][a-z0-9]*$`
  while `scopeInSubject` (`internal/convention/scopes.go:15`) is
  `^[a-z]+\(([^)]+)\)!?:`; the two disagree on a digit.

A whitespace-only scope is recorded by the terminal and not the web, and the
next composer opens on it; a configured `hotfix2` type validates but its
scopes are never suggested; a sixth progress stage compiles and panics in
the spine; a `.ghe.com` tenant is told by `doctor` to set what the code
could infer; a renamed theme key in `web/index.html` passes every gate and
shows only as a flash before first paint; a drifted release prefix pushes a
tag the gate never saw, or silently tags nothing; a required snapshot field
added to the contract dies in the e2e specs as a locator timeout; a change
to the focus ring is eight edits; and a wording change to the repository
sentence is made twice beside a comment that says there is one.

**One way to fix it.** One owner per fact: `gitrepo.ErrNotARepository`
rendered through the failure block with `notInRepository` deleted; a
`loop.RememberScope` both surfaces call, with one trim; a run-kind value
carrying title and headline; a system field on `progress.Stage` the spine
looks up in a map `exhaustive` checks; `kindOf` consulting `githubsOwn`; a
terminal test that reads the action names out of the configuration page and
holds `CheckKeys` to them; one base-URL rule in `config` that `jira` wraps;
the tag script owning the release-commit decision and the workflow reading
its answer; `satisfies Snapshot` on the e2e literals; the server sending the
effective commit types so the form holds no list; an exported `storageKey` a
test checks `web/index.html` against; one `Button` component or two class
constants; one `configHome()` the four callers share; and `scopeInSubject`
built from `commitType`'s class.

**Done when.** `grep 'inside a git repository' internal/tui` finds one
string; a test shows `' '` is recorded by neither surface; no run-title
literal appears in more than one file and each kind's headline has a test;
the spine's hue comes from a field on `progress.Stage` and `exhaustive`
fails the build when a system has no hue;
`ParseRemote("git@acme.ghe.com:owner/repo.git").Kind == KindGitHub`; a test
fails when the configuration page's action list and the bind sites differ;
one function decides a base URL's shape and both packages' tests import it;
`chore(main): release` appears in exactly one file under `.github/` and
`scripts/`; removing a required snapshot field from the e2e literals fails
`tsc -b`; `grep -n "'revert'" web/src/features/branch/CommitForm.tsx` is
empty and the options come from a server field; a test fails when
`web/index.html`'s key differs from `themeStore`'s; `grep -rn "border
border-input px-3 py-1.5" web/src --include='*.tsx'` matches one definition
site; `grep -n UserHomeDir internal/cli/*.go` returns the helper and
`targetDir`; and `Scopes([]string{"hotfix2(api): x"})` returns `["api"]`.

### DEBT-94 Arms and guards no input can reach, on every surface

Severity: low · Confidence: read

Arms and guards kept just in case that no input can reach. Each is YAGNI by
CLAUDE.md's catalog; most sit permanently in DEBT-64's worklist where no
test can close them, the two compound guards' dead first operands among
them.

- `internal/cli/status.go:413` — `statusGlyph`'s `default` arm repeats the
  `NotStarted` case; `stateWord` (`:438`) and `ciWord` (`:454`) do the same,
  and the gobco report shows each last case true many times and never false.
  `exhaustive` (`.golangci.yml:89`) checks switch and map, so a missing enum
  case already fails lint and the default arms guard nothing.
- `internal/cli/pr.go:185` — the two dry-run lines of `offerLink` and
  `offerReviewStatus` (`:244`) never print; UX-127 makes them print, which
  closes this arm.
- `internal/messaging/messaging.go:112` — `Client.checkable` returns
  `ErrNoCredential` for `config.MessagingNone` and again in `default:`
  (`:114`); `markupFor` (`internal/messaging/post.go:267`) returns
  `slackMarkup()` for `config.KindSlack` and again in `default:`.
- `internal/wiring/forgecli.go:58` — `forgeProgram`'s `case
  forge.KindUnknown:` and `default:` (`:60`) both return `"", false`; the
  gobco report lists the `KindUnknown` condition as never evaluated, and
  DEBT-64 counts `forgeProgram` among the two no black-box test reaches.
- `web/src/features/review/ReviewPanel.tsx:32` — `ReviewPanel` returns null
  under `if (!snapshot)`, the file's only uncovered line; `BranchPanel`
  (`web/src/features/branch/BranchPanel.tsx:18`), `IssuesPanel`
  (`web/src/features/issues/IssuesPanel.tsx:22`) and `MessagingPanel`
  (`web/src/features/messaging/MessagingPanel.tsx:17`) carry the guard
  verbatim, each uncovered, while `SectionPanel`
  (`web/src/shell/SectionPanel.tsx:20`) returns the connecting state before
  any of the four renders.
- `internal/webserver/announce.go:34` — `server.Announce` guards
  `request.Body == nil`; so do `server.Commit`
  (`internal/webserver/commit.go:32`), `server.UpdateConfig`
  (`internal/webserver/config.go:98`) and `server.OpenPullRequest`
  (`internal/webserver/pullrequest.go:39`), never true in the gobco report,
  and `server.Checkout` (`internal/webserver/checkout.go:34`) and
  `server.CreateBranch` (`internal/webserver/branchcreate.go:40`) carry it
  as a dead first operand, never true there either. Every one of those
  bodies is `required: true` in `api/openapi.yaml:572` (and `:637`,
  `:426`, `:700`, `:474`, `:510`), and the strict handler sets
  `request.Body = &body` unconditionally after a decode
  (`internal/api/server.gen.go:2083`).
- `internal/webserver/errors.go:57` — `codeMeaning`'s `default` arm
  duplicates the `api.Internal` arm, `code == api.Internal` 14 times true
  and never false, where `ciState` (`internal/webserver/dto.go:182`) states
  the package's own rule: a map, not a switch, so there is no last-case arm
  gobco can never see.

The condition figure is held down where no test can raise it; the web's
review panel carries an uncovered line nothing reaches; the next panel and
the next handler copy the guard; and `codeMeaning` keeps the last-case arm
`ciState` says the package avoids with a map.

**One way to fix it.** Let each `default` be the one terminal return, or
drop it and let `exhaustive` guard the switch; delete the six nil guards and
dereference the body as `Stage` and `Unstage` already do, keeping the
empty-field checks on checkout and createBranch; build `codeMeaning`'s table
as a map keyed by `api.ProblemCode` as `ciState` is; and have
`SectionPanel` pass the snapshot's parts as props so no panel guards null.

**Done when.** `task cover:branch` no longer lists the three glyph and word
switches in `internal/cli/status.go`, `checkable` in
`internal/messaging/messaging.go`, `forgeProgram` in
`internal/wiring/forgecli.go`, or the body guards in
`internal/webserver/announce.go`, `internal/webserver/commit.go`,
`internal/webserver/config.go`, `internal/webserver/pullrequest.go` and
`internal/webserver/errors.go`; `internal/webserver/checkout.go` reads
`request.Body.Branch == ""` and `internal/webserver/branchcreate.go`
`request.Body.IssueKey == ""` with `TestCheckoutRejectsAnEmptyBranch` and
`TestCreateBranchRejectsAnEmptyIssue` still passing; the Internal arm exists
once; and the v8 summary lists no uncovered null return for the four panels
and none has a null check.

### DEBT-105 The contributor documents restate counts and names the tree has moved past

Severity: low · Confidence: read

The documents a contributor and a later session read first restate numbers
and names the tree has moved past. No gate reads any of them.

- `CLAUDE.md:101`, `ARCHITECTURE.md:14` and `FEATURES.md:54` — each pairs
  `CGO_ENABLED` with the same count: "the release binaries cross-compile to
  five platforms", "so it cross-compiles to five platforms", "because the
  release cross-compiles to five platforms". `RELEASE_PLATFORMS`
  (`Taskfile.yml:73`) names three GOOS/GOARCH pairs, mirrored by the binary
  table in `.github/workflows/release.yml:108`, and `CONTRIBUTING.md:208`
  already says so: "macOS (arm64), Linux (amd64) and Windows (amd64)".
- `CLAUDE.md:74` — the `task lint` row's parenthetical lists twelve checks;
  the `lint` task (`Taskfile.yml:290`) runs sixteen sub-tasks, and the row
  omits `lint:packagesize` (`Taskfile.yml:301`), `lint:goversion`,
  `lint:goroutines` and `gen:verify`. `CLAUDE.md:156` says the package-size
  gate runs in `task lint`, contradicting the row in the same file.
- `docs/content/docs/contributing.md:50` — the `task lint` row names nine
  checks and omits `lint:markdown`, `lint:toml`, `lint:filelength`,
  `lint:packagesize`, `lint:goversion`, `lint:goroutines` and `gen:verify`.
- `CLAUDE.md:503` — the never-print-a-secret rule names `slack.token`, a key
  the decoder refuses: `ErrSlackRenamed` (`internal/config/config.go:50`)
  says the "slack" block was renamed to "messaging", and the field is
  `Messaging.Token` (`internal/config/config.go:106`, `json:"token"`).
- `CLAUDE.md:9` — the opening line names Slack alone where the same file's
  layout row (`CLAUDE.md:43`) names "Slack, Teams, Discord or a plain
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

What is open here is the answers the server gives when a read or a write
fails, the reads it repeats on every frame, the contract's prose, and the
rules the browser derives for itself; what the web's gates still lack — an
end-to-end run that drives a write — is DEBT-65, with the other gates below.

### DEBT-127 A merged pull request reads "Ready for review" on the web

Severity: medium · Confidence: read

The wire `PullRequest` carries no state, so the web cannot tell a merged
pull request from an open one, though the forge hands it both and every
other consumer branches on `State`. Finishing the branch from the web, which
would close the window, is FEAT-79.

- `api/openapi.yaml:1304` — `PullRequest`'s `required` list is number,
  url, title, draft, approvals, changes_requested and mergeable; no state
  field.
- `internal/webserver/dto.go:134` — `pullDTO` maps `forge.PullRequest`
  onto the wire and drops `State`.
- `internal/forge/pulls.go:213` — `pickPull` returns the merged pull
  request with found true when no open one exists, as `IsOpen`'s comment
  at `internal/forge/pulls.go:89` warns callers.
- `internal/cli/cli.go:305` — `WebDeps` wires `FindPull` to the raw
  `FindPullRequest`, so the web sees a merged pull as found.
- `internal/webserver/stream.go:188` — `snapshotReview` passes that found
  through to `review` unchanged.
- `internal/webserver/handlers.go:182` — `server.review` calls `CheckCI`
  for any found pull, merged included, on every snapshot; the terminal's
  `checkCI` (`internal/tui/review.go:115`) returns early unless
  `State == StateOpen`.
- `web/src/features/review/ReviewPanel.tsx:97` — `PullRequestSummary`'s
  State row is `pull.draft ? 'Draft' : 'Ready for review'`, the only two
  values it can show.
- `docs/content/docs/web.md:94` — "### Review" promises the section shows
  the pull request's state.

From the moment a pull request merges until the branch is finished, the
browser says State: Ready for review, shows a Mergeable row ("Mergeability
unknown" on GitHub, whose `githubFind` reads mergeability only while open,
`internal/forge/github.go:99`; "No conflicts" is possible on GitLab, whose
`gitlabMerge.pullRequest` maps `merge_status` for any state,
`internal/forge/gitlab.go:42`) and lists CI checks read against the merged
head. A user could wait on a review that already happened. The terminal's
`reviewRail` says "merged" instead (`internal/tui/review.go:273`),
`gatherReview` (`internal/cli/status.go:285`) treats a merged pull as no
open review, and `momentOf` (`internal/loop/announce.go:142`) never asks CI
about one. The server also spends one forge request per stream tick asking
CI about a pull that has no live CI. No test in
`internal/webserver/review_test.go` or
`web/src/features/review/ReviewPanel.test.tsx` builds a merged pull
(neither mentions `StateMerged` or "merged", by grep).

**One way to fix it.** Add a `state` enum (`open`, `merged`) to the wire
`PullRequest`, map `forge.PullRequest.State` in `pullDTO` and regenerate
both clients (`task gen`, `yarn gen`); have `server.review` skip `CheckCI`
for a pull that is not open, as the terminal does, and have the State row
say "Merged" and omit the CI section.

**Done when.** `web/src/features/review/ReviewPanel.test.tsx` renders a
snapshot whose pull is merged and finds the text "Merged" and no "Ready for
review"; an `internal/webserver/review_test.go` case with a merged pull
records no `CheckCI` call; `task gen` and `yarn gen` leave no diff.

### DEBT-128 Three mock fixtures show a shape the server never sends

Severity: low · Confidence: read

The `VITE_MOCK` build that `task web:mockup`, the layout and a11y specs and
the audit's screenshots use carries three values the real server would
never send, so a reader of the mockup judges an inconsistency that exists
only in the fixture, or learns a placeholder the product would render
literally. It is the fake that cuts a corner.

- `web/src/dev/mockIssues.ts:18` — `mockIssueDetail` sets
  `assignee: 'ana.lopez'`, a username, beside `reporter: 'Ana Lopez'` at
  `web/src/dev/mockIssues.ts:17`; `toIssueDetail`
  (`internal/jira/detail.go:179`) fills both from `DisplayName`.
- `web/src/dev/mockConfig.ts:47` — `mockConfig.branch` sets
  `template: '{type}/{key}-{slug}'`; `BranchNaming.Name`
  (`internal/convention/branch_naming.go:65`) replaces only `{prefix}`,
  `{key}` and `{slug}`, so `{type}` would stay in the branch name. Settings
  draws that value directly under `BranchFieldset`'s hint "Uses {prefix},
  {key} and {slug}; must contain {key}."
  (`web/src/features/settings/fieldsets/BranchFieldset.tsx:12`).
- `web/src/dev/mockConfig.ts:27` — `mockConfig.messaging` sets
  `announcement: 'Opened {pr} for {issue}'`; `Messaging.Announcement`'s doc
  comment (`internal/config/config.go:123`) names the seven placeholders —
  {author}, {noun}, {title}, {url}, {key}, {summary}, {issue_url} — and
  `Announcement.rendered` (`internal/messaging/post.go:346`) substitutes
  only those, so `{pr}` and `{issue}` would post literally. Beside it,
  `previewAnnouncement` (`web/src/features/messaging/announceApi.ts:9`)
  answers the built-in wording rather than that template's rendering.

A designer reading the mock Issues detail sees "Reporter Ana Lopez" over
"Assignee ana.lopez" and reads a product inconsistency that is not there; a
reviewer of Settings learns `{type}` from a value drawn under a hint that
contradicts it; anyone copying the mock announcement's syntax into a real
file gets a literal post, and the mock Messaging section previews a text
the mock template could not produce.

**One way to fix it.** `assignee: 'Ana Lopez'` in `mockIssueDetail`; the
documented default `'{prefix}/{key}-{slug}'` in `mockConfig.branch`; an
announcement written in the documented placeholder set in
`mockConfig.messaging`, with the mock preview in
`web/src/features/messaging/announceApi.ts` being that template rendered
with the mock snapshot's values.

**Done when.** The mock Issues detail shows a display name under Assignee;
`mockConfig.branch.template` contains no placeholder outside `{prefix}`,
`{key}` and `{slug}`; `mockConfig.messaging.announcement` contains only the
seven documented placeholders, and the mock preview text equals that
template rendered with the mock snapshot's values.

### DEBT-129 Announce and draft tell a forge outage as "no pull request"

Severity: medium · Confidence: read

`server.announcement` and `server.composePullRequest` reduce the loop's
error to a bool, so a `Branch` or `FindPull` read that fails is answered
409 "there is no pull request to announce" or "there is nothing to open a
pull request for" instead of being classified through `fault`, as
`GetReview` and `LinkPullRequest` already classify the same seam failures
(502 for an unreachable forge).

- `internal/webserver/announce.go:78` — `server.announcement` returns
  `announcement, err == nil`, collapsing every `ComposeAnnouncement` error,
  wrapped read failures included, into ok false.
- `internal/webserver/announce.go:22` — `GetAnnouncement` answers
  `nothingToAnnounce` 409 for any `!ok`, so a read failure is told as an
  absent pull request.
- `internal/webserver/announce.go:44` — `Announce` answers the same 409
  for the same collapsed error.
- `internal/loop/announce.go:120` and `internal/loop/announce.go:125` —
  `pullToAnnounce` wraps a `Branch` and a `FindPull` failure as "reading
  the branch: %w" and "reading the pull request: %w", without
  `ErrNoPullRequest`, so the server could tell them apart and does not.
- `internal/webserver/pullrequest.go:116` — `server.composePullRequest`
  returns `draft, branch, err == nil`, folding `branchToOpen`'s "reading
  the branch: %w" (`internal/loop/pull.go:95`) into `nothingToOpen` at
  `GetPullRequestDraft` (`internal/webserver/pullrequest.go:26`) and
  `OpenPullRequest` (`internal/webserver/pullrequest.go:49`).
- `internal/webserver/announce_test.go:316` —
  `TestAnnouncingIsAConflictWithoutAPullRequest`'s case "the forge cannot
  be reached" pins the 409 for a `FindPull` error, the wrong answer.
- `docs/content/docs/errors.md:90` — "## Unreachable" reserves 502 for an
  upstream that could not be reached, which these four handlers never
  give.

With the forge down or the branch unreadable, GET /api/announcement, POST
/api/announce, GET /api/pull-request/draft and POST /api/pull-request all
say there is nothing to act on; the person may conclude the pull request
was never opened.

**One way to fix it.** Return the error from `announcement` and
`composePullRequest`, answer 409 only for `loop.ErrNoPullRequest`,
`loop.ErrNothingToOpen` and `loop.PullAlreadyOpenError`, and route any
other error through `fault`. The 409's wording per cause
(`PullAlreadyOpenError` naming the pull request) is UX-129; make the two in
one change.

**Done when.** A test where `FindPull` returns `forge.ErrUnreachable`
wrapped with a host makes GET /api/announcement and POST /api/announce
answer 502 `unreachable` with no host in the body; a test whose `Branch`
read fails makes GET /api/pull-request/draft answer through `fault` rather
than 409.

### DEBT-130 A git read that fails gets three answers, one of them verbatim

Severity: medium · Confidence: read

A `Branch`, `Branches` or `Changes` read that fails is answered three ways
across the handlers: a bare 500 through `fault` on `GetReview`,
`LinkPullRequest` and staging; a generic 422 on `Push`, `Checkout` and
`CreateBranch`; and a verbatim 422 on `Commit`, whose default arm forwards
`err.Error()`. `Repository.ReadBranch` words its failure with the repository's
on-disk path (`internal/gitrepo/branch.go:136`, "reading the current branch of
"+r.dir), where "`detail`" in `docs/content/docs/errors.md:31` promises a read
failure stays generic. `faultClasses` has no git class, so each handler decides
for itself.

- `internal/webserver/commit.go:64` — `server.Commit`'s default arm puts
  `err.Error()` in the 422 detail for every error but `ErrNothingStaged`.
- `internal/webserver/commit.go:87` — `server.commitStaged` returns the
  raw `Changes` read error.
- `internal/webserver/commit.go:102` — `server.commitStaged` returns the
  raw `Branch` read error.
- `internal/webserver/push.go:34` — `server.Push` answers a generic 422
  "the branch could not be read" for the same failure.
- `internal/webserver/checkout.go:83` — `server.refuseADirtyTree` returns
  the raw `Changes` error, which `server.Checkout`'s default arm at
  `internal/webserver/checkout.go:53` words generically.
- `internal/webserver/branchcreate.go:120` — `server.branchExists` reads
  the `Branches` listing, and `server.startWork` (`:87`) returns its raw
  error, which `createBranchFailure`'s default arm at
  `internal/webserver/branchcreate.go:72` words generically; the
  post-create `Branch` read (`:99`) is DEBT-141's.
- `internal/webserver/staging.go:171` — `stagingProblem`'s default arm
  routes the same read failure through `fault`, a bare 500.
- `internal/webserver/issuewrite.go:63` — `server.branchPull` wraps the
  `Branch` read and routes it through `fault`;
  `TestLinkReportsWhatItCouldNotRead`
  (`internal/webserver/issuewrite_test.go:199`) pins the 500.
- `internal/webserver/handlers.go:151` — `server.GetReview` routes the
  same failure through `fault`.
- `internal/webserver/errors.go:126` — `faultClasses` has no git-read
  class, so `fault` falls to the internal problem for every read failure.
- `internal/webserver/commit_test.go:274`,
  `internal/webserver/commit_test.go:291` and
  `internal/webserver/commit_test.go:308` —
  `TestCommitReportsAChangesReadFailure`, `TestCommitReportsAFailedStart`
  and `TestCommitReportsWhenTheBranchCannotBeReadAfter` assert the status
  alone, so any detail passes.

A script switching on `code` sees `internal` on one write and
`unprocessable` on the next for the same broken repository; on Commit the
repository's absolute path reaches the page, and `runCommit`'s start arm
(`internal/webserver/commit.go:110`) forwards the seam's error the same
way.

**One way to fix it.** Add a git-read class to `faultClasses`
(`gitrepo.ErrNotARepository` and a wrapped read sentinel), route every
handler's read failure through `fault`, keep `err.Error()` on Commit for
`errCommitFailed`'s hook output only, and pin the detail in the commit
tests as `internal/webserver/checkout_test.go` pins its own.

**Done when.** One table test sends the same failing `Branch` seam to
/api/push, /api/checkout, /api/commit and /api/issues/{key}/link, and a
failing `Branches` seam to POST /api/branches, and gets one status and
code; `TestCommitReportsAChangesReadFailure` asserts the
detail names neither the seam error's text nor a path, and passes.

### DEBT-131 The announcement posted may not be the one previewed

Severity: medium · Confidence: read

POST /api/announce composes the announcement again at post time from the
request's channel alone, so when the moment changes between the preview
and the press — CI turns red, the pull request merges — the text sent
differs from the text shown. The terminal and the command line post the
text they previewed.

- `internal/webserver/announce.go:42` — `server.Announce` calls
  `s.announcement()` again at post time.
- `internal/webserver/announce.go:52` — `server.Announce` posts
  `announcement.Text()` of that recomposed announcement, not the previewed
  text.
- `api/openapi.yaml:842` — `AnnounceRequest` requires `channel` only; the
  body carries no text or moment.
- `web/src/features/messaging/announceApi.ts:31` — `announce` sends
  `{ channel }` and nothing else.
- `internal/tui/messagingpreview.go:140` — `messagingPreview.post` sends
  `p.text`, the previewed text.
- `internal/cli/announce.go:142` — `runAnnounce` delivers the same `text`
  it printed.
- `docs/content/docs/web.md:114` — "### The messaging service" promises
  nothing is sent before the second press, which reads as a promise that
  what was shown is what is sent.

The last look the web offers is of a text the server does not hold to; the
outward post can say CI is red when the person approved "opened a pull
request". The window is the time between the two presses, so it is rare
and goes unnoticed, and no announce test flips `CheckCI` between the GET
and the POST.

**One way to fix it.** Carry the previewed text, or its moment, in
`AnnounceRequest` and answer 409 when the composed announcement no longer
matches, so the page previews again.

**Done when.** A test whose `CheckCI` flips to failed between GET
/api/announcement and POST /api/announce sees the post refused and nothing
sent.

### DEBT-132 Every stream frame re-asks what it could read once

Severity: medium · Confidence: read

`snapshot` calls `s.author()` on every push and the `Author` seam is an
uncached `Whoami` GET, so each open tab spends one forge request per
interval on a value fixed for the session; the same frame calls
`deps.Branch()` three times, each running `ReadBranch`'s seven git
commands; and `review` asks `CheckCI` about a merged pull, where the
terminal skips a pull that is not open (DEBT-127).

- `internal/webserver/stream.go:90` — `server.snapshot` builds
  `messagingDTO(s.config(), s.author())` on every frame.
- `internal/webserver/handlers.go:241` — `server.author` calls
  `s.deps.Author()` with no cache.
- `internal/wiring/forge.go:109` — the `Author` seam runs
  `connection.client.Whoami(ctx)` on every call; only the connection is
  memoized.
- `internal/forge/client.go:121` — `Client.Whoami` is one uncached GET of
  `userPath`.
- `internal/tui/messaging.go:95` — `Model.loadAuthor` skips the read once
  `m.messaging.author` is set; the web diverges on the same seam.
- `internal/webserver/stream.go:153` — `server.snapshotBranch`, the
  frame's first `deps.Branch()`.
- `internal/webserver/stream.go:183` — `server.snapshotReview`, the
  second.
- `internal/webserver/stream.go:121` — `server.currentBranchName`, the
  third, called from `snapshotBranches`.
- `internal/gitrepo/branch.go:141` — `Repository.ReadBranch` runs
  `branch --show-current`, `rev-parse HEAD`, `rev-parse @{upstream}`,
  `config --get remote.pushDefault`, the base lookup, `rev-list`, `log` and
  `log -1` per read.
- `internal/webserver/handlers.go:182` — `server.review` calls `CheckCI`
  whenever a pull is found; `Model.checkCI` (`internal/tui/review.go:115`)
  does not unless `State == StateOpen`.
- `internal/forge/githubci.go:41` — `githubStatus` pages both the combined
  status and the check runs, at least two requests per frame.

On GitHub with an open pull a frame is at least six forge requests —
`githubFind`'s one plus `githubReviewState`'s two, `githubStatus`'s two,
`Whoami`'s one — about 4,320 an hour for one tab at the 5 s default
(`defaultStreamInterval`, `internal/webserver/stream.go:20`) against GitHub's
5,000-an-hour limit; when the limit is hit, `snapshot` folds the failure into
empty panels with no signal (its comment at `internal/webserver/stream.go:81`
says so). About twenty-six git processes per frame — three branch reads of
eight commands each, plus the changes and branches reads — where ten would do,
and a branch, review and in-flight marker read at three instants, so the panels
can describe different branches when a checkout lands between the reads.

**One way to fix it.** Read the author once per server, as `scopeCache`
(`internal/webserver/webserver.go:139`) reads the scope, re-reading only
after a failure; and read the branch once in `snapshot` and pass it to the
review and branches builders.

**Done when.** A stream test with counting `Author` and `Branch` seams sees
one `Author` call across three pushed snapshots and one `Branch` call per
pushed snapshot (the merged-pull `CheckCI` skip is DEBT-127).

### DEBT-133 The errors page points a 500's cause at output nothing writes

Severity: medium · Confidence: read

"## Internal" in `docs/content/docs/errors.md:103` says the cause of a 500
"is in the server's own output, not the response". Both places that answer
`internal` discard the error: `writeResponseError`
(`internal/webserver/errors.go:84`) ignores its error argument, and
`faultProblem` (`internal/webserver/errors.go:112`) returns the generic
internal problem without recording `err`. No non-test file in
`internal/webserver` writes a log line (zero matches for `log.`, `slog` or
`Stderr`, by grep) and nothing sets an `ErrorLog`. The notes writer the
command line hands `serve` (`internal/cli/cli.go:199`, `cmd.ErrOrStderr()`)
carries only the address line `WebServerAt` prints
(`internal/cli/cli.go:281`).

A user who meets a 500 is told to look at the terminal and finds only the
address line; the unclassified seam error is gone, so neither the user nor
a bug report can say what failed. `--log` (`internal/cli/cli.go:214`)
outlines each request's method, path, status and duration, not the cause.
CLAUDE.md says never to swallow an error.

**One way to fix it.** Write the discarded error to the notes writer the
command line already hands `serve`, one line per internal problem, through
a func-var seam on `Handler`; or drop the sentence from
`docs/content/docs/errors.md`.

**Done when.** A test with a failing seam that no fault class matches sees
the error's text in the notes writer, or `docs/content/docs/errors.md` no
longer says the cause is in the output.

### DEBT-134 Four GET reads no caller uses, three of them written twice

Severity: low · Confidence: read

GET /api/branch, /api/changes, /api/review and /api/messaging are called by
nothing in `web/src` or `web/e2e` outside the generated client: the page
reads the snapshot, and the one match by grep,
`web/src/api/client.test.tsx:84`, uses `/api/branch` as a sample URL for
the client wrapper's dry-run test. No doc names them for scripts, and three
snapshot builders repeat the handlers line for line with a different
failure answer.

- `internal/webserver/handlers.go:112` — `server.GetBranch`, a handler
  with no caller, is the same nil-seam, read, DTO sequence as
  `server.snapshotBranch` (`internal/webserver/stream.go:148`), which
  differs only in answering an empty branch on failure.
- `internal/webserver/handlers.go:128` — `server.ListChanges`, no caller;
  `server.snapshotChanges` (`internal/webserver/stream.go:163`) is the
  same read with `changesDTO(nil)` on failure instead of `fault`.
- `internal/webserver/handlers.go:144` — `server.GetReview`, no caller:
  `Branch`, `FindPull`, `review` through `fault`, where
  `server.snapshotReview` (`internal/webserver/stream.go:178`) runs the
  identical sequence with failures as `Found: false`.
- `internal/webserver/handlers.go:228` — `server.GetMessaging`, no caller:
  the one-line `messagingDTO` that `server.snapshot` also builds at
  `internal/webserver/stream.go:90`.
- `docs/content/docs/scripting.md:55` — "### The same families on the web"
  names the problem codes for scripts but no read endpoint.

Four handlers and their tests exist for a caller that does not exist; the
snapshot's copy is the one the page uses, so a change to how a branch, the
changes or the review is read must be made in both files, and a failure
answer proven on the handler is not proven on the copy the page reaches.

**One way to fix it.** Decide whether the four reads are the script surface
(then `docs/content/docs/web.md` names them) or not (then drop them from
the spec); either way have each snapshot builder call the one read function
so the logic lives once.

**Done when.** Either `docs/content/docs/web.md` lists GET /api/branch,
/api/changes, /api/review and /api/messaging as scriptable reads, or they
are gone from `api/openapi.yaml` and `task gen` leaves no `GetBranch`,
`ListChanges`, `GetReview` or `GetMessaging`; and each snapshot builder
calls the shared read function.

### DEBT-135 The contract's prose disagrees with the code at eight places

Severity: low · Confidence: read

`api/openapi.yaml` describes behavior the server does not have.
kin-openapi's `Validate` passes on all of it (`loadSpec`,
`internal/webserver/validator.go:29`), so no gate sees it; a client written
from the description is what it hurts.

- `api/openapi.yaml:603` — `push`'s description promises 409 "when there
  is nothing to push (no commits, or already up to date)"; `nothingToPush`
  (`internal/webserver/push.go:59`) consults only the name and the
  upstream on the push remote, deliberately (its comment at
  `internal/webserver/push.go:49`), so a branch with no upstream and no
  commits is pushed.
- `api/openapi.yaml:45` — the `events` tag says snapshots are "pushed as
  they change", and `streamEvents`'s summary at `api/openapi.yaml:727`
  says the same; `defaultStreamInterval`'s comment
  (`internal/webserver/stream.go:19`) says the server re-reads on a
  cadence and pushes the result, with no comparison to the previous frame,
  as "**The stream**" in `docs/content/docs/web.md:46` also says.
- `api/openapi.yaml:4` — the header comment credits `task gen:verify` with
  failing CI "if either drifts"; `gen:verify` (`Taskfile.yml:433`) diffs
  only `internal/api`, and the web client is checked by `web:gen:check`
  (`Taskfile.yml:179`).
- `api/openapi.yaml:472` — `checkout` carries `tags: [branches]`, as do
  `createBranch` (`api/openapi.yaml:508`), `push` (`api/openapi.yaml:605`)
  and `commit` (`api/openapi.yaml:635`); the `tags` list at
  `api/openapi.yaml:29` never declares `branches`.
- `api/openapi.yaml:22` — the `info` description says the server "keeps
  nothing between requests but the commit scope it learns"; `server` holds
  `cfg` and `seen` (`internal/webserver/webserver.go:121`) across
  requests, and `getConfig`'s own description at `api/openapi.yaml:380`
  relies on it ("A file that has been deleted leaves the configuration in
  effect as it was").
- `api/openapi.yaml:328` — `getReview`'s 200 says "pull and ci are null
  when none is found"; `Review` in `internal/api/models.gen.go:692` marks
  both `omitempty` and `server.review`
  (`internal/webserver/handlers.go:170`) leaves them nil, so they are
  absent, as the not-found frame in `web/src/test/snapshot-frames.sse:7`
  shows.
- `api/openapi.yaml:1070` — `Issue.priority` "May be empty"; `issueDTO`
  (`internal/webserver/dto.go:33`) passes it through `optional`, which
  sends an empty string as an absent field.
- `api/openapi.yaml:1168` — `Change.original_path` is "empty otherwise";
  `changesDTO` (`internal/webserver/dto.go:122`) passes it through
  `optional`, which omits it.

A client written to the description checks `pull === null` or
`priority === ''` and never matches, expects a 409 it never gets, and
expects a quiet event-driven stream; a contributor who runs `gen:verify`
after a spec change may believe the TypeScript client is current;
`getConfig`'s own description depends on state the `info` block says does
not exist.

**One way to fix it.** One pass over `api/openapi.yaml`: reword the push
409 as "a detached HEAD, or a published branch that is not ahead"; say
snapshots are pushed on connect and every few seconds; name `web:gen:check`
in the header; retag the four operations as `repository` or declare
`branches`; say the server keeps the configuration in effect and the
learned scope; say "absent" for pull, ci, priority and original_path.

**Done when.** The push description and `nothingToPush`'s comment name the
same two cases; the `events` tag and the `streamEvents` summary match
`defaultStreamInterval`'s comment; the header names `web:gen:check`; every
tag an operation uses appears in the top-level `tags` list; the `info`
description names the configuration in effect; the review 200 description
matches the not-found frame in `web/src/test/snapshot-frames.sse`, which
carries no `pull` key; the three optional strings use one word for one wire
shape; and `task gen` leaves the generated code unchanged.

### DEBT-136 A wrong method on a known path is answered 404, not 405

Severity: low · Confidence: read

The validator's router returns a nil route with
`routers.ErrMethodNotAllowed` for a method mismatch; nethttp-middleware
v1.2.0 (`go.mod:12`) then reports status 404 whenever the route is nil, and
`writeValidationError` (`internal/webserver/validator.go:56`) turns any 404
into `not_found` "no such endpoint" (`internal/webserver/validator.go:57`),
so POST /api/branch or DELETE /api/config is told the endpoint does not
exist rather than 405 with `Allow`. `TestAnUnknownEndpointIsNotFound`
(`internal/webserver/serve_test.go:82`) covers an unknown path only; no
test sends a wrong method to an /api path, and
`TestTheAppRefusesANonReadMethod` (`internal/webserver/static_test.go:131`)
pins 405 for the app only.

A script that mistypes the verb is sent looking for a typo in the path; the
`net/http` mux behind the validator would have answered 405 on its own.

**One way to fix it.** In `writeValidationError`, test
`errors.Is(err, routers.ErrMethodNotAllowed)` before the 404 branch and
answer 405 with a problem (a new enum code, added to
`docs/content/docs/errors.md`), or let the request through to the mux.

**Done when.** A test sending POST /api/branch sees 405, not 404.

### DEBT-139 The web derives its own stage rules, and they contradict `progress`

Severity: medium · Confidence: read

`WorkStory` derives the work story's stages in TypeScript with rules that
disagree with `internal/progress`: its Changes stage is done only with a
clean tree and a commit, where `commitState` reads Done on any commit; and
`pullRequestDone` reads done with no CI or with changes requested, where
`reviewState` reads in flight and failed. The package comment that says the
rule "lives in one place" no longer holds. No FEATURES entry carries the
stages in the snapshot; the first fix below is that change.

- `web/src/features/issues/WorkStory.tsx:79` — `onHeadStages`, the second
  derivation: four stages with their own names and rules.
- `web/src/features/issues/WorkStory.tsx:97` — `onHeadStages` marks
  Changes done only when
  `changes.changes.length === 0 && branch.commits.length > 0`;
  `commitState` (`internal/progress/progress.go:101`) returns Done on
  `OnFeatureBranch && Commits > 0` before it reads `UncommittedChanges`.
- `web/src/features/issues/WorkStory.tsx:124` — `pullRequestDone` is done
  unless CI is `running` or `failed`, so a CI state of none reads done and
  `changes_requested` is never read; `reviewState`
  (`internal/progress/progress.go:117`) is Failed on
  `CIFailed || ChangesRequested`, Done only on `CIPassed`, and in flight
  otherwise.
- `internal/progress/progress.go:8` — the package comment: both the spine
  and `workflow status` read it, "so the rule lives in one place".
- `internal/progress/progress_test.go:47` —
  `TestStagesDeriveHowFarTheWorkHasGot` has no case with both
  `Commits > 0` and `UncommittedChanges > 0`, so the Go precedence is
  unpinned; `web/src/features/issues/WorkStory.test.tsx` sets
  `changes_requested` only to false and never a CI state of none.

On a branch with one commit and an edited file, the spine and `status` show
Commits done while the browser shows Changes still active. A repository
without CI reads Review in flight in the terminal and done in the browser;
a reviewer's changes requested reads failed in the terminal and done in the
browser, though the snapshot carries `changes_requested`.

**One way to fix it.** Derive once in Go and carry the stages in the
snapshot (`progress.Stages` over a `Work` the server builds) so the web
renders rather than re-derives; until then port `ChangesRequested` and the
CI-none rule to `pullRequestDone`, add the commits-plus-changes case to
`internal/progress/progress_test.go`, and make the package comment name
every place a stage is derived.

**Done when.** A web test with one commit and one change shows the state
the Go table case gives; one with `review.found`, no CI and
`changes_requested` true shows the review stage failed, matching the Go case
"changes requested stops review reading done"; the commits-plus-changes
case exists in `internal/progress/progress_test.go`; and either the
snapshot schema has a stages array the web renders, or, until then,
`pullRequestDone` reads `changes_requested` and a CI state of none as the Go
table does.

### DEBT-141 A commit, create or checkout that landed is reported failed

Severity: medium · Confidence: read

`commitStaged`, `startWork` and `switchTo` each return the confirming
`Branch()` read's error after `runCommit`, `CreateBranch` or `Checkout` has
already moved the tree, and the handlers' default arms answer 422 — git's
read error verbatim for the commit, "could not be created; try again" and
"could not be checked out; try again" for the branches — so a write that
landed is told as a failure, where `publishedBranch` in the same package
deliberately answers the pre-write state when the re-read after a push
fails.

- `internal/webserver/commit.go:102` — `server.commitStaged` returns
  `s.deps.Branch()` after `runCommit` succeeded; the read's error becomes
  the commit's failure.
- `internal/webserver/commit.go:64` — `server.Commit`'s default arm
  answers that error as 422 with `err.Error()` as the detail.
- `internal/webserver/commit_test.go:308` —
  `TestCommitReportsWhenTheBranchCannotBeReadAfter` pins the 422 for a
  commit that ran.
- `internal/webserver/branchcreate.go:99` — `server.startWork` returns
  `s.deps.Branch()` after `createAndSwitch` ran.
- `internal/webserver/branchcreate.go:72` — `createBranchFailure`'s
  default arm says the branch "could not be created; try again" though it
  exists.
- `internal/webserver/branchcreate.go:87` — a retry then reaches
  `branchExists` in `server.startWork` and answers 409 `errBranchExists`,
  contradicting the 422.
- `internal/webserver/branchcreate_test.go:259` —
  `TestCreateBranchReportsWhenTheNewBranchCannotBeRead` pins the 422 for
  a branch that was created.
- `internal/webserver/checkout.go:70` — `server.switchTo` returns
  `s.deps.Branch()` after `Checkout` succeeded.
- `internal/webserver/checkout.go:53` — `server.Checkout`'s default arm
  answers 422 "could not be checked out; try again" for that read's error;
  no test covers this read.
- `internal/webserver/push.go:72` — `server.publishedBranch`'s comment: a
  re-read that fails does not undo the push, so the pre-push branch is
  returned.

The commit form shows a red alert with git's read error, and a retry
answers 409 "nothing is staged to commit" while the commit is in the
repository; the create says try again, and the retry answers 409 "a branch
for this issue already exists"; the checkout says the switch failed while
the tree is on the requested branch, until the stream corrects it a frame
later. Two tests pin the 422 with no rationale, and no trade-off records
why the commit and the branches differ from the push.

**One way to fix it.** Mirror `publishedBranch`: after the write succeeds,
answer 200 with the pre-commit branch, or with a branch built from the name
just created or requested, when the confirming read fails (the pinned
create scenario's pre-create read fails too, so the fallback must come from
the created name).

**Done when.** A test whose `Branch` seam fails only on its second read
answers 200 to POST /api/commit; a test whose `Branch` seam fails only
after `CreateBranch` ran answers 200 to POST /api/branches naming the
created branch; and a test whose `Branch` seam fails after `Checkout` ran
answers 200 to POST /api/checkout — each as
`TestPushSucceedsEvenIfTheRereadFails`
(`internal/webserver/push_test.go:188`) does for the push.

### DEBT-142 Two 422 details carry a host the docs keep off the wire

Severity: medium · Confidence: read

`openFailure` curates only `forge.ErrUnreachable` and `ErrUnknownForge`,
and `httpx.Unreachable` wraps a refused redirect as `ErrRedirected` with
`the server at <base>` in its text, so a forge answering a login redirect
makes POST /api/pull-request answer 422 naming the forge's API base — and
a rate limit answers 422 with sentinel text where the read path's
`faultClasses` gives a curated 502. `pushFailure` joins git's push
output verbatim into the detail, which git ends with `To <remote-url>` or
`failed to push some refs to <url>`, and the open reuses it, while staging
keeps git's words off the wire for exactly that reason.

- `internal/webserver/pullrequest.go:182` — `openFailure`'s
  `errors.Is(err, forge.ErrUnreachable)` is false for a redirect, which
  wraps only `ErrRedirected`.
- `internal/webserver/pullrequest.go:189` — `openFailure`'s default arm
  puts `err.Error()`, API base included, in the 422 detail.
- `internal/webserver/pullrequest.go:175` — `openFailure`'s doc comment
  counts two host-carrying failures where there are three.
- `internal/httpx/httpx.go:50` — `Unreachable` wraps a redirect as
  `ErrRedirected` with " at "+base in its text and never the caller's
  sentinel.
- `internal/forge/client.go:218` — `Client.exchange` passes `c.base` into
  that text; nothing re-wraps it before the handler.
- `internal/webserver/errors.go:148` — the `httpx.ErrRedirected` class in
  `faultClasses`, the curated 502 the read path gives for the same
  failure.
- `docs/content/docs/errors.md:31` — "`detail`" promises an unreachable
  upstream stays generic.
- `internal/webserver/push.go:65` — `pushFailure` joins the push's output
  verbatim into the detail.
- `internal/webserver/pullrequest.go:59` — `server.OpenPullRequest` reuses
  `pushFailure`, so the same output reaches its 422.
- `internal/webserver/staging.go:27` — `errGitRefused`'s comment: git's
  own words stay off the wire because a fetch that fails names the remote.
- `internal/webserver/push_test.go:101` — `TestPushReportsAFailingPush`'s
  only push output, "! [rejected] fix/PROJ-412", carries no URL.

A user behind an SSO forge sees the forge address in the browser's alert,
against the errors page's promise; a rejected push can print the remote
URL; and the package holds two policies on git's words.

**One way to fix it.** Have `openFailure` keep `err.Error()` only for
`forge.ErrRejected`, whose reason is the forge's own words, and send every
other error through `faultProblem`; and decide once for the push — strip
lines carrying a URL from the output before it reaches the detail, or write
the exception beside `errGitRefused` so the next reader knows the push is
meant to differ.

**Done when.** A test where `CreatePull` returns `httpx.Unreachable` of
`forge.ErrUnreachable`, an internal API base and `httpx.ErrRedirected`
answers 502 `unreachable` with no host in the body, and a rate-limited
create answers 502; and a test whose push output carries
`To https://git.internal.example/acme/repo.git` answers a detail that keeps
`[rejected]` and omits the host.

## The gates, the build and the tests

What is open here is the coverage worklist and the metric behind it, gates
whose printed sentence claims more than their check measures, CI jobs and
triggers that do not do what their comments say, and tests named or shaped
for something other than what they prove.

### DEBT-64 The condition-coverage worklist: 432 one-sided conditions, and 13 never evaluated

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

Thirteen conditions were never evaluated. Four are a test away:

- `internal/tui/messaging.go:83` and `:85` — `quitGuard.handleKey`'s confirm
  and stay: `TestQuittingWithAQueuedPostAsksFirst` opens the guard but
  presses neither enter (quit) nor esc (stay) in it.
- `internal/tui/prcreate.go:134` — `pullCreated.apply`'s `named` case, a
  Jira issue with no link seam: every test that opens a pull request on a
  Jira issue's branch wires `Jira.LinkPullRequest`.
- `internal/wiring/wiring.go:224` — `streamToEnd`, git failing to start: no
  wiring test fetches or pulls without git on `PATH`.

Seven more came into view once each operand counted, and each is a test
away too:

- `internal/config/config.go:348` and `:364` — `Problems`'
  `absoluteWebURL(base)` and the four operands inside `absoluteWebURL`: no
  `internal/config` test calls `Problems` with a `jira.base_url` set.
- `internal/messaging/post.go:335` — `Announcement.Text`'s `a.Kind ==
  config.KindSlack`: each test that renders a template leaves `Kind` empty,
  so the `||` never reads it.
- `internal/tui/issuekeys.go:136` — `extendFilterWith`'s `msg.Code ==
  tea.KeySpace`: no filter test types a key without text, so `text == ""`
  never lets the `&&` read it.

Two no black-box test reaches without changing the code:

- `internal/tui/tui.go:143` — `tui.Run`'s error return, which needs a real
  terminal.
- `internal/wiring/forgecli.go:58` — `forgeProgram`'s `forge.KindUnknown`
  case, which `exhaustive` requires but `connectForge` never passes, since
  `Repo.APIBase` refuses an unknown forge first.

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
the two above, and reads 92.0 % or more, so `BRANCH_COVERAGE_MIN` ratchets
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

### DEBT-146 `internal/webserver/coverage_test.go` is named for the gate, not for what it tests

Severity: low · Confidence: read

`internal/webserver/coverage_test.go` holds eleven tests over seven
handlers: `TestListIssuesUsesTheNamedView` (`:19`),
`TestGetIssueReportsAFailure` (`:44`), `TestGetBranchReportsAFailure`
(`:60`), `TestListChangesIsEmptyWithoutARepository` (`:76`),
`TestListChangesReportsAFailure` (`:92`),
`TestGetMessagingHasNoAuthorWithoutAForge` (`:108`), two `GetReview` tests
(`:124`, `:158`) and three `UpdateConfig` tests (`:191`, `:220`, `:255`).
The issue, review and config tests have home files beside it
(`internal/webserver/issues_test.go`, `internal/webserver/review_test.go`,
`internal/webserver/config_test.go`,
`internal/webserver/configrevision_test.go`); the `GetBranch`,
`ListChanges` and `GetMessaging` tests have none, their siblings sitting in
`internal/webserver/webserver_test.go` (`:347`, `:359`, `:375`, `:387`,
`:405`, `:424`), a file named for the package; and the file's name says only
why it was written. A contributor looking for the config save tests reads
`internal/webserver/config_test.go` and
`internal/webserver/configrevision_test.go` and misses three.

**One way to fix it.** Move each test beside its handler's tests; move the
`GetBranch`, `ListChanges` and `GetMessaging` tests from both files into
three new test files in `internal/webserver`, one named for each handler;
and delete `internal/webserver/coverage_test.go`.

**Done when.** No `internal/webserver/coverage_test.go` exists,
`internal/webserver/webserver_test.go` holds no `GetBranch`, `ListChanges`
or `GetMessaging` test, and each of the package's other test files is named
for a handler or a concern (`internal/webserver/webserver_test.go` keeps the
package-wide tests).

### DEBT-148 Four clicked steps in the web are never scanned or walked

Severity: medium · Confidence: read

The axe scans reach the issue detail, the opened pull's offers and a staged file
after a click, and the Tab walk runs once, on each section as it opens. Four
steps a user reaches by clicking — the pull request form, the push confirmation,
the announcement preview and a write's refusal — are scanned and walked in
neither theme, and the hermetic Settings scan settles on the heading rather than
on the read's outcome. `CLAUDE.md:239` promises axe across every section in both
themes and every control Tab reaches in view at three widths; for the clicked
steps only jsx-a11y's static rules apply. DEBT-65 (a write against a real
server) does not cover this: it is the runtime floor's reach, not the backend's.

- `web/e2e/a11y.spec.ts:102` — the populated-sections test clicks each of
  `populatedSectionNames` and scans at once; the populated snapshot's
  `review` has `found: true` (`web/src/dev/mockSnapshot.ts:108`), so the
  pull branch is taken and `PullRequestForm` never mounts there.
- `web/e2e/a11y.spec.ts:265` — the offers test clicks "Open a pull request"
  and then "Open pull request" with no scan between compose and submit, and
  `scan` runs (`web/e2e/a11y.spec.ts:273`) after `OpenPullRequest`
  (`web/src/features/review/ReviewPanel.tsx:166`) has returned null on
  `open.state === 'done'`, so the form is gone.
- `web/e2e/a11y.spec.ts:21` — `settled` returns the level-1 heading for
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
  `internal/webserver/pullrequest_test.go` (601, the web's draft and open).
  Staging, committing and pushing: `internal/tui/composer_test.go` (568,
  the terminal's commit composer), `internal/webserver/staging_test.go`
  (533, the web's stage and unstage) and
  `web/src/features/branch/BranchPanel.test.tsx` (517, the web's commit and
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
  `prComposer.onFieldNav`, `internal/tui/prcomposer.go:367`), and each blurs
  every field before focusing one (`commitComposer.focusOn`,
  `internal/tui/composer.go:297`; `prComposer.focusOn`,
  `internal/tui/prcomposer.go:388`). Two is not yet the rule of three, so
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
  (`web/vitest.config.ts:45` `thresholds`), not a gobco-style per-condition
  one: v8 marks a branch covered once its range of code has run, and never
  asks which way each operand of a condition went. The cost is that an
  `a && b` only ever seen with `b` true still passes the web's floor.
