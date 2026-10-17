# scripts

How to write, change and test a gate script, or a data file the gates read,
for any change under `scripts/`. The root `CLAUDE.md` still applies; this adds
what holds here alone. What each gate enforces, and the coverage gates'
mechanics, are written once in CONTRIBUTING.md (*Day to day*, *Tests*) and in
each script's own header.

## Shell style

Follow the
[Google Shell Style Guide](https://google.github.io/styleguide/shellguide.html):
`shfmt -i 2 -ci -bn` formatting, and shellcheck-clean against the repository's
`.shellcheckrc`, which turns on every optional check but SC2312 and follows a
sourced file from the sourcing script's own directory. lefthook formats and
checks a staged script on commit. The same style holds for shell anywhere
else in the repository, `.devcontainer/postCreate.sh` and
`.claude/hooks/session-start.sh` included, which `task lint:shell` finds.

## Every script has a sibling test

Every script here and under `release/` has a sibling `<name>_test.sh` that
drives it: a new script brings its test in the same commit, and a change to a
script changes or adds a case in its test. `check-sibling-tests.sh` fails a
script under `scripts/` that has none. `pr-comment.mjs`'s test is
`pr-comment_test.sh`, `.devcontainer/postCreate.sh`'s is
`postcreate_test.sh`, and `.claude/hooks/session-start.sh`'s is
`session-start_test.sh`; the libraries in `lib/` are driven through their
callers' tests.

- **The harness is `lib/testing.sh`**, sourced after the test's own
  `set -euo pipefail`. It makes a work directory removed on exit
  (`${workdir}`), clears git's repository variables, points every git the
  test and its script start that keeps their environment at a configuration
  of its own, with git's automatic maintenance off (a git run under `env -i`
  reads none of it: `e2e-server_test.sh`'s own gits, which only read, and
  `e2e-server.sh`'s, which read the `home/.gitconfig` it writes, maintenance
  off, and which the runner below leaves to `expect_no_maintenance`), and
  gives the two expectations most cases are:
  `expect_exit <want> <name> <command>...` and
  `expect_output <want> <name> <text> <command>...`, where `<want>` is `pass`,
  `fail` or an exact status; `run_in <dir>` runs a command from a directory,
  `count_case` and `fail_case` count a case a test checks itself, and
  `finish_tests` ends the run with its summary. A test that outgrows its file
  keeps `<name>_test.sh` and moves a behavior's cases to a
  `<name>_<behavior>_test.sh` beside it, the two sharing their fixtures in
  `lib/<name>-fixtures.sh`, as `check-tech-debt-paths.sh`'s do and
  `check-layout.sh`'s, whose `check-layout_runs_test.sh`,
  `check-layout_depguard_test.sh` and `check-layout_yaml_test.sh` share
  `lib/check-layout-fixtures.sh`.
- **`hook-environment_test.sh` is the one runner** (`task test:scripts`): it
  runs every `*_test.sh` and `release/*_test.sh` once under a hook's git
  environment naming a sentinel repository, and fails if the sentinel
  changes, so a new test needs no registration and a test that lets git
  touch the developer's repository fails. It traces every git a test starts
  that keeps the test's environment (`GIT_TRACE2_EVENT`) and fails a test
  whose gits start git's automatic maintenance, which since git 2.47 a
  commit, a fetch or a push's receive-pack leaves running detached, still
  writing into `${workdir}` as the exit trap removes it;
  `git_maintenance_started` reads the trace. A git run under `env -i`, as
  `e2e-server.sh` runs its fixture's, drops the variable, so
  `e2e-server_test.sh`'s `expect_no_maintenance` traces those itself.
- **A gate's seams are its environment.** A gate reads the repository from
  its own location unless a variable points it elsewhere
  (`PACKAGE_SIZE_ROOT`, `PACKAGE_SIZE_BUDGETS`, `DOCS_REFERENCE`, `DOCSGEN`),
  which is how its test hands it a fixture.

## Red, green, refactor here

A change starts as a case in the sibling test that runs the script against a
fixture built for it: a throwaway repository under `${workdir}`, made with
`git init` and the files the case needs (the Arrange), then one `expect_exit`
or `expect_output` line that runs the script and states what it must answer,
an exit status and, for a refusal, the words it names: the Act and the Assert
in one. No gate reads markers in a shell test, and none marks `# Arrange`:
`markdown-changed_test.sh` and `test-counts_test.sh` mark such a line
`# Act & Assert`; `go-test-json_test.sh`, `pr-comment_test.sh` and
`test-summary_test.sh` mark each run `# Act: …` and each check `# Assert`,
most saying after it what they check; the rest mark nothing. Run it with
`bash scripts/<name>_test.sh`, watch the new case fail, then change the
script. `expect_output` lets a refusal's case hold the words it names as well
as its status, so a gate that fails for the wrong reason fails the test.

