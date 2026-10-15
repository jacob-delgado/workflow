# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repository: the rules every
session needs, and pointers to the rest. An area with rules of its own keeps
them in a `CLAUDE.md` beside its code, listed under *Area guides* at the end;
read the one for a directory before your first edit there.

## What this project is

`workflow` is a Go command-line and terminal UI tool that ties Jira (on-premises
/ Data Center), a messaging service (Slack, Teams, Discord or a plain webhook),
a Git forge (GitHub or GitLab) and, where it is installed, Taskwarrior into one
developer workflow: pick up an issue, branch for it, open the pull or merge
request, tell the team. It ships as one static, pure-Go binary a developer runs
on their own machine, with three surfaces over the same domain seams (declared
in `internal/seams`, connected to the real clients in `internal/wiring`): the
Cobra command line, the Bubble Tea terminal interface it opens by default, and
an opt-in `workflow --web`, the React + TypeScript app in `web/`, embedded in
the binary and served over a REST API on `127.0.0.1` that `api/openapi.yaml`
describes — a surface, not a second implementation. A little state lives in an
on-disk SQLite store (`internal/store`), on by default and never a secret.
ARCHITECTURE.md is the map.

Layout:

```text
cmd/workflow/         thin main: cli.Execute, its exit status, the terminal prompt, the process environment
internal/cli/         the Cobra command tree
internal/cli/console/ what a command says on stderr: notes, warnings, errors and the live line of a wait
internal/cli/cmdkit/  what every command is built from: its connection, environment, shared flags, prompt and scriptable output
internal/cli/clitest/ running the command line for a test, each run in a world of its own (tests only)
internal/config/      .workflow.json loading, saving, redaction, validation
internal/keychain/    storing a token in the OS keychain and reading it back
internal/setup/       the first run: where the file goes, the Jira check, the keychain offer
internal/seams/       the seams every surface shares, over domain and loop types
internal/wiring/      connects every surface's seams to the real clients
internal/loop/        the loop composed once for every surface, over their seams
internal/progress/    how far along the loop the work is, derived each time
internal/activity/    what was done over a period, grouped by year, month, day and hour
internal/places/      where an issue can be, for narrowing the Issues list on both surfaces
internal/tui/         the Bubble Tea interface; every outside call is a Deps seam
internal/tui/frame/   a titled, bordered box of an exact size
internal/tui/layout/  where each region of the interface goes, per terminal size
internal/tui/quietinput/ the Huh field typed text goes through, whose cursor never blinks
internal/webserver/   the --web REST API and event stream, bound to 127.0.0.1
internal/webserver/problem/ the RFC 9457 problem details the --web API answers a failure with
internal/webserver/tasks/ the --web API's Taskwarrior operations, and the event stream's summary of your tasks
internal/webserver/webtest/ serving the --web API to a test, with its fakes and fixtures (tests only)
internal/web/         the built web app, committed so every build embeds it
internal/api/         the Go types and server interface generated from api/
internal/report/      what every surface reports in the API's shapes, and a failure's words
internal/store/       the on-disk SQLite store; never a secret
internal/sqlitefile/  a SQLite file opened by its path through the pure-Go driver
internal/jira/        Jira Data Center REST v2
internal/forge/       GitHub and GitLab: remotes, tokens, pull requests, templates, CI, issues
internal/messaging/   posting to Slack, Teams, Discord or a plain webhook
internal/messaging/directory/ the Slack directory people are tagged from, read once and held
internal/slackauth/   the Slack user token: refreshing, storing, locking
internal/taskwarrior/ finding Taskwarrior and reading and changing its tasks
internal/httpx/       the redirect-refusing HTTP transport the clients share
internal/fileowner/   who owns a file, as Unix keeps it (build-tagged twins)
internal/filelock/    an advisory lock on an open file (build-tagged twins)
internal/gitrepo/     reading and changing the repository through git
internal/workdirs/    the directories workflow can work in: listing, checking, naming
internal/codeowners/  who owns which paths, as GitHub or GitLab reads CODEOWNERS
internal/hooks/       lefthook: output, config, and generating lefthook.yml
internal/convention/  branch names, Conventional Commits, pull request text
internal/editor/      handing text and files to $EDITOR
internal/proc/        running programs; the one place exec lives
internal/proc/pgroup/ canceling a streamed child's whole process group (Unix)
internal/lifetime/    work that outlasts its asker: the end it still ends with, and what a quit waits for
internal/rlimit/      lowering a resource limit around one test call (tests only)
internal/ptytest/     a pseudo-terminal for a test to type at (tests only)
internal/gittest/     keeping tests off the repository a hook's git environment names (tests only)
internal/sanitize/    neutralizing terminal controls in server text
internal/buildinfo/   which build is running, from what the Go toolchain stamps
internal/testshape/   the Arrange-Act-Assert check behind cmd/testshape
internal/docsgen/     the command reference behind cmd/docsgen, from the Cobra tree
cmd/docsgen/          the thin main that runs internal/docsgen
cmd/testshape/        the thin main that runs internal/testshape
api/                  the OpenAPI contract for --web, embedded in the binary
web/                  the React + TypeScript frontend --web serves
scripts/              the gate scripts lefthook, task and CI share
build/                the build container
docs/                 the Hugo documentation site
.devcontainer/        the development container definition
.github/              CI workflows, and the issue and pull request templates
```

