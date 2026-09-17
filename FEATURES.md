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

### FEAT-01 Filter the issue list as you type

Impact: high · Effort: small

- Why: the list is whatever one query returned, and the only way through it is
  `j` and `k`. Past a screenful, finding `PROJ-388` means reading every row.
- Touches: `internal/tui/issues.go` (`issueList`), `internal/tui/keys.go`,
  `internal/tui/detail.go` (`issuesKeys`).
- Constraints: filter what is already loaded; no new request per keystroke.
- Done when: `/` opens a filter over key and summary, the list narrows as you
  type, `esc` restores the full list with the selection kept, and the bottom
  row shows the filter while it is active.

### FEAT-02 Named issue views

Impact: high · Effort: medium

- Why: the working list is one constant,
  `assignee = currentUser() AND statusCategory != done`
  (`AssignedToMe`, `internal/jira/search.go:22`). Someone who picks work from a
  sprint, a team filter or the unassigned pile cannot see it.
- Touches: `internal/config/config.go` (a `jira.views` list of name and JQL),
  `internal/jira/search.go`, `internal/wiring/wiring.go` (`jiraDeps` passes
  the constant today), `internal/tui/issues.go`,
  `docs/content/docs/configuration.md`.
- Constraints: `Search` already takes any JQL, so this is configuration and a
  key to move between views. The empty-state text "no open issues assigned to
  you" (`internal/tui/issues.go`) has to stop assuming the query.
- Done when: a configuration with two views shows the first at start, a key
  moves between them, and the pane title names the view in use.

### FEAT-03 Load past the first fifty issues

Impact: medium · Effort: small

- Why: `searchLimit = 50` (`internal/jira/search.go:34`) with no paging. The
  pane admits it with "showing 50 of 212" and offers no way to see the rest.
- Touches: `internal/jira/search.go` (`startAt`), `internal/tui/issues.go`.
- Done when: reaching the end of a truncated list loads the next page, and the
  count line reflects what is loaded.

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

- Why: the loop has natural moments for a status change (branching means work
  started; a pull request means it is in review), and today each one is a trip
  back to pane 1, `t`, and a search through the transitions.
- Touches: `internal/tui/branch.go` (`branchCreated`),
  `internal/tui/prcomposer.go` (`pullCreated`), `internal/tui/picker.go`.
- Constraints: offer, never apply. "Nothing outward facing is sent without a
  last look" is the interface's own rule. Choosing which transition to suggest
  needs either the target's status category or a configured name; a guess by
  transition name breaks on a localized Jira.
- Done when: after a branch is created for an issue in a "to do" category, the
  status picker opens with the first "in progress" transition selected, and
  `esc` leaves the issue alone.

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
- Done when: a comment written with a fenced block and a link previews and
  posts as `{code}` and `[text|url]`.

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

### FEAT-12 Fetch before branching

Impact: high · Effort: small

- Why: no code path runs `git fetch`. "The branch starts from origin's default
  branch" really means the remote-tracking ref as of the last time the user
  fetched, which can be days old. The new branch is then behind before its
  first commit, and the "behind" count is wrong in the same way.
- Touches: `internal/gitrepo/branch.go` (`ReadBranch`, `CreateBranch`),
  `internal/tui/branch.go` (`branchCreator`), `internal/tui/deps.go`
  (`GitDeps`).
- Constraints: a fetch cannot prompt, for the same reason a push cannot
  (`GIT_TERMINAL_PROMPT=0`). When it fails, say so and offer to branch from
  what is there.
- Done when: the overlay says how old the base is, fetches it before creating
  the branch, and under dry run says it would.

### FEAT-13 Switch to another task

Impact: high · Effort: medium

- Why: the branch is the state, so switching tasks is switching branches, and
  the interface cannot do it. An interruption means leaving for a shell.
- Touches: `internal/gitrepo/branch.go` (list and check out),
  `internal/tui/branch.go`, `internal/tui/deps.go`.
- Constraints: refuse on a dirty tree with the reason, and leave stashing to
  the user.
- Done when: the Branch pane lists local branches that name an issue, `enter`
  checks one out, and every pane reloads for it.

### FEAT-14 Team branch-name conventions

Impact: medium · Effort: medium

- Why: `BranchName` (`internal/convention/convention.go:70`) hardcodes `fix/`
  for an issue type that equals "bug" in English and `feat/` for everything
  else. A team that writes `bugfix/`, puts the key first, or runs Jira in
  German gets a name to retype every time.
- Touches: `internal/convention/convention.go`, `internal/config/config.go`,
  `docs/content/docs/configuration.md`.
- Constraints: `IssueKey` has to keep finding the key in whatever shape the
  template produces, because everything else is derived from it.
