---
name: cloud-lanes
description: What a Claude Code cloud session on this repository has to know to work on its own - the container's toolchain, network, disk and browsers, the gate to run before a push, how to write a lane's prompt, and how to recover lanes after a usage-limit cutoff or a workflow resume. Read it before starting lanes (workflows, subagents or cloud sessions), before running the gate or pushing from a cloud session, and whenever the container gets in the way (a tool missing or at the wrong version, a host refused, the disk full, an agent cut off).
---

# Cloud lanes

What the cloud container does to this repository's work, and what to do about
it, learned between October 5 and 17, 2026, at the cost of red CI runs, a full
disk and lanes cut off part way. The root `CLAUDE.md` holds the rules every
session needs; this holds the detail behind them.

## The container

- **The toolchain.** The image carries other versions of Go, golangci-lint
  and node than `mise.toml` pins, with `task` and `lefthook` off PATH. The
  SessionStart hook (`.claude/hooks/session-start.sh`) runs
  `scripts/cloud-session-setup.sh`, which installs the tools `task check` and
  the hooks run, and deadcode for the advisory `task deadcode`, into
  `/opt/workflow-tools`, with a copy of `gate-as-dev` that root owns, and
  links them into `~/.local/bin`; hugo-extended, air and cloc, which no gate
  runs, are left out. Confirm it took: `go version` and
  `golangci-lint --version` name `mise.toml`'s pins. If they name the
  image's, read `~/.cache/cloud-session-setup.log` and rerun the script.
- **PATH.** The shell commands run in does not read `~/.bashrc`, so a change
  there reaches no command. A tool reaches PATH by a link in `~/.local/bin`,
  or by a line in the session's `CLAUDE_ENV_FILE`, which the hook writes.
- **The user.** The session is root. Root reads any file, so the tests that
  make one unreadable skip, and gobco's verdict does not hold. The gate runs
  as the non-root `dev` user (`gate-as-dev`, below), which reaches the tools
  through root's `~/.local/bin` links; `/root` is readable by others here.
  The main checkout and its `.git` are writable by every user in this image,
  so code the gate runs as dev could change what root runs next; and with
  `fs.protected_symlinks`, `protected_regular` and `protected_fifos` all 0
  here, it could leave a link or a pipe at a name in `/tmp` that root's tools
  open, such as golangci-lint's lock. dev keeps the gate's verdict honest,
  not the session safe.
- **CPU and memory.** Four cores and about 15 GB. With three or more agents
  building at once the load reaches about 30, and vitest's timeouts fire for
  load, not for a bug: rerun when the machine is quieter before calling one
  real.

### Browsers

- **Chromium alone, and not the build Playwright pins.** `/opt/pw-browsers`
  holds Chromium revision 1194 (Chromium 141), both the full browser and its
  headless shell; Playwright 1.64 wants revision 1248, and Playwright's
  download host is refused, so `playwright install` cannot help: never run
  it. Both Playwright configs launch the browser
  `PLAYWRIGHT_CHROMIUM_EXECUTABLE` names, and the SessionStart hook sets it to
  the image's headless shell, the kind of binary CI runs
  (`/opt/pw-browsers/chromium_headless_shell-1194/chrome-linux/headless_shell`
  here), so `task e2e` runs on the image's Chromium. A spec that passes here
  on 141 can still fail in CI on the pinned build: CI's run is the verdict.
- **Other engines.** Only Chromium runs in this container. A verdict on
  Firefox or WebKit comes from CI only where the branch's `ci.yml` has a job
  for that engine; main's runs Chromium alone.
- **Before naming a cause in an engine,** read the revision Playwright ships
  (`web/node_modules/playwright-core/browsers.json`) and that build's base,
  not upstream's main (see *Premises*, below).

### The network

