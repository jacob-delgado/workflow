# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up UX-61". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `990f333` on 2026-10-05: main once #179 merged,
after the comment composer, the Tasks sort and filter, the Summary, the
Repositories and the worktrees had all landed. Every pointer was read again
at that commit. Line numbers drift, so every pointer also names the symbol
it means.

## How this was produced

The first edition was one pass over every surface and every package, from
the source and, for the web, from the screen, at `f05ae9f`. This edition
re-read it after nine milestones changed the screens it describes.

1. **Three surface audits.** Every command and flag in `internal/cli`, every
   pane, key and overlay in `internal/tui`, and every section, control and
   operation in `web/src`, `internal/webserver` and `api/openapi.yaml` were
   inventoried, held against [clig.dev](https://clig.dev), the principles
   below and the promises the terminal makes, and every open entry was
   marked open, partly fixed or shipped from the code.
2. **The screens were looked at.** The web's populated mock ran under
   Playwright (`web/e2e/screens.spec.ts`), every section in both themes at
   640, 1024 and 1440 px; the terminal ran with `--dry-run` in tmux at
   80x24, where its narrowest layout shows. A claim about how a screen
   looks names what it was read from.
3. **Every finding refuted before it was written.** Each count and pointer
   was read again as its entry was written, an independent reader tried to
   refute each new entry, and a claim that could not be pointed at a line
   was dropped.

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

## Principles

The rules every entry is judged by. Each is stated once here; an entry that
breaks one names it.

1. **One name per action, kept through its flow.** The control, its busy
   label and its done notice share a verb: Switch, Switching…, Switched to
   `~/src/api`. Each surface keeps one notice style: the terminal's notices
   are lowercase fragments with no period, the web's are sentences, and the
   command line's are sentences on stderr.
2. **Confirm what leaves the machine or cannot be undone.** A push, a post,
   a merge, a Taskwarrior sync, marking a task done, undoing in
   Taskwarrior, forgetting a person and switching the directory each wait
   on a last look. A reversible local toggle acts at once: stage and
   unstage, start and stop a task, add and remove a favorite, tag someone
   in an announcement, link an owner to a person.
3. **One key per verb, on every pane.** The same verb has the same key
   wherever it appears (`/` search, `f` filter, `O` sort, `r` refresh, `o`
   open, `y` copy), and no binding takes a key a text field uses to edit
   while that field has the focus.
4. **A failure is never drawn as empty or as rest.** The terminal says it in
   the failure voice (`errorSentence`, `internal/tui/failure.go:86`); the
   web says it in a `role="alert"` line beside what failed.
5. **Shape carries state, not color.** A glyph's shape, a word or a weight
   tells the state; a hue only repeats it.
6. **The terminal and the web are equals.** Each can do what the other
   can, in its own idiom. The command line does what is useful once or in a
   script, with `--json`, and leaves browsing to the other two.
7. **The forge's own nouns.** A merge request on GitLab, `!` before its
   number; a pull request and `#` on GitHub; never one assumed for both.

## Vocabulary

One name per concept, chosen so the three surfaces read as one tool. The
"Today" column is what each says now; UX-147 is the work of making them
agree.

| Concept | Name | Today |
| --- | --- | --- |
| Making a branch for an issue | **Start work** (the command stays `branch`; its prompt and help say "start work") | CLI "Branch NAME from BASE and switch to it"; terminal "branch for KEY", "New branch for KEY"; web "Start work" |
| The same, in a new worktree | **Start work in a new worktree** | terminal "as a worktree", "create worktree"; web "Start in a new worktree" |
| Checking out a git branch | **Switch branch** | terminal "switch task"; web "Check out", "Switching…" |
| Changing the directory workflow works in | **Switch** … **Switched to** | terminal and web agree |
| The review queue | **Reviews**, headed "Waiting on your review" | CLI `reviews`; terminal pane "Reviews"; web nav "Reviews", heading "Review requests" |
| The ready, merged or CI-red message | **Announce** … **Announced to** | agrees |
| A standup | **Post** … **Posted to** | agrees |
| Narrowing by typing | **Search** | terminal `/` "filter"; web "Filter" field |
| Narrowing by a checklist | **Filter** | "Where" on Issues, "Narrow" on Tasks, "Filter" on Reviews, on both |
| Ordering | **Sort** | terminal `s` on Reviews, `O` on Tasks; web "Sort" |
| Finishing a Taskwarrior task | **Mark done** … **Marked done** | terminal "done" then "completed"; web "Done", "Completing…", "Completed task N." |
| The local stores | **Local data**, **Remove** … **Removed** | CLI `db-clean` "Remove the local databases"; web "Clean cache…", "Remove X?", "Clean", "Removed X." |
| Reading again after a failure | **Try again** | web "Retry", "Read the code owners again", "Read the groups again", "Read the local data again"; terminal "press r to try again" |
| Reading | **Reading …** | terminal "loading…", "looking…", "checking…", "reading…"; web "Reading …", "Loading the configuration…" |
| `esc` | **discard** when it drops work, **close** when it keeps it, **cancel** for an unsent form, **back** for a step inside a flow, **stay** at a guard, **skip** for an offer after a done act | agrees in the terminal (`esc` constants beside `everywhereKeys`, `internal/tui/keys.go`) |

## What each surface can do

One row per capability, in the order the work goes. A cell names the
command, key or control; "—" names the entry or feature that would fill
it, where one exists. "Not for scripts" marks a deliberate gap: the
command line leaves browsing, and what git or Taskwarrior already do on
their own command line, to them.

| Capability | Command line | Terminal | Web |
| --- | --- | --- | --- |
| **Issue** | | | |
| List, search and filter issues | — (FEAT-78) | Issues pane: `/`, `f`, `v`, `ctrl+n` | Issues: View, Filter, Where, Load more |
| Read an issue and its comments | — (FEAT-78) | the detail | the detail |
| Comment | — (FEAT-78) | `c`, a composer with vim modes | the comment composer, Markdown |
| Change its status | moves to `jira.review_status` inside `pr` | `t` | to the review status, after opening a pull request (UX-149, FEAT-80) |
| Assign; log work | — (FEAT-78) | `a`; `w` | — (UX-149, FEAT-80) |
| Track it in Taskwarrior | not for scripts | `T` | Track in Taskwarrior |
| **Branch** | | | |
| Start work | `branch KEY` | `b` | Start work |
| Start work in a new worktree | — (UX-89) | `b`, then `ctrl+g` | Start in a new worktree |
| Switch branch | not for scripts | `s` | Check out |
| Link an issue to the branch | — | `i` | Link an issue |
| Unlink it | — | — (UX-150) | Unlink |
| Push | inside `pr` | `P` | Push branch |
| Rebase onto the base | — | `u` | — (UX-149) |
| **Commits** | | | |
| Stage, unstage, stage all | not for scripts | `space`, `a` | Stage, Unstage, Stage all |
| Read a file's diff | not for scripts | the selected file's diff | — (UX-149) |
| Commit | not for scripts | `c` | Commit staged changes |
| Amend; fix up | not for scripts | `A`; `f` | — (UX-149) |
| Run pre-commit; set up lefthook | — | `h`; `g` | — (UX-149) |
| **Review** | | | |
| Open a pull request | `pr` | `n` | Open a pull request |
| Link it on the issue, move the issue | `pr` asks both | offered after `n` | Link it on KEY, Move KEY to STATUS |
| Edit it | — | `e` | — (UX-149, FEAT-79) |
| Read the checks and a job's log | `status` shows CI | `c`, then `l` | a failed check's log only (UX-149) |
| Re-run CI; merge; finish | — (FEAT-83 for finish) | `R`; `M`; `F` | — (UX-149, FEAT-79) |
| Where the work stands | `status [DIR…] --json` | the top row | the work story, the header |
| **Messaging** | | | |
| Announce | `announce` | `p` | Announce to SERVICE |
| Announce when CI passes | — | `w` in the preview | — (FEAT-82) |
| Edit the announcement first | — | `e` in the preview | — (UX-149) |
| People and groups | — | `P` | Settings: People, Groups |
| Post a standup | `standup` | — (FEAT-86) | — (FEAT-86) |
| **Reviews** | | | |
| List what waits on your review | `reviews --json --sort` | Reviews pane: `O`, `f` | Reviews: Sort, Filter |
| **Tasks** | | | |
| Add, annotate, modify, start, stop, mark done, undo, sync | not for scripts | Tasks pane: `a`, `A`, `e`, `s`, `d`, `u`, `S` | Tasks: Add, Annotate, Modify, Start, Stop, Done, Undo, Sync |
| Sort, search, filter | not for scripts | `O`, `/`, `f` | Sort, Filter, Narrow |
| **Summary** | | | |
| Read what you did in a period | — (FEAT-86) | Summary pane: `[`, `]`, `t`, `c` | Summary: Earlier, Later, Today, the calendar |
| Copy it as Markdown | — (FEAT-86) | `Y` | Copy as Markdown |
| **Repositories** | | | |
| Where workflow works, and what configures it | `doctor`, `config show` | Repositories pane, the top row's place | Repositories, the header's place |
| Switch the directory | not for scripts | `enter`, `g` | Switch, Switch here |
| Favorites | — (UX-151) | `f` | Add to favorites, Remove |
| List the worktrees | — (UX-151) | the Repositories pane | Worktrees |
| **Settings and local data** | | | |
| Set up from nothing | `config init`, `slack login` | — (UX-153) | — (UX-153) |
| Read the configuration | `config show` | not in the terminal (UX-150) | Settings |
| Change it | edit the file | — (UX-150) | Settings |
| Check the setup | `doctor [--online]` | — | — |
| Remove the local data | `db-clean` | — (UX-150) | Settings: Local data |
| Every key or command | `--help` | `?` | — (UX-152) |
| Hold back every write | `--dry-run` | `--dry-run` | `--web --dry-run` |

## The promises the interface makes

The terminal states its own rules, in its docs and in its code. This table
is the shortest summary of how far the screen keeps them, re-counted at
`990f333`.

| The promise | Where it is made | Kept? |
| --- | --- | --- |
| "`?` lists every key" | `docs/content/docs/usage.md:90` | **Yes, by construction, and a test enumerates every placement.** Help is generated from the bindings (`helpBuilder.place`, `internal/tui/keys.go:153`, rendered in two columns split where they balance): 90 placements in 12 groups, on 89 lines, since `cycle-type-right` rides `cycle-type-left`'s line. `TestHelpListsEveryPlacedBinding` (`internal/tui/help_test.go:283`) reads `?` back and holds it to a table of every placement. |
| "the one way the interface says something broke" | `wording`, `internal/tui/failure.go:58` | **Yes: every site that renders an error's text.** Each is told through `errorSentence` (`internal/tui/failure.go:86`) and drawn by one of seven helpers: `failureBlock` at 18 sites, `pinnedOutcome` 13, `failureLine` 21, `failureSummary` 7, `noticedFailure` 10, `noticedFailureLedBy` 2 and `noticedGuidance` 3 (plain, since red means something broke). |
| "Nothing outward facing is sent without" a last look | `commentPreview`, `internal/tui/comment.go:54` | **Kept for every act that leaves the machine but Taskwarrior's sync; not kept for four that cannot be undone.** Every write to Jira, the forge and the messaging service, and every push, waits on a preview or a confirmation, most through `lastLook` (`internal/tui/overlay.go:217`). Under principle 2 the exceptions are `S` (sync, which leaves the machine), `d` (mark done) and `u` (undo) on the Tasks pane, `d` (forget) in People and groups, and `enter` (switch the directory) on the Repositories pane, which asks only when work would be lost (UX-145). |
| "a refused change must never go unseen" | `statusPicker`, `internal/tui/picker.go:321` | **Yes: 16 of 16.** Every overlay that sends a request refuses every key while it is in flight and keeps a refusal where it happened until `esc`: `branchCreator`, `branchLinker`, `branchPicker`, `commentPreview`, `finishPreview`, `hookgenOffer`, `issueLinker`, `issueWrite`, `lastLook`, `mergePicker`, `messagingPreview`, `peopleOverlay`, `prComposer`, `prEditor`, `statusPicker` (with its field form) and `taskLine`. |
| "Each pane fails on its own" | `docs/content/docs/usage.md:95` | **Yes.** `Init` (`internal/tui/tui.go:193`) batches eight loads, and the Summary and Repositories panes read on first focus (Init reads Repositories too when it starts focused there, `internal/tui/tui.go:199`); each pane holds and renders its own load's error, the Summary per source. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:16` | **Yes.** `unicodeGlyphs` and `asciiGlyphs` differ in shape (`internal/tui/glyphs.go:33`, `:44`); `NO_COLOR` keeps bold and faint. Two residues: the progress spine's five system hues are color-only, mitigated by each system's name or initial, and `◐` means both "partly staged" and "announces when CI passes" (see the visual system). |

To re-count rather than trust these numbers: `grep -n 'failureBlock('
internal/tui/*.go`, and the same for each helper, less its definition and
the wrapper beside it in `failure.go`; `grep -n 'send\.sending\|sending\.sending'
internal/tui/*.go` for the overlays that guard a send; and the
`builder.bind(` and `builder.bindShown(` calls in `internal/tui/keys.go`
for the help.

## The command line

What is open here is a slow command's silence, the flags the scriptable
commands lack and the JSON a script cannot join, time or get from `pr` and
`standup`, what `config init` claims and does, three moments that name no
next step, the body `pr` never shows, the store's fallback `doctor` has no
row for, a reference with no example or exit status, three places the
command line breaks its own exit-status and stream contract, and help and
notices worded two ways.

### UX-61 A slow command is silent while it works

Impact: low · Effort: medium

**Today.** No spinner, no elapsed time, no "checking…". `doctor --online`
makes three round trips in silence (`reportCredentials`,
`internal/cli/doctor_credentials.go:30`); `standup` fires up to fifteen
forge requests (`standupBranchLimit`, `internal/cli/standup.go:47`) plus a
Jira search (`gatherPulls`, `internal/cli/standup.go:190`); `status DIR…`
visits each directory in series (`statusesOf`,
`internal/cli/status.go:159`). The only trace is `--log`, which outlines
each request in a file for a bug report and shows the person waiting
nothing.

**Instead.** A one-line "checking Jira…" on stderr when stderr is a
terminal, replaced in place; nothing when it is not.

**Done when.** A test with a terminal-flagged stderr sees the line; one
without does not.

### UX-62 Flags the scriptable commands are missing

Impact: low · Effort: medium

**Today.** No command declares a single shorthand — there is no `VarP(`
call in `internal/cli` — so `-n`, `-y`, `-j` do not exist; `status` emits
`●◐✗○` (`statusGlyph`, `internal/cli/status.go:427`) with ASCII selectable
only through `ui.ascii` in the file, no `--plain`; `pr` declares only
`--yes` (`newPRCmd`, `internal/cli/pr.go:76`), so no draft, base,
reviewer, title or body flag; `announce` has no `--channel` (the channel
comes from `messaging.channel` alone); both `pr` and the web take the
first repository template only (`firstTemplate`,
`internal/loop/pull.go:171`, called at `:133`), where the interface cycles
them (`ctrl+t`, `internal/tui/keys.go:321`).

**Instead.** Shorthands for the three common flags; `--plain` on `status`;
`--draft`, `--base`, `--reviewer`, `--channel`, `--template` where the seam
already carries the value. `--json` on `standup` and `pr` is UX-92's.

**Done when.** Each flag has a test that it reaches the seam.

### UX-90 Two claims about `config init` that the command does not keep

Impact: low · Effort: small

**Today.** The root help, the README and two doc pages describe a
`config init` that does not exist, and the command corrects them itself
while it runs.

- `internal/cli/cli.go:39` (`longHelp`): "Write a starting file with:
  workflow config init", then "fill in the two credentials" (`:43`). That
  is `--template`'s flow; bare `init` runs the guided one
  (`newConfigInitCmd`, `internal/cli/config_cmd.go:56`, branches on
  `opts.template` at `:83` and otherwise calls `runGuidedInit` at `:87`),
  so a reader who follows the root help is prompted instead.
  `TestHelpExplainsBothTokens` (`internal/cli/cli_test.go:297`) holds the
  help to the token steps and never to the flow. The generated
  `docs/content/docs/reference/workflow.md:25` and `:29` carry the same
  two lines.
- `README.md:48` (under "Status"), `README.md:103` (under "Configure") and
  `docs/content/docs/install.md:106` and `:110` (under "First run"):
  "writes a starting configuration file", "# writes .workflow.json here",
  "Then fill in the two tokens", the same stale flow, while
  `docs/content/docs/configuration.md:38` (the "Configuration" intro) says
  it asks and checks.
- `internal/cli/cli.go:101` (the `SECURITY` paragraph of `longHelp`, and
  so `docs/content/docs/reference/workflow.md:87`): the file "is listed in
  .gitignore". `warnIfNotIgnored` (`internal/cli/config_cmd.go:450`) only
  warns when it is not, and nothing writes a `.gitignore`; the sentence is
  true of this repository's own `.gitignore:35`, not the user's.
  `README.md:218` and `docs/content/docs/configuration.md:987` (both under
  "Keeping the tokens safe") say the same, and `README.md:295` (under
  "Security") calls the file "gitignored".

**Instead.** Say what the command does: in `longHelp`, the README and
install.md, "Set it up, answering the prompts, with `workflow config init`
(`--template` writes a blank file to edit)"; in the security paragraphs,
"`config init` warns when the file is not ignored by git; add it to
`.gitignore`"; then `task docs:gen`.

**Done when.** `TestHelpExplainsBothTokens` also wants `--template` in the
root help; `grep -rn 'starting configuration file\|starting file\|fill in
the two\|listed in .gitignore\|listed in the repository\|gitignored'
README.md docs/content/docs internal/cli/cli.go` finds nothing, and
`task docs:check` passes.

### UX-91 Three moments the command line names no next step

Impact: low · Effort: small

**Today.** The scriptable writes say what to do when stdin is closed:
"pass --yes", exit 2 (`writeOptions.proceed`,
`internal/cli/scriptable.go:145`, and `docs/content/docs/scripting.md:219`
under "Writing without a person"). Three other moments end without a
pointer.

- `internal/cli/config_cmd.go:363` (`collectJira`): the guided `init`
  returns `prompt.Line`'s error raw (`:365`), and so do its other
  questions. Only `confirm` (`internal/cli/prompt.go:46`) and `slack
  login`'s `askFor` (`internal/cli/slack_cmd.go:146`) map `io.EOF` to
  `errNoTerminal`, and `io.EOF` belongs to no family in `exitFamilies`
  (`internal/cli/scriptable.go:357`), so `workflow config init <
  /dev/null` prints "workflow: EOF" and exits 1 with no mention of
  `--template`. A final line typed without a newline comes back from
  `terminalPrompt`'s `ReadString` together with `io.EOF`
  (`cmd/workflow/main.go:44`) and is discarded with it.
- `internal/cli/web.go:151` (`serveWeb`, reached from the root's `--web`
  branch at `internal/cli/cli.go:206`): every load error, `ErrNotFound`
  included, prints "configuration did not load cleanly: %v" and then
  serves. Its siblings branch on `ErrNotFound` and print
  `NoConfigHeadline`, `InitStep` and `DoctorStep`: `showLoadError`
  (`internal/cli/config_cmd.go:102`), `reportLoadError`
  (`internal/cli/doctor.go:286`) and the interface's `configErrorStatus`
  (`internal/tui/render.go:426`). The web cannot write a first file
  (`docs/content/docs/web.md:434`, under "What stays in the terminal"),
  so the one surface that most needs `workflow config init` named is the
  one whose start never names it.
- `internal/cli/pr.go:176` (`runPR`): the command ends with `followUp`,
  so "Moved PROJ-2 to In Review" (`:284`) is its last word, while the
  interface's spine keeps "nothing announced" in view
  (`docs/content/docs/usage.md:52`, "The screen") and `announce`'s own
  refusal points the other way, "(open one with workflow pr)"
  (`runAnnounce`, `internal/cli/announce.go:158`).

**Instead.** Map `io.EOF` from the guided flow's prompts to
`errNoTerminal` with "pass --template to write a file to edit by hand",
exiting 2 like the writes; in `serveWeb`, test `config.ErrNotFound` as
`showLoadError` does and print the three shared hint constants before the
serving line; when messaging is configured, end `pr` with a stderr note
"Announce it with workflow announce", so stdout stays the artifact.

**Done when.** A `config init` test whose `Line` returns `io.EOF` gets
exit 2 and an error naming `--template`; a root test with no file and
`--web` finds "workflow config init" on stderr and not "did not load
cleanly"; a `pr --yes` test with messaging configured sees "workflow
announce" on stderr, and one without messaging does not.

### UX-92 The scriptable output lacks a unique label, a timestamp and `--json` on `pr` and `standup`

Impact: low · Effort: small

**Today.** A script reading the JSON has no stable key to join on, no
time to compare, and no JSON at all from `pr` or `standup`. This entry owns
`--json` for both; UX-62 keeps the other missing flags.

- `internal/cli/status.go:202` (`repoLabel`): the label is the base name,
  and "." is returned as itself (`:205`), so `status .` labels the row "."
  (`TestStatusAcrossLabelsTheCurrentDirectory`,
  `internal/cli/status_test.go:96`, pins that prefix) and `status ~/a/api
  ~/b/api` gives two rows the same `repository`, the only key
  `docs/content/docs/scripting.md:137` (under "JSON") offers.
- `internal/cli/reviews.go:151` (`reviewReport.Age`): a string, filled by
  `renderReviewsJSON` (`:156`) through `humanizeAge` (`:130`), which
  rounds 25 h and 47 h alike to "1d"; `ReviewRequest.OpenedAt`
  (`internal/forge/pulls.go:194`) holds the time and never reaches the
  JSON. `docs/content/docs/scripting.md:153` documents `"age": "3d"` as
  the shape, so `jq 'map(select(.age > "2d"))'` compares strings.
- `internal/cli/pr.go:168` (`runPR`): the only machine-facing result is
  "Opened " + `Sigil()` + number + URL, and the sigil is `!` on GitLab
  (`internal/forge/remote.go:70`). `docs/content/docs/scripting.md:103`
  keeps `--json` to the reads, a description rather than a decision.
- `internal/cli/standup.go:75` (`newStandupCmd`): the command declares
  `--days` and `--no-edit` and nothing else, so the gathered issues and
  pull requests reach a script only as the Markdown draft.

**Instead.** Label a `status` row by the cleaned absolute path's base
name, falling back to the full path when two arguments would share a
label, and carry the given path in a second `path` field; add `opened_at`
(RFC3339 UTC, from `OpenedAt`) beside `age` and document it; give `pr` a
`--json` printing `{pull: {number, url, title, draft, …}, warning,
follow_ups}`, the web's `OpenedPullRequest` shape
(`api/openapi.yaml:3155`), with the dry run printing the draft in the
same shape; and give `standup` a `--json` printing the gathered sections
in place of the draft — `standup --json` becomes `summary --json` if
FEAT-86 lands.

**Done when.** `status .` in a repository labels the row by the
directory's name and `status a/api b/api --json` yields two distinct
`repository` values; `TestReviewsAsJSONReportsEachOldestFirstWithItsAge`
(`internal/cli/reviews_test.go:163`) also parses `opened_at` back into the
time it seeded; `workflow pr --yes --json | jq .pull.number` prints 7 in a
test, with the notes still on stderr; a `standup --json` test parses its
stdout as JSON holding the seeded issue key.

### UX-93 `pr` confirms a body the person never saw

Impact: medium · Effort: small

**Today.** `runPR` previews "Open TITLE" and "BRANCH → BASE"
(`internal/cli/pr.go:129`, `:130`), and the code owners when there are
any (`:133`), asks, and then sends the request (`:163`), whose body is
composed from the template, the commit subjects and the issue link
(`internal/loop/pull.go:166`), without ever showing it; `newPRCmd`'s
`Long` says "A preview is confirmed first." (`internal/cli/pr.go:65`).
The interface shows the body's first `prBodyPreviewLines` in
`prComposer.view` (`internal/tui/prcomposer.go:282`, the body at `:308`),
the web's `draftDTO` carries `Body` for the form to show
(`internal/webserver/pullrequest.go:217`, `:220`), and the command line's
sibling writes print their whole payload (`runAnnounce`,
`internal/cli/announce.go:175`; `runStandup` at
`internal/cli/standup.go:141`). A stale or wrong template is discovered
on the forge, after the open, on the one surface whose help promises a
last look. `docs/content/docs/scripting.md:94` (the `pr` row under
"Standard output and standard error") documents the two-line stdout, and
no commit or doc records a decision to omit the body.

**Instead.** Print the body under the header lines on stdout, where the
artifact goes, so `workflow --dry-run pr` shows the whole pull request
and the question is asked about what was shown; update the `pr` row in
scripting.md.

**Done when.** `TestPRDryRunPreviewsWithoutOpening`
(`internal/cli/pr_test.go:97`), with a template written as
`TestPRPushesThenOpensAnUnpublishedBranch` (`internal/cli/pr_test.go:271`)
writes one, sees the template's text on stdout.

### UX-94 doctor has no row for the store's silent fallback

Impact: low · Effort: small

**Today.** When no data directory can be found, the store turns itself off
without a word, and `doctor`, the command that explains the machine, has
no row that would say so.

- `internal/store/dir.go:15` (`DefaultDir`): `home, _ :=
  os.UserHomeDir()` drops the error, so an empty home reaches `Dir`
  (`:24`), which returns `ErrNoDir` for it.
- `internal/wiring/wiring.go:330` (`onDisk`): `dir, _ :=
  store.DefaultDir()` drops `ErrNoDir`, and the store is built on an empty
  directory; `storeDeps`' doc comment (`internal/wiring/wiring.go:334`)
  calls the no-op intended, "the interface simply learns nothing", which is
  what makes this a discoverability gap rather than a defect.
- `internal/store/store.go:175` (`Store.off`): `s.disabled || s.dir ==
  ""`, so every seam no-ops on the empty directory with no signal outward.
  On a home-less machine the interface opens with no seeded issue list and
  forgets the last scope and every announcement, while `store.disabled` is
  still false.
- `internal/cli/doctor_requirements.go:142` (`externalTools`) lists the
  programs, and no doctor file mentions the store or its directory.
  `db-clean` is the one command that says it, "finding the local data"
  (`localData`, `internal/cli/dbclean_cmd.go:172`), and only when asked to
  clean.

**Instead.** A doctor row that prints the store's directory, or
`ErrNoDir`'s sentence when there is none, and "off (store.disabled)" when
the configuration turned it off; the same field in `doctor --json`.

**Done when.** `workflow doctor` with `HOME` and `XDG_STATE_HOME` unset
prints a line naming the store and that no data directory could be
determined; with both set, it prints the directory.

### UX-95 The reference shows no example and never names an exit status

Impact: low · Effort: small

**Today.** clig.dev asks help to lead with examples. No command in
`internal/cli` sets cobra's `Example` (`grep -l Example internal/cli/*.go`,
tests excluded, matches no file), so no generated reference page and no
`--help` carries an Examples section, and no page under
`docs/content/docs/reference` names an exit status (`grep -ril exit` over
the directory matches nothing).

