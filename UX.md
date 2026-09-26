# User experience ideas

Ways to make `workflow` easier to learn, harder to misuse and kinder when
something goes wrong. Like [FEATURES.md](FEATURES.md), this is a brainstorm,
not a plan: nothing here is agreed or scheduled.

It is written for two readers: a contributor deciding what to improve, and a
later Claude Code session asked to "pick up UX-61". Each entry says what
happens today, what could happen instead, where the change would land, and
how to tell when it is done. The numbering continues from the entries that
have since shipped, so an ID is never reused.

Checked against commit `f05ae9f` on 2026-09-24 (main after the debt
paydown, PRs #134 and #136, and a Dependabot bump). Line numbers drift, so
every pointer also names the symbol it means.

## How this was produced

One pass over every surface and every package, from the source and, for
the web, from the screen.

1. **A read of every surface and every package.** Every command, flag and
   message in `internal/cli`; every key binding, overlay, empty state,
   loading state and error state in `internal/tui`; every panel, button,
   string and endpoint call in `web/src` and `internal/webserver`; and
   every other package under `internal/`, read against
   [CLAUDE.md](CLAUDE.md), the [clig.dev](https://clig.dev) guidelines for
   the command line, the promises the terminal interface makes about
   itself (the table below, re-counted) and the accessibility floor the
   web's gates enforce, with each unit's tests read beside it.
2. **The web was looked at.** The mock build ran under Playwright and every
   section was screenshotted in both themes at 640, 1024 and 1440 px, plus
   the production build's empty and no-API states, so a claim about how
   the web *looks* was read from the screen and names its screenshot. The
   terminal was not driven; its screens are known from the source and from
   the golden output the screen tests hold.
3. **Measurements taken once, at this commit.** `task cover:branch`,
   `scripts/check-file-length.sh --list`, `scripts/check-package-size.sh
   --list`, `task cloc`, the web's v8 summary, knip, deadcode and
   `task docs:check`, each run once and read from its output.
4. **Every finding refuted before it was written.** An independent reader
   tried to refute each finding, and a second one each rated medium or
   high; every cited line was read again as its entry was written, and a
   claim that could not be pointed at a line was dropped.

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
| "a key it does not show does nothing" | No longer stated anywhere; the sentence an earlier edition cited in `docs/content/docs/usage.md` is gone. Folds into the next row. | — |
| "`?` lists every key" | `docs/content/docs/usage.md:75` | **Yes, by construction, and a test enumerates every placement.** Help is generated from the bindings (`internal/tui/keys.go:138` `helpBuilder.place`, rendered at `internal/tui/render.go:219`): 57 of 57, on 56 lines, since `cycle-type-right` rides `cycle-type-left`'s line (`internal/tui/render.go:233` skips a binding with no help of its own). `TestHelpListsEveryPlacedBinding` (`internal/tui/help_test.go:211`) reads `?` back and holds it to a table of every placement, group by group, and each action moved to a free key must be listed on its own line in its group. |
| "the one way the interface says something broke" | `wording`, `internal/tui/failure.go:63` | **Yes: every site that renders an error's text.** Each tells it through `errorSentence` (`internal/tui/failure.go:100`), which words every sentinel the seams return briefly and in full: 6 panes, the configuration screen, 11 `pinnedOutcome` overlays and the 2 details that repeat a summary row's failure beneath it through `failureBlock` (`internal/tui/failure.go:402`), 13 one-failure rows through `failureLine`, 4 rail and summary rows through `failureSummary`, and 10 failure notices through `noticedFailure`, drawn in the failure style, the re-run's led by what failed so a clipped row still names it; a run's headline names the step a failure status stopped, and a pull request opened without every reviewer says why in brief. The 3 guidance notices stay plain, since red means something broke. The width-one glyph-only marks — a failed check, job, stage or review CI — and the three rails that point at their detail carry no error text to word. One site drops the text it has: the branch creator's fixed "could not fetch" line (UX-129). |
| "Nothing outward facing is sent without" a last look | `internal/tui/comment.go:64` | **Yes: 20 of 20.** Every act that writes through a seam — five on Jira, nine through git, four on the forge, the post and the lefthook file — waits on a preview or a confirmation; the push, `R`'s re-run of CI and `u`'s rebase share one last look, `lastLook` (`internal/tui/overlay.go:158`). Of the twenty, eleven leave the machine: the Jira five, the forge four, the post and the push; the rest are local writes that still get a look. The acts are listed under the table. |
| "a refused change must never go unseen" | `internal/tui/picker.go:312` | **Yes: 13 of 13.** Every overlay that sends a request guards it while in flight (an earlier edition counted 1 of 7; the last counted 14 by including the commit composer, which holds a `sendState` for a validation refusal only and hands its commit to a run overlay, `internal/tui/composer.go:364`), and every one keeps a refusal in the overlay, where it happened, until `esc` — the merge and finish previews last, through `pinnedOutcome` (`internal/tui/failure.go:420`). |
| "Each pane fails on its own" | `docs/content/docs/usage.md:80` | Yes. `internal/tui/tui.go:154` batches six loads; five panes hold and render their own load's error through `failureBlock` (`internal/tui/detail.go:339`, `internal/tui/branch.go:133`, `internal/tui/commits.go:125`, `internal/tui/review.go:292`, `internal/tui/reviewqueue.go:115`). The Messaging pane's load, `loadAnnounces`, reads the store, whose errors the wiring drops before they reach the model, so it has no load error to hold; its `failureBlock` site (`internal/tui/messaging.go:187`) renders a failed post. |
| State is "carried by the SHAPE of a glyph rather than its color" | `internal/tui/glyphs.go:16` | Yes. `unicodeGlyphs` and `asciiGlyphs` differ in shape (`internal/tui/glyphs.go:31`, `:43`); `NO_COLOR` keeps bold and faint (`internal/tui/tui.go:129`). One residue: the progress spine's per-system hue is color-only, mitigated by the name or its initial. |

What was counted, so the next re-count is a diff. The 20 acts behind a last
look, each named by the seam its confirm captures: `Jira.Transition`
(`internal/tui/picker.go:379`), `Jira.Comment` (`internal/tui/comment.go:149`),
`Jira.Assign` (`internal/tui/issuewrite.go:76`) and `Jira.AddWorklog`
(`:83`), both sent at `:172`, `Jira.LinkPullRequest`
(`internal/tui/issuelink.go:86`), `Git.CreateBranch`
(`internal/tui/branchresult.go:27`), `Git.CreateWorktree`
(`internal/tui/branchresult.go:18`), `Git.Checkout`
(`internal/tui/switchtask.go:181`), `Git.Commit`
(`internal/tui/composer.go:362`), `Git.Amend` (`internal/tui/commits.go:382`),
`Git.Fixup` (`internal/tui/commits.go:397`), `Git.Push`
(`internal/tui/run.go:396`), `Git.Rebase` (`internal/tui/run.go:411`),
`Git.Finish` (`internal/tui/finish.go:125`), `Forge.Rerun`
(`internal/tui/checks.go:197`), `Forge.CreatePullRequest`
(`internal/tui/prcreate.go:88`), `Forge.EditPullRequest`
(`internal/tui/preditor.go:143`), `Forge.Merge` (`internal/tui/merge.go:206`),
`Messaging.Post` (`internal/tui/messagingpreview.go:138`, posted through
`sendToMessaging` at `:172`) and `Hooks.Write`
(`internal/tui/hookgen.go:138`); the local writes with no look are stage and
unstage (`internal/tui/commits.go:269`), stage all (`:293`) and the pre-commit
run (`:319`). The 13 overlays with an in-flight guard, each refusing every
key while `send.sending`: `statusPicker` (`internal/tui/picker.go:311`),
`commentPreview` (`internal/tui/comment.go:130`), `issueWrite`
(`internal/tui/issuewrite.go:140`), `issueLinker`
(`internal/tui/issuelink.go:73`), `branchCreator`
(`internal/tui/branch.go:404`), `branchPicker`
(`internal/tui/switchtask.go:155`), `lastLook`
(`internal/tui/overlay.go:201`), `prComposer`
(`internal/tui/prcomposer.go:275`), `prEditor`
(`internal/tui/preditor.go:83`), `mergePicker` (`internal/tui/merge.go:178`),
`finishPreview` (`internal/tui/finish.go:105`), `messagingPreview`
(`internal/tui/messagingpreview.go:94`) and `hookgenOffer`
(`internal/tui/hookgen.go:118`). The failure-voice sites: `failureBlock` 9,
the six panes (`internal/tui/detail.go:339`, `internal/tui/branch.go:133`,
`internal/tui/commits.go:125`, `internal/tui/review.go:292`,
`internal/tui/reviewqueue.go:115`, and the Messaging pane's failed post at
`internal/tui/messaging.go:187`), the
configuration screen (`internal/tui/render.go:473`) and the two details
(`internal/tui/detail.go:377`, `internal/tui/review.go:321`);
`pinnedOutcome` 11 (`internal/tui/branch.go:326`,
`internal/tui/comment.go:112`, `internal/tui/composer.go:157`,
`internal/tui/finish.go:83`, `internal/tui/hookgen.go:82`,
`internal/tui/issuelink.go:53`, `internal/tui/merge.go:141`,
`internal/tui/messagingpreview.go:64`, `internal/tui/prcomposer.go:222`,
`internal/tui/overlay.go:176`, `internal/tui/preditor.go:58`); `failureLine`
13 (`internal/tui/checks.go:91`, `internal/tui/composer.go:166`, `:178`,
`internal/tui/diff.go:79`, `internal/tui/fields.go:171`,
`internal/tui/issuewrite.go:120`, `:122`, `internal/tui/merge.go:148`,
`internal/tui/picker.go:261`, `:289`, `internal/tui/run.go:236`,
`internal/tui/switchtask.go:108`, `:136`);
`failureSummary` 4 (`internal/tui/messaging.go:151`,
`internal/tui/review.go:231`, `:267`, `internal/tui/reviewqueue.go:99`);
`noticedFailure` 9 (`internal/tui/comment.go:75`, `internal/tui/checks.go:235`,
`internal/tui/commits.go:307`, `internal/tui/hookgen.go:175`,
`internal/tui/links.go:65`, `internal/tui/messaging.go:294`, `:317`,
`internal/tui/messagingpreview.go:206`, `internal/tui/run.go:373`) plus
`noticedFailureLedBy` 1 (`internal/tui/checks.go:227`); `noticedGuidance` 3
(`internal/tui/comment.go:77`, `internal/tui/composer.go:78`,
`internal/tui/messagingpreview.go:131`).

## The command line

What is open here is a slow command's silence, the flags the scriptable
commands lack and the JSON a script cannot join, time or get from `pr` and
`standup`, what `config init` claims and does, three moments that name no
next step, the body `pr` never shows, two fallbacks `doctor` has no row
for, and a reference with no example or exit status.

### UX-61 A slow command is silent while it works

Impact: low · Effort: medium

**Today.** No spinner, no elapsed time, no "checking…". `doctor --online`
makes three round trips in silence (`reportCredentials`,
`internal/cli/doctor_credentials.go:29`);
`standup` fires up to fifteen forge requests plus a Jira search
(`gatherPulls`, `internal/cli/standup.go:190`); `status DIR…` visits each directory in
series (`statusesOf`, `internal/cli/status.go:165`). The only trace is `--log`,
which outlines each request in a file for a bug report and shows the person
waiting nothing.

**Instead.** A one-line "checking Jira…" on stderr when stderr is a
terminal, replaced in place; nothing when it is not.

**Done when.** A test with a terminal-flagged stderr sees the line; one
without does not.

### UX-62 Flags the scriptable commands are missing

Impact: low · Effort: medium

**Today.** No command declares a single shorthand — there is no `VarP(`
call in `internal/cli` — so `-n`, `-y`, `-j` do not exist; `status` emits
`●◐✗○` (`statusGlyph`, `internal/cli/status.go:404`) with ASCII selectable only through
`ui.ascii` in the file, no `--plain`; `pr` has no
draft, base, reviewer, title or body flag; `announce` has no `--channel`
(the channel comes from `messaging.channel` alone); both `pr` and the web
take the first repository template only (`firstTemplate`, `internal/loop/pull.go:137`),
where the interface cycles them (`ctrl+t`).

**Instead.** Shorthands for the three common flags; `--plain` on `status`;
`--draft`, `--base`, `--reviewer`, `--channel`, `--template` where the seam
already carries the value. `--json` on `standup` and `pr` is UX-92's.

**Done when.** Each flag has a test that it reaches the seam.

### UX-90 Three claims about `config init` that the command does not keep

Impact: low · Effort: small

**Today.** The root help, the README and two doc pages describe a
`config init` that does not exist, and the command corrects them itself
while it runs.

- `internal/cli/cli.go:37` (`longHelp`): "Write a starting file with:
  workflow config init", then "fill in the two credentials" (`:41`). That
  is `--template`'s flow; bare `init` runs the guided one
  (`newConfigInitCmd`, `internal/cli/config_cmd.go:77`, branches on
  `opts.template` and otherwise calls `runGuidedInit`), so a reader who
  follows the root help is prompted instead. `TestHelpExplainsBothTokens`
  (`internal/cli/cli_test.go:295`) holds the help to the token steps and
  never to the flow.
- `README.md:47` (under "Status") and `docs/content/docs/install.md:83`
  and `:87` (under "First run"): "writes a starting configuration file",
  "# writes .workflow.json here", "Then fill in the two tokens", the same
  stale flow, while `docs/content/docs/configuration.md:36` (the
  "Configuration" intro) says it asks and checks.
- `internal/cli/cli.go:91` (the `SECURITY` paragraph of `longHelp`): the
  file "is listed in .gitignore". `warnIfNotIgnored`
  (`internal/cli/config_cmd.go:327`) only warns when it is not, and nothing
  writes a `.gitignore`; the sentence is true of this repository's own
  `.gitignore:34`, not the user's. `README.md:198` and
  `docs/content/docs/configuration.md:505` (both under "Keeping the tokens
  safe") say the same.
- `internal/cli/config_cmd.go:58` and `:59` (`newConfigInitCmd`'s `Short`
  and `Long`): "asking for and checking each credential", "check each
  one". `collectMessaging` prints "saved (a webhook cannot be checked
  without posting)" (`:308`), so the webhook is saved unchecked; the
  generated `docs/content/docs/reference/workflow_config_init.md:10`
  repeats the `Short`, and `docs/content/docs/configuration.md:36` says it
  "does the same for Slack".

**Instead.** Say what the command does: in `longHelp`, the README and
install.md, "Set it up, answering the prompts, with `workflow config init`
(`--template` writes a blank file to edit)"; in the security paragraphs,
"`config init` warns when the file is not ignored by git; add it to
`.gitignore`"; in `Short`, `Long` and configuration.md, "asking for each
credential and checking the Jira token", then `task docs:gen`.

**Done when.** `TestHelpExplainsBothTokens` also wants `--template` in the
root help; `grep -rn 'starting configuration file\|fill in the two\|listed
in .gitignore\|listed in the repository' README.md docs/content/docs
internal/cli/cli.go` finds nothing; `workflow config init --help` no
longer says every credential is checked, and `task docs:check` passes.

### UX-91 Three moments the command line names no next step

Impact: low · Effort: small

**Today.** The scriptable writes say what to do when stdin is closed:
"pass --yes", exit 2 (`writeOptions.proceed`,
`internal/cli/scriptable.go:93`, and `docs/content/docs/scripting.md:201`
under "Writing without a person"). Three other moments end without a
pointer.

- `internal/cli/config_cmd.go:244` (`collectJira`): the guided `init`,
  which asks four questions, returns `prompt.Line`'s error raw (`:246`).
  Only `confirm` maps `io.EOF` to `errNoTerminal`
  (`internal/cli/prompt.go:46`), and `io.EOF` belongs to no family in
  `exitFamilies` (`internal/cli/scriptable.go:307`), so
  `workflow config init < /dev/null` prints "workflow: EOF" and exits 1
  with no mention of `--template`. A final line typed without a newline
  comes back from `terminalPrompt`'s `ReadString` together with `io.EOF`
  (`cmd/workflow/main.go:44`) and is discarded with it.
- `internal/cli/cli.go:192` (`NewRootCmdOver`'s `--web` branch): every
  load error, `ErrNotFound` included, prints "configuration did not load
  cleanly: %v" and then serves (`:199`). Its siblings branch on
  `ErrNotFound` and print `NoConfigHeadline`, `InitStep` and `DoctorStep`:
  `showLoadError` (`internal/cli/config_cmd.go:97`), `reportLoadError`
  (`internal/cli/doctor.go:275`) and the interface's `configErrorStatus`
  (`internal/tui/render.go:466`). The web cannot write a first file
  (`docs/content/docs/web.md:162`, "What stays in the terminal"), so the
  one surface that most needs `workflow config init` named is the one that
  never names it.
- `internal/cli/pr.go:149` (`runPR`): the command ends with `followUp`, so
  "Moved PROJ-2 to In Review" is its last word, while the interface's
  spine keeps "nothing announced" in view
  (`docs/content/docs/usage.md:52`, "The screen") and `announce`'s own
  refusal points the other way, "(open one with workflow pr)"
  (`runAnnounce`, `internal/cli/announce.go:115`).

**Instead.** Map `io.EOF` from the guided flow's prompts to
`errNoTerminal` with "pass --template to write a file to edit by hand",
exiting 2 like the writes; in the `--web` branch, test
`config.ErrNotFound` as `showLoadError` does and print the three shared
hint constants before the serving line; when messaging is configured, end
`pr` with a stderr note "Announce it with workflow announce", so stdout
stays the artifact.

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

- `internal/cli/status.go:203` (`repoLabel`): the label is the base name,
  and "." is returned as itself (`:206`), so `status .` labels the row "."
  (`TestStatusAcrossLabelsTheCurrentDirectory`,
  `internal/cli/status_test.go:79`, pins that prefix) and `status ~/a/api
  ~/b/api` gives two rows the same `repository`, the only key
  `docs/content/docs/scripting.md:131` (under "JSON") offers.
- `internal/cli/reviews.go:130` (`reviewReport.Age`): a string, filled by
  `renderReviewsJSON` through `humanizeAge` (`:141`), which rounds 25 h
  and 47 h alike to "1d"; `ReviewRequest.OpenedAt`
  (`internal/forge/pulls.go:146`) holds the time and never reaches the
  JSON. `docs/content/docs/scripting.md:147` documents `"age": "3d"` as
  the shape, so `jq 'map(select(.age > "2d"))'` compares strings.
- `internal/cli/pr.go:145` (`runPR`): the only machine-facing result is
  "Opened " + `Sigil()` + number + URL, and the sigil is `!` on GitLab
  (`internal/forge/remote.go:70`). `docs/content/docs/scripting.md:100`
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
(`api/openapi.yaml:1242`), with the dry run printing the draft in the
same shape; and give `standup` a `--json` printing the gathered sections
in place of the draft.

**Done when.** `status .` in a repository labels the row by the
directory's name and `status a/api b/api --json` yields two distinct
`repository` values; `TestReviewsAsJSONReportsEachOldestFirstWithItsAge`
(`internal/cli/reviews_test.go:141`) also parses `opened_at` back into the
time it seeded; `workflow pr --yes --json | jq .pull.number` prints 7 in a
test, with the notes still on stderr; a `standup --json` test parses its
stdout as JSON holding the seeded issue key.

### UX-93 `pr` confirms a body the person never saw

Impact: medium · Effort: small

**Today.** `runPR` previews two lines, "Open TITLE" and "BRANCH → BASE"
(`internal/cli/pr.go:121`, `:122`), asks, and then sends `request.Body`
(`:140`), composed from the template, the commit subjects and the issue
link, without ever showing it; `newPRCmd`'s `Long` says "A preview is
confirmed first." (`:64`). The interface shows the body's first
`prBodyPreviewLines` in `prComposer.view`
(`internal/tui/prcomposer.go:235`), the web's `draftDTO` carries `Body`
for the form to show (`internal/webserver/pullrequest.go:161`), and the
command line's sibling writes print their whole payload (`runAnnounce`,
`internal/cli/announce.go:132`; `standup` at
`internal/cli/standup.go:141`). A stale or wrong template is discovered on
the forge, after the open, on the one surface whose help promises a last
look.
`docs/content/docs/scripting.md:91` (the `pr` row under "Standard output
and standard error") documents the two-line stdout, and no commit or doc
records a decision to omit the body.

**Instead.** Print the body under the two header lines on stdout, where
the artifact goes, so `workflow --dry-run pr` shows the whole pull request
and the question is asked about what was shown; update the `pr` row in
scripting.md.

**Done when.** `TestPRDryRunPreviewsWithoutOpening`
(`internal/cli/pr_test.go:97`), with a template written as
`TestPRPushesThenOpensAnUnpublishedBranch` (`:248`) writes one, sees the
template's text on stdout.

### UX-94 doctor has no row for two silent fallbacks: the store, glab

Impact: low · Effort: small

**Today.** Two fallbacks happen without a word, and `doctor`, the command
that explains the machine, has no row for either.

- `internal/store/dir.go:15` (`DefaultDir`): `home, _ :=
  os.UserHomeDir()` drops the error, so an empty home reaches `Dir`
  (`:24`), which returns `ErrNoDir` for it.
- `internal/wiring/wiring.go:280` (`storeDeps`): `dir, _ :=
  store.DefaultDir()` drops `ErrNoDir`, and the store is built on an empty
  directory; the doc comment above (`:275`) calls the no-op intended, "the
  interface simply learns nothing", which is what makes this a
  discoverability gap rather than a defect.
- `internal/store/store.go:135` (`Store.off`): `s.disabled || s.dir ==
  ""`, so every seam no-ops on the empty directory with no signal outward.
  On a home-less machine the interface opens with no seeded issue list and
  forgets the last scope and every announcement, while `store.disabled` is
  still false.
- `internal/cli/doctor_requirements.go:122` (`externalTools`): the tool
  list is git, lefthook and gh; no doctor file mentions the store, and
  `glab` is absent.
- `internal/wiring/forgecli.go:33` (`forgeTransport`): `if !ok ||
  !proc.Available(program)` falls back to plain HTTP with no note that
  `forge.cli` was set and ignored, and `forgeProgram` names `glab` for
  GitLab (`:56`), the program doctor never looks for. A GitLab user who
  sets `forge.cli` and forgets to install glab has every forge call go
  over HTTP with nothing saying why SSO still blocks it.

`doctor --online` already names `gh` or `glab` as the forge credential's
source when `forge.cli` routes the forge through it; this one adds the
Tooling row that says the program is absent. glab's line in install.md's
needs list is DEBT-91's.

**Instead.** A doctor row that prints the store's directory, or
`ErrNoDir`'s sentence when there is none; and glab in `externalTools` with
the effect "routes GitLab calls when forge.cli is set".

**Done when.** `workflow doctor` with `HOME` and `XDG_STATE_HOME` unset
prints a line naming the store and that no data directory could be
determined; `workflow doctor` prints a glab line under Tooling.

### UX-95 The reference shows no example and never names an exit status

Impact: low · Effort: small

**Today.** clig.dev asks help to lead with examples. No command in
`internal/cli` sets cobra's `Example` (`grep -l Example internal/cli/*.go`,
tests excluded, matches no file), so no generated reference page and no
`--help` carries an Examples section, and no page under
`docs/content/docs/reference` names an exit status (`grep -ril exit` over
the directory matches nothing).

- `docs/content/docs/reference/workflow.md:26` and `:30` (the root
  "Synopsis"): `workflow config init` and `workflow doctor`, the only
  invocations a reference page shows, as prose inside the root synopsis.
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
- `docs/content/docs/reference/workflow_config_show.md:17` (its
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
delay; undo; vim's missing keys; sentences that name a rebindable key; the
in-flight mark on four panes and three searches; two second-path acts; the
two loose applications of the visual system (UX-100); two forge fields that
stop before the screen; and a reviewers completion tab cannot take.

### UX-64 A screen reader, an alternate screen you cannot turn off, and a delay you cannot tune

Impact: low · Effort: medium

**Today.** `NO_COLOR` and `ui.color: never` keep bold and faint
(`internal/tui/tui.go:129`); `ui.ascii` swaps glyphs and borders
(`internal/tui/glyphs.go:43`); escapes in server text are neutralized. But
there is no screen-reader mode; the alternate screen is unconditional
(`internal/tui/render.go:38` `view.AltScreen = true`), so nothing the
interface prints survives quitting; `ui.color` has no `always` for a piped
terminal that does support color; and the 150 ms detail delay
(`internal/tui/detail.go:22`) is fixed.

**Instead.** `ui.alt_screen: false` for inline rendering; `ui.color:
always`, which `validateUI` (`internal/config/ui.go:78`) and the
`UIConfig.color` enum (`api/openapi.yaml:1466`) refuse until they list it;
`ui.detail_delay` in milliseconds.

**Done when.** Each setting is read and honored by a screen test.

### UX-68 Nothing can be undone

Impact: low · Effort: large

**Today.** Drafts survive `esc` (commit `internal/tui/composer.go:254`, pull request
`internal/tui/prcomposer.go:278`), a dirty tree blocks a switch instead of stashing, and
quit is guarded while an announcement waits. But a posted comment, an applied
transition, a merge and the `branch -D` in finish have no undo, and the
interface never says which acts are reversible.

**Instead.** Short of undo: the finish preview says "deletes NAME; the
commits stay reachable from BASE"; a comment's success notice carries its
URL so it can be edited where it lives.

**Done when.** Each irreversible act's preview or notice says so.

### UX-69 Vim habits stop at `j`/`k`

Impact: low · Effort: small

**Today.** `up/k`, `down/j`, `pgup/K`, `pgdn/J` (`internal/tui/keys.go:204`). No `h`/`l`
(`←`/`→` are cycle-type and cycle-channel only), no `g`/`G` to jump to the
ends of a list or the detail.

**Instead.** `g`/`G` on the lists and the detail; `h`/`l` where a pane has a
horizontal axis.

**Done when.** `G` on the Issues list selects the last loaded issue.

### UX-96 Eleven sentences name a key that `ui.keys` can move

Impact: low · Effort: small

**Today.** `ui.keys` moves an action to another key and the help follows
it — "The help then shows the new key", the `ui.keys` row of
`docs/content/docs/configuration.md:82` — but eleven sentences carry the
default key as a literal, so a rebound user is told to press a key that
does something else or nothing. `wording` (`internal/tui/failure.go:73`)
states the design the first of them breaks: the full form names no key,
and each surface's footer offers its own. Counted by reading every string
literal in `internal/tui/*.go` (tests excluded) that names a key and
checking that key's action is bound through `helpBuilder.bind`; no single
grep finds all eleven. DEBT-115 points here for the nothing-staged sentence.

- `internal/tui/detail.go:377` `Model.fullDetail` appends "press r to try
  again" after the failure block, while refresh is rebindable
  (`internal/tui/keys.go:222`, `issueKeys`): with `{"refresh": "ctrl+l"}`
  `r` does nothing there.
- `internal/tui/branch.go:135` `Model.branchDetail` says "Check out a
  branch, or press b to start one" on a detached HEAD; new-branch is
  rebindable (`internal/tui/keys.go:227`, `branchAndCommitKeys`).
- `internal/tui/branch.go:339` `branchCreator.view` says "could not fetch;
  enter branches from what you already have"; apply is rebindable
  (`internal/tui/keys.go:279`, `everywhereKeys`), and the creator's own
  footer already reads it from the binding —
  `internal/tui/branch.go:383` `branchCreator.footer` relabels
  `keys.confirm` to "branch from what you have", so a rebound session
  shows "ctrl+s branch from what you have" under a sentence that says
  enter.
- `internal/tui/commits.go:143` `Model.commitsDetail` says "Press g to set
  up lefthook" under a footer that reads its key from `keys.hookConfig`;
  set-up-lefthook is rebindable (`internal/tui/keys.go:237`,
  `branchAndCommitKeys`).
- `internal/tui/review.go:296` `Model.reviewDetail` says "n opens one from
  this branch's commits and the repository's template."; open-pull-request
  is rebindable (`internal/tui/keys.go:242`, `reviewAndMessagingKeys`), and
  the pane answers `m.keys.newPullRequest`.
- `internal/tui/review.go:325` `Model.reviewDetail` says "e edits its title
  and description."; edit is rebindable (`internal/tui/keys.go:255`,
  `composerKeys`), and the pane answers `m.keys.edit`.
- `internal/tui/finish.go:27` `Model.mergedDetail` says "F finishes the
  branch: …"; finish-branch is rebindable (`internal/tui/keys.go:246`,
  `reviewAndMessagingKeys`).
- `internal/tui/finish.go:31` `Model.mergedDetail` says "n opens a new
  pull request from this branch's commits." once one has merged;
  open-pull-request is rebindable (`internal/tui/keys.go:242`,
  `reviewAndMessagingKeys`), and the pane answers `m.keys.newPullRequest`.
- `internal/tui/failure.go:127` `localErrors` words `loop.ErrNothingStaged`
  in full as "nothing is staged: space stages the selected file" — the
  very full form `wording` says names no key; stage is rebindable
  (`internal/tui/keys.go:231`, `branchAndCommitKeys`).
- `internal/tui/checks.go:55` `checkList.view` says "Open a check's page
  with enter."; `checkList.handleKey` (`internal/tui/checks.go:113`) opens
  on `m.keys.confirm`, the rebindable apply.
- `internal/tui/composer.go:227` `commitComposer.footnotes` says "no body
  yet: ctrl+o writes one in your editor"; `commitComposer.handleKey`
  (`internal/tui/composer.go:259`) answers `m.keys.editBody`, and edit-body
  is rebindable (`internal/tui/keys.go:256`, `composerKeys`).

**Instead.** Build each sentence from the binding — `m.keys.refresh`,
`m.keys.newBranch`, `m.keys.confirm`, `m.keys.hookConfig`,
`m.keys.newPullRequest`, `m.keys.edit`, `m.keys.finish`, `m.keys.stage`,
`m.keys.editBody`, through `Help().Key` — or drop the key from the
sentence and let the footer beside it carry the offer, as `wording`
intends.

**Done when.** A screen test that rebinds refresh, new-branch, apply,
set-up-lefthook, open-pull-request, edit, finish-branch, stage and
edit-body through `cfg.UI.Keys`, as `internal/tui/issuesfooter_test.go:180`
does for apply and close, sees the bound key, or no key, in each of the
eleven sentences and never the literal `r`, `b`, `enter`, `g`, `n`, `e`,
`F`, `space` or `ctrl+o` there; the detached case of
`TestTheBranchPaneSaysWhereTheBranchStands`
(`internal/tui/branch_test.go:79`),
`TestAFailedFetchOffersToBranchFromWhatIsThere`
(`internal/tui/fetch_test.go:35`), the screens that pin the Review
detail's two sentences (`internal/tui/review_test.go:146`,
`internal/tui/preditor_test.go:24`) and the merged detail's
(`TestNIsOfferedWhileNoPullRequestIsOpen`,
`internal/tui/review_offer_test.go`) gain the rebound variant.

### UX-97 The in-flight mark is missing on four panes and three searches

Impact: low · Effort: small

**Today.** `Model.paneTitle` (`internal/tui/render.go:183`) promises the
in-flight glyph "until the answer arrives", but `Model.loading`
(`internal/tui/render.go:192`) answers only for `paneIssues` and only
`issueList` carries a `loading` flag (`internal/tui/issues.go:79`), so `r`
on Branch, Commits, Review and Reviews changes nothing on screen until the
answer lands; and three Issues searches start without setting the flag, so
the one pane that has the glyph omits it for them. On a slow forge,
pressing `r` on the Review pane looks ignored until `CheckStatus` answers;
after "● PROJ-412 is now Done" the list quietly re-sorts some time later,
where `r` would show "1 Issues ◐" for the same wait; `v` onto a view the
store remembers shows yesterday's list with no sign today's is on its way,
and a session that opens on the cache does the same while `Init`'s search
runs. Refresh is live in four of the contexts `keyContexts`
(`internal/tui/keys.go:344`) lists, five panes in all, and the glyph is
applied to one.

- `internal/tui/render.go:192` `Model.loading` returns `m.issues.loading`
  for `paneIssues` and false for every other pane.
- `internal/tui/review.go:444` `Model.handleReviewKey` reads the branch
  on refresh, which goes on to `findPullRequest` and `checkCI`, and sets
  no flag the title can read.
- `internal/tui/branch.go:258` `Model.handleBranchKey` batches
  `loadBranch` and `loadChanges` with no loading flag.
- `internal/tui/commits.go:227` `Model.handleCommitsKey` batches
  `loadChanges` and `loadBranch` with no loading flag.
- `internal/tui/reviewqueue.go:192` `Model.handleReviewQueueKey` starts
  `loadReviewQueue` with no loading flag.
- `internal/tui/picker.go:162` `transitionApplied.apply` batches
  `searchIssues` without setting `m.issues.loading`, unlike
  `Model.refreshIssues` (`internal/tui/issuekeys.go:153`); since the flag
  is clear, `Model.loadMoreIssues` (`internal/tui/detail.go:106`) is not
  held back either, so `ctrl+n` starts a second page while the search is
  still out.
- `internal/tui/views.go:54` `Model.nextIssueView` sets `loading` only
  when the seeded list is not settled, though `searchIssues` always
  follows at `internal/tui/views.go:58`; the seeded arm has never run
  under a test (`tmp/audit/cover-branch.log:269`: "was 2 times true but
  never false").
- `internal/tui/tui.go:110` `New` seeds `model.issues` from the cache with
  no loading flag while `Model.Init` (`internal/tui/tui.go:154`) always
  starts `searchIssues` — the same gap at startup.

**Instead.** Give each refreshable state — branch, changes, review, review
queue — a loading flag set where its refresh command is built and cleared
by its applier, and let `loading` read it per pane through the behavior
table; set `m.issues.loading = true` unconditionally after seeding in
`nextIssueView` and `New`, and before batching the search in
`transitionApplied.apply`, since a search is always started.

**Done when.** A test whose `CheckStatus` never answers within the horizon
presses `4`, `r` and sees "Review ◐" in the title, and the same for `2`,
`3` and `6` with their seams; `TestEnterMovesTheIssueAndRefreshesTheList`
(`internal/tui/picker_apply_test.go:15`) requires "1 Issues ◐" between the
move and the refresh's answer; a views test with `CachedIssues` for the
second view shows "◐" after `v` until the search answers, and a `New` with
a seeded cache shows it until `Init`'s search answers.

### UX-99 The worktree and review-status paths give less than their twins

Impact: low · Effort: small

**Today.** Two acts reached by a second path come back poorer than by the
first. `b` then enter on a To Do issue offers "Change status ▸ ◐ Start";
`b`, `ctrl+w`, enter on the same issue does not, though the work has just
as surely started. After opening a pull request the Change status overlay
reads "PROJ-412 " and "status  " with empty values, unlike the same
picker opened by `t`, though the list usually holds the issue.

- `internal/tui/branchresult.go:99` `worktreeCreated.apply` closes with
  "worktree for NAME at PATH" and makes no status offer, where
  `branchCreated.apply` (`internal/tui/branchresult.go:85`) calls
  `pickStatusFor(msg.issue, statusOffer{inProgress: true})`.
- `internal/tui/branchresult.go:91` `worktreeCreated` carries `name`,
  `path` and `err` only — no `issue` or `forIssue` to offer from.
- `internal/tui/picker.go:238` `Model.offerReviewStatus` passes
  `jira.Issue{Key: issueKey}` with an empty summary and status, where
  `Model.openStatusPicker` (`internal/tui/picker.go:207`) passes the
  listed issue.
- `internal/tui/picker.go:249` `statusPicker.header` draws the key, a
  space and the summary, then "status  " and the status — both empty on
  that path.

**Instead.** Carry `issue` and `forIssue` on `worktreeCreated` as
`branchCreated` does and make the same offer; in `offerReviewStatus` look
the key up with `issueList.find` (`internal/tui/issues.go:227`) before
opening, falling back to the bare key only when it is not listed.

**Done when.** A case in `internal/tui/statusafterbranch_test.go` that
creates a worktree for PROJ-388 shows the Change status overlay
pre-selected on Start; `TestLinkingAPullRequestThenOffersTheReviewStatus`
(`internal/tui/issuelink_test.go:27`) also requires the issue's summary
and "status  In Progress" in the overlay.

### UX-100 The checkbox and the diff bend the shape and hue rules

Impact: low · Effort: small

**Today.** Two places apply the settled system loosely; this is its
application, not a change to it. In the Fix Version/s step "● 1.0" means
chosen, and on the screen before it "● Done" meant a done status, so a
reader who learned the glyph vocabulary reads the checkbox as a state. A
red row in the Commits detail no longer reliably means something broke,
and a green row is not the forge's, though the `+` and `-` git leaves in
place already carry the state by shape (`Model.diffSection`,
`internal/tui/diff.go:65`: "An added line keeps its leading + and a
removed line its -, so the mark reads without color too"). The diff
exception is written down in the code and not in this file.

- `internal/tui/glyphs.go:74` `glyphs.checkbox` returns `g.done` for
  chosen and `g.notStarted` for unchosen — the fields `glyphs.status`
  (`internal/tui/glyphs.go:56`) maps `CategoryDone` and `CategoryNew` to.
- `internal/tui/fields.go:187` `fieldForm.optionLines` draws that checkbox
  beside each option of the overlay whose transition rows
  (`statusPicker.transitionRow`, `internal/tui/picker.go:277`) use
  `status` one keystroke earlier.
- `internal/tui/diff.go:111` `Model.markDiffLine` draws an added line in
  `styles.forge` — the forge's hue for a thing that is not the forge's.
- `internal/tui/diff.go:113` `Model.markDiffLine` draws a removed line in
  `styles.failure` — red for something that did not break.
- `internal/tui/glyphs.go:89` `styles` records "the diff preview reuses it
  where red instead means a removed line" — the exception, written down
  in the code and nowhere else.
- This file's "The visual system" section: names the diff as a loose
  application, never as the exception the code declares.

**Instead.** Give the checkbox its own shape pair in both glyph sets
(`[x]`/`[ ]` in ASCII, `☑`/`☐` in Unicode), keeping `○ ◐ ● ✗` for status
alone; draw removed lines faint (`styles.label`) and added lines plain,
with the `+` and `-` as the mark — or name the diff as the one documented
exception in this file's visual-system paragraph.

**Done when.** `TestAChosenVersionCanBeToggledOff`
(`internal/tui/fields_test.go:121`) asserts a checkbox glyph that is
neither `●` nor `○`; a color test asserts no `\x1b[31m` or `\x1b[32m` in a
Commits detail whose diff has removed and added lines, or the
visual-system paragraph names the diff as the exception.

### UX-101 Two fields the forge already returns never reach the screen

Impact: low · Effort: small

**Today.** Two values the forge client already reads stop before the
terminal. Without Jira, the Issues pane lists GitHub or GitLab issues but
the footer never offers to open or copy one, while the same keys work on
every Jira issue and on the Review and Reviews panes. A draft asking for
review looks like a ready one in the terminal's queue, is marked "· Draft"
in the browser, and the Review pane labels the branch's own draft
(`Model.reviewDetail`, `internal/tui/review.go:316`). The first site is a
wiring file; what it costs is the terminal's Issues pane.

- `internal/wiring/forgeissues.go:55` `forgeIssuesDeps` builds
  `tui.JiraDeps` with `Search`, `Issue`, `Transitions` and `Transition`
  only — no `BrowseURL`, though `forge.Issue` already carries `URL`
  (`internal/forge/issues.go:17`); `Model.issueURL`
  (`internal/tui/detail.go:266`) then returns "" and `Model.linkKeys`
  (`internal/tui/links.go:13`) offers nothing.
- `internal/tui/reviewqueue.go:142` `Model.reviewTail` draws "by ", the
  author, the separator and the age — never `Draft`, which
  `forge.ReviewRequest` carries (`internal/forge/pulls.go:141`) and
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:197` prints as
  "· Draft".

**Instead.** Wire `BrowseURL` in `forgeIssuesDeps` from the listed issue's
`URL`, kept per key from the last search; append "draft" to `reviewTail`
when the request is one, as `reviewDetail` labels the branch's own.

**Done when.** A wiring test with no Jira configured returns a non-empty
`BrowseURL` for a listed forge issue and the Issues footer offers "o
open"; a test with a draft review request sees "draft" on its row in pane
`6`.

### UX-102 The reviewers field shows a completion tab cannot take

Impact: low · Effort: small

**Today.** `prComposer.withReviewerSuggestions`
(`internal/tui/prcomposer.go:168`) promises the CODEOWNERS handles as the
reviewers field's "hint and completions" (`internal/tui/prcomposer.go:166`)
and turns on `ShowSuggestions` (`internal/tui/prcomposer.go:180`), so
bubbles v2.2.1's text input draws the match as ghost text and would accept
it on tab; but `prComposer.onFieldNav` (`internal/tui/prcomposer.go:304`)
completes only the base field, through `baseCanComplete`, and moves tab on
from every other field. Type `a` in reviewers with CODEOWNERS ana, ben:
"na" appears as a completion, tab jumps to assignees and leaves `a`; and
since the text input's `updateSuggestions` matches the whole value as a
prefix of a suggestion, nothing completes after "ana, ".

**Instead.** Complete the reviewers field on tab as the base is — a
`reviewersCanComplete` beside `baseCanComplete`, matching the segment after
the last comma — or keep the placeholder alone and drop `ShowSuggestions`
so the ghost text stops promising.

**Done when.** A test types `a` then tab in reviewers with code owners ana
and ben and sees "reviewers > ana".

## The web

What is open here is a stream that marks no change; Settings' unseen
sections, misleading hints, blank selects, failed read and unremovable
credential; what the browser could borrow from the interface; the Branch
section's push, the CI counts and the detached HEAD; keyboard focus,
placeholder, field-boundary and focus-ring contrast; a staged state drawn
as a colored word; one primary button in three sizes, type and measure
outside the scale; ragged rows, an empty form's heading and a far-off Copy
URL confirm; a fixed port; links, stages and controls that tell less than
their siblings; and copy settled site by site.

### UX-86 Nothing marks a change the stream just made

Impact: low · Effort: medium

**Today.** `StreamStatus` shows `Connecting` / `Live` / `Reconnecting` /
`Out of date` (`web/src/shell/StreamStatus.tsx:7`), but nothing says when
the last snapshot arrived, and a panel that changed
because CI settled looks exactly like one that re-rendered. There is no
toast, and no "CI passed" moment on the web where the interface rings the
terminal (`internal/tui/review.go:155`).

**Instead.** A "updated 3 s ago" beside the pill; a brief highlight on the
row a snapshot changed; a status line when CI settles, honoring
`ui.notify`.

**Done when.** A snapshot that flips CI to passed produces a status
region saying so.

### UX-87 Settings can edit seven sections and carry five it cannot show

Impact: low · Effort: medium

**Today.** The form seeds itself with the whole `Config`
(`web/src/features/settings/SettingsPanel.tsx:66`) so `ui`, `timing`, `headers`, `views` and
`branch.prefixes` survive a save unchanged — and cannot be edited. There is
no guided, credential-checking flow like `workflow config init`; the web
edits an existing file only.

**Instead.** Fieldsets for the five, with `views` and `prefixes` as
editable lists; a first-run flow that checks the Jira token as `config
init` does.

**Done when.** A view added in the browser appears in the interface's `v`
cycle.

### UX-88 What the browser could borrow from the interface

Impact: low · Effort: medium

**Today.** The interface shows a per-file diff under the changes list
(`internal/tui/diff.go:29`), amends (`A`) and fixups (`f`), jumps to
a failure in `$EDITOR` (`internal/tui/run.go:354`), edits an open pull
request (`internal/tui/preditor.go`), and cycles the repository's pull-request templates
(`ctrl+t`). None has a web equivalent, and the web takes the first template
only (`firstTemplate`, `internal/loop/pull.go:137`). A `?` shortcut sheet, which the interface has,
would give the web's six sections keyboard reach. Five smaller things the
terminal shows are absent on the web too, none of them among what
`docs/content/docs/web.md` says stays in the terminal.

- A renamed file shows only its new path: `ChangeRow` renders
  `change.path` alone (`web/src/features/branch/WorkingTree.tsx:91`), though
  `changesDTO` sends `OriginalPath` (`internal/webserver/dto.go:122`) and
  `original_path` is read nowhere in `web/src` outside the generated
  client; the terminal's `changeRows` draws old → new
  (`internal/tui/commits.go:159`).
- The commit subject has no length against `commit.subject_limit`:
  `MessageFields` is a bare `<input required placeholder>`
  (`web/src/features/branch/CommitForm.tsx:183`) though the form already
  reads `useConfig` and `subject_limit` is on the wire (`CommitConfig`,
  `web/src/api/generated/types.gen.ts:646`); the terminal's
  `commitComposer.view` shows "n/limit" as typed
  (`internal/tui/composer.go:151`). The limit is met only as a 422 after
  the click.
- The pull request form does not suggest CODEOWNERS reviewers:
  `PullRequestForm` seeds `reviewers: ''`
  (`web/src/features/review/ReviewPanel.tsx:247`) and `ProposalFields`
  shows a generic placeholder (`:340`), where the terminal's
  `openPullRequestComposer` calls `withReviewerSuggestions` with
  `CodeOwners` (`internal/tui/prcomposer.go:130`) and the owners become
  the placeholder (`:178`); `PullRequestDraft` carries no reviewer field
  (`api/openapi.yaml:892`).
- The Review section has no Copy URL: the title link is the only handle on
  the pull request (`PullRequestSummary`,
  `web/src/features/review/ReviewPanel.tsx:86`), where the queue's
  `CopyURL` one section over has a tested clipboard outcome
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:269`) and the
  terminal's `reviewKeys` offer `linkKeys` on the pull request
  (`internal/tui/review.go:418`).
- The Filter text survives a view change: `IssueBrowser` keeps `filter` in
  local state nothing resets
  (`web/src/features/issues/IssuesPanel.tsx:38`) and `ViewSelect`'s
  `onChange` calls `setView` alone
  (`web/src/features/issues/IssueListControls.tsx:60`), where the
  terminal's `nextIssueView` drops the old view's filter with the view
  (`internal/tui/views.go:44`); choosing a view with "proj-12" still in the
  filter shows "No loaded issue matches the filter." — the user asked for
  a view, not a narrowed one.

An Unstage all is missing on the web as in the terminal — `WorkingTree`
renders `StageAll` alone (`web/src/features/branch/WorkingTree.tsx:31`) and
the staging module has no unstage-all
(`web/src/features/branch/stagingApi.ts:33`) — so it is FEAT-23's, not an
idea to borrow. A Push branch offered on the base branch is UX-104's.

**Instead.** In rough order of value: a template select on the pull-request
form; a diff view; `?`; `original_path` drawn before the path with the
middle dot or an arrow; a muted "n/limit" hint under the subject counting
the assembled header; `suggested_reviewers` on `PullRequestDraft` from the
CodeOwners seam as the field's placeholder and datalist; `CopyURL` beside
the title with the same "Copied the URL of #128." outcome; clearing the
filter when the view changes.

**Done when.** Each lands with a role/name test; the template select shows
every repository template; a `WorkingTree` test with a renamed change finds
both paths in the row; a `CommitForm` test with `subject_limit: 20` finds
the count text change as the subject is typed; with a CODEOWNERS naming two
handles the Reviewers input's placeholder lists them; a `ReviewPanel` test
clicks "Copy URL to #128" and reads the URL back from the clipboard; a test
types a filter, selects another view, and finds the searchbox named Filter
empty once the new frame lands.

### UX-104 The Branch section's push counts what it cannot know, even on the base

Impact: medium · Effort: small

**Today.** Three lines in the Branch section print a number git counted
against nothing, and the push is offered where the terminal withholds it.
The push confirm is a last look, which the promises table
holds to naming what is about to happen, and it is the one that misleads;
the two rows above it share the defect.

- `PushConfirm` asks "Push {commits} commit(s) to the remote?"
  (`web/src/features/branch/BranchPanel.tsx:177`), fed
  `branch.commits.length` by `PushButton` (`:126`): the commits since the
  base, which `BranchSummary`'s own comment calls unknowable without a base
  (`:51`) and which `canPush` therefore ignores (`:52`). On a branch with
  no base the confirm reads "Push 0 commit(s)" and the push goes ahead;
  four commits since main and one ahead reads "Push 4"; and "the remote"
  never says which.
- A long branch reads the cap: `Branch.Truncated`
  (`internal/gitrepo/branch.go:78`) says the list is the oldest of a longer
  history, and the wire `Branch` carries no such flag.
- The server decides by the upstream on the push remote and ahead alone
  (`nothingToPush`, `internal/webserver/push.go:59`), so the count the
  confirm shows is not what the server checks.
- `BranchSummary` prints "{ahead} ahead, {behind} behind" whatever the
  upstream (`web/src/features/branch/BranchPanel.tsx:69`), so an
  unpublished branch reads "Upstream none / Tracking 0 ahead, 0 behind"
  over a Push branch button: the numbers say there is nothing to push and
  the button says there is.
- `Commits` says "No commits yet on this branch." whenever the list is
  empty (`web/src/features/branch/BranchPanel.tsx:85`), a base of `""`
  included, where the count is unknown.
- Push branch is offered on the base branch itself: `BranchSummary`'s
  `canPush` needs a name and no upstream on the push remote or ahead > 0
  (`web/src/features/branch/BranchPanel.tsx:53`), and `nothingToPush`
  accepts main ahead of origin/main (`internal/webserver/push.go:59`),
  where the terminal's `canPush` also requires `onFeatureBranch()`
  (`internal/tui/branch.go:208`).

The terminal's last look names the branch and the remote and no count
(`previewPush`, `internal/tui/branch.go:216`), and its `upstreamState`
says "not pushed yet" for a branch with no upstream (`:103`).

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
`web/src/features/issues/WorkStory.tsx:162`) — and for state none it prints
"CI none" where the terminal says "no checks reported". A GitLab user
sees a heading that contradicts the row beneath it; a GitHub user whose
checks have not started sees "0 of 0 done" over nothing and cannot tell
whether CI has not started, is not configured, or failed to load.

- `PullRequestSummary` prints "· {ci.done} of {ci.total} done"
  unconditionally (`web/src/features/review/ReviewPanel.tsx:112`) and
  renders `ci.state` nowhere; the section draws whenever `ci` is non-null
  (`:107`), so state none with no checks is the heading over an empty
  list.
- `gitlabStatus` returns Total 0, Done 0, Failed 0 with one pipeline check
  for every GitLab pipeline (`internal/forge/gitlab.go:391`), and `CINone`
  with no checks when there is no head pipeline (`:383`); the `CI` type
  documents that the counts stay zero there (`internal/forge/ci.go:32`),
  and `ciTally.ci` yields `CINone` with Total 0 for a GitHub pull with no
  statuses or check runs (`:111`).
- `ciDTO` copies State, Total, Done and Failed through unchanged
  (`internal/webserver/dto.go:173`).
- The terminal's `ciSummary` adds "(done of total finished)" only when
  Total is positive (`internal/tui/review.go:242`) and says "no checks
  reported" for `CINone` (`:235`).

**Instead.** Print the count only when total is positive and otherwise the
state word ("CI checks · running"), as `ciSummary` does; when `checks` is
empty, replace the list with "No checks reported." beside the unknown
mark.

**Done when.** A `ReviewPanel` test with total 0 and one running pipeline
check finds the heading "CI checks · running" and no "0 of 0"; one with
state none and no checks finds "No checks reported" and no list role.

### UX-106 Fourteen buttons let keyboard focus fall to the page

Impact: low · Effort: small

**Today.** Fourteen controls set native `disabled` while their request
runs, which drops focus in Chromium and WebKit, and nothing re-takes it. A
keyboard user who is refused — a dirty tree, a branch that already exists,
the dry-run hold on every Start work press — hears the reason and is left
at the top of the page. `useAsyncAction`'s catch sets the error and the
state only (`web/src/lib/useAsyncAction.ts:51`), so `onDone` never runs on
a refusal and the panel's `OutcomeLine` has nothing to follow. Counted
from `grep -rn "disabled={" web/src --include='*.tsx'`, tests and
`aria-disabled` excluded: 19 sites, of which 14 stay mounted after a
refusal with no code that re-takes focus. The other five are the push
(`PushButton` re-focuses its opener on an error,
`web/src/features/branch/BranchPanel.tsx:116`), the announce preview's
three (`handBack()` runs before `post.run()`,
`web/src/features/messaging/MessagingPanel.tsx:173`), and the pull request
form's Cancel (`web/src/features/review/ReviewPanel.tsx:282`), which is
not the control that had focus.

- The list row's `RowCheckout` is `disabled={state === 'running'}`
  (`web/src/features/issues/IssuesPanel.tsx:410`) and re-enables beside a
  `role="alert"` nothing re-focuses (`:419`).
- The story's `CheckoutButton` (`web/src/features/issues/WorkStory.tsx:263`,
  alert at `:272`) and `StartWorkButton` (`:295`, alert at `:304`) do the
  same; the latter is the path every dry-run press takes.
