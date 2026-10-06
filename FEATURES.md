# Feature ideas

A brainstorm of what `workflow` could do next. Nothing here is promised,
scheduled or even agreed: it is a wide net, written down so that ideas are
argued over in one place instead of rediscovered.

Two readers are in mind. A contributor looking for something worth building,
and a later Claude Code session asked to "pick up FEAT-83". Each entry
therefore says why it matters, which files it would touch, and how to tell
when it is done.

Checked against commit `f05ae9f` on 2026-09-24. Its entries were read at
that commit; every pointer was checked again against the symbol it names
at `29fad1b7`, main once #145 merged, with #146's commits on top, and an
entry a later change touched was checked again in that change. Line
numbers drift, so every pointer also names the symbol it means.

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

The sections follow the order the work goes in, the same order as the first
five panes: Issues, Branch, Commits, Review, Messaging. Entries are not ranked
within a section. Ranking is the maintainer's call.

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
- **Every service is reached over `net/http`, except the forge when
  `forge.cli` routes it through `gh` or `glab`.**
- **`git` is the only program that must be installed.** `lefthook`, an editor
  and Taskwarrior make more of the interface work, and their absence only hides
  the parts that need them.
- **Nine panes down the left, one progress row across the top.** The
  seventh, Tasks, is your own Taskwarrior list, the eighth, Summary, what
  you did over a period, read back each time rather than kept, and the
  ninth, Repositories, where you work and the directories you keep as
  favorites. Focus is shown
  by the weight of a border, not by color.
- **Pure Go.** `CGO_ENABLED=0`, because the release cross-compiles to every
  platform `RELEASE_PLATFORMS` names in `Taskfile.yml`.
- **A new dependency needs approval first**, and a release younger than seven
  days is not adopted.
- **No credential is ever printed.** A change that touches a token brings the
  test that proves it does not leak.
- **One configuration file.** A file found in or above the current directory,
  up to the repository root, replaces the one in the home directory, and an
  unknown key is an error.
- **A chat service is posted to, never read.** `workflow` does not read a
  channel's history, so it asks for no scope that would let it.
- **One process, which ends when the interface closes.** Nothing runs once
  it is gone, which the usage guide lists as a limit: *Nothing outlives the
  session* (`docs/content/docs/usage.md:355`).

## Issues

Declined: FEAT-11, creating an issue. A bug found mid-task is worth
capturing, but filing one — a project, a type, and a required-field set that
differs by both — is a form the browser already does well, and rebuilding it
here earns little over the one place it belongs. Out of scope by decision, not
oversight; `workflow` picks issues up, it does not open them.

### FEAT-78 Issues from the command line

Impact: medium · Effort: medium

- Why: The interface can list a view, read an issue in full, transition it
  with its fields, comment, assign and log work. The command line reads Jira
  only in passing — `workflow branch <key>` reads the issue to name the
  branch (`runBranch`, `internal/cli/branch.go:79`), `status` prints the
  branch issue's summary (`gather`, `internal/cli/status.go:250`), `standup`
  lists recently updated assigned issues inside its draft (`gatherStandup`,
  `internal/cli/standup.go:177`) and `workflow branch <tab>` completes
  assigned keys (`completeAssignedIssues`, `internal/cli/scriptable.go:176`)
  — and writes to it only as `workflow pr`'s side effects, a link and a
  transition (`followUp`, `internal/cli/pr.go:191`). No command lists a
  view, prints an issue in full or writes one on its own. A script that
  wants "the issues in my view" or "move PROJ-1 to In Review" has nothing to
  call.
- Touches: `internal/cli` (new `issues`, `issue`, `transition`, `comment`,
  `assign` and `worklog` commands over the seams already on `tui.Deps` —
  `Jira.Search`, `Issue`, `Transitions`, `Transition`, `Comment`, `Assign`,
  `AddWorklog`), the shared composition layer (`internal/loop`)
  so the transition lookup is the one the other surfaces use,
  `docs/content/docs/reference` (regenerated).
