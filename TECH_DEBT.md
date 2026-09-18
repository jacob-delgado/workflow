# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-22". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts.

Checked against commit `817d323` on 2026-09-17. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **An audit.** Every non-test Go file, every script, every workflow and
   every page of docs was read in full, against the standard the project sets
   for itself in [CLAUDE.md](CLAUDE.md).
2. **An attempt to refute it.** Each finding was handed to a second pass that
   had not written it, with instructions to prove it wrong: read the callers,
   check the primary source, run an experiment. Of 126 claims, 98 held as
   written, 28 were corrected in some detail and none was refuted; the
   corrected versions are what is written here. Experiments ran against
   scratch copies and scratch repositories; nothing in this repository was
   changed.
3. **A third read.** The cited lines of every high and medium entry were read
   again while writing this file.
4. **Measurement.** `task test:cover` and `task cover:branch` were run at this
   commit for the test-suite section.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). Anything that rests on outside behavior nobody checked says so.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it appears under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost), which lists
the choices that were made on purpose and written down, and what they cost.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule for what it found: those findings went to the maintainer directly. The
entries below contain nothing that helps anyone misuse a credential, a
terminal or a release.

## The terminal interface

### DEBT-07 Six overlays, one state machine, written six times

Severity: medium · Confidence: read

- Evidence: a `sending bool` beside an error named three ways (`applyErr`,
  `err`, `problem`) in `statusPicker`, `commentPreview`, `branchCreator`,
  `prComposer`, `slackPreview` and `hookgenOffer`. Each repeats a view tail, a
  footer guard, a key guard and a failure applier of the same shape
  (`internal/tui/picker.go:60`, `internal/tui/comment.go:134`,
  `internal/tui/branch.go:304`, `internal/tui/prcomposer.go:315`,
  `internal/tui/slack.go:305`, `internal/tui/hookgen.go:144`). The editor
  round trip is line for line the same at `internal/tui/composer.go:253`,
  `internal/tui/prcomposer.go:239` and `internal/tui/slack.go:299`. The fourth
  copy differs: `commentEdited.apply` closes the overlay on an editor error
  (`internal/tui/comment.go:47`), so a failed re-edit discards a written
  comment while the other three keep their text. Reproduced.
- Cost: six copies is twice the rule of three, each copy draws its own outcome
  line, and the one copy that diverged has a bug the others do not.
- Remedy: a small value type, `sendState{sending bool; err error}`, held as a
  named field, with the view lines, the lock and the failure transition on
  it; one `textEdited` message for the editor round trip.
- Done when: the failure appliers are one function, and the comment preview
  survives an editor failure.

### DEBT-10 Click math mirrors the view, by hand

Severity: low · Confidence: read

- Evidence: the wrapped-jobs click bug is fixed — `commandRun.click` now skips
  every row a wrapped jobs line took, not one. What remains is tidiness: each
  click handler still recomputes its view's layout from constants
  (`internal/tui/run.go`, `internal/tui/picker.go`, `internal/tui/commits.go`),
  where the issues pane already asks a shared `rowAt`.
- Cost: a change to one of those views can silently break its click handler
  again, the way the jobs line did.
- Remedy: one `selection` value with `moved`, `window` and `rowAt`, returned
  by the view so the click handler asks it.
- Done when: the picker, run and commits click handlers contain no layout
  constants.

### DEBT-12 One model that every message can change

Severity: low · Confidence: read

- Evidence: `Model` has 24 fields (`internal/tui/tui.go:34`) and 100 methods
  in 16 files. Every overlay and applier takes and returns the whole value.
  Feature state sits at the root (`runs`, `draft`). Help is an overlay in
  every way but its type: `helpOpen` is special-cased at
  `internal/tui/tui.go:159` and `:168`, `internal/tui/render.go:88` and
  `:220`. `Update` itself is small and routes through `applier`.
- Cost: DEBT-02 is state that outlived what it described, and the Slack
  pane's state once was too, which is what happens when any message can reach
  any field. This is a
  judgment call, and a rewrite is not the answer.
