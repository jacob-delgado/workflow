# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up UX-61". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `5e69cb8` on 2026-09-22 (PR #125's tip, merged to main). Line numbers drift, so every pointer
also names the symbol it means.

## How this was produced

One pass, from code only, over three surfaces.

1. **A read of every surface.** Every command, flag and message in
   `internal/cli`; every key binding, overlay, empty state, loading state
   and error state in `internal/tui`; and every panel, button, string and
   endpoint call in `web/src` and `internal/webserver`, read against three
   yardsticks — the [clig.dev](https://clig.dev) guidelines for the command
   line, the promises the terminal interface makes about itself (the table
   below, re-verified), and, for the web, the accessibility floor the gates
   already enforce plus a designer's read of whether the page has an
   identity of its own or a template's.
2. **Nothing was run.** Unlike the previous edition, no binary was driven in
   a terminal and no browser was opened. The screens are known from the
   source and from the golden output the screen tests hold. Where that
   matters — a claim about how something *looks* rather than what the code
   does — the entry says "from code".
3. **A second read while writing.** Every cited line was read again as its
   entry was written; a claim that could not be pointed at a line was
   dropped.

## How to read an entry

- **Impact** and **Effort** are estimates. Effort is small (a day or less),
  medium (a few days) or large (a week or more).
- **Today** is what happens now, with the evidence.
- **Instead** is one proposal. There are usually others.
- **Done when** is observable, so a screen test can assert it.

The sections follow the surfaces and, within the terminal, the panes in the
order the work goes. Entries are not ranked. An entry that is also a debt
points at its [TECH_DEBT.md](TECH_DEBT.md) twin; one that is really a new
feature lives in [FEATURES.md](FEATURES.md) and is only pointed at from here.

## The promises the interface makes

The interface states its own rules, in its docs and in its code. They are
good rules. This table is the shortest summary of how far the screen keeps
them, re-counted at this commit.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "a key it does not show does nothing" | No longer stated anywhere; the sentence the previous edition cited at `usage.md:55` is gone. Folds into the next row. | — |
| "`?` lists every key" | `docs/content/docs/usage.md:56` | **Yes, by construction.** Help is generated from the bindings (`internal/tui/keys.go:134` `helpBuilder.place`, rendered at `render.go:211`): 57 of 57. The one guard is that construction; `render.go:225` skips a binding with empty help silently, and the tests are three-string spot checks (`focus_test.go:124`). |
| "the one way the interface says something broke" | `failure`, `internal/tui/render.go:421` | **Partly, and this is the weakest.** Of 52 places that render an error, 19 go through the `failure` family, but only 11 reach the actionable `errorSentence` (`render.go:401`): `failureBlock` (`render.go:450`) red-wraps the raw chain, so all 8 `pinnedOutcome` overlays show the raw error; 14 sites are `failedGlyph + err.Error()`; 8 are glyph-only rail summaries; 9 are notices with no glyph and no red at all. See UX-67. |
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:60` | **16 of 18.** `R` re-run CI (`internal/tui/review.go:479`, a forge write) and `u` rebase (`internal/tui/run.go:447`, rewrites local history) are one key, straight to the request. See UX-65. |
| "a refused change must never go unseen" | `internal/tui/picker.go:269` | **13 of 13 guard while in flight** (the previous edition counted 1 of 7). 11 keep the refusal in the overlay; `mergePicker` (`internal/tui/review.go:593`) and `finishPreview` (`finish.go:139`) close and demote it to a one-line notice. See UX-66. |
| "Each pane fails on its own" | `docs/content/docs/usage.md:62` | Yes. `tui.go:157` batches six loads; each pane holds and renders its own error. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:16` | Yes. `unicodeGlyphs` and `asciiGlyphs` differ in shape (`glyphs.go:31`, `:43`); `NO_COLOR` keeps bold and faint (`tui.go:139`). One residue: the progress spine's per-system hue is color-only, mitigated by the name or its initial. |

## The command line

### UX-51 Commentary lands on stdout, where a script is reading

Impact: high · Effort: small

**Today.** Errors go to stderr (`SilenceErrors`, `internal/cli/cli.go:171`; `cmd/workflow/main.go:37`)
and prompts go to stderr (`terminalPrompt`, `cmd/workflow/main.go:51`) — correct,
and so does `config show`'s path header, leaving its stdout JSON alone
(`runConfigShow`, `internal/cli/config_cmd.go:291`). But the gitignore
**warning** (`warnIfNotIgnored`, `internal/cli/config_cmd.go:284`), the
`Not posted.`/`Not opened.` decline notices and the `dry run: would …` lines
(`writeOptions.proceed`, `internal/cli/scriptable.go:53`, `:68`), the web
server's `serving http://…` banner (`internal/cli/cli.go:261`), and
`standup`'s and `config init`'s own notices all go to stdout, where a script
capturing the artifact gets prose mixed into it. No test pins any of this
to a stream: the harness can keep them apart (`runStreams`,
`internal/cli/cli_test.go:58`), but only `pr`'s opened line and `config
show`'s JSON are checked that way (DEBT-54).

**Instead.** stdout carries the artifact — the JSON, the preview, the URL
of the thing created, the standup draft; stderr carries everything said
*about* it.

**Done when.** A test asserts the decline notice is on stderr.

### UX-52 The root's flags do not reach the subcommands

Impact: medium · Effort: small

**Today.** `--dry-run`, `--log` and `--web` are declared on `root.Flags()`
(`internal/cli/cli.go:198-203`), not `PersistentFlags()`, so `workflow --dry-run pr` and
`workflow --log f status` are unknown-flag errors; the write commands
declare their own, unrelated `--dry-run` (`internal/cli/scriptable.go:36`) with different
help; and every subcommand passes `nil` for the request log, so the `--log`
facility the root's help advertises for bug reports (`internal/cli/cli.go:202`) works
only for the interface (DEBT-51).

**Instead.** `--dry-run` and `--log` persistent on the root, declared once;
the seven wiring preambles collapse into one that opens the log.

**Done when.** `workflow --dry-run pr` is accepted and previews;
`workflow --log FILE status` appends a request line to `FILE`.

### UX-53 With no terminal, a confirmation fails without saying why

Impact: medium · Effort: small

**Today.** Nothing in `internal/cli` or `cmd/workflow` asks whether stdin is
a terminal. With stdin piped and `--yes` omitted, `branch`, `pr`, `announce`
and `standup` still call `confirm` (`prompt.go:32`); `ReadString` returns
`io.EOF`, which is propagated as an error (`:35`). `TestBranchStopsWhenThe
ConfirmationCannotBeRead` (`internal/cli/branch_test.go:192`) pins that it stops, not
that it explains.

**Instead.** `confirm` turns `io.EOF` into "no terminal to confirm on; pass
`--yes` to proceed without one".

**Done when.** Piped-stdin `workflow pr` exits non-zero with a message that
names `--yes`.

### UX-54 `standup` cannot be previewed or run unattended

Impact: medium · Effort: small

**Today.** `standup` is the only write command with no `--dry-run` and no
`--yes`: after the draft it *offers to post* (`offerToPost`, `internal/cli/standup.go:128`)
with no bypass, so it cannot run from a script, and `--days` is unvalidated
— a negative value flows into `gitSince` as `"-1 days ago"` (`:193`) and into
the JQL as `updated >= --1d` (`:200`). `config init`'s guided flow has no
`--dry-run` either; its only unattended path is `--template`.

**Instead.** `standup` takes the same `writeOptions` as the others and
rejects `--days < 1` as a usage error; `config init --dry-run` prints the
redacted file it would write.

**Done when.** `workflow standup --dry-run --yes` posts nothing and exits 0;
`--days -1` is a usage error.

### UX-55 The generated reference omits `--version`, `help` and `completion`; the usage page never mentions the commands

Impact: medium · Effort: small

**Today.** `--version` works (`Version: buildinfo.Current()`, `internal/cli/cli.go:169`),
but `cmd/docsgen/main.go` never calls cobra's `InitDefaultVersionFlag`, so
`docs/content/docs/reference/workflow.md` lists `--dry-run`, `--help`,
`--log` and `--web` only; `help` and `completion` have no page. And
`docs/content/docs/usage.md` is entirely about the terminal interface — it
never names `status`, `reviews`, `standup`, `branch`, `pr` or `announce` as
commands, and no page explains piping, the `--json` shapes or the exit
codes. The scriptable commands exist in a `README.md:46` bullet list and
the generated reference alone.

**Instead.** docsgen initializes the default version, help and completion
commands before generating; a short `scripting.md` page: exit codes,
streams, `--json`, `--yes`, `--dry-run`.

**Done when.** `task docs:check` is green with `--version` in the reference
and a scripting page in the site.

### UX-56 A typo gets no pointer to `--help`

Impact: low · Effort: small

**Today.** `SilenceUsage` and `SilenceErrors` on the root (`internal/cli/cli.go:170`) are
inherited by every subcommand, and cobra gates its "Run 'workflow --help'
for usage." hint and its suggestion list on `!SilenceErrors`. A mistyped command name prints `workflow: unknown command …` and nothing else.

**Instead.** Keep the flags (they stop the usage dump on a *real* error)
and have `Execute` print the hint and cobra's suggestions on an
unknown-command or unknown-flag error.

**Done when.** A mistyped command name gets the `--help` hint and cobra's closest suggestion.

### UX-57 A refusal tells you what, not what next

Impact: medium · Effort: small

**Today.** The strong messages say the next step — `(pass --force to
overwrite)` (`internal/cli/config_cmd.go:142`), the `chmod 600` line (`doctor.go:456`),
`run gh auth login` (`doctor.go:232`), `Create one with workflow config
init` (`internal/config/config.go:24`). The bare sentinels do not: `a branch for
this issue already exists` (`internal/cli/branch.go:21`) does not say to switch to it;
`an open pull request already exists for this branch` (`internal/cli/pr.go:28`) gives no
URL; `no messaging transport is configured` (`internal/cli/announce.go:23`) names
neither `messaging.kind` nor `workflow config init`; `there is no pull
request on this branch to announce` (`internal/cli/announce.go:19`) does not suggest
`workflow pr`.

**Instead.** Each carries its next step: the branch name and `git switch
NAME`; the pull request's URL; the config keys and `workflow doctor`;
`workflow pr`.

**Done when.** Every sentinel's message contains a command or a key the
user can act on, asserted per sentinel.

### UX-58 `branch` switches your working tree and does not say so; `pr` pushes, transitions, and asks about one thing

Impact: medium · Effort: small

**Today.** `workflow branch` runs `git switch --create`
(`internal/gitrepo/branch.go:305`), moving the working tree, but neither its help
(`internal/cli/branch.go:41`) nor its preview (`:97`) says "and switch to it". `workflow
pr` pushes the branch first when it is unpushed (`loop.EnsurePushed`,
`internal/cli/pr.go:122`) — the dry-run line says so (`pushClause`, `:193`)
but the live question is only "Open the pull request?" (`:114`) — and a
single `--yes` also authorizes
the Jira status transition that follows (`offerReviewStatus`, `:172`).

**Instead.** The help and the preview say "create NAME from BASE and switch
to it"; the question reads "Push NAME and open the pull request?" when a
push is coming; the transition asks in its own words, and `--yes` says in
its help that it covers both.

**Done when.** The preview text names the switch and the push; a test pins
each.

### UX-59 `pr` never links the pull request on the issue

Impact: medium · Effort: small

**Today.** The terminal interface, after opening, asks to link the pull
request on the issue and then offers the review status
(`internal/tui/prcomposer.go:509`, `issuelink.go:20`). `workflow pr` offers only the
status (`internal/cli/pr.go:166`); it never calls `Jira.LinkPullRequest`, though the seam
is on the same `tui.Deps` it already holds. (The web does neither — UX-75.)

**Instead.** `pr` offers the link before the status, under the same
`--yes`.

**Done when.** `workflow pr --yes` calls the fake `LinkPullRequest` with
the pull request's URL.

### UX-60 `announce` will announce the same pull request again

Impact: medium · Effort: small

**Today.** The interface seeds `posted` from the store at start
(`internal/tui/messaging.go:204`) and records each post per moment (`:196`), so a restart
never re-offers an announcement that already went out. `workflow announce`
consults no store at all — no `Store`, `RecordAnnounce` or `Announced`
reference exists anywhere in `internal/cli` — so a second run posts a second
time.

**Instead.** After posting, record it; before the preview, say "already
announced at this moment in an earlier session" and ask whether to post
again.

**Done when.** A second `workflow announce` says it already posted; the
store fake records one entry.

### UX-61 A slow command is silent while it works

Impact: low · Effort: medium

**Today.** No spinner, no elapsed time, no "checking…". `doctor --online`
makes three round trips in silence (`reportCredentials`, `doctor.go:141`);
`standup` fires up to fifteen forge requests plus a Jira search
(`gatherPulls`, `internal/cli/standup.go:178`); `status DIR…` visits each directory in
series (`statusesOf`, `internal/cli/status.go:163`). The only trace is `--log`, which
the subcommands cannot use (UX-52).

**Instead.** A one-line "checking Jira…" on stderr when stderr is a
terminal, replaced in place; nothing when it is not.

**Done when.** A test with a terminal-flagged stderr sees the line; one
without does not.

### UX-62 Flags the scriptable commands are missing

Impact: low · Effort: medium

**Today.** No command declares a single shorthand — there is no `VarP(`
call in `internal/cli` — so `-n`, `-y`, `-j` do not exist; `status` emits
`●◐✗○` (`statusGlyph`, `internal/cli/status.go:390`) with ASCII selectable only through
`ui.ascii` in the file, no `--plain`; `standup` has no `--json`; `pr` has no
draft, base, reviewer, title or body flag; `announce` has no `--channel`
(the channel comes from `messaging.channel` alone); both `pr` and the web
take the first repository template only (`firstTemplate`, `internal/loop/pull.go:136`),
where the interface cycles them (`ctrl+t`).

**Instead.** Shorthands for the three common flags; `--plain` on `status`;
`--json` on `standup`; `--draft`, `--base`, `--reviewer`, `--channel`,
`--template` where the seam already carries the value.

**Done when.** Each flag has a test that it reaches the seam.

## The terminal interface

### UX-63 Five keys the Issues pane answers never appear in its footer

Impact: medium · Effort: small

**Today.** `footerKeys` (`render.go:336`) shows each pane's verbs. The
Issues pane answers `a` assign, `w` log work, `/` filter, and `enter`/`esc`
in the collapsed layout (`issuekeys.go:22`, `:46`, `:48`, `:81`, `:85`), but
none of the five reaches the footer. Two keys are shown where they do not
work: `ctrl+w` (worktree) is filed under "Branch and Commits" (`keys.go:225`)
and listed under pane 2 in `usage.md:45`, but only the branch creator
answers it (`internal/tui/branch.go:389`); `w` post-when-green is filed under "Review and
Slack" (`keys.go:245`) and listed under pane 5 in `usage.md:57`, but only the
preview answers it (`internal/tui/messaging.go:355`).

**Instead.** The Issues pane's `keys(m)` includes the five when their seams
are wired; the two overlay-only keys move to the overlay groups; the docs
follow.

**Done when.** A structural test asserts every key a pane answers is in its
footer when it can act, replacing the three-string spot check
(`focus_test.go:124`).

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint (`tui.go:139`);
`ui.ascii` swaps glyphs and borders (`glyphs.go:43`); escapes in server text
are neutralized. But there is no screen-reader mode; the alternate screen
is unconditional (`render.go:41` `view.AltScreen = true`), so nothing the
interface prints survives quitting; `ui.color` has no `always` for a piped
terminal that does support color; and the 150 ms detail delay
(`internal/tui/detail.go:21`) is fixed.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`; `ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-65 Two outward writes go with one key and no last look

Impact: high · Effort: small

**Today.** The interface promises "nothing outward facing is sent without a
last look" (`comment.go:60`), and 16 of 18 outward acts get a preview or a
confirmation. `R` re-runs failed CI — a forge write — straight from the key
(`internal/tui/review.go:432` → `rerunChecks` `:479`); `u` rebases onto the base —
rewriting local history — straight to `git rebase` (`internal/tui/branch.go:235` →
`startRebase` `internal/tui/run.go:447`). The push already has the overlay this needs
(`pushPreview`, `internal/tui/run.go:394`, with a comment saying exactly why it exists).

**Instead.** `pushPreview` becomes a generic last-look overlay (three users
satisfies the rule of three) and both keys go through it.

**Done when.** Pressing `R` on a failed pull request opens an overlay
naming it and calls nothing; `enter` calls `Rerun`; `esc` never does. The
same for `u`.

### UX-66 Merge and finish close on a refusal and shrink it to one line

Impact: medium · Effort: small

**Today.** "A refused change must never go unseen" (`internal/tui/picker.go:269`) holds
in 13 of 13 overlays while a request is in flight, and 11 keep the refusal
where it happened. `mergeRequested.apply` (`internal/tui/review.go:593`) and
`finished.apply` (`finish.go:139`) instead close the overlay and pass the
reason to a one-line notice, which the next keypress clears — the two
overlays that still keep their own `merging`/`finishing` booleans instead
of `sendState` (DEBT-56).

**Instead.** Both adopt `sendState` and `pinnedOutcome`, and the refusal
stays in the overlay with its reason until `esc`.

**Done when.** A refused merge leaves the merge overlay open showing the
write-scope reason; the same for a failed finish.

### UX-67 One voice for failure — 11 places of 52 speak it

Impact: high · Effort: medium

**Today.** `failure` (`render.go:421`) is "the one way the interface says
something broke", and `errorSentence` (`render.go:401`) is where the useful
sentences live ("Check the VPN, then press `r`", "Run `gh auth login`"). Of
52 places that render an error, only 11 reach `errorSentence`.
`failureBlock` (`render.go:450`) red-wraps the raw error chain and never
calls it, so all 8 `pinnedOutcome` overlays show `could not reach the forge
at …: dial tcp …` instead of the sentence. Fourteen rail sites are
`failedGlyph + err.Error()` (`checks.go:96`, `diff.go:79`,
`issuewrite.go:120`, `internal/tui/picker.go:209`, `internal/tui/switchtask.go:108`, `internal/tui/review.go:219`,
`internal/tui/run.go:211`, `internal/tui/composer.go:164`, `internal/tui/fields.go:171`, …); eight are glyph-only
summaries; nine are notices with no glyph and no red (`comment.go:76`,
`internal/tui/composer.go:77`, `internal/tui/messaging.go:378`, `finish.go:141`, `internal/tui/review.go:509`,
`:511`, `:575`, `:577`, `:595`); one config screen is unstyled
(`render.go:386`). `render.go:372` points at `workflow doctor` only for a
*missing* setting, never a failing one; `forgeReason` (`internal/tui/review.go:361`),
`rerunReason` (`:524`) and `mergeReason` (`:605`) each carry a sentence
`errorSentence` does not know.

**Instead.** `failureBlock` consults `errorSentence`; a one-line
`failureLine(err)` replaces the fourteen raw sites; the nine notices go
through it; the three `*Reason` helpers fold into `errorSentence`'s table.

**Done when.** `err.Error()` appears in no non-test file under
`internal/tui` except `render.go`; a table test asserts, for every seam
sentinel, that each channel (pane, pinned overlay, notice, rail) shows the
sentence.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (commit `internal/tui/composer.go:252`, pull request
`internal/tui/prcomposer.go:275`), a dirty tree blocks a switch instead of stashing, and
quit is guarded while a post waits. But a posted comment, an applied
transition, a merge and the `branch -D` in finish have no undo, and the
interface never says which acts are reversible.

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries its
URL so it can be edited where it lives.

**Done when.** Each irreversible act's preview or notice says so.

### UX-69 Vim habits stop at `j`/`k`

Impact: low · Effort: small

**Today.** `up/k`, `down/j`, `pgup/K`, `pgdn/J` (`keys.go:200`). No `h`/`l`
(`←`/`→` are cycle-type and cycle-channel only), no `g`/`G` to jump to the
ends of a list or the detail.

**Instead.** `g`/`G` on the lists and the detail; `h`/`l` where a pane has a
horizontal axis.

**Done when.** `G` on the Issues list selects the last loaded issue.

## The web

### UX-70 The browser cannot read an issue

Impact: high · Effort: small

**Today.** `getIssue` — the full issue with description, comments,
comment count, reporter, assignee and URL — exists in the contract
(`api/openapi.yaml:95`, `IssueDetail` at `:771`) and on the server
(`internal/webserver/handlers.go:66`), and the web never calls it.
`IssueDetail` (`web/src/features/issues/IssuesPanel.tsx:83`) renders only
the snapshot's slim `Issue` (`openapi.yaml:209`: key, summary, status,
category, type, priority). No description, no comments, no link to Jira.

**Instead.** Fetch `getIssue` on selection through the query client that is
already wired (`web/src/queryClient.ts`); render the detail with its own
loading and error states; an "Open in Jira" link from `url`.

**Done when.** Selecting an issue shows its description and its comments
(a role/name test against a faked `getIssue`).

### UX-71 Under `--dry-run`, every button fails and nothing says why

Impact: high · Effort: small

**Today.** `refuseWritesInDryRun` (`internal/webserver/guard.go:46`) answers
403 to every unsafe method — the documented design. `getHealth` carries
`dry_run` (and `version`) on the wire (`openapi.yaml:42`) and the web never
fetches it, so the browser has no idea it is in that mode: each write
button sends its request and shows a refusal with no explanation, and the
version is shown nowhere.

**Instead.** One `getHealth` at mount: the version in the header; when
`dry_run`, a `role="status"` banner ("Read-only: started with `--dry-run`;
every write is held back") and each write, when clicked, says so *without*
sending — the interface's narration (`internal/tui/dryrun.go:25`) rather
than a disabled control (CLAUDE.md forbids the opacity route anyway).

**Done when.** Against a fake `getHealth` reporting `dry_run: true`, the
banner is visible and clicking Push sends no request.

### UX-72 One view, one page, no filter

Impact: medium · Effort: small

**Today.** `listViews` and the `view` query on the stream exist
(`openapi.yaml:56`, `:488`; `stream.go:38`) and the web opens
`EventSource('/api/events')` with no `?view=` (`web/src/api/snapshot.ts:39`);
the stream always pushes page 0 (`stream.go:126`); there is no client-side
filter. `No issues match this view.` (`IssuesPanel.tsx:23`) is a dead end
with no view to change. On the server, `resolveJQL` (`handlers.go:211`)
falls through to `views[0]` for an unknown name, so a typo returns the
wrong view silently.

**Instead.** A view select from `listViews` beside the list; the stream
reconnects with `?view=`; a "load more" using `start_at`; a filter box like
the interface's `/`; an unknown view answers `not_found`.

**Done when.** Choosing a view changes the stream's query; `GET
/api/issues?view=nope` is 404.

### UX-73 On GitLab the web still says "pull request"; the nav says one thing and the button another

Impact: medium · Effort: small

**Today.** The interface, the CLI and the web *server* all resolve "merge
request" on GitLab through `forge.Kind.Noun` (`internal/forge/remote.go:60`)
— the server for the announcement text only. The web *UI* hardcodes "pull request" in six strings (`ReviewPanel.tsx:141`,
`:172`, `:181`, `:326`; `WorkStory.tsx:40`; `MessagingPanel.tsx:50`);
`ForgeKind` is on the server (`webserver.go:60`) and never sent to the
browser. The nav section is `Messaging` (`sections.ts:10`) while its buttons
say `Announce to Slack` / `Post to Slack` (`MessagingPanel.tsx:170`, `:239`);
"start" has three names across the surfaces (`branch for issue`, `new
branch`, `Start work on this issue`); the web has five sections where the
interface has six (Commits folded into Branch, Reviews absent — UX-84,
Settings added).

**Instead.** `getHealth` sends the forge's noun (send the words; do not port
`Kind.Noun` to TypeScript); the section reads the service's name from the
snapshot; one verb for starting.

**Done when.** With a GitLab forge faked, the Review section reads "merge
request" throughout.

### UX-74 After opening a pull request, the web stops

Impact: medium · Effort: medium

**Today.** The interface links the pull request on the issue and then
offers the configured review status (`internal/tui/prcomposer.go:509`, `internal/tui/picker.go:186`);
the CLI offers the status (`internal/cli/pr.go:166`). `internal/webserver/pullrequest.go`
touches neither `Jira.ReviewStatus` nor `LinkPullRequest`; after `Pull
request opened.` (`ReviewPanel.tsx:141`) there is nothing more to do.

**Instead.** Two operations, spec first — `POST /api/issues/{key}/link` and
`POST /api/issues/{key}/transition` (fields-less; 409 when Jira wants
fields) — and, after opening, two inline offers: "Link it on KEY" and "Move
KEY to STATUS", each with a `role="status"` outcome. This is where the shared
composition layer, `internal/loop`, pays for itself: the web's offer takes
`loop.ReviewTransition` (`internal/loop/pull.go:154`), the CLI's rule,
rather than a third copy.

**Done when.** After a faked open, the panel offers the move and the link;
a form transition answers 409; `--dry-run` refuses both.

### UX-75 The web cannot stage, so its commit form is unreachable from a clean start

Impact: high · Effort: medium

**Today.** The working-tree list (`BranchPanel.tsx:86` `WorkingTree`) is
read-only, and the commit form is mounted only when something is *already*
staged (`:111`). A browser user with unstaged changes has a list they cannot
act on and no form — the flow the interface completes with `space` and `a`
(`internal/tui/commits.go:267`, `:288`) is not there.

**Instead.** `POST /api/stage` and `/api/unstage` taking `{path}` or
`{all: true}`; Stage / Unstage per file and Stage all; the commit form
always present, saying "Nothing staged yet — stage a file above" until it
can commit.

**Done when.** Stage all in the browser makes the commit form live.

### UX-76 `commit.default_scope` can be edited in Settings and is never used

Impact: medium · Effort: small

**Today.** The interface's composer opens on the learned scope, else
`commit.default_scope` (`internal/tui/composer.go:130` `startingScope`). The web's
`CommitForm` hardcodes `scope: ''` (`CommitForm.tsx:45`) — while
`commit.default_scope` is an editable field in the Settings form
(`SettingsPanel.tsx:182`). A setting the user can change with no visible
effect.

**Instead.** The snapshot's `changes` carries `suggested_scope` — the store's
last scope, else the configured default (the interface's own rule) — and
the form opens on it; the server records the scope after a commit, as the
interface does.

**Done when.** Editing `default_scope` in Settings pre-fills the next commit
form; a table test covers the store-then-config fallback.

### UX-77 Four of seven writes succeed in silence

Impact: high · Effort: small

**Today.** `Pull request opened.` (`ReviewPanel.tsx:141`), `Announced…`
(`MessagingPanel.tsx:137`) and `Saved.` (`SettingsPanel.tsx:321`) confirm.
Commit (`CommitForm.tsx:70`), push (`BranchPanel.tsx:130`), check-out and
start-work (`useAsyncAction.ts:19`) reset and say nothing; the stated
rationale is that the snapshot is the confirmation (`useAsyncAction.ts:8`),
but the stream re-pushes on a 5 s tick (`stream.go:20`), so a commit is
silent for up to five seconds. And the two successes that do exist are
plain `<p>` elements, not live regions — announced to nobody using a screen
reader.

**Instead.** Every write ends in a `role="status"` line that keeps the
button's verb ("Pushed NAME", "Committed abc123 subject"); the one
`useAsyncAction` gains a `done` state and the five hand-rolled copies adopt
it (DEBT-63).

**Done when.** A table test over the seven writes finds a status region for
each.

### UX-78 Focus is dropped after every action

Impact: high · Effort: small

**Today.** There is no `.focus()` call anywhere under `web/src`. On success
`OpenPullRequest` unmounts the form and renders a `<p>` in its place
(`ReviewPanel.tsx:138`); `AnnounceControls` does the same
(`MessagingPanel.tsx:135`); `PushButton` swaps its idle and confirming
states (`BranchPanel.tsx:139`). The focused button disappears and focus
falls to `<body>`. Changing section from the nav rail or a work-story row
never moves focus to `<main>`, though `tabIndex={-1}` is there for it
(`AppShell.tsx:39`).

**Instead.** Focus the outcome when a form closes; focus `<main>` on a
section change.

**Done when.** After a faked open succeeds, `document.activeElement` is the
outcome (`toHaveFocus`).

### UX-79 The fallback says what could not happen; the server sometimes says nothing

Impact: medium · Effort: small

**Today.** Server reasons are good where they exist (`the working tree has
uncommitted changes; commit or stash them before switching`, `internal/webserver/checkout.go:19`;
`nothing is staged to commit`, `internal/webserver/commit.go:21`). But the client fallbacks
are vague and near-apologetic — `The branch could not be checked out.`
(`IssuesPanel.tsx:134`), `Work could not be started.` (`WorkStory.tsx:260`),
`The push failed.` (`BranchPanel.tsx:132`), `The commit could not be
created.` (`CommitForm.tsx:74`) — and three handlers replace the tool's
reason with one of those sentences: `internal/webserver/checkout.go:44`, `internal/webserver/branchcreate.go:44`,
`internal/webserver/announce.go:57`. `writeResponseError` answers `something went wrong`
(`errors.go:79`). A user gets a dead end with no next step.

**Instead.** `checkout` and `branchcreate` pass git's own reason through
the classified problem mapping; `announce` classifies by the messaging
sentinels and never forwards the text (a messaging error can name the
webhook URL); every fallback names a next step.

**Done when.** A checkout refused by git shows git's reason in the alert; a
test proves the announce error never carries the webhook.

### UX-80 Dead-end empty states, and four ways to say "connecting"

Impact: low · Effort: small

**Today.** Good: `Select an issue to see its detail.`, `Open a pull request
first — there is nothing to announce yet.`, `{service} is not configured.
Add a token or webhook in Settings.` Dead ends: `This directory is not a
Git repository.` (`BranchPanel.tsx:22`) and `The configuration could not be
loaded.` (`SettingsPanel.tsx:16`) offer nothing to do. And the one condition
"no snapshot yet" reads `Connecting to the tracker…`, `Connecting to the
workspace…`, `Connecting to the forge…` and `Connecting…` in four panels
(`IssuesPanel.tsx:17`, `BranchPanel.tsx:14`, `ReviewPanel.tsx:31`,
`MessagingPanel.tsx:11`).

**Instead.** One connecting line, in the shell; a retry on the config
error; the not-a-repository state says what a repository would give it.

**Done when.** One string for connecting; the settings error has a Retry
button.

### UX-81 Disabled by dimming, and no rule for reduced motion

Impact: medium · Effort: small

**Today.** Every disabled button uses `disabled:opacity-60` — thirteen
places (`IssuesPanel.tsx:146`, `WorkStory.tsx:241`, `:271`,
`BranchPanel.tsx:168`, `CommitForm.tsx:128`, `ReviewPanel.tsx:179`, `:317`,
`:324`, `MessagingPanel.tsx:168`, `:214`, `:229`, `:237`,
`SettingsPanel.tsx:316`) — which CLAUDE.md's accessibility rule names as the
thing not to do (opacity dims text below the contrast floor) and which axe
does not catch on disabled controls. The app has only two `transition-colors`
and no animation, but no `prefers-reduced-motion` rule either; the a11y spec
forces `reducedMotion: 'reduce'` to stabilize its scan, not because the app
honors it.

**Instead.** A `disabled:` color treatment on the token scale; `motion-safe:`
on the two transitions and one reduced-motion rule in `index.css`.

**Done when.** `grep -c opacity-60 web/src` is 0; axe passes both themes.

### UX-82 A bad stream frame disappears without a trace

Impact: medium · Effort: small

**Today.** A frame that fails `zSnapshot.safeParse` is dropped
(`snapshot.ts:52`); the last good snapshot stays on screen and
`StreamStatus` still says `Live`. A field added on the server without
regenerating the client makes every frame vanish, indistinguishably from a
quiet repository (DEBT-67).

**Instead.** A dropped frame sets the stream status to `stale` with the
reason, shown in the same pill.

**Done when.** Feeding the store a schema-mismatched frame shows `stale` in
`StreamStatus`.

### UX-83 The web has no Reviews section

Impact: medium · Effort: medium

**Today.** The review queue — pull requests waiting on you — is a pane in
the interface (`internal/tui/reviewqueue.go`) and a command (`workflow
reviews --json`, `internal/cli/reviews.go`), both over `Forge.ReviewRequests`.
The web's `sections.ts:6` has no such section, and the contract has no
operation for it.

**Instead.** `GET /api/reviews` over the same seam, polled by the query
client (it is a cross-repository forge search, not a snapshot field), and a
sixth section listing number, title, repository, requester, CI and age with
open/copy links.

**Done when.** The web lists the same requests the CLI prints, by role and
name.

### UX-84 The web's visual system is not the product's

Impact: medium · Effort: large

**Today.** The interface's system — five hues for five systems (Jira blue,
git yellow, the forge green, chat magenta; red for failure and nothing
else) and state carried by the *shape* of `○ ◐ ● ✗` (`internal/tui/glyphs.go:16`,
`:96-127`; the spine at `spine.go:68`) — is stated, and the previous edition
says none of it should change. The web carries none of it across: one
periwinkle accent `#8b93f8` chosen "clear of the green/amber/red the status
lights own" (`web/src/index.css:24`) plus a three-color CI language; Jira is
not blue, git is not yellow, the forge is not green anywhere; the `NavRail`
icons are all muted (`NavRail.tsx:28`). State marks are the web's own:
`StageMarker` (`WorkStory.tsx:284`) invents three, and `ciDot`
(`ReviewPanel.tsx:14`) and `StreamStatus` (`StreamStatus.tsx:4`) are
color-only dots of one shape (mitigated by a text label beside each). And
the page shows the template tells the interface avoids: seven `uppercase`
eyebrow headings (`SettingsPanel.tsx:341`, `BranchPanel.tsx:67`, `:91`,
`IssuesPanel.tsx:102`, `ReviewPanel.tsx:70`, `MessagingPanel.tsx:39`, `:59`)
as the *only* heading treatment; no type or spacing tokens (raw `text-2xl`
… `text-xs`, `gap-8` … `gap-0.5` per component); one radius on everything
(`rounded-md` ×28). To its credit: no shadows, no gradients, no `→`, and a
real color-token system with hand-picked contrast (`index.css:9-96`).

**Instead.** The four system hues as tokens in both themes, at AA
contrast, carrying identity (the active nav icon, section headings); one
`StateMark` component that draws `○ ◐ ● ✗` for CI, the stream and the work
story (label kept, mark `aria-hidden`); sentence-case headings on a
`--text-*`/`--space-*` scale; the radius scale actually used. The periwinkle
accent can stay for interactive controls — it was chosen on purpose — or be
replaced; that is the maintainer's call. The interface's own system does
not change.

**Done when.** No `uppercase` heading remains; every state has a distinct
shape; both themes pass axe.

### UX-85 The layout has no breakpoints

Impact: medium · Effort: medium

**Today.** There is not one `sm:`, `md:`, `lg:` or `xl:` utility in any
`.tsx` under `web/src`. A `w-20` rail (`NavRail.tsx:12`) beside a `w-80
shrink-0` issues list (`IssuesPanel.tsx:31`) and a `flex-1` detail squeezes
the detail to nothing near 640 px; definition lists use fixed first columns
(`grid-cols-[6rem_1fr]` `BranchPanel.tsx:49`, `[8rem_1fr]`
`MessagingPanel.tsx:29`, `[9rem_1fr]` `ReviewPanel.tsx:56`); panels cap at
`max-w-2xl` and never reflow; the issues `<ul>` is not scrollable, so a long
list scrolls the page. The viewport meta tag is present (`index.html:5`) and
nothing responds to it. (From code; no viewport was rendered.)

**Instead.** The rail collapses to icons under `md`; list and detail stack
under `lg`; the grids and the caps go fluid; the list scrolls inside its
panel.

**Done when.** A Playwright spec at 640, 1024 and 1440 px finds no horizontal
scroll and every control reachable.

### UX-86 Nothing marks a change the stream just made

Impact: low · Effort: medium

**Today.** `StreamStatus` shows `Connecting` / `Live` / `Reconnecting`, but
nothing says when the last snapshot arrived, and a panel that changed
because CI settled looks exactly like one that re-rendered. There is no
toast, and no "CI passed" moment on the web where the interface rings the
terminal (`internal/tui/review.go:142`).

**Instead.** A "updated 3 s ago" beside the pill; a brief highlight on the
row a snapshot changed; a status line when CI settles, honoring
`ui.notify`.

**Done when.** A snapshot that flips CI to passed produces a status
region saying so.

### UX-87 Settings can edit seven sections and carry five it cannot show

Impact: low · Effort: medium

**Today.** The form seeds itself with the whole `Config`
(`SettingsPanel.tsx:25`) so `ui`, `timing`, `headers`, `views` and
`branch.prefixes` survive a save unchanged — and cannot be edited. There is
no guided, credential-checking flow like `workflow config init`; the web
edits an existing file only.

**Instead.** Fieldsets for the five, with `views` and `prefixes` as
editable lists; a first-run flow that checks the Jira token as `config
init` does.

**Done when.** A view added in the browser appears in the interface's `v`
cycle.

### UX-88 Ideas the interface has that the browser could borrow

Impact: low · Effort: medium

**Today.** The interface shows a per-file diff under the changes list
(`diff.go:29`), amends (`A`) and fixups (`f`), lists CI checks and jumps to
a failure in `$EDITOR` (`checks.go:37`, `internal/tui/run.go:366`), edits an open pull
request (`preditor.go`), and cycles the repository's pull-request templates
(`ctrl+t`). None has a web equivalent, and the web takes the first template
only (`firstTemplate`, `internal/loop/pull.go:136`). A `?` shortcut sheet, which the interface has,
would give the web's five sections keyboard reach.

**Instead.** In rough order of value: a template select on the pull-request
form; a checks list with links; a diff view; `?`.

**Done when.** Each lands with a role/name test; the template select shows
every repository template.

## The visual system

What is there is a real system, and a good one for a terminal: five hues
for five systems (Jira blue, git yellow, the forge green, chat magenta),
all taken from the terminal's own palette so the user's theme decides the
shades; shape for state (`○ ◐ ● ✗`); border weight for focus; red for
failure and nothing else. None of that should change. This edition found
no place in the terminal where the system is not applied; the one open
question is the web's, and it is UX-84.

## Across the surfaces

### UX-89 Worktrees and a fresh base on the command line and the web

Impact: low · Effort: medium

**Today.** The interface's branch creator fetches `origin` first and offers
to branch from what you have when the fetch fails (`internal/tui/branch.go:449`), and
`ctrl+w` creates the branch in a worktree beside the repository
(`internal/tui/branch.go:389`). `workflow branch` and `POST /api/branches` do neither:
no fetch, no worktree.

**Instead.** `--worktree` and `--fetch` on `branch`; a worktree toggle on
the web's start-work flow; both through the shared composition layer,
`internal/loop`.

**Done when.** `workflow branch KEY --worktree` creates a directory beside
the repository and says where.

## Ideas that would reopen a settled decision

None this edition. One note for the record: `FEATURES.md:52` states the
settled decision as "Five panes down the left"; the code has six
(`internal/tui/panes.go:27`), and has since the Reviews pane landed. That is
stale text to correct (TECH_DEBT.md DEBT-69), not a decision to reopen —
the six-pane rail *is* the decision as built.