- The issue detail's Retry is `disabled={retrying}` (`IssueUnread`,
  `web/src/features/issues/IssueDetailPanel.tsx:81`), and
  `IssueDetailPanel` hands focus to the heading only when `data && !error`
  (`:29`), so a second refusal takes no branch.
- Settings' Save is `disabled={state === 'running'}` (`SaveControls`,
  `web/src/features/settings/SettingsPanel.tsx:150`) and reports "Saved."
  through its own `role="status"` span (`:155`), a second state machine
  beside the shared `OutcomeLine`, so a mouse-clicked Save disables itself
  under focus and the span does not take it.
- Settings' Retry is `disabled={query.isFetching}` (`SettingsPanel`,
  `web/src/features/settings/SettingsPanel.tsx:40`), its message in an
  `EmptyState` rather than an alert, and the changed-since-read Reload
  (`ChangedSinceRead`, `:182`, alert at `:191`) the same.
- The working tree's `ChangeRow` (`web/src/features/branch/WorkingTree.tsx:98`,
  alert at `:109`) and `StageAll` (`:132`, alert at `:142`).
- `CommitForm`'s submit (`web/src/features/branch/CommitForm.tsx:107`,
  alert at `:115`).
- `OpenPullRequest`'s compose button
  (`web/src/features/review/ReviewPanel.tsx:197`, alert at `:206`), whose
  `handBack()` runs only on the form's cancel, and `PullRequestForm`'s
  submit (`:290`, alert at `:298`), which stays up on a refused open.