`task lint:layout` fails when a Go package has no line here, or a line names a
directory that holds nothing: a new package brings its line, saying what it
holds.

## Common commands

| Command | Does |
| --- | --- |
| `task --list` | every task, with descriptions |
| `task build` | build `bin/workflow`, after `web:build` rewrites the tracked bundle in `internal/web/dist` |
| `task run` | run from source; `task run -- doctor --online` passes arguments |
| `task test` | tests with the race detector |
| `task test:cover` | tests plus the coverage floor |
| `task lint` | every `lint:*` task plus `docs:check` and `gen:verify`; `task --list` names each |
| `task fmt` | format everything in place |
| `task check` | **the full gate**; CONTRIBUTING.md's *Day to day* lists its parts and what each enforces |
| `task e2e` | the web's Playwright suites, which `task check` leaves out: run it too for a change under `web/` or `internal/webserver`; it builds first, so it rewrites `internal/web/dist` too |
| `task container:check` | the same gate inside the build container |

The toolchain is pinned in `mise.toml`. After cloning: `mise trust && mise
install && task setup`.

## Working agreement

Prescriptive defaults for how changes are made. Override only when the user
explicitly asks for something different.

- **Outline first.** For tasks that would touch more than ~3 files or
  restructure a package, outline the approach first and wait for confirmation
  before writing code. Small changes and Boy Scout improvements don't need
  ceremony; large ones shouldn't start without agreement on direction.

- **Never commit to `main` — branch, and rebase before starting.** This
  applies to Claude without exception: every change lands through a branch
  named for its commit prefix (`feat/<slug>`, `fix/<slug>`, `docs/<slug>`)
  and a pull request with green CI, and a session that begins with
  uncommitted work on `main` moves it to a branch before committing. Start
  from what is on main, and rebase rather than merge, since `main` merges by
  rebase only and every commit lands as written:

  ```sh
  git fetch origin
  git switch -c feat/<slug> origin/main   # new work
  git rebase origin/main                  # work already in progress
  ```

- **Commits** follow CONTRIBUTING.md's *Commit messages*: Conventional Commits,
  the subject and body limits, one logical change per commit, and how to mark
  a breaking change. `scripts/check-commit-message.sh <file>` checks a message
  as the hook and CI do.

- **Do not bypass a hook to land work.** `LEFTHOOK=0` and `LEFTHOOK_EXCLUDE`
  exist for genuine emergencies. A failing hook is the hook working; fix the
  cause.

- **No backwards compatibility before 1.0.** workflow is only installed fresh,
  and before 1.0 it keeps no compatibility with itself: no shim, alias,
  deprecated field or "still accepted" path, and no fallback for an old format.
  A change to a public contract — the `--web` API in `api/openapi.yaml`, a
  command, flag, output or exit status (what `scripting.md` promises), a
  configuration key, a stored file's format — changes it cleanly to its best
  shape, updates every caller, test and doc, and is a breaking commit:
  `type(scope)!:` with a `BREAKING CHANGE:` footer saying what changed and
  what a user or a script does now.

- **Before 1.0, the minor digit is reserved for breaking changes.** `feat` and
  `fix` both bump the patch; only a breaking change, marked `feat!`, bumps the
  minor — the commit-msg hook refuses a body release-please would read as
  breaking unless the subject has that `!`. That is `bump-minor-pre-major` plus
  `bump-patch-for-minor-pre-major` in `release-please-config.json`, and **both
  are deliberate** — a version bump
  someone has to react to should mean something they depended on changed, not
  that features were added. This looks like a misconfiguration if you only know
  the more common pre-1.0 convention, and it has already been "fixed" once by
  mistake. Do not change it without the maintainer asking.

