# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-64". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts. An earlier edition of this file was retired once
every entry in it was done; this one carries the three entries that outlived
the debt paydown and the findings of a full read of every surface, and its
numbering continues where the earlier edition stopped, so an ID is never
reused.

Checked against commit `f05ae9f` on 2026-09-24 (main after the debt
paydown, PRs #134 and #136, and a Dependabot bump); an entry a later change
touched was checked again in that change. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **Every surface and every package, read from source.** The command
   line (`internal/cli`, `cmd/`), the terminal interface (`internal/tui`)
   and the web (`web/src`, `internal/webserver`, `api/openapi.yaml`), and
   every package under `internal/` beside them, were read in full against
   the standard the project sets for itself in [CLAUDE.md](CLAUDE.md), the
   clig.dev guidelines, the promises the interface makes in
   [UX.md](UX.md) and the web's accessibility floor, with each unit's tests
   read beside it, so a test whose Assert would pass whatever the Act did
   counts as a finding too.
2. **The web, read from the screen.** The mock build was run under
   Playwright and every section screenshotted in both themes at 640, 1024
   and 1440 px, with the production build's empty and no-API states beside
   them, so the web's look was judged from what it draws rather than from
   the source alone.
3. **Measurement, once, at this commit.** The condition-coverage figures
   come from `task cover:branch`; the file lengths from
   `scripts/check-file-length.sh --list`; the budget standings from
   `scripts/check-package-size.sh --list`; the line counts from `task
   cloc`; the web's numbers from the v8 summary and `knip`; the Go side's
   from `deadcode`; and the docs drift from `task docs:check`. An entry
   that cites a number cites the tool it came from.
4. **Every finding refuted before it was written.** Each was handed to an
   independent reader to disprove, and to a second one when rated medium or
   high; a claim that could not be pointed at a line was dropped. Every
   cited line was read again as its entry was written.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). This edition is read and measured.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it appears under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost), which lists
the choices that were made on purpose and written down, and what they cost.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule. This pass found a few; they were written to a private file for the
maintainer, not here, and nothing in this file says what they are. The
entries below contain nothing that helps anyone misuse a credential, a
terminal, a file on disk, the loopback server or a release.

## The command line

What is open here is `doctor`'s two reports and their exit families, the
clients and plumbing every command shares (the interface and the web reach
them too), the drift between the docs, the comments and the code they
describe, and three sweeps that cross every surface — tests that prove
nothing, facts written twice and arms no input reaches.

### DEBT-71 `wiring` returns `tui.Deps`, though three surfaces consume the seams

Severity: low · Confidence: read

`wiring.Deps` (`internal/wiring/wiring.go:72`) returns `tui.Deps`, so the
wiring package imports the terminal interface; the CLI's `WebDeps`
(`internal/cli/cli.go:284`) then narrows that bundle for the web server.
The seams are not the terminal's — they are the loop's. That import no
longer stands in the way of shared composition: `internal/loop` takes each
seam as a plain argument (`loop.PullSeams`, `loop.AnnounceSeams`) and never
needed `wiring`. What is left is narrower: a seam only the CLI or the web
needs must still be declared on `tui.Deps`, as a `RecentCommits` for
`standup` would be — which is why `standup` reads its commits from the
repository directly instead (`internal/cli/standup.go:98`).

**One way to fix it.** Move the bundles that depend only on leaf types —
`JiraDeps`, `GitDeps`, `ForgeDeps`, `MessagingDeps`, `HookDeps` — to a
package all three surfaces import, with type aliases left in `tui`.
`StoreDeps` carries `tui.AnnouncedPost`, so it could move only with that
type, and `EditorDeps` carries Bubble Tea's `tea.Msg` and `tea.Cmd`, so
`wiring` would still return `tui.Deps`.

**Done when.** The seam bundles are declared where all three surfaces can
import them without importing each other. Deferred — YAGNI until a seam the
terminal does not use has to be added to `tui.Deps`.

### DEBT-75 `doctor --online` ignores `forge.cli` and asks the forge over HTTP

Severity: medium · Confidence: read

`docs/content/docs/scripting.md:53` promises, under `forge.cli`, that
"`workflow doctor --online` says which it is" — `gh` or `glab`, or HTTP.
`doctor` never reads `config.Forge.CLI`: `checkForge`
(`internal/cli/doctor.go:222`) always resolves a token through
`wiring.ForgeResolver` and fails with `errCredentialRejected` when none
resolves; `askForge` (`internal/cli/doctor.go:250`) calls `forge.New(doer,
base, token).Whoami` over the doer that `onlineDoers`
(`internal/cli/doctor.go:165`) builds from `httpx` alone; and `doctor --json
--online` shares the same `checkForge` through `credentialFacts`
(`internal/cli/doctor_json.go:201`), so the JSON verdict is wrong too. The
commands' own path, `connectForge` (`internal/wiring/forge.go:244`),
tolerates a missing token when the CLI is in use (`if err != nil &&
!usingCLI`); `doctor` has no such branch.

Behind the SSO gateway `forge.cli` exists for, every command works through
`gh` or `glab` while `doctor --online` reports the token as missing (exit 3)
or the forge as unreachable (exit 5) and sends the user to fix a credential
the commands do not need. No doctor test sets `forge.cli`.

**One way to fix it.** Have `checkForge` build its doer through `wiring`'s
forge transport (exported or wrapped), tolerate a missing token when the CLI
is in use as `connectForge` does, and report "through gh" or "through glab"
as the credential's source. This leans on `forge.cli` asking the forge
through the CLI, which `FEATURES.md`'s settled line "gh is an optional
source of a token and nothing more" does not admit; that line is the stale
side — commit 015a5f4, which added `forgeTransport`
(`internal/wiring/forgecli.go:25`), says it reopens the net/http rule
deliberately and opt-in — so the maintainer refreshes the settled line
rather than this fix.

**Done when.** A doctor test with `forge.cli` true, no token in the
environment or the file, and a fake `gh` on PATH exits 0 and names `gh` as
the source; the same with a GitLab remote and a fake `glab`.

### DEBT-76 The forge-issues mode is advertised, then `doctor` and the docs deny it

Severity: medium · Confidence: read

The README's line 39 advertises running with no Jira: "With no Jira
configured, the Issues pane lists the issues assigned to you on your forge".
The mode is real — `trackerDeps` (`internal/wiring/forgeissues.go:42`) swaps
in the forge's issues when `Jira.Configured`
(`internal/config/config.go:220`) is false, and that method's own comment
says so — but the configuration still reads as incomplete on three surfaces,
and two documents disagree about whether Jira is required:

- `internal/config/config.go:380` — `Config.Missing` has no
  `Jira.Configured()` branch; `jira.base_url` and `jira.token` are listed
  whenever they are empty.
- `internal/cli/doctor_requirements.go:43` — `reportRequirements` turns any
  missing field into `errIncomplete`, which `configurationErrors`
  (`internal/cli/scriptable.go:325`) maps to exit 3.
- `internal/cli/doctor_json.go:177` — `configurationFacts` raises the same
  `errIncomplete` in the JSON report.
- `internal/tui/render.go:450` — the configuration screen's `status` reads
  `m.cfg.Missing()` and repeats the list as incomplete.
- `docs/content/docs/configuration.md:60` — the fields table marks
  `jira.base_url` "Required: yes", contradicting the README's line 39.
- `docs/content/docs/usage.md:166` — "Pick up an issue" describes the Issues
  pane in Jira terms only; the fallback is never mentioned.

A GitHub-only user follows the README, runs the recommended `workflow
doctor`, and is told two Jira fields are missing with exit 3; a script
gating on doctor's exit fails in this mode. The usage page never says what
changes when the forge is the tracker: `IssueKey`'s comment
(`internal/convention/convention.go:127`) explains that such a project names
its branches by a number rather than a key, and no page does.