- `AnnounceControls`' preview button
  (`web/src/features/messaging/MessagingPanel.tsx:187`, alert at `:197`).
- `FollowUpOffer`'s button (`web/src/features/review/OpenedOutcome.tsx:84`,
  alert at `:95`).

The project already knows the rule: Load more holds with `aria-disabled`
(`MoreIssues`, `web/src/features/issues/IssuesPanel.tsx:302`), so does the
review queue's Retry (`Queue`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:110`), and
`canHoldFocus` treats a disabled control as unable to hold focus
(`web/src/lib/Outcome.tsx:99`).

**Instead.** Hold each running button with `aria-disabled` (guarding
`onClick` as `Queue`'s Retry does) so it keeps focus through a refusal, and
say Settings' save result through `useOutcome` and `OutcomeLine` as the
other panels do, keeping the changed-since-read alert separate.

**Done when.** A test presses each of the fourteen buttons against a refused
request and finds `document.activeElement` still on the button once the
refusal is shown; a `SettingsPanel` test clicks Save with the mouse, awaits
"Saved.", and finds `document.activeElement` on the status line, not
`document.body`.

### UX-107 The web's detached HEAD offers no way out

Impact: low · Effort: small

**Today.** With HEAD detached, the Branch section heads itself "Detached
HEAD at abcdef1" and then draws the same Base, Upstream and Tracking list,
Commits and working tree as on a branch, with nothing saying what to do
next. The terminal says it: "Check out a branch, or press b to start one
for the selected issue."

- `BranchSummary`, `web/src/features/branch/BranchPanel.tsx:48`: the
  detached heading, followed by the branch's own list (`:62`).
- `BranchPanel`, `web/src/features/branch/BranchPanel.tsx:34`: renders
  `BranchSummary`, `Commits` and `WorkingTree` alike for a branch and a
  detached HEAD.
- `Model.branchDetail`, `internal/tui/branch.go:135`: the terminal's
  sentence (whose literal `b` is UX-96's).

**Instead.** Under the detached heading, one sentence pointing at Issues,
where Check out and Start work live.

**Done when.** A `BranchPanel` test with `detached: true` finds a line
naming Issues under the heading.

### UX-108 Four Settings hints that hide where a value comes from or applies

Impact: low · Effort: small

**Today.** Four Settings fields say nothing, or the wrong thing, about
their value. Beside UX-87's five sections carried unseen, these are values
carried and misdescribed.

- `JiraFieldset`'s Token hint is "Leave as-is to keep the stored token."
  (`web/src/features/settings/fieldsets/JiraFieldset.tsx:17`), over a field
  that is empty after the recommended macOS setup: `keepTokenSafe` clears
  `jira.Token` and sets `TokenCommand` (`internal/cli/config_cmd.go:287`).
  A user who types a token to "fix" it writes a secret into the file, and
  "The file's own `token` wins when set"
  (`docs/content/docs/configuration.md:195`, under "Keeping tokens out of
  the file") — what `config init` worked to avoid.
- `MessagingFieldset`'s Bot token hint is "Slack only; leave as-is to keep
  the stored token."
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:25`), with
  `Messaging.TokenCommand` and `TokenEnv` (`internal/config/config.go:109`)
  carried by the form and unmentioned.
- Announcement is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:35`), though
  `Messaging.Announcement` is a Slack-only template with seven
  placeholders (`internal/config/config.go:123`), which the fields table
  describes in one line (`docs/content/docs/configuration.md:74`). A Teams
  user edits it and sees no change; a Slack user has no placeholder list
  on screen.
- Channel is registered with no hint
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:34`), though
  `Messaging.Channel` "applies to a Slack bot token only; a webhook carries
  its own channel" (`internal/config/config.go:116`). A webhook user sees
  an editable channel that does nothing.

**Instead.** When `token_command` or `token_env` is set, replace the token
hint with "Taken from token_command: VALUE" (or the variable's name),
neither a secret; a hint on Announcement naming the placeholders, that it
applies to Slack only, and that empty keeps the built-in message; a hint on
Channel: "With a Slack bot token; a webhook posts to its own channel."

**Done when.** A `SettingsPanel` test seeding `jira.token_command` finds the
Token textbox described by text naming that command, and one seeding
`messaging.token_env` finds the Bot token textbox described by that
variable; `getByRole('textbox', { name: 'Announcement', description:
/\{author\}/ })` and `getByRole('textbox', { name: 'Channel', description:
/bot token/ })` resolve.

### UX-109 Staged state is a colored word, not a StateMark

Impact: low · Effort: small

**Today.** Each file's staged, partly staged or unstaged tag is a word
colored `text-success` when staged and muted otherwise (`ChangeRow`,
`web/src/features/branch/WorkingTree.tsx:92`), with no `StateMark` in the
file: the 1024 px dark Branch screenshot shows "staged" in green and
"unstaged" in gray beside each file, and a conflict shows kind "conflicted"
and tag "unstaged" with no failed mark. It is the one state on the web drawn
as a colored word, bypassing `StateMark` (`web/src/shell/StateMark.tsx:33`),
which the settled system says every state goes through; `text-success`
elsewhere colors outcome lines, not a thing's state. The Review section's
State, Mergeable and Changes requested rows (`PullRequestSummary`,
`web/src/features/review/ReviewPanel.tsx:97`) and the queue's "· Draft"
(`web/src/features/reviewqueue/ReviewQueuePanel.tsx:197`) are states drawn
as plain words with no mark too, uncolored. The terminal's `stageGlyph`
"says by shape how much of a change is staged"
(`internal/tui/commits.go:169`), four states by four glyphs. The word keeps
it accessible, so this is consistency inside the system, not a change to it.

**Instead.** A `StateMark` before the word — not-started, in-flight for
partly staged, done, failed for a conflict — carrying the status light,
with the word in the plain foreground.

**Done when.** The Branch screenshots show a shape before each staged word,
and `web/src/features/branch/WorkingTree.tsx` has no `text-success` on the
tag.

### UX-110 Light-theme placeholders read at 3.18:1 in the commit and pull request forms

Impact: medium · Effort: small

**Today.** `commitInputClass`, `prInputClass` and the settings `inputClass`
set no placeholder color, so Tailwind's preflight draws every placeholder
at 50% of the foreground: 3.18:1 on the light page, below the 4.5:1 floor
the gates promise, and 4.58:1 in the dark theme.
`tmp/audit/screens/1440-light-branch.png` samples the Subject hint at
(137,139,144) on (246,247,249). Two input styles disagree on one page, and
the gate cannot see it.

- `commitInputClass` (`web/src/features/branch/CommitForm.tsx:252`) has no
  placeholder color, so `MessageFields`' Subject hint "what the change
  does, in the imperative" (`:186`), the only hint of what the field wants,
  is drawn at 3.18:1.
- `prInputClass` (`web/src/features/review/ReviewPanel.tsx:379`) the same,
  so `ProposalFields`' three placeholders (`:340`) are too.
- The settings `inputClass` the same
  (`web/src/features/settings/fieldsets/Field.tsx:23`).
- The Filter's input sits on `placeholder:text-muted-foreground`
  (`IssueListControls`, `web/src/features/issues/IssueListControls.tsx:25`),
  6.10:1 in the light theme.
- `scan`'s axe tag set (`web/e2e/a11y.spec.ts:27`) has no rule that samples
  `::placeholder`, and `web/src/tokens.test.ts` compares tokens only, so
  `yarn test:e2e` stays green.

**Instead.** Add `placeholder:text-muted-foreground` to the three input
classes (or extract one input class that carries it), so every placeholder
is on the same token in both themes as the Filter's already is.

**Done when.** A Playwright check in `web/e2e/a11y.spec.ts` reads
`getComputedStyle(input, '::placeholder').color` for the Subject field in
both themes and asserts 4.5:1 against the page; or `web/src/tokens.test.ts`
grows a placeholder pair and a grep shows every input class uses it.

### UX-111 Light-theme text fields have no visible boundary: 1.3:1 on the page

Impact: medium · Effort: small

**Today.** Every text input, select and textarea is bordered by a 1 px
`--input` alone: four class strings are `bg-transparent`, and the Filter
and View controls (`IssueListControls`,
`web/src/features/issues/IssueListControls.tsx:25`, `:62`) are
`bg-background`, which on the page is the same color. The light
`--input` measures 1.29:1 on `--background` and 1.39:1 on `--card`,
below the 3:1 non-text floor of
WCAG 2.1 AA 1.4.11 the gates claim. A light-theme user looking for where
to type sees a faint outline that all but disappears on a bright display;
the empty Settings fields (User, Review status, Webhook URL, Default scope,
Types, Subject limit, Issue trailer, Slug limit) read as blank space under
their labels. `tmp/audit/screens/1440-light-settings.png` samples a field's
interior at (246,247,249), the page's own value, with (215,219,227) border
rows. Labels and the periwinkle focus ring still let a user find a field,
which is why this is medium and not high.

- `--input` is `#d7dbe3` in the light theme (`web/src/index.css:100`),
  1.29:1 on `--background` `#f6f7f9` and 1.39:1 on `--card` `#ffffff`, and
  `--border` is the same value (`:99`), so the decorative rule and the
  field boundary share one too-faint value. The dark `--input` (`#272d39`
  on `#0f1115`) is about 1.37:1 too, so both themes share the gap; the
  light one is where the outline vanishes.
- The settings `inputClass` is `bg-transparent border-input`
  (`web/src/features/settings/fieldsets/Field.tsx:23`): the border is the
  field's only boundary. `commitInputClass`
  (`web/src/features/branch/CommitForm.tsx:252`), `prInputClass`
  (`web/src/features/review/ReviewPanel.tsx:379`) and `AnnouncePreview`'s
  channel select (`web/src/features/messaging/MessagingPanel.tsx:246`) are
  the same.
- `web/src/tokens.test.ts` asserts `contrast` for the four system hues
  against the page and a card (`:122`) and for the disabled pair, never
  for a boundary token; axe cannot measure non-text contrast, so nothing
  in `task check` or `yarn test:e2e` notices.

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
exists under `web/src`, so all eight periwinkle buttons remove the browser
outline and draw keyboard focus as a 2 px box-shadow in their own color;
beside each, an outline Cancel gets a visible periwinkle ring. Tabbing from
Cancel to Announce now, or from Cancel to Push in the push confirmation,
the visible ring disappears when it reaches the button that sends: the
button grows 2 px in its own color. It hits Commit staged changes, Push,
Announce now, Open a pull request, Start work and Save changes — the
buttons a keyboard user reaches every loop. No screenshot shows it, since
nothing in the captures has focus, and `web/e2e/layout.spec.ts` checks
that a focused control is in view, not that its focus can be seen.

- `--ring` is `#8b93f8` (`web/src/index.css:43`), the value of `--primary`
  (`:26`); in the light theme `--ring` is `#4f56c9` (`:101`), the value of
  `--primary` (`:84`).
- `SaveControls`' submit is `bg-primary` with `focus-visible:ring-ring` and
  `outline-none`, no offset
  (`web/src/features/settings/SettingsPanel.tsx:151`).
- `CommitForm`'s submit, the same
  (`web/src/features/branch/CommitForm.tsx:108`).
- `PushConfirm`'s Push, the same
  (`web/src/features/branch/BranchPanel.tsx:188`), beside a Cancel (`:181`)
  whose ring is visible.
- `OpenPullRequest`'s button
  (`web/src/features/review/ReviewPanel.tsx:201`) and `PullRequestForm`'s
  submit (`:291`), the same.
- `StartWorkButton`, the same
  (`web/src/features/issues/WorkStory.tsx:299`).
- `AnnounceControls`' button
  (`web/src/features/messaging/MessagingPanel.tsx:192`) and
  `AnnouncePreview`'s Announce now (`:269`), the same, the latter beside a
  Cancel (`:261`) whose ring is visible.

**Instead.** Give the eight `bg-primary` sites `focus-visible:ring-offset-2
focus-visible:ring-offset-background` (a page-colored gap between fill and
ring), or extract one primary-button class that carries it, so the ring
reads on a periwinkle fill as it does on an outline one.

**Done when.** A Playwright test focuses Save changes in both themes and
asserts its computed `box-shadow` carries a `--background`-colored offset
(or a ring color that contrasts 3:1 with `--primary`), and a screenshot
with focus on Announce now shows a ring distinct from the fill.

### UX-113 The one control accent comes in three button sizes

Impact: low · Effort: small

**Today.** A grep for `bg-primary` under `web/src` finds exactly eight
sites and three padding pairs: five at `rounded-md px-3 py-1.5 text-sm`,
two at `px-4 py-2`, and one at `rounded-sm px-2 py-1`, so the most
consequential button in the Branch section is the smallest one on it.
Three outward acts each wait on a last look: the announcement preview
(`AnnouncePreview`, `web/src/features/messaging/MessagingPanel.tsx:269`)
and the pull request form (`PullRequestForm`,
`web/src/features/review/ReviewPanel.tsx:291`) draw the confirm as a
control; the push draws it as a chip.

- `PushConfirm`'s Push is `rounded-sm bg-primary px-2 py-1`, the third
  size (`web/src/features/branch/BranchPanel.tsx:188`); its Cancel (`:181`)
  is `rounded-sm px-2 py-1` too.
- `CommitForm`'s submit is `bg-primary px-4 py-2`, the second size
  (`web/src/features/branch/CommitForm.tsx:108`); comparing
  `tmp/audit/screens/1024-dark-branch.png` with `1024-dark-slack.png`,
  Commit staged changes is visibly taller than Announce to Slack though
  both are the section's one primary act.
- `SaveControls`' submit is `bg-primary px-4 py-2` too
  (`web/src/features/settings/SettingsPanel.tsx:151`); in
  `1024-dark-settings.png` Save changes matches the commit button and not
  the five.

**Instead.** Name a single primary-button class (`px-3 py-1.5 text-sm`,
with the focus offset UX-112 asks for) and use it at all eight sites;
`PushConfirm`'s Cancel takes the same size as `AnnouncePreview`'s. DEBT-93
records the same classes written out at each site; one `Button` component
(or one primary and one secondary class) closes both.

**Done when.** A grep for `bg-primary` in `web/src` finds one padding pair,
and a grep for `rounded-sm bg-primary` returns nothing.

### UX-114 Three places set type outside the scale and face the system names

Impact: low · Effort: small

**Today.** `web/src/index.css` reserves the code face for "a branch, a
commit, a path, a command. Nothing else is set in it." (the comment over
`--font-mono`, `:172`) and names `sm` the body of a dense tool and `base`
what heads a part of a panel (the comment over `--text-*`, `:179`). Three
places set type against that.

- `WorkStory` renders each stage's detail in a plain `text-sm` sans span
  (`web/src/features/issues/WorkStory.tsx:237`), and the detail carries
  `branch.name` from `offHeadStages` (`:70`) and `onHeadStages` (`:90`), so
  the story's Branch stage shows "fix/PROJ-412-redact-tokens · 3 ahead" in
  the sans muted foreground; `storyNote`'s "In progress on {branch}"
  (`:184`) is sans too. The Branch section's heading sets the same name in
  `font-mono` (`BranchSummary`,
  `web/src/features/branch/BranchPanel.tsx:58`);
  `tmp/audit/screens/1024-dark-issues.png` and `1024-dark-branch.png` show
  the two faces one click apart.
- Each stage title is `font-medium` with no size (`WorkStory`,
  `web/src/features/issues/WorkStory.tsx:235`), so base, over a `text-sm`
  detail: "Branch", "Changes", "Pull request" and "Announce" read at the
  size of the h3 "Work story" above them (`IssueDetailPanel`,
  `web/src/features/issues/IssueDetailPanel.tsx:52`, `text-base
  font-semibold`), differing only in weight.
- A queue row's title is `font-medium` inside a `<p>` with no size class
  (`RequestRow`, `web/src/features/reviewqueue/ReviewQueuePanel.tsx:192`),
  while issue summaries (`IssueRows`,
  `web/src/features/issues/IssuesPanel.tsx:233`), commits, changed files
  and CI checks are all `text-sm`; comparing
  `tmp/audit/screens/1024-dark-reviews.png` with `1024-dark-issues.png`,
  request titles are visibly larger than issue summaries though both are a
  row's headline.

**Instead.** Wrap the branch name in the stage detail and the note in a
`font-mono` span, leaving the separator and count in sans; set the stage
titles and the queue titles `text-sm font-medium` as the other list rows
are.

**Done when.** A `WorkStory` test finds the branch name inside an element
with the mono class (or a `<code>`); screenshots show the stage titles
smaller than the Work story heading and the queue titles at the issue
summaries' size.

### UX-115 The content measure is set three ways above lg

Impact: low · Effort: small

**Today.** A grep for `max-w-` under `web/src/features` finds exactly two
values: `max-w-3xl` on the review queue and `max-w-2xl` on Branch, Review,
the messaging section and Settings; the issue list is a fixed width with
no larger breakpoint and the issue detail has no measure at all. So the
same kind of content stops at different right edges, and a wide window
gives the list none of its width while a long description could run
950 px. The settled rule covers only the stack below `lg`; the width above
it is open.

- `Queue` is `max-w-3xl`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:96`) where the other
  four sections are `max-w-2xl`; switching from Branch to Reviews at 1024
  or 1440 px moves the content's right edge by about 96 px (the list
  border near 871 px in `tmp/audit/screens/1024-dark-reviews.png` against
  the commit form's near 775 px in `1024-dark-branch.png`), and no comment
  justifies the wider measure.
- `ListAndDetail` holds the list at `lg:w-80 lg:shrink-0` with no larger
  breakpoint (`web/src/features/issues/IssuesPanel.tsx:117`), so it stays
  320 px at 1440; `1440-dark-issues.png` is `1024-dark-issues.png` with
  416 px of blank added to the right, and six of the seven mock summaries
  wrap to two lines.
- The detail column is `min-w-0 flex-1` with no `max-w` (`:129`), and
  `Description`'s `<p>` has no measure of its own
  (`web/src/features/issues/IssueDetailPanel.tsx:153`); a real Jira
  description at 1440 px would be set at roughly 150 characters a line
  where the other sections stop at 672 px.

**Instead.** Name one content measure (`max-w-2xl`) and use it in all five
sections and on the detail's article; let the list column grow with the
window above `xl` (say `xl:w-96`) while the detail keeps its measure.

**Done when.** A grep for `max-w-` in `web/src/features` returns one value;
a 1440 px screenshot shows the mock's seven summaries on one line each,
and one with a 600-character description shows its lines no wider than the
Branch section's form.

### UX-116 Check out narrows its row, leaving the issue list ragged

Impact: low · Effort: small

**Today.** A row's Check out button is a flex sibling of the row button,
added only when a non-HEAD branch exists, so rows with one are narrower
than rows without and the list's right edge steps in and out. In
`tmp/audit/screens/1024-dark-issues.png` the selected PROJ-412 box runs the
list's full 320 px while PROJ-418 and PROJ-408 wrap their summaries in a
narrower box ending about 80 px short to fit Check out. `IssueRows`' `<li>`
is `flex flex-wrap` (`web/src/features/issues/IssuesPanel.tsx:201`), the
row button `flex-1` (`:218`), and `RowCheckout` is rendered beside it only
when `newest && !onHead` (`:236`), `shrink-0` so it takes its full width
from the row (`:414`).

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
(`web/src/features/branch/BranchPanel.tsx:85`). In
`tmp/audit/screens-hermetic/1024-dark-empty-branch.png` the eye lands on
four empty fields under "Working tree" and reads why only after scrolling
past them. `WorkingTree` renders nothing between the heading and the form
when `changes` is empty (`web/src/features/branch/WorkingTree.tsx:24`); the
explanation is `commitBlocker`'s line (`:55`), which `CommitForm` shows
beside the disabled submit, below the fields
(`web/src/features/branch/CommitForm.tsx:112`).

**Instead.** A muted line under the heading, "Clean — nothing to commit.",
with the form's foot line kept for the nothing-staged case.

**Done when.** A `WorkingTree` test with no changes finds "Clean — nothing
to commit." before the form in DOM order.

### UX-118 Copy URL confirms above the queue, not beside its row

Impact: low · Effort: small

**Today.** After Copy URL on a queue row, the only visible confirmation is
the panel's `OutcomeLine` above the list: `CopyURL` hands its done message
to the panel's teller
(`web/src/features/reviewqueue/ReviewQueuePanel.tsx:272`), which `Queue`
renders once (`:121`) under the "4 pull requests wait" summary, out of the
eye's path from the button, while its refusal renders in the row (`:289`).
`RowCheckout` uses the same success-above pattern
(`web/src/features/issues/IssuesPanel.tsx:401`), but a check-out changes
the row on the next snapshot; a copy changes nothing near the button. The
working tree's per-row `OutcomeLine` (`ChangeRow`,
`web/src/features/branch/WorkingTree.tsx:107`) shows the nearer pattern.

**Instead.** Give each row its own `OutcomeLine` under its controls, as
`ChangeRow` does.

**Done when.** A `ReviewQueuePanel` test finds the copied-URL status inside
the row's listitem.

### UX-119 A credential cannot be removed from Settings

Impact: low · Effort: small

**Today.** `keepSecret` treats an emptied secret field as "keep the stored
value" (`internal/webserver/config.go:286`) and `preserveSecrets` applies
it to all four secrets (`:258`), so Settings has no way to clear
`jira.token`, `messaging.token`, `messaging.webhook_url` or `forge.token`:
clearing a token in the form and saving keeps it, and moving from a bot
token to a webhook leaves the old token in the file, to be removed by
hand. The keep is documented — `updateConfig` says an empty or masked
`jira.token`, `messaging.token`, `messaging.webhook_url` or `forge.token`
keeps the stored secret (`api/openapi.yaml:409`), and the web page says
under "Settings" that a credential is "kept as it is unless you type a new
one" (`docs/content/docs/web.md:128`) — but no clear is offered anywhere.
UX-87 covers sections the form cannot show, not clearing a secret.

**Instead.** Accept an explicit clear — a `null` for the four secret
fields in the contract, or a per-field remove control — that writes an
empty value.

**Done when.** A test sends `jira.token` as `null` and the saved file holds
no token.

### UX-120 Settings draws a failed read as a resting state

Impact: low · Effort: small

**Today.** When the configuration cannot be read, `SettingsPanel` puts the
reason inside `<EmptyState>`
(`web/src/features/settings/SettingsPanel.tsx:35`), as muted text with no
`role="alert"` (`:37`); `EmptyState` is the dashed, centered,
`text-muted-foreground` box a panel shows when it has nothing yet
(`web/src/shell/EmptyState.tsx:8`). The review queue (`Queue`,
`web/src/features/reviewqueue/ReviewQueuePanel.tsx:103`) and the issue
detail (`IssueUnread`, `web/src/features/issues/IssueDetailPanel.tsx:76`)
show the same kind of failure as a `role="alert"` line in
`text-destructive`. Red is the failure color and nothing else, and here a
failure is not red:
`tmp/audit/screens-hermetic/1024-dark-empty-settings.png` shows gray "The
configuration could not be loaded." centered in the dashed box, the same
drawing as "Connecting to workflow…", while `1024-dark-empty-reviews.png`
shows the queue's failure red and left-aligned; a screen reader hears
nothing, since no live region carries it. A Retry is right there and a
configuration that fails to read is outside the daily loop, which is why
this is low.

**Instead.** Say the failure as the other read failures do: a
`role="alert"` paragraph in `text-destructive` with Retry beside it,
outside the `EmptyState`.

**Done when.** A `SettingsPanel` test with a failing configuration read
finds the reason by role alert; a screenshot shows it in the failure color.

### UX-121 The web port is fixed at 7000 and cannot be chosen

Impact: low · Effort: small

**Today.** `LoopbackAddr` is the constant `"127.0.0.1:7000"`
(`internal/webserver/webserver.go:229`), the only address the server ever
binds; `NewRootCmd` serves it (`internal/cli/cli.go:161`) though
`WebServerAt` already takes an address (`:270`), and `NewRootCmdOver`'s
`--web` help hard-codes it (`:216`). The root declares `--dry-run`, `--log`
and `--web` and no port, and a grep for Port or 7000 in `internal/config`
finds no setting. So a machine with 7000 taken cannot run `--web` at all —
the listen fails and the command exits — and two repositories cannot be
served side by side. `docs/content/docs/web.md:21`, under "Start it",
documents the fixed port. UX-62 lists the scriptable commands' missing
flags and does not mention `--port`.

**Instead.** A `--port` flag (loopback stays fixed) threaded to
`WebServerAt`, printed in the address line on stderr and documented in
`docs/content/docs/web.md`.

**Done when.** `workflow --web --port 7001` serves on `127.0.0.1:7001`, and
the spec's servers entry or its description says the port may vary.

### UX-122 Web copy settles plurals, case, periods and state words site by site

Impact: low · Effort: small

**Today.** The same act, state or moment reads differently depending on
where the eye lands — list versus story, one `dl` row versus the next,
header versus content, browser versus terminal.

- The web has no plural helper, where the terminal's `plural` counts in
  words (`internal/tui/render.go:478`): `changesDetail` says "{n} file(s)
  to commit" (`web/src/features/issues/WorkStory.tsx:145`), `PushConfirm`
  "Push {commits} commit(s) to the remote?"
  (`web/src/features/branch/BranchPanel.tsx:177`; the count itself is
  UX-104), and `filterOutcome` "{shown} of {loaded} loaded issues match."
  (`web/src/features/issues/IssuesPanel.tsx:346`), so "1 … match."
  disagrees in number.
- `loadOutcome` ends "4 of 5 loaded" without a period (`:329`) and "All 5
  loaded." with one (`:332`).
- Five placeholders split four lowercase to one sentence case: "what the
  change does, in the imperative" (`MessageFields`,
  `web/src/features/branch/CommitForm.tsx:186`), "comma-separated
  usernames" twice and "comma-separated labels" (`ProposalFields`,
  `web/src/features/review/ReviewPanel.tsx:340`, `:349`, `:358`), against
  "Key or summary" (`IssueListControls`,
  `web/src/features/issues/IssueListControls.tsx:21`).
- The one check-out write is "Switching…" in the list row (`RowCheckout`,
  `web/src/features/issues/IssuesPanel.tsx:416`), outside its `aria-label`
  "Check out KEY" (`:409`), so a keyboard user's visible word is not in
  the button's accessible name, and "Checking out…" in the story
  (`CheckoutButton`, `web/src/features/issues/WorkStory.tsx:269`).
- The never-done Announce stage is "Not announced" off HEAD
  (`notStartedStages`, `web/src/features/issues/WorkStory.tsx:59`;
  `offHeadStages`, `:73`) and an imperative on it: `announceDetail` reads
  "Announce to {channel}" (`:139`), on a button that opens the messaging
  section.
- A missing base is "—" (`BranchSummary`,
  `web/src/features/branch/BranchPanel.tsx:64`) beside a missing upstream
  "none" (`:66`).
- `SectionPanel` says "Connecting to workflow…" while the snapshot is null
  (`web/src/shell/SectionPanel.tsx:21`), never reading the stream's
  status, while `useEventStream` sets `reconnecting` on every
  `EventSource` error, one before any open included
  (`web/src/api/snapshot.ts:87`); so the header says "Reconnecting" over
  every section's "Connecting to workflow…" at once, where
  `docs/content/docs/web.md:51`, under "The page", defines Reconnecting as
  "the connection dropped" and Connecting as no first update yet.
- The empty review queue is "Nothing is waiting on your review." on the
  web (`queueSummary`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:134`; `Requests`,
  `:160`), "No pull requests are waiting on your review." in the
  terminal's detail (`reviewQueueDetail`,
  `internal/tui/reviewqueue.go:117`) and on the command line
  (`renderReviews`, `internal/cli/reviews.go:84`), and "none waiting on
  you" in the terminal's rail (`reviewQueueRail`,
  `internal/tui/reviewqueue.go:101`).
