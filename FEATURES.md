# Feature ideas

A brainstorm of what `workflow` could do next. Nothing here is promised,
scheduled or even agreed: it is a wide net, written down so that ideas are
argued over in one place instead of rediscovered.

Two readers are in mind. A contributor looking for something worth building,
and a later Claude Code session asked to "pick up FEAT-12". Each entry
therefore says why it matters, which files it would touch, and how to tell
when it is done.

Checked against commit `817d323` on 2026-09-17. Line numbers drift, so every
pointer also names the symbol it means.

## How to read an entry

- **Impact** and **Effort** are estimates, not measurements. Impact is how much
  of the daily loop it improves; effort is small (a day or less), medium (a
  few days) or large (a week or more, or a new package).
- **Why** describes the problem, because the
  [feature request template](.github/ISSUE_TEMPLATE/feature_request.yml) is
  right that the problem carries the weight, not the solution.
- **Touches** lists where the change would land. It is a starting point for
  reading, not a complete list.
- **Done when** is an observable result, phrased so a test can assert it.

The sections follow the order the work goes in, the same order as the panes:
Issues, Branch, Commits, Review, Slack. Entries are not ranked within a
section. Ranking is the maintainer's call.

Before building any of these, open an issue, as
[CONTRIBUTING.md](CONTRIBUTING.md) asks for anything significant. An entry
here is an invitation to make the case, not approval of it.

## What every idea has to respect

These are settled. An idea that fits them goes in the main sections. An idea
that needs one of them reopened goes in the
[last section](#ideas-that-would-reopen-a-settled-decision), and says which.

- **Nothing is stored between sessions.** Where you are in the work is read
  back from the branch name every time. There is no state file.
- **Every service is reached over the standard library's `net/http`.** `gh` is
  an optional source of a token and nothing more.
- **`git` is the only program that must be installed.** `lefthook` and an
  editor make more of the interface work, and their absence only hides the
  parts that need them.
- **Five panes down the left, one progress row across the top.** Focus is
  shown by the weight of a border, not by color.
- **Pure Go.** `CGO_ENABLED=0`, because the release cross-compiles to five
  platforms.
- **A new dependency needs approval first**, and a release younger than seven
  days is not adopted.
- **No credential is ever printed.** A change that touches a token brings the
  test that proves it does not leak.
- **One configuration file.** A file in the current directory replaces the one
  in the home directory, and an unknown key is an error.

## Issues

### FEAT-04 Open or copy a link

Impact: high · Effort: small

- Why: nothing in the interface can be opened in a browser or copied. The
  Review pane prints the pull request's URL, and while the mouse is captured
  the terminal cannot even select it: `m` has to turn capture off first. An
  issue's page is one click away in every other tool.
- Touches: `internal/tui/keys.go`, each pane's keys, `internal/proc` for the
  opener, `internal/tui/deps.go` (a new seam).
- Constraints: copying can use the OSC 52 terminal sequence, which needs no
  program and works over SSH. Opening needs the platform's opener (`open`,
  `xdg-open`, `rundll32`), which must stay optional: without one, the key is
  not offered.
- Done when: `o` opens the selected issue or the branch's pull request, `y`
  copies its URL, and both are absent from the bottom row when they cannot
  work.

### FEAT-05 Offer the status change the loop implies

Impact: high · Effort: medium

- Done: after a branch is created for an issue in a "to do" category, the status
  picker opens on it pre-selected on the first "in progress" transition — chosen
  by status category, never by a localized name — and `esc` leaves the issue
  alone. Offered, never applied (`branchCreated.apply`, `Model.pickStatusFor` and
  `firstInProgress` in `internal/tui/branchresult.go` and `picker.go`).
- Still open: the pull-request moment. "In review" shares the *category*
  (indeterminate) with "in progress", so the category rule cannot single it out;
  that moment wants a configured status name, which is a change of its own.

### FEAT-06 See the whole issue

Impact: medium · Effort: medium

- Why: the detail pane shows the description and the last five comments
  (`commentsShown = 5`, `internal/tui/detail.go:31`). Subtasks, links, the
  parent, the sprint, attachments and older comments are invisible, so the
  browser tab the tool set out to replace stays open.
- Touches: `internal/jira/detail.go` (requested fields),
  `internal/tui/detail.go`.
- Done when: the detail pane lists subtasks and linked issues with their
  status, and every comment can be reached by scrolling.