| Host | Here | What it means |
| --- | --- | --- |
| GitHub release downloads | reachable | the setup's binaries |
| `nodejs.org` | reachable | node |
| `proxy.golang.org`, `sum.golang.org` | reachable | Go, the Go tools, every module |
| `registry.npmjs.org`, `pypi.org` | reachable | Yarn (with `COREPACK_NPM_REGISTRY`), npm packages, yamllint |
| `api.github.com` | the session's own repositories only | a third-party repository answers 403 and names `add_repo`: mise's aqua backend and `task deps:age`'s dating of a GitHub-released mise tool or of mise's `min_version` fail |
| `github.com` over git | refuses the session's token for a third-party repository, with 401 | zizmor's online audits, which run whenever a GitHub token is in their environment (offline here, TRADE-54) |
| `go.dev` | refused | mise's Go install; Go comes from the `golang.org/toolchain` module instead |
| `vuln.go.dev` | refused | `task vuln`; the local gate leaves it to CI (TRADE-54) |
| `repo.yarnpkg.com` | refused | corepack's default Yarn source; the hook points corepack at the npm registry |
| Playwright's download host | refused | the pinned browsers; see *Browsers* |
| the Actions artifact hosts (`productionresultssa*.blob.core.windows.net`) | refused | traces and reports from CI; read the job's log with the GitHub MCP `get_job_logs` instead |

A refused host is the environment's network policy, which the maintainer
changes in the environment's settings; a third-party repository's 403 on
`api.github.com` is the session's GitHub access, which `add_repo` asks for.
Say which host and why, and carry on with what does not need it.

### The disk

- **It is an allowance, not the filesystem.** About 38 GB in all. `df`'s
  Size is the disk underneath; its Avail is what this session has left.
- **What fills it.** A race-and-coverage build keeps up to about 7 GB of Go
  build cache per user, the module cache about 5 GB, each worktree with its
  web dependencies about 400 MB. Root's lanes share one Go build cache, and
  dev's gate keeps another.
- **What a full disk looks like.** A build dies with "no space left on
  device", and the session's shell cannot even print a command's output: the
  tool reports ENOSPC on its own tasks directory.
- **Who clears what.** `gate-as-dev` manages dev's cache itself: under 10 GB
  free it clears it before a run, and under 8 GB even so it does not start;
  between steps it clears it under 3 GB and stops under 2 GB. Root's caches
  are every lane's: with several lanes going, a lane checks `df -h /` before a
  build and stops to report under 3 GB free, clearing nothing shared, while
  the main session clears what regenerates once free space is under 5 GB, and,
  before a gate run with under 10 GB free and no lane building, clears root's
  Go build cache (`go clean -cache`) so the gate can start. Root's module
  cache, a reviewer that builds every commit (5 GB of build cache in an hour)
  and the lanes' scratch together leave less than that: one PR went three
  review rounds with no gate verdict. Remove a finished worktree with
  `git worktree remove`.
- **What the main session can clear.**
  - Root's Go build cache. While lanes build it is trimmed, not cleared:
    `find ~/.cache/go-build -type f -mmin +120 -delete`. Go refreshes an
    entry's modification time when it uses it, about hourly, so what a
    running build reads stays. `go clean -cache` waits until no lane is
    building.
  - Root's Yarn download cache, `~/.yarn/berry/cache`: about 320 MB for the
    current lockfile right after a clear, and over 600 MB once old versions
    pile up after lockfile changes. Yarn reads it only when it installs:
    `web/.yarnrc.yml` sets `nodeLinker: node-modules`, so a worktree runs
    from its own `web/node_modules`.
  - Old Go toolchains in root's module cache,
    `golang.org/toolchain@v0.0.1-go1.x`, about 250 MB each, and their
    download zips under `cache/download/golang.org/toolchain/@v/`, once the
    pinned Go is installed: left to choose its toolchain, the go command only
    switches up from the Go it is, never down to an older one. Keep the
    pinned one, which the setup repairs Go from.
- **Watching it.** Free space swings by about 2 GB as `gate-as-dev` and the
  lanes fill and clear their build caches, so act on a trend over a few
  readings, not on one. The main session keeps a Monitor on it, a loop over
  `df -Pk /` every 15 seconds that prints a line when free space falls under
  5 GB and again once it is back over 6 GB, so a swing across one threshold
  does not print a line each way.
- **What not to touch.** Never sweep `/dev/shm`, another session's scratch or
  another lane's worktree.

## The gate before every push

- **Hooks on.** The Working agreement's hook rule holds here too, and with the
  toolchain linked nothing in a cloud session needs `LEFTHOOK=0`. Six of the
  20 CI failures on our branches from October 8 to 15 were checks the hooks
  run: two subjects over 72 characters, a typo, a markdownlint rule, and docs
  drift twice. Of the 20, 10 were ones the local gate catches as it stands, 5
  more ones it catches with the pinned tools, 4 were flakes or races, and 1
  needed an engine this container lacks.