- Past a month `waited` prints `toLocaleDateString()`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:252`) — "8/15/2026"
  or "15/08/2026" by locale — under a comment promising "the terminal's
  words" (`:229`), where the terminal's `age` prints `time.DateOnly`
  (`internal/tui/detail.go:484`).
- The configuration read is "Loading the configuration…"
  (`SettingsPanel`, `web/src/features/settings/SettingsPanel.tsx:30`) where
  every other read says "Reading", and its failure is "could not be
  loaded" (`:37`) in one place and "could not be read again"
  (`ConfigForm`, `:110`) in the same file.

**Instead.** A `plural(count, noun)` in `web/src/lib/utils.ts` for the
three counting sites; one form for both load-count states; one case for
every placeholder; "Checking out…" on both check-out buttons (a substring
of the row's `aria-label`); one Announce phrasing beginning "Not announced"
that names the channel; one placeholder word for absence in both `dl`
rows, in the muted foreground; `SectionPanel` wording its wait from the
same status table as `StreamStatus`, or the stream staying "Connecting"
until its first open; the terminal's empty-queue sentence and the ISO date
past a month; "Reading the configuration…" and "could not be read" in
Settings.

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
  `web/src/features/review/ReviewPanel.tsx:88`) with `{pull.title}` as its
  whole accessible name (`:92`) and only `underline-offset-4
  hover:underline` for a class (`:90`): no `text-primary`, no icon, no
  new-tab note, no focus-visible ring. Each CI check's name is the same
  (`:125`, `:127`, `:129`), and a check without a URL is a bare `<span>`
  (`:123`) that looks identical. On the light and dark Review screenshots
  the heading "#128 fix: redact tokens…" and the rows "lint passed" …
  "e2e running" look like static text; a focused forge link falls back to
  the browser's default outline; a screen-reader user activating either
  is moved to a new tab unwarned. The page's other two outbound links
  carry all of it: the queue's Open (`RequestRow`,
  `web/src/features/reviewqueue/ReviewQueuePanel.tsx:212`) and Open in
  Jira (`IssuePeople`, `web/src/features/issues/IssueDetailPanel.tsx:137`),
  each with the sr-only "(opens in a new tab)".
- Each work-story stage is a button that calls `setSection` (`WorkStory`,
  `web/src/features/issues/WorkStory.tsx:231`), yet it is styled only with
  `hover:bg-accent` and a focus ring (`:233`) and its sr-only span carries
  the state alone (`:236`): nothing at rest or in its name says it
  navigates, though `docs/content/docs/web.md:74` promises under "Issues"
  "each step opening the section it belongs to", so the story reads as a
  plain timeline.
- The Settings `<form>` has `onSubmit` and a `className` only
  (`ConfigForm`, `web/src/features/settings/SettingsPanel.tsx:121`), no
  `aria-label` or `aria-labelledby`, so it is not a form landmark while
  the commit and pull request forms are, and
  `getByRole('form', { name: /settings/i })` cannot resolve the site's
  largest form.
- `ThemeToggle`, icon-only at every width, carries its name in
  `aria-label` alone (`web/src/shell/ThemeToggle.tsx:24`) with no `title`,
  where `NavRail`'s icon-only buttons show theirs on hover (`title={name}`,
  `web/src/shell/NavRail.tsx:30`); a mouse resting on the toggle shows
  nothing.

**Instead.** Draw the title and check-name links as the queue draws Open —
`text-primary`, the `ExternalLink` icon, the sr-only new-tab note and the
focus-visible ring — so a check with a URL differs visibly from one
without; mark each stage as a control at rest inside the system (the title
in `text-primary`, or a trailing chevron in the muted foreground) and give
it an sr-only suffix or `aria-describedby` naming the section it opens;
`aria-labelledby` on the Settings form pointing at the section's h1; a
`title` on the toggle equal to the choice it announces ("Theme: System").

**Done when.** `web/src/features/review/ReviewPanel.test.tsx` finds the
title link and each check with a URL by role link with a name ending
"(opens in a new tab)" and `target="_blank"`, and a check without a URL as
text; a `WorkStory` test finds each stage by role button with a name or
description matching /opens (branch|review|messaging)/i and a screenshot
of the Issues section shows a rest-state mark on each stage;
`screen.getByRole('form', { name: /settings/i })` resolves in
`web/src/features/settings/SettingsPanel.test.tsx`; a `ThemeToggle` test
asserts the button's `title` names the current choice and changes with it.

### UX-124 Two Settings selects draw blank for the value in effect

Impact: low · Effort: small

**Today.** `pull_request.title_source` and `messaging.kind` may be the
empty string, and the file `config init` or a Settings save writes holds
`""` unless a value was chosen; the server reads them as `commit` and
Slack, and the spec allows it. But the Title source and Service selects
offer no option for `""`, so a default configuration shows an empty
control in either theme, and once a value is picked there is no way back
to the default. Every Settings screenshot shows Title source empty under
"Pull request", the only control on the form with no visible value. A
blank select saves back `""` harmlessly and Settings is opened rarely,
which is why this is low.

- `PullRequestFieldset` offers `commit` and `issue` only
  (`web/src/features/settings/fieldsets/PullRequestAndStoreFieldsets.tsx:16`),
  and `MessagingFieldset`'s choices begin at `slack`
  (`web/src/features/settings/fieldsets/MessagingFieldset.tsx:14`);
  `SelectField` registers the `<select>` with the value as-is
  (`web/src/features/settings/fieldsets/Field.tsx:104`), so `""` matches no
  option and shows blank. `ForgeFieldset` handles the same case with an
  explicit `['', 'Auto-detect']`
  (`web/src/features/settings/fieldsets/ForgeFieldset.tsx:13`).
- `Default` sets no `PullRequest`, so `title_source` is `""`
  (`internal/config/config.go:186`), and its `Messaging` leaves `Kind` `""`
  (`:190`); `collectMessaging` returns `config.Messaging{}` when Slack is
  skipped (`internal/cli/config_cmd.go:305`), so the guided init writes
  kind `""` and the Service select draws empty while the messaging
  section's rail label and heading say "Slack" — two surfaces disagreeing
  about one file.
- `PullRequest.TitleSource` documents `commit` as the default
  (`internal/config/pullrequest.go:20`) and its tag is
  `json:"title_source"` without `omitempty` (`:21`),
  `validatePullRequest` accepts `""` (`:27`), and `write`'s `MarshalIndent`
  of the whole struct writes the empty value
  (`internal/config/save.go:140`).
- `MessagingConfig`'s `kind` `enum` is `["", slack, teams, discord,
  webhook]` (`api/openapi.yaml:1425`), so the spec allows what the select
  cannot show; the mock the screenshots show has `title_source: ''`
  (`web/src/dev/mockConfig.ts:55`).

**Instead.** A first choice `['', "The branch's oldest commit (default)"]`
and `['', 'Slack (default)']` as `ForgeFieldset` does for its empty kind —
or normalize `""` onto the default when seeding the form and let the save
write it.

**Done when.** A `SettingsPanel` test seeding `title_source: ''` finds the
Title source combobox with a selected option naming the oldest-commit
default, and one seeding `messaging.kind: ''` finds the Service combobox
with a selected option named for Slack.

## The visual system

What is there is a real system, and a good one for a terminal: four
systems' hues (Jira blue, git yellow, the forge green, chat magenta) and
red for failure alone, all taken from the terminal's own palette so the
user's theme decides the shades; shape for state (`○ ◐ ● ✗`); border weight
for focus. None of that should change. This edition found two places in the
terminal where the system is applied loosely, the checkbox and the diff
(UX-100), and none where it is broken.

The web now speaks it too. The four systems' hues are tokens in both
themes (`web/src/index.css:62`), in the terminal's hue families but held at
least 30° of OKLCH hue from the status lights, the periwinkle control
accent and each other, and at 4.5:1 as text, by `web/src/tokens.test.ts`;
they mark whose a thing is — the active rail icon, each section's heading,
the work story's stages and an issue's local branch — and never how it
stands. Every state is drawn by its shape through one `StateMark`
(`web/src/shell/StateMark.tsx:33`), hidden from assistive tech beside its
words. Type, space and corners each have one scale
(`web/src/index.css:162`), headings are set in sentence case — the web lint
refuses an `uppercase` class — and monospace is for code alone. Periwinkle
stays the one control accent, and the middle dot the separator.

It follows the window, too. Below `md` the rail keeps its icons alone, each
name kept for assistive tech and shown on hover; below `lg` the issue list
sits over its detail, and at every width it scrolls in its own pane, over a
line saying how many it holds. The header holds still while the content
scrolls beneath it, and a word wider than the content breaks rather than
scroll it sideways. `web/e2e/layout.spec.ts` holds every section to 640,
1024 and 1440 px in both themes: nothing scrolls sideways, nor the page
down, Tab reaches every control, each in view as it takes focus, and axe
finds nothing.

## Across the surfaces

What is open here is worktrees and a fresh base beyond the terminal, a
merged branch's stage, GitHub- and terminal-shaped sentences, four dry runs
that say less than the live path, failed reads told as empty answers,
failures that tell less than the seam knows, and sentences that disagree.

### UX-89 Worktrees and a fresh base on the command line and the web

Impact: low · Effort: medium

**Today.** The interface's branch creator fetches `origin` first and offers
to branch from what you have when the fetch fails (`internal/tui/branch.go:468`), and
`ctrl+w` creates the branch in a worktree beside the repository
(`internal/tui/branch.go:408`). `workflow branch` and `POST /api/branches` do neither:
no fetch, no worktree.

**Instead.** `--worktree` and `--fetch` on `branch`; a worktree toggle on
the web's start-work flow; both through the shared composition layer,
`internal/loop`.

**Done when.** `workflow branch KEY --worktree` creates a directory beside
the repository and says where.

### UX-125 The surfaces disagree on a merged branch's review stage

Impact: medium · Effort: medium

**Today.** Once a pull request has merged, the three surfaces give three
answers on the same branch: the command line says the review never began,
the terminal's spine says it is still going, and the browser says the pull
request is ready for review. `status` already reads the store's announce
memory as the spine does, so its last stage agrees once it counts a merged
pull request as found. The browser's side is DEBT-127; the Review pane
already offers `n` on the same state.

- `status`: `○ Review` and the JSON state `not_started` (`gatherReview`,
  `internal/cli/status.go:286`; the comment at `:287` says a merged branch
  is treated as having no open review). The scripting guide's last-stage
  sentence (`docs/content/docs/scripting.md:125`) says `done` only for an
  open pull request, and widens to a merged one with this fix.
- The spine: the review stage in flight on a fresh session. `reviewState`,
  `internal/progress/progress.go:115`, has no case for a merged pull and
  reaches `Done` only through `CIPassed`, while `pullFound.apply`,
  `internal/tui/review.go:102`, keeps `found: msg.found` whatever the pull's
  state — the opposite choice from `status`.
- The review rail, `internal/tui/review.go:273`: "● merged", in the Review
  pane below the spine's in-flight stage.
- `checkCI`, `internal/tui/review.go:115`, never polls a merged pull, so
  the spine's stage never reaches `Done` through `CIPassed` either.
- `Model.work`, `internal/tui/spine.go:91`, passes `m.review.found` as
  `PullRequestFound`, true for a merged pull as much as an open one.
- `Work.PullRequestFound`, `internal/progress/progress.go:49`, is
  documented as an open pull request, which the spine does not hold to.
- The browser: `server.review`, `internal/webserver/handlers.go:182`, polls
  CI for a merged pull, and `PullRequestSummary`'s State row,
  `web/src/features/review/ReviewPanel.tsx:97`, can say only Draft or Ready
  for review (DEBT-127).
- `StateMerged`, `internal/forge/pulls.go:61`: "so its branch's work is
  done" — what no surface's stage says.

No test in `internal/cli/status_test.go` or `internal/progress` uses a
merged fixture; `mergedPull` (`internal/cli/forgefake_test.go:40`) serves
the announce and standup tests only.

**Instead.** Give `progress.Work` the pull's state (open, merged or none)
and let `reviewState` read merged as `Done`, set from both `gatherReview`
and the interface's `work()`, so the command line, the spine and the rail
agree; the browser's State row follows DEBT-127's wire `state`.

**Done when.** A `status` test with a merged pull request fixture prints
`● Review` and JSON state `done`, a spine screen test on the same fixture
shows the review stage done beside the rail's "● merged", a `ReviewPanel`
test with a merged pull finds the State row saying merged, and a progress
table case for a merged pull exists.

### UX-126 Sentences that assume GitHub or the terminal, told elsewhere

Impact: low · Effort: small

**Today.** The forge's noun and sigil and the no-token hint are GitHub's
wherever a site does not draw them from `Kind` or `forge.Sources`, and
three Slack refusals carry the terminal's "press enter" onto the command
line. A GitLab user reads "merge request" and `!7`
on one line and "pull request" and `#7` on the next, is told to run `gh
auth login` by the interface and to set `$GITLAB_TOKEN` by `doctor`, and a
script is told to press a key it does not have.

- `errNoCommitsToOpen`, `internal/cli/pr.go:24`, and `errPullAlreadyOpen`,
  `:28`: fixed "pull request" sentences that `composeRefusal` returns
  (`:214`, `:210`), while the same command's question uses
  `seams.Kind.Noun()` (`:124`) and its success line `Kind.Sigil()` (`:145`).
- `errNoPullRequest`, `internal/cli/announce.go:22`: fixed "pull request",
  wrapped at `:115` without the noun, though `announceSeams` carries `Kind`
  and uses it at `:125`.
- `renderReviews`, `internal/cli/reviews.go:84`: "No pull requests are
  waiting on your review." with no forge kind in reach; `reviewLine`,
  `:103`, writes `#%d` before every number.
