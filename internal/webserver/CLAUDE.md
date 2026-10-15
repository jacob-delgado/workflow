# internal/webserver

How a request to `workflow --web` becomes a seam call and a problem or a DTO,
for a change to an API operation, the guards or the event stream. The root
`CLAUDE.md` still applies; this adds what holds here alone.

## Spec first

- `api/openapi.yaml` is the source. Change it first, then `task gen` (two
  oapi-codegen configs, `api/oapi-codegen-models.yaml` and
  `api/oapi-codegen-server.yaml`, into `internal/api`) and
  `corepack yarn gen` in `web/`. `task gen:verify` and `task web:gen:check`
  fail when either generated side drifts from the spec.
- `server` implements `api.StrictServerInterface` (`webserver.go`); every
  operation the spec names is a method on it, its own or one of the
  `tasks.Handlers` or the `runs.Runner` it embeds. An operation whose
  `x-switch` extension is set switches the directory (`switchExtension`,
  `validator.go`).

## One file per operation, and when code may leave

The package is the loopback REST surface spelled one file per operation, or
per pair of writes that share their checks, beside the shared guard, faults,
wire shapes, stream and server. It answers to a cohesive ceiling in
`scripts/package-size-budgets.txt`: read the number there, and
`scripts/check-package-size.sh --list` for the count. A new operation is a new
file. The package is not split to chase a number, but code in it with a reason
to change of its own may leave where no import cycle follows: the request
boundary (the guards, the policy headers, the session and the contract
validator), which changes for security's reasons. Three have left already:
the RFC 9457 problem details, to `internal/webserver/problem`, which changes
only with the error format; the Taskwarrior handlers, to
`internal/webserver/tasks`, which change with Taskwarrior; and the git runs
and lefthook's hooks, to `internal/webserver/runs`, which change with git and
lefthook. `tasks.New` builds the Taskwarrior handlers over `tasks.Seams`,
bound with the taskwarrior settings the server started with, and takes what
they need of the server through a `tasks.Kit`: `fault`, the settings in
effect, the clock, the tracker's issue links, `worlds.outliving` and your
home directory. `runs.New` builds the server's `runs.Runner`, which holds the
run going, over `runs.Seams`, and takes the server's `unexpected` and its
`indexWrites` lock, since a run holds the index the server's own writes queue
for. Reaching the ceiling means rewriting the WHY above the entry and a
history row, or such an extraction, never a split for the count.

## A request, outside in

`Handler` (`webserver.go`) wraps, from the outside:

1. `withPolicyHeaders` (`guard.go`): the content policy, `default-src 'self'`
   with no nonce (TRADE-50), and `X-Frame-Options: DENY`.
2. `guardLoopback`: a non-loopback Host, or a cross-origin write, is refused.
3. `refuseWritesInDryRun`: every write is refused under `--dry-run` (TRADE-3).
4. Under `/api/`, the contract validator (`validator.go`), admitting only a
   request that presents the session, then `worlds.serve` (`world.go`). Every
   other path is the embedded app (`static.go`), which answers GET and HEAD
   alone.

The guards in 2 and 3, and the app's refusal of any other method, answer in
plain text; that boundary stays. The validator answers as problem details,
through `problem.Write`: `unauthorized` without the session, `not_found` and
`method_not_allowed` for a route the spec lacks, `bad_request` for a request
that does not match it. The session (`Session`, `guard.go`) rides the
fragment of the address the server prints, which no browser sends to a
server; the page keeps it and presents it where the contract's security
schemes say, a bearer token or, for the event stream, a query parameter.

`worlds.serve` gives each request the server's lifetime; runs every write
under a context that outlasts its page and ends only when the server stops
(`outlivingItsPage`, TRADE-37); holds the write gate shared for every write
but an `x-switch` one, which takes it alone and is refused while a write is in
flight; and refuses, as `conflict`, a write whose `Workflow-Here` header
(`HereHeader`) names another directory than the one served.

## Errors are RFC 9457 problem details

- Every error a handler returns is an `application/problem+json` object —
  `type`, `title`, `status`, `detail`, and a stable machine `code` from the
  spec's enum — built through `problem.New`, `problem.Answer` and
  `problem.Write` (`internal/webserver/problem`), or `fault` (`faults.go`),
  never an ad-hoc envelope.
- **`fault` classifies** a seam's error (a missing resource → `not_found` 404,
  an unreachable upstream → `unreachable` 502) rather than flattening it to a
  generic 500. A new class of failure gets a new class, not a bare 500: one
  every surface tells alike in `internal/report/classes.go`, one only the
  server tells in `faults.go`.
- **The `detail` never leaks a secret or an internal host.** Tokens are
  redacted before an error forms, and an unreachable upstream is genericized
  through `fault` precisely because its error carries the host. A write's
  refusal may still carry git's or the forge's own reason, so the user can act
  (TRADE-9). The test that an unreachable service's detail omits its host is
  part of the change, as `TestUnreachableJiraDetailOmitsItsHost`
  (`issuewrite_test.go`) is.
- `codeMeaning` (`problem/problem.go`) is a map, not a switch, so gobco sees
  no arm it can never reach, and `exhaustive` keeps it complete.
- The `type` URI points at a section of `docs/content/docs/errors.md`
  (`problemBase`, beside it), one `##` heading per code: add the heading with
  the code.
  `errorsdoc_test.go` fails on a code with no heading, or none in
  `scripting.md`'s families.