- Remedy: make help an `overlay`; move `runs` and `draft` beside the code that
  owns them; give each pane's state the methods that change it.
- Done when: `helpOpen` is gone.

### DEBT-13 Small things in the interface

Severity: low · Confidence: read

- The branch's issue is derived at six sites
  (`convention.IssueKey(m.branch.branch.Name)` in `branch.go`, `composer.go`,
  `detail.go`, `prcomposer.go`, `spine.go`, `slack.go`), three of which drop
  the found flag and use `""` as "none". `"origin/"` is trimmed by hand at
  `internal/tui/branch.go:66` and `:90` and `internal/tui/prcomposer.go:78`.
  `m.branch.branch` appears 23 times. Remedy: `Model.branchIssue()` returning
  a typed value, and `BaseName()` on `gitrepo.Branch`.
- The pane set is written down in six places: `internal/tui/panes.go`,
  `key.WithKeys("1","2","3","4","5")` (`internal/tui/keys.go:58`),
  `pane(msg.String()[0] - '1')` (`internal/tui/tui.go:196`), the help groups,
  a second list of names in `internal/tui/spine.go:54`, and the literal
  "(4 Review)" in `internal/tui/slack.go:113`.
- The run overlay calls `hooks.Jobs(r.lines)` on every frame and on every
  click (`internal/tui/run.go:176` and `:247`), and appends output with no cap
  (`internal/tui/run.go:105`). Each output line is a message and each message
  redraws, so a chatty hook is quadratic.
- Vestigial: `Style.ASCII()` (`internal/tui/frame/frame.go:36`) is called
  only by a test; `hookgenState.offered` can never be true when it is read;
  `var _ help.KeyMap = keyMap{}` (`internal/tui/keys.go:11`) asserts an
  interface nothing uses; the comment "FullHelp is every key"
  (`internal/tui/keys.go:101`) is false by eleven bindings; the comment that
  the model "never holds … a credential" (`internal/tui/deps.go:19`) sits
  beside `Model.cfg`, which is the unredacted configuration (it is not shown;
  a test proves that).
- "1 files staged" (`internal/tui/composer.go:155`) is pinned by
  `internal/tui/composer_test.go:50`. `Breaking: false` is hardcoded at
  `internal/tui/composer.go:92`.
- Loads carry no sequence number, so the last answer to arrive wins.
  `detailLoaded` checks the key only (`internal/tui/detail.go:49`). Every
  request is bounded at ten seconds, which keeps the window narrow.

## The clients: forge, Jira and Slack

### DEBT-17 Three HTTP clients copied by hand, already drifting

Severity: medium · Confidence: read

- Evidence: `HTTPClient`, `Doer`, `ErrRedirected`, `ErrUnreachable`,
  `ErrUnexpectedStatus` and `bodyLimit` are each defined three times
  (`internal/jira/jira.go`, `internal/forge/client.go`,
  `internal/slack/slack.go`). Drift so far: Jira and Slack's post path strip
  the transport error's URL (`internal/jira/search.go:132`,
  `internal/slack/post.go:152`) while the forge and Slack's identity check do
  not (`internal/forge/client.go:179`, `internal/slack/slack.go:129`), so the
  same failure reads two ways; a refused redirect says "could not reach" in
  all four although the server answered, and its target survives only where
  the URL was not stripped; hitting the body limit reads as "unexpected end
  of JSON input". Inside Jira, request-exchange-decode-wrap is repeated five
  times (`internal/jira/jira.go:112`, `internal/jira/search.go:99`,
  `internal/jira/detail.go:77` and `:117`, `internal/jira/transitions.go:54`)
  where the forge has `call[T]` (`internal/forge/client.go:113`).
- Cost: the redirect policy is the security-critical part of a client, and it
  exists three times with nothing holding the copies equal. `config.go`
  records the lesson already: "when each did this by hand, one of them forgot
  the mask."