- **The maintainer's push rule.** When `gate-as-dev` passed on the exact
  commit being pushed, push with `LEFTHOOK_EXCLUDE=unit-go,web`: the gate ran
  those, and the pre-push hook's other checks still run. Any other push runs
  the whole hook. `LEFTHOOK=0` the classifier refuses.
- **`gate-as-dev <worktree> [task...]`** (`scripts/gate-as-dev.sh`), as root,
  hands the run to dev in dev's own checkout of that worktree's HEAD: what is
  committed, never the uncommitted changes, which it warns of. Given no task
  it runs all of `task check` but `vuln` while `vuln.go.dev` is refused. It
  checks the disk before the run and between steps, and one run goes at a
  time: a second waits for the first. dev gets a temporary directory of its
  own under `~/.cache`, since root's tools keep fixed names in `/tmp`: a
  root golangci-lint's lock there would fail dev's `lint:go` with "parallel
  golangci-lint is running".
- **A worktree is not a clone.** A lane's worktree has a `.git` file; dev's
  gate checkout and CI's checkout have a `.git` directory, of objects and
  git's own sample hooks. So a check that walks hidden files can pass in a
  worktree and fail in a clone, which is what CI and `gate-as-dev` check
  out: `typos` did, in a fresh clone, until `_typos.toml` excluded `.git`.
  `gate-as-dev` is what catches that.
- **What the hooks run** (`lefthook.yml`). The pre-commit hook reads the
  staged files alone: golangci-lint on the lines the commit changes, the
  license headers of staged Go and the Arrange-Act-Assert markers of each
  staged test's package, shfmt, shellcheck, yamllint, hadolint, typos,
  markdownlint, taplo and zizmor on what is staged, and gitleaks on the
  staged diff; actionlint and the sibling-test check read the whole tree.
  The commit-msg hook runs `scripts/check-commit-message.sh`. No hook runs
  `docs:check`, the coverage floors, the whole-tree secret scan, govulncheck,
  or, golangci-lint aside (below), those linters over the whole tree: those
  are `task check`'s and CI's.
- **The pre-push hook is part of the gate, not the gate.** It runs
  `task test` (unit-go) and golangci-lint when the push carries a Go change,
  the web's lint, client-drift check and unit tests (web) when it carries a
  `web/` or API-spec change, and the file-length, package-size, trade-off
  and `TECH_DEBT.md` path checks on every push. On a branch new to the
  remote, the range it checks takes in changes the branch did not make, so
  unit-go and web run even for a branch that changes only scripts.
  `task test` alone takes about 15 minutes here, which is why the gate runs
  before the push and not because of it.
- **A web change** runs `task e2e` too (see *Browsers*).
- **What catches what.**

| CI failed on | Caught here by |
| --- | --- |
| a commit subject or body | the commit-msg hook, `scripts/check-commit-message.sh` |
| typos, markdownlint, shellcheck | the pre-commit hook, `task lint` |
| the command reference out of date | `task lint` (`docs:check`) |
| a docs test, any other test | `task test`, which the pre-push hook runs when the push carries a Go change; after a docs-only change, run it yourself |
| a file too long, a package over its budget, a stale trade-off site or `TECH_DEBT.md` path | the pre-push hook on every push, `task lint` |
| the statement coverage floor | `task test:cover` |
| a condition no test evaluated | `task cover:branch`, as dev |
| golangci-lint against a new Go | the pinned pair from the setup, `task lint:go` |
| a timing flake | `go test -race -count=20 -run <test> ./<pkg>`; `--repeat-each` for a spec |
| a dependency under a week old | `task deps:age` in the lane's own worktree, for an npm or Go module and for a `mise.toml` pin of go, node, yamllint or a `go:` or `npm:` tool; a GitHub-released mise tool, or mise's `min_version`, is dated only in CI; no script dates an Action bump, so check its release date by hand |
| a new Go advisory | CI only, while `vuln.go.dev` is refused |
| an engine other than Chromium | CI only |

## Writing a lane's prompt

A lane, whether a workflow agent, a subagent or a cloud session, starts with
`CLAUDE.md` and none of the conversation. Its prompt carries:

- **Where.** Its worktree and branch, and that it touches nothing else; it
  never pushes a branch it does not own. A cloud session's harness names its
  branch (`claude/…`); the branches its lanes push are named from that one.
- **State from git.** It reads `git status` and `git log` itself. A SHA
  pasted into a prompt is stale after the first rebase.