- Done when: a configured template and a type-to-prefix map produce the
  proposed name, and the default output is unchanged.

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

### FEAT-16 Catch up with the base

Impact: medium · Effort: medium

- Why: the pane reports `↑2 ↓5` against the base and offers nothing to do
  about the five.
- Touches: `internal/gitrepo/branch.go`, `internal/tui/branch.go`,
  `internal/tui/run.go` (it streams, like a push).
- Constraints: a rebase that stops on a conflict cannot be finished inside
  this interface. Detect it, say where things stand, and hand over to the
  shell.
- Done when: `u` rebases onto the fetched base, streams its output, and a
  conflict leaves the repository mid-rebase with a message that says so.

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
- Done when: the selected file's diff shows beside the list, scrolls, and
  marks added and removed lines by more than color.

### FEAT-19 Start the composer on the right type

Impact: medium · Effort: small

- Why: on branch `fix/PROJ-412-token-redaction` the composer opens on `feat`,
  because the type comes from the last draft or else the first in the list
  (`openCommitComposer`, `internal/tui/composer.go`). The branch already says
  what kind of change this is.
- Touches: `internal/tui/composer.go`, `internal/convention/convention.go`.
- Done when: a branch whose prefix is a commit type opens the composer on that
  type, and a kept draft still wins.

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
- Done when: the scope field offers completions and `tab` accepts one.

### FEAT-22 Amend and fix up

Impact: medium · Effort: medium

- Why: a review comment means either a new "address review" commit or a trip
  to the shell. With rebase-only merging, as this repository uses, every
  commit lands as written, so tidy history matters.
- Touches: `internal/gitrepo/branch.go`, `internal/tui/commits.go`,
  `internal/tui/composer.go`.
- Constraints: only for commits that are not pushed, or say plainly that the
  next push must be forced, and never force by default.
- Done when: the staged changes can be folded into the last commit, or
  recorded as a `fixup!` of a chosen one.

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

- Why: pairing and DCO projects need `Co-authored-by:` and `Signed-off-by:`
  trailers, typed by hand today, and the `Refs:` trailer is added as its own
  paragraph, which pushes other trailers out of git's trailer block.
- Touches: `internal/convention/convention.go` (`Message`),
  `internal/tui/composer.go`.
- Done when: recent authors can be picked as co-authors, a toggle signs off,
  and `git interpret-trailers --parse` reads every trailer the composer wrote.

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

### FEAT-28 See which check failed

Impact: high · Effort: medium

- Why: the pane says "failed (3 of 4 finished)". Which one, and why, is a
  browser tab.
- Touches: `internal/forge/ci.go` (`CI` holds counts only),
  `internal/forge/github.go`, `internal/forge/gitlab.go`,
  `internal/tui/review.go`.
- Done when: every check is listed with its state, and `enter` on one opens
  its page (FEAT-04) or shows the end of its log.

### FEAT-29 Review state at a glance

Impact: medium · Effort: medium

- Why: after opening, the only thing followed is CI. Approvals, requested
  changes, unresolved threads and merge conflicts decide what happens next and
  none of them are shown.
- Touches: `internal/forge/pulls.go` (`PullRequest`), `internal/tui/review.go`.
- Done when: the pane shows approvals and whether the branch can merge, and
  the progress row's Review stage reflects "changes requested".

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

### FEAT-33 Say when CI finishes

Impact: medium · Effort: small

- Why: CI is polled every twenty seconds and the result changes a glyph. A
  developer who switched windows finds out by checking back.
- Touches: `internal/tui/review.go` (`ciChecked`), `internal/config/config.go`
  (a `ui` setting).
- Constraints: the terminal bell and the OSC 9 notification sequence need no
  dependency. Off unless configured.
- Done when: with the setting on, a change from running to passed or failed
  rings once.

### FEAT-34 Link the pull request on the issue

Impact: high · Effort: small

- Why: the Slack post links the issue and the pull request to each other, and
  Jira, where the rest of the team looks, learns nothing unless an integration
  is installed on the server.
- Touches: `internal/jira` (a remote link or a comment),
  `internal/tui/prcomposer.go` (`pullCreated`).
- Constraints: previewed, like the comment it is.
- Done when: after a pull request opens, one confirmation adds its link to the
  issue, and dry run reports it.

### FEAT-35 Choose the base from the branches that exist

Impact: low · Effort: small

- Why: the base is a free-text field. A stacked branch, or a release branch,
  is typed from memory and checked by the forge's error.
- Touches: `internal/tui/prcomposer.go`, `internal/gitrepo/branch.go`.
- Done when: the base field completes from remote branches.

## Slack