- `docs/content/docs/scripting.md:85`, the `reviews` row of the stdout and
  stderr table: quotes that sentence verbatim, so the row moves with it.
- `writePulls`, `internal/cli/standup.go:272`: `- #` before each number;
  `standupSeams` carries no `Kind`.
- `Model.reviewQueueDetail`, `internal/tui/reviewqueue.go:111` and `:117`:
  "pull requests" whatever the forge; `Model.reviewRows`, `:132`, writes
  `" #"` where the Review pane's rail uses `m.vocab.sigil`
  (`internal/tui/review.go:272`).
- `prBodyHelp`, `internal/tui/prcomposer.go:30`: a fixed "Write the pull
  request description above this line", handed to `$EDITOR` by the
  composer (`:376`) and the editor (`internal/tui/preditor.go:109`).
- `forgeErrors`, `internal/tui/failure.go:189`: the full form of
  `forge.ErrNoToken` says "Run `gh auth login`, or set `$GITHUB_TOKEN`" for
  every host, though `Sources`, `internal/forge/token.go:245`, already
  names the variable and tool per host and `noForgeTokenMessage`,
  `internal/cli/doctor_credentials.go:163`, prints it — so `doctor` and
  the interface disagree on a GitLab host.
- `rejectionReason`, `internal/messaging/post.go:175`, `:177` and `:178`:
  three sentences end "then press enter to try again" inside a domain
  error; `runAnnounce`, `internal/cli/announce.go:146`, wraps it with `%w`
  and `main`, `cmd/workflow/main.go:27`, prints it on stderr, key and all —
  against the interface's own rule, in the doc comment above `wording` at
  `internal/tui/failure.go:76`, that a full form names no key to press.

