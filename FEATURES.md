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

### FEAT-11 Create an issue — declined

A bug found mid-task is worth capturing, but filing one — a project, a type,
and a required-field set that differs by both — is a form the browser already
does well, and rebuilding it here earns little over the one place it belongs.
Out of scope by decision, not oversight; `workflow` picks issues up, it does not
open them.

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

## Commits

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

### FEAT-31 Merge

Impact: medium · Effort: medium

- Why: the last outward step of the loop is a button in a browser.
- Touches: `internal/forge` (merge, and which methods the repository allows),
  `internal/tui/review.go`.
- Constraints: previewed like every other write; offered only when the forge
  says it can merge; uses the method the repository permits.
- Done when: a green, approved pull request can be merged after a preview,
  and FEAT-17 follows.
- Done: `forge.Client.Merge` merges by a `MergeMethod` the repository permits,
  and `MergeMethods` reads which those are — GitHub from its allow flags,
  GitLab from the project's merge method and squash option. `M` on a green,
  approved, clean pull request opens a preview of the permitted methods; enter
  merges by the chosen one, esc cancels, a refusal names the missing write
  scope, and a dry run reports it. Held back at the seam like every write. The
  web review panel shows a pull request but has no write actions yet (like
  re-run checks); a web merge is a follow-up. FEAT-17 (finishing the merged
  branch) follows.

## Across the loop

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

## Beyond the loop

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

### FEAT-72 Notifications after the interface closes

Impact: low · Effort: large

- Reopens: a single process that ends when the interface closes.
- Why: CI results and review requests arrive when nobody is looking at the
  terminal.
- Done when: a background process raises a desktop notification for a
  finished CI run.