### FEAT-36 Choose the channel when posting

Impact: medium · Effort: small

- Why: a bot token can post anywhere it is invited, and `slack.channel` is one
  string. A change that concerns another team goes to the wrong room or is
  pasted by hand.
- Touches: `internal/config/config.go`, `internal/slack/post.go`,
  `internal/tui/slack.go` (`slackPreview`).
- Done when: the preview's destination can be changed to another configured
  channel before posting.

### FEAT-37 A team's own words

Impact: medium · Effort: medium

- Why: the announcement is one sentence built in code (`Announcement.Text`,
  `internal/slack/post.go`). Teams have house styles: an emoji, a reviewers
  line, a mention of a group.
- Touches: `internal/slack/post.go`, `internal/config/config.go`.
- Constraints: every substituted value stays escaped. The escaping exists
  because a pull request title containing `<!channel>` would otherwise ping
  everyone.
- Done when: a configured template with named placeholders produces the
  preview, and a test shows a hostile title cannot break out of it.

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

### FEAT-40 Say which version this is

Impact: high · Effort: small

- Why: `workflow --version` answers "unknown flag", and the bug report
  template requires its output.
- Touches: `internal/cli/cli.go`, `internal/cli/doctor.go`, then
  `task docs:gen`.
- Constraints: `debug.ReadBuildInfo` needs no linker flags and works for
  `go install`.
- Done when: `--version` prints the module version or the commit, and `doctor`
  prints it first.

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

### FEAT-42 One line of status

Impact: medium · Effort: small

- Why: the progress row is the most useful thing on the screen and it is only
  visible while the interface is open. A shell prompt or a tmux status line
  could carry it.
- Touches: `internal/cli`, `internal/tui/spine.go` (the stage logic would
  move somewhere both can use).
- Done when: `workflow status` prints the issue, the stage glyphs and the CI
  state on one line, and `--json` prints the same as data.

### FEAT-43 Keep tokens out of the file

Impact: high · Effort: medium

- Why: `.workflow.json` holds three live credentials in plain text. Mode
  `0600` is the whole defense, and a repository-local file is one careless
  `git add -A` from a commit.
- Touches: `internal/config/config.go`, `internal/proc`,
  `docs/content/docs/configuration.md`.
- Constraints: a `token_command` that runs a program (`pass`, `op`,
  `security`) keeps pure Go and adds no dependency, the same way `gh auth
  token` is used today. Environment variables are the simpler half. Either
  way the value goes through `config.Redact` and the change brings its leak
  test.
- Done when: a token can come from a command or a variable, `doctor` says
  which source was used without showing the value, and the file can hold no
  secret at all.

### FEAT-44 Find the configuration from a subdirectory

Impact: medium · Effort: small

- Why: the file is looked for in the current directory and then the home
  directory. Started from `src/`, a repository's own configuration is skipped
  in silence for the one at home.
- Touches: `internal/cli/cli.go` (`loadFromEnvironment`),
  `internal/wiring/wiring.go` (`Locate` already finds the root).
- Done when: running in any directory of a repository reads the file at its
  root, and `doctor` names the file it read.

### FEAT-45 A guided first run

Impact: high · Effort: medium

- Why: `config init` writes a template of empty strings and the user edits
  JSON by hand, then runs `doctor`, then `doctor --online`. The three steps
  could be one conversation that checks each answer as it is given.
- Touches: `internal/cli/config_cmd.go`, `internal/cli/doctor.go`.
- Constraints: a token typed at a prompt must not echo.
- Done when: `config init` asks for the Jira address and token, checks them
  online, does the same for Slack, warns when the file would not be ignored by
  git, and writes only what passed.

### FEAT-46 Machine-readable doctor

Impact: low · Effort: small

- Why: `doctor` is what bug reports paste, and what a setup script would want
  to check. It prints aligned prose only.
- Touches: `internal/cli/doctor.go`.
- Done when: `doctor --json` prints the same facts as data, with the same
  masking.

### FEAT-47 Timing you can set

Impact: low · Effort: small

- Why: every request has ten seconds (`RequestTimeout`,
  `internal/wiring/wiring.go`) and CI is asked every twenty
  (`defaultCIInterval`, `internal/tui/deps.go`). A Jira behind a slow VPN, or
  a forge with a tight rate limit, needs different numbers. The interval's
  seam already exists; wiring always passes zero.
- Touches: `internal/config/config.go`, `internal/wiring/wiring.go`.
- Done when: both can be set, and a nonsense value is refused at load.

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

### FEAT-50 A log for bug reports

Impact: medium · Effort: medium

- Why: when Jira answers something unexpected, the user sees one sentence and
  the maintainer sees nothing. There is no way to find out what was asked.
