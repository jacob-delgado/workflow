# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository. It holds **only what's relevant in every session**.

## What this project is

`workflow` is a Go command-line and terminal UI tool that ties Jira (on-premises
/ Data Center), Slack, and a Git forge (GitHub or GitLab) into one developer
workflow: pick up an issue, branch for it, open the pull or merge request, tell
the team.

Go at its core, shipped as a single static binary a developer runs on their own
machine: no database, and nothing stored between sessions. The default surface
is a terminal UI — the Charm stack (Bubble Tea, Bubbles, Lip Gloss) with Cobra
for the command tree. An opt-in `workflow --web` serves a React + TypeScript
frontend, embedded in the same binary, over a local REST API bound to
`127.0.0.1`, described by `api/openapi.yaml`; it drives the same domain seams
(`internal/wiring`) the TUI and CLI already do — a surface, not a second
implementation. The web frontend lives in `web/`.

Layout:

```text
cmd/workflow/         thin main; wires cli.Execute and the exit status
internal/cli/         the Cobra command tree
internal/config/      .workflow.json loading, redaction, validation
internal/wiring/      connects the interface's seams to the real clients
internal/tui/         the Bubble Tea interface; every outside call is a Deps seam
internal/jira/        Jira Data Center REST v2
internal/forge/       GitHub and GitLab: remotes, tokens, pull requests, CI
internal/slack/       posting through a bot token or a webhook
internal/httpx/       the redirect-refusing HTTP transport the clients share
internal/gitrepo/     reading and changing the repository through git
internal/hooks/       lefthook: output, config, and generating lefthook.yml
internal/convention/  branch names, Conventional Commits, pull request text
internal/editor/      handing text and files to $EDITOR
internal/proc/        running programs; the one place exec lives
internal/sanitize/    neutralizing terminal controls in server text
internal/testshape/   the Arrange-Act-Assert check behind cmd/testshape
cmd/docsgen/          generates the command reference from the Cobra tree
cmd/testshape/        the thin main that runs internal/testshape
scripts/              the gate scripts lefthook, task and CI share
build/                the build container
docs/                 the Hugo documentation site
.devcontainer/        the development container definition
```

## Common commands

| Command | Does |
| --- | --- |
| `task --list` | every task, with descriptions |
| `task build` | build `bin/workflow` |
| `task run` | run from source; `task run -- doctor --online` passes arguments |
| `task test` | tests with the race detector |
| `task test:cover` | tests plus the coverage floor |
| `task lint` | every linter (Go, shell, YAML, Dockerfile, Actions + security, Markdown, TOML, headers, spelling, file length, test markers, docs drift) |
| `task fmt` | format everything in place |
| `task cloc` | count the source lines, and the Go test ratio (advisory) |
| `task check` | **the full gate** — lint, tests + coverage, govulncheck, gitleaks |
| `task container:check` | the same gate inside the build container |

The toolchain is pinned in `mise.toml`. After cloning: `mise trust && mise
install && task setup`.

## Working style

Prescriptive defaults for how new code should be written and how changes should
be made. Override only when the user explicitly asks for something different.

For tasks that would touch more than ~3 files or restructure a package, outline
the approach first and wait for confirmation before writing code. Small changes
and Boy Scout improvements don't need ceremony; large ones shouldn't start
without agreement on direction.

### Code style