- `docs/content/docs/reference/workflow.md:27` and
  `docs/content/docs/reference/workflow.md:31` (the root "Synopsis"):
  `workflow config init` and `workflow doctor`, the only command
  invocations a generated page shows, as prose inside the root synopsis;
  the hand-written index adds only two `--help` lines
  (`docs/content/docs/reference/_index.md:18`).
- `docs/content/docs/scripting.md:16` (under "Scripting"): the five real
  examples, `status --json`, `--dry-run pr`, `pr --yes`, `standup
  --no-edit --yes` and `--log … doctor --online`, live here and nowhere
  else; a reader of `workflow pr --help` must infer them from the flag
  descriptions.
- `internal/cli/status.go:52` (`newStatusCmd`'s `Long`): "the command
  then fails, as it does outside a repository", the text
  `docs/content/docs/reference/workflow_status.md:21` (its "Synopsis") is
  generated from, where `docs/content/docs/scripting.md:34` (the "Exit
  status" table's family 4 row) names "a directory that is not a git
  repository" as exit 4.
- `internal/cli/config_cmd.go:118`, generated into
  `docs/content/docs/reference/workflow_config_show.md:17` (its
  "Synopsis"): "fails, as doctor does" with no configuration file, where
  `docs/content/docs/scripting.md:33` (family 3) says exit 3.
- `docs/content/docs/reference/_index.md:9` ("Command reference"): says
  every command and flag is here and links nowhere;
  `docs/content/docs/scripting.md:13` links back to the reference, but
  nothing on the reference index points at the one page where the
  0/1/2/3/4/5/130 contract lives. The sidebar lists Scripting beside the
  reference, so it is findable, just not from here.

**Instead.** Set `Example` on each scriptable command with the lines
scripting.md already shows, so `--help` and the generated page carry them
together; have the reference index point at the Scripting page for exit
status and streams; let a synopsis that says "fails" name the family.

**Done when.** The pages for `status`, `reviews`, `standup`, `branch`,
`pr`, `announce` and `doctor` each have an "### Examples" section and
`task docs:check` passes; `docs/content/docs/reference/_index.md` links
`/docs/scripting`; the status page's synopsis names the exit status a
non-repository directory produces.

## The terminal interface

What is open here is a screen-reader mode, the alternate screen and a fixed
delay; undo; vim's missing keys; sentences that name a rebindable key; an
in-flight mark only the Issues pane wears; two second-path acts; a checkbox
that borrows the status shapes (UX-100); a draft the review queue does not
show; one verb on different keys, and two bindings that take a text field's
editing keys; six words for `esc`; empty states written two ways; the
screen at 80 by 24; a calendar whose up and down move a day; and a long
path that breaks a Repositories row.

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint
(`UI.DrawColor`, `internal/config/ui.go:53`; `Model.WithoutColor`,
`internal/tui/tui.go:162`); `ui.ascii` swaps glyphs and borders
(`asciiGlyphs`, `internal/tui/glyphs.go:45`); escapes in server text are
neutralized. But there is no screen-reader mode; the alternate screen is
unconditional (`Model.View`, `internal/tui/render.go:38`, `view.AltScreen
= true`), so nothing the interface prints survives quitting; `ui.color`
has no `always` for a piped terminal that does support color; and the
150 ms detail delay (`detailDelay`, `internal/tui/detail.go:20`) is fixed.
A terminal at 80 by 24 gets the full-screen layout at its tightest,
and nothing offers a plain mode that prints a line at a time
instead; the command line is the only other way in.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`, which `Config.validateUI` (`internal/config/ui.go:61`) and the
`UIConfig.color` enum (`api/openapi.yaml:3432`) refuse until they list it;
`ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (the commit, `commitComposer.handleKey`,
`internal/tui/composer.go:257`; the pull request, `prComposer.handleKey`,
`internal/tui/prcomposer.go:351`), a dirty tree blocks a switch instead of
stashing (`errDirtyTree`, `internal/tui/switchtask.go:25`), and quit is
guarded while an announcement waits (`quitGuard`,
`internal/tui/messaging.go:67`). But a posted comment, an applied
transition, a merge and the `git branch -D` in finish
(`finishPreview.commands`, `internal/tui/finish.go:76`) have no undo, and
the interface never says which acts are reversible: the finish preview
lists the three commands and says nothing of what `-D` costs
(`finishPreview.view`, `internal/tui/finish.go:82`), and a comment's notice
is "commented on KEY" (`commentPosted.apply`,
`internal/tui/comment.go:144`).

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries the
issue's link so the comment can be edited where it lives. Undo itself,
over a log of what workflow did, is FEAT-87's.

**Done when.** Each irreversible act's preview or notice says so.

### UX-69 Vim habits stop at `j`/`k`

Impact: low · Effort: small

**Today.** `up/k`, `down/j`, `pgup/K`, `pgdn/J` (`movingKeys`,
`internal/tui/keys.go:224`–`:227`). Nothing jumps to the ends of a list or
the detail. vim's own keys for that are spoken for: `g` is set-up-lefthook
on the Commits pane (`internal/tui/keys.go:261`) and go-to-directory on
the Repositories pane (`:301`). `h` and `l` move the cursor only in the
comment composer's NORMAL mode (`writingKeys`, `:346`–`:347`); elsewhere
`h` runs pre-commit on the Commits pane (`:260`) and `l` shows a job's log
(`:325`), and `←`/`→` change the commit type or the channel.

**Instead.** `home` and `end`, which no context binds, on every list and
the detail, with `G` — also free in every context — for the end; leave
`g`, `h` and `l` with the verbs they carry now.

**Done when.** `end` and `G` on the Issues list select the last loaded
issue, `home` the first, and `CheckKeys` still passes the default set.

### UX-96 Ten sentences name a key that `ui.keys` can move

Impact: low · Effort: small

**Today.** `ui.keys` moves an action to another key and the help follows
it — "The help then shows the new key", the `ui.keys` row of
`docs/content/docs/configuration.md:131` — but ten sentences carry the
default key as a literal, so a rebound user is told to press a key that
does something else or nothing. `wording` (`internal/tui/failure.go:58`)
states the design the first of them breaks: the full form names no key,
and each surface's footer offers its own. Others already build the key
from the binding — the comment composer's "draft kept for KEY; c picks it
up again" reads `m.keys.comment` (`commentComposer.close`,
`internal/tui/commentcomposer.go:342`), and the Tasks offers read
`m.keys.refresh` and `m.keys.narrowTasks` (`internal/tui/tasklist.go:309`,
`:315`, `:317`) — so the ten are the exceptions. Counted by reading every
string literal in `internal/tui/*.go` (tests excluded) that names a key
and checking that key's action is bound through `helpBuilder.bind`; no
single grep finds all ten.

- `internal/tui/detail.go:392` `Model.fullDetail` appends "press r to try
  again" after the failure block, while refresh is rebindable
  (`internal/tui/keys.go:244`, `issueKeys`): with `{"refresh": "ctrl+l"}`
  `r` does nothing there.
- `internal/tui/branch.go:135` `Model.branchDetail` says "Check out a
  branch, or press b to start one for the selected issue." on a detached
  HEAD; new-branch is rebindable (`internal/tui/keys.go:250`,
  `branchAndCommitKeys`).
- `internal/tui/branch.go:352` `branchCreator.view` says "could not fetch;
  enter branches from what you already have"; apply is rebindable
  (`internal/tui/keys.go:359`, `everywhereKeys`), and the creator's own
  footer already reads it from the binding — `branchCreator.footer`
  (`internal/tui/branch.go:382`) relabels `keys.confirm` to "branch from
  what you have", so a rebound session shows "ctrl+s branch from what you
  have" under a sentence that says enter.
- `internal/tui/commits.go:143` `Model.commitsDetail` says "Press g to set
  up lefthook." under a footer that reads its key from `keys.hookConfig`;
  set-up-lefthook is rebindable (`internal/tui/keys.go:261`,
  `branchAndCommitKeys`).
- `internal/tui/review.go:294` `Model.reviewDetail` says "n opens one from
  this branch's commits and the repository's template."; open-pull-request
  is rebindable (`internal/tui/keys.go:266`, `reviewAndMessagingKeys`), and
  the pane answers `m.keys.newPullRequest`.
- `internal/tui/review.go:324` `Model.reviewDetail` says "e edits its title
  and description."; edit is rebindable (`internal/tui/keys.go:319`,
  `composerKeys`), and the pane answers `m.keys.edit`.
- `internal/tui/finish.go:27` `Model.mergedDetail` says "F finishes the
  branch: …"; finish-branch is rebindable (`internal/tui/keys.go:270`,
  `reviewAndMessagingKeys`).
- `internal/tui/finish.go:31` `Model.mergedDetail` says "n opens a new
  pull request from this branch's commits." once one has merged;
  open-pull-request is rebindable (`internal/tui/keys.go:266`), and the
  pane answers `m.keys.newPullRequest`.
- `internal/tui/checks.go:58` `checkList.view` says "Open a check's page
  with enter."; `checkList.handleKey` (`internal/tui/checks.go:113`) opens
  on `m.keys.confirm` (`:121`), the rebindable apply.
- `internal/tui/composer.go:230` `commitComposer.footnotes` says "no body
  yet: ctrl+o writes one in your editor"; `commitComposer.handleKey`
  (`internal/tui/composer.go:254`) answers `m.keys.editBody` (`:262`), and
  edit-body is rebindable (`internal/tui/keys.go:320`, `composerKeys`).

**Instead.** Build each sentence from the binding — `m.keys.refresh`,
`m.keys.newBranch`, `m.keys.confirm`, `m.keys.hookConfig`,
`m.keys.newPullRequest`, `m.keys.edit`, `m.keys.finish`,
`m.keys.editBody`, through `Help().Key`, as the comment composer does — or
drop the key from the sentence and let the footer beside it carry the
offer, as `wording` intends.

**Done when.** A screen test that rebinds refresh, new-branch, apply,
set-up-lefthook, open-pull-request, edit, finish-branch and edit-body
through `cfg.UI.Keys`, as `internal/tui/issuesfooter_test.go:203` does for
apply and close, sees the bound key, or no key, in each of the ten
sentences and never the literal `r`, `b`, `enter`, `g`, `n`, `e`, `F` or
`ctrl+o` there; the detached case of
`TestTheBranchPaneSaysWhereTheBranchStands`
(`internal/tui/branch_test.go:126`),
`TestAFailedFetchOffersToBranchFromWhatIsThere`
(`internal/tui/fetch_test.go:35`), the screens that pin the Review
detail's two sentences (`TestTheReviewPaneOffersToOpenAPullRequest`,
`internal/tui/review_test.go:154`;
`TestTheReviewPaneOffersEditingThePullRequest`,
`internal/tui/preditor_test.go:17`) and the merged detail's
(`TestNIsOfferedWhileNoPullRequestIsOpen`,
`internal/tui/review_offer_test.go:45`) gain the rebound variant.

### UX-97 The in-flight mark is on one pane of nine, and the waits say it four ways

Impact: low · Effort: small

**Today.** `Model.paneTitle` (`internal/tui/render.go:187`) promises the
in-flight glyph "until the answer arrives", but `Model.loading`
(`internal/tui/render.go:196`) answers only for `paneIssues`, and only
`issueList` carries a `loading` flag (`internal/tui/issues.go:80`).
Refresh is live on all nine panes — every entry of `behaviorOf`
(`internal/tui/panes.go:99`) has one, and each pane's key handler answers
`m.keys.refresh` — so `r` on eight of them changes nothing on screen until
the answer lands, and neither does the silent reload `refreshPane`
(`internal/tui/panes.go:166`) runs when you switch to a stale pane. On a
slow forge, `r` on the Review pane looks ignored until `CheckStatus`
answers. Three Issues searches also start without the flag, so the one
pane that has the glyph omits it for them: after "● PROJ-412 is now Done"
the list quietly re-sorts some time later, where `r` would show "1 Issues
◐" for the same wait; `v` onto a view the store remembers shows
yesterday's list with no sign today's is on its way; and a session that
opens on the cache does the same while `Init`'s search runs.

Where a pane does say it is waiting, before its first answer, it says so
in its own word, and none of them wears `◐`:

- "loading…" — the Issues list (`issueList.render`,
  `internal/tui/issues.go:275`), the Branch rail (`Model.branchRail`,
  `internal/tui/branch.go:84`) and the Commits rail
  (`Model.commitsRail`, `internal/tui/commits.go:104`).
- "looking…" — the Review rail (`Model.reviewRail`,
  `internal/tui/review.go:263`), the Reviews rail and detail
  (`internal/tui/reviewqueue.go:152`, `:168`), the Tasks rail and detail
  (`internal/tui/tasks.go:180`, `:230`), and the go-to prompt checking a
  typed path (`dirPrompt`, `internal/tui/reposwitch.go:266`).
- "checking…" — CI on the Review pane (`Model.ciSummary`,
  `internal/tui/review.go:233`).
- "reading…" — the Summary rail and its sources
  (`internal/tui/summary.go:324`, `:375`), the diff
  (`internal/tui/diff.go:77`) and a job's log
  (`internal/tui/checks.go:301`).
- The Repositories rail says "favorites, read when opened"
  (`Model.favoritesCount`, `internal/tui/repositories.go:217`) for as long
  as `m.repositories.read` is false — which includes the whole first read
  the opening started, so the rail tells you to open the pane you are
  looking at, over a detail whose Favorites heading stands empty.

Only the overlays and the Messaging pane put `◐` before their wait —
"◐ announcing…" (`internal/tui/messaging.go:145`), "◐ sending…" on the
Tasks rail (`internal/tui/tasks.go:184`), "◐ reading the user groups…"
(`internal/tui/people.go:299`).

The searches that skip the flag:

- `internal/tui/picker.go:171` `transitionApplied.apply` batches
  `searchIssues` without setting `m.issues.loading`, unlike
  `Model.refreshIssues` (`internal/tui/issuekeys.go:168`); since the flag
  is clear, `Model.loadMoreIssues` (`internal/tui/detail.go:111`) is not
  held back either, so `ctrl+n` starts a second page while the search is
  still out.
- `internal/tui/views.go:61` `Model.nextIssueView` sets `loading` only
  when the seeded list is not settled, though `relistIssues` always
  follows (`:65`).
- `internal/tui/tui.go:134` `New` seeds `model.issues` from the cache with
  no loading flag while `Model.Init` (`internal/tui/tui.go:193`) always
  starts `searchIssues` — the same gap at startup.