- Remedy: one small internal package, standard library only, holding `Doer`,
  the client constructor, the three transport sentinels, a bounded read that
  says "too large", and one decision about what a transport error shows.
  Status mapping stays per service, where it genuinely differs.
- Done when: `CheckRedirect` is written once.

### DEBT-19 Smaller items in the clients

Severity: low · Confidence: read

- No client reads `Retry-After` or a rate-limit header. Jira maps 429 to an
  unexpected status, the forge folds it into 403's message
  (`internal/forge/client.go:228`), Slack calls it a rejected credential.
  Polling alone will not reach GitHub's limit (about 360 requests an hour of
  5,000), so this is about saying the right thing when it happens.
- Adding a forge means editing eight places. Three are switches the
  `exhaustive` linter checks (`Kind.String`, `APIBase`, `environmentNames`);
  five are not, and fail quietly: `kindOf` and `ParseKind`
  (`internal/forge/remote.go`), `dialectFor`'s map
  (`internal/forge/pulls.go:46`, a runtime error), `FindTemplates`' map
  (`internal/forge/templates.go:36`, silently nothing) and `cliCommand`
  (`internal/forge/token.go:250`, silently false). Folding these into
  `dialect` would make FEAT-54 one file.
- `forge.Token`'s comment says a token cannot be printed by accident "nested
  inside any struct". It has a `String` method and nothing else, so `%#v`,
  `%d`, `json.Marshal` and `%+v` of a struct holding one in an unexported
  field (`forge.Client` is one) print the value. No production code does
  this; the guard is narrower than it claims. The four secrets in
  `config.Config` are bare strings. None of the five `Stringer` types carries
  the static assertion CLAUDE.md asks for.

## Configuration, wiring and the command line

### DEBT-21 The forge connection is built twice, and remembered when it fails

Severity: low · Confidence: read

Fixed: `onceConnected` (`internal/wiring/wiring.go`) caches a connection only
once it succeeds and retries after a failure, so `gh auth login` in another
terminal is found on the next attempt rather than only on a restart — the
"Done when" is met. The duplicated `forge.Resolver` literal is now the shared
`wiring.ForgeResolver`, which both `connectForge` and `checkForge` call, so the
two cannot drift in where they look for a credential.

- What remains: the preamble around the resolver — parse the remote, apply the
  configured kind, find the API base — is still shaped the same in both, but the
  two report failure differently (doctor prints a line per step; wiring returns
  wrapped errors), so a single `connect` would have to thread that divergence.
  Left until a third caller earns it. The success arm of the interface's own
  `connect()` sites is DEBT-36.

### DEBT-23 The file format has no version and rejects what it does not know

Severity: medium · Confidence: reproduced

- Evidence: `Config` has no version field; `LoadFile` calls
  `DisallowUnknownFields` (`internal/config/config.go:219`) and returns
  `Default()` on any error; the README says the format may change before 1.0.
- Cost: the first renamed key fails every existing file with
  `json: unknown field "url"`, and the run carries on with every credential
  gone. A newer file on an older binary fails the same way. The strictness is
  deliberate and good; the missing half is what to do when the shape changes.
- Remedy: decide the story before the first rename. At the least, catch the
  unknown-field error and name the key that replaced it. See FEAT-51.
- Done when: an old key produces a message that names the new one.

### DEBT-24 Nothing can be canceled, and no subprocess has a deadline

Severity: medium · Confidence: reproduced

- Evidence: `root.Execute()` (`internal/cli/cli.go:89`) gives every command
  `context.Background()`; production code contains no `ExecuteContext`,
  `NotifyContext`, `WithTimeout`, `WithCancel` or `WithDeadline`. HTTP has
  `RequestTimeout`; `git`, `gh` and `lefthook` have nothing. `build`
  (`internal/proc/start.go:137`) sets no process group, `Cancel` or
  `WaitDelay`. `Start` documents "Canceling ctx kills the program"
  (`internal/proc/start.go:49`), which cannot happen. A push gets
  `GIT_TERMINAL_PROMPT=0` (`internal/gitrepo/branch.go:238`); a commit gets no
  environment at all (`:222`).