**Instead.** Carry the forge `Kind` on `reviewsSeams` and `standupSeams` as
`prSeams` and `announceSeams` already do, and word every fixed "pull
request" and `#` through `Kind.Noun()` / `Kind.Sigil()` on the command line
and `m.vocab` in the terminal and the editor help, keeping the sentinels for
`errors.Is`. Wrap `ErrNoToken` with `Sources(kind, host)` where `Resolve`
fails (`internal/forge/token.go:162`) so every surface carries the per-host
hint. Keep `rejectionReason` to the fix ("invite the bot to #dev") and leave the
key out, as the terminal's rule already asks — it keeps `ErrPostRefused` in
its own words (`ownWords`, `internal/tui/failure.go:257`) and the overlay's
footer offers enter itself; change the `reviews` row of scripting.md with
it.

**Done when.** On a GitLab remote, tests of `pr`'s two refusals,
`announce`'s refusal, `reviews` (the empty-queue note on stderr, `!` before
each number on stdout), `standup`'s draft (`!7`), the Reviews pane and the
composer's `ctrl+o` help all see "merge request" and `!` and never "pull
request" or `#`; a screen test with a gitlab.com remote and no token shows
`$GITLAB_TOKEN` and not `gh auth login`; an `announce` test whose fake
Slack answers `not_in_channel` sees the fix on stderr and no "press enter";
`docs/content/docs/scripting.md:85` matches the new `reviews` note.