## What the gates hold to

- **A gate measures tracked files only**, from `git ls-files`, so a scratch
  file cannot fail it and a new file counts once `git add`ed; and it refuses
  to pass having measured nothing when git lists nothing (the
  `check-file-length.sh`, `check-goroutines.sh` and `check-layout.sh` headers
  say why).
- **`check-layout.sh` and `coverage-gate.sh` read the root `CLAUDE.md`'s
  `Layout:` block**, through `lib/layout.sh`: the first holds it to the Go
  packages, and the second leaves each package whose line ends
  `(tests only)` out of the statement total. So the block stays in the root,
  and both gates' tests (`check-layout.sh`'s, through
  `lib/check-layout-fixtures.sh`, and `coverage-gate_test.sh`) build fixture
  `CLAUDE.md` files; moving the block, or changing the mark, means changing
  the library and both.
- **A `(tests only)` mark must be the build's word.** Since the mark takes a
  package out of the statement total, `check-layout.sh` fails a marked line
  unless a rule under `linters.settings.depguard.rules` in `.golangci.yml`
  denies the package's import path (go.mod's module, then the directory) to
  every file but a test: its files are `"!$test"`, alone or beside `"$all"`,
  the shape of the `*-only-in-tests` rules, or besides it leave out only a
  `"!**/<dir>/**"` naming a package held to tests in turn, as is every package
  under a directory of that name at any depth, which the pattern leaves out
  too (a test helper may import another), and its `list-mode` is unset or
  `original`, since under `lax` or `strict` an allow item longer than the deny
  item lets the import through. A rule holds only while depguard runs, so the
  check also fails a mark when `.golangci.yml` does not make sure it does:
  `linters.disable` lists depguard, `linters.default` is not `all` and
  `linters.enable` does not list it (`standard`, `none` and `fast` each need
  it there), `linters` holds a key besides `default`, `enable`, `disable`,
  `settings` and `exclusions`, or its `default`, `enable` or `disable`, or an
  item of either list, is a block scalar, which the check does not read.
  Exclusion rules and `//nolint:depguard` stay unread, and so do `issues.new`
  and its `new-from-*` kin, which hide depguard's findings as they do, and
  `run.issues-exit-code`, which set to 0 lets golangci-lint pass with them
  printed.
- **The check reads `.golangci.yml` as golangci-lint does, or not at all.**
  It exits 2 beside a `.golangci.json`, `.golangci.toml` or `.golangci.yaml`,
  tracked or not, each of which golangci-lint reads in its place. It reads
  the first YAML document's block style a line at a time, each key lowered
  and split at its dots as golangci-lint's configuration library does, so
  two keys that then read alike leave their setting unread; it passes over a
  block scalar's text, so a rule's files or pkg written as one hold nothing;
  and it exits 2 naming the first line it cannot read so (an alias, a tag, a
  merge or explicit key, an escape, a value running on past its line). Its
  tests build a fixture `.golangci.yml` and `go.mod`: `check-layout_test.sh`
  holds the mark's need of a rule, and of the file golangci-lint reads,
  `check-layout_runs_test.sh` its need of a depguard that runs,
  `check-layout_depguard_test.sh` which rules hold, and
  `check-layout_yaml_test.sh` which YAML the check reads and which it
  refuses; a change to what it reads changes the check and that test.
- **`release/`** belongs to the maintainer's release path, and the root's
  release rule covers it. `release-please.yml` runs `push-release-tag.sh`
  twice, as `decide` in its gate job and again in its tag job to push the
  tag; `packages.yml` runs `package-manifests.sh` and `publish-packages.sh`;
  and `ci.yml`'s Cross-compile job runs `package-manifests.sh` and
  `check-formula.sh` on every pull request. `release.yml` runs none of them;
  the tasks it runs are `task check`, `task release:binaries` and
  `task release:verify`.

## Data files and their reasons

- `package-size-budgets.txt`: every entry has its WHY on the lines directly
  above it, and `check-package-size.sh` fails one without. A change to a
  number, a ratchet as much as a bump, appends a row to
  `package-size-budget-history.md`; an earlier row is never rewritten.
- `gobco-allowlist.txt`: each line keeps one arm for the `TRADE-n` it names,
  which `check-tradeoffs.sh` holds to the register; a line whose arm a test
  comes to reach fails until it goes. `gobco-floors.txt` holds a package to a
  condition floor of its own, raised only once the coverage is there, to
  floor(measured) − 2.
- `dependency-age-exceptions.txt`: a version younger than a week, with the
  CVE or GHSA it fixes or the maintainer's dated waiver; an npm version also
  needs its line in `web/.yarnrc.yml`. Each line goes once its version is a
  week old; the age gate warns of a waiver still listed a week after its day,
  as an annotation on the pull request in CI.