### FEAT-07 Fill more kinds of transition field

Impact: medium · Effort: medium

- Why: a field that is a user picker, a date or a cascading select is
  `FieldUnsupported` (`internal/jira/fields.go`), and the transition is refused
  with "make this change in Jira". A multi-value field takes one value. Text
  is one line.
- Touches: `internal/jira/fields.go`, `internal/tui/fields.go`.
- Done when: a transition that requires an assignee, a date or two fix
  versions can be completed without leaving the terminal.

### FEAT-08 Write comments in Markdown

Impact: medium · Effort: medium

- Why: Jira Data Center reads wiki markup, and a developer's hands write
  Markdown. A fenced code block posted as a comment arrives as three backticks.
- Touches: a small converter beside `internal/jira/detail.go` (`AddComment`),
  `internal/tui/comment.go` (the preview should show what Jira will show).
- Constraints: convert a short, certain list (code, links, lists, emphasis,
  headings) and pass anything else through untouched.
- Done: opt-in `jira.markdown_comments` rewrites a comment written in Markdown as
  wiki markup before posting — fenced blocks to `{code}`, links to `[text|url]`,
  emphasis, headings, lists and inline code — while a code span shields its
  contents (`jira.WikiFromMarkdown` in `internal/jira/wiki.go`, applied in
  `AddComment`). Off by default, so an instance already writing wiki markup is
  left untouched; the comment editor's help names which markup is in force.

### FEAT-09 Assign an issue to yourself

Impact: medium · Effort: small

- Why: with views (FEAT-02) the list can show unassigned work, and the next
  thing anyone does with unassigned work is take it.
- Touches: `internal/jira` (a new `Assign`), `internal/tui/deps.go`
  (`JiraDeps`), `internal/tui/detail.go`.
- Done when: `a` on an unassigned issue previews "assign PROJ-412 to you",
  `enter` does it, and dry run reports it instead.

### FEAT-10 Log time against an issue

Impact: low · Effort: small

- Why: teams that bill or report by worklog make every developer open Jira
  once per issue just to type "2h".
- Touches: `internal/jira` (a new `AddWorklog`), `internal/tui/detail.go`.
- Done when: a key opens a one-line input that accepts Jira's own duration
  syntax, previews it, and posts it.

### FEAT-11 Create an issue

Impact: medium · Effort: large

- Why: a bug found mid-task is either written down now or lost. Today it means
  the browser.
- Touches: `internal/jira` (create metadata and create), a new overlay in
  `internal/tui`, reusing the field form in `internal/tui/fields.go`.
- Constraints: required fields differ by project and type, which is the same
  problem the transition form already solves for a smaller set of kinds.
- Done when: a bug can be filed with a project, a summary and a description
  written in the editor, and it appears in the list.

## Branch

### FEAT-15 Work with a fork

Impact: medium · Effort: medium

- Why: the remote is `origin` everywhere (`internal/gitrepo/gitrepo.go`,
  `internal/gitrepo/branch.go`, `internal/tui/branch.go`,
  `internal/tui/prcomposer.go`). With a fork, the base should come from
  `upstream` and the push should go to `origin`, and an open-source
  contributor cannot use the tool at all.
- Touches: the files above; `internal/forge/pulls.go` (a head on another
  repository).
- Constraints: read git's own answers first (`remote.pushDefault`,
  `branch.<name>.pushRemote`) before adding a setting.
- Done when: in a clone with `origin` and `upstream`, the base is
  `upstream/main`, the push goes to the fork, and the pull request opens
  against upstream.

### FEAT-17 Finish a merged branch

Impact: medium · Effort: small

- Why: the loop ends at "announce", but the work ends when the pull request
  merges: switch to the default branch, pull, delete the branch. Nothing here
  notices a merge.
- Touches: `internal/forge` (a merged state on `PullRequest`),
  `internal/tui/review.go`, `internal/gitrepo/branch.go`.
- Done when: a branch whose pull request has merged offers one action that
  previews the three git commands and runs them.

## Commits

### FEAT-18 See what changed before staging it

Impact: high · Effort: medium

- Why: the Commits pane lists file names and status letters. Deciding whether
  to stage `README.md` means opening another terminal to read the diff.
- Touches: `internal/gitrepo/status.go` (a `Diff`), `internal/tui/commits.go`,
  `internal/sanitize` (a diff is outside text).