**Instead.** Give each pane's state a loading flag set where its refresh
command is built and cleared by its applier, and let `loading` read it per
pane through `behaviorOf`, so every refresh, and every reload on a switch,
shows "◐" in the title; set `m.issues.loading = true` unconditionally after
seeding in `nextIssueView` and `New`, and before batching the search in
`transitionApplied.apply`. Say a first wait one way on every pane —
"◐ loading…", say — and let the Repositories rail say "loading…" while its
first read is out, keeping "read when opened" for before it starts.

**Done when.** A test whose `CheckStatus` never answers within the horizon
presses `4`, `r` and sees "Review ◐" in the title, and the same for `2`,
`3`, `5`, `6`, `7`, `8` and `9` with their seams;
`TestEnterMovesTheIssueAndRefreshesTheList`
(`internal/tui/picker_apply_test.go:15`) requires "1 Issues ◐" between the
move and the refresh's answer; a views test with `CachedIssues` for the
second view shows "◐" after `v` until the search answers, and a `New` with
a seeded cache shows it until `Init`'s search answers; `grep -n
'"looking"\|"checking"'` in `internal/tui` finds nothing outside tests.

### UX-99 The worktree and review-status paths give less than their twins

Impact: low · Effort: small

**Today.** Two acts reached by a second path come back poorer than by the
first. `b` then enter on a To Do issue offers "Change status ▸ ◐ Start";
`b`, `ctrl+g`, enter on the same issue offers only to switch to the new
worktree, though the work has just as surely started. After opening a
pull request the Change status overlay reads "PROJ-412 " and "status  "
with empty values, unlike the same picker opened by `t`, though the list
usually holds the issue.

- `internal/tui/branchresult.go:109` `worktreeCreated.apply` closes with
  "worktree for NAME at PATH" and follows up with `worktreeOffer` alone,
  where `branchCreated.apply` (`internal/tui/branchresult.go:74`) calls
  `pickStatusFor(msg.issue, statusOffer{inProgress: true})` (`:93`).
- `internal/tui/branchresult.go:99` `worktreeCreated` carries `name`,
  `path` and `err` only — no `issue` or `forIssue` to offer from.
- `internal/tui/picker.go:246` `Model.offerReviewStatus` passes
  `jira.Issue{Key: issueKey}` with an empty summary and status, where
  `Model.openStatusPicker` (`internal/tui/picker.go:210`) passes the
  listed issue.
- `internal/tui/picker.go:259` `statusPicker.header` draws the key, a
  space and the summary, then "status  " and the status — both empty on
  that path.

**Instead.** Carry `issue` and `forIssue` on `worktreeCreated` as
`branchCreated` does and make the same offer, then the worktree offer
after it; in `offerReviewStatus` look the key up with `issueList.find`
(`internal/tui/issues.go:246`) before opening, falling back to the bare
key only when it is not listed.

**Done when.** A case in `internal/tui/statusafterbranch_test.go` that
creates a worktree for PROJ-388 shows the Change status overlay
pre-selected on Start; `TestLinkingAPullRequestThenOffersTheReviewStatus`
(`internal/tui/issuelink_test.go:27`) also requires the issue's summary
and "status  In Progress" in the overlay.

### UX-100 The checkbox borrows the status shapes

Impact: low · Effort: small

**Today.** In the Fix Version/s step "● 1.0" means chosen, and on the
screen before it "● Done" meant a done status, so a reader who learned the
glyph vocabulary reads the checkbox as a state. The same checkbox now
draws every facet checklist too — the Filter checklist on the Issues,
Tasks and Reviews panes — where "● in flight" means
the place is chosen, beside a list whose `◐` means in flight.

- `internal/tui/glyphs.go:76` `glyphs.checkbox` returns `g.done` for
  chosen and `g.notStarted` for unchosen — the fields `glyphs.status`
  (`internal/tui/glyphs.go:58`) maps `CategoryDone` and `CategoryNew` to.
- `internal/tui/fields.go:179` `fieldForm.optionLines` draws that checkbox
  (`:187`) beside each option of the overlay whose transition rows
  (`statusPicker.transitionRow`, `internal/tui/picker.go:286`) use
  `status` one keystroke earlier.
- `internal/tui/overlay.go:341` `checklist.choiceRow` draws it beside each
  facet, the Issues places among them (`markInFlight`, "in flight",
  `internal/tui/issueplaces.go:25`).

The diff is the other place the earlier edition named: red for a removed
line and the forge's green for an added one (`Model.markDiffLine`,
`internal/tui/diff.go:111`, `:113`). The code declares it an exception
(`styles`, `internal/tui/glyphs.go:90`), the `+` and `-` git leaves in
place carry the mark by shape (`Model.diffSection`,
`internal/tui/diff.go:65`), and this edition's "The visual system" now
names it as the one exception, so that half is closed.

**Instead.** Give the checkbox its own shape pair in both glyph sets
(`[x]`/`[ ]` in ASCII, `☑`/`☐` in Unicode), keeping `○ ◐ ● ✗` for state
alone.

**Done when.** `TestAChosenVersionCanBeToggledOff`
(`internal/tui/fields_test.go:121`) and a checklist test assert a
checkbox glyph that is neither `●` nor `○`.

### UX-101 A draft review request looks ready in the queue

Impact: low · Effort: small

**Today.** A draft asking for your review looks like a ready one in the
terminal's queue, is marked "· Draft" in the browser, and the Review pane
labels the branch's own draft. The queue can even be filtered to drafts,
yet no row says which ones they are. (The other half of the earlier
entry has shipped: forge issues listed without Jira now open and copy,
since `forgeIssuesDeps` wires `BrowseURL`,
`internal/wiring/forgeissues.go:74`.)

- `internal/tui/reviewqueue.go:230` `Model.reviewTail` draws the
  repository, "by ", the author, the separator and the age — never
  `Draft`, which `forge.ReviewRequest` carries
  (`internal/forge/pulls.go:189`) and
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:307` prints as
  "· Draft".
- `internal/tui/review.go:315` `Model.reviewDetail` labels the branch's
  own pull request "draft".
- `internal/tui/reviewfacets.go:150` offers "draft" and "ready" in the
  Filter checklist.

**Instead.** Append "draft" to `reviewTail` when the request is one, as
`reviewDetail` labels the branch's own.

**Done when.** A test with a draft review request sees "draft" on its row
in pane `6`, and a ready one does not.

## The web

What is open here is a stream that marks no change; Settings' unseen
sections, misleading hints, blank selects, failed read and unremovable
credential; what the browser could borrow from the interface; the Branch
section's push, the CI counts and the detached HEAD; keyboard focus,
placeholder, field-boundary and focus-ring contrast; a staged state drawn
as a colored word; type, measure, inputs and buttons outside the system;
ragged rows, an empty form's heading and a far-off Copy URL confirm;
links, stages and controls that tell less than their siblings; failures
said without a voice, loading drawn four ways, facts run together with
middle dots, three ways to write a date; and copy settled site by site.

Every pointer here was checked again at `990f333`. A screenshot named by
its file (`1440-dark-tasks.png`, say) is from a pass on 2026-10-05 over
every section at 640, 1024 and 1440 px in both themes, of the mock build,
and like the 2026-09-24 audit's is not in the repository.

### UX-86 Nothing marks a change the stream just made

Impact: low · Effort: medium

**Today.** `StreamStatus` shows `Connecting` / `Live` / `Reconnecting` /
`Out of date` (`web/src/shell/StreamStatus.tsx:7`), but nothing says when
the last snapshot arrived — the store keeps `receivedAt`
(`web/src/api/snapshot.ts:28`, set on every frame at `:78` and `:100`) and
nothing draws it — and a panel that changed because CI settled looks
exactly like one that re-rendered. There is no toast, and no "CI passed"
moment on the web where the interface rings the terminal
(`ciFinishNotice`, `internal/tui/review.go:155`).

**Instead.** A "updated 3 s ago" beside the pill, read from `receivedAt`;
a brief highlight on the row a snapshot changed; a status line when CI
settles, honoring `ui.notify` (`internal/config/ui.go:34`).

**Done when.** A snapshot that flips CI to passed produces a status
region saying so.

### UX-87 Settings can edit eight sections and carry six it cannot show

Impact: low · Effort: medium

**Today.** The form seeds itself with the whole `Config` (`ConfigForm`,
`web/src/features/settings/SettingsPanel.tsx:82`, its comment at `:79`
naming what rides along), so `ui`, `timing`, `jira.headers`, `jira.views`,
`branch.prefixes` and `messaging.channels`
(`internal/config/config.go:124`) survive a save unchanged — and cannot
be edited. The eight fieldsets it draws (`:141` to `:148`) are Jira,
messaging, the forge, commits, branches, pull requests, the store and
Taskwarrior. There is no guided, credential-checking flow like `workflow
config init`; the web edits an existing file only.

**Instead.** Fieldsets for the six, with `views`, `prefixes` and
`channels` as editable lists. A first run from no file is UX-153's.

**Done when.** A view added in the browser appears in the interface's `v`
cycle, and a channel added there is offered in the announcement preview.

### UX-88 What the browser could borrow from the interface

Impact: low · Effort: medium

**Today.** The interface shows a per-file diff under the changes list
(`loadDiff`, `internal/tui/diff.go:29`), amends (`A`) and fixups (`f`),
jumps to a failure in `$EDITOR` (`openFailure`,
`internal/tui/run.go:382`), edits an open pull request
(`internal/tui/preditor.go`), and cycles the repository's pull-request
templates (`ctrl+t`). None has a web equivalent, and the web takes the
first template only (`firstTemplate`, `internal/loop/pull.go:171`); the
`?` sheet is UX-152's. Four smaller things the terminal shows are
absent on the web too, none of them among what
`docs/content/docs/web.md` says stays in the terminal.

- A renamed file shows only its new path: `ChangeRow` renders
  `change.path` alone (`web/src/features/branch/WorkingTree.tsx:97`),
  though `changesDTO` sends `OriginalPath`
  (`internal/webserver/dto.go:175`) and `original_path` is read nowhere in
  `web/src` outside the generated client; the terminal's `changeRows`
  draws old → new (`internal/tui/commits.go:153`).
- The commit subject has no length against `commit.subject_limit`:
  `MessageFields`' Subject is a bare `<input required placeholder>`
  (`web/src/features/branch/CommitForm.tsx:160`) though `subject_limit` is
  on the wire (`CommitConfig`,
  `web/src/api/generated/types.gen.ts:1347`); the terminal's
  `commitComposer.view` shows "n/limit" as typed
  (`internal/tui/composer.go:152`). The limit is met only as a 422 after
  the click.
- The Review section has no Copy URL: the title link is the only handle
  on the pull request (`PullRequestSummary`,
  `web/src/features/review/ReviewPanel.tsx:100`), where the queue's
  `CopyURL` one section over has a tested clipboard outcome
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:379`) and the
  terminal's `reviewKeys` offer `linkKeys` on the pull request
  (`internal/tui/review.go:417`).
- The Filter text survives a view change: `IssueBrowser` keeps `filter`
  in local state nothing resets
  (`web/src/features/issues/IssuesPanel.tsx:52`) — the places picked
  beside it are reset per view (`:55`), the text is not — and
  `ViewSelect`'s `onChange` calls `setView` alone
  (`web/src/features/issues/IssueListControls.tsx:59`), where the
  terminal's `nextIssueView` seeds a fresh list, filter and all
  (`internal/tui/views.go:49`); choosing a view with "proj-12" still in
  the filter shows "No loaded issue matches the filter." — the user asked
  for a view, not a narrowed one.

An Unstage all is missing on the web as in the terminal — `WorkingTree`
renders `StageAll` alone (`web/src/features/branch/WorkingTree.tsx:33`)
and the staging module has no unstage-all
(`web/src/features/branch/stagingApi.ts:33`) — so it is FEAT-23's, not an
idea to borrow. A Push branch offered on the base branch is UX-104's.

**Instead.** In rough order of value: a template select on the
pull-request form; a diff view; `original_path` drawn before the path
with an arrow; a muted "n/limit" hint under the subject counting the
assembled header; `CopyURL` beside the title with the same "Copied the URL
of #128." outcome; clearing the filter when the view changes.

**Done when.** Each lands with a role/name test; the template select shows
every repository template; a `WorkingTree` test with a renamed change
finds both paths in the row; a `CommitForm` test with `subject_limit: 20`
finds the count text change as the subject is typed; a `ReviewPanel` test
clicks "Copy URL to #128" and reads the URL back from the clipboard; a
test types a filter, selects another view, and finds the searchbox named
Filter empty once the new frame lands.

### UX-104 The Branch section's push counts what it cannot know, even on the base

Impact: medium · Effort: small

**Today.** Three lines in the Branch section print a number git counted
against nothing, and the push is offered where the terminal withholds it.
The push confirm is a last look, which the promises table holds to naming
what is about to happen, and it is the one that misleads; the two rows
above it share the defect.

- `PushConfirm` asks "Push {commits} commit(s) to the remote?"
  (`web/src/features/branch/BranchPanel.tsx:182`), fed
  `branch.commits.length` by `PushButton` (`:131`): the commits since the
  base, which `BranchSummary`'s own comment calls unknowable without a
  base (`:52`) and which `canPush` therefore ignores (`:54`). On a branch
  with no base the confirm reads "Push 0 commit(s)" and the push goes
  ahead; four commits since main and one ahead reads "Push 4"; and "the
  remote" never says which.
- A long branch reads the cap: `Branch.Truncated`
  (`internal/gitrepo/branch.go:82`) says the list is the oldest of a
  longer history, and the wire `Branch` carries no such flag.
- The server decides by the upstream on the push remote and ahead alone
  (`nothingToPush`, `internal/webserver/push.go:66`), so the count the
  confirm shows is not what the server checks.
- `BranchSummary` prints "{ahead} ahead, {behind} behind" whatever the
  upstream (`web/src/features/branch/BranchPanel.tsx:73`), so an
  unpublished branch reads "Upstream none / Tracking 0 ahead, 0 behind"
  over a Push branch button: the numbers say there is nothing to push and
  the button says there is.
- `Commits` says "No commits yet on this branch." whenever the list is
  empty (`web/src/features/branch/BranchPanel.tsx:90`), a base of `""`
  included, where the count is unknown.
- Push branch is offered on the base branch itself: `canPush` needs a
  name and no upstream on the push remote or ahead > 0
  (`web/src/features/branch/BranchPanel.tsx:54`), and `nothingToPush`
  accepts main ahead of origin/main (`internal/webserver/push.go:66`),
  where the terminal's `canPush` also requires `onFeatureBranch()`
  (`internal/tui/branch.go:212`).

The terminal's last look names the branch and the remote and no count
(`previewPush`, `internal/tui/branch.go:217`), and its `upstreamState`
says "not pushed yet" for a branch with no upstream (`:102`).

**Instead.** Word the confirm as the terminal does — "Push fix/PROJ-1 to
origin?" — naming branch and remote (the branch's `push_remote`) and no
count; with no upstream the Tracking row says "not pushed yet" and the
counts appear only once there is one; with no base the Commits section
says the base is unknown rather than that there are no commits; hide the
push when `branch.name` equals the base's short name, or move that guard
into `internal/loop` for both surfaces.

**Done when.** A `BranchPanel` test with `name: 'fix/PROJ-1'`,
`commits: []`, `upstream: ''` and `base: ''` opens the confirm and finds
the group named exactly "Push fix/PROJ-1 to origin?", finds "not pushed
yet" and no "ahead" in the summary, and does not find "No commits yet"; a
test with `name: 'main'`, `base: 'origin/main'` and `ahead: 1` finds no
Push branch button.

### UX-105 "0 of 0 done" over a GitLab pipeline or no checks

Impact: medium · Effort: small

**Today.** The CI heading always prints done of total, and the Review
section's `PullRequestSummary` renders the overall `ci.state` nowhere,
though the work story does ("#128 · CI running", `reviewDetail`,
`web/src/features/issues/WorkStory.tsx:189`) — and for state none it
prints "CI none" where the terminal says "no checks reported". A GitLab
user sees a heading that contradicts the row beneath it; a GitHub user
whose checks have not started sees "0 of 0 done" over nothing and cannot
tell whether CI has not started, is not configured, or failed to load.

- `PullRequestSummary` prints "· {ci.done} of {ci.total} done"
  unconditionally (`web/src/features/review/ReviewPanel.tsx:122`) and
  renders `ci.state` nowhere; the section draws whenever `ci` is non-null
  (`:117`), so state none with no checks is the heading over an empty
  list.
- `gitlabStatus` returns Total 0, Done 0, Failed 0 with one pipeline check
  for every GitLab pipeline (`internal/forge/gitlab.go:362`), and `CINone`
  with no checks when there is no head pipeline (`:350`); the `CI` type
  documents that the counts stay zero there (`internal/forge/ci.go:43`),
  and `ciTally.ci` yields `CINone` with Total 0 for a GitHub pull with no
  statuses or check runs (`:114`).
- `ciDTO` copies State, Total, Done and Failed through unchanged
  (`internal/webserver/dto.go:231`).
- The terminal's `ciSummary` adds "(done of total finished)" only when
  Total is positive (`internal/tui/review.go:240`) and says "no checks
  reported" for `CINone` (`:234`).

**Instead.** Print the count only when total is positive and otherwise the
state word ("CI checks · running"), as `ciSummary` does; when `checks` is
empty, replace the list with "No checks reported." beside the unknown
mark.

**Done when.** A `ReviewPanel` test with total 0 and one running pipeline
check finds the heading "CI checks · running" and no "0 of 0"; one with
state none and no checks finds "No checks reported" and no list role.

### UX-106 Twenty-seven controls let keyboard focus fall to the page

Impact: low · Effort: small

**Today.** Twenty-seven controls set native `disabled` while their request
runs, which drops focus in Chromium and WebKit, and nothing re-takes it.
A keyboard user who is refused — a dirty tree, a branch that already
exists, the dry-run hold on every Start work press, a Taskwarrior refusal
— hears the reason and is left at the top of the page.
`useAsyncAction`'s catch sets the error and the state only
(`web/src/lib/useAsyncAction.ts:51`), so `onDone` never runs on a refusal
and the panel's `OutcomeLine` has nothing to follow. Counted from `grep -rn
"disabled={" web/src --include='*.tsx'`, tests and `aria-disabled`
excluded: 41 lines, three of them a prop handed down, so 38 controls, of
which 27 stay mounted after a refusal with no code that re-takes focus.
The other eleven: the push (`PushButton` re-focuses its opener on an
error, `web/src/features/branch/BranchPanel.tsx:121`) and Local data's two
clean openers, which do the same
(`web/src/features/settings/people/LocalData.tsx:186`); the announce
preview's channel select, Cancel, Announce now and tag-group checkboxes
(`handBack()` runs before `post.run()`,
`web/src/features/messaging/MessagingPanel.tsx:181`); three controls that
did not have focus — the pull request form's Cancel
(`web/src/features/review/ReviewPanel.tsx:307`), a code owner's Forget…
held while the row's select saves
(`web/src/features/settings/people/PeopleTable.tsx:231`) and the forget
confirm's Cancel (`:330`); and a group checkbox held under `--dry-run`
alone (`web/src/features/settings/people/RepoGroups.tsx:79`). The 27, by
section:

- Issues: the list row's `RowCheckout`
  (`web/src/features/issues/IssuesPanel.tsx:494`, alert at `:503`); the
  story's `CheckoutButton` (`web/src/features/issues/WorkStory.tsx:344`,
  alert at `:353`) and `StartWorkButton` (`:376`, alert at `:385`), the
  path every dry-run press takes; `StartInWorktreeButton`
  (`web/src/features/issues/StartInWorktree.tsx:45`, alert at `:54`) and
  `SwitchToWorktreeButton` (`:126`, alert at `:136`) — while the
  worktree offer's own Switch to it beside them holds with `aria-disabled`
  (`:97`).
- Branch: `ChangeRow` (`web/src/features/branch/WorkingTree.tsx:104`,
  alert at `:115`) and `StageAll` (`:138`, alert at `:148`); `CommitForm`'s
  submit (`web/src/features/branch/CommitForm.tsx:95`, alert at `:103`);
  `LinkForm`'s Link (`web/src/features/branch/IssueLink.tsx:196`, refusal
  at `:204`), while Unlink in the same file holds with `aria-disabled`
  (`:116`).
- Review: `OpenPullRequest`'s compose button
  (`web/src/features/review/ReviewPanel.tsx:223`, alert at `:232`), whose
  `handBack()` runs only on the form's cancel; `PullRequestForm`'s submit
  (`:310`, alert at `:316`), which stays up on a refused open; and
  `FollowUpOffer`'s button (`web/src/features/review/OpenedOutcome.tsx:85`,
  alert at `:96`).
- Messaging: `AnnounceControls`' preview button
  (`web/src/features/messaging/MessagingPanel.tsx:195`, alert at `:205`);
  in the tag picker, an untagged owner's select and Not on Slack, held
  while a link saves (`web/src/features/messaging/TagPicker.tsx:236`,
  `:258`, alert at `:68`).
- Tasks: `TaskLineForm`'s submit
  (`web/src/features/tasks/TaskLineForm.tsx:73`, alert at `:77`), every
  `Verb` — Start, Stop, Done and the rest
  (`web/src/features/tasks/TaskDetail.tsx:240`, alert at `:255`) — and
  `TrackIssue`'s Track in Taskwarrior
  (`web/src/features/tasks/IssueTasks.tsx:187`, alert at `:195`).
- Settings: the configuration's Retry (`ConfigArea`,
  `web/src/features/settings/SettingsPanel.tsx:58`), its message in an
  `EmptyState` rather than an alert (UX-120); Save (`SaveControls`, `:163`),
  which reports "Saved." through its own `role="status"` span (`:166`), a
  second state machine beside the shared `OutcomeLine`, so a
  mouse-clicked Save disables itself under focus and the span does not
  take it; the changed-since-read Reload (`ChangedSinceRead`, `:193`,
  alert at `:201`); the three "Read … again" buttons of people, local
  data and groups (`web/src/features/settings/people/PeopleTable.tsx:43`,
  `web/src/features/settings/people/LocalData.tsx:47`,
  `web/src/features/settings/people/RepoGroups.tsx:27`), whose failures
  are said without a role (UX-140); a code owner's Slack select
  (`web/src/features/settings/people/PeopleTable.tsx:266`, alert at
  `:120`) and the forget confirm's Forget (`:339`, alert at `:323`); and
  Save groups (`web/src/features/settings/people/RepoGroups.tsx:94`, alert
  at `:100`).

The project already knows the rule, and keeps it in nine places: Load
more (`MoreIssues`, `web/src/features/issues/IssuesPanel.tsx:385`), the
review queue's Retry (`Queue`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:127`), the issue's
Retry (`IssueUnread`, `web/src/features/issues/IssueDetailPanel.tsx:113`),
Show log (`CheckLog`, `web/src/features/review/ReviewPanel.tsx:476`),
Unlink, Switch to it, the comment's send
(`web/src/features/issues/CommentComposer.tsx:383`), Repositories' Switch
(`web/src/features/repositories/RepositoriesPanel.tsx:176`) and the Tasks
Refresh (`web/src/features/tasks/TasksPanel.tsx:223`) all hold with
`aria-disabled`; and `canHoldFocus` treats a disabled control as unable
to hold focus (`web/src/lib/Outcome.tsx:98`).