- Cost: a hung `git status` or `gh auth token` leaves its pane on "loading…"
  forever. A streamed run ignores every key but `ctrl+c`, which quits the
  program; a parent built the same way left its child running with a parent
  process of 1, so a push can complete after the user "quit", and the commit
  message's temporary file is never removed. `GIT_TERMINAL_PROMPT` governs
  git's own prompts only: ssh and gpg open the terminal directly while the
  interface owns it (shown for ssh's documented behavior and a child's access
  to `/dev/tty`; gpg was not installed to try).
- Remedy: `signal.NotifyContext` with `ExecuteContext`; a default deadline in
  `proc.Run`; a build-tagged Unix file that starts streamed children in their
  own session, so prompts fail fast and the group can be stopped; a "stop" key
  in the run overlay.
- Done when: a test cancels a streamed run and its grandchild exits.

## Git, hooks, conventions and processes

### DEBT-30 `origin` is spelled out in eight places, and nothing fetches

Severity: medium · Confidence: read

- Evidence: `internal/gitrepo/gitrepo.go:71`; `internal/gitrepo/branch.go:58`,
  `:133`, `:138` and `:237`; `internal/tui/branch.go:66` and `:90`;
  `internal/tui/prcomposer.go:78`. No code path runs `git fetch`. The base
  falls back through `origin/HEAD`, `origin/main`, `origin/master`, local
  `main`, local `master`, then nothing (`base`,
  `internal/gitrepo/branch.go:132`).
- Cost: a remote under another name means "not pushed yet" forever. With a
  fork, the base is the fork's default branch. When no base is found,
  `Commits` stays empty, and both `canPush` and `canOpenPullRequest` are
  false: the loop stops, with no setting to restart it. "Starts from origin's
  default branch" means "as of the last fetch", which the screen does not say.
  See FEAT-12 and FEAT-15.
- Remedy: carry the remote as one value on a gitrepo type; ask git for
  `remote.pushDefault` before assuming; say how old the base is.
- Done when: the word `origin` appears once in production code.

### DEBT-32 Windows is a release target the code has not met

Severity: low · Confidence: read

- Evidence: `windows/amd64` is built (`Taskfile.yml:30`); there are no build
  tags and no `runtime.GOOS` checks. `ExistingHooks` requires an executable
  bit (`internal/hooks/generate.go:82`), and Go reports `0666` or `0444` for
  every file on Windows, so no hook is ever found. The editor falls back to
  `vi` (`internal/editor/editor.go:31`). The location pattern
  (`internal/hooks/output.go:131`) cannot match `C:\path\file.go:12`. CI never
  tests the Windows build, though the cross-compile leg now proves it links.
  None of this was run on Windows.
- Remedy: a macOS and a Windows leg in CI first, to learn what else is true;
  then small build-tagged helpers.
- Done when: the test suite runs on Windows in CI.

### DEBT-33 Smaller items in the local packages

Severity: low · Confidence: read

The reproduced bugs here are fixed: the issue-key match no longer reads `UTF-8`,
`SHA-256`, `CVE-2024` or `ISO-8601` as a key (`standardAbbreviations` in
`internal/convention/convention.go` skips them); `Message` and `PullRequestBody`
find an existing reference by whole word, not substring, and keep an appended
`Refs:` in the same trailer block as `Co-authored-by:`; `ReadBranch` reverses the
whole range before capping, so a branch past 200 commits still titles its pull
request from its first commit; the dead `ReadConfig`/`wireHook`/`Runner` and
`File.Executable` are gone; and a `@` branch name is refused as `@`, not as empty.

- What remains, its own follow-up: `(ctx, run Runner, dir string)` repeats on
  eight gitrepo functions, and `Status` and `Stage` silently require `dir` to be
  the root. A `Repository` value returned by `Describe` would carry all three.