- Constraints: read-only. Staging by hunk stays out, as the usage guide says:
  that is lazygit's whole project.
- Done: the Commits pane reads the selected file's diff (`gitrepo.Diff`, an
  untracked file noted rather than shown) and draws it below the list, headed by
  the path; it scrolls with the pane, and an added line keeps its `+` and a
  removed its `-`, tinted green and red so the mark reads without color too.

### FEAT-20 Mark a breaking change

Impact: medium · Effort: small

- Why: the composer sets `Breaking: false` unconditionally
  (`internal/tui/composer.go:92`) although `convention.Subject` can write the
  `!`. In this repository that marker decides the version bump, and its own
  commit hook refuses a breaking body without it.
- Touches: `internal/tui/composer.go`, `internal/tui/keys.go`.
- Done when: a key toggles `!` in the subject preview and asks for the
  `BREAKING CHANGE:` paragraph in the body.

### FEAT-21 Suggest a scope

Impact: low · Effort: small

- Why: the scope is typed from memory every time, and the two best guesses
  are free: the directory the staged files share, and the scopes already in
  `git log`.
- Touches: `internal/tui/composer.go`, `internal/gitrepo/branch.go`.
- Done: the scope field completes from the name of the directory the staged
  files share and from the scopes already in `git log` (`gitrepo.RecentSubjects`
  → `convention.Scopes` → `scopeSuggestions`), and `tab` accepts a pending
  completion or moves on when there is nothing to take.

### FEAT-22 Amend and fix up

Impact: medium · Effort: medium

- Why: a review comment means either a new "address review" commit or a trip
  to the shell. With rebase-only merging, as this repository uses, every
  commit lands as written, so tidy history matters.
- Touches: `internal/gitrepo/branch.go`, `internal/tui/commits.go`,
  `internal/tui/composer.go`.
- Constraints: only for commits that are not pushed, or say plainly that the
  next push must be forced, and never force by default.
- Done: the Commits pane offers `A` to amend the last commit and `f` to record
  a `fixup!` of a chosen one — both previewed, both running the hooks, both
  dry-runnable. Offered only while there is an unpushed commit to fold into
  (`gitrepo.Branch.Unpushed`), so history that is already on the remote is never
  rewritten and nothing is force-pushed.

### FEAT-23 Unstage everything, and discard a change

Impact: medium · Effort: small

- Why: `a` stages every file and nothing reverses it. A stray edit can only be
  dropped from a shell.
- Touches: `internal/gitrepo/status.go`, `internal/tui/commits.go`.
- Constraints: discarding destroys work, so it previews the file and needs
  `enter`, unlike staging.
- Done when: one key unstages all, and another discards the selected file's
  changes after a confirmation.

### FEAT-24 Run any hook, not only pre-commit

Impact: low · Effort: small

- Why: `h` runs `pre-commit`. The slow checks usually live in `pre-push`, and
  the only way to try them is to push.
- Touches: `internal/tui/commits.go` (`runPreCommit`),
  `internal/wiring/wiring.go`.
- Done when: the hooks lefthook configures can be chosen and run from the
  pane.

### FEAT-25 Recognize more failure formats

Impact: medium · Effort: small

- Why: a failed hook lists the `file:line` places it can parse, and `enter`
  opens the editor there. The pattern needs a file extension and a colon
  form, so `Dockerfile:3`, TypeScript's `file.ts(12,5)`, ESLint's stylish
  output and a Python traceback all fall through to raw output.
- Touches: `internal/hooks/output.go` (`Locations`), and its table test.
- Done when: each new format has a table case and opens at the right line.

### FEAT-26 Add co-authors and a sign-off

Impact: low · Effort: small

- Done: the `Refs:` trailer now stays inside git's trailer block
  (`trailerJoin`/`endsWithTrailerBlock`, `internal/convention/convention.go`), so
  `git interpret-trailers --parse` reads it alongside a `Co-authored-by:` the
  author typed by hand.
- What remains: picking recent authors as co-authors and a key that toggles
  `Signed-off-by:`. `convention.Message` takes only the subject, body and issue
  key, and the composer's fields are type, scope, subject and body.
- Done when: recent authors can be picked as co-authors and a toggle signs off.

### FEAT-14 Conventions a team can change

Impact: low · Effort: large

