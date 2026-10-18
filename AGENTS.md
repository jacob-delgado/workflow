# AGENTS.md

For a coding agent working on this repository. `CLAUDE.md` holds the rules,
and they bind every agent, not Claude Code alone: read it first, and the
`CLAUDE.md` beside a directory's code before your first edit there (its *Area
guides* table lists them). Where those files say Claude, read yourself. This
file repeats only what an agent most often gets wrong; where it differs from
`CLAUDE.md`, `CLAUDE.md` wins, and for *Working as a lane* the `cloud-lanes`
skill (`.claude/skills/cloud-lanes/SKILL.md`) wins.

## Before anything else

- **Branch, never `main`.** A change lands through a branch named for its
  commit prefix and a pull request with green CI. Start from a fresh
  `origin/main` (`git fetch origin` first), move any uncommitted work on
  `main` to a branch before committing, and rebase on `origin/main`, never
  merge it in.
- **Hooks on.** Never bypass a hook to land work: no `LEFTHOOK=0`,
  `LEFTHOOK_EXCLUDE` or `--no-verify`. A failing hook is fixed at its cause.
  The one exclusion is the `cloud-lanes` skill's push rule, for a push of the
  exact commit the gate as dev passed.
- **Test first.** A new feature or a bug fix starts with a failing test you
  have run and seen fail, black-box in `package <name>_test`; every Go and
  web test marks its parts with `// Arrange`, `// Act` and `// Assert`
  (`CLAUDE.md`'s *Testing* says how).
- **The gate before you say done.** `task check`, as a non-root user, and
  `task e2e` for a change under `web/` or `internal/webserver`. Where you are
  root, run it as another user that owns a checkout; where there is none,
  take CI's Branch coverage job as the verdict and say so. Never call work
  done while the build is red: say what is broken and why.
- **Ask, don't assume.** Propose a new dependency, by name and why, and wait
  for approval; never adopt a version published under 7 days ago. For a
  change that would touch more than about three files or restructure a
  package, outline it and wait for confirmation before writing code. Write
  commits that feed a release, but never merge the release pull request,
  push a `v*` tag or run the release workflow. A decision that is the
  maintainer's waits for the maintainer.
- **Never log or print a secret.** A credential reaches output only through
  `config.Redact`, and the store never holds one.

## Working as a lane

A lane is an agent another session started to do part of the work, in a
worktree of its own. The `cloud-lanes` skill holds the detail; in brief:

- **Read state from git.** Run `git status`, `git log` and `git reflog`
  yourself; a SHA in your prompt may be stale. An earlier attempt at your own
  step, cut short by a restart, may already be in `git reflog -15`: redo
  only what it left undone, and leave any reset or stash to the main session.
- **Stay in your worktree,** and never push a branch you do not own.
- **The maintainer's latest message is not always yours.** You may see it
  beside your prompt. When your prompt says what that message answered and
  that you carry on, carry on, and say so in your report. When it does not,
  and the message could be about your task, ask the main session rather than
  stop or carry on on a guess.
- **Run the gate yourself** before you report done: `gate-as-dev <worktree>`
  where the session-start hook linked it, and otherwise (an agent other than
  Claude Code, or a session older than the hook) the worktree's own
  `bash scripts/gate-as-dev.sh <worktree>`, never another worktree's copy.
  Without the hook, export `PLAYWRIGHT_CHROMIUM_EXECUTABLE`,
  `COREPACK_NPM_REGISTRY` and `ZIZMOR_OFFLINE=true` yourself in each shell
  that builds, tests or commits (the skill gives their values and why).
  Report the gate's last lines as they are; when it refuses to start under
  8 GB free, report and stop.
- **Mind the disk.** Check `df -h /` before a build; under 3 GB free, stop and
  report, clearing nothing shared.
- **A refusal stops you.** When the auto-mode classifier or a permission
  prompt refuses a step (a `git reset`, a rebase that drops a commit, a
  sweep), report and stop; do not look for another way around. A refused
  network host is not this: say which host and carry on with what does not
  need it.
- **Settle everything.** Every finding, nits included, is fixed or answered
  with evidence that it is wrong; nothing is left as a follow-up.