**Instead.** Hold each running control with `aria-disabled` (guarding
`onClick` or `onChange` as `Queue`'s Retry does) so it keeps focus
through a refusal, and say Settings' save result through `useOutcome` and
`OutcomeLine` as the other panels do, keeping the changed-since-read
alert separate.

**Done when.** A test presses each of the twenty-seven against a refused
request and finds `document.activeElement` still on it once the refusal
is shown; a `SettingsPanel` test clicks Save with the mouse, awaits
"Saved.", and finds `document.activeElement` on the status line, not
`document.body`; the grep above finds no `disabled={` that names a
request's running state.

### UX-107 The web's detached HEAD offers no way out

Impact: low · Effort: small

**Today.** With HEAD detached, the Branch section heads itself "Detached
HEAD at abcdef1" and then draws the same Base, Upstream and Tracking list,
Commits and working tree as on a branch, with nothing saying what to do
next — and Link an issue, the one control below the list, is withheld
too (`web/src/features/branch/BranchPanel.tsx:77`). The terminal says it:
"Check out a branch, or press b to start one for the selected issue."

- `BranchSummary`, `web/src/features/branch/BranchPanel.tsx:47`: the
  detached heading (`:49`), followed by the branch's own list (`:66`).
- `BranchPanel`, `web/src/features/branch/BranchPanel.tsx:31`: renders
  `BranchSummary`, `Commits` and `WorkingTree` alike for a branch and a
  detached HEAD.
- `Model.branchDetail`, `internal/tui/branch.go:123`: the terminal's
  sentence (`:135`, whose literal `b` is UX-96's).

**Instead.** Under the detached heading, one sentence pointing at Issues,
where Check out and Start work live.

**Done when.** A `BranchPanel` test with `detached: true` finds a line
naming Issues under the heading.

### UX-108 Three Settings hints that hide where a value comes from or applies

Impact: low · Effort: small

**Today.** Three Settings fields say nothing, or the wrong thing, about
their value. Beside UX-87's six carried unseen, these are values carried
and misdescribed.

- `JiraFieldset`'s Token hint is "Leave as-is to keep the stored token."
  (`web/src/features/settings/fieldsets/JiraFieldset.tsx:18`; on screen in
  `1440-light-settings.png`), over a field that is empty after the
  recommended macOS setup: `keepTokenSafe`
  (`internal/cli/config_cmd.go:391`) clears `jira.Token` and sets
  `TokenCommand` (`:406`). A user who types a token to "fix" it writes a
  secret into the file, and "The file's own `token` wins when set"
  (`docs/content/docs/configuration.md:269`, under "Keeping tokens out of
  the file") — what `config init` worked to avoid.
- Announcement is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:50`), though
  `Messaging.Announcement` is a Slack-only template with seven
  placeholders (`internal/config/config.go:125`), which the fields table
  describes in one line (`docs/content/docs/configuration.md:121`). A
  Teams user edits it and sees no change; a Slack user has no placeholder
  list on screen.
- Channel is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:49`), though
  `Messaging.Channel` "applies to a Slack user token only; a webhook
  carries its own channel" (`internal/config/config.go:118`). A webhook
  user sees an editable channel that does nothing.

**Instead.** When `token_command` or `token_env` is set, replace the token
hint with "Taken from token_command: VALUE" (or the variable's name),
neither a secret; a hint on Announcement naming the placeholders, that it
applies to Slack only, and that empty keeps the built-in message; a hint
on Channel: "With a Slack user token; a webhook posts to its own channel."

**Done when.** A `SettingsPanel` test seeding `jira.token_command` finds
the Token textbox described by text naming that command;
`getByRole('textbox', { name: 'Announcement', description: /\{author\}/
})` and `getByRole('textbox', { name: 'Channel', description: /user token/
})` resolve.

### UX-109 Staged state is a colored word, not a StateMark

Impact: low · Effort: small

**Today.** Each file's staged, partly staged or unstaged tag is a word
colored `text-success` when staged and muted otherwise (`ChangeRow`,
`web/src/features/branch/WorkingTree.tsx:98`), with no `StateMark` in the
file: `1440-light-branch.png` shows "staged" in green and "unstaged" in
gray beside each file, and a conflict shows kind "conflicted" and tag
"unstaged" with no failed mark. It is the one state on the web drawn as a
colored word, bypassing `StateMark` (`web/src/shell/StateMark.tsx:36`),
which the settled system says every state goes through; `text-success`
elsewhere colors outcome lines, not a thing's state. The Review section's
State, Mergeable and Changes requested rows (`PullRequestSummary`,
`web/src/features/review/ReviewPanel.tsx:111`; `ReviewRows`, `:158`) and
the queue's "· Draft"
(`web/src/features/reviewqueue/ReviewQueuePanel.tsx:307`) are states drawn
as plain words with no mark too, uncolored. The terminal's `stageGlyph`
"says by shape how much of a change is staged"
(`internal/tui/commits.go:170`), four states by four glyphs. The word
keeps it accessible, so this is consistency inside the system, not a
change to it.

**Instead.** A `StateMark` before the word — not-started, in-flight for
partly staged, done, failed for a conflict — carrying the status light,
with the word in the plain foreground.

**Done when.** The Branch screenshots show a shape before each staged
word, and `web/src/features/branch/WorkingTree.tsx` has no `text-success`
on the tag.

### UX-110 Light-theme placeholders read at 3.18:1 in six fields

Impact: medium · Effort: small

**Today.** Four input classes set no placeholder color, so Tailwind's
preflight draws their placeholders at 50% of the foreground: 3.18:1 on the
light page, below the 4.5:1 floor the gates promise, and 4.58:1 in the
dark theme. A 1440 px light Branch screenshot, taken in the 2026-09-24
audit, sampled the Subject hint at (137,139,144) on (246,247,249); the
same pale hint is on `1440-light-branch.png`, and the directory field's on
`1440-light-repositories.png`. Two input styles disagree on one page, and
the gate cannot see it.

- `commitInputClass` (`web/src/features/branch/CommitForm.tsx:234`), so
  `MessageFields`' Subject hint "what the change does, in the imperative"
  (`:163`), the only hint of what the field wants.
- `prInputClass` (`web/src/features/review/ReviewPanel.tsx:396`), so
  `ProposalFields`' three placeholders (`:358`, `:367`, `:376`).
- `IssueLink`'s `inputClass` (`web/src/features/branch/IssueLink.tsx:11`),
  so the Issue field's "PROJ-7 or #42" (`:179`).
- `DirectoryPicker`'s input
  (`web/src/features/repositories/DirectoryPicker.tsx:40`), whose
  placeholder is the directory shown (`:36`).
- The settings `inputClass`
  (`web/src/features/settings/fieldsets/Field.tsx:22`) the same, though no
  Settings field sets a placeholder today.
- Three inputs already sit on `placeholder:text-muted-foreground`, 6.10:1
  in the light theme: the issue Filter (`IssueListControls`,
  `web/src/features/issues/IssueListControls.tsx:25`), the task Filter
  (`web/src/features/tasks/TaskListControls.tsx:68`) and the comment box
  (`web/src/features/issues/CommentComposer.tsx:163`).
- `scan`'s axe tag set (`web/e2e/a11y.spec.ts:55`) has no rule that
  samples `::placeholder`, and `web/src/tokens.test.ts` compares tokens
  only, so `yarn test:e2e` stays green.

**Instead.** Add `placeholder:text-muted-foreground` to the input classes
— best through the one input primitive UX-142 proposes — so every
placeholder is on the same token in both themes.

**Done when.** A Playwright check in `web/e2e/a11y.spec.ts` reads
`getComputedStyle(input, '::placeholder').color` for the Subject field in
both themes and asserts 4.5:1 against the page; or `web/src/tokens.test.ts`
grows a placeholder pair and a grep shows every input class uses it.

### UX-111 Light-theme text fields have no visible boundary: 1.3:1 on the page

Impact: medium · Effort: small

**Today.** Every text input, select and textarea is bordered by a 1 px
`--input` alone: some class strings are `bg-transparent`, the rest
`bg-background`, which on the page is the same color (UX-142 counts
them). The light `--input` measures 1.29:1 on `--background` and 1.39:1 on
`--card`, below the 3:1 non-text floor of WCAG 2.1 AA 1.4.11 the gates
claim. A light-theme user looking for where to type sees a faint outline
that all but disappears on a bright display: in `1440-light-settings.png`
the empty fields (User, Review status, Client secret, Refresh token,
Webhook URL, Default scope, Types, Subject limit, Issue trailer, Slug
limit, Task program) read as blank space under their labels. The
2026-09-24 audit sampled the interior of a field `TextField` draws
(`web/src/features/settings/fieldsets/Field.tsx:97`) at (246,247,249),
the page's own value, with (215,219,227) border rows. Labels and the
periwinkle focus ring still let a user find a field, which is why this is
medium and not high.

- `--input` is `#d7dbe3` in the light theme (`web/src/index.css:102`),
  1.29:1 on `--background` `#f6f7f9` (`:77`) and 1.39:1 on `--card`
  `#ffffff` (`:80`), and `--border` is the same value (`:101`), so the
  decorative rule and the field boundary share one too-faint value. The
  dark `--input` (`#272d39`, `:42`, on `#0f1115`) is about 1.37:1 too, so
  both themes share the gap; the light one is where the outline vanishes.
- The settings `inputClass` is `bg-transparent border-input`
  (`web/src/features/settings/fieldsets/Field.tsx:22`): the border is the
  field's only boundary. `commitInputClass`
  (`web/src/features/branch/CommitForm.tsx:234`), `prInputClass`
  (`web/src/features/review/ReviewPanel.tsx:396`), the task line
  (`web/src/features/tasks/TaskLineForm.tsx:71`) and the announcement's
  channel select (`web/src/features/messaging/MessagingPanel.tsx:298`)
  are the same.
- `web/src/tokens.test.ts` asserts `contrast` for the system hues against
  the page and a card (`:123`) and for the disabled pair, never for a
  boundary token; axe cannot measure non-text contrast, so nothing in
  `task check` or `yarn test:e2e` notices.

**Instead.** Darken `--input` in both themes until it holds 3:1 against
`--background` and `--card`, leaving `--border` for the decorative rules,
and add that pair to `web/src/tokens.test.ts` beside the hue checks.

**Done when.** `web/src/tokens.test.ts` asserts
`contrast(--input, --background) >= 3` and `contrast(--input, --card) >= 3`
in both themes and passes; the light Settings screenshot shows every empty
field with a visible outline.

### UX-112 A primary button's focus ring is the color of its fill

Impact: medium · Effort: small

**Today.** `--ring` equals `--primary` in both themes and no `ring-offset`
exists under `web/src`, so every periwinkle button removes the browser
outline and draws keyboard focus as a 2 px box-shadow in its own color;
beside each, an outline Cancel gets a visible periwinkle ring. Tabbing
from Cancel to Announce now, or from Cancel to Push in the push
confirmation, the visible ring disappears when it reaches the button that
sends: the button grows 2 px in its own color. It hits Commit staged
changes, Push, Announce now, Open a pull request, Start work, Comment and
Save changes — the buttons a keyboard user reaches every loop. No
screenshot shows it, since nothing in the captures has focus, and
`web/e2e/layout.spec.ts` checks that a focused control is in view, not
that its focus can be seen.

- `--ring` is `#8b93f8` (`web/src/index.css:43`), the value of `--primary`
  (`:26`); in the light theme `--ring` is `#4f56c9` (`:103`), the value of
  `--primary` (`:86`).
- The `primary` variant of `Button` (`web/src/lib/Button.tsx:10`) is
  `bg-primary` with `focus-visible:ring-ring` and `outline-none`, no
  offset, and fifteen buttons draw through it, among them `SaveControls`'
  submit (`web/src/features/settings/SettingsPanel.tsx:163`),
  `CommitForm`'s submit (`web/src/features/branch/CommitForm.tsx:93`),
  `PushConfirm`'s Push (`web/src/features/branch/BranchPanel.tsx:186`)
  beside a Cancel (`:183`) whose ring is visible, `OpenPullRequest`'s
  button (`web/src/features/review/ReviewPanel.tsx:221`),
  `StartWorkButton` (`web/src/features/issues/WorkStory.tsx:375`),
  the comment's send (`web/src/features/issues/CommentComposer.tsx:383`),
  and `AnnouncePreview`'s Announce now
  (`web/src/features/messaging/MessagingPanel.tsx:323`) beside a Cancel
  (`:320`) whose ring is visible.
- The calendar's picked day is drawn the same way: `bg-primary` with
  `focus-visible:ring-ring` (`DayCell`,
  `web/src/features/summary/MonthGrid.tsx:162`), so the day in focus,
  once picked, shows no ring.

**Instead.** Add `focus-visible:ring-offset-2
focus-visible:ring-offset-background` to the primary variant in
`web/src/lib/Button.tsx` and to the picked day (a page-colored gap between
fill and ring), so the ring reads on a periwinkle fill as it does on an
outline one.

**Done when.** A Playwright test focuses Save changes in both themes and
asserts its computed `box-shadow` carries a `--background`-colored offset
(or a ring color that contrasts 3:1 with `--primary`), and a screenshot
with focus on Announce now shows a ring distinct from the fill.

### UX-114 Three places set type outside the scale and face the system names

Impact: low · Effort: small

**Today.** `web/src/index.css` reserves the code face for "a branch, a
commit, a path, a command. Nothing else is set in it." (the comment over
`--font-mono`, `:174`) and names `sm` the body of a dense tool and `base`
what heads a part of a panel (the comment over `--text-*`, `:181`). Three
places set type against that; UX-142 has the headings and controls that
drift beside them.

- `WorkStory` renders each stage's detail in a plain `text-sm` sans span
  (`web/src/features/issues/WorkStory.tsx:318`), and the detail carries
  `branch.name` from `offHeadStages` (`:89`) and `onHeadStages` (`:109`),
  so the story's Branch stage shows "fix/PROJ-412-redact-tokens · 3
  ahead" in the sans muted foreground; `storyNote`'s "In progress on
  {branch}" (`:211`) is sans too. The Branch section's heading sets the
  same name in `font-mono` (`BranchSummary`,
  `web/src/features/branch/BranchPanel.tsx:62`); `1440-dark-issues.png`
  and `1440-light-branch.png` show the sans name and the mono heading one
  click apart.
- Each stage title is `font-medium` with no size (`WorkStory`,
  `web/src/features/issues/WorkStory.tsx:316`), so base, over a `text-sm`
  detail: "Branch", "Changes", "Pull request" and "Announce" read at the
  size of the h3 "Work story" above them (`IssueDetailPanel`,
  `web/src/features/issues/IssueDetailPanel.tsx:62`, `sectionHeading`
  `text-base font-semibold` at `:15`), differing only in weight, as
  `1440-dark-issues.png` shows.
- A queue row's title is `font-medium` inside a `<p>` with no size class
  (`RequestRow`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:302`),
  while issue summaries (`IssueRows`,
  `web/src/features/issues/IssuesPanel.tsx:282`), commits, changed files
  and CI checks are all `text-sm`; in `1024-dark-reviews.png` and
  `1024-dark-issues.png` the request titles are visibly larger than the
  issue summaries, though both are a row's headline.

**Instead.** Wrap the branch name in the stage detail and the note in a
`font-mono` span, leaving the separator and count in sans (UX-143's
separate elements make this one line); set the stage titles and the queue
titles `text-sm font-medium` as the other list rows are.

**Done when.** A `WorkStory` test finds the branch name inside an element
with the mono class (or a `<code>`); screenshots show the stage titles
smaller than the Work story heading and the queue titles at the issue
summaries' size.

### UX-115 The content measure is set four ways above lg

Impact: low · Effort: small

**Today.** A grep for `max-w-` under `web/src/features` finds three
measures on a section's content — `max-w-2xl` on Branch, Review, the
messaging section and Settings, `max-w-3xl` on the review queue and
`max-w-prose` on Repositories — and Issues, Tasks and Summary have none:
their lists are a fixed width and their detail runs to the window's edge.
So the same kind of content stops at different right edges, and a wide
window gives the lists none of its width while a detail or a timeline runs
1000 px. At 1440 px Repositories is narrow while Tasks and Summary run
full width: in `1440-light-repositories.png` the Worktrees rule ends near
765 px, in `1440-light-branch.png` the commit form near 775 px, in
`1024-dark-reviews.png` the queue near 871 px, while in
`1440-dark-tasks.png` the `task add` field and the Annotate and Modify
fields run to about 1415 px and in `1440-dark-summary.png` the timeline
runs past 1090 px. The settled rule covers only the stack below `lg`; the
width above it is open.

- `Queue` is `max-w-3xl`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:111`) where Branch,
  Review, Messaging and the Settings form are `max-w-2xl`
  (`web/src/features/branch/BranchPanel.tsx:32`,
  `web/src/features/review/ReviewPanel.tsx:56`,
  `web/src/features/messaging/MessagingPanel.tsx:32`,
  `web/src/features/settings/SettingsPanel.tsx:139`, and Settings' two
  areas below it, `web/src/features/settings/people/PeopleAndGroups.tsx:14`
  and `web/src/features/settings/people/LocalData.tsx:18`), and `Read` is
  `max-w-prose` (`web/src/features/repositories/RepositoriesPanel.tsx:78`),
  65 characters of the body face rather than a width; no comment
  justifies either.
- `ListAndDetail` holds the issue list at `lg:w-80 lg:shrink-0` with no
  larger breakpoint (`web/src/features/issues/IssuesPanel.tsx:157`), so it
  stays 320 px at 1440 and, in `1440-dark-issues.png`, six of the seven
  mock summaries wrap to two lines or three; the Tasks list is the same
  (`web/src/features/tasks/TasksPanel.tsx:334`).
- The issue detail column is `min-w-0 flex-1` with no `max-w`
  (`web/src/features/issues/IssuesPanel.tsx:175`), and `Description`'s
  `<p>` has no measure of its own
  (`web/src/features/issues/IssueDetailPanel.tsx:190`); a real Jira
  description at 1440 px would be set at roughly 150 characters a line
  where the other sections stop at 672 px. The task detail
  (`web/src/features/tasks/TasksPanel.tsx:337`) and the Summary grid's
  right column (`web/src/features/summary/SummaryPanel.tsx:35`) are the
  same.

**Instead.** Name one content measure (`max-w-2xl`) and use it in every
section and on the issue's and task's detail; let the list columns grow
with the window above `xl` (say `xl:w-96`) while the detail keeps its
measure.

**Done when.** A grep for `max-w-` in `web/src/features` returns one value
on section content; a 1440 px screenshot shows the mock's seven summaries
on one line each, the Tasks and Summary content ending where Branch's
does, and one with a 600-character description shows its lines no wider
than the Branch section's form.

### UX-116 Check out narrows its row, leaving the issue list ragged

Impact: low · Effort: small

**Today.** A row's Check out button is a flex sibling of the row button,
added only when a non-HEAD branch exists, so rows with one are narrower
than rows without and the list's right edge steps in and out. In
`1440-dark-issues.png` the selected PROJ-412 box runs the list's full
320 px while PROJ-418 and PROJ-408 wrap their summaries in a narrower box
ending about 80 px short to fit Check out — PROJ-418's to three lines;
`640-dark-issues.png` shows the same step at the narrow width. `IssueRows`'
`<li>` is `flex flex-wrap` (`web/src/features/issues/IssuesPanel.tsx:253`),
the row button `flex-1` (`:270`), and `RowCheckout` is rendered beside it
only when `newest && !onHead` (`:284`), `shrink-0` so it takes its full
width from the row (`:498`).

**Instead.** Reserve the button's column on every row (an invisible
placeholder of the same width when the row offers no check-out), or put
the button inside the row's bottom line.

**Done when.** A screenshot shows every row button in the list sharing one
right edge.

### UX-117 A clean working tree leaves its heading over an empty form

Impact: low · Effort: small

**Today.** With no changes, the Working tree heading is followed directly
by the commit form; the only words about the empty tree are "Clean —
nothing to commit." beside the disabled button at the form's foot, where
`Commits` says "No commits yet on this branch." under its heading
(`web/src/features/branch/BranchPanel.tsx:90`). In a 1024 px dark
screenshot of the production build's empty Branch section, taken in the
2026-09-24 audit, the eye lands on four empty fields under "Working tree"
and reads why only after scrolling past them. `WorkingTree` renders
nothing between the heading and the form when `changes` is empty
(`web/src/features/branch/WorkingTree.tsx:26`); the explanation is
`commitBlocker`'s line (`:61`), which `CommitForm` shows beside the
disabled submit, below the fields
(`web/src/features/branch/CommitForm.tsx:100`).

**Instead.** A muted line under the heading, "Clean — nothing to commit.",
with the form's foot line kept for the nothing-staged case.

**Done when.** A `WorkingTree` test with no changes finds "Clean — nothing
to commit." before the form in DOM order.

### UX-118 Copy URL confirms above the queue, not beside its row

Impact: low · Effort: small

**Today.** After Copy URL on a queue row, the only visible confirmation is
the panel's `OutcomeLine` above the list: `CopyURL` hands its done message
to the panel's teller
(`web/src/features/reviewqueue/ReviewQueuePanel.tsx:384`), which `Queue`
renders once (`:152`), under the summary, the Sort select and the filter
chips, out of the eye's path from the button — two rows of chips in
`1024-dark-reviews.png` — while its refusal renders in the row (`:398`).
`RowCheckout` uses the same success-above pattern
(`web/src/features/issues/IssuesPanel.tsx:486`, line at `:105`), but a
check-out changes the row on the next snapshot; a copy changes nothing
near the button. The working tree's per-row `OutcomeLine` (`ChangeRow`,
`web/src/features/branch/WorkingTree.tsx:113`) shows the nearer pattern.