- **Releases are the maintainer's to publish — Claude prepares, never ships.**
  release-please opens a release PR from the Conventional Commits on main;
  merging it is what tags the version and triggers the build (CONTRIBUTING.md,
  *Releases*). Claude may write commits that feed that PR, but must **not**,
  without an instruction naming that exact action in the moment: merge the
  release PR, push a `v*` tag, or run the release workflow. A published release
  is something other people download; it cannot be cleanly withdrawn. When
  preparation is done, print the one step the maintainer takes and stop.

- **Scratch files** go in `tmp/` under the repo root, which is gitignored, or
  in the session's own scratchpad directory; never anywhere else outside the
  repo. A scratch `.go` file in `tmp/` still joins `go test ./...`, so keep Go
  scratch outside the module.

## Code style

- **Go**: follow [Effective Go](https://go.dev/doc/effective_go) and
  [Google's Go Style Guide](https://google.github.io/styleguide/go/). Accept
  interfaces, return structs; small interfaces (1–3 methods); embedding only
  for behavior delegation; three similar lines beat one abstract one. Keep the
  build pure Go (`CGO_ENABLED=0`): the release binaries cross-compile to every
  platform `RELEASE_PLATFORMS` names in `Taskfile.yml` only without cgo.

- **Interface compliance — assert it at compile time.** Every concrete type
  meant to satisfy an interface carries a static assertion next to its
  definition, in the form matching its receiver: `(*T)(nil)` for a pointer,
  as `var _ api.StrictServerInterface = (*server)(nil)`, and `T{}` for a
  value, as `var _ tea.Model = Model{}`.

- **One receiver kind per type** (`recvcheck`): choose when the type is
  introduced and hold to it. `ireturn` flags returning an interface: return a
  concrete type or a func type, and keep `//nolint:ireturn` for the signatures
  a third-party interface forces on you, as `tea.Model` does.

- **Language: American English.** Identifiers, comments, docs, commit messages,
  and user-facing copy all use American spellings — color, canceled, organize.
  Enforced by `misspell` (Go, locale US) and `typos` (repo-wide, `_typos.toml`),
  both wired into lefthook and `task lint`.

- **Naming**: identifiers reveal intent without a comment; a name that needs
  one is wrong. Abbreviations only where universally understood (`ctx`,
  `err`, `cfg`), and no single-letter names outside loop counters.

- **Function size and focus**: a function does **one thing**, in about 25
  lines at most and one level of abstraction; if describing it needs "and",
  extract.

- **McCabe cyclomatic complexity ≤ 10 — enforced, not aspirational.** `gocyclo`
  fails past 10, with `gocognit` (≤ 20) and `funlen` as backstops; test files
  are exempt.

- **Comments**: default to none; only a WHY that is non-obvious, never WHAT the
  code does. Doc comments on exported symbols are expected: the contract.

- **Error handling — explicit and early.** Never swallow an error. Wrap with
  `%w` when crossing into another package, and define static sentinels
  (`var ErrNotFound = errors.New(…)`), never a dynamic error at the point of
  failure (`err113`).

- **Every seam method that does I/O takes `ctx context.Context` first.** A
  surface passes the context of whatever asked — a web read its request's, the
  command line `cmd.Context()`, the terminal its session's — and the wiring
  never keeps a context past the call that handed it one. What must outlive its
  asker detaches where the work is done, each case written down once:
  `Execute`'s doc comment (`internal/cli/cli.go`), TRADE-37 and TRADE-38,
  and `internal/lifetime`. `internal/seams`' test holds the rule and names
  the exceptions.

- **File length — 500-line soft target, 800-line hard ceiling, 700 for a
  test**, gated by `scripts/check-file-length.sh` (`--list` for the
  standings, its header for why). A file past 500 usually carries more than one
  concern: split it file-per-concern, a test along its behaviors with shared
  fixtures in a sibling file.

- **Package & directory size — cohesion first, a budget as the backstop.** A Go
  package is *one* cohesive responsibility behind a small exported API; many
  files in it is idiomatic and good, and a package must **not** be split to
  chase a number. Cohesion also says what may leave: extract code into a
  package of its own when it has a reason to change of its own *and* the
  extraction makes no import cycle, even out of a package cohesive as a whole —
  the web server's request boundary, its Taskwarrior handlers and git runs, the
  command line's doctor checks and pr/announce flow, a Huh widget of the
  terminal's own. What hangs off a shared type stays with it: a pane of
  `internal/tui` takes and returns `Model`. On the web, group by feature
  (`web/src/features/<feature>/`). Every grouping also answers to a **file
  budget**, `scripts/check-package-size.sh` against
  `scripts/package-size-budgets.txt`, whose header says what counts and the
  two kinds of entry; **read the numbers there, never here.** A trip has two
  honest answers: **split** when the grouping carries a second reason to
  change, cycle-free, or **bump** (raise the number, rewrite the WHY above the
  entry, append a row to `scripts/package-size-budget-history.md`) when the
  new file is the same responsibility one concern wider. "The gate was in the
  way" is neither.

- **License headers**: every `.go` file begins with the two SPDX lines
  CONTRIBUTING.md gives; `scripts/check-license-headers.sh` gates it.

- **Shell** follows the style `scripts/CLAUDE.md` sets, wherever it lives.

## Design principles

- **SOLID — a lens, not ceremony.** Go is not OOP; apply the underlying idea
  where it improves testability or readability.
  - **S — Single responsibility.** A function or type does one thing; config
    loading, command wiring and rendering live in separate packages.
  - **O — Open/closed.** Adding a variant should *extend*, not edit.
    `gochecknoglobals` is on, so a package-level lookup map is a build
    failure: build the map in a constructor and hang it off the value that uses
    it, put the behavior on the discriminant type as a method, or keep a
    `switch` where every kind can arrive and let `exhaustive` guard it. A
    function only some kinds reach compares the kind to those it handles, and
    its doc comment says what any other kind gets: a case no run can take is a
    condition the gate fails as never evaluated. Exempt: `var Err… =
    errors.New(…)` sentinels; a compiled regular expression, declared once as
    a package-level `var` named for what it matches (only a pattern built from
    run-time input keeps a builder, outside any loop); and a value derived once
    from immutable embedded data, with a `//nolint:gochecknoglobals` and its
    reason, as the web server's parsed contract has.
  - **L — Liskov.** A fake that cuts corners is a broken fake, not a shortcut.
  - **I — Interface segregation.** Small consumer-side interfaces at the point
    of use.
  - **D — Dependency inversion.** Depend on a seam: `config.Load(workDir,
    homeDir)` takes its directories rather than reading the environment.

- **Composition over inheritance**, and **function-variable seams over
  interfaces for one-method dependencies**: `now func() time.Time` beats a
  `Clock` interface.

- **Law of Demeter — accept what you read**: bundle many returned values into
  one typed value. **DRY with the rule of three**: two is coincidence.

- **YAGNI — hard line.** No speculative interfaces, no "just in case" error
  handling for impossible conditions, no backwards-compat shims for unreleased
  code. If a feature is needed, the user will ask.

- **No premature goroutines.** A goroutine starts only in `internal/proc`, the
  tests, and the exceptions `scripts/check-goroutines.sh` writes down.

- **Boy Scout Rule — leave it better than you found it.** Every time you touch a
  file, improve one thing: rename a cryptic identifier, break up an oversized
  function, remove dead code, delete a stale comment, drop a branch. This is not
  optional on a feature or fix branch — it is part of the definition of done.
  Land the improvement as its own commit (`refactor:`, `style:` or `chore:`) on
  the same branch, before or after the feature or fix commit, so each commit
  stays one logical change.

### Code smells

A smell is a *hint* to look closer, not a defect to reflexively refactor —
weigh it against YAGNI and the rule of three first. The linters already fail
the smells they can see (a long or deeply nested function, a magic number, an
ignored or dynamic error, a naked return, dead or duplicated code, a stutter);
review looks for the ones they cannot:

- a bare `string` or `int` carrying domain meaning (an issue key, a channel
  name) → a named type; the same few fields traveling together → a type;
  five or more positional parameters → a struct, or a split;
- a boolean that forks behavior, `f(…, true)` → two functions or a named enum;
  a parameter reassigned in place → return a new value;
- a field set in some flows and nil in others → a separate type or a
  parameter; a type that fights what it embeds → compose small parts;
- one file edited for unrelated reasons → split by reason to change; one
  change touching many files → centralize the knowledge;
- a comment covering for a bad name → rename; an abstraction for a caller that
  doesn't exist, or a type that only delegates → delete or inline it;
- a method using another type's data more than its own, a reach into another
  unit's internals, or `a.b().c().d()` through layers → move the method, use
  the public surface, pass the one bundled value needed;
- an interface with one implementation, or declared by its producer → declare
  it at the consumer once a second implementation or a fake earns it;
- `""`, `0` or `nil` meaning "no result" → a typed result or an explicit
  error.

## Testing

### TDD process

For **new features and bug fixes**:

1. **RED first.** Write a failing test that reproduces the bug or demonstrates
   the feature's contract. Run it. Watch it fail with a message that names the
   gap.
2. **GREEN minimal.** The smallest production change that makes it pass.
3. **REFACTOR if it earns it**, while the tests stay green; mechanical
   reshuffling is noise.

For a bug fix, the failing test that reproduces the bug is the most valuable
artifact in the commit; do **not** write the fix first and add a test "to
cover it". Each area's `CLAUDE.md` names the harness and fixtures to reuse.

**Test public interfaces, not internals — black-box only.** Tests declare
`package <name>_test` (`testpackage` enforces it). If something seems
untestable black-box, fix the API, don't white-box the test.

**Arrange, Act, Assert — marked in every test.** Every `Test` and `Fuzz` body
names its parts with a comment on a line of its own, below any `t.Parallel()`:
`// Arrange`, `// Act`, `// Assert`. Leave out a part that would be empty, and
when the call under test sits inside its check — `if got := f(); got != want` —
mark that part `// Act & Assert`. One Act per test: independent scenarios run
back to back are separate tests or table cases. The exception is a flow whose
intermediate states are themselves the contract ("nothing is posted before the
preview is confirmed"), which labels every step instead — `// Act: open the
preview`, `// Assert: nothing is sent yet` — while a single-cycle test carries
no labels, because its name is the label. A table test puts the markers inside
each `t.Run` closure, a fuzz test inside `f.Fuzz`, and a test on a fake clock
inside its `synctest.Test` bubble; the cases, the loop and the seeds carry none.
A test whose Assert reaches no `t.Error` or `t.Fatal`, directly or through a
helper, asserts nothing and is useless. `cmd/testshape` fails a body whose
markers are missing, malformed or out of order, or whose Assert reaches no
failure; it runs in `task lint`, on commit, and in CI. The web's unit tests and
e2e specs answer to the same rule through its TypeScript twin,
`web/eslint-rules/arrange-act-assert.js`, in `corepack yarn lint`. It is a
floor: it cannot tell a meaningful assertion from one that passes whatever the
Act did, so ask of every Assert whether it would fail if the Act did nothing.

**Tables when cases differ in data, not behavior.** A case that needs its own
closure to set up or to check is clearer as a test of its own.

**Coverage: read the floors in the gates, never here** — `COVERAGE_MIN` and
`BRANCH_COVERAGE_MIN` in `Taskfile.yml`, and each consequential package's own
condition floor in `scripts/gobco-floors.txt`. Statement coverage says a line
ran; condition coverage (gobco, `task cover:branch`) says whether an
`if a && b` was ever seen with `b` false, and its per-condition output is a
worklist of missing test cases, not a percentage to chase. A condition no test
evaluated, or an error check whose error arm no test reached, fails the gate
unless `scripts/gobco-allowlist.txt` keeps it for a trade-off;
CONTRIBUTING.md's *Tests* says what counts as an error check. A floor is
raised only once the coverage is already there, to `floor(measured) − 2`,
which the gates print. These are minimums, not targets — aim higher where the
code touches a credential.

**Exempt** (no TDD ceremony): typo fixes, doc-only edits, formatter and linter
passes, dependency bumps, configuration-only changes and behavior-free
scaffolding. A refactor that changes observable behavior needs a test.

**Before declaring any task done**, run `task check`; for a change under `web/`
or `internal/webserver` also run `task e2e`, since `task check` leaves the
Playwright suites out. Run the gate as a user, not as root: root reads any
file, so the tests that make one unreadable skip under it, and `task
cover:branch` can fail where it passes as a user (`scripts/gobco-report.sh`
warns when it starts as root). In a session whose `id -u` is 0, run it as
another user that owns a checkout (the dev container's `vscode`), or, where
there is none, take CI's Branch coverage job, which runs as a user, as its
verdict and say so. Never present work as complete while the build is red or
tests are failing — say what's broken and why instead.

## Secrets, dependencies and contracts

- **Never log or print a secret.** Any credential — a `config.Secret` field:
  the tokens, the webhook URL and every `jira.headers` value — is masked by
  `config.Redact` before it reaches any output. A code path that touches one
  ships with the test that proves it never reaches output unmasked; `gitleaks`
  (`task secrets`) is the backstop, not the plan.

- **The store (`internal/store`) never holds a secret, and is never
  trusted.** It is keyed only by credential-free identifiers, and the text in
  it is sanitized on the way in and again at the seam that renders it.

- **New dependencies require approval.** Propose a module, name it and say why,
  and get explicit approval before adding it; prefer the standard library and
  what `go.mod` already has.

- **Week-long dependency age gate.** Never adopt a version published less than
  7 days ago — freshly compromised releases are usually detected and yanked
  within days. Go has no native gate: before `go get`, check the publish date
  (`curl https://proxy.golang.org/<module>/@v/<version>.info`). The same rule
  governs `mise.toml` and `web/yarn.lock`, and
  `scripts/check-dependency-age.sh` holds every pull request to it.
  **Known-CVE fixes override the cooldown — always**: list such a version in
  `scripts/dependency-age-exceptions.txt` with its advisory. The maintainer,
  and only the maintainer, may also waive a version by name, with a dated
  `waived-YYYY-MM-DD` line there. An npm version takes a second line, in
  `web/.yarnrc.yml`'s `npmPreapprovedPackages`, which yarn's own gate reads.
  Each line goes once its version is a week old. Dependabot's cooldown exempts
  `actions/*` and `github/*`, GitHub's own actions, which this rule governs
  by hand.

- **Tool versions are exact, and they live in `mise.toml`**, whose header says
  how `build/Dockerfile` and CI read them. Every copy that has to live
  elsewhere is gated, not trusted: Go's version (`scripts/check-go-version.sh`),
  mise's own on each `jdx/mise-action` step and in the dev container
  (`scripts/check-mise-version.sh`), the container's Go patch (the
  Dockerfile's check after `FROM`), and each downloaded tool's SHA-256 lines
  in `build/tool-checksums.txt` (`task lint:checksums`). A floating `latest`
  changes what the gate accepts without anyone deciding to.

- **The `--web` API is spec first.** Change `api/openapi.yaml`, then run `task
  gen` (the Go side) and `corepack yarn gen` in `web/` (the web client), so
  both stay generated from it; a change there usually reaches `web/` and
  `docs/` too. Its errors are RFC 9457 problem details, whose rules are in
  `internal/webserver/CLAUDE.md`.

- **The command reference is generated.** Everything under
  `docs/content/docs/reference/` but `_index.md` comes from the Cobra tree:
  a command or flag change runs `task docs:gen` in the same commit, and
  `task docs:check` fails when the two disagree.

- **Kept trade-offs live in `TECH_DEBT.md`'s register**: an entry headed
  `### TRADE-n <title>` with **Decided.**, **Cost.** and **Reopen when.**
  paragraphs, and a `Trade-off TRADE-n:` comment at the code it keeps
  (`scripts/check-tradeoffs.sh`). A review or an audit drops a finding that
  restates an entry unless the entry's reopen trigger has fired.

## Area guides

A nested `CLAUDE.md` loads into a session, subagents included, the first time
the session reads or edits a file in its directory, and not before. So read
the one for a directory before your first edit there, and the rules a change
needs before it reaches one (spec first, secrets, dependencies, commits,
releases, no backwards compatibility) stay above.

| File | Read it before you change |
| --- | --- |
| `internal/store/CLAUDE.md` | a table, a key, or how the store's files are opened |
| `internal/tui/CLAUDE.md` | a pane, an overlay, a key or a form of the terminal interface |
| `internal/webserver/CLAUDE.md` | an API operation, a guard, a problem or the event stream |
| `internal/cli/CLAUDE.md` | a command, a flag, what a command prints or how it exits |
| `web/CLAUDE.md` | anything under `web/`, the `--web` frontend |
| `scripts/CLAUDE.md` | a gate script, its test, or a data file a gate reads |
| `docs/CLAUDE.md` | a page of the documentation site |