- Why: the commit types, the 72- and 48-character subject and body limits, the
  `fix/` and `feat/` branch prefixes, the `Refs:` trailer and the title taken
  from the oldest commit are all constants. A team with other conventions has
  nowhere to set them.
- Touches: `internal/convention/convention.go`, a new section in
  `internal/config/config.go`, and every caller that reads a rule as a literal.
- Constraints: the defaults stay as they are, so a repository with no
  configuration behaves exactly as it does now.
- Done when: a repository can set its own commit types and limits, and the
  composer and the branch names honor them.

## Review

### FEAT-27 Reviewers, assignees and labels

Impact: high · Effort: medium

- Why: `NewPullRequest` carries a title, a body, two branches and a draft flag
  (`internal/forge/pulls.go`). Asking for a review, which is the point of
  opening one, still happens in the browser.
- Touches: `internal/forge/pulls.go`, `internal/forge/github.go`,
  `internal/forge/gitlab.go`, `internal/tui/prcomposer.go`.
- Constraints: both forges or neither. GitHub takes reviewers in a second
  request; GitLab takes them at creation. `CODEOWNERS` is a free first
  suggestion.
- Done when: the composer suggests reviewers, the opened pull request has
  them, and a failure to add one does not lose the pull request.

### FEAT-30 Edit a pull request after opening it

Impact: medium · Effort: medium

- Why: `n` is offered only while no pull request exists. A typo in the title,
  a description written too early, or a draft that is ready all need the
  browser.
- Touches: `internal/forge/pulls.go` (an update), `internal/tui/prcomposer.go`.
- Constraints: GitHub marks a draft ready only through its GraphQL API, which
  is still a plain POST over `net/http`.
- Done when: the composer opens on the existing pull request, and saving
  updates it.

### FEAT-31 Merge

Impact: medium · Effort: medium

- Why: the last outward step of the loop is a button in a browser.
- Touches: `internal/forge` (merge, and which methods the repository allows),
  `internal/tui/review.go`.
- Constraints: previewed like every other write; offered only when the forge
  says it can merge; uses the method the repository permits.
- Done when: a green, approved pull request can be merged after a preview,
  and FEAT-17 follows.

### FEAT-32 Run failed checks again

Impact: low · Effort: small

- Why: a flaky job is a browser trip and three clicks.
- Touches: `internal/forge/github.go`, `internal/forge/gitlab.go`,
  `internal/tui/review.go`.
- Constraints: needs a token scope the read path does not; say so when it is
  missing.
- Done when: a key on a failed pull request restarts the failed jobs and the
  pane returns to "running".

### FEAT-35 Choose the base from the branches that exist

Impact: low · Effort: small

- Why: the base is a free-text field. A stacked branch, or a release branch,
  is typed from memory and checked by the forge's error.
- Touches: `internal/tui/prcomposer.go`, `internal/gitrepo/branch.go`.
- Done: the base field offers the remote branches as completions and `tab`
  accepts the match (`gitrepo.RemoteBranches` lists them by name, prefix dropped
  and deduped; `prComposer.onFieldNav` completes on tab or moves on when there is
  nothing to take). `tab` keeps its field-navigation meaning when no completion
  is pending.

## Slack

### FEAT-38 Announce more than "opened"

Impact: medium · Effort: medium

- Why: the moments a team cares about are "ready for review", "merged" and
  "CI is red on main". Only the first exists.
- Touches: `internal/slack/post.go`, `internal/tui/slack.go`,
  `internal/tui/review.go`.
- Constraints: each is its own message. Replying in a thread is in the last
  section, because it needs somewhere to keep a timestamp.
- Done when: a merged pull request offers a "merged" post with the same
  preview and the same dry-run behavior.

## Across the loop

### FEAT-41 Steps you can script

Impact: high · Effort: large

- Why: every step exists only inside the interface. A shell alias, a git
  hook, a CI job or an editor plugin cannot say "branch for PROJ-412" or "open
  the pull request".
- Touches: `internal/cli` (new commands), `internal/wiring/wiring.go` (the
  seams already exist as plain functions), `docs/` through `task docs:gen`.
- Constraints: the same previews, as printed text with a `--yes` to skip them;
  the same dry run.
- Done when: `workflow branch PROJ-412`, `workflow pr` and `workflow announce`
  do what their panes do, with no terminal interface.

### FEAT-44 Find the configuration from a subdirectory