- The server does not sanitize on the way out: the web app renders its text as
  React text only (`web/CLAUDE.md` says why, and what ends that exemption).

## Deps, lifetimes and boundaries

- `Deps` holds the seam bundles of `internal/seams` whole. A nil function
  means the service is not configured: a list or snapshot read answers empty,
  the issue read and a write whose own seam is nil answer `unprocessable`
  (422), and the announcement and pull request drafts answer `conflict` (409)
  when the branch read or the pull request find is missing (the `Deps` doc
  comment says each case).
- A read others share — the Slack directory, the stream's caches, the
  announcement memory — detaches from its page through `worlds.outliving`
  (`world.go`) and ends with the server. `Serve` drains requests in flight for
  `shutdownGrace`, five seconds, on a stop; the stream re-pushes a snapshot
  every `defaultStreamInterval`, five seconds, unless `Info` sets another.
- depguard (`.golangci.yml`) keeps the direction: this package imports neither
  `internal/tui` nor `internal/wiring` (`webserver-not-terminal`), and only
  `internal/cli/web.go` imports it (`cli-serves-the-web-only-from-web-go`);
  what the command line shares with it lives in `internal/report`.
- `gochecknoglobals` forbids a package-level value, with one exception
  written beside it: the parsed contract (`routedContract`, `validator.go`) is
  a package-level `sync.OnceValues` with a `//nolint:gochecknoglobals` and its
  reason, since every parse of the embedded spec gives the same contract and
  parsing it for each `Handler` cost the race suite minutes.

## Tests: red, green, refactor

- **Red first, through the handler.** A new operation, or a change to one,
  starts as a test that sends the request and asserts the status and body the
  spec promises, or the problem the refusal is; watch it fail, then write the
  method. Tests are black-box, in `package webserver_test` (and
  `package problem_test`, `package runs_test` and `package tasks_test` in
  theirs), marked `// Arrange`, `// Act`, `// Assert`.
- **The harness** (`internal/webserver/webtest`, which only tests import):
  `webtest.Serve`, `ServeWith` and `ServeWorld` build the handler under a
  session `Presenting` sends on every request; `Get` and `Send` make a
  loopback request; `Decode[T]` reads the answer. `FilledDeps()` is every seam
  with a canned answer, so a test nils or replaces only the one it exercises;
  `ErrSeam` fails a seam with no class, which the opaque 500 answers; the
  fixture constants (`TestKey`, `RepoPath`, `LoopbackHost` and the rest) sit
  beside them. `assertProblem` (`problemcheck_test.go`) checks a refusal's
  status, media type and detail.
- **Helpers that cross a line sit in `webtest`.** A helper the tests of code
  that has left or may leave (the Taskwarrior handlers, the git runs, the
  request boundary) share with the tests of the rest is exported from
  `internal/webserver/webtest`, with what it needs, in a file named for what it
  holds: the harness (`harness.go`) and fixtures (`fixtures.go`) above, the
  event stream's reader (`streamclient.go`), a seam call held on its context
  (`heldcall.go`), the configuration's edits (`configedits.go`), the
  Taskwarrior fake (`taskwarriorfake.go`) and lefthook's fixtures
  (`hookfixtures.go`). A package that leaves keeps its own black-box tests,
  which import `webtest` as these do and drive the server through
  `webserver.Handler`, or the package itself through its exported API, as
  `tasks`' `handlers_test.go` and `runs`' `runner_test.go` do. A helper only
  one side's tests use stays in a test file of that side, unless a shared
  helper needs it or it belongs to a grouped declaration that moved whole.
  `webtest` has no tests of its own, as `internal/ptytest` has none: depguard's
  `webtest-only-in-tests` keeps it out of the binary, gobco names it in
  `NO_TESTS`, and its two goroutines are written exceptions in
  `scripts/check-goroutines.sh`. Unlike gobco, `scripts/coverage-gate.sh` does
  not leave it out: its statements count, at 0%, in the total `COVERAGE_MIN`
  holds, as ptytest's do, so the gate prints a total lower than the code's own
  by webtest's share of the statements.
  A helper that fails the test for an Assert stays a test helper of each
  package whose tests call it, as `assertProblem` (`problemcheck_test.go`,
  in the server's tests and in `runs`') does, with only what it reads, as
  `ProblemJSON` (`harness.go`), in `webtest`: `cmd/testshape` reads the
  helpers in a test package's own files alone, so an Assert whose only check
  is a call into `webtest` reaches no failure it can see.
- **What else holds a change.** `contract_test.go` checks the contract refuses
  what the server cannot use; a stream or a cache that waits is tested inside
  `synctest.Test`, on its fake clock (`stream_test.go`,
  `announcequeue_test.go`).
- **End to end.** `task e2e` runs the web's Playwright suites, which
  `task check` leaves out; run it for a change here. Its server half
  (`web/e2e/server`, `scripts/e2e-server.sh`) drives
  `workflow --web --port 13580` from the binary `task build` makes, so nothing
  else may listen on that port.
- **Test files near the ceiling.** A test file fails past 700 lines, and
  `issuewrite_test.go`, `repositories_test.go`, `setup_test.go` and
  `reviewwrites_test.go` are at 600 or more
  (`scripts/check-file-length.sh --list` prints each count): a new case for
  one of them likely starts a sibling file named for its behavior.