**Instead.** Give each row its own `OutcomeLine` under its controls, as
`ChangeRow` does.

**Done when.** A `ReviewQueuePanel` test finds the copied-URL status inside
the row's listitem.

### UX-119 A credential cannot be removed from Settings

Impact: low · Effort: small

**Today.** `keepSecret` treats an emptied secret field as "keep the stored
value" (`internal/webserver/config.go:385`) and `preserveSecrets` applies
it to all six secrets and every Jira header value
(`internal/webserver/config.go:357`), so Settings has no way to clear
`jira.token`, `messaging.client_secret`, `messaging.refresh_token`,
`messaging.webhook_url` or `forge.token`: clearing one in the form and
saving keeps it, and moving from a user token to a webhook leaves the old
client secret and refresh token in the file, to be removed by hand. The
keep is documented — `updateConfig` says an empty or masked secret field
keeps the stored secret (`api/openapi.yaml:525`), and the web page says
under "Settings" that a credential is "kept as it is unless you type a new
one" (`docs/content/docs/web.md:349`) — but no clear is offered anywhere.
UX-87 covers sections the form cannot show, not clearing a secret.

**Instead.** Accept an explicit clear — a `null` for the secret fields in
the contract, or a per-field remove control — that writes an empty value.

**Done when.** A test sends `jira.token` as `null` and the saved file holds
no token.

### UX-120 Settings draws a failed read as a resting state

Impact: low · Effort: small

**Today.** When the configuration cannot be read, `ConfigArea` puts the
reason inside `<EmptyState>`
(`web/src/features/settings/SettingsPanel.tsx:53`), as muted text with no
`role="alert"` (`:55`); `EmptyState` is the dashed, centered,
`text-muted-foreground` box a panel shows when it has nothing yet
(`web/src/shell/EmptyState.tsx:6`). The review queue (`Queue`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:120`) and the issue
detail (`IssueUnread`, `web/src/features/issues/IssueDetailPanel.tsx:108`)
show the same kind of failure as a `role="alert"` line in
`text-destructive`. Red is the failure color and nothing else, and here a
failure is not red: a 1024 px dark screenshot of the production build's
empty Settings section, taken in the 2026-09-24 audit, showed gray "The
configuration could not be loaded." centered in the dashed box, the same
drawing as "Connecting to workflow…" (`web/src/shell/SectionPanel.tsx:42`),
while its empty Reviews twin showed the queue's failure red and
left-aligned; a screen reader hears nothing, since no live region carries
it. A Retry is right there and a configuration that fails to read is
outside the daily loop, which is why this is low. The rest of the section
and four others say a failure without a role too; that is UX-140's.

**Instead.** Say the failure as the other read failures do: a
`role="alert"` paragraph in `text-destructive` with Retry beside it,
outside the `EmptyState`.

**Done when.** A `SettingsPanel` test with a failing configuration read
finds the reason by role alert; a screenshot shows it in the failure color.

### UX-122 Web copy settles plurals, case, periods and state words site by site

Impact: low · Effort: small

**Today.** The same act, state or moment reads differently depending on
where the eye lands — list versus story, one `dl` row versus the next,
header versus content, browser versus terminal.

- The web has no plural helper, where the terminal's `plural` counts in
  words (`internal/tui/render.go:439`): `changesDetail` says "{n} file(s)
  to commit" (`web/src/features/issues/WorkStory.tsx:172`; "3 file(s) to
  commit" in `1440-dark-issues.png`), `PushConfirm` "Push {commits}
  commit(s) to the remote?" (`web/src/features/branch/BranchPanel.tsx:182`;
  the count itself is UX-104), and `filterOutcome` "{shown} of {loaded}
  loaded issues match." (`web/src/features/issues/IssuesPanel.tsx:430`),
  so "1 … match." disagrees in number; meanwhile three sites count
  correctly inline, each its own way (`queueSummary`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:181`; the task count,
  `web/src/features/tasks/TasksPanel.tsx:274`; the comment's characters,
  `web/src/features/issues/CommentComposer.tsx:380`).
- `loadOutcome` ends "4 of 5 loaded" without a period
  (`web/src/features/issues/IssuesPanel.tsx:412`) and "All 5 loaded." with
  one (`:415`).
- Eight placeholders split four lowercase to three sentence case and one
  path: "what the change does, in the imperative" (`MessageFields`,
  `web/src/features/branch/CommitForm.tsx:163`), "comma-separated
  usernames or org/team", "comma-separated usernames" and
  "comma-separated labels" (`ProposalFields`,
  `web/src/features/review/ReviewPanel.tsx:358`, `:367`, `:376`), against
  "Key or summary" (`web/src/features/issues/IssueListControls.tsx:21`),
  "Text, +tag, issue or #id"
  (`web/src/features/tasks/TaskListControls.tsx:64`) and "Write a
  comment…" (`web/src/features/issues/CommentComposer.tsx:156`).
- The one check-out write is "Switching…" in the list row (`RowCheckout`,
  `web/src/features/issues/IssuesPanel.tsx:500`), outside its
  `aria-label` "Check out KEY" (`:493`), so a keyboard user's visible word
  is not in the button's accessible name, and "Checking out…" in the
  story (`CheckoutButton`, `web/src/features/issues/WorkStory.tsx:350`).
- The never-done Announce stage is "Not announced" off HEAD
  (`notStartedStages`, `web/src/features/issues/WorkStory.tsx:78`;
  `offHeadStages`, `:92`) and an imperative on it: `announceDetail` reads
  "Announce to {channel}" (`:166`), on a button that opens the messaging
  section — "Announce to #dev-workflow" in `1440-dark-issues.png`.
- A missing base is "—" (`BranchSummary`,
  `web/src/features/branch/BranchPanel.tsx:68`) beside a missing upstream
  "none" (`:70`); Repositories says "None" for a missing origin
  (`web/src/features/repositories/WorkingIn.tsx:43`).
- `SectionPanel` says "Connecting to workflow…" while the snapshot is null
  (`web/src/shell/SectionPanel.tsx:42`), never reading the stream's
  status, while `useEventStream` sets `reconnecting` on every
  `EventSource` error, one before any open included
  (`web/src/api/snapshot.ts:117`); so the header says "Reconnecting" over
  every section's "Connecting to workflow…" at once, where
  `docs/content/docs/web.md:59`, under "The page", defines Reconnecting as
  "the connection dropped" and Connecting (`:57`) as no first update yet.
- The empty review queue is "Nothing is waiting on your review." on the
  web (`queueSummary`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:175`; `Requests`,
  `:241`), "No pull requests are waiting on your review." in the
  terminal's detail (`reviewQueueDetail`,
  `internal/tui/reviewqueue.go:172`) and on the command line
  (`renderReviews`, `internal/cli/reviews.go:105`), and "none waiting on
  you" in the terminal's rail (`reviewQueueRail`,
  `internal/tui/reviewqueue.go:156`).
- Past a month `waited` prints `toLocaleDateString()`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:362`) — "8/15/2026"
  or "15/08/2026" by locale — under a comment promising "the terminal's
  words" (`:339`), where the terminal's `age` prints `time.DateOnly`
  (`internal/tui/detail.go:480`). The web's other dates are UX-144's.
- The configuration read is "Loading the configuration…" (`ConfigArea`,
  `web/src/features/settings/SettingsPanel.tsx:48`) where every other
  read says "Reading" — "Reading the code owners…", "Reading your
  tasks…", "Reading {KEY}…" (UX-141 lists them) — and its failure is
  "could not be loaded" (`:55`) in one place and "could not be read
  again" (`ConfigForm`, `:124`) in the same file, while the three reads
  below it say "could not be read"
  (`web/src/features/settings/people/PeopleTable.tsx:39`).

**Instead.** A `plural(count, noun)` in `web/src/lib/utils.ts` for the
three `(s)` sites and the three inline ones; one form for both load-count
states; one case for every placeholder; "Checking out…" on both
check-out buttons (a substring of the row's `aria-label`); one Announce
phrasing beginning "Not announced" that names the channel; one
placeholder word for absence in every `dl` row, in the muted foreground;
`SectionPanel` wording its wait from the same status table as
`StreamStatus`, or the stream staying "Connecting" until its first open;
the terminal's empty-queue sentence and the ISO date past a month;
"Reading the configuration…" and "could not be read" in Settings.

**Done when.** `WorkStory`, `BranchPanel` and `IssueListControls` tests
read "1 file to commit", "3 files to commit", "Push 1 commit" and "1 of 2
loaded issues matches."; `grep -rn "(s)" web/src --include='*.tsx'` finds
nothing outside generated code; every `placeholder=` under `web/src` is in
one case; the paging test asserts one load-count form; a test pressing
each check-out button with a pending promise finds a button named
/checking out/i in both list and story and no "Switching…"; a grep for
"Loading the" in `web/src/features` returns nothing; the three-case
Announce test asserts a phrase beginning "Not announced"; a test with base
`''` and upstream `''` finds the same placeholder in both `dd` cells; a
`SectionPanel` test with status `reconnecting` and no snapshot finds header
and content agree; `ReviewQueuePanel` tests assert the terminal's
empty-queue sentence and the ISO date past a month, with the comment and
code agreeing.

### UX-123 Forge links, story stages and two controls tell less than their siblings

Impact: low · Effort: small

**Today.** jsx-a11y strict and axe A/AA pass, since each control has some
name; what each says at rest, or in its name, is less than its neighbors
say.

- In the Review section `PullRequestSummary`'s title link opens the forge
  in a new tab (`target="_blank"`,
  `web/src/features/review/ReviewPanel.tsx:102`) with `{pull.title}` as
  its whole accessible name (`:106`) and only `underline-offset-4
  hover:underline` for a class (`:104`): no `text-primary`, no icon, no
  new-tab note, no focus-visible ring. The Issue row's link (`IssueRow`,
  `:411`) and each CI check's name (`CheckRow`, `:437`) are the same, and
  a check without a URL is a bare `<span>` (`:435`) that looks identical.
  The Summary's activity links (`ActivityLine`,
  `web/src/features/summary/ActivityList.tsx:101`) are underlined and
  ringed but carry no new-tab note either. On the Review screenshots
  (`1440-dark-review.png`) the heading "#128 fix: redact tokens…" and the
  check rows look like static text; a focused forge link falls back to
  the browser's default outline; a screen-reader user activating any of
  them is moved to a new tab unwarned. The page's other three outbound
  links carry all of it: the queue's Open (`RequestRow`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:315`), Open in Jira
  (`IssuePeople`, `web/src/features/issues/IssueDetailPanel.tsx:166`) and
  the task's issue link (`web/src/features/tasks/TaskDetail.tsx:126`),
  each with the `ExternalLink` icon and the sr-only "(opens in a new
  tab)".
- Each work-story stage is a button that calls `setSection` (`WorkStory`,
  `web/src/features/issues/WorkStory.tsx:309`), yet it is styled only with
  `hover:bg-accent` and a focus ring (`:314`) and its sr-only span carries
  the state alone (`:317`): nothing at rest or in its name says it
  navigates, though `docs/content/docs/web.md:96` promises under "Issues"
  each "step opening the section it belongs to", so the story reads as a
  plain timeline in `1440-dark-issues.png`.
- The Settings `<form>` has `onSubmit` and a `className` only
  (`ConfigForm`, `web/src/features/settings/SettingsPanel.tsx:135`), no
  `aria-label` or `aria-labelledby`, so it is not a form landmark while
  the commit and pull request forms are, and
  `getByRole('form', { name: /settings/i })` cannot resolve the site's
  largest form.
- `ThemeToggle`, icon-only at every width, carries its name in
  `aria-label` alone (`web/src/shell/ThemeToggle.tsx:24`) with no `title`,
  where `NavRail`'s buttons show theirs on hover (`title={name}`,
  `web/src/shell/NavRail.tsx:30`); a mouse resting on the toggle shows
  nothing.

**Instead.** Draw the title, issue and check-name links as the queue draws
Open — `text-primary`, the `ExternalLink` icon, the sr-only new-tab note
and the focus-visible ring — so a check with a URL differs visibly from
one without, and give the activity links the note; mark each stage as a
control at rest inside the system (the title in `text-primary`, or a
trailing chevron in the muted foreground) and give it an sr-only suffix or
`aria-describedby` naming the section it opens; `aria-labelledby` on the
Settings form pointing at the section's h1; a `title` on the toggle equal
to the choice it announces ("Theme: System").

**Done when.** `web/src/features/review/ReviewPanel.test.tsx` finds the
title link, the issue link and each check with a URL by role link with a
name ending "(opens in a new tab)" and `target="_blank"`, and a check
without a URL as text; a `WorkStory` test finds each stage by role button
with a name or description matching /opens (branch|review|messaging)/i
and a screenshot of the Issues section shows a rest-state mark on each
stage; `screen.getByRole('form', { name: /settings/i })` resolves in
`web/src/features/settings/SettingsPanel.test.tsx`; a `ThemeToggle` test
asserts the button's `title` names the current choice and changes with
it.

### UX-124 Two Settings selects draw blank for the value in effect

Impact: low · Effort: small

**Today.** `pull_request.title_source` and `messaging.kind` may be the
empty string, and the server reads them as `commit` and Slack, and the
spec allows it. But the Title source and Service selects offer no option
for `""`, so a configuration holding it shows an empty control in either
theme, and once a value is picked there is no way back to the default.
Every Settings screenshot (`1440-light-settings.png` among them) shows
Title source empty under "Pull request", the only control on the form
with no visible value. A blank select saves back `""` harmlessly and
Settings is opened rarely, which is why this is low.

- `PullRequestFieldset` offers `commit` and `issue` only
  (`web/src/features/settings/fieldsets/PullRequestAndStoreFieldsets.tsx:15`),
  and `MessagingFieldset`'s choices begin at `slack`
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:15`);
  `SelectField` registers the `<select>` with the value as-is
  (`web/src/features/settings/fieldsets/Field.tsx:126`), so `""` matches
  no option and shows blank. `ForgeFieldset` handles the same case with an
  explicit `['', 'Auto-detect']`
  (`web/src/features/settings/fieldsets/ForgeFieldset.tsx:13`).
- Neither `Default` (`internal/config/config.go:195`) nor `Template`
  (`:207`), which `config init` writes, sets a `PullRequest`, so
  `title_source` is `""` in every file workflow writes. `Template` sets
  `Kind: KindSlack` (`:217`), but `collectMessaging` returns
  `config.Messaging{}` when the webhook prompt is left blank for the user
  token (`internal/cli/config_cmd.go:428`), so that path writes kind `""`
  and the Service select draws empty while the messaging section's rail
  label and heading say "Slack" — two surfaces disagreeing about one file.
- `PullRequest.TitleSource` documents `commit` as the default
  (`internal/config/pullrequest.go:19`) and its tag is
  `json:"title_source"` without `omitempty` (`:21`),
  `validatePullRequest` accepts `""` (`:27`), and `write`'s
  `MarshalIndent` of the whole struct writes the empty value
  (`internal/config/save.go:161`).
- `MessagingConfig`'s `kind` `enum` is `["", slack, teams, discord,
  webhook]` (`api/openapi.yaml:3371`), so the spec allows what the select
  cannot show; the mock the screenshots show has `title_source: ''`
  (`web/src/dev/mockConfig.ts:57`).

**Instead.** A first choice `['', "The branch's oldest commit
(default)"]` and `['', 'Slack (default)']` as `ForgeFieldset` does for its
empty kind — or normalize `""` onto the default when seeding the form and
let the save write it.

**Done when.** A `SettingsPanel` test seeding `title_source: ''` finds the
Title source combobox with a selected option naming the oldest-commit
default, and one seeding `messaging.kind: ''` finds the Service combobox
with a selected option named for Slack.

### UX-140 Failures in Summary, Repositories and three Settings reads are not announced

Impact: medium · Effort: small

**Today.** Two sections say a failure as a plain paragraph in the body
color, and three Settings reads say it in red with no role, so a screen
reader hears none of them and, in the first two, nothing on screen sets a
failure apart from a fact. The issue detail (`IssueUnread`,
`web/src/features/issues/IssueDetailPanel.tsx:108`), the review queue
(`web/src/features/reviewqueue/ReviewQueuePanel.tsx:120`) and every
refused write say theirs as a `role="alert"` line in `text-destructive`;
Settings' own failed configuration read is UX-120's.

- In the body color, no role: Summary's `Refused`, "What you did could not
  be read." (`web/src/features/summary/SummaryPanel.tsx:79`), its failed
  copy (`:162`) and each source that could not be read (`:167`), a line
  `1440-dark-summary.png` draws as plainly as "Jira had more than this
  shows."; Repositories' failed read
  (`web/src/features/repositories/RepositoriesPanel.tsx:29`), its failed
  favorite (`:97`) and its refused switch (`ConfirmSwitch`, `:172`); the
  directory picker's "That directory could not be listed."
  (`web/src/features/repositories/DirectoryPicker.tsx:48`); and "The
  worktrees could not be read" (`WorktreesList`,
  `web/src/features/repositories/WorktreesList.tsx:24`).
- In red, no role: "The code owners could not be read."
  (`PeopleTable`, `web/src/features/settings/people/PeopleTable.tsx:38`),
  "The local data could not be read."
  (`web/src/features/settings/people/LocalData.tsx:42`) and "The
  repository’s groups could not be read."
  (`web/src/features/settings/people/RepoGroups.tsx:22`), each above a
  Retry that disables itself under focus (UX-106).

A truncated source ("had more than this shows") is a note, not a failure,
and can stay plain.

**Instead.** Say each as the other failures are said: a `role="alert"`
paragraph in `text-destructive`, keyed so a second refusal is heard
again as `IssueUnread`'s is; the truncation line a `role="note"` in the
muted foreground.