Impact: medium · Effort: small

- Done: `config.Discover` walks up from the working directory to the repository
  root — the directory holding `.git` — before falling back to home, so a
  session in a subdirectory reads the repository's own file rather than skipping
  it for the one at home. `config.RepoRoot` makes `config init` write at that
  root, where every subdirectory can see it; outside a repository both keep
  their old behavior.

### FEAT-48 Keys you can change

Impact: low · Effort: medium

- Why: `newKeyMap` (`internal/tui/keys.go`) is literals. A user whose terminal
  eats `ctrl+e`, or who wants `g` and `G`, has no recourse.
- Touches: `internal/tui/keys.go`, `internal/config/config.go`.
- Constraints: refuse a configuration where two actions in one context share
  a key.
- Done when: a `ui.keys` map overrides a binding and the help shows the new
  key.

### FEAT-49 Completion that knows your issues

Impact: low · Effort: small

- Why: Cobra already provides `workflow completion` and nothing mentions it.
  With FEAT-41 it could complete issue keys.
- Touches: `docs/content/docs/install.md`, `cmd/docsgen/main.go` (the
  reference leaves the command out), `internal/cli`.
- Done when: the install page says how to turn completion on, and
  `workflow branch <tab>` offers assigned issue keys.

### FEAT-51 A configuration that can change shape

Impact: medium · Effort: medium

- Why: the README says the format may change before 1.0, the decoder rejects
  unknown keys, and there is no version field. The first renamed key breaks
  every existing file with `json: unknown field`.
- Touches: `internal/config/config.go`, `internal/cli/config_cmd.go`.
- Done when: a file written for an older shape is either read or refused with
  the name of the key that replaced the old one.

### FEAT-52 Packages

Impact: medium · Effort: medium

- Why: installing means `go install` or downloading a binary and checking a
  checksum by hand.
- Touches: `.github/workflows/release.yml`, `docs/content/docs/install.md`.
- Constraints: publishing is the maintainer's to do. This is preparation only.
- Done when: a Homebrew formula and a Scoop manifest are generated by the
  release and documented.

## New integrations

### FEAT-53 Jira Cloud

Impact: high · Effort: large

- Why: the client speaks Data Center's REST v2 only. Cloud moved search to
  `/search/jql`, writes descriptions and comments as Atlassian Document
  Format, and signs in with an email and an API token.
- Touches: `internal/jira` (a second client or a dialect, as `internal/forge`
  has), `internal/config/config.go`, `internal/wiring/wiring.go`.
- Constraints: `tui.JiraDeps` is already plain functions, so the interface
  does not change. The work is the wire format.
- Done when: the whole loop runs against a Cloud site, and `doctor` says which
  kind of Jira it found.

### FEAT-54 Bitbucket

Impact: high · Effort: large

- Why: a company that runs Jira on its own servers very often runs Bitbucket
  Data Center beside it. That is the audience this tool was written for, and
  they have no forge here.
- Touches: `internal/forge` throughout. The `dialect` struct
  (`internal/forge/pulls.go`) covers find, create and status; the API base,
  the token variables, the templates and the `Kind` parsing live elsewhere and
  each needs a case.
- Done when: a Bitbucket remote is recognized, a pull request can be opened,
  and its build status is followed.

### FEAT-55 Gitea and Forgejo

Impact: medium · Effort: medium

- Why: self-hosters. The API was modeled on GitHub's, so most of a dialect
  can be shared.
- Touches: `internal/forge`.
- Done when: `forge.kind: gitea` runs the Review pane end to end.

### FEAT-57 Linear

Impact: medium · Effort: large

- Why: the tracker many small teams use. Its keys already look like `ENG-123`,
  so branch naming needs nothing.
- Touches: a new client package, `internal/wiring`.
- Constraints: its API is GraphQL, which is a POST and a JSON body over
  `net/http`. No client library is needed.
- Done when: the Issues pane lists assigned Linear issues and a status change
  works.

### FEAT-58 Teams, Discord and others

Impact: medium · Effort: medium

- Why: the last step assumes Slack. Many Jira Data Center shops are Microsoft
  shops.