- Touches: `internal/wiring/wiring.go` (wrap each `Doer`),
  `internal/cli/cli.go`.
- Constraints: method, path, status and duration only. Never a header, a
  body or a query string, and the test that proves it comes with it.
- Done when: `--log FILE` records each request's outline and no credential
  appears in the file.

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

### FEAT-56 Issues from the forge itself

Impact: medium · Effort: large

- Why: an open-source project has no Jira. Its issues are on GitHub or
  GitLab, where the tool already has a credential.
- Touches: `internal/convention/convention.go` (`IssueKey` assumes `ABC-123`),
  `internal/tui/deps.go` (`JiraDeps` becomes a tracker), `internal/wiring`.
- Constraints: the key must still be readable back out of the branch name.
  `42-fix-typo` is how GitLab itself names such branches.
- Done when: with no Jira configured and a GitHub remote, the Issues pane
  lists assigned GitHub issues and the loop completes.

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

Impact: high · Effort: large

- Why: half of a developer's review time is other people's work, and the tool
  knows nothing about it.
- Touches: `internal/forge` (search by requested reviewer), a new screen in
  `internal/tui`.
- Constraints: the five-pane rail is settled, so this is a separate screen or
  command, not a sixth pane.
- Done when: a list of pull requests that request your review shows title,
  author, CI and age, and opens one in the browser.

### FEAT-60 Standup notes

Impact: medium · Effort: medium

- Why: "what did I do yesterday" is a question git, Jira and the forge can
  already answer by date, with no state kept.
- Touches: `internal/cli` (a new command), `internal/gitrepo`, `internal/jira`,
  `internal/forge`.
- Done when: `workflow standup` drafts yesterday's commits, status changes and
  pull requests in the editor, and can post the result to Slack after a
  preview.

### FEAT-61 Several repositories at once

Impact: medium · Effort: large

- Why: one task often spans two repositories, and the tool sees whichever one
  it was started in.
- Touches: `internal/cli`, `internal/wiring` (`Locate` per directory).
- Done when: `workflow status ~/src/*` prints one status line per repository
  (FEAT-42).

### FEAT-62 A worktree per task

Impact: medium · Effort: medium

- Why: FEAT-13 switches tasks by switching branches, which a dirty tree
  blocks. A worktree per issue lets two tasks be open at once.
- Touches: `internal/gitrepo/branch.go`, `internal/tui/branch.go`.
- Done when: branching can create a worktree instead, and says where it is.

### FEAT-63 The active sprint

Impact: low · Effort: medium

- Why: "what is left this sprint" is a board in a browser.
- Touches: `internal/jira` (the Agile API), `internal/tui/issues.go`.
- Done when: a view (FEAT-02) shows the active sprint grouped by status.

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
- Why: the last channel, the last scope and the usual reviewers are retyped.
- A version that fits: configuration keys for defaults, which are chosen
  rather than learned.
- Done when: the composer opens with the scope last used in this repository.

### FEAT-68 Start instantly from a cache

Impact: low · Effort: medium

- Reopens: nothing stored between sessions. It also puts issue text at rest
  on disk, which the security policy would then have to cover.
- Why: every start waits on Jira before the first pane is useful.
- Done when: the list shows at once from the last session and updates when
  the answer arrives.

### FEAT-69 Let `gh` and `glab` make the calls

Impact: medium · Effort: large

- Reopens: every service over `net/http`, and git as the only required
  program.
- Why: the official tools already handle single sign-on, enterprise hosts and
  token storage, each of which this project handles again by hand.
- Done when: with `gh` installed, no forge token is resolved by this program
  at all.

### FEAT-70 A sixth pane

Impact: medium · Effort: medium

- Reopens: five panes in the order the work goes.
- Why: review requests (FEAT-59) and check details (FEAT-28) are both things
  to glance at constantly, which is what a pane is for.
- A version that fits: an overlay for check details, a separate screen for
  review requests.
- Done when: the rail has a sixth entry and `6` jumps to it.

### FEAT-71 The operating system's keychain

Impact: medium · Effort: medium

- Reopens: pure Go, where a keychain library needs cgo on macOS; or no new
  dependency.
- Why: it is where credentials are supposed to live.
- A version that fits: FEAT-43's `token_command`, which reaches the keychain
  through `security` or `secret-tool` without linking anything.
- Done when: `config init` stores tokens in the keychain and the file holds
  none.

### FEAT-72 Notifications after the interface closes

Impact: low · Effort: large

- Reopens: a single process that ends when the interface closes.
- Why: CI results and review requests arrive when nobody is looking at the
  terminal.
- Done when: a background process raises a desktop notification for a
  finished CI run.