**Done when.** Unit or e2e tests make each of the eleven fail and find its
reason by role alert — `SummaryPanel`, `RepositoriesPanel`,
`DirectoryPicker` and `WorktreesList` with a refused read or write, and
`PeopleTable`, `LocalData` and `RepoGroups` with a refused read — and a
grep for `text-sm text-foreground` before `.error` in `web/src/features`
finds nothing.

### UX-141 Loading is drawn four ways

Impact: low · Effort: small

**Today.** A read in flight is said in four drawings and two words,
depending on the section, so the same wait looks like an empty section in
one place and a status line in the next, and only two of them reach a
screen reader as they land.

- The dashed, centered `EmptyState` (`web/src/shell/EmptyState.tsx:6`),
  no role: "Reading your review queue…"
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:51`), "Reading your
  tasks…" (`web/src/features/tasks/TasksPanel.tsx:32`), "Reading the
  {view} view…" (`web/src/features/issues/IssuesPanel.tsx:108`) and
  "Connecting to workflow…" (`web/src/shell/SectionPanel.tsx:42`) — the
  box the same component draws for a service not configured. Its own
  comment (`:3`–`:5`) gives it to "nothing yet — connecting, or a service
  the workspace has not configured", which fits the last of these and
  none of the three reads.
- A `role="status"` line in the muted foreground: "Reading what you
  did…" (`Reading`, `web/src/features/summary/SummaryPanel.tsx:97`) and
  "Reading where the server works…"
  (`web/src/features/repositories/RepositoriesPanel.tsx:42`).
- A plain muted line with no role: "Reading {KEY}…"
  (`web/src/features/issues/IssueDetailPanel.tsx:44`), "Reading the code
  owners…" (`web/src/features/settings/people/PeopleTable.tsx:32`),
  "Reading the local data…"
  (`web/src/features/settings/people/LocalData.tsx:36`) and "Reading the
  repository's groups…"
  (`web/src/features/settings/people/RepoGroups.tsx:16`).
- "Loading the configuration…" in an `EmptyState`
  (`web/src/features/settings/SettingsPanel.tsx:47`–`:48`), the one read
  that says "Loading" — so Settings shows a dashed "Loading" box first,
  then the form with three plain "Reading" lines in it. Its word is in
  UX-122 too.

**Instead.** One form for a read in flight: a `Reading …` component in
`web/src/shell` that draws a muted `role="status"` line, as Summary's
`Reading` does, used for every first read and every re-read; keep
`EmptyState` and its comment for "nothing here" and "not configured"
alone, and say "Reading the configuration…" in Settings.
`web/src/shell` is at its file budget (12 of 12,
`scripts/package-size-budgets.txt`), so the new component comes with a
budget entry and its reason, or lives in an existing shell file.

**Done when.** A test per section finds its wait by role status with a
name beginning "Reading"; a grep finds no `EmptyState` whose text ends in
"…" and no "Loading the" in `web/src/features`.

### UX-142 Inputs, secondary buttons and headings outside the system

Impact: low · Effort: medium

**Today.** The type, space and corner scales are tokens, but the controls
and headings built on them are class strings written site by site, so
they drift: a field's padding, background and placeholder change from one
form to the next, a secondary act comes in three sizes, and Repositories
heads its parts a step above every other section. Type that breaks the
code-face and size rules is UX-114's; this is the controls' and
headings' half.

- Text fields are drawn by about a dozen class strings in three paddings
  and three backgrounds: `bg-transparent px-3 py-2` (the settings
  `inputClass`, `web/src/features/settings/fieldsets/Field.tsx:22`;
  `commitInputClass`, `web/src/features/branch/CommitForm.tsx:234`;
  `prInputClass`, `web/src/features/review/ReviewPanel.tsx:396`;
  `IssueLink`'s, `web/src/features/branch/IssueLink.tsx:11`),
  `bg-transparent px-3 py-1.5` (the task line,
  `web/src/features/tasks/TaskLineForm.tsx:71`), `bg-background px-2
  py-1` (the issue and task filters and view and sort selects,
  `web/src/features/issues/IssueListControls.tsx:25`, `:62`,
  `web/src/features/tasks/TaskListControls.tsx:49`, `:68`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:204`; the calendar's
  selects, `web/src/features/summary/PeriodPicker.tsx:32`; the directory,
  `web/src/features/repositories/DirectoryPicker.tsx:40`), `bg-transparent
  px-2 py-1` (the people, tag and channel selects,
  `web/src/features/settings/people/PeopleTable.tsx:23`,
  `web/src/features/messaging/TagPicker.tsx:19`,
  `web/src/features/messaging/MessagingPanel.tsx:298`, which carry no
  focus ring of their own and fall back to the browser's outline) and
  the comment box on `bg-card`
  (`web/src/features/issues/CommentComposer.tsx:69`). Only the two
  filters and the comment box set a placeholder color (UX-110).
- The Tasks Filter is `w-48`
  (`web/src/features/tasks/TaskListControls.tsx:68`) for a placeholder
  longer than that, so `1440-dark-tasks.png` shows it clipped to "Text,
  +tag, issue or #ic" with the row's width to spare.
- A secondary act comes in three styles: the `Button` component's
  `secondary` (`rounded-md px-3 py-1.5 text-sm`,
  `web/src/lib/Button.tsx:12`); `stagingButtonClass` and `RowCheckout`'s
  copy of it (`rounded-sm px-2 py-1 text-xs`,
  `web/src/features/branch/WorkingTree.tsx:157`,
  `web/src/features/issues/IssuesPanel.tsx:498`); and Load more, a raw
  `<button>` at `rounded-sm px-2 py-1` in the inherited `text-sm`, with
  no held style for its `aria-disabled` (`MoreIssues`, `:389`). In
  `1440-light-branch.png` Stage all and Stage sit visibly smaller than
  Push branch above them; in `1024-dark-reviews.png` the queue's Copy URL
  is the large one and the list's Check out the small one.
- The Branch section's two acts stack: `PushButton`'s wrapper is a column
  (`web/src/features/branch/BranchPanel.tsx:128`) and `IssueLink`'s
  button another `self-start` row (`web/src/features/branch/IssueLink.tsx:55`),
  so `1440-light-branch.png` shows Push branch and Link an issue on two
  rows, where every other section puts its acts in one row
  (`StoryAction`, `web/src/features/issues/WorkStory.tsx:240`).
- Repositories heads its parts at `text-lg font-semibold` — Working in,
  Worktrees, Favorites and Open another directory
  (`web/src/features/repositories/WorkingIn.tsx:9`,
  `web/src/features/repositories/WorktreesList.tsx:20`,
  `web/src/features/repositories/FavoritesList.tsx:24`,
  `web/src/features/repositories/DirectoryPicker.tsx:18`) — where
  `index.css` says `base` heads a part of a panel and `lg` titles the thing
  shown (`web/src/index.css:181`), as every other section's parts are
  `text-base font-semibold`; and it sets the path it works in at
  `text-2xl` (`web/src/features/repositories/WorkingIn.tsx:12`), the size
  that titles a section (the h1, `web/src/shell/AppShell.tsx:108`).
  `1440-light-repositories.png` shows "~/src/api/cmd" a step under the
  section's own title and the four part headings a step over Settings'.
- Summary's period heading adds `font-semibold` to `text-lg`
  (`web/src/features/summary/SummaryPanel.tsx:113`), overriding the
  weight the scale gives `lg` (`--text-lg--font-weight: 500`,
  `web/src/index.css:193`) that the issue, task and pull request headings
  keep (`web/src/features/issues/IssueDetailPanel.tsx:141`,
  `web/src/features/tasks/TaskDetail.tsx:33`,
  `web/src/features/review/ReviewPanel.tsx:95`).
- The corner scale names `md` a control and `lg` a container of controls
  (`web/src/index.css`, the comment over `--radius-*`), and the commit,
  pull request and announcement forms are `rounded-lg`
  (`web/src/features/branch/CommitForm.tsx:84`,
  `web/src/features/review/ReviewPanel.tsx:296`); but the switch
  confirmation (`ConfirmSwitch`,
  `web/src/features/repositories/RepositoriesPanel.tsx:161`) and the
  worktree offer (`WorktreeMadeOffer`,
  `web/src/features/issues/StartInWorktree.tsx:80`), each a box of
  controls, are `rounded-md`.

**Instead.** An `Input`, `Select` and `TextArea` in `web/src/lib` carrying
one padding, one background, the placeholder token and the focus ring,
with a `size` for the dense filter row; one `Button` with a `size` (`sm`
for a row's inline act) that Load more, Stage and Check out draw
through; the Branch section's Push branch and Link an issue in one action
row; Repositories' part headings at `text-base font-semibold` and its path
at `text-lg`; Summary's heading at the scale's own weight; the two boxes
`rounded-lg`; the Tasks Filter wide enough for its placeholder (or a
shorter one).

**Done when.** A grep for `border-input` under `web/src/features` finds no
class string (every field and button draws through `web/src/lib`); a grep
for `text-lg font-semibold` and for `<button` under `web/src/features`
finds neither outside a documented exception; `1440-light-branch.png`
shows Push branch and Link an issue on one row; `1440-dark-tasks.png`
shows the Filter's whole placeholder.

### UX-143 Meta joined with middle dots, and a task line in monospace

Impact: low · Effort: small

**Today.** A row's facts are built into one string joined by " · ", so
none of them can be read, styled or marked up on its own: a branch name in
the joined string cannot take the code face (UX-114), a date cannot be a
`<time>`, and a screen reader hears "middle dot" or nothing between facts
depending on the voice. The visual system keeps the middle dot as the
separator; the defect is the joined string, not the dot.

- The review queue's "acme/workflow · by sam.ortiz · 9d ago · Draft"
  (`whereAndWho`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:335`,
  joined again at `:305` and `:307`), as `1024-dark-reviews.png` shows —
  the one site that already wraps its age in a `<time>` (`:306`).
- A task row's tail, "PROJ-408 · due in 1d 23h · urgency 9.1"
  (`web/src/features/tasks/TaskList.tsx:209`), in `1440-dark-tasks.png`;
  the task detail's Due, "{date} · {due words}"
  (`web/src/features/tasks/TaskDetail.tsx:85`), and Issue, "{key} ·
  {summary}" (`:92`); an issue's linked task, "{state} · {due words}"
  (`linkedNote`, `web/src/features/tasks/IssueTasks.tsx:150`).
- A GitLab check's "{stage} · {name}" (`CheckRow`,
  `web/src/features/review/ReviewPanel.tsx:428`), which is also the
  check's whole link name, and the CI heading's "· {done} of {total}
  done" (`:122`).
- The issue's "Bug · High priority" (`IssueHeading`,
  `web/src/features/issues/IssueDetailPanel.tsx:147`).
- The story's "fix/PROJ-412-redact-tokens · 3 ahead"
  (`onHeadStages`, `web/src/features/issues/WorkStory.tsx:109`) and
  "#128 · CI running" (`reviewDetail`, `:189`), in `1440-dark-issues.png`.

Beside them, the `task add` and per-task line inputs are set `font-mono`
(`TaskLineForm`, `web/src/features/tasks/TaskLineForm.tsx:71`), though
what is typed there is mostly prose — a description, an annotation — and
`web/src/index.css` keeps the code face for what a reader would type into
a terminal as an identifier (the comment over `--font-mono`, `:174`); the
command beside it (`:61`) is rightly mono. In `1440-dark-tasks.png` an
annotation shows in sans in the list above and would be typed in mono in
the field below.

**Instead.** Draw each row's facts as separate elements — a small `Meta`
in `web/src/lib` taking a list of nodes, spaced with `gap-item` and with
the dot drawn between them as an `aria-hidden` separator, or a `dl` where
the facts have names — so the branch can be `<code>`, the date a
`<time>`, and each fact styled alone; set the task line in the body face.

**Done when.** A grep for `' · '` and `join(' · ')` under `web/src/features`
finds no string built from facts; a `TaskList` test finds the issue key
and the urgency as separate elements; `grep -n font-mono
web/src/features/tasks/TaskLineForm.tsx` finds only the command label.

### UX-144 Three ways to write a date

Impact: low · Effort: small

**Today.** The web writes a date in at least three styles, two of them in
whatever locale the browser reports and one pinned to US English, so the
same day reads differently a section apart; and a one-day Summary heads
that day three times.

- A comment older than a week is a medium date, "Sep 18, 2026"
  (`sinceWritten`, `web/src/features/issues/IssueDetailPanel.tsx:344`),
  in the browser's locale, with a full date and time on hover (`:310`);
  `640-dark-issues.png` shows it.
- A task annotation and a task's due date are the browser's default short
  date, "10/2/2026" (`Annotations`,
  `web/src/features/tasks/TaskDetail.tsx:166`; `Due`, `:84`), as
  `1440-dark-tasks.png` shows — and so is the review queue's date past a
  month (`waited`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:362`),
  which UX-122 asks to follow the terminal's ISO date.
- The Summary heads its period with `spokenDate`, "Tuesday, September 15,
  2026" (`web/src/features/summary/civilDate.ts:151`, pinned to `en-US`,
  at `web/src/features/summary/SummaryPanel.tsx:113`), then
  `ActivityList` heads the month "September 2026" from the server's
  English month name (`MonthSection`,
  `web/src/features/summary/ActivityList.tsx:45`; `yearsDTO`,
  `internal/webserver/activity.go:146`) and the day "Tuesday 15"
  (`DaySection`, `:59`). For the one day `1440-dark-summary.png` shows,
  that is the same day said three ways in three headings, one above the
  next; and the copy's outcome names it a fourth way, "Copied the summary
  of 2026-09-15." (`written`, `web/src/features/summary/SummaryPanel.tsx:137`).

**Instead.** One date formatter in `web/src/lib` — one style for a date in
running text (a short, unambiguous form such as "Sep 18, 2026", or the
terminal's ISO date if the surfaces should agree) and one for a heading,
in one locale decision made once — used by the comments, the tasks, the
queue and the Summary; a one-day Summary heads the day once, dropping the
month and day headings when the period is a single day, and its copy
outcome uses the running-text style.

**Done when.** A grep for `toLocaleDateString` and `toLocaleString` under
`web/src/features` finds nothing; a `TaskDetail` test and an
`IssueDetailPanel` test with dates set to the same day find the same text;
a `SummaryPanel` test for a one-day period finds that day's date in one
heading only.

## The visual system

What is there is a real system, and a good one for a terminal: five
systems' hues — Jira blue, git yellow, the forge green, chat magenta and
Taskwarrior cyan (`newStyles`, `internal/tui/glyphs.go:114`; Taskwarrior's
marks the active task at the spine's end, `internal/tui/spine.go:122`) —
and red for failure alone, all taken from the terminal's own palette as
ANSI indices (`internal/tui/glyphs.go:102`) so the user's theme decides
the shades; shape for state (`○ ◐ ● ✗`, or `o * # x` in ASCII,
`internal/tui/glyphs.go:35`, `:47`); border weight for focus. None of that
should change.

The shapes now mark more than a status, and stay on one axis while they
do: `○` not begun, `◐` under way or partway, `●` done, `✗` broke. `◐`
means both "partly staged" on the Commits pane (`Model.stageGlyph`,
`internal/tui/commits.go:174`) and "announces when CI passes" on the
Messaging pane (`Model.messagingState`, `internal/tui/messaging.go:151`);
that is not a conflict, since each is the halfway point of its own
progression and words stand beside it. The rule a new use must keep: a
state glyph says how far something has got, never anything else. The
checkbox breaks it — `●` for chosen (UX-100). Three marks are not states
and take shapes of their own: `★` (`^` in ASCII) for a favorite
directory, `‹›` (`<>` in ASCII) for the value under a cursor — the
calendar's and the commit type's (`internal/tui/calendar.go:117`,
`internal/tui/composer.go:216`) — and `[]` for the value another calendar
column stands on (`:119`). The diff is the one documented exception to
the hue rule: an added line is drawn in the forge's green and a removed
one in red (`Model.markDiffLine`, `internal/tui/diff.go:111`, `:113`;
`styles`, `internal/tui/glyphs.go:90`), where the `+` and `-` git leaves
in place carry the meaning by shape (`internal/tui/diff.go:65`). Border
weight is the one rule the screen loses at its commonest size: at 80 by
24 the detail draws borderless, and the rail's heavy rules around the
focused pane carry the focus instead.

The web now speaks it too. The five systems' hues are tokens in both
themes (`web/src/index.css:63`), held at least 30° of OKLCH hue from the
status lights, the periwinkle control accent and each other, and at 4.5:1
as text, by `web/src/tokens.test.ts`; that separation moves two off the
terminal's families — the forge is teal, clear of the success light's
green, and Taskwarrior violet. They mark whose a thing is — the active
rail icon, each section's heading, the work story's stages and an issue's
local branch — and never how it stands. Every state is drawn by its shape
through one `StateMark` (`web/src/shell/StateMark.tsx:36`), hidden from
assistive tech beside its words. Type, space and corners each have one
scale (`web/src/index.css:167`), headings are set in sentence case — the
web lint refuses an `uppercase` class (`web/eslint.config.js:38`) — and
monospace is for code alone. Periwinkle stays the one control accent, and
the middle dot the separator.

It follows the window, too. Below `md` the rail keeps its icons alone, each
name kept for assistive tech and shown on hover; below `lg` the issue list
sits over its detail, and at every width it scrolls in its own pane, over a
line saying how many it holds. The header holds still while the content
scrolls beneath it, and a word wider than the content breaks rather than
scroll it sideways. `web/e2e/layout.spec.ts` holds every section to 640,
1024 and 1440 px in both themes (`:115`): nothing scrolls sideways, nor
the page down, Tab reaches every control, each in view as it takes focus,
and axe finds nothing. It holds the steps a click opens to the same
widths — the pull request form, the push confirmation, the announcement
preview and a refused write.

## Across the surfaces

What is open here is worktrees and a fresh base beyond the terminal;
GitHub- and terminal-shaped sentences; failed reads told as empty answers;
sentences that disagree; writes that skip the last look the rule asks for,
and looks the rule does not; actions that change their name partway
through; one concept under several names; docs that drift from the screen;
and three lists of what one surface can do and another cannot.

The rule the last looks follow was chosen by the maintainer: **confirm
what leaves the machine or cannot be undone; a reversible local toggle
acts at once.** The parity rule beside it: the terminal and the web cover
the loop, each for browsing; the command line covers one-shot and scripted
work, with `--json`, and is not a browser.

### UX-89 Worktrees and a fresh base on the command line and the web

Impact: low · Effort: medium

**Today.** Half of this has shipped. The web now makes a worktree: Start
in a new worktree (`StartInWorktreeButton`,
`web/src/features/issues/StartInWorktree.tsx:31`) calls `POST
/api/worktrees` (`server.CreateWorktree`,
`internal/webserver/branchcreate.go:178`) and offers to switch to it
(`WorktreeMadeOffer`, `:69`). What is still open:

- `workflow branch` has neither: `newBranchCmd`
  (`internal/cli/branch.go:34`) declares only the write flags, and
  `runBranch` (`:79`) creates off `currentBase` (`:110`) with no fetch and
  no worktree, though `connect` hands it a `deps.Git` that carries both
  `Fetch` and `CreateWorktree`.
- The web fetches nothing first: `startWork`
  (`internal/webserver/branchcreate.go:90`) and `CreateWorktree` both
  start from `currentBranchBase()` (`:134`), the base as it was last
  fetched, and the server's `Deps` has no fetch seam.
- The terminal does both: the branch creator fetches before it creates
  (`branchCreator.create`, `internal/tui/branch.go:468`, `willFetch`
  `:495`), offers "branch from what you have" when the fetch fails (`:390`),
  and `ctrl+g` makes a worktree instead.

**Instead.** `--worktree` and `--fetch` on `branch`, printing the
directory made; a fetch before the web's start-work and worktree writes,
with the terminal's "branch from what you have" as the refusal's way out;
all three over one composition in `internal/loop`, which today holds
neither branch write.

**Done when.** `workflow branch KEY --worktree` creates a directory beside
the repository and prints it on stdout; a `branch --fetch` test sees the
fetch seam called before the create; a webserver test with a fetch seam
that fails gets a problem naming the fetch, and one that succeeds sees the
fetch before `CreateBranch`.

### UX-126 Sentences that assume GitHub or the terminal, told elsewhere

Impact: low · Effort: small

**Today.** The no-token hint is fixed: `Resolve` wraps
`forge.ErrNoToken` with `Sources(kind, host)`
(`internal/forge/token.go:164`, `Sources` at `:252`), and the terminal
keeps that error in its own words (`forgeErrors`,
`internal/tui/failure.go:193`), so `doctor` and the interface now name the
same variable and tool for a GitLab host. The rest is still GitHub's or
the terminal's: a GitLab user reads "merge request" and `!7` on one line
and "pull request" and `#7` on the next, and a script is told to press a
key it does not have.

- `errNoCommitsToOpen`, `internal/cli/pr.go:25`, and `errPullAlreadyOpen`,
  `:29`: fixed "pull request" sentences that `composeRefusal` returns
  (`:238`, `:242`), while the same command's question uses
  `seams.Kind.Noun()` (`:136`) and its success line `Kind.Sigil()`
  (`:168`).
- `errNoPullRequest`, `internal/cli/announce.go:25`: fixed "pull request",
  wrapped at `:158`, though `announceSeams` carries `Kind` (`:37`) and
  uses it at `:168`.
- `renderReviews`, `internal/cli/reviews.go:103`: "No pull requests are
  waiting on your review." (`:105`) with no forge kind in reach —
  `reviewsSeams` (`:26`) carries none; `reviewLine`, `:118`, writes `#%d`
  before every number (`:124`).
- `docs/content/docs/scripting.md:88`, the `reviews` row of the stdout and
  stderr table: quotes that sentence verbatim, so the row moves with it.
- `draftStandup`, `internal/cli/standup.go:236`: the heading "## Pull
  requests"; `writePulls`, `:272`: `- #` before each number;
  `standupSeams` (`:24`) carries no `Kind`.
- `Model.reviewQueueDetail`, `internal/tui/reviewqueue.go:163`: "pull
  requests" whatever the forge (`:166`, `:172`); `Model.reviewRows`,
  `:214`, writes `" #"` (`:220`) where the Review pane uses
  `m.vocab.sigil` (`internal/tui/review.go:270`).
- `prBodyHelp`, `internal/tui/prcomposer.go:31`: a fixed "Write the pull
  request description above this line", handed to `$EDITOR` by the
  composer (`:465`) and the editor (`internal/tui/preditor.go:123`).
- `rejectionReason`, `internal/messaging/post.go:205`: three sentences
  end "then press enter to try again" (`:212`, `:214`, `:215`) inside a
  domain error. `runAnnounce` wraps it with `%w`
  (`internal/cli/announce.go:189`), as does `offerToPost`
  (`internal/cli/standup.go:166`), and `main`
  (`cmd/workflow/main.go:27`) prints it on stderr, key and all — against
  the interface's own rule, in the doc comment on `wording`
  (`internal/tui/failure.go:55`), that a full form names no key to press.
  The web, for its part, drops the reason and says "announce from a
  terminal to see its reason" (`messagingFaults`,
  `internal/webserver/errors.go:325`).

**Instead.** Carry the forge `Kind` on `reviewsSeams` and `standupSeams`
as `prSeams` and `announceSeams` already do, and word every fixed "pull
request" and `#` through `Kind.Noun()` / `Kind.Sigil()` on the command
line and `m.vocab` in the terminal and the editor help, keeping the
sentinels for `errors.Is`. Keep `rejectionReason` to the fix
(`join #dev`) and leave the key out — the overlay's footer already offers enter
— and, once the sentence names no key, let the web's detail carry it
rather than sending the user to a terminal. Change the `reviews` row of
scripting.md with it.

**Done when.** On a GitLab remote, tests of `pr`'s two refusals,
`announce`'s refusal, `reviews` (the empty-queue note on stderr, `!`
before each number on stdout), `standup`'s draft (`!7`, "Merge
requests"), the Reviews pane and the composer's `ctrl+o` help all see
"merge request" and `!` and never "pull request" or `#`; an `announce`
test whose fake Slack answers `not_in_channel` sees the fix on stderr and
no "press enter"; `docs/content/docs/scripting.md:88` matches the new
`reviews` note.

### UX-128 Failed reads pass as empty answers in status, standup and the web

Impact: medium · Effort: medium

**Today.** `status`, `standup` and the web server each drop a seam's read
error into their "nothing found" path, and nothing says so anywhere: a
prompt shows "CI none" while CI is red because a token expired and a
script cannot tell "none" from "unknown"; up to fifteen failing forge
requests are made in silence and the team receives a standup saying
nothing happened; in the browser three sections carry misleading copy
during an outage under a header that says Live, with no reason and no
Retry.

- `issueSummary`, `internal/cli/status.go:278`: `if err != nil` returns
  `""` (`:280`) — the issue read's error is dropped and the summary left
  blank.
- `gatherReview`, `internal/cli/status.go:295`: `err != nil` is folded
  into the not-found return (`:303`), so an unreachable forge reads `○
  Review`; at `:314` a `CheckStatus` error becomes `forge.CINone`, the same
  as no CI.
- `countChanges`, `internal/cli/status.go:323`: a `Changes` error becomes
  0 uncommitted files (`:325`).
- `TestStatusWhenCICannotBeRead`, `internal/cli/status_test.go:405`:
  asserts "CI none" and nothing about stderr, so the silence is neither
  pinned nor caught.
- "Standard output and standard error",
  `docs/content/docs/scripting.md:77`: "Standard error carries …
  warnings" (`:81`) — the contract `status` does not meet.
- `gatherStandup`, `internal/cli/standup.go:177`: `issues, _ :=
  seams.Search(…)` (`:183`) discards the search error; its doc comment
  settles "leaves its section empty rather than failing", not silence.
- `gatherPulls`, `internal/cli/standup.go:190`: a `Branches` error returns
  nil with no note, and at `:204` `err == nil && found && pull.IsOpen()`
  drops every `FindPull` error and keeps looping, up to
  `standupBranchLimit` (`:47`) failing requests.
- `writeIssues`, `internal/cli/standup.go:254`, and `writePulls`, `:266`:
  "- none" whether the service answered or refused; `--no-edit --yes`
  posts it. If FEAT-86 lands, these reads move to `summary`, and
  `standup --json` becomes `summary --json`.
- `server.snapshot`, `internal/webserver/stream.go:186`: its doc comment
  codifies the rule — a seam that fails yields an empty panel, the
  forge's keeping its last answer. `snapshotIssues` (`:344`) returns an
  empty first page when `Search` fails (`:351`); `frameBranch` (`:209`)
  answers an empty `gitrepo.Branch{}` when the read fails (`:211`), which
  `snapshot` hands to `branchDTO` (`:191`) and which makes
  `snapshotReview` answer not found (`:374`); `snapshotChanges` (`:360`)
  answers `changesDTO(nil)`. `forgeReview` (`:383`) keeps the answer it
  holds for the branch at its head when a read fails (`:402`); with none
  held it keeps and serves `readForge`'s empty review for an interval
  (`:407`) — indistinguishable from no pull request.
- `Review`, `api/openapi.yaml:2901`: carries `found`, `pull`, `ci` and
  `issue` only, and `Snapshot` (`api/openapi.yaml:2619`) has no per-panel
  problem.
- `server.review`, `internal/webserver/handlers.go:245`: a `CheckCI`
  error drops `ci` from the answer (`:260`), which the stream fills only
  with the CI it holds for the same pull request
  (`internal/webserver/stream.go:404`); `PullRequestSummary`
  (`web/src/features/review/ReviewPanel.tsx:81`) renders nothing for a
  null `ci` (`:117`), where the terminal's `Model.reviewDetail`
  (`internal/tui/review.go:281`) shows the CI failure under the pull
  request (`:319`).
- `BranchReview`, `web/src/features/review/ReviewPanel.tsx:45`: `found`
  false falls through to the `OpenPullRequest` form (`:67`), so a forge
  outage offers to open a pull request. The offer is wrong copy, not a
  wrong write: `ComposePull`'s doc comment (`internal/loop/pull.go:66`)
  lets a forge that cannot say through, and the open lands on the forge's
  own answer.
- `BranchPanel`, `web/src/features/branch/BranchPanel.tsx:22`: an empty
  name with `detached` false says "This directory is not a Git
  repository", which a failed read also produces; `StreamStatus`
  (`web/src/shell/StreamStatus.tsx:17`) sets Out of date only for an
  unreadable frame (`:11`), so a frame with an emptied panel reads Live.
- `ListAndDetail`, `web/src/features/issues/IssuesPanel.tsx:147`: the
  emptied first page renders "No issues match this view." (`:152`) — a
  Jira outage reads as an empty view.

`TestStreamSnapshotDegradesWhenSeamsFail`
(`internal/webserver/stream_test.go:272`) pins the web's silence (its
assert at `:289` wants `snap.Issues.Total` zero for a failing `Search`),
the terminal's "each pane fails on its own" has no web twin, and only the
opt-in `--log` records the failed request.