- The agreed upgrade for the issue-key match — accept only a key whose project is
  the configured Jira project, rather than a denylist of common tokens — needs a
  `config.Jira.Project` field threaded through the `IssueKey` call sites, a
  cross-cutting change of its own.
- Left as they are, YAGNI over microseconds: the `regexp.MustCompile` calls sit
  inside functions, and `Output.Wait` (`internal/proc/start.go`) blocks on a
  second call that no caller makes.

### DEBT-34 Domain values travel as bare strings

Severity: low · Confidence: read

- Evidence: five `JiraDeps` functions take `issueKey string`
  (`internal/tui/deps.go:44`), and `Comment func(issueKey, text string)`
  compiles with its arguments swapped. `BranchName(issueType, key, summary
  string)` (`internal/convention/convention.go:70`) has the same shape, and
  matches the issue type against the English word "bug", which a localized or
  renamed type defeats. Jira's status categories (`"new"`,
  `"indeterminate"`, `"done"`) exist only as map keys in
  `internal/tui/glyphs.go:56`. `GitHook.Name` is a string joined into a path
  (`internal/hooks/generate.go:130`) on the strength of a `//nolint` comment.
  `config.Forge.Kind` is a string parsed again at each use.
- Cost: CLAUDE.md names the issue key as its own example of primitive
  obsession. The compiler cannot help with any of these today.
- Remedy: `jira.Key`, `jira.StatusCategory`, a `HookName` built only from the
  known list. The interface inherits the types.
- Done when: `Comment(text, key)` does not compile.

## The test suite

Snapshot at this commit: statement coverage 98.1% against a floor of 95, and
condition coverage 96.4% (1642 of 1704 arms) against a floor of 93. Both
gates print an available ratchet (to 96 and to 94). The weakest packages by
condition coverage are `internal/cli` at 75.7% and `internal/wiring` at 81.6%.
These numbers are a dated reading, not a floor; the floors live in
`Taskfile.yml`.

### DEBT-35 No test runs the root command — DONE

`tui.Run` is now injected into the command tree through `newRootCmd(prompt,
run)` (`internal/cli/cli.go`), so `cli_dryrun_internal_test.go` runs the bare
`workflow` RunE with a fake runner and asserts the model it built shows "DRY
RUN" with the flag and not without. Removing `if dryRun { model.WithDryRun() }`
now fails the suite; gobco sees both arms of `dryRun`.

### DEBT-36 The forge's success path is never exercised outside its package

Severity: low · Confidence: measured

Done for the interface's forge seams: `forgeDepsFrom` (`internal/wiring/forge.go`)
takes the `connect` as a parameter, and `forgedeps_internal_test.go` drives
FindPullRequest, CreatePullRequest, CheckStatus, ReviewRequests and Author with a
client pointed at a fake GitHub, so the success arm of each connect guard is now
exercised — only failures were before. `config init --global` is tested too
(`config_init_guided_test.go`).

- What remains: `doctor`'s `askForge` and `checkSlack` still build real clients
  against real addresses (`internal/cli/doctor.go`), so their success arm is only
  reached against a live forge/Slack. Injecting the API base and `Doer` there is
  the remaining work, tied to DEBT-17's shared transport.

### DEBT-37 The shared fake world is 27 lines from the file-length gate

Severity: low · Confidence: measured

- Evidence: `internal/tui/world_test.go` is 473 lines of a 500-line limit.
  Of 187 tests in `internal/tui`, 135 build a world and 173 use a helper
  defined in that file. `internal/tui/edges_test.go` (447 lines) holds 21
  tests across composer, review, Slack, commits, mouse, picker and hook
  generation, each of which has a test file of its own.
- Cost: the next `tui.Deps` seam pushes the largest file in the repository
  past the gate in the middle of a feature. `edges_test.go` is the "divergent
  change" smell CLAUDE.md names.
- Remedy: split the world by seam; move each edge test to its concern's file.
- Done when: no test file is within 50 lines of the limit.

### DEBT-38 Smaller items in the tests

Severity: low · Confidence: read

