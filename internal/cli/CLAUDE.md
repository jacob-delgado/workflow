# internal/cli

The scriptable contract and the output plumbing every command keeps, for a
change to a command, a flag, what a command prints or how it exits. The root
`CLAUDE.md` still applies; this adds what holds here alone.

## A published contract

- **`docs/content/docs/scripting.md` is the contract a script relies on**:
  the exit statuses, which stream carries what, the JSON shapes, and the flags
  that make a write safe unattended. Changing any of it is a breaking change:
  mark the commit `!` with a `BREAKING CHANGE:` footer, and change the page in
  the same commit. `scripting_docs_test.go` holds the page to the command
  tree.
- **A command or a flag added, removed or renamed runs `task docs:gen`**, which
  rewrites the generated reference under `docs/content/docs/reference/`;
  `task docs:check` fails when the two disagree.
- Shared output shapes — the Summary, the repositories, the review queue, a
  failure's words — come from `internal/report`, which the web server reads
  too, never from the server: only `web.go` imports `internal/webserver`, as
  depguard's `cli-serves-the-web-only-from-web-go` rule holds.

## One file per command, and when code may leave

The package is the Cobra command tree spelled one file per command, per family
of commands over one seam, or per section of a command that outgrew one, as
doctor's and status's did. It answers to a ratcheted budget in
`scripts/package-size-budgets.txt`, the count its last split left it at: read
the number there, and `scripts/check-package-size.sh --list` for the count,
which a ratcheted budget always equals, so the next command file trips the
gate and is a decision. The package is not split to chase a number, but code
in it with a reason to change of its own may leave where no import cycle
follows: doctor's checks, and status and issues. A trip means a bump, with
the WHY above the entry rewritten and a history row, or such an extraction,
never a split for the count; each extraction ratchets the budget down to the
files left, and the entry goes once the package holds no more than the
default allows.

The pr and announce flow over the forge and messaging has left as
`internal/cli/pr`, one package for both, since `pr --announce` hands what it
opened to announce's wait and `pr --json`'s report carries fields only the two
read. It exports the two commands `cli.go` registers (`pr.NewCmd`,
`pr.NewAnnounceCmd`) and the sentinels the exit families name. Its tests,
`package pr_test`, drive the commands through `cli.Execute` and clitest as
this package's do, with a `TestMain` and hook-safety proof of their own, and
reach every condition it holds, since the condition gate measures a package
by its own tests alone.

What every command is built from has left already, as `internal/cli/cmdkit`
beneath the tree: the connection to where a command runs, the environment it
reads the process through, the flags every command shares, the prompt and the
scriptable output. It imports no command, and not the web server
(`.golangci.yml`'s `cmdkit-without-the-web-server`), so `cli.go` hands
`RootFlags.Declare` the port `--web` serves on by default. The exit statuses
and the families of errors that earn them stay at the root (`exitstatus.go`),
since the families name sentinels the commands declare, `internal/cli/pr`'s
among them, and so do the two terminal reads `cmd/workflow` wires
(`LineReader` and `SecretReader`, `prompt.go`).

## What a command prints

- **stdout is the artifact**, and nothing else. It is written through
  `cmdkit.CheckedOutput` (`cmdkit/scriptable.go`), checked once where
  `prepare` (`cli.go`) sets it up, so an artifact that went nowhere fails the
  command (exit 1) while each `fmt.Fprint` is left unchecked
  (`.golangci.yml`'s errcheck note).
- **stderr is `console.Notes`** (`console/console.go`): a note is the bare
  sentence, a warning and an error carry their tags, and `--log-format` says
  them as JSON or logfmt. A command writes stderr through it, never around
  it.
- **A wait with a person watching draws a live line** at the foot of stderr
  (`Notes.Watched`, `console/live.go`); everything said meanwhile prints above
  it, and when stdout is the same terminal the artifact does too
  (`artifactBeside`, `cmdkit/prompt.go`, TRADE-43). Piped, the artifact goes to
  stdout untouched.
- **fang renders the help, the usage and the errors** (`execute`, `cli.go`,
  TRADE-44), in the terminal's own sixteen colors (`ansiScheme`).

## How a command exits

`ExitStatus` (`exitstatus.go`) is 0 for none, 130 when interrupted (checked
first), 1 when the output could not be written, and otherwise the first
family the error is in, in order: usage 2, configuration 3, a refused
precondition 4, unreachable 5; any other failure is 1. `interrupted`
(`cli.go`) puts the context's error back on a child's failure, so a Ctrl+C
still reads as 130. A new sentinel a script should tell apart joins its
family's list (`exitFamilies`, `configurationErrors`, `refusalErrors`, or,
for a client's unreachable service, `cmdkit.ServicesUnreachable` in
`cmdkit/scriptable.go`) and `scripting.md`'s table, in the same commit.

## The process, signals and the quit

- **A command reads the process only through `cmdkit.Environment`**
  (`cmdkit/environment.go`): the working directory, home, variables, store
  directory and programs. `Execute`'s caller builds it from the operating
  system, so a test hands each run its own.
- `Execute`'s doc comment (`cli.go`) is the reference for signals: SIGINT and
  SIGTERM cancel every command's context; after the interface quits, workflow
  waits up to `timing.request_timeout` for the writes it left in flight
  (`finishwrites.go`), and names what detaches from its asker and why.

## Tests: red, green, refactor

- **Red first, through `cli.Execute`.** A change starts as a test that runs
  the command line a user or a script would type and asserts the stdout, the
  stderr and the exit status the contract promises; watch it fail, then make
  the change. Tests are black-box, in `package cli_test`, marked
  `// Arrange`, `// Act`, `// Assert` as the root asks.
- **The runners** (`internal/cli/clitest`, which only tests import):
  `clitest.Run(t, dir, args...)` executes in `dir` with an empty home of its
  own and none of the developer's environment, and returns everything
  printed; `clitest.RunStreams` keeps stdout and stderr apart, for a test
  that says which stream a line is on; `clitest.RunAtHome` makes `dir` the
  home too; `clitest.RunGuided` answers prompts through
  `clitest.Scripted` (`clitest/prompt.go`). `clitest.UnusedPrompt` fails a
  test whose command reads a prompt it was not meant to. What a run sees of
  the outside world is its test's own (`clitest.ProgramsOf`,
  `clitest.SetVariable`). A fixture the tests of more than one package of the
  tree need lives there; one only this package's tests need stays in its own
  `_test.go` files. `clitest` has no tests of its own, as `internal/ptytest`
  has none: depguard's `clitest-only-in-tests` keeps it out of the binary,
  gobco names it in `NO_TESTS` (`scripts/gobco-report.sh`), and
  `scripts/coverage-gate.sh` leaves it out of the statement total
  `COVERAGE_MIN` holds, as it leaves every package the root `CLAUDE.md`'s
  layout marks `(tests only)`.