### UX-127 Four dry runs say less than the live path would do

Impact: low · Effort: small

**Today.** A rehearsal on either surface leaves out a write the live path
makes, or claims one it would first ask about, and the two surfaces
disagree about the same act.

- `branchCreator.create`, `internal/tui/branch.go:436`: "dry run: would " +
  `dryRunAction()` + the name + the start, and `dryRunAction`, `:453`,
  yields "fetch origin, then create " or "create a worktree for " — never
  the switch that the success notice reports ("created and switched to",
  `branchCreated.apply`, `internal/tui/branchresult.go:76`) and that the
  command line's dry-run line names (`runBranch`,
  `internal/cli/branch.go:103`).
- `prComposer.dryRunNotice`, `internal/tui/prcreate.go:76`: appends " and
  link it on KEY" whenever a Jira issue is named, while the live path
  (`pullCreated.apply`, `:130-137`) offers the link and then the review
  status instead of making either.
- `writeOptions.proceed`, `internal/cli/scriptable.go:82`: a dry run prints
  one line and returns false, so `runPR`, `internal/cli/pr.go:131`, returns
  before `followUp` at `:149`; the "dry run: would push … open TITLE" line
  (`:128`) is all `pr --dry-run` says, and the dry-run lines of `offerLink`
  (`:185`) and `offerReviewStatus` (`:244`) never print — though "Writing
  without a person", `docs/content/docs/scripting.md:186`, says `--yes`
  answers the push, the open and the offers. DEBT-94 counts these two
  strings as dead arms; this entry makes them reachable rather than
  deleting them.