- Done when: `workflow issues --json` prints the default view; `workflow
  issue PROJ-1 --json` prints its detail; `workflow transition PROJ-1 "In
  Review" --yes` applies a fields-less move and refuses one that needs
  fields, naming them.

### FEAT-80 Issue writes on the web

Impact: medium · Effort: large

- Why: The browser can read an issue and comment on it (`AddComment`,
  `internal/webserver/issuewrite.go:156`), and otherwise changes one only
  through the two offers after opening a pull request: it links the pull
  request on the issue (`LinkPullRequest`, `:34`) and moves the issue to the
  configured review status, fields-less and nowhere else (`TransitionIssue`,
  `:105`). Beyond those it has no transition with its field form, no
  assign, no log work — all of which the interface offers from the Issues
  pane (`openStatusPicker`, `internal/tui/picker.go:210`; `openAssign`,
  `internal/tui/issuewrite.go:79`). This is the rest.
- Touches: `api/openapi.yaml` (operations for a transition with fields,
  assign, worklog), `internal/webserver`
  (`internal/webserver/issuewrite.go` holds the post-open link and move; each
  new write grows it or earns its own file and budget row),
  `web/src/features/issues`, the shared composition (`internal/loop`).
- Done when: a transition that needs a field shows its form and applies; the
  handler tests answer each new write path 403 under `--dry-run` (every write
  is a non-GET, so `refuseWritesInDryRun`,
  `internal/webserver/guard.go:48`, covers it, as
  `TestDryRunRefusesTheIssueWrites` asserts for the link and the move, and
  `TestDryRunRefusesAComment` for the comment); and a
  write's problem `detail` omits the tracker's host.

### FEAT-85 The web paints from the cached issue list

Impact: low · Effort: small

- Why: The interface opens on the view's cached issues, shown at once while
  Jira is asked again (`seededIssues`, `internal/tui/issues.go:59`, over
  `Store.CachedIssues`, `internal/store/cache.go:30`). The web's stream reads
  Jira before it sends its first frame (`snapshotIssues`,
  `internal/webserver/stream.go:183`), so every section says *Connecting to
  workflow…* until Jira answers.
- Touches: `internal/webserver` (the stream's first frame, from a
  cached-issues seam on `Deps`), `internal/store`.
- Done when: with a cached view, the web lists its issues before Jira
  answers.

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
  composes and previews (`internal/tui/finish.go:48`, the commands at
  `:76`). On the command line that is three commands to remember and get
  right, with no preview and no check that the branch really merged.
- Touches: `internal/cli` (a `finish` command over `Git.Finish`, previewed
  like `branch` and `pr`, with `--dry-run`/`--yes`), the shared composition
  layer (`internal/loop`).
- Done when: `workflow finish --dry-run` prints the three commands and runs
  none; `--yes` runs them and says the branch is gone; a branch that has not
  merged is refused with the reason.

## Commits

### FEAT-23 Unstage everything, and discard a change

Impact: medium · Effort: small

- Why: `a` in the terminal and Stage all on the web stage every file, and
  nothing reverses either. A stray edit can only be dropped from a shell.
- Touches: `internal/gitrepo/status.go`, `internal/tui/commits.go`,
  `web/src/features/branch/WorkingTree.tsx:33` (`WorkingTree` renders
  `StageAll` alone) and `web/src/features/branch/stagingApi.ts:33` (no
  unstage-all beside `stageEverything`). Unstaging all is already
  `loop.UnstageAll` (`internal/loop/stage.go:42`), which the web server's
  `POST /api/unstage` answers `{all: true}` with.
- Constraints: discarding destroys work, so it previews the file and needs
  `enter`, unlike staging.
- Done when: one key unstages all, and another discards the selected file's
  changes after a confirmation; a test in `web/src/features/writes.test.tsx`
  keeps the requests `fakeApi` returns, clicks Unstage all with two staged
  files and finds one `POST /api/unstage` whose body is `{all: true}`.

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

- Why: a commit that pairs or lands under a DCO needs its trailers typed by
  hand. `CommitConvention.Message` takes only the subject, body and issue key
  (`internal/convention/commit.go:159`), and the composer's fields are type,
  scope, subject, body and a breaking toggle (`commitComposer`,
  `internal/tui/composer.go:51`) — no trailer among them.