Fixed: there is now an advisory `task fuzz` that discovers every Fuzz function
and gives each a short `-fuzztime` run, so a target that would find a new input
gets the chance the seed-only gate never gave it. The "1 files staged"
pluralization defect is gone (`composer.go` renders `plural(...)`), so no test
pins it any more.

- What remains, low value: `patience` and `within`/`drain`
  (`internal/tui/world_test.go`) still wait on wall-clock time and drop a command
  that overruns, which a slow runner under `-race` can turn into a confusing
  failure. Reworking the Bubble Tea drain to settle on quiescence rather than a
  timer is a delicate test-harness change on its own.
- Three wiring tests skip when lefthook is absent
  (`internal/wiring/hooks_test.go`), which it is in the build container — tied to
  DEBT-41.

## Build, CI, scripts and release

### DEBT-41 The build container is never built, and is not the same gate

Severity: medium · Confidence: read

- Evidence: no workflow builds `build/Dockerfile`. `scripts/tool-versions.sh`
  passes 14 of `mise.toml`'s 19 pins; lefthook, shellcheck, hugo-extended,
  cloc and deadcode are left out. shellcheck, jq, nodejs and npm come from apt
  with no version (`build/Dockerfile:45`). typos, hadolint, taplo and zizmor
  are downloaded with `curl` and no checksum, while
  `.devcontainer/postCreate.sh:17` argues for verifying one.
- Cost: `task container:check` is documented as "the same gate", and it skips
  three tests, lints with whichever shellcheck Debian ships, and would not
  notice a dead download URL until someone built it by hand.
- Remedy: build the image and run `task container:check` weekly in CI; pass
  the missing pins; have `tool-versions.sh` fail when a tool is neither passed
  nor skipped on purpose; verify the downloads.
- Done when: a workflow builds the image.

### DEBT-42 mise itself floats in CI — DONE

Every `jdx/mise-action` use now pins `version: 2026.9.3` (the seven `ci.yml`
uses, plus `release.yml` and `pages.yml`), so the mise binary — which carries
the tool registry — no longer floats to the latest release, which for the
uncached release job was younger than the seven-day gate allows anything else
to be. `min_version` in `mise.toml` is raised to `2026.9.3` to match, with a
note that mise cannot pin its own binary from there and the workflows and
devcontainer must travel with it.

### DEBT-43 The gate needs jq and node — now pinned in `mise.toml`

Severity: low · Confidence: read

Fixed for a clean machine: `mise.toml` now pins `node` (the runtime the `npm:`
tools need, which mise does not add on its own) and `jq` (the JSON arithmetic in
`scripts/gobco-report.sh` and `scripts/coverage-summary.sh`), so `mise install &&
task check` no longer depends on whatever `jq`/`node` happen to be installed.

- What remains: the build container still `apt install`s jq and nodejs
  (`build/Dockerfile`) rather than provisioning the pinned versions through
  `scripts/tool-versions.sh`, so the "clean container" leg of the done-when
  belongs to DEBT-41, which also has to teach `tool-versions.sh` to pass them.

### DEBT-44 A gate can still pass having measured only part

Severity: low · Confidence: reproduced

The two clear halves are fixed: `check-file-length.sh` now captures the tracked
list up front, so a `git ls-files` that fails — outside a repository, say — stops
the gate instead of escaping `set -e` and passing on nothing measured
(`scripts/check-file-length_test.sh` reproduces the old pass and guards the fix);
and both coverage floors are required arguments rather than defaulting to 0 or 70
when a caller drops one.

Also fixed: `gobco-report.sh` now lists every package in the module, not only
those with tests, and requires a package with no tests to be named in a
`NO_TESTS` allowlist with its reason — the three thin `cmd/` mains are there. A
new package with neither tests nor an entry fails the gate rather than dropping
out of the "every package" total unseen.

- Deliberate divergence, left as is: the file-length gate measures tracked files
  only, where the license check and testshape also see untracked ones. Its header
  argues the case (a scratch file cannot fail the gate; a new file counts once it
  is `git add`ed), so this is a decision to revisit, not a defect to fix blind.