**Instead.** Keep degrading, but say so. On the command line, one stderr
line per service that failed, through the `output.notes` the writes use
and the sentinel wording the other commands share, stopping at the first
forge error in `standup` rather than making fourteen more, and leaving
stdout and the draft's "- none" as they are — telling "nothing to ask"
(`jira.ErrNoCredential`, no forge configured) from a service that
refused. On the wire, an optional problem per panel in the Snapshot (the
`Problem` shape `fault` already curates) and a `ci_error` on the Review,
rendered in that section as a failure — the failure `StateMark` and a
`role=alert` line — rather than as the empty state.

**Done when.** A `status` test with a forge that errors sees "CI none" on
stdout and a line naming the forge on stderr, and one whose forge answers
sees an empty stderr; a `standup` test with a 500 from Jira sees the
draft on stdout and a note naming Jira on stderr, while
`TestStandupWithNoWorkSaysEachSectionIsEmpty`
(`internal/cli/standup_test.go:172`) still sees three "- none" and no
note; a stream test with a failing `FindPull` sees a review panel
carrying a problem, beside the last answer held for that branch and head
when there is one; ReviewPanel, BranchPanel and IssuesPanel tests render
such snapshots by role alert rather than as the open-a-pull-request
form, the not-a-repository state or "No issues match this view."; a
ReviewPanel test with `ci` null and `ci_error` set finds the alert naming
the reason.

### UX-130 Four sentences that disagree with a neighbor or a sibling surface

Impact: low · Effort: small

**Today.** Four things are said two or three ways.

- `ErrDirtyTree`, `internal/loop/guards.go:16`: "the working tree has
  uncommitted changes"; `errDirtyTree`, `internal/tui/switchtask.go:25`,
  is a second sentence ("uncommitted changes — commit or stash them
  before switching tasks"), and `errDirtyTree`,
  `internal/webserver/checkout.go:19`, a third ("…; commit or stash them
  before switching") — one guard, three sentences, so a user who meets the
  refusal in the browser and then in the terminal reads two, and a change
  to the guidance is made in three places.
- `announceTarget`, `internal/cli/announce.go:199`: "the configured
  SERVICE channel" for a webhook and for a bot with no channel, used by
  `runAnnounce` for the `to` line (`:176`), the dry-run line
  (`announcePrompt`, `:226`) and the done notice (`:192`) — where
  `Messaging.Target`, `internal/config/config.go:284`, says "(no channel
  set)" (`:287`) and "the channel its webhook is bound to" (`:292`), which
  the interface's notice uses (`internal/tui/messagingpreview.go:245`),
  and the web's announce says "Announced to SERVICE." for a webhook
  (`web/src/features/messaging/MessagingPanel.tsx:225`). With a user
  token and no channel the command line's preview claims a channel that
  does not exist, and the post then fails.
- `placeholder`, `internal/tui/fields.go:99`: returns `dateLayout`
  (`:101`), Go's reference date `2006-01-02` (`:36`), as the hint, while
  `errNeedsDate`, `:29`, says "must be a date like 2026-09-21" — the hint
  reads as a stale date rather than a shape.
- `Model.messagingDetail`, `internal/tui/messaging.go:160`: "SERVICE is
  not set up" and "to ~/" + `config.FileName` (`:162`), with a literal
  newline mid-sentence that `wrap` re-breaks, where `messagingErrors`'
  `messaging.ErrNoCredential` wording (`internal/tui/failure.go:240`)
  words the same condition as "Messaging has no credential", names `workflow
  slack login` as well, and names no file; `FileName`'s comment
  (`internal/config/config.go:15`) says the name serves both search
  locations.

**Instead.** Let `loop.ErrDirtyTree` carry the guidance once ("…; commit
or stash them before switching") and have both surfaces render it
through their failure voice, dropping the two local sentinels; drop
`announceTarget` for `seams.Messaging.Target()` on the `to` line, the
dry-run line and the done notice; one example in `placeholder` and
`errNeedsDate` (from the fake-able clock, or a plain `YYYY-MM-DD` in
both); `config.FileName`
without the `~/` and the embedded newline, letting the
`messaging.ErrNoCredential` wording serve both places.

**Done when.** `grep -rn 'commit or stash'` finds one string outside
tests; `TestAnnounceDryRunComposesTheReadyMoment`
(`internal/cli/announce_test.go:51`) sees `Target()`'s wording and `grep
-rn announceTarget internal/cli` finds nothing;
`TestATransitionFillsAUserDateAndSeveralVersions` and
`TestADateFieldRefusesWhatIsNotADate` (`internal/tui/fields_test.go:41`,
`:80`) agree on one example; `TestTheSlackPaneNamesWhatItNeedsWhenUnset`
(`internal/tui/messaging_test.go:531`) refuses `~/` and the pane and the
failure wording name the same settings.

### UX-145 Writes that act at once, outside the rule

Impact: medium · Effort: small

**Today.** The rule — confirm what leaves the machine or cannot be
undone; let a reversible local toggle act at once — is kept by most
writes and broken by six, three of them on both surfaces, and two acts
are asked about on one surface and not the other.

- Tasks `d` done (`completeTask`, `internal/tui/taskactions.go:217`),
  `u` undo (`undoTasks`, `:241`) and `S` sync (`syncTasks`, `:271`) send
  at once through `actOnTask` (`:224`) or their own command. Sync leaves
  the machine; undo cannot itself be undone, since Taskwarrior has no
  redo; and done is reversible only by `u` while it is still
  Taskwarrior's last change, and runs the task's hooks. The same
  completion offered at a loop moment *does* get a last look — "Mark task
  N done?" (`offerComplete`, `internal/tui/taskoffers.go:249`) — so one
  act is asked about on one path and not the other.
- The web's Done (`TaskVerbs`, `web/src/features/tasks/TaskDetail.tsx:203`),
  Undo and Sync (`TasksPanel`, `web/src/features/tasks/TasksPanel.tsx:234`,
  `:244`) are each a `Verb` button (`TaskDetail.tsx:228`) that writes on
  click.
- People and groups `d` forget (`handlePersonKey`,
  `internal/tui/people.go:397`) deletes a kept decision at once, while the
  web asks first — "Forget OWNER?" with what forgetting costs
  (`ForgetConfirm`, `web/src/features/settings/people/PeopleTable.tsx:298`).
  What `kept.db` holds is what the user decided and "cannot be seen again"
  (CLAUDE.md), which is the rule's second clause.
- The web's two directory switches disagree with each other: Repositories
  asks "Switch to DIR?" first (`ConfirmSwitch`,
  `web/src/features/repositories/RepositoriesPanel.tsx:146`), while Switch
  to it and Switch to its worktree (`WorktreeMadeOffer` and
  `SwitchToWorktreeButton`,
  `web/src/features/issues/StartInWorktree.tsx:69`, `:119`) switch the
  server's directory on click. The terminal switches at once from the
  Repositories pane and asks only when the switch would lose something
  this session holds — a draft commit message, an edited pull request, a
  queued announcement, a forge comment (`leaveFor` and `lostOnLeaving`,
  `internal/tui/reposwitch.go:104`, `:141`; `switchGuard`, `:167`). The
  rule counts every directory switch as one to confirm: it changes what
  every pane, and on the web every open tab, reads and writes, so a
  stray `enter` or click lands work in the wrong repository.

Acting at once, by the rule and on both surfaces: start and stop a task;
favorite and unfavorite a directory; check a group or a person for the
announcement's tags; link an owner, or mark one not on Slack; stage and
unstage, one file or all; link and unlink a branch's issue where no pull
request description changes. The terminal's branch creator, task
switcher and worktree toggle also show what they will do first, but as
the place the name is chosen rather than as a confirmation, so the web's
one-click Start work, Check out and Start in a new worktree
(`web/src/features/issues/WorkStory.tsx:364`,
`web/src/features/issues/IssuesPanel.tsx:480`,
`StartInWorktree.tsx:31`) are inside the rule — local, and undone by
switching back.

**Instead.** A last look on each of the four, on both surfaces: done,
undo and sync through the terminal's `lastLook`
(`internal/tui/overlay.go:217`), worded as `offerComplete` already words
done, and through a confirm step on the web shaped like `ForgetConfirm`;
forget through a `lastLook` naming what the next announcement will ask
again. For the switch, one look on both surfaces: the terminal's
`enter` and the dir prompt's go ask "Switch to DIR?" through `lastLook`,
folding in `switchGuard`'s list of what would be lost when there is
any, and the web's Switch to it and Switch to its worktree pass through
`ConfirmSwitch` rather than switching around it. The promises table at
the head of this file names the rule and the acts it exempts, so the
next re-count checks them.

**Done when.** A screen test presses `d`, `u` and `S` on the Tasks pane
and finds no call to the fake Taskwarrior until enter; a screen test
presses `d` in People and groups and finds the fake store's
`ForgetOwner` uncalled until enter; web tests click Done, Undo and Sync
and find no request until the confirm; a screen test presses `enter` on a
favorite and finds no `Next` until the look is confirmed; a web test
clicks Switch to it and finds no `PUT /api/repositories/here` until
Switch is pressed in the confirmation.

### UX-146 An action changes its name partway through

Impact: low · Effort: small

**Today.** An action should keep one name from the key or button that
starts it, through the label it shows while busy, to the notice that says
it is done — "Stage", "Staging…", "Staged". Many do. These do not.

In the terminal:

- The pull request editor's enter says "save" (`prEditor.footer`,
  `internal/tui/preditor.go:78`) and its notice "updated #N" (`:184`).
- The branch creator's enter says "create worktree" in worktree mode
  (`branchCreator.footer`, `internal/tui/branch.go:393`) and its notice
  "worktree for NAME at DIR" (`internal/tui/branchresult.go:115`) — no
  verb at all.
- The branch linker's enter says "link and update the description"
  (`internal/tui/branchlink.go:116`) and its notice "linked BRANCH to KEY"
  (`:243`), dropping the description it also changed.
- Tasks `d` is "done" in the footer and help
  (`internal/tui/keys.go:285`), its dry run says "would complete task N"
  and its notice "completed N" (`completeTask`,
  `internal/tui/taskactions.go:218`); the loop-moment offer's title is
  "Complete the task", its enter "done", its busy line "completing"
  (`offerComplete`, `internal/tui/taskoffers.go:258`).
- The `lastLook` bodies speak two ways: lowercase imperatives with no stop
  — "push NAME to REMOTE" (`internal/tui/branch.go:220`), "rebase NAME
  onto BASE" (`:232`), "re-run the failed checks on #N TITLE"
  (`internal/tui/checks.go:200`), and the amend preview's "fold the staged
  changes into SUBJECT" (`amendPreview.view`,
  `internal/tui/commits.go:434`) — beside sentence questions — "Start task
  N in Taskwarrior? …" (`internal/tui/taskoffers.go:116`), "Mark task N
  done?" (`:259`), "Annotate task N with #7 and its URL?" (`:241`).

On the web:

- The issue row's Check out (`RowCheckout`,
  `web/src/features/issues/IssuesPanel.tsx:500`) is busy as
  "Switching…" and done as "Checked out NAME." (`:484`), while the work
  story's Check out this branch is busy as "Checking out…"
  (`web/src/features/issues/WorkStory.tsx:350`).
- Done is busy as "Completing…" and done as "Completed task N."
  (`web/src/features/tasks/TaskDetail.tsx:203`–`:207`).
- Local data opens with "Clean cache…" and "Clean everything…"
  (`CleanOpeners`, `web/src/features/settings/people/LocalData.tsx:250`,
  `:262`), asks "Remove NAMES?" (`:300`) over a button labeled "Clean"
  (`:311`), shows "Cleaning…" (`:265`) and ends "Removed NAMES." (`:172`);
  the command line calls the same act `db-clean`, "Remove the local
  databases" (`internal/cli/dbclean_cmd.go:31`).
- Five buttons keep their resting label while their request is out:
  ConfirmSwitch's Switch (`RepositoriesPanel.tsx:183`), each file's Stage
  or Unstage (`ChangeRow`, `web/src/features/branch/WorkingTree.tsx:110`),
  Unlink and Link (`web/src/features/branch/IssueLink.tsx:123`, `:197`),
  and the confirm step's Forget
  (`web/src/features/settings/people/PeopleTable.tsx:342`) — where Stage
  all, beside them, says "Staging…" (`WorkingTree.tsx:144`).

**Instead.** One verb per action, from start to notice, as the
Vocabulary table at the head of this file names it: the editor's enter
"save" and its notice "saved #N"; "create worktree" and "created
worktree for NAME at DIR"; "link and update the description" and "linked
BRANCH to KEY and updated #N"; "mark done" → "marked N done" on both
surfaces; every `lastLook` body a sentence ending in a question, as the
task offers already are; both checkouts, the row's and the story's, busy
as "Switching…" and done as "Switched to NAME."; Local data's opener,
question, button and notice all "Remove"; and a busy label on the five.

**Done when.** A table test over the terminal's footer labels and the
notices each act ends in finds the same verb stem in both; a test reads
every `lastLook` body and finds it ends in "?"; web tests find each of
the five buttons' busy label while its fake request hangs; the row's and
the story's checkouts both say "Switching…" while busy and "Switched to
NAME." when done; and Local data names no button or notice "Clean".

### UX-147 One concept, several names

Impact: medium · Effort: medium

**Today.** The same thing goes by a different name on each surface, or
by the same name as a different thing. This is the work item behind the
Vocabulary table at the head of this file; each row below is a rename
that table settles.

- **Branching for an issue.** The command line's `branch` says "Branch
  NAME from BASE and switch to it" (`runBranch`,
  `internal/cli/branch.go:99`) and "Created NAME" (`:115`); the terminal's
  key is "branch for KEY" (`internal/tui/detail.go:214`) and its overlay
  "New branch for KEY" (`internal/tui/branch.go:361`), ending "created and
  switched to NAME" (`internal/tui/branchresult.go:79`); the web's button
  is "Start work", ending "Started work on NAME."
  (`web/src/features/issues/WorkStory.tsx:382`, `:367`).
- **The review queue.** `workflow reviews`
  (`internal/cli/reviews.go:43`); the terminal's pane "Reviews"
  (`pane.title`, `internal/tui/panes.go:42`) whose rail counts "review
  requests" (`internal/tui/reviewqueue.go:159`); the web's rail and
  heading "Reviews" (`web/src/shell/sections.ts:26`) over an unseen
  heading and list named "Review requests"
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:155`, `:248`) —
  and "Review", one letter shorter, is the branch's own pull request.
- **The chip checklist that narrows a list.** "Where" on Issues (the
  terminal's `p`, `internal/tui/keys.go:239`; the web's chips,
  `web/src/features/issues/IssuesPanel.tsx:90`), "Narrow" on Tasks (`f`,
  `internal/tui/keys.go:292`;
  `web/src/features/tasks/TaskListControls.tsx:73`),
  "Filter" on Reviews (`f`, `internal/tui/keys.go:278`;
  `ReviewQueuePanel.tsx:141`) — while "Filter" also names the typed text
  search on Issues and Tasks (`/`, `internal/tui/keys.go:238`, `:291`;
  `web/src/features/issues/IssueListControls.tsx:17`,
  `TaskListControls.tsx:59`). Reviews' chips and the other two lists'
  search share one word for two controls.
- **"Task".** The terminal's git checkout picker is "switch task" (`s`,
  `internal/tui/keys.go:251`) titled "Switch task"
  (`internal/tui/switchtask.go:20`), and its refusal says "before
  switching tasks" (`:25`) — while "task" everywhere else is a Taskwarrior
  task, and "Switch the task" (`internal/tui/taskoffers.go:145`) stops one
  Taskwarrior task and starts another.