- `runAnnounce`, `internal/cli/announce.go:124`: `offerAgain` runs before
  the preview at `:132` and `proceed` at `:135`, and under `--yes` prints
  "Not announced again; run without --yes to be asked." (`offerAgain`,
  `:171`) even with `--dry-run` — so the documented unattended form plus
  the documented safe form together show nothing of the announcement. The
  `--yes` bullet of "Writing without a person"
  (`docs/content/docs/scripting.md:186`) settles the skip, and the
  `--dry-run` bullet (`:194`) promises "the preview and what the command
  would do"; the two leave the combination unspecified, and the code
  answers it with the skip.

**Instead.** Append "and switch to it" (or "in a worktree at …") to
`dryRunAction`'s sentence, and have `dryRunNotice` name the offers rather
than the link. After `pr`'s open line, print `dry run: would link it on
KEY` and `dry run: would move KEY to STATUS` when those offers would be
made. In `announce`, check `dryRun` before the `--yes` skip, say the moment
was already announced, still print the preview and end with a dry-run line
saying `--yes` would leave it as it is — keeping the settled skip.

**Done when.** `TestADryRunBranchIsOnlyDescribed` requires "and switch to
it"; `TestADryRunOpensNothing`'s expected notice names the offers rather
than the link; a `pr --dry-run` test on a review-configured repository sees
the open, link and move `dry run: would …` lines on stderr and no Jira
write; an `announce --yes --dry-run` test with an announced moment sees the
message on stdout and no "run without --yes" on stderr.

### UX-128 Failed reads pass as empty answers in status, standup and the web

Impact: medium · Effort: medium

**Today.** `status`, `standup` and the web server each drop a seam's read
error into their "nothing found" path, and nothing says so anywhere: a
prompt shows "CI none" while CI is red because a token expired and a script
cannot tell "none" from "unknown"; up to fifteen failing forge requests are
made in silence and the team receives a standup saying nothing happened; in
the browser three sections carry misleading copy during an outage under a
header that says Live, with no reason and no Retry.

- `issueSummary`, `internal/cli/status.go:283`: `if err != nil` returns
  `""` — the issue read's error is dropped and the summary left blank.
- `gatherReview`, `internal/cli/status.go:286`: `err != nil` is folded into
  the not-found return, so an unreachable forge reads `○ Review`; at `:294`
  a `CheckStatus` error becomes `forge.CINone`, the same as no CI.
- `countChanges`, `internal/cli/status.go:304`: a `Changes` error becomes 0
  uncommitted files.
- `TestStatusWhenCICannotBeRead`, `internal/cli/status_test.go:318`: asserts
  "CI none" and nothing about stderr, so the silence is neither pinned nor
  caught.
- "Standard output and standard error", `docs/content/docs/scripting.md:76`:
  "Standard error carries … warnings" — the contract `status` does not
  meet.
- `gatherStandup`, `internal/cli/standup.go:183`: `issues, _ :=
  seams.Search(…)` discards the search error; its doc comment at `:175`
  settles "leaves its section empty rather than failing", not silence.
- `gatherPulls`, `internal/cli/standup.go:191`: a `Branches` error returns
  nil with no note, and at `:203` `err == nil && found && pull.IsOpen()`
  drops every `FindPull` error and keeps looping, up to
  `standupBranchLimit` failing requests.
- `writeIssues`, `internal/cli/standup.go:255`, and `writePulls`, `:267`:
  "- none" whether the service answered or refused; `--no-edit --yes` posts
  it.
- `server.snapshot`, `internal/webserver/stream.go:82`: the comment
  codifies the rule — a seam that fails yields an empty panel;
  `snapshotIssues`, `:138`, returns an empty first page when `Search`
  fails, `snapshotBranch`, `:154`, returns `branchDTO(gitrepo.Branch{})`,
  `snapshotChanges`, `:169`, `changesDTO(nil)`, and `snapshotReview`,
  `:184` and `:189`, `api.Review{Found: false}` — indistinguishable from no
  pull request.
- `Review`, `api/openapi.yaml:1180`: carries `found`, `pull` and `ci` only,
  and `Snapshot` (`:989`) has no per-panel problem.
- `server.review`, `internal/webserver/handlers.go:182`: a `CheckCI` error
  drops `ci` from the answer; `PullRequestSummary`,
  `web/src/features/review/ReviewPanel.tsx:137`, renders nothing for a null
  `ci`, where the terminal's `Model.reviewDetail`,
  `internal/tui/review.go:320`, shows the CI failure under the pull
  request.
- `BranchReview`, `web/src/features/review/ReviewPanel.tsx:59`: `found`
  false falls through to the `OpenPullRequest` form, so a forge outage
  offers to open a pull request. The offer is wrong copy, not a wrong
  write: `ComposePull`'s comment, `internal/loop/pull.go:70`, lets a forge
  that cannot say through, and the open lands on the forge's own answer.
- `BranchPanel`, `web/src/features/branch/BranchPanel.tsx:25`: an empty
  name with `detached` false says "This directory is not a Git repository",
  which a failed read also produces; `StreamStatus`,
  `web/src/shell/StreamStatus.tsx:11`, sets Out of date only for an
  unreadable frame, so a frame with an emptied panel reads Live.
- `ListAndDetail`, `web/src/features/issues/IssuesPanel.tsx:112`: the
  emptied first page renders "No issues match this view." — a Jira outage
  reads as an empty view.

`TestStreamSnapshotDegradesWhenSeamsFail`
(`internal/webserver/stream_test.go:232`) pins the web's silence (its
assert at `:249` wants `snap.Issues.Total` zero for a failing `Search`), the
terminal's "each pane fails on its own" has no web twin, and only the
opt-in `--log` records the failed request.

**Instead.** Keep degrading, but say so. On the command line, one stderr
line per service that failed, through the `output.notes` the writes use and
the sentinel wording the other commands share, stopping at the first forge
error in `standup` rather than making fourteen more, and leaving stdout and
the draft's "- none" as they are — telling "nothing to ask"
(`jira.ErrNoCredential`, no forge configured) from a service that refused.
On the wire, an optional problem per panel in the Snapshot (the `Problem`
shape `fault` already curates) and a `ci_error` on the Review, rendered in
that section as a failure — the failure `StateMark` and a `role=alert` line
— rather than as the empty state.

**Done when.** A `status` test with a forge that errors sees "CI none" on
stdout and a line naming the forge on stderr, and one whose forge answers
sees an empty stderr; a `standup` test with a 500 from Jira sees the draft
on stdout and a note naming Jira on stderr, while
`TestStandupWithNoWorkSaysEachSectionIsEmpty` still sees three "- none" and
no note; a stream test with a failing `FindPull` sees a review panel
carrying a problem, and ReviewPanel, BranchPanel and IssuesPanel tests
render such snapshots by role alert rather than as the open-a-pull-request
form, the not-a-repository state or "No issues match this view."; a
ReviewPanel test with `ci` null and `ci_error` set finds the alert naming
the reason.

### UX-129 Four failures tell less than the seam knows

Impact: low · Effort: small

**Today.** Four places hold a reason or a result they do not show.

- `issueList.settle`, `internal/tui/issues.go:132`: `l.found, l.err =
  answer.found, answer.err` replaces a populated list with a failed first
  page, while the `startAt > 0` arm (`:122`) keeps what is there when a
  further page fails — so `r` while Jira is down turns the pane into
  "✗ failed · see detail" until Jira answers, though the store's cache
  still holds the issues (`cacheIssues`, `:49`, skips a failed page).
- `fetched.apply`, `internal/tui/branchresult.go:50`: stores git's error in
  `creator.fetchProblem`, which `branchCreator.view`,
  `internal/tui/branch.go:339`, tests only for non-nil before a fixed
  "could not fetch; enter branches from what you already have" — one
  sentence for a dead network and a bad remote. The promises table's row
  for "the one way the interface says something broke" does not hold for
  it: there is error text to word, and the line drops it.
- `server.writeOver`, `internal/webserver/config.go:110`: any `fromDTO`
  failure — `config.Parse` behind it (`:252`) — answers 422 with the fixed
  detail "the configuration is not valid", dropping the field and value the
  validators name (`refs_trailer` with a colon,
  `internal/config/commit.go:58`; a bad `title_source`,
  `internal/config/pullrequest.go:30`), which `doctor` prints
  (`reportLoadError`, `internal/cli/doctor.go:275`);
  `TestUpdateConfigRejectsAnInvalidConfig`,
  `internal/webserver/config_test.go:107`, asserts status and code only.
- `nothingToOpen`, `internal/webserver/pullrequest.go:88`, words
  `PullAlreadyOpenError` — which carries the open pull request "so a
  surface can point at it" (`internal/loop/pull.go:30`) — and
  `ErrNothingToOpen` alike as "there is nothing to open a … for", where
  the command line names the open one's URL (`internal/cli/pr.go:210`); a
  stale tab that opens a pull request twice is told there is nothing to
  open, not that #42 is already open.
  `TestOpenPullRequestIsAConflictWhenOneIsAlreadyOpen`,
  `internal/webserver/pullrequest_test.go:422`, asserts status only. (A
  Branch read failure answered with the same 409, through
  `server.composePullRequest`'s `err == nil` at
  `internal/webserver/pullrequest.go:116`, is DEBT-129.)

**Instead.** On a failed first page keep `l.found`, record `l.err` and let the
rail say "failed · see detail" beside the stale list, as a failed further page
already does; draw `failureLine` on `fetchProblem` above the offer line; carry
`Parse`'s wrapped reason after the `ErrInvalid` prefix in the 422 detail; word
the 409 by cause — `PullAlreadyOpenError` names the open pull request, and
`ErrNothingToOpen` says no branch or commits.

**Done when.** An issues refresh test whose second search fails still shows
PROJ-412 in the rail beside the failure;
`TestAFailedFetchOffersToBranchFromWhatIsThere` also requires git's words
("could not read from remote repository") on screen;
`TestUpdateConfigRejectsAnInvalidConfig` asserts the detail names the refused
field or value; `TestOpenPullRequestIsAConflictWhenOneIsAlreadyOpen` asserts
the detail names the open pull request.

### UX-130 Six sentences that disagree with a neighbor or a sibling surface

Impact: low · Effort: small

**Today.** Six things are said two or three ways.

- `ErrDirtyTree`, `internal/loop/guards.go:16`: "the working tree has
  uncommitted changes"; `errDirtyTree`, `internal/tui/switchtask.go:25`, is
  a second sentinel with its own wording, and `errDirtyTree`,
  `internal/webserver/checkout.go:19`, a third — one guard, three
  sentences, so a user who meets the refusal in the browser and then in the
  terminal reads two, and a change to the guidance is made in three places.
- `announceTarget`, `internal/cli/announce.go:156`: "the configured SERVICE
  channel" for a webhook and for a bot with no channel, used by
  `runAnnounce` for the `to` line (`:133`), the dry-run line (`:137`) and
  the done notice (`:149`) — where `Messaging.Target`,
  `internal/config/config.go:279`, says "(no channel set)" and, at `:284`,
  "the channel its webhook is bound to", which the interface's notice uses
  (`internal/tui/messagingpreview.go:217`), and the web's `AnnounceControls`
  says "Announced to SERVICE." for a webhook
  (`web/src/features/messaging/MessagingPanel.tsx:150`). With a bot token
  and no channel the command line's preview claims a channel that does not
  exist and the post then fails.
- `runAnnounce`, `internal/cli/announce.go:149`: "Announced to …" ends
  without a period, as do `offerLink`'s "Linked … on …"
  (`internal/cli/pr.go:199`) and `offerReviewStatus`'s "Moved … to …"
  (`:256`), while `offerLink`'s "Could not link … ." (`:194`),
  `offerToPost`'s "Posted to %s." (`internal/cli/standup.go:169`) and every
  web notice (`offerWords`, `web/src/features/review/OpenedOutcome.tsx:48`)
  carry one; a `pr --yes` run whose link fails mixes both.
- `placeholder`, `internal/tui/fields.go:101`: returns `dateLayout`, Go's
  reference date `2006-01-02` (`:36`), as the hint, while `errNeedsDate`,
  `:29`, says "must be a date like 2026-09-21" — the hint reads as a stale
  date rather than a shape.
- `Model.messagingDetail`, `internal/tui/messaging.go:167`: "SERVICE is
  not set up" and "to ~/" + `config.FileName`, with a literal newline
  mid-sentence that `wrap` re-breaks, where `messagingErrors`'
  `messaging.ErrNoCredential` wording, `internal/tui/failure.go:243`, words
  the same condition as "Messaging has no credential", names
  `token_command` and `token_env` as well, and names no file; `FileName`'s
  comment, `internal/config/config.go:15`, says the name serves both search
  locations.
- `prComposer.footer`, `internal/tui/prcomposer.go:263`: relabels
  `closeOverlay` "discard" while `handleKey` (`:278`) snapshots the draft to
  `m.prDraft`; `prEditor.footer`, `internal/tui/preditor.go:77`, says
  "discard" for an `esc` that does discard (`:86`); `commitComposer.footer`,
  `internal/tui/composer.go:246`, keeps the "close" label for a draft it
  also keeps. The same word means the opposite in two overlays a row apart,
  and the kept draft is unannounced.

**Instead.** Let `loop.ErrDirtyTree` carry the guidance once ("…; commit or
stash them before switching") and have both surfaces render it through
their failure voice, dropping the two local sentinels; drop `announceTarget`
for `seams.Messaging.Target()` on the `to` line, the dry-run line and the
done notice; end the three done notices with a period; one example in
`placeholder` and `errNeedsDate` (from the fake-able clock, or a plain
`YYYY-MM-DD` in both); `config.FileName` without the `~/` and the embedded
newline, letting the `messaging.ErrNoCredential` wording serve both places;
label the composer's `esc` "close" and reserve "discard" for overlays that
discard.

**Done when.** `grep -rn 'commit or stash'` finds one string outside tests;
`TestAnnounceDryRunComposesTheReadyMoment` sees `Target()`'s wording and
`grep -rn announceTarget internal/cli` finds nothing; the expected
"Announced to", "Linked", "Moved" and "Posted to" strings in the write
commands' tests all end with a period;
`TestATransitionFillsAUserDateAndSeveralVersions` and
`TestADateFieldRefusesWhatIsNotADate` agree on one example;
`TestTheSlackPaneNamesWhatItNeedsWhenUnset` refuses `~/` and the pane and
the failure wording name the same settings;
`TestAFailedPushKeepsThePullRequestDraft` asserts the footer's `esc` label
is not "discard".

## Ideas that would reopen a settled decision

None this edition. The six-pane rail (`internal/tui/panes.go:27`), once
written up as five in FEATURES.md, *is* the decision as built.