- **The rules that bite.** Hooks on; test-first with the red line quoted; the
  exact commit trailers, copied from the main session's attribution lines,
  since a lane started as its own cloud session is shown its own session's
  lines instead; and no deferring: every finding, nits included, is settled
  or answered with evidence that it is wrong, with nothing left as a
  follow-up. It asks the main session when a decision is the maintainer's.
- **The gate.** It runs `gate-as-dev <worktree>` before reporting done, and
  reports what it said.
- **The disk.** It checks `df -h /` before a build and stops to report under
  3 GB free, clearing nothing shared (see *The disk*, above).
- **Refusals.** When the auto-mode classifier refuses a step (a `git reset`,
  a rebase that drops a commit, a sweep), it reports and stops; it does not
  look for another way around. The main session decides.
- **For a reviewer.** It works in a detached worktree of its own under the
  session's scratchpad (`git worktree add --detach <dir> <sha>`, the lane's
  `web/node_modules` linked in when it needs them), removes it with
  `git worktree remove --force` after, and answers sound only for what it
  confirmed.

## Usage limits and workflow resumes

- **The five-hour window is what cuts lanes off,** not the weekly budget.
  Before a cutoff can land, the main session arms a `send_later` wake a few
  minutes after `get_session`'s `rate_limit_info.resetsAt`, and keeps a
  state file in its scratchpad (the worktrees, branches, workflow run ids,
  pull requests and next steps) that the wake reads first.
- **A cutoff stops an agent mid-step.** One stopped right after a
  `git reset`, its rebuild half done and uncommitted. Before relaunching
  anything, the main session reads `git status` and `git reflog -10` in every
  lane's worktree. Where work is half done, it keeps it aside first
  (`git stash push --include-untracked -m "cutoff <lane>"`), then moves the
  branch back to the reflog's last complete commit (`git reset --keep <sha>`),
  and tells the next agent the stash is there. A lane never does this itself.
- **Probe first.** A one-agent workflow that answers "ok" shows whether
  agents run again before a fleet is relaunched into the limit.
- **A resume replays only the unchanged start.** `resumeFromRunId` returns
  cached results for the longest unchanged prefix of `agent()` calls; from the
  first call whose prompt or options differ, every call runs live, the same
  or not. A stage whose prompt embeds an earlier agent's result can miss the
  cache and start over on a branch that already holds its work. After a
  resume, read the run's journal (`journal.jsonl` in the transcript directory
  the launch names); if a finished stage started again, stop the run at once
  and write a short continuation script that starts at the right round.
- **An empty review is no verdict.** An agent a cutoff stopped returns
  nothing, and a script that read an empty findings list as sound reported
  two cut-off review rounds as sound. A workflow script counts the agents
  that returned nothing, and retries the round or stops when there are any;
  it calls a round sound only on reports that say so.
- **Review rounds cost the budget a cutoff comes from.** Fix a reviewer's
  last nits in the main session rather than paying for another round.

## Premises that rest on an outside version

Before naming a cause in a browser, a tool or a library, read the version the
repository actually runs, not upstream's main. A WebKit fix was once written
for code WebKit added on September 15, while Playwright's WebKit build came
from a base of August 20: read `web/node_modules/playwright-core/browsers.json`
for the revision, and that build's base, before blaming the engine's source.
When a diagnosis cannot be confirmed here, make the next CI run carry what
settles it, such as a helper that names the source of each refusal.

## Pull requests from a session

- **Merges are serial.** Main merges by rebase only and wants a branch up to
  date, so each pull request rebases on the one before it, and a stacked one
  is retargeted to main once its base merges, its package-size ratchets
  recomputed.
- **A sibling's merge moves main.** Rebase on `origin/main` right before
  pushing. A push still inside its pre-push hook when a sibling pull request
  merges would push a stale base: stop it, rebase and push again.
- **Stopping a push.** Find the `git push` by its exact command line, in
  `/proc/<pid>/cmdline`, and stop that process: `pgrep -f` matches the shell
  running `pgrep` as well. The classifier refuses `ps` over test processes as
  interfering with workloads, so whatever the stopped hook left running, its
  tests, is left to end on its own.
- **CI takes about ten minutes** and a red run about an hour to turn green,
  which is why the gate above runs first.
- **Watch, don't poll.** `subscribe_pr_activity` brings CI and reviews to the
  session as events.
