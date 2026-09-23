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

- **A little is stored between sessions, on by default.** Where you are in the
  work is still read back from the branch name every time; the on-disk store
  (`internal/store`, SQLite under the OS-native data directory) only remembers
  conveniences — the scope last used, what was announced, the last issue list —
  and never a secret. `store.disabled` keeps nothing on disk, the way it once
  always was.
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

### FEAT-11 Create an issue — declined

A bug found mid-task is worth capturing, but filing one — a project, a type,
and a required-field set that differs by both — is a form the browser already
does well, and rebuilding it here earns little over the one place it belongs.
Out of scope by decision, not oversight; `workflow` picks issues up, it does not
open them.

### FEAT-78 Issues from the command line

Impact: medium · Effort: medium

- Why: The interface can list a view, read an issue in full, transition it
  with its fields, comment, assign and log work; the command line can do
  none of those on its own — `workflow branch <tab>` completes assigned keys
  (`internal/cli/scriptable.go:105`) and `workflow pr` transitions as a side
  effect (`internal/cli/pr.go:167`), and that is all. A script that wants
  "the issues in my view" or "move PROJ-1 to In Review" has nothing to call.
- Touches: `internal/cli` (new `issues`, `issue`, `transition`, `comment`,
  `assign` and `worklog` commands over the seams already on `tui.Deps` —
  `Jira.Search`, `Issue`, `Transitions`, `ApplyTransition`, `AddComment`,
  `Assign`, `AddWorklog`), the shared composition layer (REVIEW.md Phase 1)
  so the transition lookup is the one the other surfaces use,
  `docs/content/docs/reference` (regenerated).
- Done when: `workflow issues --json` prints the default view; `workflow
  issue PROJ-1 --json` prints its detail; `workflow transition PROJ-1 "In
  Review" --yes` applies a fields-less move and refuses one that needs
  fields, naming them.

### FEAT-80 Issue writes on the web

Impact: medium · Effort: large

- Why: The browser can read an issue (once REVIEW.md Phase 3 lands) but
  cannot change one: no transition with its field form, no comment, no
  assign, no log work — all of which the interface offers from the Issues
  pane (`internal/tui/picker.go:155`, `comment.go:37`, `issuewrite.go:74`).
  REVIEW.md Phase 9 adds the fields-less transition the post-open offer
  needs; this is the rest.
- Touches: `api/openapi.yaml` (operations for a transition with fields,
  comment, assign, worklog), `internal/webserver` (a handler file per write,
  each a budget row), `web/src/features/issues`, the shared composition
  (REVIEW.md Phase 1).
- Done when: a transition that needs a field shows its form and applies; a
  comment posted from the browser appears among the issue's comments; each
  write is refused under `--dry-run`, and its problem `detail` omits the
  tracker's host.

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

### FEAT-83 `workflow finish`

Impact: low · Effort: small

- Why: Finishing a merged branch is three git commands in a fixed order —
  switch to the base, `pull --ff-only`, `branch -D` — that the interface
  composes and previews (`internal/tui/finish.go:44`, the commands at
  `:71`). On the command line that is three commands to remember and get
  right, with no preview and no check that the branch really merged.
- Touches: `internal/cli` (a `finish` command over `Git.Finish`, previewed
  like `branch` and `pr`, with `--dry-run`/`--yes`), the shared composition
  layer (REVIEW.md Phase 1).
- Done when: `workflow finish --dry-run` prints the three commands and runs
  none; `--yes` runs them and says the branch is gone; a branch that has not
  merged is refused with the reason.

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

### FEAT-79 Review actions on the web

Impact: medium · Effort: large

- Why: The web's Review section shows a pull request and its CI and can
  open one, but cannot re-run failed checks, merge, finish the merged
  branch or edit the pull request's title and body — all of which the
  interface does with `R`, `M`, `F` and `e` (`internal/tui/review.go:479`,
  `:626`, `finish.go:44`, `preditor.go:37`). FEAT-31's own note already
  records the web merge as a follow-up; this formalizes the set.
- Touches: `api/openapi.yaml` (four operations), `internal/webserver` (a
  handler per action, each a budget row; merge gated exactly as `canMerge`
  gates it, `internal/tui/review.go:534`, over the shared composition —
  REVIEW.md Phase 1), `web/src/features/review`, `web/e2e` (a write driven
  against a running server, which the suite does not yet do —
  TECH_DEBT.md DEBT-65).
- Done when: a green, approved pull request can be merged from the browser
  after a preview of the permitted methods; a refused merge names the
  missing scope; every action is held back under `--dry-run`.

## Messaging

### FEAT-81 Reply in the announcement's own thread

Impact: medium · Effort: medium

- Why: A pull request is announced up to three times — ready, CI red,
  merged — as three top-level posts, and readers lose the story. The
  interface once could not thread because there was nowhere to keep a
  message timestamp; the on-disk store (`internal/store`, on by default) now
  keeps what was announced per pull request and moment
  (`internal/store/announce.go`), and a Slack `ts` is not a secret. The
  variant that *reads* a channel's history stays fenced as FEAT-64; this
  one only writes, and fits the settled decisions.
- Touches: `internal/store` (a reply-timestamp column on `announces`,
  STRICT, migrated forward), `internal/messaging` (a `thread_ts` on a
  bot-token post — a webhook cannot thread, so this is bot-only and the
  preview says so), the announcement composition shared by all three
  surfaces (REVIEW.md Phase 1), `docs/content/docs/usage.md:253`.
- Done when: the second announcement of a pull request is posted as a reply
  to the first when a bot token is configured; with a webhook it posts
  top-level and the preview says why; the store still holds no token.

### FEAT-82 Post when CI passes, from the web

Impact: low · Effort: medium

- Why: The interface's preview offers `w` — post the announcement when CI
  goes green — and keeps the queued post until it does or the run fails
  (`internal/tui/messaging.go:394`). The web announces now or not at all.
- Touches: `internal/webserver` (a queued post needs somewhere to live
  across requests — the store, or the stream's server state),
  `api/openapi.yaml`, `web/src/features/messaging`.
- Done when: "Post when CI passes" queues the announcement and the section
  shows it waiting; it posts on the first snapshot with green CI; a red run
  drops it with the reason.

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

- A version that fits the decision: FEAT-81 replies in the announcement's
  *own* thread from a timestamp the store keeps, and reads nothing.
- Reopens: reading a chat service's recent history — still a non-goal, since
  workflow posts but does not read a channel.
- Why: "CI failed" and "merged" belong under the announcement, not beside it.
  Threading needs the first message's timestamp; the store can keep that now, so
  a no-history version may fit and this may move out of this section.
- A version that fits: with a bot token, find the earlier message by searching
  the channel's recent history for the pull request's URL. It costs a
  `channels:history` scope and a request, and it cannot work with a webhook.
- Done when: a later post about the same pull request arrives as a reply.

### FEAT-65 A queued post that survives quitting

Impact: medium · Effort: large

- Reopens: a single process that ends when the interface closes. The store can
  keep the queued post now, but nothing runs to send it once the interface is
  gone.
- Why: "post when CI passes" is dropped if you quit first, which the usage
  guide lists as a limit. CI takes longer than most people keep a terminal
  open.
- A version that fits: FEAT-41 makes `workflow announce --when-green` a
  foreground command that a shell can background.
- Done when: a post queued before quitting is sent when CI passes.

### FEAT-72 Notifications after the interface closes

Impact: low · Effort: large

- Reopens: a single process that ends when the interface closes.
- Why: CI results and review requests arrive when nobody is looking at the
  terminal.
- Done when: a background process raises a desktop notification for a
  finished CI run.