- **Taskwarrior's done.** "done" (the terminal's key), "Done" (the web's
  button), "completed" and "Complete the task" (the notices and the
  offer's title, UX-146), and "task done" (a web issue row's mark,
  `docs/content/docs/web.md:78`).
- **The local stores.** `db-clean`, "Remove the local databases"
  (`internal/cli/dbclean_cmd.go:31`); the web's "Local data", "Clean
  cache…", "Clean everything…" and a table named "Local databases"
  (`web/src/features/settings/people/LocalData.tsx:20`, `:77`).
- **Reading again.** The web says "Retry" after a failed read and
  "Refresh" otherwise (`readAgainLabel`,
  `web/src/features/tasks/TasksPanel.tsx:284`), but "Read the code owners
  again" (`web/src/features/settings/people/PeopleTable.tsx:48`), "Read
  the groups again" (`RepoGroups.tsx:32`) and "Read the local data again"
  (`LocalData.tsx:52`) in Settings; the terminal says "refresh" (`r`,
  `internal/tui/keys.go:244`).

**Instead.** The Vocabulary table at the head of this file names one
word per concept, and each surface takes it:

| Concept | Command line | Terminal | Web |
| --- | --- | --- | --- |
| Branch for an issue | `branch`; "Start work on KEY: create NAME from BASE and switch to it" | key "start work", overlay "Start work on KEY" | "Start work" (kept) |
| The review queue | `reviews` (kept) | pane "Reviews", rail "N waiting" | rail "Reviews", heading and list "Waiting on your review" |
| Chip checklist | — | "filter" (`f`) on all three | "Filter" on all three |
| Typed search | — | "search" (`/`) | "Search" |
| Git checkout picker | — | "switch branch", "Switch branch", "Switching…" | "Switch branch", "Switching…" |
| Taskwarrior's done | — | "mark done", "marked N done" | "Mark done", "Marked task N done." |
| Local stores | `db-clean` (kept), "Remove workflow's local data" | — | "Local data", "Remove…" |
| Read again | — | "refresh"; after a failure "press r to try again" (kept) | "Try again" after a failure, "Refresh" otherwise, everywhere |

A rename of a key's help changes `ui.keys`' action names only if the
action is renamed too; this keeps the action names (`switch-task`,
`filter-tasks`, `narrow-tasks`) and changes the shown words, so no
configuration breaks.

**Done when.** `grep -rn '"switch task"\|Switch task\|switching tasks'
internal/tui` finds nothing; the three chip checklists' accessible names
are "Filter" in a web test, and the terminal's help shows "filter" under
`f` in each group; a web test finds no button named "Read … again" or
"Retry" after a failed read, only "Try again".

### UX-148 Docs that drift from the screen

Impact: low · Effort: small

**Today.** Three pages say less, or other, than the screen.

- The example screen in "The screen", `docs/content/docs/usage.md:34`–
  `:61`, draws seven panes, 1 Issues to 7 Tasks, though the rail it
  introduces is "the nine panes" (`:71`) and `paneCount` is 9
  (`internal/tui/panes.go:36`): Summary and Repositories are missing.
- The pane table in the same page numbers every pane but two: "| Summary
  |" (`:184`) and "| Repositories |" (`:190`) have no number, where "6
  Reviews" (`:167`) and "7 Tasks" (`:171`) do, and `1`–`9` reach them
  (`:126`).
- "Review", `docs/content/docs/web.md:154`: the state is "Draft or Ready
  for review while it is open, Merged once it has merged", but
  `stateLabel` also says "Closed"
  (`web/src/features/review/ReviewPanel.tsx:149`); and the issue the pull
  request is for is listed among what shows "while it is open" (`:155`–
  `:157`), but `IssueRow` is drawn in every state (`ReviewPanel.tsx:110`),
  only the review rows waiting on `state === 'open'` (`:113`).

**Instead.** Redraw the example at nine panes (a taller screen, or the
lower panes folded to their title lines, as the rail does when rows run
short); number the two rows "8 Summary" and "9 Repositories"; in web.md,
"Draft or Ready for review while it is open, Merged or Closed once it is
not", with the issue moved out of the open-only sentence.

**Done when.** The example screen holds nine titled panes; `grep -n '^|
[A-Z]' docs/content/docs/usage.md` finds no pane row without its number;
web.md names Closed and lists the issue outside "While it is open".

### UX-149 What the web cannot do that the terminal can

Impact: medium · Effort: large

**Today.** By the parity rule the web should cover the loop. Checked
against `api/openapi.yaml`'s operations and `web/src`, these the terminal
does and the web cannot, by stage. `docs/content/docs/web.md:406`, "What
stays in the terminal", lists most of them as decisions for now.

- **Jira.** Move an issue to any status, with its fields: the web moves
  only to `jira.review_status`, only from the open-pull-request offer
  (`moveToReview`, `web/src/features/review/followUpApi.ts:23`; the
  operation is "not a general transition", `api/openapi.yaml:199`), where
  the terminal has `t` (`openStatusPicker`, `internal/tui/picker.go:210`).
  Assign (`a`, `openAssign`, `internal/tui/issuewrite.go:79`) and log work
  (`w`). All three: FEAT-80.
- **Git.** Rebase onto the base (`u`, `previewRebase`,
  `internal/tui/branch.go:229`): no entry yet. Amend and fix up (`A`, `f`,
  `internal/tui/commits.go:374`, `internal/tui/picker.go:438`) and a
  file's diff (`internal/tui/diff.go:29`): UX-88. Run pre-commit on its
  own (`h`, `internal/tui/commits.go:318`) and set up lefthook (`g`,
  `openHookgen`, `internal/tui/hookgen.go:56`): no entry yet.
- **The forge.** Edit an open pull request (`e`), merge (`M`), finish a
  merged branch (`F`) and re-run failed CI (`R`): FEAT-79. Read the log of
  any check the forge keeps one for: the web offers Show log on a failed
  check alone (`CheckRow`, `web/src/features/review/ReviewPanel.tsx:451`),
  the terminal's checks overlay on every check with a log
  (`internal/tui/checks.go:105`): no entry yet. Choose among the
  repository's pull request templates (`ctrl+t`): UX-88, with UX-62 for
  the command line.
- **Messaging.** Edit the announcement's text before it goes (`e` in the
  preview, `messagingPreview.applyEdit`,
  `internal/tui/messagingpreview.go:211`): no entry yet. Announce when CI
  passes (`w`, `postWhenGreen`, `:169`): FEAT-82. Know an announcement was
  already made: FEAT-84.
- **Everywhere.** A sheet of every key, `?`
  (`internal/tui/help.go`), and reaching any action without the mouse
  beyond Tab: UX-152.

**Instead.** Nothing here beyond the entries named: this list is the
index, so the next edition can strike a row as its entry ships. Three
rows have no entry yet — rebase, pre-commit and lefthook, a log for any
check, the announcement's text — and each would be a FEATURES.md entry
in its stage's section, an endpoint and a section control over the seam
the terminal already calls.

**Done when.** Every row names an entry, and web.md's "What stays in the
terminal" lists the same rows, no more and no fewer.

### UX-150 What the terminal cannot do that the web can

Impact: low · Effort: medium

**Today.** Three things the web does have no key in the terminal,
checked against `internal/tui`.

- **Editing the settings.** The web's Settings writes the configuration
  (`PUT /api/config`, `server.UpdateConfig`,
  `internal/webserver/config.go:139`); the terminal has no settings pane,
  and with a configuration that will not load shows only what is wrong
  and the command to run (`configErrorStatus`,
  `internal/tui/render.go:426`).
- **Cleaning the local data.** The web's Local data
  (`web/src/features/settings/people/LocalData.tsx`) and `workflow
  db-clean` (`internal/cli/dbclean_cmd.go:31`) remove the store's files;
  the terminal has neither key nor overlay.
- **Unlinking a branch from its issue.** The web's Unlink
  (`web/src/features/branch/IssueLink.tsx:123`) calls
  `Git.UnlinkIssue` (`internal/seams/seams.go:123`, wired at
  `internal/wiring/wiring.go:250`); the terminal links (`i`,
  `internal/tui/keys.go:252`) and nothing in `internal/tui` calls
  `UnlinkIssue`.

Tracking an issue in Taskwarrior is in both (`T`,
`internal/tui/keys.go:245`; Track in Taskwarrior,
`web/src/features/tasks/IssueTasks.tsx:192`), as are favorites, worktrees,
people and groups, and the Summary.

**Instead.** Unlink in the terminal's link overlay (`branchLinker`, a
second action when the branch is already linked); a Local data overlay
from the Repositories pane, over the same `db-clean` composition; and
settings left to the web and `config init`, saying so in usage.md's
limits rather than building a second form — the terminal's
configuration screen already names the command, and UX-153 is where a
first-run flow belongs.

`internal/tui` is at its file budget (57 of 57,
`scripts/package-size-budgets.txt`), so a new overlay file there needs a
budget bump with its reason, or a home in an existing file.

**Done when.** A screen test on a linked branch unlinks it at once — a
last look only when the pull request's description would change — and
finds the fake `UnlinkIssue` called once; a screen test confirms the
cache's removal (it cannot be undone), then finds the store's clean seam
called with the cache scope; usage.md names settings as the web's and
`config`'s.

### UX-151 What a script still cannot ask for

Impact: low · Effort: medium

**Today.** By the parity rule the command line covers one-shot and
scripted work, with `--json`, and leaves browsing to the two interfaces.
It has `status`, `reviews`, `standup`, `branch`, `pr`, `announce`,
`db-clean`, `doctor`, `config` and `slack` (`internal/cli`, each
command's `Use:`), and `--json` on `status`, `reviews` and `doctor`. A
script still cannot:

- branch into a worktree or from a fresh base — UX-89;
- read the issues in a view, an issue in full, or move, comment on,
  assign or log work on one — FEAT-78, whose `comment` should take the
  text on stdin, as `git commit -F -` does, so a script never quotes it;
- read the Summary for a period — FEAT-86;
- read what `pr` opened as JSON — UX-92;
- finish a merged branch — FEAT-83;
- list the repository's worktrees and the favorite directories, which
  the Repositories pane and `GET /api/repositories` both read: no entry
  yet.

Tasks stay out on purpose: Taskwarrior is its own command line, `task
export` is its JSON, and a `workflow task` would only wrap it.

**Instead.** `workflow repositories --json`, printing the worktrees and
favorites the API's `Repositories` shape carries, over the same seams;
the rest in the entries named. `internal/cli` is at its file budget (18
of 18, `scripts/package-size-budgets.txt`), so the new command's file
comes with a budget bump and its reason.

**Done when.** `workflow repositories --json | jq '.worktrees[0].dir'`
prints a path in a test with a second worktree, and every row above
names an entry.

## New ideas

Ideas this edition adds that no surface has begun: the keyboard reach the
web lacks, a first run that starts inside the interface rather than
before it, and a mode a screen reader can follow. Two more are features
rather than changes to how the interface is used, so they live in
[FEATURES.md](FEATURES.md) and are only pointed at here:

- **FEAT-86 One summary, read and posted everywhere** — `standup` and the
  Summary pane become one read, with `workflow summary --json` and a
  Post… from the Summary on both interfaces.
- **FEAT-87 What workflow did, and taking it back** — a session's log of
  every write, with undo where the system allows it; the feature UX-68
  stops short of.

### UX-152 Keyboard on the web

Impact: medium · Effort: medium

**Today.** The web is reached by Tab and the mouse alone. No section
listens for a key outside its own fields — the only `onKeyDown` handlers
are the comment composer's and the calendar grid's
(`web/src/features/issues/CommentComposer.tsx:243`,
`web/src/features/summary/MonthGrid.tsx:160`) — and nothing lists what
the keyboard can do. The terminal's every action has a name and a key,
generated into its `?` sheet (`helpBuilder.place`,
`internal/tui/keys.go:153`; the bindings, `:220`–`:364`), and `ui.keys`
moves any of them (`UI.Keys`, `internal/config/ui.go:47`), a map the API
already carries (`UIConfig.keys`, `api/openapi.yaml:3438`) and the web
server already checks through the terminal's own `tui.CheckKeys`
(`internal/cli/web.go:121`). Someone fluent in the terminal starts over
in the browser.

**Instead.** Three parts, one source.

- **The same names, served.** A `keys` list on the API — each action's
  name, its help words, its group and its key with `ui.keys` applied —
  generated from the terminal's `helpBuilder`, handed to the server as a
  seam as `CheckKeys` is, so a rebinding in the file reaches both. The
  web binds the actions it has (`comment`, `change-status` once FEAT-80
  lands, `stage`, `commit`, `push`, `open-pull-request`, `filter`, the
  pane numbers to the sections) and leaves the rest unlisted.
- **A `?` sheet.** A dialog listing the bound actions by group, as the
  terminal's help does, with each control also carrying
  `aria-keyshortcuts`.
- **A command palette.** `ctrl+k` opens a combobox over the current
  section's actions, by their help words, so an action is reachable by
  name without its key.

Accessibility shapes it. No single-character shortcut fires while focus
is in a text field, a textarea or the comment composer. WCAG 2.1.4
(character key shortcuts) asks that single keys can be turned off or
remapped: remapping is `ui.keys`, and a "Single-key shortcuts" switch in
Settings turns them off, leaving `?` behind `shift` and `ctrl+k`. The
sheet and the palette pass the axe scan and `web/e2e/layout.spec.ts` at
every width in both themes. `web/src/shell` is at its file budget (12 of
12, `scripts/package-size-budgets.txt`), so the sheet and the palette
live under a feature of their own, or the shell's budget is bumped with
its reason.

**Done when.** A web test presses `?` and finds a dialog listing
"comment" under Issues with key `c`; with `ui.keys: {"comment": "C"}` the
same test finds `C`; typing `c` in the filter box opens nothing; with
single-key shortcuts off, `c` opens nothing and `ctrl+k` still does;
`ctrl+k`, "stage all", enter stages every change; the a11y spec passes
with the sheet and the palette open.

### UX-153 Set up from inside the interface

Impact: medium · Effort: medium

**Today.** The first run happens before the interface, at a prompt.
`workflow` with no file shows "No .workflow.json found." and "Create one
with `workflow config init`." (`configErrorStatus`,
`internal/tui/render.go:426`, from `config.NoConfigHeadline` and
`config.InitStep`, `internal/config/config.go:22`, `:24`) and nothing
else to do; `workflow --web` with no file prints "configuration did not
load cleanly" on stderr (`serveWeb`, `internal/cli/web.go:151`; UX-91)
and serves a page whose Settings edits a file that exists
(`docs/content/docs/web.md:434`, "What stays in the terminal"). The
questions `config init` asks — Jira's address and token, checked
against Jira before they are kept, then a messaging webhook
(`runGuidedInit`, `internal/cli/config_cmd.go:268`; `collectJira`,
`:362`; `collectMessaging`, `:418`) — are asked only on a terminal's
stdin.

**Instead.** The same questions in both interfaces, over the same
composition, moved from `internal/cli` to where all three surfaces reach
it. In the terminal, the no-file screen offers enter to set up: a form of
the guided init's questions, the token read without echo and stored as
`config init` stores it (`Prompt.StoreSecret`, the OS keychain), the Jira
check shown as it runs, then the file written and the panes loaded
without a restart. On the web, Settings with no file becomes that form,
with where to write — the repository or the home directory — chosen
first, and `PUT /api/config` allowed to create the file it names; the
stderr line names `workflow config init` as UX-91 asks. A credential
typed in the browser travels only to the loopback server, which the
same-origin guard already holds, and is masked in every answer.
`internal/tui` is at its file budget (57 of 57,
`scripts/package-size-budgets.txt`), so the terminal's form needs a
budget bump with its reason, or a home in an existing file.

**Done when.** A screen test with no file presses enter, answers the
questions against a fake Jira that accepts the token, and finds the
Issues pane loaded and the file written with the token absent from it;
a web test with no file fills Settings, saves, and finds the stream
connected; the test that the token never reaches a response or a log
line ships with it.

### UX-154 A screen-reader and plain mode

Impact: low · Effort: large

**Today.** UX-64 asks for the alternate screen to be optional; a screen
reader needs more than that. The interface redraws one full screen of
boxes each frame (`view.AltScreen = true`, `internal/tui/render.go:38`),
so a reader re-reads the rail, the borders and the footer at every
change, and the one line that says what just happened — the notice
(`Model.noticed`, `internal/tui/overlay.go:169`) — sits among them.

**Instead.** `ui.screen: lines` (and `--plain` for one run): no
alternate screen, no rail, no boxes; the interface prints, and each
change is one appended line.

- **What prints.** On start, where you work and the progress row in
  words ("Issue done, Branch done, Commits in flight, Review not
  started"); then a line per notice, failure and settled read ("CI
  passed on #42", "3 review requests waiting"), each once, printed above
  the input line as Bubble Tea's print command does outside the
  alternate screen.
- **Reading a pane.** The digit keys still choose a pane, and enter
  prints its detail as plain text, glyphs spelled out ("in flight", not
  `◐`), the way `ui.ascii` swaps them now.
- **Overlays as questions.** A picker prints its choices numbered and
  takes a number; a preview prints what it will send, then "Send? enter
  sends, esc cancels" — the command line's `writePrompt` shape.
- **Keys unchanged.** The same bindings and `ui.keys`, and `?` prints the
  help, one key to a line.

**Done when.** A screen test in lines mode sees no box-drawing character
in any frame, and after a fake CI turns green finds one new line "CI
passed on #42" and no redrawn rail; a test in lines mode presses `t`,
`2`, enter and finds the issue moved, with each step's question on a line
of its own.

## Ideas that would reopen a settled decision

None this edition. The nine-pane rail (`paneCount`,
`internal/tui/panes.go:36`), once written up as five in FEATURES.md, six
before the Tasks pane joined it, seven before the Summary and eight
before Repositories, *is* the decision as built. UX-154's lines mode
draws the same nine panes one at a time, on request, rather than
reopening the rail, and UX-153 moves an item off web.md's "What stays in
the terminal", which lists choices for now, not settled decisions.