### DEBT-46 Smaller items in the tooling

Severity: low · Confidence: reproduced

- `_typos.toml` sets no locale, so `typos` accepts British spellings: the
  British forms of "color" and "organized" pass it without a word. Its header
  and CLAUDE.md both say it enforces American English. With
  `locale = "en-us"` the whole repository still passes today, and the wire
  value `cancelled` is not flagged, so the fix is free.
- `check-commit-message.sh` checks neither the 72-character limit nor the
  trailing period that CLAUDE.md states: a 127-character subject ending in a
  period passed. It reads the raw message file, so under `git commit -v` its
  unanchored `BREAKING-CHANGE:` pattern matched a line of the diff and the
  commit was refused. CI is unaffected.
- `.golangci.yml` has no `exhaustive` settings, so only switches are checked,
  and the interface deliberately uses map literals to avoid a switch arm that
  gobco cannot cover. A new `forge.CIState` or `hooks.JobState` therefore
  draws an empty glyph in silence (`internal/tui/review.go:128` and `:147`,
  `internal/tui/run.go:180`). `check: [switch, map]` exists and would flag
  them.
- Checks differ by where they run. Pre-push runs tests, golangci-lint and the
  file-length check, under a comment that says a green push is very likely a
  green CI; it runs no coverage floor, docs check, script test or whole-tree
  linter. The `shfmt` flags are written out three times. "`task check` is
  what CI runs" is said in four documents and is literally true of
  `release.yml` only; `ci.yml` restates the steps, and today restates them
  all.
- Dependabot covers the root module, Actions and `build/`. It does not cover
  `docs/go.mod` or the devcontainer, whose `base:trixie` and
  `docker-in-docker:2` float. `build/Dockerfile:14` gives `GO_VERSION` a
  default, a version written outside `mise.toml`.
- The coverage comment job asks for `pull-requests: write` with no guard for
  forks (`.github/workflows/ci.yml:103`). GitHub gives fork pull requests a
  read-only token, so it should fail on every outside contribution, outside
  the required checks. No fork pull request exists yet to show it.
- `push-release-tag.sh` pushes the tag before `release.yml` runs `task check`
  (`.github/workflows/release.yml:42`), so a gate that is red at release time
  leaves a tag with no release behind it.
- Two scripts have a test, `scripts/check-commit-message.sh` and
  `scripts/release/push-release-tag.sh`. The coverage gates, the file-length and
  license checks, `tool-versions.sh` and the docs drift check have none, and
  it is their failure paths that never run.
- A scratch `.go` file under the gitignored `tmp/` joins `go vet ./...`,
  `go test ./...` and golangci-lint, as an experiment confirmed, and CLAUDE.md
  sends scratch files there without saying so.

## Docs and configuration drift

### DEBT-49 Smaller drift — DONE

Severity: low · Confidence: read

- The usage guide's per-pane Keys table now lists the five wired bindings it
  omitted — switch view (`v`), switch task (`s`), rebase (`u`), worktree
  (`ctrl+w`) and post-when-green (`w`). `?` is complete by construction:
  `help_internal_test.go` asserts the help groups hold every binding the keyMap
  declares.
- `CONTRIBUTING.md` and `docs/content/docs/contributing.md` no longer enumerate
  the tools — they point at `mise.toml` as the list, so neither can drift from it
  or from each other.

The gobco "reads every package" comments, the version and platform lists, the
scissors line and the CLAUDE.md layout block were corrected earlier.

## Deliberate trade-offs that carry a cost

These were chosen, and the reason is written down in the code or the docs.
They are listed so the cost is visible, not so they get "fixed".

- **The issue list is one query, capped at 50, with no paging.** The pane
  says "showing N of M", so the cut is not silent. FEAT-02, FEAT-03.
- **Every redirect is refused**, in all three clients, to keep credentials
  from following one. The cost is DEBT-17's unhelpful message.