- **Fakes for the services**: `clitest.FakeGh` and `clitest.FakeGlab`
  (`clitest/forge.go`, a fake `gh` or `glab` and its pull requests),
  `trackerfake_test.go` and `clitest.FakeSlack` (`clitest/messaging.go`)
  stand in for the forge, Jira and Slack, and `clitest.WorkingJira` is a Jira
  that accepts the credential. The repositories they answer for are made with
  git itself (`clitest.PRRepo`, `clitest.GithubRepo` and their kin,
  `clitest/repo.go`), which `.golangci.yml`'s `gittest-only-in-tests` lets
  clitest alone do outside a test file; `runRoot` (`root_test.go`) runs the
  root command over a stand-in interface and web server.
- **`TestMain` (`main_test.go`) clears git's repository variables** before
  any test runs (`gittest.ClearLocalVariables`), so a hook's `GIT_DIR` never
  sends a test's git at the developer's repository; it then runs the tests
  through `clitest.Main`, which keeps every program a test starts out of the
  developer's home and git configuration and every task program off PATH.
  `TestTheTestsLeaveTheRepositoryAHookNamesAlone` proves the scrub; a new
  package whose tests run git needs the same (CONTRIBUTING.md, *Tests*).
- **The kit's own tests live beside it**, in `internal/cli/cmdkit`: through
  `cli.Execute` where a command is how a script meets the kit, and otherwise
  over a small tree of their own (`probeTree`, `kit_test.go`) declared as the
  root declares workflow's. Its `TestMain` clears git's variables as this
  package's does, with a proof of its own. The kit's tests reach each of its
  conditions both ways, since the condition gate measures a package by its
  own tests alone, but for the two Windows clauses of `relativeToWorkingDir`
  (`cmdkit/environment.go`) that TRADE-35 keeps: only
  `cmdkit/environment_windows_test.go` sees them false, and only a Windows
  `go test` runs it. gobco lists the two as seen one way, which fails
  nothing: the gate fails a condition no test evaluated and an error check
  whose error arm none reached, and these are neither.
- **A terminal** is a pseudo-terminal from `internal/ptytest`, as
  `help_terminal_test.go` and `prompt_terminal_test.go` use it.
- **Test files near the ceiling.** A test file fails past 700 lines, and
  `root_test.go` is at 600 or more (`scripts/check-file-length.sh --list`
  prints its count): a new case for it likely starts a sibling file named for
  its behavior.
