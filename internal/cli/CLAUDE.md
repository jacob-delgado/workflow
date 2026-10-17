# internal/cli

The scriptable contract and the output plumbing every command keeps, for a
change to a command, a flag, what a command prints or how it exits. The root
`CLAUDE.md` still applies; this adds what holds here alone. UX.md's Principles
and Vocabulary are binding here: an action keeps one name through its flow, and
a new word comes from that table.

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
doctor's, status's, pr's and announce's did. It answers to a cohesive ceiling
in `scripts/package-size-budgets.txt`: read the number there, and
`scripts/check-package-size.sh --list` for the count, which is at the ceiling
as this is written, so the next command file is a decision. The package is not
split to chase a number, but code in it with a reason to change of its own may
leave where no import cycle follows: doctor's checks, and the pr and announce
flow over the forge and messaging. Reaching the ceiling means rewriting the
WHY above the entry and a history row, or such an extraction, never a split
for the count.

## What a command prints

- **stdout is the artifact**, and nothing else. It is written through
  `checkedOutput` (`scriptable.go`), checked once where `prepare` (`cli.go`)
  sets it up, so an artifact that went nowhere fails the command (exit 1)
  while each `fmt.Fprint` is left unchecked (`.golangci.yml`'s errcheck note).
- **stderr is `console.Notes`** (`console/console.go`): a note is the bare
  sentence, a warning and an error carry their tags, and `--log-format` says
  them as JSON or logfmt. A command writes stderr through it, never around
  it.
- **A wait with a person watching draws a live line** at the foot of stderr
  (`Notes.Watched`, `console/live.go`); everything said meanwhile prints above
  it, and when stdout is the same terminal the artifact does too
  (`artifactBeside`, `prompt.go`, TRADE-43). Piped, the artifact goes to
  stdout untouched. The line follows the configuration the terminal interface
  draws by, through `console.Look` (`liveLook`, `prompt.go`): `ui.ascii`, or
  `status --plain`, draws it in ASCII, and `ui.motion: reduce` holds its mark
  still on the in-flight glyph; its mark's frames are `progress.Turn`'s, the
  terminal's own.
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
for a client's unreachable service, `servicesUnreachable` in
`scriptable.go`) and `scripting.md`'s table, in the same commit.

## The process, signals and the quit

- **A command reads the process only through `cli.Environment`**
  (`environment.go`): the working directory, home, variables, store directory
  and programs. `Execute`'s caller builds it from the operating system, so a
  test hands each run its own.
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
  that says which stream a line is on; `runAtHome` (`cli_test.go`) makes
  `dir` the home too; `clitest.RunGuided` answers prompts through `scripted`
  (`cli_test.go`). `clitest.UnusedPrompt` fails a test whose command reads a
  prompt it was not meant to. What a run sees of the outside world is its
  test's own (`clitest.ProgramsOf`, `clitest.SetVariable`). A fixture the
  tests of more than one package of the tree need lives there; one only this
  package's tests need stays in its own `_test.go` files.
- **Fakes for the services**: `forgefake_test.go` (a fake `gh` and its pull
  requests), `trackerfake_test.go` and `slackfake_test.go` stand in for the
  forge, Jira and Slack, and `clitest.WorkingJira` is a Jira that accepts
  the credential; `runRoot` (`root_test.go`) runs the root command over a
  stand-in interface and web server.
- **`TestMain` (`main_test.go`) clears git's repository variables** before
  any test runs (`gittest.ClearLocalVariables`), so a hook's `GIT_DIR` never
  sends a test's git at the developer's repository; it then runs the tests
  through `clitest.Main`, which keeps every program a test starts out of the
  developer's home and git configuration and every task program off PATH.
  `TestTheTestsLeaveTheRepositoryAHookNamesAlone` proves the scrub; a new
  package whose tests run git needs the same (CONTRIBUTING.md, *Tests*).
- **A terminal** is a pseudo-terminal from `internal/ptytest`, as
  `help_terminal_test.go` and `prompt_terminal_test.go` use it.
- **Test files near the ceiling.** A test file fails past 700 lines, and
  `root_test.go` is at 600 or more (`scripts/check-file-length.sh --list`
  prints its count): a new case for it likely starts a sibling file named for
  its behavior.