- **`gh` failures are swallowed** when resolving a token, so `doctor` cannot
  say "`gh` is installed but not signed in to this host".
- **There is no `glab` step**, because it reports its token as prose.
- **Configuration is looked for in the current directory, then home.** From
  a subdirectory of a repository, the repository's own file is skipped for
  the one at home, and `config init` writes to wherever it was run. FEAT-44.
- **Convention rules are constants**: the eleven commit types, the 72 and 48
  character limits, `fix/` and `feat/`, the `Refs:` trailer, the title from
  the oldest commit. A team with other conventions has no setting. FEAT-14.
- **`$EDITOR` is split on spaces and never given to a shell**, so an editor
  whose path contains a space fails, and `GIT_EDITOR` and `core.editor` are
  ignored.
- **Failure locations need a file extension**, which misses `Dockerfile:3`,
  though hadolint is in this repository's own gate. FEAT-25.
- **lefthook's decorative output is scraped**, with no check of the installed
  version. A change degrades to "no job rows", not to a wrong answer.
- **A line over 1 MiB ends output capture** for that run; the exit status is
  still reported.
- **Interface seams may be nil**, which costs 29 nil checks in production
  code for the benefit of partial test fakes. One path has no check
  (`startPush` calls `m.deps.Git.Push`), which today's wiring always sets.
- **Map literals stand in for switches** so that gobco has no uncoverable
  arm. The cost is in DEBT-46.
- **Timing and layout are named constants, not settings**: the ten-second
  request timeout, the twenty-second CI interval, the 90- and 60-column
  breakpoints, five comments shown. FEAT-47, UX-02.
- **Test files are exempt from the complexity linters**, and
  `internal/cli` and `internal/wiring` run their tests serially.
- **gobco runs without `-race`, one package at a time**, so `task check` runs
  the suite twice.
- **The license header's year is a fixed string.** From 2027 a new file still
  says 2026, and a contributor's own copyright line is refused. A fixed
  first-publication year is common practice; it should be written down as the
  policy.
- **Dependabot's cooldown exempts `actions/*` and `github/*`.** CLAUDE.md's
  age-gate rule names only the CVE exception.

## Checked and found clean

Recorded so the next audit can spend its time elsewhere.

- `os/exec` is imported only in `internal/proc`.
- Every response body is closed and read through a limit, and every client is
  built with a timeout.
- Server text is sanitized before it is decoded or shown, at every site in
  the three clients.
- No request or parse error echoes a URL that could hold a credential; the
  webhook URL never enters an error; Jira refuses userinfo in its base URL
  before sending anything.
- Slack announcement text is escaped; issue keys, repository paths and commit
  hashes are path-escaped.
- Configuration: unknown keys are rejected, defaults survive a partial file,
  and "local replaces home, never merged" is implemented as documented.
- Sentinel errors are static and wrapped with `%w` throughout; nothing builds
  an error at the point of failure.
- No file is over 500 lines, there is no package-level mutable state, and
  there are no goroutines outside `tea.Cmd` and `sync.OnceValues`.
- Every `//nolint` directive was read; each one's stated reason holds.
- The porcelain `-z` status parser handles spaces, quotes, newlines and
  renames. Detached heads, unborn branches and worktrees are handled. No
  parsing depends on the locale.
- `ValidateBranchName` agrees with `git check-ref-format` except for the name
  `HEAD`.
- Stale-response guards are correct for issue detail, transitions, pull
  request lookup, CI checks and the three run messages.
- All 40 `uses:` lines in the workflows are pinned to a full commit hash, and
  workflow permissions are least-privilege.
- The Go version agrees across `go.mod`, `mise.toml`, the Dockerfile, the docs
  module and the README badge. `go.mod` has no `replace`, and every direct
  dependency is used.
- The generated command reference is checked for drift in both directions.
- Every shell script sets `set -euo pipefail` and quotes its expansions; none
  disables a shellcheck rule inline.
- Across 48 commits, no subject is over 72 characters and none ends with a
  period.