- **Go**: follow [Effective Go](https://go.dev/doc/effective_go) and
  [Google's Go Style Guide](https://google.github.io/styleguide/go/). Accept
  interfaces, return structs. Small interfaces (1–3 methods). Composition over
  inheritance. Embedding only for behavior delegation, never just to store a
  field. No premature abstraction — three similar lines beat one abstract one.
  Keep the build pure Go (`CGO_ENABLED=0`): the release binaries cross-compile
  to five platforms, and that only stays true without cgo.

- **Interface compliance — assert it at compile time.** Every concrete type
  meant to satisfy an interface carries a static assertion next to its
  definition, so a drifting method set breaks the build at the type rather than
  at some distant call site: `var _ tea.Model = (*Model)(nil)`. Use the form
  matching the receiver — `(*T)(nil)` for pointer receivers, `T{}` for value
  receivers.

- **One receiver kind per type.** `recvcheck` fails a type whose methods mix value
  and pointer receivers, so choose when the type is introduced and hold to it: a
  type that satisfies an interface by value (`var _ tea.Model = Model{}`) is
  all-value from then on. Relatedly, `ireturn` flags returning an interface —
  return a concrete type or a func type, and keep `//nolint:ireturn` for the
  signatures a third-party interface forces on you, as `tea.Model` does.

- **Language: American English.** Identifiers, comments, docs, commit messages,
  and user-facing copy all use American spellings — color, canceled, organize.
  Enforced by `misspell` (Go, locale US) and `typos` (repo-wide, `_typos.toml`),
  both wired into lefthook and `task lint`.

- **Naming**: identifiers must reveal intent without a comment. If you find
  yourself writing a comment to explain a name, the name is wrong — rename it.
  Abbreviations only where universally understood (`ctx`, `err`, `cfg`). No
  single-letter names outside loop counters. Exported names are the public
  contract and deserve extra care; unexported names should still be unambiguous
  in their package.

- **Function size and focus**: functions and methods do **one thing**. Aim for
  ~25 lines as a soft ceiling. When a function grows past that, or handles more
  than one level of abstraction, extract. The test: can you describe what it does
  in a single clause without using "and"?

- **File length — 500-line soft target, 800-line hard ceiling.** Aim for under
  500: a file past that is usually carrying more than one concern and wants
  splitting, file-per-concern. `scripts/check-file-length.sh` *warns* past 500
  but only *fails* past 800, so the guidance nudges without blocking a file with
  a genuine reason to be long. It gates every tracked `.go` and `.sh` file in
  `task lint` and on pre-push; `--list` prints the current standings, flagging
  each file `soft` or `OVER`. Tests count: a 900-line test file usually means
  the unit under test does too much. There is no exemption list, deliberately —
  add one only when a file genuinely earns it, with the reason written beside it.

- **Package & directory size — cohesion first, a budget as the backstop.** Size
  a grouping by responsibility, not by a file count. A Go package is *one*
  cohesive responsibility behind a small exported API; many files in it is
  idiomatic and good — `internal/tui` is one interface spelled file-per-pane and
  must **not** be split to chase a number. Split a package only when it carries
  more than one reason to change *and* the extraction won't make an import
  cycle; on the web, group by feature (`web/src/features/<feature>/`), never one
  flat directory. But cohesion is a judgment call and judgment loses to entropy —
  a folder reaches thirty files one defensible file at a time. So every grouping
  also answers to a **file budget**, gated by `scripts/check-package-size.sh`
  against `scripts/package-size-budgets.txt` (`task lint`, `task check`, CI, and
  lefthook pre-push; `--list`, or `task package-size`, prints the standings).
  **Read the numbers there, never here** — a prose restatement drifts. Budgets
  are zero-headroom both ways: over the number fails, and so does a budget left
  sitting *above* its directory's count (a forgotten post-split ratchet). Counts
  are source files: unit tests, stylesheets and generated code don't count, but
  integration tests (`*_integration_test.go`) and e2e specs (everything under
  `web/e2e/`) do — an end-to-end surface must break into per-surface folders
  rather than hide a hundred scenario files in one directory. A trip has
  two honest answers, named in the gate's own output: **split** when the grouping
  really carries a second reason to change, or **bump** (raise the number,
  rewrite the WHY above the entry, and append a row to
  `scripts/package-size-budget-history.md`) when the new file is the same
  responsibility one concern wider. "The gate was in the way" is neither.

- **McCabe cyclomatic complexity ≤ 10 — enforced, not aspirational.** `gocyclo`
  fails the build past 10, with `gocognit` (≤ 20) and `funlen` as backstops.
  This is a small program; a function needing more than ten branches wants
  extracting. Test files are exempt — a test's branch count is its assertions.

- **Comments**: default to none. Only when the WHY is non-obvious — a hidden
  constraint, a surprising invariant, a workaround for a specific bug. Never
  re-explain WHAT the code does; well-named identifiers already do that.
  Exception: doc comments on exported symbols are expected — they document the
  contract, not the implementation.

- **Error handling — explicit and early.** Return errors; never swallow them
  silently. Wrap with `%w` when crossing into another package's territory, and
  define static sentinel errors (`var ErrNotFound = errors.New(…)`) rather than
  building dynamic ones at the point of failure — `err113` enforces this. Avoid
  sentinel zero-values as implicit "no result" signals when a typed result or
  error would be clearer.

- **License headers**: every `.go` file begins with the two SPDX lines from
  CONTRIBUTING.md. `scripts/check-license-headers.sh` gates this in lefthook,
  `task lint` and CI.

- **Shell scripts**: follow the
  [Google Shell Style Guide](https://google.github.io/styleguide/shellguide.html).
  `shfmt -i 2 -ci -bn` formatting, shellcheck-clean against the repo's
  `.shellcheckrc` (every optional check on).

- **Accessibility is enforced, not aspirational (the `--web` app).**
  `eslint-plugin-jsx-a11y` runs at **strict** in the web lint (`web/eslint.config.js`,
  in `yarn lint` / `task check`), and a runtime **axe** scan
  (`web/e2e/a11y.spec.ts`, `yarn test:e2e`) fails on any WCAG 2.1 A/AA violation —
  across every section **and both themes**, because a light theme is only real
  once its contrast holds. Keep both green: give every control an accessible name
  (an icon-only button carries an `aria-label`), keep the skip link and the
  `:focus-visible` ring, and de-emphasize with color rather than `opacity` (which
  dims text below the contrast floor). The web tests are black-box — assert on
  role, accessible name and `aria-current`, never on classes, inline styles or
  `data-*` hooks. The TUI needs no theme of its own: it draws in ANSI palette
  indices, so the terminal's own theme — light or dark — already decides the
  shades.

### Design principles

- **SOLID — a lens, not ceremony.** Go is not OOP; apply the underlying idea
  where it improves testability or readability.
  - **S — Single responsibility.** A function or type does one thing (see
    *Function size*, *File length*). Config loading, command wiring, and
    rendering are separable concerns and live in separate packages.
  - **O — Open/closed.** Adding a variant should *extend*, not edit. Note how to
    spell that here: `gochecknoglobals` is on, so a package-level lookup map is
    a build failure, not an option. Build the map inside a constructor and hang
    it off the value that uses it, put the behavior on the discriminant type as
    a method, or keep a `switch` and let `exhaustive` fail the build when a new
    case is added without a branch. Package-level `var Err… = errors.New(…)`
    sentinels are exempt and stay.
  - **L — Liskov substitution.** An implementation honors the contract its
    callers rely on; a fake that cuts corners is a broken fake, not a shortcut.
  - **I — Interface segregation.** Depend only on what you use. Declare small
    consumer-side interfaces at point of use, not fat producer-side ones.
  - **D — Dependency inversion.** High-level logic depends on a seam, not a
    concrete. `config.Load(workDir, homeDir)` takes its directories as arguments
    rather than reading the environment, which is exactly what lets its tests run
    in parallel against temp directories.

- **Composition over inheritance.** Assemble small collaborators. Embed for
  behavior delegation only, never merely to borrow a field.

- **Prefer function-variable seams over interfaces for one-method
  dependencies.** When a seam has a single method and a single fake, an interface
  is YAGNI. `now func() time.Time` beats a `Clock` interface.

- **Law of Demeter — accept what you read.** When a call returns many values,
  bundle them into a single typed value rather than threading scalars through
  layers.

- **DRY with the rule of three.** Don't extract on the second occurrence — two is
  coincidence.

- **YAGNI — hard line.** No speculative interfaces, no "just in case" error
  handling for impossible conditions, no backwards-compat shims for unreleased
  code. If a feature is needed, the user will ask.

- **Boy Scout Rule — leave it better than you found it.** Every time you touch a
  file, improve one thing: rename a cryptic identifier, break up an oversized
  function, remove dead code, delete a stale comment, drop a branch. This is not
  optional on feature or fix commits — it is part of the definition of done.

### Code smells

A smell is a *hint* to look closer, not a defect to reflexively refactor — weigh
it against YAGNI and the rule of three first (a two-case `switch` is not yet a
registry). A parenthetical marks which linter catches it (**lint**) or which rule
above it restates (*see*). The rest is review-time judgment.

**Bloaters** — grown too big to hold in your head:

- *Long method* — does more than one thing → extract (*see Function size*;
  **lint** gocyclo, gocognit, funlen).
- *Large type / god package* — too many responsibilities → split by concern
  (*see File length*).
- *Primitive obsession* — a bare `string`/`int` carrying domain meaning (an issue
  key, a channel name) → a named type the compiler can track.
- *Long parameter list* — 5+ positional params → bundle the cohesive ones into a
  struct, or split.
- *Data clumps* — the same few fields travel together everywhere → give them a
  type.

**Object-orientation abusers:**

- *Type/kind `switch` every new case must edit* → a map built in a constructor
  and keyed by the discriminant, or a method on that type (*see Open/closed* for
  why it cannot be a package-level map; **lint** gochecknoglobals, and exhaustive
  keeps a genuine switch honest).
- *Temporary field* — set in some flows, nil otherwise → a separate type or a
  parameter.
- *Refused bequest* — a type that ignores or fights what it embeds → compose
  small parts instead.

**Change preventers:**

- *Divergent change* — one file edited for unrelated reasons → split by reason to
  change.
- *Shotgun surgery* — one conceptual change touches many files → centralize the
  knowledge in one place.

**Dispensables:**

- *Comments as deodorant* — a comment covering for a bad name → rename.
- *Duplicated code* → extract on the third occurrence (*see DRY*).
- *Dead code* — unused funcs/params/vars/branches → delete (**lint** unused,
  unparam, ineffassign).
- *Speculative generality* — abstraction for a caller that doesn't exist →
  delete (*see YAGNI*).
- *Middle man / lazy type* — a type that only delegates → inline it.

**Couplers:**

- *Feature envy* — a method uses another type's data more than its own → move it
  onto that type.
- *Inappropriate intimacy* — reaching into another unit's internals; tests
  asserting on privates → use the public surface (*see Test public interfaces*).
- *Message chains* — `a.b().c().d()` threaded through layers → pass the one
  bundled value needed.

**Modern additions:**

- *Boolean/flag parameter* — `f(…, true)` that forks behavior → two functions or
  a named enum, so the call site reads.
- *Magic number/string* — an unexplained literal → a named constant (**lint**
  mnd).
- *Deep nesting / arrow code* — pyramids of `if` → early returns, guard clauses,
  extracted helpers (**lint** nestif).
- *Mutating a parameter* — reassigning an argument in place → return a new value.

**Go-specific:**

- `interface{}` / `any` as a shortcut → a concrete type or a small interface
  (**lint** gocritic flags a range of Go micro-smells).
- *Interface pollution* — an interface with one implementation, or defined on the
  producer side → declare it at the *consumer*, once a second implementation or a
  fake earns it. A one-method seam is a func var, not an interface.
- *Ignored error* — `_ = f()` dropping a real error → handle or return it
  (**lint** errcheck).
- *Dynamic errors* — `errors.New` at the point of failure → a package-level
  sentinel, wrapped (**lint** err113).
- *Sentinel zero-value as "no result"* — `""`/`0`/`nil` meaning absence → a typed
  result or an explicit error.
- *Naked return* in more than a couple of lines → return values explicitly
  (**lint** nakedret).
- *Stutter* — `config.ConfigLoader`, `tui.TUIModel` → drop the package prefix.
- *Premature goroutines/channels* — concurrency with no measured need → simple
  synchronous code first (*see YAGNI*).

### TDD process

For **new features and bug fixes**:

1. **RED first.** Write a failing test that reproduces the bug or demonstrates
   the feature's contract. Run it. Watch it fail with a message that names the
   gap.
2. **GREEN minimal.** Smallest production change that makes the test pass. Resist
   scope creep.
3. **REFACTOR if it earns it.** Clean up only when the resulting shape is
   genuinely better. Mechanical reshuffling is noise.

For bug fixes specifically: the failing test that reproduces the bug is the most
valuable artifact in the commit — it documents both the bug and the contract that
prevents its return. Do **not** write the fix first and add a test "to cover it";
ordering matters.

**Test public interfaces, not internals — black-box only.** Tests declare
`package <name>_test` (the external test package), so only exported identifiers
are reachable — `testpackage` enforces it. Do not reach into unexported helpers
or assert on private fields. If something seems untestable black-box, that is a
design smell: fix the API, don't white-box the test.

**Arrange, Act, Assert — marked in every test.** Every `Test` and `Fuzz` body
names its parts with a comment on a line of its own, below any `t.Parallel()`:
`// Arrange`, `// Act`, `// Assert`. Leave out a part that would be empty, and
when the call under test sits inside its check — `if got := f(); got != want` —
mark that part `// Act & Assert`. One Act per test: independent scenarios run
back to back are separate tests or table cases. The exception is a flow whose
intermediate states are themselves the contract ("nothing is posted before the
preview is confirmed"), which labels every step instead — `// Act: open the
preview`, `// Assert: nothing is sent yet` — while a single-cycle test carries no
labels, because its name is the label. A table test puts the markers inside each
`t.Run` closure and a fuzz test inside `f.Fuzz`; the cases, the loop and the
seeds carry none. A test whose Assert reaches no `t.Error` or `t.Fatal`, directly
or through a helper, asserts nothing and is useless. `cmd/testshape` fails a body
whose markers are missing, malformed or out of order, or whose Assert reaches no
failure; it runs in `task lint`, on commit, and in CI. It is a floor: it cannot
tell a meaningful assertion from one that passes whatever the Act did, so ask of
every Assert whether it would fail if the Act did nothing.

**Tables when cases differ in data, not behavior.** When adding tests, prefer a
table-driven test for cases that differ only in their inputs and expectations,
as Go's own tests do. It is a preference, not a rule: a case that needs its own
closure to set up or to check is clearer as a test of its own.

**Read the coverage floors in the gates, never here** — `COVERAGE_MIN` and
`BRANCH_COVERAGE_MIN` in `Taskfile.yml`, enforced by `scripts/coverage-gate.sh`
and `scripts/gobco-report.sh`. A number restated in prose drifts from the number
the gate enforces.

**Two coverage metrics, and the second is the useful one.** Statement coverage
says a line ran; condition coverage (gobco, `task cover:branch`) says whether an
`if a && b` was ever seen with `b` false. Its per-condition output — "condition
`err != nil` was 8 times false but never true" — is a worklist of missing test
cases, not a percentage to chase. gobco reads every package in this module
(`scripts/gobco-report.sh` would name any it cannot); a package that becomes
unreadable without being listed fails the gate rather than quietly shrinking what
the number covers.

**Raising the floor: the ratchet is `floor(measured) − 2`.** Two points is the
whole tolerance — enough for an incidental refactor, not enough to land a feature
with its tests missing. A floor is raised only after the coverage is already
there; the gate prints the available ratchet each run.

These are minimums, not targets — aim higher where the code is consequential
(credential handling, redaction, anything that touches a token).

**Exempt** (no TDD ceremony): typo fixes, doc-only edits, formatter/linter
passes, dependency bumps, configuration-only changes, and repo scaffolding that
carries no behavior. Use judgment for refactors — extracting a helper rarely
needs a new test, but changing observable behavior does.

**Before declaring any task done**, run `task check`. Never present work as
complete while the build is red or tests are failing — say what's broken and why
instead.

### What to avoid

The quick list; see **Code smells** for the full catalog and what's lint-enforced.
Speculative interfaces; abstract layers without a second concrete caller;
backwards-compat shims for unreleased code; "just in case" error handling for
impossible conditions; over-engineering for hypothetical future requirements;
tests that assert on unexported identifiers or internal data structures rather
than observable, public behavior.

## Cross-cutting conventions

- **New dependencies require approval.** Do not add Go modules without first
  proposing the dependency and getting explicit approval. Prefer the standard
  library and what's already in `go.mod`. When a new dependency is genuinely the
  right call, name it and explain why before adding it.

- **Week-long dependency age gate.** Never adopt a version published less than 7
  days ago — freshly compromised releases are usually detected and yanked within
  days. Go has no native gate: before `go get`, check the publish date
  (`curl https://proxy.golang.org/<module>/@v/<version>.info`) and pick an older
  version if it is younger than a week. The same rule governs `mise.toml` pins.
  **Known-CVE fixes override the cooldown — always.** The gate guards against
  *unknown* compromised releases, not *published* security fixes. Dependabot's
  cooldown deliberately exempts `actions/*` and `github/*`: they are GitHub's own
  first-party actions, and this age-gate rule is what governs them by hand.

- **Tool versions are exact, and they live in `mise.toml`.** Nothing else states
  a version: `build/Dockerfile` reads them through
  `scripts/tool-versions.sh`, and CI provisions them with mise. A floating
  `latest` changes what the gate accepts without anyone deciding to.

- **Never log or print a secret.** Tokens are masked by `config.Redact` before
  they reach any output — `config show`, the TUI, and error messages all go
  through it. When adding a code path that touches `jira.token` or
  `slack.token`, the test that proves it doesn't leak is part of the change.
  `gitleaks` (`task secrets`) is the backstop, not the plan.

- **Use `tmp/` under the repo root for ad-hoc scratch files** — never `/tmp/…` or
  any path outside the repo. PR-body drafts, intermediate output, log dumps:
  `tmp/foo.md` (gitignored). One caveat: a scratch `.go` file there still joins
  `go test ./...` even though `tmp/` is gitignored, so keep Go scratch outside the
  module.

- **Commits**: Conventional Commits prefix (`feat` `fix` `chore` `docs`
  `refactor` `test` `perf` `build` `ci` `revert` `style`), enforced by lefthook's
  `commit-msg` hook and mirrored in CI, plus a Linux-kernel-style body: subject ≤
  72 characters, imperative, no period; body wrapped at 72 explaining *why*. One
  logical change per commit.

- **Never commit to `main` — branch, and rebase before starting.** Every change
  lands via a branch and a PR with green CI. Branch names mirror the commit
  prefix: `feat/<slug>`, `fix/<slug>`, `docs/<slug>`.

  Before starting work, sync with the remote so the branch starts from what is
  actually on main rather than from a stale local copy:

  ```sh
  git fetch origin
  git switch -c feat/<slug> origin/main   # new work
  git rebase origin/main                  # work already in progress
  ```

  This applies to Claude without exception: no commit goes onto `main` directly,
  and a session that begins with uncommitted work already on `main` moves it to a
  branch before committing. Rebase rather than merge, so the branch stays a
  readable series of commits.

  The remote enforces this rather than trusting it: `main` is protected, takes no
  direct or force pushes, and **merges by rebase only** — merge commits and squash
  are both disabled. Every commit therefore lands on `main` exactly as written,
  which is why each one should stand on its own rather than relying on a squash to
  tidy it up later.

- **Do not bypass a hook to land work.** `LEFTHOOK=0` and `LEFTHOOK_EXCLUDE` exist
  for genuine emergencies. A failing hook is the hook working; fix the cause.

- **Documentation lives in `docs/` and some of it is generated.** The site is
  Hugo (`task docs:serve` to preview, `task docs:build` to build — the same
  command CI publishes with, so there is one build path rather than two that
  drift). Prose pages are hand-written; everything under
  `docs/content/docs/reference/` is generated from the Cobra command tree by
  `task docs:gen` and must never be hand-edited. `task docs:check` fails when the
  two disagree, which is what stops a new flag from shipping undocumented.

- **Releases are the maintainer's to publish — Claude prepares, never ships.**
  release-please opens a release PR from the Conventional Commits on main;
  merging it is what tags the version and triggers the build. Claude may write
  commits that feed that PR, but must **not**, without an instruction naming that
  exact action in the moment: merge the release PR, push a `v*` tag, or run the
  release workflow. A published release is something other people download; it
  cannot be cleanly withdrawn. When preparation is done, print the one step the
  maintainer takes and stop.

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