- Touches: `internal/slack` (or a sibling package), `internal/config/config.go`,
  `internal/tui/slack.go`, `internal/tui/panes.go` (the pane's name).
- Constraints: a generic "POST this JSON template to this URL" covers most
  chat webhooks with one implementation. The URL is a credential and is
  masked like `slack.webhook_url`.
- Done when: a Teams webhook receives the announcement, and the pane is
  titled for the service in use.

## Beyond the loop

### FEAT-59 Pull requests waiting on you

Impact: high · Effort: medium

- Done: `forge.Client.ReviewRequests` searches each forge for pull requests that
  request your review, `workflow reviews` (`internal/cli/reviews.go`) lists each
  one's title, author, CI word and humanized age, and the Reviews pane
  (`internal/tui/reviewqueue.go`, `6`) shows the same queue oldest-first, opening
  or copying the selected request with `o` and `y`.

### FEAT-63 The active sprint

Impact: low · Effort: medium

- Why: "what is left this sprint" is a board in a browser.
- Touches: `internal/jira` (the Agile API), `internal/tui/issues.go`.
- Done when: a view (FEAT-02) shows the active sprint grouped by status.

## Build and platform

Mac and Linux are the primary targets; Windows is secondary, run as the
cross-compiled binary rather than through an installer. These are the platform
and build-reproducibility items the debt file tracked — kept here because none
can be exercised from a developer's own machine.

### FEAT-73 Windows as a tested target

Impact: low · Effort: medium

- Why: the code carries Windows branches — a plain file counts as a runnable
  hook, the editor falls back to `notepad`, a `C:\` path is read as one place,
  and the process group has a no-op twin where a Unix session does not apply —
  but none runs on Windows, so each branch is tested only from one machine by
  its GOOS. The process-group kill has no Windows job-object twin.
- Touches: `.github/workflows/ci.yml` (a Windows leg), `internal/proc/pgroup`
  (a job-object `Isolate`), and whatever the first real run turns up.
- Constraints: a Windows leg must be watched and iterated on a real runner, not
  added blind where it would sit red.
- Done when: the suite runs on a Windows runner in CI, and stopping a run kills
  its child processes there as it does on Unix.

### FEAT-74 The container gate is the host gate

Impact: low · Effort: medium

- Why: `build/Dockerfile` provisions some tools from `apt` (jq, node,
  shellcheck) rather than the `mise.toml` pins, gives `GO_VERSION` a default of
  its own, and does not checksum the tools it downloads, so `task
  container:check` can pass against versions the host gate never saw.
- Touches: `build/Dockerfile`, `scripts/tool-versions.sh` (which passes only
  some of the pins today), `.github/workflows/container.yml`.
- Constraints: needs a machine with a container runtime; the weekly
  `container.yml` run is where this is exercised and driven.
- Done when: every tool in the container comes from a `mise.toml` pin with a
  verified download, and the three wiring tests that skip for want of lefthook
  inside the image no longer have to.

### FEAT-75 Guard the release tag on a green gate

Impact: low · Effort: small

- Why: the release-tag script pushes the tag before `release.yml` runs `task
  check`, so a red gate could leave a tag with no release behind it.
- Touches: `scripts/release/push-release-tag.sh`,
  `.github/workflows/release.yml`.
- Constraints: this is the maintainer's release path and cannot be exercised
  without a real release.
- Done when: the tag is pushed only once the gate has passed.

## Ideas that would reopen a settled decision

Each of these is a reasonable thing to want. Each also needs one of the
settled decisions above to be argued again, so it is fenced off here with the
decision named. Where a version exists that fits the decision, it is listed
first.

### FEAT-64 Reply in a thread

Impact: medium · Effort: medium

- Reopens: nothing stored between sessions.
- Why: "CI failed" and "merged" belong under the announcement, not beside it.
  Threading needs the first message's timestamp, and there is nowhere to keep
  it.
- A version that fits: with a bot token, find the earlier message by searching
  the channel's recent history for the pull request's URL. It costs a
  `channels:history` scope and a request, and it cannot work with a webhook.
- Done when: a later post about the same pull request arrives as a reply.

### FEAT-65 A queued post that survives quitting

Impact: medium · Effort: large

- Reopens: nothing stored between sessions, and a single process that ends
  when the interface closes.
- Why: "post when CI passes" is dropped if you quit first, which the usage
  guide lists as a limit. CI takes longer than most people keep a terminal
  open.
- A version that fits: FEAT-41 makes `workflow announce --when-green` a
  foreground command that a shell can background.
- Done when: a post queued before quitting is sent when CI passes.

### FEAT-66 Remember what was announced

Impact: low · Effort: small

- Reopens: nothing stored between sessions.
- Why: after a restart the Slack pane says "nothing posted" for a pull request
  that was announced an hour ago, and offers to announce it again.
- A version that fits: the same history search as FEAT-64.
- Done when: a pull request announced in an earlier session shows as posted.

### FEAT-67 Remember choices

Impact: low · Effort: small

- Reopens: nothing stored between sessions.
- Done, the version that fits: `commit.default_scope` is a configured default the
  composer pre-fills (`Model.startingScope`, `internal/tui/composer.go`) — chosen,
  not learned.
- Still a non-goal: remembering the scope *last used* in this repository across
  sessions, which would need the state file the settled decision rules out.

### FEAT-68 Start instantly from a cache

Impact: low · Effort: medium

- Reopens: nothing stored between sessions. It also puts issue text at rest
  on disk, which the security policy would then have to cover.
- Why: every start waits on Jira before the first pane is useful.
- Done when: the list shows at once from the last session and updates when
  the answer arrives.

### FEAT-70 A sixth pane

Impact: medium · Effort: medium

- Done: the rail has a sixth pane, Reviews (`internal/tui/reviewqueue.go`), and
  `6` jumps to it; it holds the review queue (FEAT-59). The pane count is driven
  by `paneCount` in `internal/tui/panes.go`, so the layout, the jump keys and the
  focus lap all followed from the one edit.
- Still open, the version that fits check details (FEAT-28): an overlay rather
  than a further pane.

### FEAT-72 Notifications after the interface closes

Impact: low · Effort: large

- Reopens: a single process that ends when the interface closes.
- Why: CI results and review requests arrive when nobody is looking at the
  terminal.
- Done when: a background process raises a desktop notification for a
  finished CI run.

### FEAT-76 A local web mode

Impact: medium · Effort: large

- Reopens: no server, and no frontend. NOT persistence — `--web` fetches live
  and writes only the config file, so nothing new is stored between sessions
  and no database is added; the binary stays a single static file with the
  frontend embedded.
- Why: some people would rather see the loop — issues, branch, changes, PR/CI,
  Slack — and edit the whole configuration in a browser than in the terminal,
  and a richer surface (forms, history views) is easier to grow there.
- The shape: `workflow --web` serves a React + TypeScript app on
  `127.0.0.1:7000` only, over a REST API described by `api/openapi.yaml` (the Go
  server and the typed client both generated from it). It reuses the same
  `wiring.Deps` seams the TUI and `workflow status`/`reviews` already use, so it
  is another consumer of the domain, not a second implementation. The TUI and
  CLI stay the default; the web mode is opt-in behind the flag.
- Done when: `workflow --web` shows the live loop and round-trips the
  configuration in the browser, and the default binary is unchanged.

### FEAT-77 Switch between issues by their branches, in the web

Impact: medium · Effort: medium

- Builds on FEAT-76, and is taken up only after the web read surface is
  complete. The read half fits the settled invariants: where you are is read
  back from the branch names, exactly as "Nothing is stored between sessions"
  says. The write half is the first of the web mode's deferred write actions.
- Why: the TUI's task switcher (`internal/tui/switchtask.go`) lists your local
  branches — each named for its issue — and checks one out to switch tasks. The
  web sees only the checked-out branch, so its per-issue work story shows one
  issue in flight and the rest not started, where the TUI shows every in-flight
  item. There is no database to add: the local branches are the record, and the
  issue↔branch link is the branch name (`convention.IssueKey`).
- The shape:
  - Read (fits v1, no new persistence): add the local task-branches — which
    issues have a branch, and its state — to the API, as a snapshot field or an
    endpoint, so the Issues list marks in-flight issues and each shows its own
    work story rather than the checked-out one alone.
  - Write (later phase): "open" an issue checks out, or creates, its branch,
    guarded by the same dirty-tree refusal the TUI uses (`errDirtyTree`). Part
    of the web write-actions phase, not the read surface.
- Touches: `api/openapi.yaml`, `internal/webserver`, `internal/wiring` (its
  `Branches` and `Checkout` seams already exist), `web/src/features/issues`.
- Done when: the web Issues list shows more than one issue in flight when more
  than one local branch names an issue, each with its own work story; and, in
  the write phase, choosing an issue checks out its branch.