- Touches: `internal/convention/commit.go` (trailers on `Message`),
  `internal/tui/composer.go` (a co-author picker over recent authors and a
  sign-off toggle), `internal/gitrepo` (the recent authors).
- Done when: recent authors can be picked as co-authors and a toggle signs off.

## Review

### FEAT-79 Review actions on the web

Impact: medium · Effort: large

- Why: The web's Review section shows a pull request and its CI and can
  open one, but cannot re-run failed checks, merge, finish the merged
  branch or edit the pull request's title and body — all of which the
  interface does with `R`, `M`, `F` and `e` (`internal/tui/checks.go:192`,
  `internal/tui/merge.go:39`, `internal/tui/finish.go:48`,
  `internal/tui/preditor.go:41`). The terminal's merge and finish shipped
  with the web's left for later; this is that later, with the re-run and
  the edit beside them.
- Touches: `api/openapi.yaml` (four operations), `internal/webserver` (a
  handler per action, each a budget row; merge gated exactly as `canMerge`
  gates it, `internal/tui/merge.go:23`, over the shared composition in
  `internal/loop`), `web/src/features/review`, `web/e2e/server` (the run
  that drives writes against a running server, where a merge needs a forge
  the fixture does not yet stand in for).
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
  STRICT, at the next `schemaVersion`), `internal/messaging` (a `thread_ts` on a
  user-token post — a webhook cannot thread, so this is user-token-only and the
  preview says so; `Post` in `internal/messaging/post.go` decodes no `ts`
  from chat.postMessage's `verdict` and returns only an error, so it must
  return the timestamp), the post seam that carries it to the record on
  every surface (`loop.Deliver`'s `post` and `Announced`,
  `internal/loop/announce.go:206`, which the interface's
  `seams.Messaging.Post`, `internal/seams/seams.go:126`, and
  `announceSeams.Post` in `internal/cli/announce.go` both post through; the
  web server's `Deps.Post`, which records nothing yet — FEAT-84), and the
  *Each announcement is its own message* limit in
  `docs/content/docs/usage.md:365`.
- Done when: the second announcement of a pull request is posted as a reply
  to the first when a user token is configured; with a webhook it posts
  top-level and the preview says why; the store still holds no token.

### FEAT-82 Announce when CI passes, from the web

Impact: low · Effort: medium

- Why: The interface's preview offers `w` — announce when CI goes
  green — and keeps the queued announcement until it does or the run fails
  (`messagingPreview.postWhenGreen`, `internal/tui/messagingpreview.go:169`).
  The web announces now or not at all.
- Touches: `internal/webserver` (a queued post needs somewhere to live
  across requests — the store, or the stream's server state),
  `api/openapi.yaml`, `web/src/features/messaging`.
- Done when: "Announce when CI passes" queues the announcement and the
  section shows it waiting; it is sent on the first snapshot with green CI; a
  red run drops it with the reason.

### FEAT-84 The web remembers what was announced

Impact: low · Effort: small

- Why: The interface and `workflow announce` record each announcement in the
  store and do not offer again a moment an earlier session announced
  (`loop.Deliver`, `internal/loop/announce.go:206`; `offerAgain`,
  `internal/cli/announce.go:162`). The web's `Announce`
  (`internal/webserver/announce.go:43`) posts through `Deps.Post` and
  neither records nor reads, so an announcement made in the browser is
  invisible to the terminal, which offers it again, and the web's work story
  never marks its Announce step done (`onHeadStages`,
  `web/src/features/issues/WorkStory.tsx:92`).
- Touches: `internal/webserver` (post through `loop.Deliver`, with the
  store's memory on `Deps` as `RecordScope` is), `api/openapi.yaml` (the
  snapshot carries what was announced), `web/src/features/issues`,
  `web/src/features/messaging`.
- Done when: an announcement sent from the web shows as announced in the
  terminal, and one sent from any surface marks the web's Announce step
  done.

## Across the loop

### FEAT-52 Packages

Impact: medium · Effort: medium

- Why: installing means `go install` or downloading a binary and checking a
  checksum by hand.
- Touches: `.github/workflows/release.yml`, `docs/content/docs/install.md`.
- Constraints: publishing is the maintainer's to do. This is preparation only.
- Done when: a Homebrew formula and a Scoop manifest are generated by the
  release and documented.

### FEAT-87 What workflow did, and taking it back

Impact: medium · Effort: large

- Why: Every write workflow makes is told once, in a notice that the next
  action clears (`Model.noticed`, `internal/tui/overlay.go:169`) or an
  outcome line beside a button, and then is gone. After a burst — a
  branch, three stages, a commit, a push, a pull request, a link, a move,
  an announcement — nothing shows what was done or where, and nothing
  takes any of it back. UX-68 asks only that each irreversible act say so;
  this is the feature it stops short of. The Summary (FEAT-86) is not the
  same list: it reads what the sources keep, so it shows a commit but not
  a stage, a favorite, a push or a post.
- Touches: `internal/loop` (a `Did` record — what was done, to what, the
  link where there is one, when — and an `Undo` func on it where the
  system allows one); `internal/wiring` (each write seam wrapped once, so
  a write made through any seam is recorded the same way whichever
  surface made it); `internal/tui` (a "What workflow did" overlay, newest
  first, with `u` on a row that can be undone, behind a `lastLook`, since
  an undo is itself a write); `internal/webserver` and `api/openapi.yaml`
  (`GET /api/did` and `POST /api/did/{id}/undo`, the undo refused under
  `--dry-run` like every write) and `web/src/shell` (a drawer, reached
  from the header); the clients that need a delete:
  `internal/jira` (a comment's id, which `Comment` does not carry today,
  `internal/jira/detail.go:24`, and `DELETE` on it; a worklog's id, which
  `Worklog` already carries, `internal/jira/worklog.go:13`),
  `internal/forge` (a comment's id, `IssueComment` at
  `internal/forge/issues.go:47` carrying none, and its delete).
- What can be undone, and how: stage and unstage, each other
  (`Repository.Unstage`, `internal/gitrepo/status.go:205`); favorite and
  unfavorite (`DELETE /api/repositories/favorites`); a task's start,
  stop, done, add, annotate or modify, through Taskwarrior's own undo
  (`Client.Undo`, `internal/taskwarrior/write.go:82`) only while it is
  still Taskwarrior's last change, which the row checks first; an issue's
  move, by moving it back when Jira offers a transition to the old status
  from the new one (`Transitions`, `internal/jira/transitions.go:61`) —
  and saying so when it does not, or wants fields; an assignment, by
  assigning the issue to whoever had it, which the record keeps; a
  comment or a worklog, by deleting it; a branch or a worktree just made
  and not yet pushed, by removing it once nothing is uncommitted; a branch
  link, by unlinking it (`Git.UnlinkIssue`, `internal/seams/seams.go:123`).
- What cannot, said on the row: a push (taking it back would be a force
  push to a shared remote); a merge (its reversal is a revert, a new pull
  request); finishing a branch (`branch -D`; the commits stay reachable
  from the base); a pull request opened or edited, which the forge keeps;
  a commit or an amend once pushed; and a post: `Client.Post` returns
  only an error (`internal/messaging/post.go:87`), so there is nothing to
  delete a Slack message by until FEAT-81 decodes its timestamp, and a
  Slack, Teams, Discord or plain webhook post cannot be taken back through
  this package at all.
- Constraints: The log is kept in memory, for one process, and nowhere
  else. `workflow.db` holds conveniences a session can see again and
  `kept.db` what the user decided (CLAUDE.md, *Two files*); a list of one
  session's writes is neither, and keeping it would be a cache of what
  Jira's history, git's reflog and Taskwarrior's undo file already hold —
  TRADE-32's reasoning. An undo also only makes sense close to the act: a
  move undone tomorrow, after someone else has moved the issue, is a new
  move, not an undo. So the interface's log ends with the interface, and
  the web server's with the server, as "one process" settles. The record
  holds no text a user typed — a comment's body, a commit message —
  only what it was done to and the link, and no credential, since
  everything recorded is a credential-free identifier; the test that a
  token cannot reach a record ships with it. `internal/tui` (57 of 57)
  and `web/src/shell` (12 of 12) are at their file budgets
  (`scripts/package-size-budgets.txt`), so the overlay and the drawer
  each come with a budget bump and its reason, or a home elsewhere.
- Done when: after a screen test stages a file, comments and moves an
  issue, the overlay lists the three newest first with their links; `u`
  on the comment, after its last look, calls the fake Jira's delete with
  the comment's id; the push row says it cannot be undone and offers no
  key; a web test reads `GET /api/did`, undoes the stage, and finds the
  file unstaged; restarting the server empties the log.

## New integrations

### FEAT-53 Jira Cloud

Impact: high · Effort: large

- Why: the client speaks Data Center's REST v2 only. Cloud moved search to
  `/search/jql`, writes descriptions and comments as Atlassian Document
  Format, and signs in with an email and an API token.
- Touches: `internal/jira` (a second client or a dialect, as `internal/forge`
  has), `internal/config/config.go`, `internal/wiring/wiring.go`.
- Constraints: `seams.Jira` is already plain functions, so the interface
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

- Why: a view on `sprint in openSprints()` already lists what is left this
  sprint — the configuration guide's *Sprint board* example
  (`docs/content/docs/configuration.md:189`) — but flat, one row per issue
  with a status glyph, in update order (`issueList.render`,
  `internal/tui/issues.go:256`). What is missing is grouping by status: how
  much is to do, in progress or in review is read by scanning glyphs.
- Touches: `internal/tui/issues.go` (status headings in the list), and
  `internal/jira`'s Agile API only for what JQL cannot give — the sprint's
  name and dates in the heading.
- Done when: the Sprint view (`jira.views`, `internal/config/config.go:73`)
  renders its issues under status headings.

### FEAT-86 One summary, read and posted everywhere

Impact: medium · Effort: medium

- Why: Two features answer "what did I do?", and they do not agree.
  `workflow standup` gathers the commits since `--days` ago through `git
  log`, the Jira issues assigned to you and updated in the window, and the
  open pull requests on your fifteen most recent local branches
  (`gatherStandup`, `internal/cli/standup.go:177`; `standupJQL`, `:220`;
  `gatherPulls`, `:190`), opens the Markdown in `$EDITOR` and offers to
  post it (`offerToPost`, `:150`). The Summary pane and section read
  something else for a period of days: the commits you wrote, the tasks
  you touched, what you did to Jira issues and the pull requests you
  opened, had merged and reviewed, each through the shared reads
  (`loop.CommitsRead`, `TasksRead`, `JiraRead`, `ForgeRead`,
  `internal/loop/summary.go:36`, `:138`, `:180`, `:220`), grouped by year,
  month, day and hour (`activity.Group`, `internal/activity/group.go:44`)
  and copied as Markdown (`Summary.Text`, `internal/activity/text.go:32`;
  `Y` in the terminal, Copy as Markdown on the web). So the standup a
  team receives misses the reviews and the tasks the Summary shows,
  counts an issue touched by anyone as yours, and cannot be read for last
  Thursday; the Summary cannot be posted at all, and a script cannot read
  it. The two interfaces also assemble the four reads twice, field by
  field (`Model.summaryReads`, `internal/tui/summary.go:160`;
  `server.activityReads`, `internal/webserver/activity.go:84`) — a third
  caller makes it the rule of three.
- Touches: `internal/loop` (one `SummaryReads` over a struct of the four
  activity seams, which the terminal, the web server and the command line
  all call — the two copies above removed); `internal/cli` (a `summary`
  command: `--from` and `--to` read as `YYYY-MM-DD` by
  `activity.ParseDate`, `internal/activity/period.go:55`, and defaulting
  to the previous working day as `GET /api/activity` does,
  `api/openapi.yaml:1639`; `--json` printing the API's `Activity` shape;
  `--post`, which previews, asks and posts as `announce` does, `--yes`
  and `--dry-run` included; `standup` removed, its `--days` gone with
  it); `internal/tui/summary.go` (a Post… key opening a preview of the
  text, its channel cycled and `e` editing it, as `messagingPreview`
  does for an announcement, `internal/tui/messagingpreview.go`);
  `internal/webserver` and `api/openapi.yaml` (`POST /api/activity/post`
  taking the period and the text as previewed, refused under `--dry-run`
  by `refuseWritesInDryRun`, `internal/webserver/guard.go:48`, its
  refusal worded through `messagingFaults`); `web/src/features/summary`
  (a Post… beside Copy as Markdown, through the messaging section's
  preview);
  `internal/messaging` (the Markdown rendered for the service:
  `markupFor`, `internal/messaging/post.go:298`, words links and escapes
  per service, but nothing yet turns `Summary.Text`'s `#` headings into
  what Slack's mrkdwn shows); `docs/content/docs/scripting.md` and the
  generated reference.
- Constraints: Nothing is stored — TRADE-32 holds: the summary is read
  back from the sources each time, and a post is not recorded as an
  announcement is. The read is the same on all three surfaces, so a
  period gives the same items and the same "could not be read" notes in
  the pane, the section and `--json`. Every value from a service is
  neutralized before it reaches a terminal or the editor, as
  `draftStandup` does now. Dropping `standup` is a breaking change: the
  commit is `feat!` and its body names `workflow summary --post` as the
  replacement, so release-please bumps the minor. UX-92's `standup
  --json` and UX-128's standup bullets move to `summary`.
- Done when: `workflow summary --from 2026-10-01 --to 2026-10-02 --json`
  prints the same items `GET /api/activity` answers for that period in a
  test over the same fakes; `workflow summary --post --yes` posts the
  rendered text and a `--dry-run` posts nothing; the Summary pane's Post…
  and the web's open a preview, post nothing until confirmed, and a
  webhook-only setup shows the webhook's channel; `workflow standup` is
  an unknown command; `grep -rn 'CommitsRead' internal/tui
  internal/webserver` finds nothing.

## Build and platform

Mac and Linux are the primary targets and Windows is secondary, run as the
cross-compiled binary rather than through an installer, so what is open here
cannot be exercised from a developer's own machine.

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

- A version that fits the decision: FEAT-81 would reply in the
  announcement's *own* thread from a timestamp the store would keep, and
  read nothing.
- Reopens: reading a chat service's recent history — still a non-goal, since
  workflow posts but does not read a channel.
- Why: "CI failed" and "merged" belong under the announcement, not beside it.
  Threading needs the first message's timestamp; FEAT-81 would keep it in
  the store, and this entry is the other way to find it.
- The version fenced here: with a user token, find the earlier message by
  searching the channel's recent history for the pull request's URL. It
  costs a `channels:history` scope and a request, and it cannot work with a
  webhook.
- Touches: `internal/messaging` (a history read on the user-token client),
  `internal/loop/announce.go`.
- Done when: a later post about the same pull request arrives as a reply.

### FEAT-65 A queued post that survives quitting

Impact: medium · Effort: large

- Reopens: one process, which ends when the interface closes. The store can
  keep the queued post now, but nothing runs to send it once the interface is
  gone.
- Why: "post when CI passes" is dropped if you quit first, which the usage
  guide lists as a limit. CI takes longer than most people keep a terminal
  open.
- A version that fits: a `--when-green` flag on `workflow announce`
  (`internal/cli/announce.go`) that waits in the foreground, where a shell
  can background it.
- Touches: `internal/cli/announce.go` (a command that stays resident),
  `internal/store` (the queued post).
- Done when: a post queued before quitting is sent when CI passes.

### FEAT-72 Notifications after the interface closes

Impact: low · Effort: large

- Reopens: one process, which ends when the interface closes.
- Why: CI results and review requests arrive when nobody is looking at the
  terminal.
- Touches: the resident process FEAT-65 needs, and a desktop-notification
  seam wired in `internal/wiring`.
- Done when: a background process raises a desktop notification for a
  finished CI run.