**One way to fix it.** Have `Missing` skip the Jira fields when
`Jira.Configured()` is false and a forge remote exists, have `doctor` say
which tracker is in effect, qualify the `jira.base_url` row ("required for
Jira as the tracker; leave empty to use the forge's issues"), and add a
paragraph under "Pick up an issue" saying what the pane shows and which
Issues keys do not apply.

**Done when.** A doctor test with only a messaging block and a GitHub remote
exits 0 and names the forge as the tracker; the `jira.base_url` row and the
README's line 39 state the same rule; and `grep -n forge
docs/content/docs/usage.md` prints a line number that falls between the
"Pick up an issue" heading and the next heading.

### DEBT-77 `doctor --json` exits 0 on a configuration the prose report fails

Severity: high · Confidence: read

`doctor` gathers its facts twice, once per report, and the configuration
pair has diverged. `runDoctorJSON`'s comment
(`internal/cli/doctor_json.go:77`) promises "the same aggregate error the
prose report would", and `docs/content/docs/scripting.md:155` says "It exits
as the prose report does"; neither holds for a set-but-invalid value:

- `internal/cli/doctor_requirements.go:23` — `reportRequirements` appends
  `cfg.Problems()` and `forgeKindProblem(cfg.Forge)` to the problems it
  counts as `errInvalid`.
- `internal/cli/doctor_requirements.go:25` — `reportRequirements` also
  counts `tui.CheckKeys(cfg.UI.Keys)` as a problem.
- `internal/cli/doctor_json.go:156` — `configurationFacts` reads only
  `cfg.Missing()` and `config.SharedMode`; it never asks `cfg.Problems()`,
  `forgeKindProblem` or `tui.CheckKeys`.
- `internal/cli/doctor_json.go:176` — `configurationFacts` joins only
  `errShared` and `errIncomplete`, so `errInvalid` is never produced and
  exit 3 never reached for an invalid value.
- `docs/content/docs/scripting.md:151` — the `configuration` field list has
  no `problems` entry a script could inspect instead.
- `internal/cli/doctor_test.go:422` — `TestDoctorReportsSetButInvalidValues`
  drives the invalid-values fixture (`:427`) through the prose report only;
  it has no `--json` twin.
- `internal/cli/doctor_requirements.go:120` — `reportTooling` ranges
  `externalTools()`, builds the missing list and the `errMissingTooling`
  error.
- `internal/cli/doctor_json.go:136` — `toolingFacts` ranges
  `externalTools()` again and builds the same list and the same error; it
  has simply not diverged yet.
- `internal/cli/doctor_json.go:183` — `credentialFacts`'s comment gives the
  reason the credential checks are shared ("what keeps the two reports from
  ever masking differently"); the reasoning is not applied to configuration
  or tooling.

A file with an `ftp://` base URL, an `http` webhook, a `forge.kind` of
`githb` or a conflicting `ui.keys` map exits 3 under `doctor` and 0 under
`doctor --json`. A script keyed on the exit status, the documented way,
reads an invalid configuration as healthy, and the JSON carries no
`problems` field. Nothing on the load path closes the gap: `tui.CheckKeys`
runs only in `openInterface` (`internal/cli/cli.go:234`) and in the prose
report.

**One way to fix it.** Gather `Missing`, `Problems`, the forge-kind problem
and the key problems once into a facts value that both `reportRequirements`
and `configurationFacts` render, adding a `problems` array to `configFacts`
and documenting it at `docs/content/docs/scripting.md:151`; do the same for
tooling, so `externalTools()` is ranged over in one function and
`reportTooling` renders `toolingFacts()`'s result.

**Done when.** A test runs `doctor --json` over the fixture
`TestDoctorReportsSetButInvalidValues` writes
(`internal/cli/doctor_test.go:427`) and gets exit 3 with
`configuration.problems` naming `jira.base_url`, `messaging.webhook_url` and
`forge.kind`; `externalTools()` is ranged over in one function.

### DEBT-78 `doctor --json --online` checks credentials over a configuration that did not load

Severity: medium · Confidence: read

The prose `runDoctor` returns before `reportCredentials` when `run.loadErr`
is set (`internal/cli/doctor.go:117`); the JSON report does not mirror it:

- `internal/cli/doctor_json.go:92` — `runDoctorJSON` calls `credentialFacts`
  with no `run.loadErr` guard.
- `internal/cli/doctor_json.go:186` — `credentialFacts` guards only on
  `!run.online`, then runs `checkJira`, `checkMessaging` and `checkForge`
  over `run.cfg` whatever `loadErr` says.
- `internal/config/load.go:120` — `LoadFile` returns `Default()` on a failed
  open, so those checks run over a configuration that is not the user's.
- `internal/cli/doctor.go:222` — `checkForge` runs the forge resolver (an
  environment lookup, and `gh` where it is installed, as
  `noForgeTokenMessage`'s comment at `:232` says) and `askForge` then calls
  `Whoami`, all over the defaults.
- `internal/cli/doctor_json.go:100` — `runDoctorJSON` then drops `credErr`
  from the joined error when `loadErr` is set, so the results it printed
  affect nothing.
- `internal/cli/doctor_json_test.go:81` —
  `TestDoctorJSONReportsAMissingFileAsData`, the only missing-file `--json`
  test, omits `--online`;
  `TestDoctorJSONOnlineNamesTheIdentityWithoutTheToken` (`:92`) uses a valid
  file.

The two reports disagree on whether credentials were checked: the JSON says
`credentials.checked` is true and lists results. It makes network and
subprocess calls the prose report would not make on a broken file, and its
results change nothing.

**One way to fix it.** Return `credentialsFacts{Checked: false}` from
`credentialFacts` when `run.loadErr` is set, mirroring `runDoctor`'s early
return.

**Done when.** A test writes `{not json`, runs `doctor --json --online` with
a forge fake that records whether it was reached, and reads
`credentials.checked == false` with the fake never reached. `Default()` has
an empty Jira base URL, so `checked == false` is the assertion that carries
weight.

### DEBT-79 `doctor --json` calls a missing credential "rejected" and an unchecked webhook "ok"

Severity: medium · Confidence: read

`credentialStatus` (`internal/cli/doctor_json.go:233`) has three arms — nil
is `ok`, `errUnreachable` is `unreachable`, anything else is `rejected` —
and its comment (`:231`) defines `ok` as a working credential. Two outcomes
are misnamed in the machine field a script keys on:

- `internal/cli/doctor.go:300` — `credentialOutcome` wraps every error that
  is not the unreachable sentinel as `errCredentialRejected` (`:305`),
  `jira.ErrNoCredential` and `messaging.ErrNoCredential` included.
- `internal/wiring/token.go:38` — `ResolveToken` resolves an unset token to
  `""` with a nil error, so the check goes on to the service and is refused
  there.
- `internal/jira/jira.go:113` — `Client.newRequest` returns
  `ErrNoCredential` when no auth mode is configured: the route a missing
  `jira.token` takes to "rejected".
- `internal/messaging/messaging.go:121` — `checkable` returns
  `ErrNoCredential` for `MessagingNone`: the same route for a missing
  messaging credential.
- `internal/cli/doctor.go:314` — `checkJira` wraps a `token_command` that
  failed in `errCredentialRejected`, though nothing was rejected.
- `internal/cli/doctor.go:273` — `checkMessaging` does the same for a
  messaging `token_command`.
- `internal/cli/doctor.go:211` — `checkForge` wraps a `forge.kind` naming no
  forge in `errCredentialRejected`, and (`:226`) no forge token resolved.
- `internal/cli/doctor.go:38` — `errCredentialRejected`'s comment, "a
  credential a service would not accept", no longer covers a missing
  credential, a failed `token_command` or a bad `forge.kind`.
- `internal/cli/doctor.go:286` — `checkMessaging` returns nil for
  `ErrWebhookUncheckable`, which `credentialStatus` turns into `ok`.
- `docs/content/docs/scripting.md:154` — `status` is documented as `ok`,
  `rejected` or `unreachable`; there is no value for missing or unchecked.
- `internal/cli/online_test.go:151` —
  `TestDoctorOnlineSaysAWebhookCannotBeChecked` reads the prose only; no
  `--json` case reads the status.

The exit family is right either way (3 for a missing credential, 0 for the
uncheckable webhook), but `status` cannot tell "rotate the token" from
"there is no token" or "fix `forge.kind`", the last stderr line contradicts
the report line above it ("no jira.token is configured", then "a credential
was rejected: jira"), and a script trusts a webhook `doctor` never tried.

**One way to fix it.** Wrap resolver and kind failures in the services' own
sentinels (`jira.ErrNoCredential`, `messaging.ErrNoCredential`,
`forge.ErrNoToken`, `forge.ErrKindNeedsHost`, all already in
`configurationErrors`), let `credentialOutcome` keep them distinct from a
rejection, and give `credentialStatus` a `missing` arm and an `unchecked`
arm (the latter for `messaging.ErrWebhookUncheckable`, keeping the nil
aggregate so the exit stays 0); document both at
`docs/content/docs/scripting.md:154`.

**Done when.** A test runs `doctor --json --online` with no `jira.token` and
reads `status: "missing"` for jira with no "rejected" in the error text;
`TestDoctorOnlineSaysAWebhookCannotBeChecked` gains a `--json` case that
reads `status: "unchecked"` and exit 0.

### DEBT-80 `doctor` says nothing about a configuration it cannot open

Severity: low · Confidence: measured

`reportLoadError` (`internal/cli/doctor.go:435`) switches on
`config.ErrNotFound` and `config.ErrInvalid` only, and returns `loadErr`
(`:445`) with no Configuration line written when neither matches. `LoadFile`
(`internal/config/load.go:120`) wraps a failed `os.Open` as "opening %s: %w"
with neither sentinel, so on a file that exists but cannot be read (mode 0)
the prose report has Version, Repository and Tooling and then no
Configuration section at all, with the reason only in the bare error `main`
prints last. The gobco report (`task cover:branch`) confirms the
fall-through has never run under test: the `errors.Is(loadErr,
config.ErrInvalid)` condition in `reportLoadError`
(`internal/cli/doctor.go:440`) was once true but never false.
`TestACommandRefusesAConfigurationItCannotOpen`
(`internal/cli/exitstatus_test.go:265`) makes the mode-0 file for `status`,
not `doctor`.

`doctor`'s whole purpose is to say what is wrong; on an unreadable file it
stops short.

**One way to fix it.** Add a default arm that writes a `Configuration` line
saying the file cannot be read, followed by the error, as the `ErrInvalid`
arm prints its own.

**Done when.** A test chmods the file to 0, runs `doctor`, and finds a
`Configuration:` line saying it cannot be read, with exit 1; `task
cover:branch` no longer lists `internal/cli/doctor.go:440` as one-sided.

### DEBT-81 `cmd/workflow/main.go` is not the thin main CLAUDE.md describes

Severity: low · Confidence: read

`CLAUDE.md:27` describes `cmd/workflow/` as "thin main; wires cli.Execute
and the exit status". The file is 128 lines and carries three
implementations no test reaches, plus user-facing copy:

- `cmd/workflow/main.go:28` — `standupHelp`, the copy the standup editor
  shows below the scissors line, kept in the untested main.
- `cmd/workflow/main.go:42` — `terminalPrompt`, whose comment says "in the
  untested main".
- `cmd/workflow/main.go:74` — `composeInEditor`, the standup compose path —
  temp file, draft, editor, parse — of which only `command.Run()` needs a
  terminal.
- `cmd/workflow/main.go:75` — `composeInEditor`'s `os.CreateTemp`, write,
  joined `Close` and `Remove`: the third copy of the draft-file dance.
- `cmd/workflow/main.go:113` — `keychainStore`, a third implementation main
  carries untested.
- `internal/editor/editor.go:276` — `writeDraft`, in the package that
  already owns the draft file; `composeInEditor` does not use it.
- `internal/wiring/wiring.go:245` — `commitWith`, the second copy of the
  temp-file dance.

Most of the file is logic its own comments (`cmd/workflow/main.go:44`,
`:72`) call untested, including the compose path a `standup` user drives and
the scissors text they read. Only `command.Run()` needs a real editor; the
file handling and the `Draft`/`Parse` round trip
(`internal/editor/editor.go:119`, `:125`) are testable, and
`internal/editor` already owns the draft file.

**One way to fix it.** Move `composeInEditor` into `internal/editor` as an
exported `Compose` over `writeDraft`, `Draft`, `Parse` and `Invocation`,
tested with a scripted `$EDITOR`; move `standupHelp` beside the standup
command; leave `main` with `Execute`, the exit status and the two terminal
reads.

**Done when.** `cmd/workflow/main.go` holds no `os.CreateTemp` and no
user-facing string, and `go test ./internal/editor` covers a compose that
round-trips a draft through a fake editor.

### DEBT-82 The interface offers to link a forge issue number on Jira

Severity: medium · Confidence: read

The rule that a branch's issue is a Jira key only when it contains a dash is
written in two surfaces and missing from the third:

- `internal/cli/pr.go:285` — `isJiraKey`, the dash guard, first copy, with
  the comment (`:283`) saying why Jira must not see a bare number: it "would
  refuse that number, or read it as the id of an unrelated issue".
- `internal/webserver/issuewrite.go:189` — `server.branchIssue`, the dash
  guard, second copy.
- `internal/tui/branch.go:233` — `Model.branchIssue` has no guard:
  `convention.IssueKey`'s forge-number fallback is typed as a `jira.Key`
  with ok true.
- `internal/convention/convention.go:141` — `IssueKey` falls back to
  `forgeKey(text)` even with a Jira project configured, returning a bare
  number with ok true.
- `internal/tui/issuelink.go:34` — `Model.issueToLink` treats any named key
  as linkable when `Jira.LinkPullRequest` is wired.
- `internal/tui/issuelink.go:46` — `issueLinker.view` asks "Add this pull
  request's link to 42?"; the last look does not mark 42 as a forge number.
- `internal/tui/prcreate.go:130` — `pullCreated.apply` opens `issueLinker`
  whenever `named && m.deps.Jira.LinkPullRequest != nil`, so the bare number
  reaches a Jira write on confirm.
- `internal/cli/pr.go:147` — `runPR` re-derives the key with
  `convention.IssueKey` after `draft` (`internal/loop/pull.go:123`) already
  derived it in the same run; `ComposeAnnouncement`
  (`internal/loop/announce.go:67`) derives it a third time.

On a branch like `fix/42-typo` with Jira configured, the command line and
the web make no link offer, while the interface opens `issueLinker` on `42`
and a confirm posts a remote link to Jira issue id 42. Commit e4bf56f's
message acknowledges that the interface "shares the rule's edge, but asks
before it links"; the acknowledgment is in no trade-off, and the last look
does not name the edge. `internal/tui/issuelink_test.go` uses only
`PROJ-412`; no interface test uses a bare-number branch.

**One way to fix it.** Let `internal/loop` own it: a `JiraIssue(branch
gitrepo.Branch, project string) (jira.Key, bool)` with the guard, returned
from `ComposePull` alongside the branch and used by `pr` (dropping the
re-derivation at `internal/cli/pr.go:147`), the web's `branchIssue` and the
interface's `branchIssue`.

**Done when.** An interface test on `fix/42-typo` with Jira configured opens
a pull request and sees no link offer, and `strings.Contains(…, "-")` as a
Jira-key guard appears once in the module outside tests.

### DEBT-84 `git` missing from PATH is reported as "not a git repository"

Severity: medium · Confidence: read

`notInWorkTree` (`internal/gitrepo/gitrepo.go:90`), which
`Repository.Describe` and the probe behind the other reads share, turns every
failure of `git rev-parse --show-toplevel` but a timeout into
`ErrNotARepository`, discarding the runner's error; `build`
(`internal/proc/start.go:214`) is where a missing git becomes
`proc.ErrNotFound`, which gitrepo then discards. So on a machine without the
one program the settled decisions require, `unreadReason`
(`internal/cli/status.go:129`) prints `gitrepo.ErrNotARepository.Error()`
and `status` exits 4 under the wrong family, `Model.outsideRepository`
(`internal/tui/branch.go:112`) has every pane give the work-tree advice, and
the `proc.ErrNotFound` wording in `programErrors`
(`internal/tui/failure.go:287`, "Install it; `workflow doctor` names what is
missing") is unreachable from gitrepo. Only `doctor` notices, through its
own `proc.Available` check.

**One way to fix it.** In `notInWorkTree`, keep the runner's error when
`errors.Is(err, proc.ErrNotFound)`, as it already keeps a timeout, so the
existing wording and exit mapping for a missing program apply.

**Done when.** A `Describe` test whose fake runner returns
`proc.ErrNotFound` gets an error that `errors.Is` `proc.ErrNotFound` and not
`ErrNotARepository`; `workflow status` with git off PATH names the missing
program.

### DEBT-88 A key one separator after another key-shaped token is missed

Severity: medium · Confidence: read

`issueKey`'s pattern (`internal/convention/convention.go:51`) ends in
`(?:[^0-9]|$)`, which consumes the separator after a candidate, and
`jiraKey` (`internal/convention/convention.go:146`) walks
`FindAllStringSubmatch`, which resumes where neither `^` nor `[^A-Za-z0-9]`
can match; `forgeKey` (`:178`) then sees no digit after `^` or `/`. So in
`fix/UTF-8-PROJ-412` (no project) or `feat/ABC-1-PROJ-9` (project PROJ) the
real key is never found, contrary to `standardAbbreviations`' comment
(`:188`), which promises a real key beside a standard is still found. The
only test, the "a standard then a key" case of
`TestIssueKeyIsFoundWhereJiraWouldFindIt`
(`internal/convention/convention_test.go:140`), separates the two by a word
(`UTF-8-and-PROJ-412`).

Such a branch reads as having no issue on every surface that reads the
branch — the interface's panes, `status`, `pr`, `announce`, the commit
trailer and the web's task list: no `Refs` trailer, no issue in the
announcement, no transition offered. Generated branches never trip it (key
first, then the slug); a hand-named branch or another project's key directly
before the real one does.

**One way to fix it.** Match the bare key with `FindAllStringSubmatchIndex`
and check the neighboring characters by index rather than consuming them in
the pattern.

**Done when.** `IssueKey("fix/UTF-8-PROJ-412", "")` and
`IssueKey("feat/ABC-1-PROJ-9", "PROJ")` return the PROJ key, as table cases.

### DEBT-89 Comments and layout rows that no longer say what the code does

Severity: low · Confidence: read

Across the terminal, the command line, the clients, the plumbing, the web
server and the two layout maps, doc comments and layout rows describe an
earlier shape of the code. No linter reads a comment, so every one passes
the gate. The `wiring` package comment
below is the same drift DEBT-71 records at the type level.

The terminal:

- `internal/tui/branch.go:230` — `Model.branchIssue`'s comment calls itself
  "the one place the interface reads a branch name as an issue key" while
  `branchDetail` (`:141`) and `taskBranches`
  (`internal/tui/switchtask.go:62`) call `convention.IssueKey` too.
- `internal/tui/run.go:267` — `maxRunLines`' comment says the places to jump
  to are folded in as the lines arrive; `runLine.apply` (`:126`) folds in
  only `hooks.NextJob`, and `runFinished.apply` (`:158`) computes
  `hooks.Failures` from the capped lines after exit.
- `internal/tui/tui.go:188` — `Model.handleKey`'s comment orders overlay,
  help, global keys, pane; the switch (`:196`) has overlay,
  `filteringIssues`, global, and no help step.
- `internal/tui/overlay.go:15` — `overlay`'s comment says "Lip Gloss v1
  cannot layer one view over another" while the module requires
  `charm.land/lipgloss/v2`.
- `internal/tui/render.go:427` — "status describes the configuration…" sits
  atop `messagingLabel`'s comment block; `Model.status` (`:435`) has no
  comment.
- `internal/tui/keys.go:337` — `keyContexts`' comment says refresh,
  open-link and copy-link act on the Branch, Commits, Review and
  review-requests panes; the contexts (`:347`) give Branch and Commits only
  `actionRefresh`, and open and copy are answered on Issues
  (`handleIssuesKey`, `internal/tui/issuekeys.go:18`), Review
  (`handleReviewLink`, `internal/tui/review.go:448`) and Reviews
  (`handleReviewQueueKey`, `internal/tui/reviewqueue.go:188`).
- `internal/tui/keys.go:59` — `keyMap`'s comment on `openLink` and
  `copyLink`, "on the Issues and Review panes", omits the Reviews pane that
  `handleReviewQueueKey` (`internal/tui/reviewqueue.go:188`) handles.

The command line:

- `internal/cli/doctor.go:196` — `checkForge`'s comment says it "does not
  call the forge" and (`:198`) "costs no network round trip"; `askForge`
  (`:250`) calls `forge.New(doer, base, token).Whoami(ctx)`, reached from
  `checkForge` at `:229`, and
  `TestDoctorOnlineNamesWhereTheForgeTokenCameFrom`
  (`internal/cli/online_test.go:267`) exercises that round trip.
- `internal/cli/prompt.go:16` — `Prompt` is "the guided command's seams"
  though the `Compose` field's own comment (`:33`) says it edits a standup
  note and `confirm` (`:41`) reads `prompt.Line` for every scriptable
  write's yes/no question; `terminalPrompt` (`cmd/workflow/main.go:42`)
  repeats the stale scope with "reads guided-init answers".
- `CLAUDE.md:27` — the layout row "thin main; wires cli.Execute and the exit
  status" no longer describes `cmd/workflow/main.go` (DEBT-81).

The clients:

- `internal/forge/pulls.go:237` — `FindPullRequest`'s comment sends a caller
  to `Opened` "rather than trusting found alone"; `Opened` (`:118`) is
  `Number != 0`, true for a merged pull, and `IsOpen` is meant.
- `internal/forge/pulls.go:92` — `IsOpen`'s comment says "(Opened, above,
  …)"; `Opened` is declared at `:118`, below.
- `CLAUDE.md:42` — the layout row for `internal/forge/` lists "remotes,
  tokens, pull requests, CI", not the issues (`AssignedIssues`,
  `internal/forge/issues.go:30`) or the templates
  (`internal/forge/templates.go`).
- `internal/jira/jira.go:6` — the package comment names search, read, move,
  comment, link and whoami, not `Assign` (`internal/jira/assignee.go:16`),
  `AddWorklog` (`internal/jira/worklog.go:36`) or `WikiFromMarkdown`
  (`internal/jira/wiki.go:22`); `ARCHITECTURE.md:140` repeats the list
  without `Assign` and `AddWorklog`.
- `internal/jira/search.go:135` — `wireIssue`'s comment says the fields
  "past Reporter ride only on the detail request"; `searchFields` (`:27`) is
  `summary,status,issuetype,priority`, so `Description` (`:148`) and
  `Reporter` are detail-only too.
- `internal/jira/detail.go:35` — `LinkedIssue` is "a parent or a subtask";
  `IssueLink.Issue` (`:48`) is a `LinkedIssue` too, as `wireLinked`'s
  comment (`internal/jira/search.go:96`) says.
- `internal/jira/wiki.go:14` — `boldSentinel`'s comment calls `"\x00"` "A
  caret-feed control byte"; it is NUL, and there is no caret-feed control.
- `internal/config/config.go:4` — the package comment says `config` "loads
  the workflow configuration file"; `Save`, `SaveOver`, `RevisionOf`,
  `ParseRevision` and `SharedMode` are exported from
  `internal/config/save.go:108` onward, the `CLAUDE.md:29` row says
  "loading, redaction, validation", and the budget file's WHY
  (`scripts/package-size-budgets.txt:36`) says "load and save".

The plumbing:

- `internal/wiring/wiring.go:4` — the package comment "connects the terminal
  interface" to the clients, and the `CLAUDE.md:31` row "connects the
  interface's seams", name one of three consumers: `connectAt`
  (`internal/cli/cli.go:377`) builds every command over `wiring.Deps`,
  `WebDeps` (`internal/cli/cli.go:284`) hands the same bundle to the web,
  and the row below (`CLAUDE.md:32`) already says `loop` is "for every
  surface".
- `internal/wiring/wiring.go:138` — `browserCommand`'s comment promises
  every branch can be tested from one machine; its one caller,
  `openInBrowser` (`:115`), passes `runtime.GOOS`, and the gobco report
  shows the `goos == "windows"` condition never evaluated (DEBT-64).
- `internal/gitrepo/gitrepo.go:4` — the package comment says `gitrepo`
  "reads the git repository"; `Repository`'s own doc (`:28`) says reads and
  changes, and `Repository.Stage` (`internal/gitrepo/status.go:191`) is one
  of the writes.
- `internal/convention/convention.go:4` — the package comment names three
  concerns; `internal/convention/pullrequest.go` and
  `internal/convention/scopes.go` are two more, and the `CLAUDE.md:47` row
  already lists "pull request text".
- `internal/messaging/post.go:328` — `Announcement.Text`'s comment says
  "Every substituted value is escaped for Slack"; `markupFor` (`:267`)
  escapes per kind, and `keepText` (`:316`) not at all.
- `internal/messaging/messaging.go:44` — `ErrRejected` "reports a token
  Slack would not accept" and is returned for any kind's webhook 4xx by
  `deliver` (`internal/messaging/post.go:208`).

The web server:

- `internal/webserver/webserver.go:33` — `Deps`' comment says a nil read
  seam answers "with an empty result rather than an error";
  `server.GetIssue` (`internal/webserver/handlers.go:81`) answers 422 "no
  issue tracker is configured" for a nil `Issue`, and
  `server.GetAnnouncement` (`internal/webserver/announce.go:22`) and
  `server.GetPullRequestDraft` (`internal/webserver/pullrequest.go:26`)
  answer 409 for a nil `Branch` or `FindPull`, which `pullToAnnounce`
  (`internal/loop/announce.go:87`) reports as `ErrNoPullRequest`.
- `internal/webserver/webserver_test.go:25` — `errSeam`'s comment, "generic
  500 so the wire message carries no detail", predates `fault`
  (`internal/webserver/errors.go:91`), which classes by sentinel before the
  internal fallback.

A reader of `go doc`, of CLAUDE.md's layout table or of ARCHITECTURE.md is told
something the code beside it does not do, and acts on it: changes one call site
of three, expects `doctor --online` to stay off the forge, treats a merged pull
as open, or tries a layering the v2 upgrade already allows. The cost is paid at
the next change, when the comment is trusted over the code.

**One way to fix it.** One pass, file by file, rewording each sentence to
what the code does now — or deleting the enumerations that go stale a verb
at a time.

**Done when.** `go doc` for `config`, `wiring`, `gitrepo`, `jira`,
`convention` and `forge.Client.FindPullRequest` reads as the code does; both
pane lists in `internal/tui/keys.go` match the
handlers; the `Deps` comment names the answers the handlers give; and each
of these prints nothing — `grep -n "the one place the interface reads"
internal/tui/branch.go`, `grep -n "cannot layer one view over another"
internal/tui/overlay.go`, `grep -n "does not call the forge"
internal/cli/doctor.go`, `grep -n "guided command's seams"
internal/cli/prompt.go`, `grep -n "checks Opened rather than trusting"
internal/forge/pulls.go`, `grep -n "caret-feed" internal/jira/wiki.go`,
`grep -n "status describes the configuration" internal/tui/render.go` and
`grep -n "so the wire message carries no detail"
internal/webserver/webserver_test.go`. The greps are a sample; every other
cited sentence is checked the same way, by grepping the phrase its bullet
quotes in the file it cites.

### DEBT-90 The docs site trails the code across usage, configuration, web and install

Severity: low · Confidence: read

The site's pages, the README and the docs index promise things the code does
not do or stay silent on things it does. `scripts/check-docs-drift.sh`
compares only the generated command reference, so no gate sees any of these
pages. UX-87 counts the same Settings undercount from the user's side.

The usage page:

- `docs/content/docs/usage.md:173` — "Pick up an issue" names three field
  cases (a fixed set, text, any other kind sent to Jira) where
  `fieldForm.textual` (`internal/tui/fields.go:79`) fills `FieldUser` and
  `FieldDate` as typed inputs and `fieldForm.multi` (`:90`) takes any number
  of a `FieldOptionList`'s options.
- `docs/content/docs/usage.md:185` — "Branch" says the branch "starts from
  origin's default branch, which the overlay names" with no word of the
  fetch (`branchCreator.create`, `internal/tui/branch.go:442`), the "fetched
  AGE" line (`branchCreator.start`, `:364`) or the offer after a failed
  fetch (`fetched.apply`, `internal/tui/branchresult.go:49`).
- `docs/content/docs/usage.md:192` — "Stage and commit" describes the list
  and the composer; the page never mentions a diff (`grep -ic diff` is 0)
  while `diffSection` (`internal/tui/diff.go:67`) draws the selected file's
  diff beneath the list.
- `docs/content/docs/usage.md:225` — "The title is the branch's oldest
  commit subject" is true only when `pull_request.title_source`
  (`PullRequest.TitleSource`, `internal/config/pullrequest.go:21`) is the
  default; the key is validated (`:24`) and on no docs page.
- `docs/content/docs/usage.md:300` — "Editor" says "`$VISUAL`, else
  `$EDITOR`, else `vi`"; `chosen` (`internal/editor/editor.go:85`) consults
  `GIT_EDITOR` first and `defaultEditor` (`:97`) returns `notepad` on
  Windows, while `defaultEditor`'s own comment (`:94`) and
  `composeInEditor`'s (`cmd/workflow/main.go:71`, "$EDITOR (or $VISUAL, else
  vi)") omit `$GIT_EDITOR` too.

The configuration page (the Fields table's missing rows are DEBT-91):

- `docs/content/docs/configuration.md:37` — "does the same for Slack" claims
  `config init` checks the webhook; `newConfigInitCmd`'s Short
  (`internal/cli/config_cmd.go:58`, "asking for and checking each
  credential") and Long (`:59`, "check each one") say the same, reproduced
  at `docs/content/docs/reference/workflow_config_init.md:10` and `:14`,
  while `collectMessaging` (`internal/cli/config_cmd.go:308`) prints "saved
  (a webhook cannot be checked without posting)".
- `docs/content/docs/configuration.md:44` — "looks for .workflow.json in the
  current directory first, then in your home directory", as does
  `Discover`'s comment (`internal/config/load.go:18`, "searching workDir
  first and then homeDir"), where `Discover` (`:26`) calls `nearest`, which
  walks up to the directory holding `.git` (`:37`).
- `docs/content/docs/configuration.md:375` — "While it is on and no
  `timing.ci_interval` is set, CI is polled every three minutes" gives two
  of `pollInterval`'s three conditions (`internal/tui/review.go:185`): no
  announcement may be waiting either.
- `docs/content/docs/configuration.md:472` — the store is "keyed only by a
  repository's host and path and by a hash of your Jira URL", and
  `ARCHITECTURE.md:228` says the repository key is the remote's parsed host
  and path; `migrate` (`internal/store/store.go:202`) keys the cache by
  `(instance, view)`, where `Model.cacheIssues`
  (`internal/tui/issues.go:47`) passes the view's JQL text, and `repoKey`
  (`internal/wiring/wiring.go:396`) falls back to `where.Root` when there is
  no remote or it does not parse.

The README and the docs index:

- The README (line 45) and `docs/content/_index.md:29` — "`workflow doctor
  --online` asks Jira, your messaging service and your forge whether each
  credential actually works"; `checkMessaging`'s comment
  (`internal/cli/doctor.go:263`) says a webhook is uncheckable,
  `ErrWebhookUncheckable` returns nil (`:286`), `credentialStatus` reads
  that as `ok` (`internal/cli/doctor_json.go:237`), and
  `TestDoctorOnlineSaysAWebhookCannotBeChecked`
  (`internal/cli/online_test.go:169`) pins "cannot be checked".
- The README's line 262 — "There are no releases yet." while nine tags
  exist, v0.3.0 the latest; the README's line 66 and
  `docs/content/docs/install.md:37` still say that until the first tag
  exists `@latest` resolves to `main`, and the pinned example (`:34`, and
  the README's line 65) is `@v0.0.5`, five releases behind.

The web page:

- `docs/content/docs/web.md:126` — Settings lists its seven parts as Jira,
  the forge, messaging, branches, commits, pull requests and the store;
  `ConfigForm` (`web/src/features/settings/SettingsPanel.tsx:127`) renders
  Jira, Messaging, Forge, Commit, Branch, Pull request, Store.
- `docs/content/docs/web.md:129` — "The parts the form does not show yet
  (`ui`, `timing`, `headers`, `views` and `branch.prefixes`)" names five of
  ten carried keys, as UX-87 in `UX.md` ("carry five it cannot show") and
  `ConfigForm`'s comment (`web/src/features/settings/SettingsPanel.tsx:67`)
  do; the whole `Config` seeds the form and rides back, and `token_command`
  and `token_env` (`api/openapi.yaml:1396`; messaging's at `:1431`) and
  `channels` (`:1437`; `Messaging.Channels`,
  `internal/config/config.go:121`) are in the schema and registered by no
  fieldset.
- `docs/content/docs/web.md:131` — "a save never overwrites a change it has
  not seen" is stronger than `SaveOver` makes it: its comment
  (`internal/config/save.go:117`) says the check (`:121`) and the write
  (`:130`) are not one step, and the `staleTime: Infinity` trade-off in this
  file repeats the page's phrasing.
- `web/src/queryClient.ts:4` — the `queryClient` comment says "the stream's
  snapshots update it through setQueryData"; the stream handler in
  `useEventStream` (`web/src/api/snapshot.ts:71`) writes `useSnapshotStore`,
  and the only `setQueryData` callers are `useReloadConfig`
  (`web/src/features/settings/configApi.ts:129`) and the save (`:111`).

The reference index:

- `docs/content/docs/reference/_index.md:9` — "Every command and flag,
  generated from the command tree itself", and "The same text is available
  offline" (`:13`); `run` in `cmd/docsgen/main.go:88` says cobra's `help`
  command gets no page, `docs/content/docs/scripting.md:38` documents `help`
  as a command with its own exit behavior, and
  `scripts/check-docs-drift.sh:30` copies the hand-written index around the
  comparison, so the gate cannot see the claim.

A reader expects a webhook typo caught at init and it is saved unchecked,
then trusts a `doctor` check that never ran; sets `$GIT_EDITOR` and is told
another editor opens; in a subdirectory gets the repository's file after
reading that the current directory wins; reads "never overwrites" as a
guarantee the code does not make; looks for `workflow help` in the reference
and finds nothing; is warned off unreleased code they will not get and shown
a pin five releases old; follows the web page to the second Settings part
and finds a third; and meets a date input, a "could not fetch" offer and a
diff the usage page never mentions.

**One way to fix it.** One editing pass over the cited pages with the code's
own comment as the source of each sentence: say the webhook is saved
unchecked on the configuration page and in `config init`'s Short and Long,
then `task docs:gen`, and drop the `doctor` promise from the README and the
docs index; state the editor order as git's (`$GIT_EDITOR`, `$VISUAL`,
`$EDITOR`, else `vi`, `notepad` on Windows) on the usage page and in both
comments; describe the walk to `.git` on the page and in `Discover`'s
comment; name the three store identifiers and the root-path fallback on both
pages; qualify "never overwrites" with the revision check's window; state
the third notify condition; name `help` as the one command without a page;
drop the "no releases yet" and "until the first tag" sentences and refresh
the pinned example; reorder the web page's Settings list to the form's and
name every carried key; add the field kinds, the fetch and the diff to the
usage page; and reword the `queryClient` comment to `useSnapshotStore`.

**Done when.** `grep -c 'does the same for Slack'
docs/content/docs/configuration.md`, `grep -c 'no releases yet' README.md`,
`grep -ci 'until the first tag' README.md docs/content/docs/install.md` and
`grep -c setQueryData web/src/queryClient.ts` all print 0; `grep -n
GIT_EDITOR docs/content/docs/usage.md cmd/workflow/main.go
internal/editor/editor.go`, `grep -n help
docs/content/docs/reference/_index.md` and `grep -n '\.git'
docs/content/docs/configuration.md` each match a sentence that says what the
code does; the README and `docs/content/_index.md` no longer say `doctor`
checks a webhook; the web page's Settings list reads in `ConfigForm`'s
fieldset order; the usage page names user, date and multi-select fields, the
fetch and the diff; and `task docs:check` is green after `task docs:gen`.

### DEBT-91 The configuration page's Fields table omits thirteen keys the code reads

Severity: low · Confidence: read

Thirteen keys the loader reads, validates or writes — `version`, `jira.project`,
`jira.review_status`, `forge.cli`, `messaging.channels`, `ui.comments_shown`,
`timing.request_timeout`, `timing.ci_interval`, `branch.slug_limit`,
`commit.types`, `commit.subject_limit`, `commit.refs_trailer` and
`pull_request.title_source` — have no row in the Fields table of
`docs/content/docs/configuration.md:58`, the `timing` and `pull_request`
sections are never named, and the sample file at the top of the page omits the
`version` key `config init` writes first. Counted by grepping each key on the
page: 0 hits each, `ci_interval` once in prose. The page says an unknown key is
an error, then omits thirteen it accepts. A user who meets "Review status" or
"Use the forge CLI" in the browser, the channel cycle in the terminal, the
72-character ruler or the `Refs:` trailer in the composer, or `"version": "1"`
at the top of a file `config init` just wrote, opens the reference and finds
nothing; `forge.cli`, the setting that reaches a forge behind SSO, is documented
in the README (line 191) and `docs/content/docs/scripting.md:51` but has no row
on the configuration page; usage.md presents the limit, the trailer and the
oldest-commit title as fixed when each has a key. `doctor` and the Settings form
honor every one, and no gate sees the page. The web side of the same gap is
UX-87.

- `docs/content/docs/configuration.md:58` — the Fields table the thirteen
  rows are missing from.
- `docs/content/docs/configuration.md:10` — the sample file under
  "Configuration" lists `jira`, `messaging`, `forge` and `ui`; no
  `version`.
- `internal/config/config.go:164` — `Config.Version`, written first by
  `config init` and validated; 0 hits on the page.
- `internal/config/config.go:80` — `Jira.Project`, the branch-name key
  guard; 0 hits.
- `internal/config/config.go:90` — `Jira.ReviewStatus`, which drives the
  post-open transition offer; named in errors.md and the `pr` reference
  only.
- `internal/config/config.go:121` — `Messaging.Channels`, the channel
  cycle's source; 0 hits.
- `internal/config/config.go:151` — `Forge.CLI`, which routes forge calls
  through `gh` or `glab`; on the site only at
  `docs/content/docs/scripting.md:51`.
- `internal/config/ui.go:37` — `UI.CommentsShown`; 0 hits.
- `internal/config/timing.go:21` — `Timing.RequestTimeout`; 0 hits, and no
  `timing` row at all.
- `internal/config/timing.go:24` — `Timing.CIInterval`, named once in
  prose at `docs/content/docs/configuration.md:375` ("The interface:
  mouse, ASCII and color") as if already introduced; its format and
  twenty-second default are nowhere.
- `internal/config/branch.go:37` — `Branch.SlugLimit`, validated at `:55`;
  0 hits, and the Branch names section lists the other three fields only.
- `internal/config/commit.go:29` — `Commit.Types`, validated at load; 0
  hits.
- `internal/config/commit.go:32` — `Commit.SubjectLimit`, default 72; 0
  hits.
- `internal/config/commit.go:35` — `Commit.RefsTrailer`, default `Refs`; 0
  hits.
- `internal/config/pullrequest.go:21` — `PullRequest.TitleSource`, refused
  when unknown at load; the whole `pull_request` block is absent from the
  page.
- `docs/content/docs/usage.md:197` — "Stage and commit": "the 72-character
  limit" stated as fixed though `commit.subject_limit` changes it.
- `docs/content/docs/usage.md:198` — "A `Refs:` trailer" stated as fixed
  though `commit.refs_trailer` relabels it.
- `docs/content/docs/usage.md:225` — "Open the pull request": "The title
  is the branch's oldest commit subject" stated as fixed though
  `pull_request.title_source` can switch it to the issue.
- `docs/content/docs/usage.md:259` — "Announce it" promises a channel
  choice "with more than one channel to choose from" that the
  configuration page never says how to set up.
- `docs/content/docs/install.md:14` — "What it needs" lists `gh` only, as an
  optional place to find a GitHub token; `glab`, which `forge.cli` needs on
  GitLab, is not listed.

**One way to fix it.** One row per key in the Fields table with the struct
field's own comment as the text and the default stated (ten seconds,
twenty seconds, 48, 72, `Refs`, `commit`); `"version": "1"` in the sample
file; `timing` and `pull_request` named as sections; usage.md's sentences
on the subject limit, the trailer and the title qualified with "by
default"; and `glab` beside `gh` in install.md's needs list.

**Done when.** For each of the thirteen keys
`grep -c '<key>' docs/content/docs/configuration.md` returns at least 1,
`grep -c '"version"' docs/content/docs/configuration.md` returns at least
1, `task docs:check` stays green, and `grep -n glab
docs/content/docs/install.md` prints a line.

### DEBT-92 Tests in Go, the scripts and the web that prove nothing

Severity: low · Confidence: read

Across `internal/tui`, `internal/cli`, `internal/forge`, `internal/config`,
`internal/store`, the release and coverage scripts and the web's vitest
suite, tests named for a rule would pass with the rule gone. `testshape`
checks only that a failure call is reachable and v8's range count cannot see
a weak assertion, so every one clears the gate.

The terminal:

- `internal/tui/hookgen_test.go:118` — `TestTheOfferIsMadeOnlyWhenItHelps`
  calls `refuseScreen` on the start screen, where
  `TestTheLefthookOfferOpensFromTheCommitsPaneNotAtStart` (`:36`) shows the
  offer never opens at start; it cannot see the `msg.configured ||` guard in
  `hooksFound.apply` (`internal/tui/hookgen.go:45`).
- `internal/tui/commits_test.go:84` —
  `TestTheCommitsDetailWaitsForTheStatus` calls `requireScreen` for
  "loading" on the whole screen; `commitsRail`
  (`internal/tui/commits.go:104`) says it in the rail whether or not the
  detail is open.
- `internal/tui/notify_test.go:120` —
  `TestNotifyPollsOnALongerBeatWithNoIntervalSet` asserts only "running" and
  no ring; `horizon` (`internal/tui/harness_test.go:27`) is one second, so
  any beat over a second is indistinguishable from `notifyPollInterval`
  (`internal/tui/review.go:179`).
- `internal/tui/merge_test.go:328` — `TestTheMergePreviewShowsItIsMerging`
  presses `j` in flight and asserts only "merging"; without the
  `p.send.sending` guard in `mergePicker.handleKey`
  (`internal/tui/merge.go:151`), `j` would move the selection and the word
  would still show.
- `internal/tui/finish_test.go:226` —
  `TestTheFinishPreviewShowsItIsFinishing` presses `x`, a key
  `finishPreview.handleKey` (`internal/tui/finish.go:99`) ignores anyway,
  and asserts only "finishing".
- `internal/tui/preditor_test.go:179` — `TestTheEditorShowsItIsSaving`
  presses `x` and asserts only "saving"; without the guard in
  `prEditor.handleKey` (`internal/tui/preditor.go:83`), `x` lands in the
  title (`:94`) unseen.
- `internal/tui/edges_test.go:27` — `TestNothingInterruptsAWriteBeingSent`,
  the table the three guards should join, covers a branch, a pull request,
  an announcement, a configuration and a re-run.

The command line:

- `internal/cli/branch_test.go:289` — `TestBranchReportsAFailedCreate`
  asserts `err == nil` only; so do `TestBranchReportsAnUnreadableRepository`
  (`:306`), leaving the documented exit 4 unpinned, and
  `TestBranchReportsAnUnreachableTracker` (`:322`), whose fixture answers
  500 (`:315`) — a rejection, exit 1, since `statusError`
  (`internal/jira/jira.go:240`) makes a 500 `ErrUnexpectedStatus`, never
  `ErrUnreachable` — so "Unreachable" in its name is wrong.
- `internal/cli/scriptable_test.go:25` — `TestBranchCommandNeedsATracker`
  asserts `err == nil` only and cannot tell the exit-3 refusal from any
  other failure; `TestPRCommandReadsTheBranch` (`:36`),
  `TestStandupOutsideARepositoryReportsSo`
  (`internal/cli/standup_test.go:80`) and
  `TestReviewsWithoutAForgeReportsSo` (`internal/cli/reviews_test.go:53`) do
  the same, while `TestAnnounceCommandNeedsMessaging`
  (`internal/cli/scriptable_test.go:46`) shows the file's own stronger
  shape, `wantExit` (`internal/cli/exitstatus_test.go:30`) is the helper the
  seven could call, and `TestRequestLogReportsAFileItCannotOpen`
  (`internal/cli/reqlog_test.go:40`) explains why a bare error check proves
  nothing.

The clients:

- `internal/forge/reviews_test.go:185` —
  `TestReviewRequestsForAnUnknownForge` accepts any error and never names
  `ErrUnknownForge`; `TestForgeIssueMethodsRejectAnUnknownForge`
  (`internal/forge/issues_test.go:232`) does the same in each of three
  subtests.
- `internal/config/version_test.go:24` —
  `TestLoadAcceptsTheCurrentVersionAndRejectsAnUnknownOne` runs two `Load`s
  under one Act (`:24`, `:25`) and checks
  `strings.Contains(unknownErr.Error(), "99")` (`:36`) rather than
  `errors.Is`, so `ErrUnknownVersion` (`internal/config/config.go:45`) is
  exported, wrapped and asserted by nothing.
- `internal/config/load_test.go:169` —
  `TestDiscoverIgnoresAnEmptyDirectory`'s comment describes a machine with
  no home directory, but the Act (`:170`) passes `""` as workDir, so
  `nearest("")` never enters its loop, and the `dir == ""` guard in `fileIn`
  (`internal/config/load.go:85`) the test means to cover was, per the gobco
  report, 66 times false and never true.

The scripts:

- `scripts/release/push-release-tag_test.sh:32` — the `gh` stub answers any
  `gh api` call with `GH_STUB_PULL` (`:33`) and ignores `--jq`, so the label
  filter in `pr_number` (`scripts/release/push-release-tag.sh:71`) is
  evaluated by no test, and the "no pull request labeled" case
  (`scripts/release/push-release-tag_test.sh:96`) passes because the stub
  printed nothing, not because the filter selected nothing.
- `scripts/coverage-summary_test.sh:69` — the summary-shape case asserts the
  substrings `"statements"` and `"branch"` only; the `stats` fixture (`:51`)
  yields 50 %, which nothing compares, so the arms arithmetic in
  `scripts/coverage-summary.sh:41` is unprotected.
- `scripts/gobco-report_test.sh:23` — the suite's only case is the no-floor
  argument; the untested-package refusal (`unaccounted`,
  `scripts/gobco-report.sh:176`) and the no-statistics refusal (`summary`,
  `:241`) are exercised only in their passing direction.

The web:

- `web/src/features/review/ReviewPanel.tsx:275` — `PullRequestForm`'s "The
  branch is not pushed yet; opening will push it first." is asserted
  nowhere; the mocked draft in
  `web/src/features/review/ReviewPanel.test.tsx:20` sets `needs_push: true`
  for every case.
- `web/src/features/issues/IssueDetailPanel.test.tsx:311` — "a Retry refused
  again beside an issue already read keeps its focus" names a browser
  outcome jsdom cannot observe: `expect(document.activeElement).toBe(retry)`
  (`:338`) passes because jsdom does not move focus off a disabled control,
  while `IssueUnread`'s `disabled={retrying}`
  (`web/src/features/issues/IssueDetailPanel.tsx:81`) drops it in Chromium.
- `web/src/features/issues/IssueListControls.test.tsx:124` — "offers no view
  select when the views cannot be read" waits only for a request, and
  `queryByRole('combobox')` (`:126`) is null while pending too, since
  `ViewSelect` (`web/src/features/issues/IssueListControls.tsx:50`) returns
  null for an empty list either way.
- `web/src/features/review/ReviewPanel.test.tsx:92` —
  `expect(screen.getByText('build')).toBeTruthy()` matches a span as readily
  as a link, so the `check.url === ''` branch in `PullRequestSummary`
  (`web/src/features/review/ReviewPanel.tsx:120`) is exercised by the one
  fixture with a URL (`web/src/features/review/ReviewPanel.test.tsx:79`) and
  never checked by role or href.
- `web/src/features/writes.test.tsx:326` — the writes table discards the
  `Request[]` that `fakeApi` (`web/src/test/fakeApi.ts:14`) returns for
  exactly this, so `breaking: fields.breaking` in `CommitForm`
  (`web/src/features/branch/CommitForm.tsx:75`) can be dropped —
  `CommitRequest.breaking` is optional
  (`web/src/api/generated/types.gen.ts:114`), so it compiles — and no test
  fails.

The store:

- `internal/store/store.go:64` — `timestamp`, the RFC3339 rule, has no test
  behind it; `Store.CachedIssues` (`internal/store/cache.go:43`) scans
  `cached_at` (`:46`) into a variable nothing reads.
- `internal/store/store.go:56` — `dsnPragmas`' `foreign_keys(1)` and the `ON
  DELETE CASCADE` on `cached_issue` (`:215`) are exercised by nothing:
  `writeCachedIssues` (`internal/store/cache.go:142`) deletes the children
  itself, so the cascade guards nothing.
- `internal/store/store_test.go:79` — `TestScopesAreKeptPerRepository`
  discards `RecordScope`'s error and asserts only that another repository
  reads nothing (`:85`); `TestTheCacheIsKeptPerInstanceAndView`
  (`internal/store/cache_test.go:85`) discards `CacheIssues`' error and
  asserts only that another view and instance read nothing (`:92`), so a
  `RecordScope` or `CacheIssues` that writes nothing passes.

The regressions these tests exist to catch pass the suite green: a dropped
in-flight guard on the merge picker, the finish preview or the pull request
editor; `branch` exiting 1 where the contract says 4; a lost
`ErrUnknownForge` or `ErrUnknownVersion` wrap; a release-label filter typo
found at the next release; a `timestamp()` that writes `now.String()`; a
commit sent without its `!` or body; the "opening will push it first" note
deleted.

**One way to fix it.** Sharpen each Assert to what its name claims:
`wantExit` or `errors.Is` with the family or sentinel meant, and rename the
500 case "rejected"; the focused pane's title and first body row rather than
the whole screen; a recording timer for the notify beat; the three missing
guards as cases of `TestNothingInterruptsAWriteBeingSent`; `aria-disabled`
on Retry or a Playwright focus case; the refused read awaited before
asserting the select is absent; the check asserted by role and href; the
recorded requests read for their bodies; the `gh` stub running the script's
own `--jq` over a fixture of pulls; exact JSON from
`scripts/coverage-summary.sh`; a stub gobco for the gate's refusals;
raw-file reads that parse each `_at`, a cascade a test makes fire, and each
Arrange's error fatal.

**Done when.** Each named mutation fails a test: deleting `msg.configured
||` from `hooksFound.apply`; setting `notifyPollInterval` to 20 seconds;
removing `case p.send.sending` from `mergePicker.handleKey`,
`finishPreview.handleKey` and `prEditor.handleKey`; changing `branch`'s
non-repository exit from 4; returning a different sentinel for `KindUnknown`
from `ReviewRequests` or the issue methods; removing `select(any(.labels[];
…))` from `scripts/release/push-release-tag.sh`; changing `($conditions *
2)` to `$conditions` in `scripts/coverage-summary.sh`; deleting a name from
`NO_TESTS` in `scripts/gobco-report.sh`; changing `timestamp()` to
`now.String()`, removing `foreign_keys(1)` from `dsnPragmas`, or making
`RecordScope` or `CacheIssues` return nil without writing; rendering a
select while `useViews` is in error; replacing the check anchor in
`web/src/features/review/ReviewPanel.tsx` with a span; deleting `breaking:
fields.breaking` from `web/src/features/branch/CommitForm.tsx`; and deleting
the "opening will push it first" paragraph.

### DEBT-93 Facts written in two places with nothing holding the copies together

Severity: low · Confidence: read

Facts the code needs in more than one place are written in each, with
nothing keeping the copies equal. CLAUDE.md names the smell and the rule of
three; no linter or knip rule sees any of it.

- `internal/tui/branch.go:22` — `notInRepository` is a second wording of
  `gitrepo.ErrNotARepository`, under a comment (`:21`) saying the fact is
  "stated one way"; `programErrors` (`internal/tui/failure.go:281`) holds
  the first.
- `internal/tui/composer.go:378` — `Model.recordScope` tests `scope != ""`
  on the raw `c.scope.Value()` (`:143`), so `' '` is recorded;
  `server.rememberScope` (`internal/webserver/commit.go:174`) trims first,
  and `ValidateScope` (`internal/convention/convention.go:62`) accepts a
  whitespace-only scope.
- `internal/tui/run.go:222` — `commandRun.failureHeadline` is a map keyed by
  run-title literals, "the fixup was refused" (`:225`) untested, while six
  sites type the titles: `commitComposer.commit`
  (`internal/tui/composer.go:364`, "git commit"), the amend
  (`internal/tui/commits.go:384`), the fixup (`:399`), `preCommit` (`:22`),
  the push (`internal/tui/run.go:398`) and the rebase (`:418`).
- `internal/tui/spine.go:68` — `Model.stages` hard-codes five hues in stage
  order and indexes them (`:74`) by the position of `Stages`' result
  (`internal/progress/progress.go:66`), the one place the stages are
  derived.
- `internal/forge/remote.go:153` — `kindOf` knows `github.com` only, so a
  `.ghe.com` host is `KindUnknown`; `githubsOwn`
  (`internal/forge/host.go:40`) and `githubAPIBase`
  (`internal/forge/remote.go:186`) both know the `.ghe.com` rule, and
  `checkForge` (`internal/cli/doctor.go:216`) tells such a tenant to "set
  forge.kind and forge.host" for a host the code could classify.
- `internal/config/ui.go:45` — the rebindable action names are listed
  in `UI.Keys`' comment, again under "Rebinding keys"
  (`docs/content/docs/configuration.md:405`), and bound in `CheckKeys`
  (`internal/tui/keys.go:293`); no test holds the three to each other.
- `internal/config/config.go:349` — `Config.Problems`' sentence
  "jira.base_url is not an absolute http or https URL" is
  `ErrInvalidBaseURL`'s text verbatim (`internal/jira/jira.go:41`), and the
  rule behind it is written twice: `absoluteWebURL`
  (`internal/config/config.go:364`) accepts userinfo that `usable`
  (`internal/jira/jira.go:151`) refuses first (`:147`).
- `.github/workflows/release-please.yml:67` — "Provision the toolchain for
  the gate" and "Verify the gate before tagging" (`:79`) each restate the
  release subject with `startsWith(…, 'chore(main): release ')`, "Push tag
  for a merged release PR" (`:89`) runs the script with no `if:`, and
  `version` in `scripts/release/push-release-tag.sh:46` decides the same
  question with its own `sed` regex.
- `web/e2e/a11y.spec.ts:141` — `issuesSnapshot` and `pagedSnapshot`
  (`web/e2e/layout.spec.ts:351`) are untyped literals of the empty snapshot
  shape that `makeSnapshot` (`web/src/test/fixtures.ts:23`) builds typed.
- `web/src/features/branch/CommitForm.tsx:12` — `defaultCommitTypes` copies
  the eleven Go types (`internal/convention/commit.go:40`) in order, and
  `useCommitTypes` (`web/src/features/branch/CommitForm.tsx:131`) falls back
  to it when `config.commit.types` is empty; `server.commitConvention`
  (`internal/webserver/commit.go:73`) resolves the same empty list through
  `convention.NewCommitConvention`, so the server's default is never sent,
  and no test compares the two.
- `web/src/shell/themeStore.ts:11` — `storageKey` is `'workflow-theme'` with
  a "Keep the two in step" comment and is not exported; `web/index.html:25`
  spells it again in the pre-paint script, `web/src/shell/theme.test.tsx:21`
  pins the store's copy only, and the e2e theme case
  (`web/e2e/theme.spec.ts:12`) stores `'system'`, which resolves as an
  unread key does.
- `web/src/features/settings/SettingsPanel.tsx:59` — `secondaryButton`
  carries `text-foreground` where every other copy has `text-sm`; `control`
  (`web/src/features/reviewqueue/ReviewQueuePanel.tsx:27`) lacks the
  `disabled:` classes; and the same string is inline in `AnnouncePreview`
  (`web/src/features/messaging/MessagingPanel.tsx:261`), `PushButton`
  (`web/src/features/branch/BranchPanel.tsx:144`), `PullRequestForm`
  (`web/src/features/review/ReviewPanel.tsx:284`), `CheckoutButton`
  (`web/src/features/issues/WorkStory.tsx:267`), `FollowUpOffer`
  (`web/src/features/review/OpenedOutcome.tsx:88`) and `IssueUnread`
  (`web/src/features/issues/IssueDetailPanel.tsx:83`), with the primary's
  inline in `CommitForm` (`web/src/features/branch/CommitForm.tsx:108`).
  The primary button's three sizes are UX-113; one `Button` component (or
  one primary and one secondary class) closes both.
- `internal/cli/cli.go:366` — `connectLeniently` discards `os.UserHomeDir`'s
  error under a "not a failure" comment (`:364`); `loadFromEnvironment`
  (`:440`), `statusesOf` (`internal/cli/status.go:162`) and
  `completeAssignedIssues` (`internal/cli/scriptable.go:132`) repeat the
  discard and the comment, and `targetDir`
  (`internal/cli/config_cmd.go:128`) is the one caller that must keep the
  error.
- `internal/convention/commit.go:23` — `commitType` is `^[a-z][a-z0-9]*$`
  while `scopeInSubject` (`internal/convention/scopes.go:15`) is
  `^[a-z]+\(([^)]+)\)!?:`; the two disagree on a digit.

A whitespace-only scope is recorded by the terminal and not the web, and the
next composer opens on it; a configured `hotfix2` type validates but its
scopes are never suggested; a sixth progress stage compiles and panics in
the spine; a `.ghe.com` tenant is told by `doctor` to set what the code
could infer; a renamed theme key in `web/index.html` passes every gate and
shows only as a flash before first paint; a drifted release prefix pushes a
tag the gate never saw, or silently tags nothing; a required snapshot field
added to the contract dies in the e2e specs as a locator timeout; a change
to the focus ring is eight edits; and a wording change to the repository
sentence is made twice beside a comment that says there is one.

**One way to fix it.** One owner per fact: `gitrepo.ErrNotARepository`
rendered through the failure block with `notInRepository` deleted; a
`loop.RememberScope` both surfaces call, with one trim; a run-kind value
carrying title and headline; a system field on `progress.Stage` the spine
looks up in a map `exhaustive` checks; `kindOf` consulting `githubsOwn`; a
terminal test that reads the action names out of the configuration page and
holds `CheckKeys` to them; one base-URL rule in `config` that `jira` wraps;
the tag script owning the release-commit decision and the workflow reading
its answer; `satisfies Snapshot` on the e2e literals; the server sending the
effective commit types so the form holds no list; an exported `storageKey` a
test checks `web/index.html` against; one `Button` component or two class
constants; one `configHome()` the four callers share; and `scopeInSubject`
built from `commitType`'s class.

**Done when.** `grep 'inside a git repository' internal/tui` finds one
string; a test shows `' '` is recorded by neither surface; no run-title
literal appears in more than one file and each kind's headline has a test;
the spine's hue comes from a field on `progress.Stage` and `exhaustive`
fails the build when a system has no hue;
`ParseRemote("git@acme.ghe.com:owner/repo.git").Kind == KindGitHub`; a test
fails when the configuration page's action list and the bind sites differ;
one function decides a base URL's shape and both packages' tests import it;
`chore(main): release` appears in exactly one file under `.github/` and
`scripts/`; removing a required snapshot field from the e2e literals fails
`tsc -b`; `grep -n "'revert'" web/src/features/branch/CommitForm.tsx` is
empty and the options come from a server field; a test fails when
`web/index.html`'s key differs from `themeStore`'s; `grep -rn "border
border-input px-3 py-1.5" web/src --include='*.tsx'` matches one definition
site; `grep -n UserHomeDir internal/cli/*.go` returns the helper and
`targetDir`; and `Scopes([]string{"hotfix2(api): x"})` returns `["api"]`.

### DEBT-94 Arms and guards no input can reach, on every surface

Severity: low · Confidence: read

Arms and guards kept just in case that no input can reach. Each is YAGNI by
CLAUDE.md's catalog; most sit permanently in DEBT-64's worklist where no
test can close them, the two compound guards' dead first operands among
them.

- `internal/cli/status.go:414` — `statusGlyph`'s `default` arm repeats the
  `NotStarted` case; `stateWord` (`:439`) and `ciWord` (`:455`) do the same,
  and the gobco report shows each last case true many times and never false.
  `exhaustive` (`.golangci.yml:87`) checks switch and map, so a missing enum
  case already fails lint and the default arms guard nothing.
- `internal/cli/pr.go:185` — the two dry-run lines of `offerLink` and
  `offerReviewStatus` (`:244`) never print; UX-127 makes them print, which
  closes this arm.
- `internal/messaging/messaging.go:120` — `Client.checkable` returns
  `ErrNoCredential` for `config.MessagingNone` and again in `default:`
  (`:122`); `markupFor` (`internal/messaging/post.go:267`) returns
  `slackMarkup()` for `config.KindSlack` and again in `default:`.
- `internal/wiring/forgecli.go:59` — `forgeProgram`'s `case
  forge.KindUnknown:` and `default:` (`:61`) both return `"", false`; the
  gobco report lists the `KindUnknown` condition as never evaluated, and
  DEBT-64 counts `forgeProgram` among the four no black-box test reaches.
- `web/src/features/review/ReviewPanel.tsx:32` — `ReviewPanel` returns null
  under `if (!snapshot)`, the file's only uncovered line; `BranchPanel`
  (`web/src/features/branch/BranchPanel.tsx:18`), `IssuesPanel`
  (`web/src/features/issues/IssuesPanel.tsx:22`) and `MessagingPanel`
  (`web/src/features/messaging/MessagingPanel.tsx:17`) carry the guard
  verbatim, each uncovered, while `SectionPanel`
  (`web/src/shell/SectionPanel.tsx:20`) returns the connecting state before
  any of the four renders.
- `internal/webserver/announce.go:34` — `server.Announce` guards
  `request.Body == nil`; so do `server.Commit`
  (`internal/webserver/commit.go:32`), `server.UpdateConfig`
  (`internal/webserver/config.go:98`) and `server.OpenPullRequest`
  (`internal/webserver/pullrequest.go:39`), never true in the gobco report,
  and `server.Checkout` (`internal/webserver/checkout.go:34`) and
  `server.CreateBranch` (`internal/webserver/branchcreate.go:40`) carry it
  as a dead first operand, never true there either. Every one of those
  bodies is `required: true` in `api/openapi.yaml:569` (and `:634`,
  `:425`, `:697`, `:471`, `:507`), and the strict handler sets
  `request.Body = &body` unconditionally after a decode
  (`internal/api/server.gen.go:2083`).
- `internal/webserver/errors.go:57` — `codeMeaning`'s `default` arm
  duplicates the `api.Internal` arm, `code == api.Internal` 14 times true
  and never false, where `ciState` (`internal/webserver/dto.go:182`) states
  the package's own rule: a map, not a switch, so there is no last-case arm
  gobco can never see.

The condition figure is held down where no test can raise it; the web's
review panel carries an uncovered line nothing reaches; the next panel and
the next handler copy the guard; and `codeMeaning` keeps the last-case arm
`ciState` says the package avoids with a map.

**One way to fix it.** Let each `default` be the one terminal return, or
drop it and let `exhaustive` guard the switch; delete the six nil guards and
dereference the body as `Stage` and `Unstage` already do, keeping the
empty-field checks on checkout and createBranch; build `codeMeaning`'s table
as a map keyed by `api.ProblemCode` as `ciState` is; and have
`SectionPanel` pass the snapshot's parts as props so no panel guards null.

**Done when.** `task cover:branch` no longer lists the three glyph and word
switches in `internal/cli/status.go`, `checkable` in
`internal/messaging/messaging.go`, `forgeProgram` in
`internal/wiring/forgecli.go`, or the body guards in
`internal/webserver/announce.go`, `internal/webserver/commit.go`,
`internal/webserver/config.go`, `internal/webserver/pullrequest.go` and
`internal/webserver/errors.go`; `internal/webserver/checkout.go` reads
`request.Body.Branch == ""` and `internal/webserver/branchcreate.go`
`request.Body.IssueKey == ""` with `TestCheckoutRejectsAnEmptyBranch` and
`TestCreateBranchRejectsAnEmptyIssue` still passing; the Internal arm exists
once; and the v8 summary lists no uncovered null return for the four panels
and none has a null check.

### DEBT-95 Service answers that exit 1 where the contract promises 3 or 5

Severity: medium · Confidence: read

Three answers a service gives reach the command line under a sentinel the
exit-family tables in `internal/cli/scriptable.go` do not list, or list
inconsistently, so a script keyed on the documented status is misled:

- `internal/jira/answer.go:75` — `Client.answerError` wraps the reason in
  `ErrRejected` alone and returns it (`:80`); only a 404 gets the dual
  identity of `notFoundError` (`:76`, `:86`), while `statusError`
  (`internal/jira/jira.go:233`) makes a bodiless 403 `ErrForbidden`.
  `configurationErrors` (`internal/cli/scriptable.go:324`) lists
  `jira.ErrForbidden` (exit 3) and nothing lists `jira.ErrRejected` (exit
  1), so a Jira 403 exits 3 when its body is empty and 1 when it carries a
  reason — the shape `TestAssignReportsJirasReason`
  (`internal/jira/assignee_test.go:61`) serves and asserts as `ErrRejected`
  (`:67`), never `ErrForbidden`.
- `internal/messaging/post.go:124` — `Client.postAsBot` turns every
  `ok:false` into `ErrPostRefused`, which `configurationErrors`
  (`internal/cli/scriptable.go:319`) lists in no family, so Slack's
  `invalid_auth`, `token_revoked`, `not_authed` and `account_inactive` exit
  1 from `announce`, while `Client.send`
  (`internal/messaging/messaging.go:159`) maps the same `ok:false` from
  `auth.test` to `ErrRejected` and exit 3.
- `internal/httpx/httpx.go:50` — `Unreachable` returns `ErrRedirected`
  without wrapping the caller's unreachable sentinel; `unreachableErrors`
  (`internal/cli/scriptable.go:346`) lists the three services' sentinels,
  `errUnreachable` and `httpx.ErrRateLimited` (`:347`) but not
  `httpx.ErrRedirected`, so `ExitStatus` (`:282`) falls to `exitFailure`,
  while `credentialOutcome` (`internal/cli/doctor.go:305`) maps the same
  error to `errCredentialRejected` (exit 3) and `transportFaults`
  (`internal/webserver/errors.go:148`) to `unreachable`.
  `docs/content/docs/scripting.md:25` says a script tells failures apart by
  the exit status and (`:35`) promises 5 for a service that did not answer,
  `docs/content/docs/configuration.md:119` describes the login redirect an
  SSO gateway sends, and `TestExitStatusDistinguishesFailureKinds`
  (`internal/cli/exitstatus_test.go:71`) has a rate-limited case and no
  redirect case.

A script that re-provisions on exit 3 never sees a token that died between
`doctor` and `announce`; one that retries on 3 after a login misses the 403
with a reason; one that waits on 5 behind an SSO gateway gets 1 and gives
up.

**One way to fix it.** Wrap the 403 reason in a type whose `Is` matches
`ErrForbidden` as `notFoundError` does for 404, then pick the one family; in
`postAsBot` classify Slack's auth error codes as `ErrRejected` and the rest
as `ErrPostRefused`; and have `httpx.Unreachable` wrap the caller's
sentinel around `ErrRedirected` — or list `ErrRedirected` in
`unreachableErrors` — so every surface classifies a redirect the same way.

**Done when.** A case in `internal/jira/answer_test.go` serving 403 with a
JSON reason asserts `errors.Is` holds for both `jira.ErrForbidden` and
`jira.ErrRejected` with the reason in the message, and an exit-status test
maps it to one family; a fake `chat.postMessage` answering
`{"ok":false,"error":"invalid_auth"}` yields `errors.Is(err, ErrRejected)`
and `ExitStatus` 3; and an exit-status case where the forge or Jira
answers a redirect exits 5 (or the 3 `doctor --online` gives, whichever the
maintainer picks), never 1.

### DEBT-99 `wiring` connects to the forge twice, over five clumped signatures

Severity: low · Confidence: read

`forgeConnection`'s comment (`internal/wiring/forge.go:19`) promises the
connection is "found once, on first use", but `forgeDeps`
(`internal/wiring/forge.go:31`) and `forgeIssuesDeps`
(`internal/wiring/forgeissues.go:56`) each wrap `connectForge` in their own
`onceConnected`, and `Deps` (`internal/wiring/wiring.go:76`) builds both
bundles, so a session without Jira holds both caches and resolves the
token twice — two `gh auth token` child processes, a failure remembered
by neither. The values that connection needs travel positionally through
`forgeDeps` (`internal/wiring/forge.go:28`; ctx, settings, where, timeout,
log), `forgeIssuesDeps` (`internal/wiring/forgeissues.go:53`; the same
five), `trackerDeps` (`internal/wiring/forgeissues.go:39`; ctx, cfg, where,
timeout, log), `connectForge` (`internal/wiring/forge.go:228`; ctx,
settings, remote, timeout, log) and `forgeTransport`
(`internal/wiring/forgecli.go:25`; six), past the "5+ positional params"
line in CLAUDE.md's catalog. The user-visible cost is one extra child
process in the terminal; the larger cost is the next forge-backed seam,
which adds a third cache, and the next value the connection needs, which
edits five signatures. DEBT-71 holds the package's other composition debt.

**One way to fix it.** A `forgeSetup` struct of settings, where, timeout and
log, and one `connect` built once in `Deps` and passed to both bundles and
to `connectForge`.

**Done when.** A wiring test with no token in the environment and a stand-in
`gh` answering `auth token` sees one such invocation across `Search` and
`FindPullRequest`, and `connectForge` and the two bundle builders take the
struct and `ctx` only.

### DEBT-101 `workflow status` never reads the store's `Announced`, so its last stage lags the spine

Severity: medium · Confidence: read

`newStatusCmd`'s long help (`internal/cli/status.go:47`) promises "the same
progress the interface's top row shows", but the command builds its
`progress.Work` without the announce memory the spine reads. The
merged-pull disagreement between `status` and the spine is UX-125.

- `internal/cli/status.go:191` — `seamsFor` wires seven seams, none from
  `conn.deps.Store`, and `gather` (`internal/cli/status.go:265`) builds
  `progress.Work` without setting `Announced`; the spine's `work`
  (`internal/tui/spine.go:94`) passes `Announced: m.announced()`, which
  `announced` (`internal/tui/messaging.go:196`) reads from this session or,
  from the store, an earlier one. After `workflow announce` (or an
  announcement from the interface), `status` and `status --json` print the
  messaging stage not started while the spine, seeded from the same store,
  prints it done.
- `internal/progress/progress.go:56` — the `Announced` field's comment
  still calls it "Session knowledge", and the `Work` comment (`:33`) says a
  one-shot command leaves it false, which predates the store's announce
  memory (f99e34a; `internal/store/announce.go`, wired into
  `loop.AnnounceMemory` at `internal/cli/announce.go:190`).

A script reading `status --json` after an announcement acts on a wrong last
stage. `internal/cli/status_test.go` never asserts the last stage after an
announcement.

**One way to fix it.** Give `statusSeams` the store's `Announced` and set
`Work.Announced` when the found pull's current moment is in it, as the
interface's `announced` does; reword the `Work` comment so only
`IssueSelected` and `PostPending` are session knowledge.

**Done when.** A status test whose `Store.Announced` holds the open pull
request at its ready moment prints the messaging stage done in the line and
in `--json`.

### DEBT-102 Token commands run at wiring, for commands that never reach the service

Severity: low · Confidence: read

`jiraDeps` (`internal/wiring/wiring.go:165`) calls `ResolveToken` inside
`Deps`, running `token_command` as a subprocess on every connect, and
`messagingDeps` (`internal/wiring/wiring.go:287`) does the same for the
messaging token, while the forge in the same package connects lazily through
`onceConnected` (`forgeIssuesDeps`, `internal/wiring/forgeissues.go:57`).
`connectAt` (`internal/cli/cli.go:384`) builds `wiring.Deps` for every
command, so `runReviewsCommand` (`internal/cli/reviews.go:56`), which uses
only `Forge.ReviewRequests`, runs `jira.token_command` anyway, and
`statusesOf` (`internal/cli/status.go:166`) calls `connectAt` per directory,
so `status DIR…` runs it once per directory. A token command that prompts (a
password manager with a biometric or passphrase prompt) fires on `workflow
reviews`, on every command that only touches git or the forge, once per
directory on `status`, and before the interface draws its first frame; a
slow command delays every command by its run time.

Both calls also discard `ResolveToken`'s error —
`settings.Token, _, _ = ResolveToken(…)`. For messaging, a `token_command`
that fails leaves the token empty while `hasToken`
(`internal/config/config.go:314`) still counts the set command as a token
and the mode stays bot; `postAsBot` (`internal/messaging/post.go:109`) then
sends a `Bearer` header with nothing after the word, and the Messaging pane
and `workflow announce` word a command that could not run as a refused
credential, in the service's own words. The gobco report shows the
`MessagingBot` arm at `internal/wiring/wiring.go:286` "56 times false but
never true".

**One way to fix it.** Resolve the Jira and messaging tokens on first use,
through the same `onceConnected` pattern the forge seams already use, so a
failed resolve is retried rather than remembered, and return the resolve
error, wrapped, from the first `Jira.Search` and the first `Messaging.Post`.

**Done when.** A wiring test with `jira.token_command` pointing at a script
that records each run sees zero runs after `Deps()` and after
`Forge.ReviewRequests`, and one after the first `Jira.Search`; and a wiring
test with `messaging.token_command` set to `false` sees `Messaging.Post`
return an error naming the token command.

### DEBT-103 The request log's write and close errors are dropped

Severity: low · Confidence: read

`record` (`internal/wiring/reqlog.go:88`) writes each outline with
`fmt.Fprintf` and drops its error; `.golangci.yml:76` excludes `fmt.Fprintf`
from `errcheck` by name, so its comment that "Writes to FILES are still
checked" does not hold for this file write, and `openRequestLog`
(`internal/cli/cli.go:425`) hands the log an `*os.File` and ignores the
close error too. A `--log` on a full disk records nothing and nobody is
told, so the bug report the log exists for arrives empty; the reqlog tests
write to a `strings.Builder`, so a failing writer is untested. The token
command's dropped error is DEBT-102's.

**One way to fix it.** Check the write once in `record` and keep the first
error on the log for the command line to report at close; and make the
`.golangci.yml` comment say which file writes are and are not covered.

**Done when.** A reqlog test with a writer that fails sees the failure
surfaced.

### DEBT-105 The contributor documents restate counts and names the tree has moved past

Severity: low · Confidence: read

The documents a contributor and a later session read first restate numbers
and names the tree has moved past. No gate reads any of them.

- `CLAUDE.md:101`, `ARCHITECTURE.md:14` and `FEATURES.md:54` — each pairs
  `CGO_ENABLED` with the same count: "the release binaries cross-compile to
  five platforms", "so it cross-compiles to five platforms", "because the
  release cross-compiles to five platforms". `RELEASE_PLATFORMS`
  (`Taskfile.yml:73`) names three GOOS/GOARCH pairs, mirrored by the binary
  table in `.github/workflows/release.yml:108`, and `CONTRIBUTING.md:208`
  already says so: "macOS (arm64), Linux (amd64) and Windows (amd64)".
- `CLAUDE.md:74` — the `task lint` row's parenthetical lists twelve checks;
  the `lint` task (`Taskfile.yml:290`) runs sixteen sub-tasks, and the row
  omits `lint:packagesize` (`Taskfile.yml:301`), `lint:goversion`,
  `lint:goroutines` and `gen:verify`. `CLAUDE.md:156` says the package-size
  gate runs in `task lint`, contradicting the row in the same file.
- `docs/content/docs/contributing.md:50` — the `task lint` row names nine
  checks and omits `lint:markdown`, `lint:toml`, `lint:filelength`,
  `lint:packagesize`, `lint:goversion`, `lint:goroutines` and `gen:verify`.
- `CLAUDE.md:498` — the never-print-a-secret rule names `slack.token`, a key
  the decoder refuses: `ErrSlackRenamed` (`internal/config/config.go:50`)
  says the "slack" block was renamed to "messaging", and the field is
  `Messaging.Token` (`internal/config/config.go:106`, `json:"token"`).
- `CLAUDE.md:9` — the opening line names Slack alone where the same file's
  layout row (`CLAUDE.md:43`) names "Slack, Teams, Discord or a plain
  webhook".
- `SECURITY.md:61` — the in-scope list is "`cmd/`, `internal/`, `scripts/`,
  `build/`, and every file under `.github/workflows/`": no `web/`, no
  `api/`, no loopback server and no browser, though `web/README.md:4`
  describes the app "served locally by `workflow --web`".
- `web/README.md:13` — "Running the cockpit takes two shells", with `task
  dev` and `task web:mockup` mentioned nowhere; the comment above `dev`
  (`Taskfile.yml:104`) names `web` and `web:ui` as the two shells, directly
  above the one-shell `dev` task (`Taskfile.yml:111`, "Run the whole cockpit
  in one shell"), and `web:ui`'s `desc` (`Taskfile.yml:145`) still says
  "Shell 2". `web:mockup` (`Taskfile.yml:130`, "Serve the web UI against
  mock data in one shell") is undocumented in the README.

A session reading CLAUDE.md learns a platform count the gate does not hold,
a lint list that omits four gates it will trip on, and a secret key it
cannot find in the code; a contributor tripped by `lint:goversion` or
`lint:packagesize` finds no mention on the published page; a reporter can
read SECURITY.md literally and not report a hole in the React client or the
contract; a frontend contributor opens two terminals and never learns about
`task dev` or the mock mode.

**One way to fix it.** Replace each count with a pointer at its source
(`RELEASE_PLATFORMS`, `task --list`) or the full list, at every site in one
commit; name `jira.token`, `messaging.token`, `messaging.webhook_url` and
`forge.token` — or "any `Secret` field" — in the secret rule and the four
services in the opening line; add `web/` and `api/` to SECURITY.md's scope
with the loopback server and the browser named; lead `web/README.md` with
`task dev`, add one line for `task web:mockup`, and move the two-shell
comment to the `web` and `web:ui` pair it describes.

**Done when.** `grep -n 'five platforms' CLAUDE.md ARCHITECTURE.md` prints
nothing, the `CGO_ENABLED=0` bullet in `FEATURES.md` names no platform
count (the phrase wraps there, so grep it with `-z` or read it), and `grep
-n 'slack.token' CLAUDE.md` prints nothing; every
sub-task under `Taskfile.yml`'s `lint` is named in, or referenced by,
CLAUDE.md's and `docs/content/docs/contributing.md`'s `task lint` rows;
CLAUDE.md's opening line names the services its `internal/messaging` row
names; `grep -n 'web/' SECURITY.md` matches inside the in-scope list; `grep
-n 'task dev' web/README.md` and `grep -n 'web:mockup' web/README.md` match;
and `grep -n 'two shells\|Shell 2' Taskfile.yml` prints nothing above the
`dev` task.

### DEBT-106 Two helpers the clients spell by hand past the rule of three

Severity: low · Confidence: read

Two helpers the clients spell by hand past the rule of three. Both sit
below `dupl`'s token threshold, so no gate sees them.

- The JSON write. Jira's marshal, `newRequest`, `Content-Type` triple
  appears in five write methods in two shapes: `Assign`
  (`internal/jira/assignee.go:29`), `LinkPullRequest`
  (`internal/jira/detail.go:248`) and `ApplyTransition`
  (`internal/jira/transitions.go:89`) set the header after a return-early
  check, while `AddComment` (`internal/jira/detail.go:211`) and `AddWorklog`
  (`internal/jira/worklog.go:47`) set it inside `if err == nil`. The next
  Jira write copies the triple a sixth time and picks a shape, and the
  one-sided `err == nil` guards the copies carry sit in DEBT-64's worklist
  (`internal/jira/worklog.go:46`, "2 times true but never false").
- The transport. `HTTPClient` in `internal/messaging/messaging.go:86`,
  `internal/jira/jira.go:98` and `internal/forge/client.go:101` each return
  `httpx.Client(timeout)` unchanged — the middle man CLAUDE.md's catalog
  names, written three times — while `onlineDoer`
  (`internal/cli/doctor.go:151`) already calls `httpx.Client` directly. A
  change to the transport's construction is a four-site edit.

**One way to fix it.** One `newJSONRequest(ctx, method, path, body any)`
in `jira` that marshals, calls `newRequest` and sets the header; and
`wiring` calling `httpx.Client` once with the three wrappers deleted.

**Done when.** `grep -c 'Content-Type'
internal/jira/{assignee,detail,transitions,worklog}.go` reads 0 with one
helper carrying it; and no package but `httpx` defines `HTTPClient`.

## The terminal interface

What is open here is a set of appliers and guards that read stale state or skip
a check the other surfaces make, two configuration sections nothing validates,
the composition the interface keeps beside `loop`'s, the hook seam with its
failure resolver and the `lefthook.yml` it writes, and a color read no black-box
test reaches; what it carries on purpose — the two composers' field handling
written twice, and the spine's color-only hue — is under [Deliberate
trade-offs](#deliberate-trade-offs-that-carry-a-cost).

### DEBT-107 A search answer for a view no longer shown replaces the list

Severity: medium · Confidence: read

`issuesLoaded.apply` (`internal/tui/issues.go:32`) settles whatever answer
arrives into the current list with no look at which view it answers. The
message carries the query — the `jql` field of `issuesLoaded`
(`internal/tui/issues.go:24`) — but only `Model.cacheIssues`
(`internal/tui/issues.go:47`) reads it, to file the page under its view.
`Model.nextIssueView` (`internal/tui/views.go:58`) switches the view and
starts a new search while the old one may still be out, so when the first
view's answer lands last, the pane titled for the second view lists the
first view's issues, and a following select or transition acts on an issue
the title says is not there. `detailLoaded.apply` and
`transitionsListed.apply` guard the same race by key; this applier does not.
It self-corrects on the next `r` or `v`, and any action lands on the key
actually shown, so the trigger is narrow.

**One way to fix it.** In `issuesLoaded.apply`, cache the page under
`msg.jql` as now, but settle it into the list only when `msg.jql` equals
`m.activeView().jql`.

**Done when.** A test in `internal/tui/views_test.go` holds `Init`'s batch,
presses `v`, then delivers the first view's answer, and the screen shows the
second view's issue and none of the first's.

### DEBT-108 The task switcher's dirty-tree guard reads a snapshot, not the tree

Severity: medium · Confidence: read

`branchPicker.choose` (`internal/tui/switchtask.go:179`) refuses a switch
from `m.changes.changes`, the Commits pane's list as it was last loaded.
`Model.openBranchPicker` (`internal/tui/switchtask.go:88`) lists the
branches and nothing else, and no tick or focus event in `internal/tui`
reloads the changes, so a file edited in another terminal after the last
load passes the guard and `git switch` carries the edit onto the other
branch — the very thing `errDirtyTree` says the guard prevents. The web
reads the tree at request time: `server.refuseADirtyTree`
(`internal/webserver/checkout.go:81`) calls `s.deps.Changes()` before it
decides. git still refuses an edit that conflicts, so the harm is a
non-conflicting change landing on another branch without a word.

**One way to fix it.** Have `openBranchPicker` (or `choose`) read
`Git.Changes` again before deciding, as the web's `refuseADirtyTree` does,
and refuse on the fresh answer.

**Done when.** A test in `internal/tui/switchtask_test.go` sets the world's
changes to a modified file after `live()`, presses `s` then `enter`, and the
switch is refused with no checkout call recorded.

### DEBT-109 A failed transitions listing for the review-status offer closes silently

Severity: medium · Confidence: read

When the status picker opens as a named-status offer after a pull request
and `Jira.Transitions` fails, `transitionsListed.apply`
(`internal/tui/picker.go:90`) tests `picker.offer.absent(msg.found)` before
it looks at `msg.err`. A failed read has an empty `found`, and
`statusOffer.absent` (`internal/tui/picker.go:128`) judges a named status
absent from an empty list, so the applier closes the overlay
(`internal/tui/picker.go:93`) and the error goes with it. The in-progress
and by-hand offers reach line 97, store `listErr`, and `statusPicker.view`
(`internal/tui/picker.go:260`) renders it. With `jira.review_status` set
and Jira unreachable, the link step reports and then the status offer
vanishes with no word: one failure told two ways, one of them not at all,
against the "one voice for failure" promise.

**One way to fix it.** Check `msg.err` before `absent()`: record `listErr`
and settle the picker so its view shows the failure, and close only on a
successful listing that lacks the status.

**Done when.** A test in `internal/tui/issuelink_test.go` with
`ReviewStatus` configured and `Transitions` returning `jira.ErrUnreachable`
(overriding the world's deps as
`TestOutsideARepositoryEachRepoPaneSaysSoAndOffersNoRepoKeys` does at
`internal/tui/branch_test.go:60`) shows "Jira did not answer in time" in
the Change status overlay.

### DEBT-110 Below 80 columns, clicking the issue being read selects another issue

Severity: medium · Confidence: read

`Model.pickIssue` (`internal/tui/detail.go:320`) maps a click in the detail
to a list row whenever the layout is collapsed; its only early return is
for a layout that is not. But in the collapsed layout `Model.issuesNarrow`
(`internal/tui/detail.go:177`) draws the issue's text, not the list, once
`viewing` is set, so there is no list row under the click, and `pickIssue`
still sets `issues.selected` from the clicked line
(`internal/tui/detail.go:329`) while `viewing` stays true. At 79 columns,
press `enter` to read one issue, click the second line of its description,
and the detail switches to whichever issue sits at that row; a following
`t`, `c` or `a` acts on the issue the click chose. Mouse capture is on by
default (`Default` sets `UI{Mouse: true}`, `internal/config/config.go:192`),
and the narrow case of `TestClickingPicksAnIssue`
(`internal/tui/mouse_test.go:96`) is width 80 with no `enter`, so
collapsed-plus-viewing is untested.

**One way to fix it.** Return early from `pickIssue` when
`m.issues.viewing`, since no list is drawn to pick from.

**Done when.** A test in `internal/tui/mouse_test.go` at 79x24 presses
`enter`, clicks row 3 of the detail, and the screen still shows the first
issue's description.

### DEBT-111 After writing `lefthook.yml` the pane still nags and `g` reopens the offer

Severity: medium · Confidence: read

`hooksWritten.apply` closes the offer and notices, but never clears
`m.hookgen.hooks`, and the only write to that field happens in
`hooksFound.apply`, which runs once from `Init`. So for the rest of the
session the Commits pane keeps saying a hook is unmanaged, keeps offering
`g`, and a second `enter` is refused with "file already exists" — a
refused write for something the user just did. The same staleness holds
for a `lefthook.yml` written outside the interface: `r` on Commits reloads
the changes and the branch, not the hooks. `docs/content/docs/usage.md:282`
("Existing git hooks") promises the hint only while there is no lefthook
configuration, and `TestExistingHooksAreOfferedALefthookConfiguration`
(`internal/tui/hookgen_test.go:71`) stops at the success notice.

- `internal/tui/hookgen.go:156` — `hooksWritten.apply` on success only
  closes the overlay and notices; `m.hookgen.hooks` is left set.
- `internal/tui/hookgen.go:49` — `hooksFound.apply`, the package's only
  write to `m.hookgen.hooks`, reached only from `Init`'s `findHooks`
  (`internal/tui/tui.go:161`).
- `internal/tui/commits.go:225` — `Model.handleCommitsKey` keeps `g` live
  on `len(m.hookgen.hooks) > 0` and reopens the offer.
- `internal/tui/commits.go:228` — the refresh case of `handleCommitsKey`
  batches `loadChanges` and `loadBranch` but not `findHooks`.
- `internal/hooks/generate.go:375` — `Write` refuses the second write with
  `fs.ErrExist`.

**One way to fix it.** `hooksWritten.apply` clears `m.hookgen.hooks` on
success (or re-runs `findHooks`), and the Commits refresh key batches
`findHooks` beside `loadChanges` and `loadBranch`.

**Done when.** A test writes the configuration, then asserts the Commits
detail no longer shows the hint and the footer no longer lists `g`; a
second test writes `lefthook.yml` into the world, presses `r` on Commits,
and sees the same.

### DEBT-112 The Review pane never offers `n` after a merged pull request

Severity: medium · Confidence: read

`Model.canOpenPullRequest` (`internal/tui/review.go:355`) gates `n` on
`!m.review.found`, and a find returns the merged pull request when no open
one exists (`pickPull`, `internal/forge/pulls.go:213`), so a branch whose
earlier pull request merged is never offered `n`. `refuseAnOpenPull`
(`internal/loop/pull.go:113`), which the command line's `runPR`
(`internal/cli/pr.go:116`) and the web's `composePullRequest`
(`internal/webserver/pullrequest.go:105`) compose through, refuses only
`pull.IsOpen()`, and its comment says why: a merged one's branch may carry
new commits worth a fresh pull request. Once the branch does carry
unpushed commits, `canFinish` (`internal/tui/finish.go:39`) withholds `F`
too on `HasUnpushedWork`, leaving the pane with no way forward: `workflow
pr` opens a new pull request where the terminal shows "merged" with neither
`n` nor `F`. The two surfaces disagree on the shared layer's contract.

**One way to fix it.** Gate `n` on `!m.review.found ||
!m.review.pull.IsOpen()`, as `refuseAnOpenPull` does, and let
`mergedDetail` say so. The same function's `err == nil` gate is UX-103; fix
both in one change.

**Done when.** A test in `internal/tui/review_test.go` with
`pull.State = forge.StateMerged` and a commit on the branch sees "n open
pull request" in the footer and opens one.

### DEBT-113 The pull request composer titles from the listed issue, not Jira

Severity: medium · Confidence: read

`Model.openPullRequestComposer` reads the issue summary from the loaded
issue list — `m.issues.find(issueKey)` (`internal/tui/prcomposer.go:114`)
returns a zero `jira.Issue` when the key is not listed — and passes
`issue.Summary` to `convention.PullRequestTitleFrom`
(`internal/tui/prcomposer.go:119`). With `pull_request.title_source =
issue` and a branch whose issue is not in the list (assigned to someone
else, in another view, still loading), `titleFromIssue`
(`internal/convention/pullrequest.go:54`) sees an empty summary and
silently selects the oldest commit subject. The command line and the web
read the issue from the tracker: `issueSummary`
(`internal/loop/loop.go:40`) calls the `Issue` seam. Same branch, same
configuration, `workflow pr` proposes "PROJ-500: Summary" and the terminal
proposes a commit subject, with nothing on screen saying the issue was not
consulted — though the Branch pane already knows the case ("not among your
open issues"). `Deps.Jira.Issue` is bound but unused in
`internal/tui/prcomposer.go`, and the one test of the source,
`TestTheComposerTakesItsTitleFromTheConfiguredSource`
(`internal/tui/prbase_test.go:59`), uses an issue that is in the world's
list. The wrong title is shown in an editable composer before anything
goes outward.

**One way to fix it.** Read the summary through `Deps.Jira.Issue` (the
seam `ComposePull` uses) when the list does not hold the key, or compose
through `loop.ComposePull`'s draft and fill the composer from it.

**Done when.** A test with `cfg.PullRequest.TitleSource = "issue"` and a
branch naming an issue absent from the world's list sees the issue's
summary as the title.

### DEBT-114 The merge picker opens over whatever overlay is open

Severity: medium · Confidence: read

`Model.startMerge` (`internal/tui/merge.go:46`) returns the model untouched
with only a command: no overlay, no `sendState`, and the keyboard still
reaches the panes while `MergeMethods` is out. `mergeMethodsLoaded.apply`
(`internal/tui/merge.go:69`) then assigns `m.overlay` with no check of what
is open, so an overlay opened meanwhile — the pull request editor, the
help — is replaced and its input lost. The `applier` contract
(`internal/tui/overlay.go:62`) says a result for an overlay that has since
closed does nothing, because the message checks what is open first; the
sibling loaders `transitionsListed` and `branchesListed` open their overlay
at once and check it is still open. Press `M`, then `e` and type a new
title while the methods are still out: when they answer, the editor
vanishes under the merge picker. Nothing on screen says the methods are
being read, so a slow forge also invites a second `M` and a second read.
UX.md's promises table (the "a refused change must never go unseen" row)
counts 13 overlays that guard a request in flight, each holding a
`sendState`; this read holds none, so the merge picker's methods read is
outside the count.

**One way to fix it.** Have `startMerge` open the picker at once in a
"reading methods…" `sendState` and let `mergeMethodsLoaded.apply` fill it
through `keepOpenWith`, as every other overlay result does.

**Done when.** A test dispatches `M`, presses `e`, then delivers the
methods, and the editor is still open.

### DEBT-115 A `ui.keys` override crashes `jump-to-pane` and leaves the wheel dead in every picker

Severity: medium · Confidence: read

Two places assume the default key where `ui.keys`, which `CheckKeys`
accepts and `docs/content/docs/configuration.md:405` ("Rebinding keys")
lists, can rebind it. `Model.handleGlobalKey` derives the pane from the
pressed key's first byte, so an override of `jump-to-pane` to a non-digit
indexes past the pane table and the next `View` panics: with
`{"jump-to-pane": "f12"}` — the very `freeKey` that
`internal/tui/help_test.go:16` uses to prove every action is accepted —
pressing `f12` sets focus to `pane('f' - '1')`, 53, and `pane.title`
indexes a six-element array with it. The wheel synthesizes `tea.KeyDown`
and `tea.KeyUp` for overlays, which the pickers match against
`m.keys.down` and `m.keys.up`, so a rebound `down` leaves the wheel dead
in every picker. The nothing-staged guidance that names `space` is UX-96's.

- `internal/tui/tui.go:241` — `Model.handleGlobalKey` computes
  `pane(msg.String()[0] - '1')` under a comment (`:239`) that assumes only
  digits match.
- `internal/tui/keys.go:150` — `helpBuilder.bindingFor` replaces the whole
  key list with the override, so `paneNumbers()` is gone.
- `internal/tui/panes.go:35` — `pane.title` indexes `titles[p]` on a
  `[paneCount]string`; index 53 panics.
- `internal/tui/keys.go:293` — `CheckKeys` checks only unknown actions and
  conflicts; a non-digit for `jump-to-pane` passes.
- `internal/tui/mouse.go:100` — `Model.wheel` builds
  `tea.KeyPressMsg{Code: tea.KeyDown}` rather than the bound key.
- `internal/tui/picker.go:319` — `statusPicker.handleKey` matches the
  synthesized key against `m.keys.down`; so do
  `branchPicker.handleKey` (`internal/tui/switchtask.go:158`),
  `statusPicker.handleFormKey` (`internal/tui/fields.go:240`) and
  `fixupPicker.handleKey` (`internal/tui/picker.go:466`).

**One way to fix it.** Refuse an override of `jump-to-pane` in `CheckKeys`
(its keys are derived from `paneCount` and cannot be one key) or map the
pressed key back to its index in the binding's key list, and give
`pickList` a `moved(step)` path the wheel calls through a small overlay
method.

**Done when.** A test with `{"jump-to-pane": "f12"}` either gets an error
from `CheckKeys` or presses `f12` on the live model and sees the screen
unchanged, with no panic, and `TestTheWheelMovesAnOverlaysList`
(`internal/tui/mouse_test.go:66`) passes with `cfg.UI.Keys` rebinding
`down` and `up`.

### DEBT-116 Dry run's backstop skips `Forge.EditPullRequest` and the store

Severity: low · Confidence: read

`errDryRun` promises that a call which forgot its own dry-run guard fails
safe, reaching no service. `heldBackServices` keeps that promise for
`CreatePullRequest`, `Rerun`, `Merge`, `Post`, `Run` and `Write`, but not
for `EditPullRequest`, so the pull request editor relies on its call-site
guard alone: a future edit path that drops the `if m.dryRun` in
`prEditor.save` would edit a pull request for real under `--dry-run`, and
no test enumerates the write seams against `heldBack`. And `heldBack`
holds back Jira, git, the forge, the messaging service and the hook writes
but never `deps.Store`, so under `--dry-run` each first issue page is still
written through `Store.CacheIssues`, where `docs/content/docs/usage.md:274`
says a dry run "writes nothing" and the web server refuses even to open the
store. The world records "cache N" and `TestASearchCachesTheIssueList`
(`internal/tui/issuecache_test.go:45`) asks for it on the live path, but no
dry-run test asserts the call is absent. The trade-off bullet on the web's
dry run covers the web, not the terminal's store writes.

- `internal/tui/dryrun.go:17` — `errDryRun`'s comment: a call that forgot
  its own guard "reaches no service".
- `internal/tui/dryrun.go:113` — `heldBackServices` replaces
  `CreatePullRequest`, `Rerun`, `Merge`, `Post`, `Run` and `Write`; not
  `EditPullRequest`.
- `internal/tui/deps.go:125` — `ForgeDeps` declares `EditPullRequest`
  among the forge write seams.
- `internal/tui/preditor.go:138` — the `if m.dryRun` in `prEditor.save`
  that is the editor's only guard.
- `internal/tui/dryrun.go:25` — `heldBack` touches `Jira`, `Git` and the
  services, never `deps.Store`.
- `internal/tui/issues.go:47` — `Model.cacheIssues`, called from
  `issuesLoaded.apply` with no dry-run guard, writes the first page under
  `--dry-run`.
- `docs/content/docs/usage.md:274` — "Dry run": `workflow --dry-run`
  "writes nothing".
- `internal/webserver/commit.go:152` — the web's contrary rule on
  `server.learnedScope`: "A dry run never reads it: the store makes its
  directory and opens its database even to read."

**One way to fix it.** Replace `deps.Forge.EditPullRequest` in
`heldBackServices` like its siblings, and hold back `StoreDeps.CacheIssues`,
`RecordScope` and `RecordAnnounce` in `heldBack` (or state the cache
exception in usage.md's Dry run section).

**Done when.** A test builds a dry-run model whose write seams
(`EditPullRequest` included) record calls, drives the editor to save and
the list to search, and sees no forge call and no "cache" call in the
world; or usage.md names the cache as the one thing dry run keeps writing.

### DEBT-117 The footer offers a dead key and hides a live one

Severity: low · Confidence: read

Two sites break the footer's contract in opposite directions.
`Model.commitsKeys` lists `stageAll` whenever a change is selected and
`Stage` is wired, but `Model.stageAll` returns without a notice when
`loop.Stageable` is empty, so with every file staged the footer shows a key
that does nothing — on the default world (one file, wholly staged) it reads
"a stage all", pressing `a` changes nothing and says nothing, which
`TestStageAllWithEverythingStagedStagesNothing`
(`internal/tui/commits_test.go:146`) pins. `commit`, by contrast, is
offered only when something is staged. The announcement preview omits `w`
from its footer when the pull request reports no CI, as
`docs/content/docs/usage.md:325` promises, yet `handleKey` still routes `w`
to `postWhenGreen`, which never reads `noCI`: pressing it out of habit
closes the preview with "will announce … once CI passes", marks the spine's
last stage in flight, blocks quitting behind the quit guard and polls
`CheckStatus` on every interval for checks that will never report. It is
recoverable by announcing with `enter`, so friction rather than a wrong
result. UX-98 records more footer keys that break the same contract (dead
`b` and `r`, and two scroll keys the footer omits); fix them together.

- `internal/tui/commits.go:192` — `Model.commitsKeys` appends
  `m.keys.stageAll` on a selected change and a wired `Stage`, not on
  anything being stageable.
- `internal/tui/commits.go:195` — the sibling gate `commit` already has:
  `m.changes.staged() > 0`.
- `internal/tui/commits.go:285` — `Model.stageAll` returns silently when
  `loop.Stageable` is empty.
- `internal/tui/messagingpreview.go:78` — `messagingPreview.footer` reads
  `noCI` (declared at `:43`) only to hide `w`.
- `internal/tui/messagingpreview.go:108` — `messagingPreview.handleKey`
  routes `w` to `postWhenGreen`, which gates on the moment and `CIPassed`
  only (`:148`), never on `noCI`.
- `docs/content/docs/usage.md:325` — "Limits": "`w` needs checks to wait
  for" and the preview "does not offer it"; the key still acts.

**One way to fix it.** Offer `stageAll` only when
`loop.Stageable(m.changes.changes)` is non-empty, as `commit` is gated on
`staged()`; return early in `postWhenGreen` under the same condition
`footer` uses (`p.noCI`).

**Done when.** A screen test with every file staged refuses "a stage all"
in the footer; a test with `w.ci = []forge.CI{{State: forge.CINone}}`
presses `5`, `p`, `w` and sees no "will announce" notice, the footer still
offering `p`, and no further "ci" call recorded past the horizon.

### DEBT-118 A failed find on a branch reload strands a queued announcement

Severity: low · Confidence: read

`pullFound.apply` (`internal/tui/review.go:92`) replaces the found review
through `beginReview` whenever the answer is not for the same pull request
number — and an answer carrying an error has `found` false, so it always
is. `beginReview` bumps `reviewsBegun`, which is the chain `ciPoll.apply`
(`internal/tui/review.go:201`) checks before polling again, and the applier
returns before `checkCI`. Every branch reload batches `findPullRequest`
(`branchLoaded.apply`, `internal/tui/branch.go:57`), so with `w` pressed
and CI running, a commit, a push or `r` on Branch whose `FindPullRequest`
fails transiently ends the polling: the queued post is neither sent nor
dropped, the Review rail shows the failure, and `Model.messagingState`
(`internal/tui/messaging.go:155`) keeps saying "announces when CI passes"
for a post nothing will send until a later find succeeds.

**One way to fix it.** In `pullFound.apply`, keep the found review when
the answer is an error for the same branch (record `err` beside it, as the
same-number branch does) so the chain and the queued post survive a blip.

**Done when.** A test queues a post, delivers a `pullFound` with
`ErrUnreachable`, then a green CI, and sees the post sent.

### DEBT-119 `tui.Run` reads `NO_COLOR` itself, which keeps its branch untestable

Severity: low · Confidence: read

`Run` (`internal/tui/tui.go:142`) calls `os.Getenv("NO_COLOR")` inside the
package rather than taking the value or the decision from its caller, which is
why the color branch there and the error return at `:149` are reachable by no
black-box test: DEBT-64's "no black-box test reaches without changing the code"
list names both. The repository already has the other pattern: `editorDeps`
(`internal/wiring/wiring.go:460`) passes `os.Getenv` into `editor.Edit` as a
parameter, and CLAUDE.md's dependency-inversion example is `config.Load` taking
its directories rather than reading the environment. The cost is that the
`NO_COLOR` path and the `WithoutColor` call it makes are exercised only by hand.
This is the design shortcut behind one of DEBT-64's accepted-unreachable
conditions, not a restatement of that entry.

**One way to fix it.** Have the command line decide
(`cfg.UI.DrawColor(os.Getenv("NO_COLOR"))`) and pass a model already
`WithoutColor` into `Run`, or give `Run` a `getenv` parameter as
`editor.Edit` takes one, so the branch is a plain function of its inputs.

**Done when.** A test builds the model the root command hands to its
`RunInterface` fake with `NO_COLOR` set and sees the no-hue styles, and
`task cover:branch` no longer lists `internal/tui/tui.go:142` as never
evaluated.

### DEBT-120 The help overlay's scroll is unclamped and its column split unbalanced

Severity: low · Confidence: read

`helpOverlay.handleKey` adds a half page to `h.scroll` on every `pgdn` or
`j` with no upper bound, and only the draw clamps, so after paging past
the end several `pgup` presses move nothing: at 120x20 nine `pgdn` presses
leave the offset at 72 while the draw shows the same last page, and the
first `pgup` lands on 64, still past the end, so nothing moves until the
sixth press. `TestTheHelpScrollsBackUp` (`internal/tui/edges_test.go:379`)
presses `j`, `j` then `k`, `k` and never overshoots. Beside it,
`helpColumnSplit`'s comment says the split is chosen so the two columns
come out close to the same height, but that stopped holding as keys were
added: counted from the bindings `internal/tui/keys.go` declares, less the
one with no help text, the seven groups hold 7, 11, 11, 6, 12, 3 and 6
lines, so a split after group 4 gives 42 lines against 26 and a split
after group 3 gives 34 against 34. At 120x40 the help says "more below"
and needs a scroll a balanced layout would not.

- `internal/tui/help.go:78` — `helpOverlay.handleKey` does
  `h.scroll += m.halfPage()` with no clamp; `:80` only floors at 0.
- `internal/tui/help.go:49` — `helpOverlay.view` clamps only at draw,
  through `scrolled`.
- `internal/tui/tui.go:264` — `Model.scrollDetail`, the pane path that
  clamps before and after through `firstShown`, the pattern the help
  lacks.
- `internal/tui/render.go:202` — `helpColumnSplit` is 4 under a comment
  (`:200`) whose balance no longer holds; `helpColumn` (`:219`) is what
  renders the groups it divides.

**One way to fix it.** Clamp `h.scroll` through `firstShown` against the
fitting content's line count before and after each move, as
`scrollDetail` does; set the split to 3, or compute it from the group
lengths so it stays balanced as bindings are added.

**Done when.** A test opens the help at 120x20, presses `pgdown` nine
times then `pgup` once, and the first help line shown differs from before
the `pgup`; a test opens the help at 120x40 and sees "Everywhere" and no
"more below", or asserts the two columns differ by at most a few lines.

### DEBT-123 Resolving hook failures walks the whole work tree per place, in `Update`

Severity: medium · Confidence: read

`editor.Resolve` falls back to a full `filepath.WalkDir` of the checkout,
skipping only `.git`, for every place a tool printed that is not a file as
printed, and the interface calls it once per location synchronously inside
`runFinished.apply`, on the update loop. A failing `go test` hook prints
package-relative places ("run_test.go:12"), each of which costs one walk
of the entire checkout — `node_modules`, `tmp` and all — on the Bubble Tea
goroutine, so ten such lines freeze the interface for ten walks before the
run overlay shows its failures. `hooks.Failures` dedupes by file, line and
column only, and nothing caches between places.

- `internal/editor/editor.go:212` — `Resolve` calls `matchesBelow` for
  every place `Locate` rejects.
- `internal/editor/editor.go:228` — `matchesBelow` runs
  `filepath.WalkDir` over the whole root.
- `internal/editor/editor.go:233` — the walk in `matchesBelow` skips only
  `.git`.
- `internal/tui/deps.go:220` — `Deps.resolvedFailures` calls `Resolve`
  once per `Location`, with no cache.
- `internal/tui/run.go:158` — `runFinished.apply` runs the resolution
  synchronously in `Update`.

**One way to fix it.** Walk once per run, index files by their trailing
path, skip directories git ignores (or at least `node_modules` and `tmp`),
and do it in the `tea.Cmd` that delivers `runFinished` rather than in
`Update`.

**Done when.** A test resolving N package-relative places against a tree
with a counting fs sees one walk, and a screen test shows the run
overlay's failures without the walk running in `Update`.

### DEBT-124 `Hooks.Existing` answers configured=true when it cannot read the hooks dir

Severity: low · Confidence: read

`hookDeps` (`internal/wiring/wiring.go:419`) returns `nil, true` from
`Existing` when `HooksDir` fails, using the bool the seam documents as
"whether the repository already configures lefthook" (`HookDeps.Existing`,
`internal/tui/deps.go:191`) to mean "offer nothing", and
`TestTheHookSeamsOfferNothingOutsideARepository`
(`internal/wiring/hooks_test.go:97`) pins the contradictory answer with
`!configured`. The contract and the answer disagree; a caller that showed
"lefthook is configured" from this bool would lie outside a repository. It
works only because `hooksFound.apply` (`internal/tui/hookgen.go:44`)
happens to treat `configured` and an empty list alike — a sentinel with
two meanings, which CLAUDE.md's catalog names.

**One way to fix it.** Return `nil, false` on the error — an empty hook
list already yields no offer in `hooksFound.apply` — and drop the test's
expectation that `configured` is true.

**Done when.** `TestTheHookSeamsOfferNothingOutsideARepository` asserts
`found` is empty and `configured` is false.

### DEBT-125 The interface composes what `loop` composes once: announcement, draft, memory

Severity: low · Confidence: read

`internal/loop` says it composes the developer loop once for every
surface, but the interface builds the `messaging.Announcement` literal
field for field beside `loop.ComposeAnnouncement`, calls
`convention.PullRequestTitleFrom` and `convention.PullRequestBody` with the
arguments `loop.draft` passes, and keeps its own posted-moment list and
post-then-record beside `AnnounceMemory.Holds` and `Deliver` — over a store
seam typed `tui.AnnouncedPost` (`Moment int`) that the command line adapts
to `loop.Announced` (`Moment messaging.Moment`) by hand. A new
announcement field (FEAT-81's thread timestamp) is added in two places or
the interface's post differs from the command line's and the web's; giving
the command line and the web a template choice (UX-62, UX-88) means
changing `loop.draft` and hoping it still matches `prcomposer`; a moment
is converted between a typed value and a bare int twice, and the
interface's "already announced" logic must be kept in step with `Holds` by
hand. FEAT-84 would need a third adapter from `webserver.Deps`.

- `internal/tui/messaging.go:133` — `Model.announcement` builds the
  ten-field `messaging.Announcement` literal.
- `internal/loop/announce.go:70` — `ComposeAnnouncement` builds the same
  literal from seams.
- `internal/loop/loop.go:4` — the package comment of `loop`: "composes the
  developer loop once, for every surface".
- `internal/tui/prcomposer.go:119` — `openPullRequestComposer` calls
  `convention.PullRequestTitleFrom` with `loop.draft`'s arguments.
- `internal/tui/prcomposer.go:203` — `prComposer.withTemplate` calls
  `convention.PullRequestBody` with `loop.draft`'s arguments.
- `internal/loop/pull.go:127` — `draft`, the loop's own title and body
  proposal.
- `internal/tui/deps.go:183` — `AnnouncedPost` carries `Moment int` at the
  store seam.
- `internal/loop/announce.go:147` — `Announced` carries
  `Moment messaging.Moment` in the loop.
- `internal/cli/announce.go:198` — `announceMemory` converts between the
  two shapes by hand.
- `internal/tui/messaging.go:199` — `Model.announced` reimplements
  `AnnounceMemory.Holds` over its own posted list.
- `internal/loop/announce.go:165` — `AnnounceMemory.Holds`, the seam the
  interface does not use.

**One way to fix it.** Split `loop.ComposeAnnouncement` into a pure
value-level compose the interface calls with what it holds and the
seam-reading wrapper the command line and the web keep; expose a
value-level `loop.Draft(subjects, key, summary, url, template,
titleSource)` both `loop.draft` and the composer call; declare
`StoreDeps.Announced` and `RecordAnnounce` over `loop.Announced` and have
the interface hold a `loop.AnnounceMemory`.

**Done when.** `internal/tui` contains no `messaging.Announcement` literal
and `internal/tui/prcomposer.go` calls no `convention.PullRequest*`
function (grep); `tui.AnnouncedPost` is gone and
`internal/cli/announce.go`'s `announceMemory` adapter is deleted; the
interface's announcement text in a screen test equals `loop`'s for the
same inputs.

### DEBT-126 Generated `lefthook.yml` header claims jobs were converted under Verbatim

Severity: low · Confidence: read

`generate` (`internal/hooks/generate.go:125`) writes one `HeadComment` for
both modes — "Hooks it could run as jobs are jobs; the rest run from
.lefthook as scripts" (`:126`) — but `Verbatim`
(`internal/hooks/generate.go:119`) calls `generate(hooks, false)` and
converts nothing. A user who chose to keep every hook as its script
commits a `lefthook.yml` whose first lines say the opposite about how it
was made. It hurts the interface's hook offer (`g`), the only path that
writes the file; `TestVerbatimKeepsEveryHookAsItsScript`
(`internal/hooks/generate_test.go:198`) does not assert the header.

**One way to fix it.** Word the header per mode, with Verbatim's saying
every hook runs as the script it was.

**Done when.** `TestVerbatimKeepsEveryHookAsItsScript` asserts the header
does not claim jobs.

## The web

What is open here is the answers the server gives when a read or a write
fails, the reads it repeats on every frame, the contract's prose, and the
rules the browser derives for itself; what the web's gates still lack — an
end-to-end run that drives a write — is DEBT-65, with the other gates below.

### DEBT-127 A merged pull request reads "Ready for review" on the web

Severity: medium · Confidence: read

The wire `PullRequest` carries no state, so the web cannot tell a merged
pull request from an open one, though the forge hands it both and every
other consumer branches on `State`. Finishing the branch from the web, which
would close the window, is FEAT-79.

- `api/openapi.yaml:1301` — `PullRequest`'s `required` list is number,
  url, title, draft, approvals, changes_requested and mergeable; no state
  field.
- `internal/webserver/dto.go:134` — `pullDTO` maps `forge.PullRequest`
  onto the wire and drops `State`.
- `internal/forge/pulls.go:213` — `pickPull` returns the merged pull
  request with found true when no open one exists, as `IsOpen`'s comment
  at `internal/forge/pulls.go:89` warns callers.
- `internal/cli/cli.go:296` — `WebDeps` wires `FindPull` to the raw
  `FindPullRequest`, so the web sees a merged pull as found.
- `internal/webserver/stream.go:188` — `snapshotReview` passes that found
  through to `review` unchanged.
- `internal/webserver/handlers.go:182` — `server.review` calls `CheckCI`
  for any found pull, merged included, on every snapshot; the terminal's
  `checkCI` (`internal/tui/review.go:105`) returns early unless
  `State == StateOpen`.
- `web/src/features/review/ReviewPanel.tsx:97` — `PullRequestSummary`'s
  State row is `pull.draft ? 'Draft' : 'Ready for review'`, the only two
  values it can show.
- `docs/content/docs/web.md:94` — "### Review" promises the section shows
  the pull request's state.

From the moment a pull request merges until the branch is finished, the
browser says State: Ready for review, shows a Mergeable row ("Mergeability
unknown" on GitHub, whose `githubFind` reads mergeability only while open,
`internal/forge/github.go:99`; "No conflicts" is possible on GitLab, whose
`gitlabMerge.pullRequest` maps `merge_status` for any state,
`internal/forge/gitlab.go:42`) and lists CI checks read against the merged
head. A user could wait on a review that already happened. The terminal's
`reviewRail` says "merged" instead (`internal/tui/review.go:263`),
`gatherReview` (`internal/cli/status.go:286`) treats a merged pull as no
open review, and `momentOf` (`internal/loop/announce.go:115`) never asks CI
about one. The server also spends one forge request per stream tick asking
CI about a pull that has no live CI. No test in
`internal/webserver/review_test.go` or
`web/src/features/review/ReviewPanel.test.tsx` builds a merged pull
(neither mentions `StateMerged` or "merged", by grep).

**One way to fix it.** Add a `state` enum (`open`, `merged`) to the wire
`PullRequest`, map `forge.PullRequest.State` in `pullDTO` and regenerate
both clients (`task gen`, `yarn gen`); have `server.review` skip `CheckCI`
for a pull that is not open, as the terminal does, and have the State row
say "Merged" and omit the CI section.

**Done when.** `web/src/features/review/ReviewPanel.test.tsx` renders a
snapshot whose pull is merged and finds the text "Merged" and no "Ready for
review"; an `internal/webserver/review_test.go` case with a merged pull
records no `CheckCI` call; `task gen` and `yarn gen` leave no diff.

### DEBT-128 Three mock fixtures show a shape the server never sends

Severity: low · Confidence: read

The `VITE_MOCK` build that `task web:mockup`, the layout and a11y specs and
the audit's screenshots use carries three values the real server would
never send, so a reader of the mockup judges an inconsistency that exists
only in the fixture, or learns a placeholder the product would render
literally. It is the fake that cuts a corner.

- `web/src/dev/mockIssues.ts:18` — `mockIssueDetail` sets
  `assignee: 'ana.lopez'`, a username, beside `reporter: 'Ana Lopez'` at
  `web/src/dev/mockIssues.ts:17`; `toIssueDetail`
  (`internal/jira/detail.go:182`) fills both from `DisplayName`.
- `web/src/dev/mockConfig.ts:47` — `mockConfig.branch` sets
  `template: '{type}/{key}-{slug}'`; `BranchNaming.Name`
  (`internal/convention/branch_naming.go:65`) replaces only `{prefix}`,
  `{key}` and `{slug}`, so `{type}` would stay in the branch name. Settings
  draws that value directly under `BranchFieldset`'s hint "Uses {prefix},
  {key} and {slug}; must contain {key}."
  (`web/src/features/settings/fieldsets/BranchFieldset.tsx:12`).
- `web/src/dev/mockConfig.ts:27` — `mockConfig.messaging` sets
  `announcement: 'Opened {pr} for {issue}'`; `Messaging.Announcement`'s doc
  comment (`internal/config/config.go:123`) names the seven placeholders —
  {author}, {noun}, {title}, {url}, {key}, {summary}, {issue_url} — and
  `Announcement.rendered` (`internal/messaging/post.go:346`) substitutes
  only those, so `{pr}` and `{issue}` would post literally. Beside it,
  `previewAnnouncement` (`web/src/features/messaging/announceApi.ts:9`)
  answers the built-in wording rather than that template's rendering.

A designer reading the mock Issues detail sees "Reporter Ana Lopez" over
"Assignee ana.lopez" and reads a product inconsistency that is not there; a
reviewer of Settings learns `{type}` from a value drawn under a hint that
contradicts it; anyone copying the mock announcement's syntax into a real
file gets a literal post, and the mock Messaging section previews a text
the mock template could not produce.

**One way to fix it.** `assignee: 'Ana Lopez'` in `mockIssueDetail`; the
documented default `'{prefix}/{key}-{slug}'` in `mockConfig.branch`; an
announcement written in the documented placeholder set in
`mockConfig.messaging`, with the mock preview in
`web/src/features/messaging/announceApi.ts` being that template rendered
with the mock snapshot's values.

**Done when.** The mock Issues detail shows a display name under Assignee;
`mockConfig.branch.template` contains no placeholder outside `{prefix}`,
`{key}` and `{slug}`; `mockConfig.messaging.announcement` contains only the
seven documented placeholders, and the mock preview text equals that
template rendered with the mock snapshot's values.

### DEBT-129 Announce and draft tell a forge outage as "no pull request"

Severity: medium · Confidence: read

`server.announcement` and `server.composePullRequest` reduce the loop's
error to a bool, so a `Branch` or `FindPull` read that fails is answered
409 "there is no pull request to announce" or "there is nothing to open a
pull request for" instead of being classified through `fault`, as
`GetReview` and `LinkPullRequest` already classify the same seam failures
(502 for an unreachable forge).

- `internal/webserver/announce.go:78` — `server.announcement` returns
  `announcement, err == nil`, collapsing every `ComposeAnnouncement` error,
  wrapped read failures included, into ok false.
- `internal/webserver/announce.go:22` — `GetAnnouncement` answers
  `nothingToAnnounce` 409 for any `!ok`, so a read failure is told as an
  absent pull request.
- `internal/webserver/announce.go:44` — `Announce` answers the same 409
  for the same collapsed error.
- `internal/loop/announce.go:93` and `internal/loop/announce.go:98` —
  `pullToAnnounce` wraps a `Branch` and a `FindPull` failure as "reading
  the branch: %w" and "reading the pull request: %w", without
  `ErrNoPullRequest`, so the server could tell them apart and does not.
- `internal/webserver/pullrequest.go:116` — `server.composePullRequest`
  returns `draft, branch, err == nil`, folding `branchToOpen`'s "reading
  the branch: %w" (`internal/loop/pull.go:94`) into `nothingToOpen` at
  `GetPullRequestDraft` (`internal/webserver/pullrequest.go:26`) and
  `OpenPullRequest` (`internal/webserver/pullrequest.go:49`).
- `internal/webserver/announce_test.go:312` —
  `TestAnnouncingIsAConflictWithoutAPullRequest`'s case "the forge cannot
  be reached" pins the 409 for a `FindPull` error, the wrong answer.
- `docs/content/docs/errors.md:89` — "## Unreachable" reserves 502 for an
  upstream that could not be reached, which these four handlers never
  give.

With the forge down or the branch unreadable, GET /api/announcement, POST
/api/announce, GET /api/pull-request/draft and POST /api/pull-request all
say there is nothing to act on; the person may conclude the pull request
was never opened.

**One way to fix it.** Return the error from `announcement` and
`composePullRequest`, answer 409 only for `loop.ErrNoPullRequest`,
`loop.ErrNothingToOpen` and `loop.PullAlreadyOpenError`, and route any
other error through `fault`. The 409's wording per cause
(`PullAlreadyOpenError` naming the pull request) is UX-129; make the two in
one change.

**Done when.** A test where `FindPull` returns `forge.ErrUnreachable`
wrapped with a host makes GET /api/announcement and POST /api/announce
answer 502 `unreachable` with no host in the body; a test whose `Branch`
read fails makes GET /api/pull-request/draft answer through `fault` rather
than 409.

### DEBT-130 A git read that fails gets three answers, one of them verbatim

Severity: medium · Confidence: read

A `Branch`, `Branches` or `Changes` read that fails is answered three ways
across the handlers: a bare 500 through `fault` on `GetReview`,
`LinkPullRequest` and staging; a generic 422 on `Push`, `Checkout` and
`CreateBranch`; and a verbatim 422 on `Commit`, whose default arm forwards
`err.Error()`. `Repository.ReadBranch` words its failure with the repository's
on-disk path (`internal/gitrepo/branch.go:136`, "reading the current branch of
"+r.dir), where "`detail`" in `docs/content/docs/errors.md:31` promises a read
failure stays generic. `faultClasses` has no git class, so each handler decides
for itself.

- `internal/webserver/commit.go:64` — `server.Commit`'s default arm puts
  `err.Error()` in the 422 detail for every error but `ErrNothingStaged`.
- `internal/webserver/commit.go:87` — `server.commitStaged` returns the
  raw `Changes` read error.
- `internal/webserver/commit.go:102` — `server.commitStaged` returns the
  raw `Branch` read error.
- `internal/webserver/push.go:34` — `server.Push` answers a generic 422
  "the branch could not be read" for the same failure.
- `internal/webserver/checkout.go:83` — `server.refuseADirtyTree` returns
  the raw `Changes` error, which `server.Checkout`'s default arm at
  `internal/webserver/checkout.go:53` words generically.
- `internal/webserver/branchcreate.go:120` — `server.branchExists` reads
  the `Branches` listing, and `server.startWork` (`:87`) returns its raw
  error, which `createBranchFailure`'s default arm at
  `internal/webserver/branchcreate.go:72` words generically; the
  post-create `Branch` read (`:99`) is DEBT-141's.
- `internal/webserver/staging.go:171` — `stagingProblem`'s default arm
  routes the same read failure through `fault`, a bare 500.
- `internal/webserver/issuewrite.go:65` — `server.branchPull` wraps the
  `Branch` read and routes it through `fault`;
  `TestLinkReportsWhatItCouldNotRead`
  (`internal/webserver/issuewrite_test.go:199`) pins the 500.
- `internal/webserver/handlers.go:151` — `server.GetReview` routes the
  same failure through `fault`.
- `internal/webserver/errors.go:126` — `faultClasses` has no git-read
  class, so `fault` falls to the internal problem for every read failure.
- `internal/webserver/commit_test.go:244`,
  `internal/webserver/commit_test.go:261` and
  `internal/webserver/commit_test.go:278` —
  `TestCommitReportsAChangesReadFailure`, `TestCommitReportsAFailedStart`
  and `TestCommitReportsWhenTheBranchCannotBeReadAfter` assert the status
  alone, so any detail passes.

A script switching on `code` sees `internal` on one write and
`unprocessable` on the next for the same broken repository; on Commit the
repository's absolute path reaches the page, and `runCommit`'s start arm
(`internal/webserver/commit.go:110`) forwards the seam's error the same
way.

**One way to fix it.** Add a git-read class to `faultClasses`
(`gitrepo.ErrNotARepository` and a wrapped read sentinel), route every
handler's read failure through `fault`, keep `err.Error()` on Commit for
`errCommitFailed`'s hook output only, and pin the detail in the commit
tests as `internal/webserver/checkout_test.go` pins its own.

**Done when.** One table test sends the same failing `Branch` seam to
/api/push, /api/checkout, /api/commit and /api/issues/{key}/link, and a
failing `Branches` seam to POST /api/branches, and gets one status and
code; `TestCommitReportsAChangesReadFailure` asserts the
detail names neither the seam error's text nor a path, and passes.

### DEBT-131 The announcement posted may not be the one previewed

Severity: medium · Confidence: read

POST /api/announce composes the announcement again at post time from the
request's channel alone, so when the moment changes between the preview
and the press — CI turns red, the pull request merges — the text sent
differs from the text shown. The terminal and the command line post the
text they previewed.

- `internal/webserver/announce.go:42` — `server.Announce` calls
  `s.announcement()` again at post time.
- `internal/webserver/announce.go:52` — `server.Announce` posts
  `announcement.Text()` of that recomposed announcement, not the previewed
  text.
- `api/openapi.yaml:839` — `AnnounceRequest` requires `channel` only; the
  body carries no text or moment.
- `web/src/features/messaging/announceApi.ts:31` — `announce` sends
  `{ channel }` and nothing else.
- `internal/tui/messagingpreview.go:141` — `messagingPreview.post` sends
  `p.text`, the previewed text.
- `internal/cli/announce.go:144` — `runAnnounce` delivers the same `text`
  it printed.
- `docs/content/docs/web.md:114` — "### The messaging service" promises
  nothing is sent before the second press, which reads as a promise that
  what was shown is what is sent.

The last look the web offers is of a text the server does not hold to; the
outward post can say CI is red when the person approved "opened a pull
request". The window is the time between the two presses, so it is rare
and goes unnoticed, and no announce test flips `CheckCI` between the GET
and the POST.

**One way to fix it.** Carry the previewed text, or its moment, in
`AnnounceRequest` and answer 409 when the composed announcement no longer
matches, so the page previews again.

**Done when.** A test whose `CheckCI` flips to failed between GET
/api/announcement and POST /api/announce sees the post refused and nothing
sent.

### DEBT-132 Every stream frame re-asks what it could read once

Severity: medium · Confidence: read

`snapshot` calls `s.author()` on every push and the `Author` seam is an
uncached `Whoami` GET, so each open tab spends one forge request per
interval on a value fixed for the session; the same frame calls
`deps.Branch()` three times, each running `ReadBranch`'s seven git
commands; and `review` asks `CheckCI` about a merged pull, where the
terminal skips a pull that is not open (DEBT-127).

- `internal/webserver/stream.go:90` — `server.snapshot` builds
  `messagingDTO(s.config(), s.author())` on every frame.
- `internal/webserver/handlers.go:241` — `server.author` calls
  `s.deps.Author()` with no cache.
- `internal/wiring/forge.go:112` — the `Author` seam runs
  `connection.client.Whoami(ctx)` on every call; only the connection is
  memoized.
- `internal/forge/client.go:105` — `Client.Whoami` is one uncached GET of
  `userPath`.
- `internal/tui/messaging.go:102` — `Model.loadAuthor` skips the read once
  `m.messaging.author` is set; the web diverges on the same seam.
- `internal/webserver/stream.go:153` — `server.snapshotBranch`, the
  frame's first `deps.Branch()`.
- `internal/webserver/stream.go:183` — `server.snapshotReview`, the
  second.
- `internal/webserver/stream.go:121` — `server.currentBranchName`, the
  third, called from `snapshotBranches`.
- `internal/gitrepo/branch.go:141` — `Repository.ReadBranch` runs
  `branch --show-current`, `rev-parse HEAD`, `rev-parse @{upstream}`,
  `config --get remote.pushDefault`, the base lookup, `rev-list`, `log` and
  `log -1` per read.
- `internal/webserver/handlers.go:182` — `server.review` calls `CheckCI`
  whenever a pull is found; `Model.checkCI` (`internal/tui/review.go:105`)
  does not unless `State == StateOpen`.
- `internal/forge/githubci.go:41` — `githubStatus` pages both the combined
  status and the check runs, at least two requests per frame.

On GitHub with an open pull a frame is at least six forge requests —
`githubFind`'s one plus `githubReviewState`'s two, `githubStatus`'s two,
`Whoami`'s one — about 4,320 an hour for one tab at the 5 s default
(`defaultStreamInterval`, `internal/webserver/stream.go:20`) against GitHub's
5,000-an-hour limit; when the limit is hit, `snapshot` folds the failure into
empty panels with no signal (its comment at `internal/webserver/stream.go:81`
says so). About twenty-six git processes per frame — three branch reads of
eight commands each, plus the changes and branches reads — where ten would do,
and a branch, review and in-flight marker read at three instants, so the panels
can describe different branches when a checkout lands between the reads.

**One way to fix it.** Read the author once per server, as `scopeCache`
(`internal/webserver/webserver.go:136`) reads the scope, re-reading only
after a failure; and read the branch once in `snapshot` and pass it to the
review and branches builders.

**Done when.** A stream test with counting `Author` and `Branch` seams sees
one `Author` call across three pushed snapshots and one `Branch` call per
pushed snapshot (the merged-pull `CheckCI` skip is DEBT-127).

### DEBT-133 The errors page points a 500's cause at output nothing writes

Severity: medium · Confidence: read

"## Internal" in `docs/content/docs/errors.md:101` says the cause of a 500
"is in the server's own output, not the response". Both places that answer
`internal` discard the error: `writeResponseError`
(`internal/webserver/errors.go:84`) ignores its error argument, and
`faultProblem` (`internal/webserver/errors.go:112`) returns the generic
internal problem without recording `err`. No non-test file in
`internal/webserver` writes a log line (zero matches for `log.`, `slog` or
`Stderr`, by grep) and nothing sets an `ErrorLog`. The notes writer the
command line hands `serve` (`internal/cli/cli.go:196`, `cmd.ErrOrStderr()`)
carries only the address line `WebServerAt` prints
(`internal/cli/cli.go:273`).

A user who meets a 500 is told to look at the terminal and finds only the
address line; the unclassified seam error is gone, so neither the user nor
a bug report can say what failed. `--log` (`internal/cli/cli.go:210`)
outlines each request's method, path, status and duration, not the cause.
CLAUDE.md says never to swallow an error.

**One way to fix it.** Write the discarded error to the notes writer the
command line already hands `serve`, one line per internal problem, through
a func-var seam on `Handler`; or drop the sentence from
`docs/content/docs/errors.md`.

**Done when.** A test with a failing seam that no fault class matches sees
the error's text in the notes writer, or `docs/content/docs/errors.md` no
longer says the cause is in the output.

### DEBT-134 Four GET reads no caller uses, three of them written twice

Severity: low · Confidence: read

GET /api/branch, /api/changes, /api/review and /api/messaging are called by
nothing in `web/src` or `web/e2e` outside the generated client: the page
reads the snapshot, and the one match by grep,
`web/src/api/client.test.tsx:84`, uses `/api/branch` as a sample URL for
the client wrapper's dry-run test. No doc names them for scripts, and three
snapshot builders repeat the handlers line for line with a different
failure answer.

- `internal/webserver/handlers.go:112` — `server.GetBranch`, a handler
  with no caller, is the same nil-seam, read, DTO sequence as
  `server.snapshotBranch` (`internal/webserver/stream.go:148`), which
  differs only in answering an empty branch on failure.
- `internal/webserver/handlers.go:128` — `server.ListChanges`, no caller;
  `server.snapshotChanges` (`internal/webserver/stream.go:163`) is the
  same read with `changesDTO(nil)` on failure instead of `fault`.
- `internal/webserver/handlers.go:144` — `server.GetReview`, no caller:
  `Branch`, `FindPull`, `review` through `fault`, where
  `server.snapshotReview` (`internal/webserver/stream.go:178`) runs the
  identical sequence with failures as `Found: false`.
- `internal/webserver/handlers.go:228` — `server.GetMessaging`, no caller:
  the one-line `messagingDTO` that `server.snapshot` also builds at
  `internal/webserver/stream.go:90`.
- `docs/content/docs/scripting.md:57` — "### The same families on the web"
  names the problem codes for scripts but no read endpoint.

Four handlers and their tests exist for a caller that does not exist; the
snapshot's copy is the one the page uses, so a change to how a branch, the
changes or the review is read must be made in both files, and a failure
answer proven on the handler is not proven on the copy the page reaches.

**One way to fix it.** Decide whether the four reads are the script surface
(then `docs/content/docs/web.md` names them) or not (then drop them from
the spec); either way have each snapshot builder call the one read function
so the logic lives once.

**Done when.** Either `docs/content/docs/web.md` lists GET /api/branch,
/api/changes, /api/review and /api/messaging as scriptable reads, or they
are gone from `api/openapi.yaml` and `task gen` leaves no `GetBranch`,
`ListChanges`, `GetReview` or `GetMessaging`; and each snapshot builder
calls the shared read function.

### DEBT-135 The contract's prose disagrees with the code at eight places

Severity: low · Confidence: read

`api/openapi.yaml` describes behavior the server does not have.
kin-openapi's `Validate` passes on all of it (`loadSpec`,
`internal/webserver/validator.go:29`), so no gate sees it; a client written
from the description is what it hurts.

- `api/openapi.yaml:600` — `push`'s description promises 409 "when there
  is nothing to push (no commits, or already up to date)"; `nothingToPush`
  (`internal/webserver/push.go:59`) consults only the name and the
  upstream on the push remote, deliberately (its comment at
  `internal/webserver/push.go:49`), so a branch with no upstream and no
  commits is pushed.
- `api/openapi.yaml:45` — the `events` tag says snapshots are "pushed as
  they change", and `streamEvents`'s summary at `api/openapi.yaml:724`
  says the same; `defaultStreamInterval`'s comment
  (`internal/webserver/stream.go:19`) says the server re-reads on a
  cadence and pushes the result, with no comparison to the previous frame,
  as "**The stream**" in `docs/content/docs/web.md:46` also says.
- `api/openapi.yaml:4` — the header comment credits `task gen:verify` with
  failing CI "if either drifts"; `gen:verify` (`Taskfile.yml:433`) diffs
  only `internal/api`, and the web client is checked by `web:gen:check`
  (`Taskfile.yml:179`).
- `api/openapi.yaml:469` — `checkout` carries `tags: [branches]`, as do
  `createBranch` (`api/openapi.yaml:505`), `push` (`api/openapi.yaml:602`)
  and `commit` (`api/openapi.yaml:632`); the `tags` list at
  `api/openapi.yaml:29` never declares `branches`.
- `api/openapi.yaml:22` — the `info` description says the server "keeps
  nothing between requests but the commit scope it learns"; `server` holds
  `cfg` and `seen` (`internal/webserver/webserver.go:118`) across
  requests, and `getConfig`'s own description at `api/openapi.yaml:380`
  relies on it ("A file that has been deleted leaves the configuration in
  effect as it was").
- `api/openapi.yaml:328` — `getReview`'s 200 says "pull and ci are null
  when none is found"; `Review` in `internal/api/models.gen.go:692` marks
  both `omitempty` and `server.review`
  (`internal/webserver/handlers.go:170`) leaves them nil, so they are
  absent, as the not-found frame in `web/src/test/snapshot-frames.sse:7`
  shows.
- `api/openapi.yaml:1067` — `Issue.priority` "May be empty"; `issueDTO`
  (`internal/webserver/dto.go:33`) passes it through `optional`, which
  sends an empty string as an absent field.
- `api/openapi.yaml:1165` — `Change.original_path` is "empty otherwise";
  `changesDTO` (`internal/webserver/dto.go:122`) passes it through
  `optional`, which omits it.

A client written to the description checks `pull === null` or
`priority === ''` and never matches, expects a 409 it never gets, and
expects a quiet event-driven stream; a contributor who runs `gen:verify`
after a spec change may believe the TypeScript client is current;
`getConfig`'s own description depends on state the `info` block says does
not exist.

**One way to fix it.** One pass over `api/openapi.yaml`: reword the push
409 as "a detached HEAD, or a published branch that is not ahead"; say
snapshots are pushed on connect and every few seconds; name `web:gen:check`
in the header; retag the four operations as `repository` or declare
`branches`; say the server keeps the configuration in effect and the
learned scope; say "absent" for pull, ci, priority and original_path.

**Done when.** The push description and `nothingToPush`'s comment name the
same two cases; the `events` tag and the `streamEvents` summary match
`defaultStreamInterval`'s comment; the header names `web:gen:check`; every
tag an operation uses appears in the top-level `tags` list; the `info`
description names the configuration in effect; the review 200 description
matches the not-found frame in `web/src/test/snapshot-frames.sse`, which
carries no `pull` key; the three optional strings use one word for one wire
shape; and `task gen` leaves the generated code unchanged.

### DEBT-136 A wrong method on a known path is answered 404, not 405

Severity: low · Confidence: read

The validator's router returns a nil route with
`routers.ErrMethodNotAllowed` for a method mismatch; nethttp-middleware
v1.2.0 (`go.mod:12`) then reports status 404 whenever the route is nil, and
`writeValidationError` (`internal/webserver/validator.go:56`) turns any 404
into `not_found` "no such endpoint" (`internal/webserver/validator.go:57`),
so POST /api/branch or DELETE /api/config is told the endpoint does not
exist rather than 405 with `Allow`. `TestAnUnknownEndpointIsNotFound`
(`internal/webserver/serve_test.go:82`) covers an unknown path only; no
test sends a wrong method to an /api path, and
`TestTheAppRefusesANonReadMethod` (`internal/webserver/static_test.go:131`)
pins 405 for the app only.

A script that mistypes the verb is sent looking for a typo in the path; the
`net/http` mux behind the validator would have answered 405 on its own.

**One way to fix it.** In `writeValidationError`, test
`errors.Is(err, routers.ErrMethodNotAllowed)` before the 404 branch and
answer 405 with a problem (a new enum code, added to
`docs/content/docs/errors.md`), or let the request through to the mux.

**Done when.** A test sending POST /api/branch sees 405, not 404.

### DEBT-139 The web derives its own stage rules, and they contradict `progress`

Severity: medium · Confidence: read

`WorkStory` derives the work story's stages in TypeScript with rules that
disagree with `internal/progress`: its Changes stage is done only with a
clean tree and a commit, where `commitState` reads Done on any commit; and
`pullRequestDone` reads done with no CI or with changes requested, where
`reviewState` reads in flight and failed. The package comment that says the
rule "lives in one place" no longer holds. No FEATURES entry carries the
stages in the snapshot; the first fix below is that change.

- `web/src/features/issues/WorkStory.tsx:79` — `onHeadStages`, the second
  derivation: four stages with their own names and rules.
- `web/src/features/issues/WorkStory.tsx:97` — `onHeadStages` marks
  Changes done only when
  `changes.changes.length === 0 && branch.commits.length > 0`;
  `commitState` (`internal/progress/progress.go:101`) returns Done on
  `OnFeatureBranch && Commits > 0` before it reads `UncommittedChanges`.
- `web/src/features/issues/WorkStory.tsx:124` — `pullRequestDone` is done
  unless CI is `running` or `failed`, so a CI state of none reads done and
  `changes_requested` is never read; `reviewState`
  (`internal/progress/progress.go:117`) is Failed on
  `CIFailed || ChangesRequested`, Done only on `CIPassed`, and in flight
  otherwise.
- `internal/progress/progress.go:8` — the package comment: both the spine
  and `workflow status` read it, "so the rule lives in one place".
- `internal/progress/progress_test.go:47` —
  `TestStagesDeriveHowFarTheWorkHasGot` has no case with both
  `Commits > 0` and `UncommittedChanges > 0`, so the Go precedence is
  unpinned; `web/src/features/issues/WorkStory.test.tsx` sets
  `changes_requested` only to false and never a CI state of none.

On a branch with one commit and an edited file, the spine and `status` show
Commits done while the browser shows Changes still active. A repository
without CI reads Review in flight in the terminal and done in the browser;
a reviewer's changes requested reads failed in the terminal and done in the
browser, though the snapshot carries `changes_requested`.

**One way to fix it.** Derive once in Go and carry the stages in the
snapshot (`progress.Stages` over a `Work` the server builds) so the web
renders rather than re-derives; until then port `ChangesRequested` and the
CI-none rule to `pullRequestDone`, add the commits-plus-changes case to
`internal/progress/progress_test.go`, and make the package comment name
every place a stage is derived.

**Done when.** A web test with one commit and one change shows the state
the Go table case gives; one with `review.found`, no CI and
`changes_requested` true shows the review stage failed, matching the Go case
"changes requested stops review reading done"; the commits-plus-changes
case exists in `internal/progress/progress_test.go`; and either the
snapshot schema has a stages array the web renders, or, until then,
`pullRequestDone` reads `changes_requested` and a CI state of none as the Go
table does.

### DEBT-140 `Validate` accepts a description with a newline

Severity: low · Confidence: read

`CommitConvention.Validate` (`internal/convention/commit.go:133`) checks a
description for emptiness, a trailing period and length only, so a
description holding a newline passes and `Subject.String`
(`internal/convention/convention.go:288`) keeps it, writing a subject that
spans two lines. `CommitRequest.subject` is an unconstrained string
(`api/openapi.yaml:877`), so POST /api/commit can produce a message that is
not a well-formed Conventional Commit despite the handler's 422 promise.
Neither the browser's Subject, an `<input>` as the test "after a commit the
form opens on the scope just used" reads it
(`web/src/features/branch/CommitForm.test.tsx:84`), nor the terminal's text
input can produce the input, so only a hand-built request reaches it.

With lefthook the commit-msg hook refuses the commit after the fact;
without it a two-line subject lands.

**One way to fix it.** Add an `ErrMultilineSubject` sentinel and refuse a
description containing a newline or a control character in `Validate`.

**Done when.** `Validate(Subject{Type: "fix", Description: "a\nb"})`
returns the sentinel and POST /api/commit with that subject answers 422.

### DEBT-141 A commit, create or checkout that landed is reported failed

Severity: medium · Confidence: read

`commitStaged`, `startWork` and `switchTo` each return the confirming
`Branch()` read's error after `runCommit`, `CreateBranch` or `Checkout` has
already moved the tree, and the handlers' default arms answer 422 — git's
read error verbatim for the commit, "could not be created; try again" and
"could not be checked out; try again" for the branches — so a write that
landed is told as a failure, where `publishedBranch` in the same package
deliberately answers the pre-write state when the re-read after a push
fails.

- `internal/webserver/commit.go:102` — `server.commitStaged` returns
  `s.deps.Branch()` after `runCommit` succeeded; the read's error becomes
  the commit's failure.
- `internal/webserver/commit.go:64` — `server.Commit`'s default arm
  answers that error as 422 with `err.Error()` as the detail.
- `internal/webserver/commit_test.go:278` —
  `TestCommitReportsWhenTheBranchCannotBeReadAfter` pins the 422 for a
  commit that ran.
- `internal/webserver/branchcreate.go:99` — `server.startWork` returns
  `s.deps.Branch()` after `createAndSwitch` ran.
- `internal/webserver/branchcreate.go:72` — `createBranchFailure`'s
  default arm says the branch "could not be created; try again" though it
  exists.
- `internal/webserver/branchcreate.go:87` — a retry then reaches
  `branchExists` in `server.startWork` and answers 409 `errBranchExists`,
  contradicting the 422.
- `internal/webserver/branchcreate_test.go:259` —
  `TestCreateBranchReportsWhenTheNewBranchCannotBeRead` pins the 422 for
  a branch that was created.
- `internal/webserver/checkout.go:70` — `server.switchTo` returns
  `s.deps.Branch()` after `Checkout` succeeded.
- `internal/webserver/checkout.go:53` — `server.Checkout`'s default arm
  answers 422 "could not be checked out; try again" for that read's error;
  no test covers this read.
- `internal/webserver/push.go:72` — `server.publishedBranch`'s comment: a
  re-read that fails does not undo the push, so the pre-push branch is
  returned.

The commit form shows a red alert with git's read error, and a retry
answers 409 "nothing is staged to commit" while the commit is in the
repository; the create says try again, and the retry answers 409 "a branch
for this issue already exists"; the checkout says the switch failed while
the tree is on the requested branch, until the stream corrects it a frame
later. Two tests pin the 422 with no rationale, and no trade-off records
why the commit and the branches differ from the push.

**One way to fix it.** Mirror `publishedBranch`: after the write succeeds,
answer 200 with the pre-commit branch, or with a branch built from the name
just created or requested, when the confirming read fails (the pinned
create scenario's pre-create read fails too, so the fallback must come from
the created name).

**Done when.** A test whose `Branch` seam fails only on its second read
answers 200 to POST /api/commit; a test whose `Branch` seam fails only
after `CreateBranch` ran answers 200 to POST /api/branches naming the
created branch; and a test whose `Branch` seam fails after `Checkout` ran
answers 200 to POST /api/checkout — each as
`TestPushSucceedsEvenIfTheRereadFails`
(`internal/webserver/push_test.go:188`) does for the push.

### DEBT-142 Two 422 details carry a host the docs keep off the wire

Severity: medium · Confidence: read

`openFailure` curates only `forge.ErrUnreachable` and `ErrUnknownForge`,
and `httpx.Unreachable` wraps a refused redirect as `ErrRedirected` with
`the server at <base>` in its text, so a forge answering a login redirect
makes POST /api/pull-request answer 422 naming the forge's API base — and
a rate limit answers 422 with sentinel text where the read path's
`faultClasses` gives a curated 502. `pushFailure` joins git's push
output verbatim into the detail, which git ends with `To <remote-url>` or
`failed to push some refs to <url>`, and the open reuses it, while staging
keeps git's words off the wire for exactly that reason.

- `internal/webserver/pullrequest.go:182` — `openFailure`'s
  `errors.Is(err, forge.ErrUnreachable)` is false for a redirect, which
  wraps only `ErrRedirected`.
- `internal/webserver/pullrequest.go:189` — `openFailure`'s default arm
  puts `err.Error()`, API base included, in the 422 detail.
- `internal/webserver/pullrequest.go:175` — `openFailure`'s doc comment
  counts two host-carrying failures where there are three.
- `internal/httpx/httpx.go:50` — `Unreachable` wraps a redirect as
  `ErrRedirected` with " at "+base in its text and never the caller's
  sentinel.
- `internal/forge/client.go:214` — `Client.exchange` passes `c.base` into
  that text; nothing re-wraps it before the handler.
- `internal/webserver/errors.go:148` — the `httpx.ErrRedirected` class in
  `faultClasses`, the curated 502 the read path gives for the same
  failure.
- `docs/content/docs/errors.md:31` — "`detail`" promises an unreachable
  upstream stays generic.
- `internal/webserver/push.go:65` — `pushFailure` joins the push's output
  verbatim into the detail.
- `internal/webserver/pullrequest.go:59` — `server.OpenPullRequest` reuses
  `pushFailure`, so the same output reaches its 422.
- `internal/webserver/staging.go:27` — `errGitRefused`'s comment: git's
  own words stay off the wire because a fetch that fails names the remote.
- `internal/webserver/push_test.go:101` — `TestPushReportsAFailingPush`'s
  only push output, "! [rejected] fix/PROJ-412", carries no URL.

A user behind an SSO forge sees the forge address in the browser's alert,
against the errors page's promise; a rejected push can print the remote
URL; and the package holds two policies on git's words.

**One way to fix it.** Have `openFailure` keep `err.Error()` only for
`forge.ErrRejected`, whose reason is the forge's own words, and send every
other error through `faultProblem`; and decide once for the push — strip
lines carrying a URL from the output before it reaches the detail, or write
the exception beside `errGitRefused` so the next reader knows the push is
meant to differ.

**Done when.** A test where `CreatePull` returns `httpx.Unreachable` of
`forge.ErrUnreachable`, an internal API base and `httpx.ErrRedirected`
answers 502 `unreachable` with no host in the body, and a rate-limited
create answers 502; and a test whose push output carries
`To https://git.internal.example/acme/repo.git` answers a detail that keeps
`[rejected]` and omits the host.

## The gates, the build and the tests

What is open here is the coverage worklist and the metric behind it, gates
whose printed sentence claims more than their check measures, CI jobs and
triggers that do not do what their comments say, and tests named or shaped
for something other than what they prove.

### DEBT-64 The condition-coverage worklist: 431 one-sided conditions, and 17 never evaluated

Severity: low · Confidence: measured

Re-measured at this commit (`task cover:branch` on macOS, floor 89 %, 23
packages measured), with gobco counting every operand of an `&&` or `||` as
a condition of its own: 4,873 of 5,338 arms, 91.3 %. Of 2,669 conditions,
431 were observed only one way — 80 of them an `err != nil` never seen
true. By package: `internal/tui` 172, `internal/cli` 48, `internal/forge`
41, `internal/webserver` 25, `internal/config` 17, `internal/jira` 16,
`internal/testshape` 15, `internal/messaging` 14, `internal/wiring` 14,
`internal/gitrepo` 13, `internal/hooks` 12, `internal/store` 10,
`internal/tui/frame` 10, `internal/convention` 8, `internal/editor` 6,
`internal/buildinfo` 5, and five across `sanitize` (two) and `httpx`,
`proc` and `tui/layout` (one each).

The store has ten. Five are `sql.Open` in `Store.open`
(`internal/store/store.go:154`), which fails only for an unregistered
driver, and four that need SQLite to fail partway through a statement:
`BeginTx` and `Commit` in `Store.CacheIssues` (`internal/store/cache.go:110`,
`:121`), and `rows.Err` in `readCachedIssues` (`internal/store/cache.go:88`)
and `Store.Announces` (`internal/store/announce.go:80`). The other five are
the do-nothing guards' second operands, never seen true: `repo == ""` in
`Store.RecordAnnounce` and `Store.Announces`
(`internal/store/announce.go:24`, `:50`), `instance == ""` in
`Store.CachedIssues` and `Store.CacheIssues` (`internal/store/cache.go:32`,
`:99`), and `s.dir == ""` in `Store.off` (`internal/store/store.go:135`).

Seventeen conditions were never evaluated. Five are a test away:

- `internal/cli/doctor_json.go:237` — `credentialStatus`'s `errUnreachable`
  case: no `doctor --json --online` test has a credential check fail.
- `internal/tui/messaging.go:90` and `:92` — `quitGuard.handleKey`'s confirm
  and stay: `TestQuittingWithAQueuedPostAsksFirst` opens the guard but
  presses neither enter (quit) nor esc (stay) in it.
- `internal/tui/prcreate.go:134` — `pullCreated.apply`'s `named` case: every
  test that opens a pull request on an issue's branch wires
  `Jira.LinkPullRequest`.
- `internal/wiring/wiring.go:234` — `streamToEnd`, git failing to start: no
  wiring test fetches or pulls without git on `PATH`.

Eight more came into view once each operand counted, and each is a test
away too:

- `internal/cli/doctor_json.go:142` — `toolingFacts`'s `program.required`:
  no test runs `doctor` with a tool missing from `PATH`, so `!installed`
  never lets the `&&` read it.
- `internal/config/config.go:348` and `:364` — `Problems`'
  `absoluteWebURL(base)` and the four operands inside `absoluteWebURL`: no
  `internal/config` test calls `Problems` with a `jira.base_url` set.
- `internal/messaging/post.go:335` — `Announcement.Text`'s `a.Kind ==
  config.KindSlack`: each test that renders a template leaves `Kind` empty,
  so the `||` never reads it.
- `internal/tui/issuekeys.go:136` — `extendFilterWith`'s `msg.Code ==
  tea.KeySpace`: no filter test types a key without text, so `text == ""`
  never lets the `&&` read it.

Four no black-box test reaches without changing the code:

- `internal/tui/tui.go:142` and `:149` — inside `tui.Run`, which needs a real
  terminal.
- `internal/wiring/wiring.go:144` — `browserCommand`'s `"windows"` case,
  evaluated only where `runtime.GOOS` is not `"darwin"`: CI's Linux run reaches
  it, a Mac never does.
- `internal/wiring/forgecli.go:59` — `forgeProgram`'s `forge.KindUnknown`
  case, which `exhaustive` requires but `connectForge` never passes, since
  `Repo.APIBase` refuses an unknown forge first.

**What it costs.** Condition coverage reads 91.3 %, 2.3 points above the
89 % floor, which is the ratchet's own slack, so an untested error path in
the next feature no longer fails the gate on someone else's pull request.
The cost now is the ratchet: floor(91.3) − 2 is today's 89, so
`BRANCH_COVERAGE_MIN` cannot rise until the report reads 92.0 % — 4,909
arms, 36 more than today, since the gate rounds to one decimal first.

**One way to fix it.** The reachable sites above, one test each; then the
report is the worklist, most of it in `internal/tui`, `internal/cli` and
`internal/forge`.

**Done when.** `task cover:branch` names no never-evaluated condition but
the four above, and reads 92.0 % or more, so `BRANCH_COVERAGE_MIN` ratchets
to 90.

### DEBT-65 The web's e2e drives no write

Severity: medium · Confidence: read

`task check` (`Taskfile.yml:502`) runs the web's lint, client-drift check and
unit tests beside the Go gates, but not its end-to-end suite. That suite is
six specs (`web/e2e/a11y.spec.ts`, `web/e2e/layout.spec.ts`,
`web/e2e/panes.spec.ts`, `web/e2e/screens.spec.ts`,
`web/e2e/smoke.spec.ts`, `web/e2e/theme.spec.ts`), outside `task check`
(CI's `e2e` job and `yarn test:e2e` run it), with no `workflow --web`
backend — acknowledged at `.github/workflows/ci.yml:101` ("No backend": the
specs answer the API themselves, or read a VITE_MOCK build's fixtures) — so
no test drives any of the twelve write actions end to end.

**One way to fix it.** The e2e job starts `workflow --web` against a fixture
repository so one spec can commit, push and open a pull request.

**Done when.** One Playwright spec performs a write against a running server.

### DEBT-143 Twelve `nolint:paralleltest` reasons name a profile `forceANSI` does not own

Severity: low · Confidence: read

Twelve tests in `internal/tui` carry the same directive,
`//nolint:paralleltest // forceANSI owns the global color profile; must run
serially`, but `forceANSI` (`internal/tui/color_test.go:24`) is documented
as a no-op that returns a no-op: Lip Gloss v2 renders a style's colors into
the string whether or not the output is a terminal, so there is no global
profile to own. `redOpen` (`internal/tui/color_test.go:17`) still says it
"needs forceANSI to be in effect". `paralleltest` runs on `internal/tui`, so
the directive counts as used and no gate sees the stale reason. Counted by
grep of the exact string: 12 lines in 6 files.

- `internal/tui/styles_test.go:13` —
  `TestTheFooterUsesTheThemeNotFixedGrays` carries the reason and a `defer
  forceANSI(t)()` that restores nothing.
- `internal/tui/styles_test.go:36` —
  `TestEmptyStateSentencesAreNotDrawnFaint`; its subtests (`:59`) omit
  `t.Parallel()` too.
- `internal/tui/styles_test.go:77` —
  `TestNoColorKeepsTheCursorButDropsTheHue`, though `WithoutColor`
  (`internal/tui/tui.go:124`) is per model, not global.
- `internal/tui/styles_test.go:103` — `TestTheFocusedPaneWearsABoldTitle`.
- `internal/tui/wrap_color_test.go:17` —
  `TestAPaneFailureClosesItsColorEachRow` builds its own world and shares
  nothing.
- `internal/tui/failure_color_test.go:20` —
  `TestEveryFailureGlyphRendersRed`.
- `internal/tui/failure_notice_test.go:33`, `:112`, `:157` and `:175` —
  `TestARefusalNoticeWearsTheFailureStyle`, `TestAGuidanceNoticeStaysPlain`,
  `TestADroppedPostIsNoticedAsAFailure` and
  `TestAShortTerminalsFooterDrawsAFailureInRed`.
- `internal/tui/failure_channels_test.go:218` —
  `TestEveryChannelSpeaksTheFailureSentence`.
- `internal/tui/failure_test.go:391` —
  `TestTheConfigurationScreenShowsItsErrorAsAFailure`.

The cost is twelve tests and their subtests running serially for a mechanism
that no longer exists, each behind a comment that covers for it.

**One way to fix it.** Drop the directive and the `defer` at each site, add
`t.Parallel()` to each test and its subtests, trim `redOpen`'s comment, and
delete `forceANSI` if nothing calls it.

**Done when.** `grep -rn 'forceANSI owns the global color profile'
internal/tui` prints nothing, each of the twelve tests calls `t.Parallel()`,
and `task lint` passes.

### DEBT-144 `internal/tui/layout/layout_test.go` describes a five-pane rail the interface no longer draws

Severity: low · Confidence: read

`internal/tui/layout/layout_test.go:14` declares `railPanes = 5`, and the
comment above it (`:13`) names "Issues, Branch, Commits, Review and Slack",
while `paneCount` (`internal/tui/panes.go:27`) is 6 and the rail draws a
Reviews pane. The height tables are built on the five:
`TestTheFocusedPaneTakesTheRoomTheOthersDoNotNeed`
(`internal/tui/layout/layout_test.go:123`) expects `{25, 3, 3, 3, 3}` for a
rail the interface never draws, and `TestRailAt`'s "last row of the rail"
case (`internal/tui/layout/layout_test.go:248`) expects index 4. The
six-pane geometry `Compute` (`internal/tui/layout/layout.go:71`) is asked
for is exercised only through `internal/tui`'s screen tests, which render
through the same call. The gobco report reads `internal/tui/layout` at 21
of 22 arms; the one it misses, `column >= b.X` in `Box.Contains`
(`internal/tui/layout/layout.go:58`) never seen false, has nothing to do
with the rail's pane count, so the drift is in what the tests describe,
not in what they reach.

**One way to fix it.** Set `railPanes` to 6, name Reviews in the comment,
and recompute the expected heights and the `RailAt` rows.

**Done when.** `railPanes` reads 6, every height table in
`internal/tui/layout/layout_test.go` sums with six boxes, and `TestRailAt`'s
last-row case expects index 5.

### DEBT-145 The test world's runs carry no Stop, and production says so

Severity: low · Confidence: read

The `stop` field of `commandRun` (`internal/tui/run.go:37`) is commented as
nil "before then, and for a run started by a fake that supplies none", which
names a test fixture in production code. `Start`
(`internal/proc/start.go:131`), the only production constructor of a live
`proc.Output`, always sets `Stop: cancel`; the test world's `output` helper
(`internal/tui/world_test.go:271`) builds a `proc.Output` without one, a
fake that cuts a corner the real seam never does. `stopRun`
(`internal/tui/run.go:327`) returns at once when `stop` is nil, so pressing
`s` on any fake run does nothing and no test would notice; only
`blockingOutput` (`internal/tui/world_test.go:202`) supplies a `Stop`.

**One way to fix it.** Have `output` return a no-op `Stop`, as `proc.Start`
always does, and trim the comment to the real case: nil before `runStarted`
arrives.

**Done when.** The comment on `commandRun.stop` names only the pre-start
case, and every fake `proc.Output` built in `internal/tui`'s tests carries a
`Stop`.

### DEBT-146 `internal/webserver/coverage_test.go` is named for the gate, not for what it tests

Severity: low · Confidence: read

`internal/webserver/coverage_test.go` holds eleven tests over seven
handlers: `TestListIssuesUsesTheNamedView` (`:19`),
`TestGetIssueReportsAFailure` (`:44`), `TestGetBranchReportsAFailure`
(`:60`), `TestListChangesIsEmptyWithoutARepository` (`:76`),
`TestListChangesReportsAFailure` (`:92`),
`TestGetMessagingHasNoAuthorWithoutAForge` (`:108`), two `GetReview` tests
(`:124`, `:158`) and three `UpdateConfig` tests (`:191`, `:220`, `:255`).
The issue, review and config tests have home files beside it
(`internal/webserver/issues_test.go`, `internal/webserver/review_test.go`,
`internal/webserver/config_test.go`,
`internal/webserver/configrevision_test.go`); the `GetBranch`,
`ListChanges` and `GetMessaging` tests have none, their siblings sitting in
`internal/webserver/webserver_test.go` (`:347`, `:359`, `:375`, `:387`,
`:405`, `:424`), a file named for the package; and the file's name says only
why it was written. A contributor looking for the config save tests reads
`internal/webserver/config_test.go` and
`internal/webserver/configrevision_test.go` and misses three.

**One way to fix it.** Move each test beside its handler's tests; move the
`GetBranch`, `ListChanges` and `GetMessaging` tests from both files into
three new test files in `internal/webserver`, one named for each handler;
and delete `internal/webserver/coverage_test.go`.

**Done when.** No `internal/webserver/coverage_test.go` exists,
`internal/webserver/webserver_test.go` holds no `GetBranch`, `ListChanges`
or `GetMessaging` test, and each of the package's other test files is named
for a handler or a concern (`internal/webserver/webserver_test.go` keeps the
package-wide tests).

### DEBT-148 Four clicked steps in the web are never scanned or walked

Severity: medium · Confidence: read

The axe scans reach the issue detail, the opened pull's offers and a staged file
after a click, and the Tab walk runs once, on each section as it opens. Four
steps a user reaches by clicking — the pull request form, the push confirmation,
the announcement preview and a write's refusal — are scanned and walked in
neither theme, and the hermetic Settings scan settles on the heading rather than
on the read's outcome. `CLAUDE.md:238` promises axe across every section in both
themes and every control Tab reaches in view at three widths; for the clicked
steps only jsx-a11y's static rules apply. DEBT-65 (a write against a real
server) does not cover this: it is the runtime floor's reach, not the backend's.

- `web/e2e/a11y.spec.ts:102` — the populated-sections test clicks each of
  `populatedSectionNames` and scans at once; the populated snapshot's
  `review` has `found: true` (`web/src/dev/mockSnapshot.ts:108`), so the
  pull branch is taken and `PullRequestForm` never mounts there.
- `web/e2e/a11y.spec.ts:265` — the offers test clicks "Open a pull request"
  and then "Open pull request" with no scan between compose and submit, and
  `scan` runs (`web/e2e/a11y.spec.ts:273`) after `OpenPullRequest`
  (`web/src/features/review/ReviewPanel.tsx:166`) has returned null on
  `open.state === 'done'`, so the form is gone.
- `web/e2e/a11y.spec.ts:21` — `settled` returns the level-1 heading for
  every section but Reviews, and the heading is drawn regardless of panel
  state; the hermetic loop (`web/e2e/a11y.spec.ts:72`) scans as soon as it
  is visible, before the config read fails to Retry, so it may land on
  `SettingsPanel`'s "Loading the configuration…" placeholder
  (`web/src/features/settings/SettingsPanel.tsx:30`).
- `web/e2e/layout.spec.ts:180` — the every-section-fits test runs
  `openSection` then `walkTabOrder` once per section with no click between.
- `web/src/features/branch/stagingApi.ts:13` — `stageFile` under `VITE_MOCK`
  returns before the SDK, so no write can fail and no `role=alert` refusal
  appears.
- `web/src/features/review/ReviewPanel.tsx:265` — `PullRequestForm`'s
  `aria-label="Open a …"` form, with its seven labeled fields, is scanned
  and walked in neither build.
- `web/src/features/branch/BranchPanel.tsx:172` — `PushConfirm`'s
  `role="group"` with `aria-labelledby` sits behind `confirming`, which no
  spec sets.
- `web/src/features/messaging/MessagingPanel.tsx:231` — `AnnouncePreview`'s
  `role="group"` sits behind `preview.state === 'done'`, which no spec
  reaches.

So `PushConfirm`'s `aria-labelledby` resolving, the form's label
associations, and the focus order of a form that opens inside the scrolling
pane — exactly where a focused control is clipped at 640 px — can regress
green. The hermetic Settings scan is timing-dependent; what it can hide is
one `EmptyState` with a Retry button.

**One way to fix it.** In `web/e2e/a11y.spec.ts`, scan with the pull request
form, the push confirmation and the announcement preview open, and with one
write routed to a 500 so its alert is on screen, in both themes; in
`web/e2e/layout.spec.ts`, walk again after opening each step and add a
hermetic pull-request-form case at 640 px; settle hermetic Settings on Retry
the way Reviews settles on its list.

**Done when.** `web/e2e/a11y.spec.ts` scans a page on which the "Open a pull
request" form, the push confirmation group, the "Announcement preview" group
and a `role=alert` refusal are each visible, in both themes;
`web/e2e/layout.spec.ts` reports Push, Cancel, Channel, Announce now and the
form's fields among the controls reached, each at least 99 % in view, at 640
px; and the hermetic Settings scan waits on the Retry button, so a
deliberate delay in the config read does not change what axe reports on.

## Deliberate trade-offs that carry a cost

These were chosen on purpose and are written down at their sites. They are
not debt; they are listed because each one costs something a reader should
know about.

- **Every package with a declared file budget sits exactly at it** — the
  numbers are in `scripts/package-size-budgets.txt`, and
  `scripts/check-package-size.sh --list` shows the standings. That is the
  gate working as designed, since a budget left above its count fails too:
  the next file in any of them is a decision (a split, or a bump with the
  WHY rewritten and a history row), not an accident. A directory the file
  does not list answers to the default, which `internal/forge` fills
  exactly, so its next file is a first entry with its WHY. The cost is
  that any change adding a file there must carry its budget row in the
  same commit or fail `task check`.
- **Seven test files stay past the 500-line soft target**, all under the
  800 ceiling (`scripts/check-file-length.sh --list`). They were left whole
  on purpose when the source files past the target were split by concern;
  each holds the cases of one behavior. Announcing:
  `internal/messaging/post_test.go` (766, the post to each service and the
  announcement's text) and `internal/tui/messaging_test.go` (612, the
  terminal's Messaging pane). Opening a pull request:
  `internal/webserver/pullrequest_test.go` (601, the web's draft and open).
  Staging, committing and pushing: `internal/tui/composer_test.go` (568,
  the terminal's commit composer), `internal/webserver/staging_test.go`
  (533, the web's stage and unstage) and
  `web/src/features/branch/BranchPanel.test.tsx` (517, the web's commit and
  push). Reading and writing an issue: `internal/jira/detail_test.go` (501,
  Jira's issue read, comment and pull request link). The cost is that
  `scripts/check-file-length.sh` still warns on every run, so a source file
  newly past the target is one more line among seven a reader has learned
  to skim.
- **The web is read-only under `--dry-run`** (`internal/webserver/guard.go:40`
  documents it): every unsafe method answers 403 at one gate, where the
  terminal simulates each write and narrates it. The browser reads
  `dry_run` from `getHealth`, says so in a banner, and holds every write
  before sending it (`web/src/api/client.ts:25`), so the server's 403 is
  only the backstop. The cost is that a web write under dry run is refused
  outright rather than simulated: the browser cannot show what the write
  would have done, as the terminal's narration does. The blanket refusal
  itself is the intended design.
- **`staleTime: Infinity`** (`web/src/queryClient.ts:12`) with the event
  stream as the sole freshness source. Correct for a pushed snapshot; the
  cost is that a stalled stream leaves stale data with no refetch to fall
  back on. Three queries set their own `staleTime`. The issue detail's and
  the review queue's are a minute (`web/src/features/issues/issueApi.ts:23`,
  `web/src/features/reviewqueue/reviewQueueApi.ts:23`): the stream carries
  only the list's slim issues and never the queue, so each is read again
  when reopened after a minute, and the queue's Refresh reads it at once.
  The configuration's is 0 (`web/src/features/settings/configApi.ts:65`):
  the file can change on disk, which no event reports, so Settings and the
  commit form read it again each time they open. A save that still meets a
  change it has not seen is refused (409) and nothing is written; Settings
  offers **Reload**.
- **The progress spine's per-system hue is color-only**
  (`internal/tui/spine.go:68`), mitigated by the stage name, or its initial
  when compact (`internal/tui/spine.go:51`). Part of the visual system UX.md
  says should not change; the cost is one channel the monochrome reader does
  not get.
- **The two composers' field handling is written twice.** The commit and
  pull request composers each pair an `onFieldNav` with a `*CanComplete`
  check (`commitComposer.onFieldNav`, `internal/tui/scopesuggest.go:17`;
  `prComposer.onFieldNav`, `internal/tui/prcomposer.go:304`), and each blurs
  every field before focusing one (`commitComposer.focusOn`,
  `internal/tui/composer.go:297`; `prComposer.focusOn`,
  `internal/tui/prcomposer.go:325`). Two is not yet the rule of three, so
  they stay apart until a third composer needs them. The cost is that a
  change to field navigation is made twice, and a third composer must copy
  the pairs or extract them then.
- **A condition-coverage skip list of two** (`UNANALYZABLE`,
  `scripts/gobco-report.sh:82`): gobco ignores build tags, so it cannot read
  a package whose files come in tagged twins, and `internal/proc/pgroup` and
  `internal/web` are named there with that reason beside them. Every other
  package is read, and one that becomes unreadable without being named fails
  the gate rather than shrinking the number. The cost is that the two named
  packages' conditions go unmeasured — platform glue and an embed stub, with
  no branch worth the count — and that the next tagged twin must join them.
- **The web's branch floor is v8's range-based count**
  (`web/vitest.config.ts:45` `thresholds`), not a gobco-style per-condition
  one: v8 marks a branch covered once its range of code has run, and never
  asks which way each operand of a condition went. The cost is that an
  `a && b` only ever seen with `b` true still passes the web's floor.
