# Surface review: the command line, the terminal interface and the web

A plan for a session to execute. It is the one file in this repository that
ranks and orders work — because it is a handoff, not a backlog. Delete it
when it is done. The durable records are [UX.md](UX.md) (the user-facing
gaps, `UX-nn`), [TECH_DEBT.md](TECH_DEBT.md) (the debts, `DEBT-nn`) and
[FEATURES.md](FEATURES.md) (the ideas, `FEAT-nn`); every item below points
at its record there, and an item that is fixed is removed from its record in
the same pull request, per the standing rule.

Checked against commit `5e69cb8` on 2026-09-22 (PR #125's tip, merged to main — the state that includes the web
features, the on-disk store and the RFC 9457 errors). Line numbers drift, so
every pointer also names the symbol it means. Re-verify a cited line before
acting on it.

## How this was produced

Three read-only maps of the code — the command line (`internal/cli`,
`cmd/`), the terminal interface (`internal/tui`) and the web (`web/src`,
`internal/webserver`, `api/openapi.yaml`) — each producing a list of what a
user can *do* from that surface, and a review of that surface against a
yardstick:

- **Command line:** the [clig.dev](https://clig.dev) guidelines — help and
  discoverability, exit codes that distinguish failures, stdout for the
  artifact and stderr for commentary, machine-readable output and
  pipeability, confirmation with `--yes` to skip, `--dry-run`, actionable
  errors.
- **Terminal interface:** the promises it makes about itself (re-counted in
  UX.md's table) plus the conventions of a developer-facing TUI —
  discoverable keys, one key one meaning, `Esc`/`Enter`, feedback and a
  last look before anything outward, small-terminal and `NO_COLOR` behavior.
- **Web:** WCAG 2.1 AA (already gate-enforced) plus a designer's read: does
  the page share the product's visual system or a template's, do buttons
  say what happens and keep their names, do errors direct, is there
  feedback after a write, does the layout respond.

**Nothing was run.** No binary was driven and no browser was opened; the
screens are known from the source and from the golden output the tests
hold. Every claim is cited; the ones that are about how something *looks*
say "from code". A second reading of every cited line was made while
writing this file, and the phasing was designed against the import graph
(`go list -deps`), the package budgets (`scripts/check-package-size.sh
--list`) and the condition-coverage report at this commit.

## The parity matrix

**Y** yes · **P** partial · **N** no · **F** a deliberate, declared follow-up.
The last column answers the question that matters: *is an equivalent
warranted?* Parity is not the goal — a mouse gesture needs no CLI twin and
`git push` is the CLI's own push — so each "N" says why it is, or is not, a
gap, and which phase closes it.

### Issues

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 1 | List the view's issues | P (only `branch <tab>` completion, `completeAssignedIssues`, `internal/cli/scriptable.go:125`) | Y | Y (the stream's page, and more on request since Phase 3) | CLI read: a feature, FEAT-78 |
| 2 | Switch view / next page | N | Y (`v`, `ctrl+n`) | Y since Phase 3 — a view select from `listViews` (`ViewSelect`, `web/src/features/issues/IssueListControls.tsx:37`), `useEventStream(view)` (`web/src/api/snapshot.ts:34`), and Load more over `start_at` (`useMoreIssues`, `web/src/features/issues/issueApi.ts:73`); an unknown view is 404 | — |
| 3 | Filter the list | N | Y (`/`, `internal/tui/issues.go:159`) | Y since Phase 3 — the same `KEY summary` match (`matchesFilter`, `web/src/features/issues/IssuesPanel.tsx:311`) | CLI no |
| 4 | Read an issue in full | N | Y (`internal/tui/detail.go:285`) | Y since Phase 3 — description, comments, reporter, assignee (`IssueDetailPanel`, `web/src/features/issues/IssueDetailPanel.tsx:16`) | CLI: FEAT-78 |
| 5 | Transition, with field forms | P (`pr`'s side effect, fields-less, `internal/cli/pr.go:236`) | Y (`internal/tui/picker.go:155`) | N | The post-PR **review-status offer** both other surfaces make: Phase 9, UX-74. A general transition: FEAT-78 (CLI), FEAT-80 (web) |
| 6 | Comment | N | Y (`internal/tui/comment.go:41`) | N | FEAT-78 / FEAT-80; not this plan |
| 7 | Assign / log work | N | Y (`internal/tui/issuewrite.go:74`, `:81`) | N | FEAT-78 / FEAT-80; not this plan |
| 8 | Link the pull request on the issue | Y since Phase 8 — `pr` offers it before the status, under the same `--yes` (`offerLink`, `internal/cli/pr.go:176`) | Y (`issuelink.go`, `internal/tui/prcomposer.go:510`) | N | **Web yes** — the seam exists (`Jira.LinkPullRequest`) and the flow is the interface's. Phase 9, UX-74 |
| 9 | Open / copy the issue URL | n/a | Y (`o`, `y`) | Y since Phase 3 — "Open in Jira" from the detail's `url` (`web/src/features/issues/IssueDetailPanel.tsx:77`); copying is the browser's own link menu | — |
| 10 | Cache-seeded first paint | n/a | Y (`internal/tui/issues.go:53`) | **F** | FEAT-68, declared |
| 11 | Branch for the issue | Y | Y | Y | The CLI's help and preview say it switches to the branch, since Phase 8 (`internal/cli/branch.go:40`, `:99`) |

### Branch

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 12 | Create in a worktree | N | Y (`ctrl+w`, `internal/tui/branch.go:404`) | N | Idea, UX-89; not a gap |
| 13 | Task switch (check out an issue branch) | N | Y (`s`) | Y (`POST /api/checkout`) | CLI: `git switch` is the twin — no |
| 14 | Push | P (inside `pr` only) | Y (`P`, previewed) | Y | CLI: `git push` is the twin — no |
| 15 | Rebase onto base | N | Y, previewed since Phase 4 (`previewRebase`, `internal/tui/branch.go:215`, through `lastLook`) | N | CLI/web: `git rebase` |
| 16 | Finish a merged branch | N | Y (`F`, `finish.go`) | N | A composed three-command flow: FEAT-83 (CLI), FEAT-79 (web); not this plan |

### Commits

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 17 | Stage / unstage / stage all | N | Y (`space`, `a`) | **N** — the list is read-only (`BranchPanel.tsx:86`) and the form appears only when already staged (`:111`) | CLI: `git add`. **Web yes** — its own commit flow is unreachable from the browser. Phase 10, UX-75 |
| 18 | Discard a change | N | N | N | FEAT-23, already filed; all three lack it |
| 19 | Per-file diff | N | Y (`internal/tui/diff.go:29`) | N | Web idea, UX-88 |
| 20 | Commit with the convention | N | Y | Y (`CommitForm.tsx:36`) | CLI: `git commit` + the commit-msg hook |
| 21 | Scope from `commit.default_scope` / the learned scope | n/a | Y (`internal/tui/composer.go:131`) | **N** — `CommitForm.tsx:45` hardcodes `''`, though `default_scope` is *editable* in Settings (`SettingsPanel.tsx:182`) | **Yes** — a setting silently ignored is a defect; the learned scope is not a declared follow-up. Phase 10, UX-76 |
| 22 | Amend / fixup | N | Y | N | git is the twin; web idea, UX-88 |
| 23 | Run a hook / generate `lefthook.yml` | N | Y (`h`, `g`) | N | `lefthook` is the twin — no |

### Review

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 24 | Find the pull request and its CI | Y (`status --json`) | Y | Y | — |
| 25 | Compose and open (push first) | Y (`pr`) | Y (`n`) | Y | CLI and web compose it once, in `internal/loop` (Phase 1, which closed DEBT-50); the terminal's composer is its own editor. CLI lacks draft/base/reviewer flags; CLI and web take `templates[0]` only: UX-62 |
| 26 | After opening: link, then the review-status offer | Y since Phase 8 (`followUp`, `internal/cli/pr.go:163`) | Y | **N** (`webserver/pullrequest.go` never touches `Jira.ReviewStatus`) | **Web yes** — Phase 9, UX-74 |
| 27 | Edit the pull request | N | Y (`e`, `preditor.go`) | N | `gh pr edit` is the twin; web idea, FEAT-79 |
| 28 | Checks list; a failure into `$EDITOR` | N | Y (`checks.go`) | P (a CI link) | A terminal gesture; web list idea, UX-88 |
| 29 | Follow CI; notify on settle | n/a | Y (`internal/tui/review.go:189`) | P (5 s tick, no signal) | Web "CI settled" idea, UX-86 |
| 30 | Re-run CI | N | Y, previewed since Phase 4 (`previewRerun`, `internal/tui/checks.go:183`, through `lastLook`) | **F** | `gh run rerun` is the CLI's. Web declared (`FEATURES.md:148`), FEAT-79 |
| 31 | Merge | N | Y (`M`, gated `internal/tui/merge.go:23`) | **F** | FEAT-31 chose the interface; web declared (`FEATURES.md:150`), FEAT-79 |
| 32 | The review queue | Y (`reviews --json`) | Y (pane 6) | **N** (no section) | **Yes** — a read both other surfaces have, over a seam that exists. Phase 12, UX-83 |

### Messaging

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 33 | Announce: compose → preview → post | Y | Y | Y | CLI and web compose it once, in `internal/loop` (Phase 1, which closed DEBT-50); the terminal renders its own from cached state, with `loop.AnnounceMoment` |
| 34 | Call it a "merge request" on GitLab | Y (`forge.Kind.Noun`, `internal/forge/remote.go:60`) | Y (the same) | **N** — six hardcoded strings; `ForgeKind` on the server (`internal/webserver/webserver.go:61`), never sent | **Yes** — Phase 9, UX-73 |
| 35 | Post when CI passes | N | Y (`internal/tui/messaging.go:406`) | N | CLI `announce --when-green` is FEAT-65's fit; web FEAT-82; not this plan |
| 36 | Announced history (never re-offer) | Y since Phase 8 — `announce` records each post in the store the interface reads, and says when an earlier session already announced the moment (`offerAgain`, `internal/cli/announce.go:167`; `loop.Deliver`, `internal/loop/announce.go:179`) | Y (`internal/tui/messaging.go:216`) | **F** | Web: FEAT-66, declared |
| 37 | Choose the channel | N (config only) | Y (`←`/`→`) | Y | CLI `--channel`: UX-62 |
| 38 | Standup | Y | N | N | Reasonable CLI-only (an `$EDITOR` flow); `--dry-run`/`--yes` since Phase 2 (`offerToPost`, `internal/cli/standup.go:144`) |

### Across the loop

| # | Action | CLI | TUI | Web | Warranted? |
| --- | --- | --- | --- | --- | --- |
| 39 | Read the config, redacted | Y (`config show`) | P (`internal/tui/render.go:423`) | Y | Fine as is |
| 40 | Write / initialize the config | Y (`config init`) | N | P (7 sections; five carried but not editable) | Interface: `config init` + `doctor` are the path — no. Web remainder: UX-87 |
| 41 | Doctor | Y | N | N | Reasonable CLI-only; the interface points at it (`internal/tui/render.go:443`) |
| 42 | Status across the loop | Y (`status DIR…`) | Y (the spine) | Y (`WorkStory`) | — |
| 43 | Dry run | Y (one persistent `--dry-run` since Phase 2, `internal/cli/cli.go:204`) | Y (per seam, `dryrun.go:25`) | Y since Phase 3 — a read-only banner (`web/src/shell/AppShell.tsx:54`) and one hold over every write before it is sent (`web/src/api/client.ts:24`), over the server's 403 (`guard.go:46`) | — |
| 44 | Request log `--log` | Y (persistent since Phase 2, `internal/cli/cli.go:206`) | Y | Y | — |
| 45 | Help / discoverability | Y since Phase 8 — an unknown command or flag points at `--help`, after cobra's closest commands (`usageHint`, `internal/cli/scriptable.go:210`) | Y since Phase 7 — `?` lists all 57 bindings where each works, held to a table of every placement (`internal/tui/help_test.go:211`), and the Issues footer shows every key it answers (`internal/tui/detail.go:186`) | n/a | Web `?` idea, UX-88 |
| 46 | Version | Y | n/a | Y since Phase 3 — in the header (`web/src/shell/AppShell.tsx:43`) | — |

## Where a surface breaks a convention

The detail — today's behavior, the evidence, a proposal and a Done-when —
lives in the UX entry each row names. This is the summary a session needs
to see the shape.

### The command line (clig.dev)

| Convention | Verdict | Where |
| --- | --- | --- |
| `--help` on every command; useful long help | Met | `internal/cli/cli.go:26` `longHelp`; pinned by `internal/cli/cli_test.go:295` |
| `--version` | Met, and in the generated reference since Phase 2 | `cmd/docsgen/main.go` |
| Exit codes distinguish failure kinds | Met since Phase 2 — 0/1/2/3/4/5/130 | `cli.ExitStatus`, `internal/cli/scriptable.go:265`; `docs/content/docs/scripting.md` |
| stdout = artifact, stderr = commentary | Met since Phase 2 | `output`, `internal/cli/scriptable.go:29` |
| Root flags compose with subcommands | Met since Phase 2 for `--dry-run` and `--log`; `--web` is the root's alone | `internal/cli/cli.go:204` |
| POSIX short flags | **Gap** — none declared | UX-62 |
| `--json` on reads | Partial — `status`, `reviews`, `doctor`, and `config show` parses since Phase 2; not `standup` | UX-62 |
| No color/prompts off a TTY; `NO_COLOR` | Met by construction (no color emitted); a prompt with no terminal says to pass `--yes` since Phase 2 | `errNoTerminal`, `internal/cli/prompt.go:14` |
| Confirm before outward acts; `--yes`; `--dry-run` | Met, on `standup` and `config init` too since Phase 2 | `writeOptions.proceed`, `internal/cli/scriptable.go:81` |
| Preview before the write | Met | `internal/cli/branch.go:99`, `internal/cli/pr.go:121`, `internal/cli/announce.go:132`, `internal/cli/standup.go:135` |
| Progress for slow operations | **Gap** — silent | UX-61 |
| Ctrl-C | Met | `internal/cli/cli.go:102` `signal.NotifyContext` |
| Errors say what to do next | Met since Phase 8 — strong in `doctor`/`config`, and each refusal names its next step: `git switch NAME`, the open pull request's address, the messaging keys and `workflow doctor`, `workflow pr` | `runBranch`, `internal/cli/branch.go:79`; `composeRefusal`, `internal/cli/pr.go:208`; `runAnnounce`, `internal/cli/announce.go:107` |
| A hint on misuse | Met since Phase 8 — `SilenceErrors` still stops cobra's usage dump; an unknown command or flag points at `--help`, after the closest commands | `usageHint`, `internal/cli/scriptable.go:210` |
| No surprises | Met since Phase 8 — `branch` says it switches; `pr`'s question names the push, and its `--yes` help names the push, the link and the status move it answers | `internal/cli/branch.go:99`; `openQuestion`, `internal/cli/pr.go:264` |
| Secrets never printed | Met | pinned by `internal/cli/cli_test.go:270`, `doctor_json_test.go:65` |
| Shell completion | Met, incl. dynamic issue keys | `internal/cli/scriptable.go:125` |
| Docs cover the commands | Met since Phase 2 | `docs/content/docs/scripting.md` |

### The terminal interface

The seven promises, re-counted, are the table in UX.md. In one line each:
`?` lists every key (57/57, by construction, and since Phase 7 a test
enumerates every placement); one voice for failure is
kept by every site that renders an error's text since Phase 6; a last look before anything outward by
**18 of 18** since Phase 4 gave `R` and `u` one; a refused change stays in
view in **14 of 14** overlays since Phase 5 kept merge's and finish's; panes
fail alone and state is by shape (kept). Beyond the promises: since Phase 7
the Issues footer shows every key it answers, and every key is filed under
the panes or overlays that answer it; `Esc`/`Enter`, loading, success and empty states,
resize down to 24 rows, mouse, `NO_COLOR`/`ui.ascii` and rebinding are all
met and stay.

### The web

| Point | Verdict | Where |
| --- | --- | --- |
| Color tokens; no shadows, gradients or `→` | Met | `web/src/index.css:9-96` |
| Type and spacing scale | **Gap** — none | UX-84 |
| The product's five hues and shape-for-state | **Gap** — one accent, color-only dots | UX-84 |
| ALL-CAPS eyebrow headings | **Gap** — the only heading treatment, nine headings from seven class strings | UX-84 |
| Buttons say what happens | Met | `Open pull request`, `Commit staged changes`, `Push branch` |
| An action keeps its name; errors direct; empty states invite | Partial | UX-79, UX-80 |
| Feedback after a write | **3 of 7**; two successes are not live regions | UX-77 |
| Focus management | **Gap** — none | UX-78 |
| Accessible names, landmarks, skip link, focus ring, axe both themes | Met | enforced |
| Disabled by opacity; reduced motion | **Gap** — thirteen places; no rule | UX-81 |
| Responsive | **Gap** — zero breakpoints | UX-85 |
| Vocabulary shared with the interface | **Gap** — "pull request" on GitLab; `Messaging` vs `Slack`; five sections vs six | UX-73, UX-83 |
| Dry run visible | Met since Phase 3 — a banner, and every write held before it is sent | `web/src/shell/AppShell.tsx:54`, `web/src/api/client.ts:24` |
| The contract it never calls | Met since Phase 3 for `getIssue`, `getHealth`, `listViews` and pagination; the generated `streamEvents` stays unused | DEBT-67 |

## The plan

Sixteen phases in three tracks after the first two — **CLI** (2, 8),
**terminal** (4 → 5 → 6 → 7), **web** (3 → 9 → 10 → 11 → 12 → 13 → 14) — and
15 closes. The maintainer chose to land them **in numeric order as one pull
request** (branch `feat/surface-review`), with an adversarial review after
each phase; each phase is a series of commits, and because `main` merges by
rebase and every commit lands as written, every commit gets the targeted
checks for what it touches and the commit-message check, while the full
gate — `task check` — runs on the last commit of each phase. A phase that
is finished says so in its heading. Before acting on a phase, read its
entry under
[Corrections from the feasibility pass](#corrections-from-the-feasibility-pass)
— they override the phase text where the two disagree. Red-first, black-box tests (`package x_test`),
Arrange/Act/Assert marked, `task check` green before a phase is called
done; and a web phase runs `task web:lint`, `task web:test`, `yarn gen:check`
after any spec change, and `yarn test:e2e` by hand until Phase 0 puts the
first two inside `task check`.

Where the order bites: **1 before 8, 9 and 10** (they consume `loop`);
**0 before any web phase** (or the web gates are outside `task check`);
**4 → 5 → 6 → 7 one at a time** (each moves the same golden screens);
**3 before 9** (the health call carries the forge's noun); **11 before 13**
(feedback components exist before restyling); **15 last** (it documents the
end state).

### Phase 0 — Gates first — done

Configuration only; TDD-exempt. Closes DEBT-61, DEBT-66 and the gate half
of DEBT-62 and DEBT-65.

- Delete `//nolint:slicesbackward` at `internal/hooks/generate.go:384` (not
  a linter) and *try* deleting its four `modernize` siblings
  (`internal/gitrepo/log.go:51`, `internal/gitrepo/status.go:129`, `internal/gitrepo/branch.go:330`, `:357`):
  `internal/tui` ranges `strings.SplitSeq` unguarded (`internal/tui/render.go:282`) under
  a green `task cover:branch`, so let gobco answer — keep only a directive
  gobco still needs, and name the failure in its comment.
- `scripts/check-file-length.sh:77`: add `*.ts *.tsx`, excluding
  `web/src/api/generated`. It passes today (the largest is 371).
- `web/.dependency-cruiser.cjs`: the rule its own comment promised — value
  imports of `src/api/generated/**` only from `src/api/**` (type-only
  imports allowed anywhere).
- `Taskfile.yml:450` `check` gains `web:lint` and `web:test` (e2e stays an
  explicit/CI step). **Decided:** yes, with `yarn gen:check` — the local
  gate now needs node.

**Touches.** `scripts/check-file-length.sh`, `Taskfile.yml`,
`web/.dependency-cruiser.cjs`, `internal/hooks/generate.go`,
`internal/gitrepo/{log,status,branch}.go`.

**Done when.** `golangci-lint run` prints no "unknown linters" warning;
`scripts/check-file-length.sh --list` lists `.tsx` files; `task check` runs
the web lint and unit tests; `depcruise` fails a deliberate feature-file
`import … from '@/api/generated/…'` (try it, then revert).

**Proof.** The gates' own runs.

### Phase 1 — The shared composition layer: `internal/loop` — done

Closes DEBT-50. The layer is a **new leaf package**, `internal/loop` ("the
loop" is the house word — `docs/content/docs/usage.md:116`, `FEATURES.md:153`). It imports
only `config`, `convention`, `forge`, `gitrepo`, `jira`, `messaging`, `proc`;
it is imported by `cli`, `tui`, `webserver`. It cannot be `wiring` (which
imports `tui` for `tui.Deps` — `tui` importing it back is a cycle), `tui`
(the web server must not import the terminal) or `convention` (`config`
imports it, so it can never take a `config.Config`). Direction: `cli → {tui,
webserver, wiring} → loop → leaves`. Hold it with a `depguard` rule in the
first commit, so the build and not a reviewer keeps it.

Two duplicates have better homes than a new package and go there first:
the noun as a method on the discriminant (`forge.Kind`), and branch naming
as a method on the section that owns its four fields (`config.Branch`).

One commit each, red first, in this order:

1. `forge.Kind.Noun()` and `Sigil()` replace `internal/tui/review.go:45`
   `forgeVocab`, `internal/cli/announce.go` `forgeNoun`,
   `internal/webserver/announce.go` `noun`. Add the methods to an
   existing `forge` file — `internal/forge` is 11 of 12.
2. `config.Branch.Naming() convention.BranchNaming` replaces the three
   identical `NewBranchNaming(…)` calls (`internal/tui/branch.go`,
   `internal/cli/branch.go`, `internal/webserver/branchcreate.go`).
3. `loop.ComposePull(seams PullSeams, opts PullOptions)
   (forge.NewPullRequest, gitrepo.Branch, error)` with `loop.ErrNothingToOpen`
   and `loop.ErrPullAlreadyOpen`, and `loop.EnsurePushed(push, branch) error`
   with `loop.ErrPushFailed` carrying the drained lines — replacing the
   composition and `ensurePushed` in `internal/cli/pr.go` and
   `internal/webserver/pullrequest.go`, which were near line-for-line.
4. `jira.FindTransition(moves, status)` (pure, over its own types, from
   `internal/cli/pr.go` `transitionTo`) and `loop.ReviewTransition(transitions,
   key, status) (jira.Transition, bool)` — the fields-less lookup from
   `internal/cli/pr.go` `reviewTarget`.
5. `loop.AnnounceMoment(pull forge.PullRequest, ci forge.CI, ciKnown bool)
   messaging.Moment` and `loop.ComposeAnnouncement(seams AnnounceSeams, cfg
   config.Messaging, project string, kind forge.Kind) (messaging.Announcement,
   error)` with `loop.ErrNoPullRequest` — replacing
   `internal/cli/announce.go` and `internal/webserver/announce.go` `composeAnnouncement`
   and the interface's `announceMoment` in `internal/tui/messaging.go` (a third caller; it
   passes its own `review.ci`).
6. `loop.ErrDirtyTree`, `loop.ErrNothingStaged`, `loop.RefuseDirty(changes)`,
   `loop.RefuseUnstaged(changes)` — the guards that `internal/webserver/checkout.go`
   and `internal/webserver/commit.go` said were "the same guard the terminal interface applies"
   (`internal/tui/switchtask.go:200`, the composer).
7. `CLAUDE.md`'s layout block gains `internal/loop/` (one line; the rest of
   the drift is Phase 15).

**Touches.** New `internal/loop/{loop,pull,announce,guards}.go` and tests;
`internal/forge`, `internal/config/branch.go`, `internal/jira`;
`internal/cli/{pr,announce,branch}.go`; `internal/webserver/{pullrequest,
announce,branchcreate,checkout,commit}.go`; `internal/tui/{review,branch,
messaging}.go`; `.golangci.yml`.

**Budget.** **No new file in any frozen package** — `cli`, `tui` and
`webserver` only lose lines. `internal/loop` answers to the default 12; aim
for four or five source files.

**Done when.** `grep -rn 'NewBranchNaming(' internal/{cli,tui,webserver}`
and `grep -rn '"merge request"' internal/{cli,tui,webserver}` print
nothing; `ensurePushed`, `composeAnnouncement` and `announceMoment` exist
only in `loop`; **every existing `cli_test`, `webserver_test` and `tui` test
passes with its goldens unchanged** — this is a pure refactor, and the
de-duplication is the proof the layer is right; depguard green.

**Proof.** `internal/loop/*_test.go`: `TestComposePullRefusesAnOpenPull`
(`ErrPullAlreadyOpen`), `TestEnsurePushedSkipsAPushedBranch`,
`TestAnnounceMomentReadsMergedBeforeCI`, `TestRefuseDirtyNamesTheGuard`;
`forge_test.TestKindNoun` (GitLab → "merge request", sigil `!`);
`config_test.TestBranchNamingUsesTheSection`.

### Phase 2 — The command line is scriptable — done

Closes UX-50, UX-51, UX-52, UX-53, UX-54, UX-55 and DEBT-51, DEBT-52,
DEBT-53, DEBT-54. Depends softly on Phase 1 (`ExitStatus` classifies `loop.Err*`;
without it, classify the CLI-local sentinels and re-point later).

- **Exit codes.** `cli.ExitStatus(err) int`, called from
  `cmd/workflow/main.go:38`: **0** success · **1** failure · **2** usage
  (cobra's flag and argument errors, via `SetFlagErrorFunc`) · **3**
  configuration (`config.ErrNotFound`, `ErrInvalid`) · **4** refused
  precondition (`loop.ErrPullAlreadyOpen`, `ErrNothingToOpen`,
  `ErrNoPullRequest`, `ErrDirtyTree`, `errBranchExists`,
  `errMessagingNotConfigured`, doctor's `errShared`) · **5** unreachable
  (`jira.ErrUnreachable`, `forge.ErrUnreachable`) — the families the web's
  problem codes use (`docs/content/docs/errors.md`). **Decided** — the
  refined table under *Decisions the maintainer made* supersedes this one.
  Align `config show` with `doctor` when there is no file
  (`internal/cli/config_cmd.go:96` vs `internal/cli/doctor.go:434`, both → 3) and `status` with
  `status .` outside a repository (`statusHere`, `internal/cli/status.go:76`,
  vs `statusAcross`, `:91`).
- **Streams.** The rule: stdout carries the artifact (JSON, the preview
  text, the created thing's URL, the standup draft); stderr carries
  commentary (`Warning:` `internal/cli/config_cmd.go:333`, `Not opened.` and `dry run:
  would …` `internal/cli/scriptable.go:83`, `:102`, the no-config guidance
  (`showLoadError`, `internal/cli/config_cmd.go:96`), the web banner
  `internal/cli/cli.go:266`, `config show`'s `# <path>` header
  (`runConfigShow`, `internal/cli/config_cmd.go:340`)). Split the harness
  **first** (`runStreams`, `internal/cli/cli_test.go:56`, returns both streams).
- **Flags.** `--dry-run` and `--log` become root `PersistentFlags`;
  `writeOptions.addFlags` (`internal/cli/scriptable.go:52`) stops declaring its own
  `--dry-run`; the seven wiring preambles (`runReviewsCommand`,
  `runBranchCommand`, `runPRCommand`, `status`'s `seamsFor`,
  `runStandupCommand`, `runAnnounceCommand`, `completeAssignedIssues`, and
  the root's `RunE`) collapse into one
  `connect(cmd)` returning `{cfg, deps, where, closeLog}`, so `--log` reaches
  every subcommand. `standup` gains `--dry-run`/`--yes` (`internal/cli/standup.go:144`)
  and rejects `--days < 1` (`:207`, `:214`) as a usage error; `config init
  --dry-run` prints the redacted file it would write, if the guided flow
  bends easily. One JSON encoder (DEBT-53).
- **Non-TTY.** `confirm` (`internal/cli/prompt.go:41`) turns `io.EOF` into `errNoTerminal`:
  "no terminal to confirm on; pass --yes".
- **Reference.** `cmd/docsgen/main.go` calls `InitDefaultVersionFlag`,
  `InitDefaultHelpCmd`, `InitDefaultCompletionCmd` before generating; a new
  prose page `docs/content/docs/scripting.md` (exit codes, streams, the
  `--json` shapes, `--yes`, `--dry-run`).

**Touches.** `cmd/workflow/main.go`, `internal/cli/{cli,scriptable,prompt,
config_cmd,standup,status,doctor,cli_test}.go`, `cmd/docsgen/main.go`,
`docs/content/docs/reference/*` (regenerated), new
`docs/content/docs/scripting.md`.

**Budget.** `internal/cli` is 13 of 13. Put `ExitStatus` and `connect` in
`cli.go` / `scriptable.go`; **if** an `exit.go` is unavoidable, bump 13 → 14
with the WHY rewritten in `scripts/package-size-budgets.txt` and a row in
`scripts/package-size-budget-history.md`, in the same commit.

**Done when.** `workflow --dry-run pr` and `workflow --log f status` are
accepted; `workflow config show | jq .` parses; `workflow standup
--dry-run --yes` posts nothing and exits 0; piped-stdin `workflow pr` says
"pass --yes" and exits 1; `task docs:check` is green with `--version` in
`reference/workflow.md`.

**Proof** (`package cli_test`, red first): `TestExitStatusDistinguishes
FailureKinds` (table: sentinel → code); `TestConfigShowExitsLikeDoctor
WithoutAConfig`; `TestStatusExitsAlikeOutsideARepository`;
`TestConfigShowWritesOnlyJSONToStdout` (`json.Unmarshal(stdout)`);
`TestDeclineNoticeGoesToStderr`; `TestDryRunIsAPersistentFlag`;
`TestLogReachesASubcommand` (the log file has a line after `status`);
`TestConfirmWithoutATerminalNamesYes`; `TestStandupRejectsNegativeDays`;
`task docs:check`.

### Phase 3 — The web uses what the contract already offers — done

Closes UX-70, UX-71, UX-72 and the `resolveJQL` half of DEBT-67. Depends on
Phase 0.

- Call `getIssue` on selection (the query client is wired,
  `web/src/queryClient.ts`); split `IssueDetail` (`IssuesPanel.tsx:83`) into
  its own component with loading and error states through `apiErrorMessage`;
  render description, comments, reporter, assignee and an "Open in Jira"
  link.
- Call `getHealth` once at mount: the version in the header; when
  `dry_run`, a `role="status"` banner ("Read-only: started with `--dry-run`;
  every write is held back") and each write button, when clicked, says
  "held back by --dry-run" **without sending** — the interface's narration
  (`internal/tui/dryrun.go:25`), never a disabled control (CLAUDE.md forbids
  the opacity route in any case).
- A view switcher from `listViews`; `useEventStream(view)` reconnects with
  `?view=`; `No issues match this view.` (`IssuesPanel.tsx:23`) gains the
  switcher beside it.
- Server: `resolveJQL` (`internal/webserver/handlers.go:217`) answers
  `not_found` for an unknown view instead of `views[0]`; the stream
  validates `?view=` before upgrading.

**Touches.** `web/src/features/issues/{IssuesPanel.tsx, IssueDetail.tsx
(new), issueApi.ts (new), ViewSwitcher.tsx (new)}`,
`web/src/api/{snapshot.ts, health.ts (new)}`, `web/src/shell/AppShell.tsx`,
`internal/webserver/{handlers,stream}.go`.

**Budget.** No Go file added. `web/src/features/issues` 6 → 9 of 12,
`web/src/api` 3 → 4.

**Done when.** Selecting an issue shows its description and comments; a
`--dry-run` server shows the banner and no write leaves the browser; the
view select changes the stream's query; `GET /api/issues?view=nope` is 404
`not_found`.

**Proof.** vitest, role and name only: *IssueDetail shows the description
and comments from getIssue*; *a dry-run server shows the read-only banner*;
*clicking Push under dry run reports the hold without a request* (the fake
counts zero POSTs). `webserver_test.TestListIssuesRefusesAnUnknownView`
(404, `code: not_found`). axe on the banner in both themes.

### Phase 4 — The terminal takes a last look before `R` and `u` — done

Closes UX-65. Before 5–7, so the goldens move once.

Generalize `pushPreview` (it was `internal/tui/run.go:394`) into `lastLook{title,
body, verb, proceed func(Model) (Model, tea.Cmd)}` (now `internal/tui/overlay.go:139`)
— three users now, the rule of three is met — and route `rerunChecks`
(`internal/tui/checks.go:201`, a forge write) and `startRebase`
(`internal/tui/run.go:419`, rewrites local history) through it;
the dry-run narration moves inside `proceed`. Relabel the footers
("re-run", "rebase"); `usage.md`'s key rows follow.

**Touches.** `internal/tui/{run,review,branch}.go`, their tests,
`docs/content/docs/usage.md`. **Budget.** None — `lastLook` replaces
`pushPreview` in `run.go` (462 lines; stays under 500).

**Done when.** The promise reads 18 of 18: no forge or history-rewriting
write is one key from the request.

**Proof** (`package tui_test`, red first): `TestRerunAsksBeforeTheRequest`
— press `R` on a failed pull request: the fake `Rerun` is not called and the
overlay names the pull request; `enter` calls it; `esc` never does.
`TestRebaseAsksBeforeTheRequest` likewise. Re-point the existing `R`/`u`
tests to press `enter`.

### Phase 5 — A refused change stays in view, and the honest `review.go` split — done

Closes UX-66, DEBT-55 (for `review.go`), DEBT-56. Depends on Phase 4.

1. `refactor(tui): split the merge picker out of review.go` — `canMerge …
   mergePicker.confirm` (it was lines 562–727 of `internal/tui/review.go`)
   to `internal/tui/merge.go`, and the re-run block (`canRerun …
   rerunReason`, `previewRerun` included) to `internal/tui/checks.go`.
   **`internal/tui` 38 → 39 in this commit**, with the WHY rewritten
   ("merge.go: the Review pane's merge picker and its permitted-methods
   read, split out of review.go when it passed the 500-line target") and a
   row in `scripts/package-size-budget-history.md` — same commit, or the
   commit is red.
2. `mergePicker.merging bool` → `send sendState` (now `mergePicker.send`,
   `internal/tui/merge.go:119`); `mergeRequested.apply`
   (`internal/tui/merge.go:84`) keeps the overlay open through
   `pinnedOutcome` (`internal/tui/failure.go:413`) instead of
   `closeOverlay().noticed(…)`. A write-scope refusal is wrapped in
   `errNeedsWriteScope` so the pinned line keeps the hint until Phase 6.
3. The same for `finishPreview` (`internal/tui/finish.go:59`,
   `finished.apply` `:139`); `oneLine` is deleted.
4. `branchPicker.sending` and `switchErr` (`internal/tui/switchtask.go:74`)
   adopt `sendState` (pure refactor; it already kept its refusal).
5. The Messaging pane's `messagingState.sending` and `err`
   (`internal/tui/messaging.go:36`) adopt `sendState` (pure refactor), or the
   Done-when grep cannot pass.

**Touches.** `internal/tui/{review, checks, merge (new), finish,
switchtask, messaging, branch, sendstate}.go`,
`scripts/package-size-budgets.txt`, `scripts/package-size-budget-history.md`.

**Done when.** `grep -nE '(sending|merging|finishing)\s+bool'
internal/tui/*.go` matches only `sendstate.go` (since renamed `failure.go`); the promise is 14 of 14
*held in the overlay* (Phase 4's re-run look is the fourteenth); `review.go`
is under 500 lines (467).

**Proof.** `TestRefusedMergeStaysInItsPreview` — `Merge` returns
`forge.ErrRefused`; the merge overlay is still open and shows the
write-scope reason; `esc` closes it. `TestRefusedFinishStaysInItsPreview`
likewise with a git failure. The merge and finish goldens change once.

### Phase 6 — One voice for failure, measured — done

Closes UX-67. Depends on Phase 5 (every overlay outcome then flows through
`pinnedOutcome`).

- `failureBlock` (`internal/tui/failure.go:395`) consults `errorSentence` —
  fixes all eleven `pinnedOutcome` overlays at once.
- `m.failureLine(err)` — glyph plus the sentence, or the raw text when
  there is none, one line — replaces the fourteen `failedGlyph() +
  err.Error()` rail sites (`internal/tui/checks.go:100`, `internal/tui/diff.go:79`, `internal/tui/issuewrite.go:120`,
  `:122`, `internal/tui/picker.go:209`, `:246`, `internal/tui/switchtask.go:107`, `:147`,
  `internal/tui/messaging.go:155`, `internal/tui/review.go:217`, `internal/tui/run.go:212`, `internal/tui/composer.go:166`,
  `:178`, `internal/tui/fields.go:171`).
- The seven bare notices (`internal/tui/comment.go:80`, `internal/tui/composer.go:78`,
  `internal/tui/messaging.go:390`, `internal/tui/checks.go:237`, `:245`,
  `internal/tui/merge.go:64`, `:66`) go through `m.noticed(m.failureLine(err))`;
  `internal/tui/render.go:453` `configErrorStatus` is styled.
- `forgeReason` (it was in `internal/tui/review.go`), `rerunReason` (in
  `internal/tui/checks.go`) and `errNeedsWriteScope` (in `internal/tui/merge.go`,
  now `internal/tui/failure.go:50`) fold into `errorSentence`'s table
  (`forge.ErrRefused`/`ErrUnauthorized` → the write-scope sentence;
  `jira.ErrNotFound`; `forge.ErrNoRepository`; `errDryRun`).

**Target.** From 11 of 53 to *every site that renders an error's text* —
the eight width-one glyph-only rail cells excepted, by name. Operationally:
`err.Error()` appears in no non-test `internal/tui` file but `failure.go`.

**Touches.** `internal/tui/render.go` and the nine files above;
`failure_test.go`. **Budget.** None.

**Done when.** The grep above; `errorSentence` has an entry for every
sentinel the seams can return.

**Proof.** A table test, `TestEveryChannelSpeaksTheFailureSentence`: for
each known sentinel × {pane, pinned overlay, notice, rail}, the rendered
frame contains the sentence and, where styled, the failure style. Expect
the widest golden churn of the terminal track; regenerate deliberately,
file by file, with eyes on each diff.

### Phase 7 — Keys shown where they work — done

Closes UX-63. Depends on Phase 4 (relabels).

The Issues pane's `keys(m)` gains `a`, `w`, `/` when the Jira write seams
exist, and `enter`/`esc` in the collapsed layout (`internal/tui/issuekeys.go:22-85` vs
`internal/tui/render.go:338`); `ctrl+w` moves from "Branch and Commits" (`branchAndCommitKeys`)
to the composer's group; `w` post-when-green (`reviewAndSlackKeys`) to the
preview's (both now in `composerKeys`, `internal/tui/keys.go:266-267`); `docs/content/docs/usage.md:107`, `:108` follow. Replace the three-string spot check
(`TestHelpShowsEveryKey` in `internal/tui/focus_test.go`) with a structural test that every placed binding with
help text is rendered by `?`. `ShortHelp`'s omissions (`shift+tab`, `m`,
the scroll keys, `ctrl+c`) are the deliberate tail — leave them.

**Touches.** `internal/tui/{keys,issues,render}.go`, `focus_test.go`,
`docs/content/docs/usage.md`. **Budget.** None.

**Done when.** Every key the Issues pane answers appears in its footer when
it can act; the `?` groups name where each key works.

**Proof.** `TestIssuesFooterShowsEveryLiveKey` (Jira writes wired → the
footer contains `a`, `w`, `/`); `TestHelpListsEveryPlacedBinding`.

### Phase 8 — The command line says what it does — done

Closes UX-56, UX-57, UX-58, UX-59, UX-60. Depends on Phase 1 (`loop`) and
Phase 2 (notices are on stderr).

- Typo hint: keep `SilenceErrors` (it stops the usage dump on a real
  error) and have `Execute` print "Run 'workflow --help' for usage." plus
  cobra's suggestions on an unknown command or flag (`internal/cli/cli.go:174`).
- Sentinels carry a next step: `errBranchExists` names the branch and
  `git switch NAME` (`internal/cli/branch.go:20`); `errPullAlreadyOpen` carries the URL
  (`internal/cli/pr.go:28`); `errMessagingNotConfigured` names the keys and `workflow
  doctor` (`internal/cli/announce.go:26`); `errNoPullRequest` points at `workflow pr`
  (`:22`).
- `branch`'s help and preview say it switches (`internal/cli/branch.go:40`, `:99`;
  `internal/gitrepo/branch.go:305`).
- `pr`'s question names the push when the branch is unpushed — "Push NAME
  and open the pull request?" (`internal/cli/pr.go:127`); `pr` offers to link the pull
  request on the issue before the status offer (`loop.ReviewTransition` +
  `Jira.LinkPullRequest`, matrix row 8), under the same `--yes`, and
  `--yes`'s help says it covers all three.
- `announce` records in the store (`deps.Store.RecordAnnounce`, already on
  `tui.Deps`) and the preview says "already announced at this moment in an
  earlier session" (row 36).

**Touches.** `internal/cli/{cli,branch,pr,announce}.go` and tests.
**Budget.** None.

**Done when.** Every bare sentinel's message contains a next step;
`workflow pr --yes` links and offers the status like the interface; an
announce at a moment the store already holds says it already posted.

**Proof.** `TestUnknownCommandPointsAtHelp`; `TestBranchExistsSaysHowTo
Switch`; `TestPullAlreadyOpenCarriesItsURL`; `TestPRQuestionNamesThePush`;
`TestPRLinksThePullOnTheIssue` (the fake `LinkPullRequest` receives the
URL); `TestDeliverRecordsTheAnnouncementOnceItIsPosted` (in `loop`, with
fakes: a real post cannot be made black-box) and
`TestAnnounceSaysWhenAlreadyPosted` (a seeded store).

### Phase 9 — The web's follow-through, and the forge's own words

Closes UX-73, UX-74. Depends on Phases 1 and 3.

- `getHealth` gains `forge_noun` — send the words; do not port `Kind.Noun`
  to TypeScript. The six hardcoded "pull request" strings
  (`ReviewPanel.tsx:141`, `:172`, `:181`, `:326`; `WorkStory.tsx:40`;
  `MessagingPanel.tsx:50`) read it; the Messaging section label reads the
  service from the snapshot (`sections.ts:10` vs `MessagingPanel.tsx:170`);
  one name for "start" across `WorkStory` and `IssuesPanel`.
- Spec first: `POST /api/issues/{key}/link` and `POST
  /api/issues/{key}/transition` (fields-less only; 409 when Jira wants
  fields), in one handler file `issuewrite.go` (mirrors
  `internal/tui/issuewrite.go`); `webserver.Deps` gains `LinkPullRequest`,
  `Transitions`, `Transition` (mapped in `cli.webDeps`, `internal/cli/cli.go:274`).
  `OpenedPullRequest` gains `follow_ups` (via `loop.ReviewTransition`).
  After `Pull request opened.`, the panel offers "Link it on KEY" and "Move
  KEY to STATUS" inline, each with a `role="status"` outcome.
- `api/openapi.yaml` → `task gen` → `yarn gen`; `errors.md` unchanged
  (`conflict`/`unprocessable` cover the new refusals).

**Touches.** `api/openapi.yaml`, `internal/webserver/{webserver,
pullrequest, issuewrite (new)}.go`, `internal/cli/cli.go`,
`web/src/features/review/ReviewPanel.tsx`,
`web/src/features/messaging/MessagingPanel.tsx`, `web/src/shell/sections.ts`.

**Budget.** **`internal/webserver` 15 → 16** with the WHY ("issuewrite.go —
the two post-open Jira writes the interface and the CLI offer, link and
review status; the spec recorded the operations first") and a history row,
in the same commit as the file.

**Done when.** On a GitLab remote the web says "merge request" everywhere;
opening a pull request offers the link and the status move; `--dry-run`
refuses both new POSTs (the method-based guard covers them — prove it).

**Proof.** `webserver_test.TestTransitionRefusesAFormTransition` (409);
`TestLinkRecordsThePullOnTheIssue`; `TestDryRunRefusesTheIssueWrites`; and
the CLAUDE.md-required `TestUnreachableJiraDetailOmitsItsHost`. vitest:
*after opening, the panel offers to move the issue*. `yarn gen:check`.

### Phase 10 — The web can stage, and picks the scope the interface would

Closes UX-75, UX-76. Depends on Phases 1 (`loop.RefuseNothingStaged`) and 3.

- `POST /api/stage` and `POST /api/unstage` taking `{path}` or `{all:
  true}`, in one `staging.go`; `webserver.Deps` gains `Stage`, `Unstage`.
  The working-tree list (`BranchPanel.tsx:86`) gets Stage / Unstage per
  file and Stage all; the commit form is always mounted, with a "Nothing
  staged yet — stage a file above" state instead of vanishing (`:111`).
- The snapshot's `changes` gains `suggested_scope` — the store's last
  scope, else `commit.default_scope` — the interface's rule
  (`internal/tui/composer.go:131-137`); `Deps.LastScope`/`RecordScope` from
  `deps.Store`; `CommitForm` opens on it; the server records the scope
  after a commit.

**Touches.** `api/openapi.yaml`, `internal/webserver/{webserver, staging
(new), commit, stream, dto}.go`, `internal/cli/cli.go`,
`web/src/features/branch/{BranchPanel,CommitForm}.tsx`, new
`stagingApi.ts`.

**Budget.** **`internal/webserver` → 17** with a history row. Note: the
three bumps across Phases 9, 10 and 12 are the WHY's own anticipated path
("grows only when the API gains an operation, which the spec records
first") — a split would separate handlers that change together, so bump,
don't split.

**Done when.** A browser can stage, commit, and see the scope pre-filled;
`commit.default_scope` edited in Settings is honored on the next form.

**Proof.** `webserver_test.TestStageAllStagesEveryChange`;
`TestSnapshotCarriesTheSuggestedScope` (a table over the store-then-config
fallback); vitest *the commit form opens on the suggested scope*, *Stage all
makes the commit form live*.

### Phase 11 — Feedback, focus and wording on the web

Closes UX-77, UX-78, UX-79, UX-80, UX-81, UX-82 and DEBT-63. Depends on
Phase 3.

- `useAsyncAction` (`useAsyncAction.ts:11`) gains a `done` state with a
  message; the five hand-rolled copies (`PushButton`, `CommitForm`,
  `AnnounceControls`, `OpenPullRequest`, `ConfigForm`) adopt it — six
  machines → one. Commit, push, check-out and start-work confirm in a
  `role="status"` that keeps the button's verb ("Pushed NAME", "Committed
  abc123 subject"); `Pull request opened.` (`ReviewPanel.tsx:141`) and
  `Announced…` (`MessagingPanel.tsx:137`) become live regions.
- Focus moves to the outcome when a form unmounts (`ReviewPanel.tsx:138`,
  `MessagingPanel.tsx:135`, the `PushButton` swap) and to `<main>` on a
  section change (`AppShell.tsx:68`).
- One announce verb through the flow — **decided: "Announce to X"** (opens
  the preview) and "Announce now" (sends), on every surface.
- `internal/webserver/checkout.go:44` and `internal/webserver/branchcreate.go:44` pass git's own reason through
  `fault`. **`internal/webserver/announce.go:57` does not**: a messaging error can name the
  webhook URL, so classify by `messaging.ErrRejected`/`ErrUnreachable` and
  never forward the text. `"something went wrong"` (`errors.go:79`) → a
  directive sentence. One "connecting" phrasing (`IssuesPanel.tsx:19`,
  `BranchPanel.tsx:14`, `ReviewPanel.tsx:31`, `MessagingPanel.tsx:11`);
  dead ends get a way out (`BranchPanel.tsx:22`, a Retry at
  `SettingsPanel.tsx:16`).
- A dropped stream frame sets `status: 'stale'` with the reason
  (`snapshot.ts:63-69`).
- `opacity-60` ×13 → a `disabled:` color treatment (CLAUDE.md's rule);
  `motion-safe:` on the two transitions and a `prefers-reduced-motion`
  rule in `index.css`.

**Touches.** `web/src/features/**`, `web/src/api/snapshot.ts`,
`web/src/index.css`, `internal/webserver/{checkout,branchcreate,errors}.go`.
**Budget.** None.

**Done when.** 7 of 7 writes confirm; every outcome is a live region;
`grep -c opacity-60 web/src` is 0; a schema-mismatched frame shows in
`StreamStatus`.

**Proof.** vitest *every write reports its success in a status region* (a
table over the seven), *focus lands on the outcome after a form closes*
(`toHaveFocus`), *a frame that fails the schema marks the stream stale*;
`webserver_test.TestCheckoutCarriesGitsReason`,
`TestAnnounceNeverForwardsTheWebhook`; axe e2e in both themes.

### Phase 12 — The web's Reviews section

Closes UX-83. Depends on Phase 3's patterns.

`GET /api/reviews` over `Forge.ReviewRequests` (the seam the CLI and the
interface use) in `reviews.go`; polled by the query client with a
`staleTime` (it is a cross-repository forge search, not a snapshot field);
a sixth section `reviews` (`uiStore.ts:6`, `sections.ts`) with
`web/src/features/reviews/`; number, title, repository, requester, CI, age;
open/copy links. The a11y spec covers the sixth section in both themes.

**Touches.** `api/openapi.yaml`, `internal/webserver/{webserver, reviews
(new)}.go`, `internal/cli/cli.go`, new `web/src/features/reviews/`,
`web/src/shell/{uiStore,sections}.ts`, `web/e2e/a11y.spec.ts`.

**Budget.** **`internal/webserver` → 18** with a history row;
`web/src/shell` stays 10 of 12 (no new file); the new feature directory
answers to the default.

**Done when.** The web has the interface's six sections and lists the same
requests `workflow reviews` prints.

**Proof.** `webserver_test.TestListReviewsAnswersTheForgeQueue`; vitest *the
Reviews section lists requests by role*; axe on the new section.

### Phase 13 — The web's visual system

Closes UX-84 and the `ConfigForm` half of DEBT-62. Depends on Phase 11.
The maintainer chose this: the web aligns to the product's system; the
interface's own system does not change.

- Four system hues as tokens in both themes — Jira, git, the forge,
  messaging (`internal/tui/glyphs.go:96-127`; the spine, `internal/tui/spine.go:68`) —
  at AA contrast, carrying *identity*: the active `NavRail` icon
  (`NavRail.tsx:28`), section headings. **Periwinkle stays the
  interactive-control accent** (`index.css:24` is a documented choice)
  — **decided: it stays**, and the identity hues stay distinct from it.
- One `StateMark` component drawing `○ ◐ ● ✗` for CI (`ReviewPanel.tsx:14`),
  the stream (`StreamStatus.tsx:4`) and the work story
  (`WorkStory.tsx:284`); the text label stays, the mark is `aria-hidden`.
- The seven `uppercase` eyebrows (`SettingsPanel.tsx:341`,
  `BranchPanel.tsx:67`, `:91`, `IssueDetailPanel.tsx:8`, `ReviewPanel.tsx:70`,
  `MessagingPanel.tsx:39`, `:59`) → sentence-case headings on a
  `--text-*`/`--space-*` scale in `@theme`; the four-step radius actually
  used.
- Split `ConfigForm` (304 lines, `SettingsPanel.tsx:24`) per fieldset and
  add `max-lines-per-function` to `web/eslint.config.js:80` at a number the
  split meets.

**Touches.** `web/src/index.css`, `web/src/shell/*`, `web/src/features/**`,
`web/eslint.config.js`. **Budget.** `web/src/features/settings` 2 → ~6 of
12; `web/src/shell` 10 → 11 (`StateMark.tsx`).

**Done when.** No `uppercase` heading remains; every state has a distinct
shape; both themes pass axe; `max-lines-per-function` green.

**Proof.** The black-box vitest role/name tests are unchanged; axe e2e in
both themes is the contrast proof; `yarn lint`.

### Phase 14 — The web responds to its viewport

Closes UX-85. Depends on Phase 13 (tokens).

Breakpoints (none today): the rail collapses to icons under `md`; list and
detail stack under `lg` (`IssuesPanel.tsx:101-102`); `grid-cols-[6rem_1fr]`
and its siblings go responsive; `max-w-2xl` goes fluid; the issues `<ul>`
scrolls inside its panel.

**Touches.** `web/src/**/*.tsx`, a new `web/e2e/layout.spec.ts` (`web/e2e`
3 → 4 of 12).

**Done when.** At 640, 1024 and 1440 px there is no horizontal scroll and
every control is reachable.

**Proof.** Playwright `layout.spec.ts` over the three viewports, axe
included.

### Phase 15 — Docs that lag the code

Closes DEBT-69, DEBT-70. Last, so it documents the end state. TDD-exempt.

- `CLAUDE.md:35` `internal/slack` → `internal/messaging`, and the missing
  layout lines: `api/`, `internal/api`, `internal/loop`, `internal/keychain`,
  `internal/progress`, `internal/buildinfo`, `internal/store`, `internal/web`,
  `internal/webserver`, `internal/tui/frame`, `internal/tui/layout`, `web/`.
- The web server's sanitize exemption, one sentence beside the RFC 9457
  convention (React escapes; `dangerouslySetInnerHTML` is banned;
  what would change that).
- `usage.md`: six panes (`:55`), `1`–`6` (`:77`), Reviews in the mock
  (`:30-49`), `R` and `u` now previewed, the moved keys; and **a web page**
  — the six sections, the stream, the theme, the actions the web supports —
  written last so it is true. README mentions `--web`.
- Residual "Slack": `internal/cli/cli.go:171`, `groupReviewSlack`
  `internal/tui/keys.go:88`, `web/index.html:9`, `FEATURES.md:30`.
- **`FEATURES.md:52` "Five panes down the left" → six** — stale
  settled-decision text corrected, not a decision reopened; say so.
  `FEATURES.md:12` re-pinned; the `Done:` notes on FEAT-26 (`:118`) and
  FEAT-31 (`:143`) removed per the standing rule.
- `docs/content/docs/usage.md:266`'s threading limit → point at FEAT-81.

**Proof.** `task lint:markdown`, `task docs:check`, `task docs:build`.

## Decisions the maintainer made

Settled on 2026-09-22, before Phase 0 began.

- **Delivery** — all sixteen phases, in numeric order, as one pull request;
  an adversarial review after each phase; this file marks each phase done
  and is deleted by the last commit. On 2026-09-23 the maintainer set the
  gate's cadence: targeted checks and the commit-message check on every
  commit, and the full `task check` on each phase's last commit.
- **Scope beyond the phases** — also close DEBT-52 (Phase 2), DEBT-60
  (Phase 4, before any screen assertion moves), the rest of UX-72 — load
  more and a filter (Phase 3) — and DEBT-67's cross-language frame test
  (Phase 11).
- **Phase 0** — `task check` runs `web:lint`, `web:test` and `yarn
  gen:check` (the web twin of Go's `gen:verify`).
- **Phase 2** — exit codes: **0** success · **1** failure (not found, push
  failed, missing tooling, anything else) · **2** usage (flag and argument
  errors, an unknown command, `--days < 1`, no terminal to confirm on) ·
  **3** configuration (`config.ErrNotFound`/`ErrInvalid`, a rejected
  credential, messaging not configured, doctor's `errShared`) · **4**
  refused precondition (the `loop.Err*` refusals, `errBranchExists`,
  `errConfigExists`, `gitrepo.ErrNotARepository`) · **5** unreachable
  (including `httpx.ErrRateLimited`) · **130** Ctrl-C. A joined error takes
  the first family in the order 2, 3, 4, 5, 1. The change is **breaking**:
  `feat!`/`fix!` with a `BREAKING CHANGE:` footer. `status DIR…` prints
  every row and exits 4 when any directory is not a repository; a
  configuration file that exists but does not parse is refused with 3
  instead of silently replaced by the defaults.
- **Phase 8** — `announce --yes` at a moment the store already holds skips
  with a note on stderr and exits 0.
- **Phase 1** — when the forge cannot be read while composing a pull
  request, compose anyway (the CLI's behavior); the open fails later with
  the forge's own error.
- **The announce verb** — "Announce" on all three surfaces (the CLI in
  Phase 8, the terminal in Phase 7, the web in Phase 11); the web's two
  buttons read "Announce to X" (opens the preview) and "Announce now"
  (sends). `standup` keeps "Post". The `ui.keys` action ids `post` and
  `post-when-green` stay, so no configuration breaks.
- **Phase 13** — periwinkle stays the interactive-control accent; the four
  identity hues are kept visibly distinct from the status lights and from
  periwinkle; REVIEW's scope with a deliberate system font stack and no new
  dependency; the middle-dot separator stays.

## Corrections from the feasibility pass

A read-only pass per track, run after this file was written, found places
where a phase as written would not land green. Where a correction and a
phase disagree, the correction wins.

- **Phase 0.** The depcruise rule allows value imports of the generated SDK
  from `src/api/**` *and* the seven `src/features/*/*Api.ts` wrappers (as
  written it fails all seven); add `not-to-unresolvable`. Deleting the four
  `//nolint:modernize` means converting those loops to `range
  strings.SplitSeq`. The file-length change is red-first in
  `scripts/check-file-length_test.sh`. `task check` needs an install step
  (`web:install`), and `container:check`/`container:release` need an
  anonymous `/src/web/node_modules` volume, or the host's platform-specific
  bindings load in the Linux container. lefthook's `go test ./...` becomes
  `./cmd/... ./internal/...` (`web/node_modules` ships a Go package). Every
  doc that describes the gate changes in the same commit.
- **Phase 1.** Commit order: `forge.Kind.Noun()/Sigil()` (in
  `internal/forge/remote.go`); `config.Branch.Naming()`; a
  `fix(webserver)` that stops a forge read failure from blocking the draft;
  `internal/loop` with its depguard rules (in the commit that creates the
  package — a rule for an absent package cannot be proved), CLAUDE.md and
  ARCHITECTURE.md; `jira.FindTransition` (retiring the terminal's
  `firstWithStatus`) and `loop.ReviewTransition`; `AnnounceMoment` and a
  `ComposeAnnouncement` that also returns the pull request (Phase 8 records
  it); the guards, with `RefuseUnstaged` named `RefuseNothingStaged`.
  `loop` is nil-safe for every seam and reads CI lazily; each surface maps
  `loop.Err*` to its own words, so nothing visible changes;
  `ErrPullAlreadyOpen` carries the open pull and the CLI adds its URL.
  `loop.Push` replaces `internal/webserver/push.go`'s `pushBranch` too.
  The terminal keeps its own announcement (it renders from cached state).
  `loop_test` must cover every arm (coverage is measured per package). The
  Done-when greps exclude `_test.go` files and comments; there are no golden
  files — the screen tests hold string literals.
- **Phase 2.** The harness gains a both-streams variant first, keeping the
  old helpers. DEBT-52's honest scope: `status` reads the branch and changes
  through `deps.Git`, `standup` its branches through `deps.Git.Branches`
  (it already exists); `RecentCommits` stays direct. The usage family needs
  `SetFlagErrorFunc`, every `Args` wrapped, and a `RunE` on `config` that
  refuses an unknown subcommand. `standup` and `config init` honor
  `--dry-run` **before** it becomes persistent. docsgen can add
  `--version` and the completion pages but never a `help` page. Every
  flag or help change regenerates the reference in the same commit.
- **Phase 3.** `IssueDetail` needs `url` and `assignee` in the spec (a
  contract change). The dry-run hold is one interceptor over every non-GET
  request, not a per-button check. Panels that start querying move their
  tests onto a query client.
- **Phase 4.** DEBT-60 first: a `Deps.After` timer seam, then a harness that
  drains on a fake clock, before any screen assertion moves. `lastLook`
  lives in `overlay.go`; a refused re-run stays pinned in it. usage.md has
  no `R` row today; the `u` row is at `:90`.
- **Phase 5.** The re-run block (`canRerun … rerunReason`, Phase 4's
  `previewRerun` included) moves to `checks.go` as well, or `review.go`
  stays over 500 lines. The messaging pane's own `sending` flag
  adopts `sendState` too, or the Done-when grep cannot pass.
- **Phase 6.** First, red: `failureHeadline` never matches production's
  `git: exit status N`. The failure family moves to `failure.go` (renamed
  from `sendstate.go`, so no budget change). `errorSentence` gains a short
  form for rails; write refusals are wrapped locally rather than mapping
  `forge.ErrRefused`, which also means rate limiting. Guidance notices stay
  plain — red is for failure.
- **Phase 7.** The Issues keys live in `detail.go` `issuesKeys`; `?` is
  reserved in every footer; `internal/progress`'s "Slack" stage takes the
  service's name; the terminal's verb change is this phase's last commit.
- **Phase 8.** The CLI harness pins the store directory. Suggestions come
  from `root.SuggestionsFor` (the root's `NoArgs` stops cobra's own). The
  record-after-post logic lives in `loop` and is proved there.
- **Phase 9.** Health sends the forge's noun **and sigil**; there are about
  fourteen noun sites, not six. The link endpoint derives the pull request
  on the server and never accepts a URL; the transition endpoint moves only
  to the configured `jira.review_status`. `useAsyncAction` moves to
  `web/src/lib` here, and the offers render in a slot that survives the
  snapshot.
- **Phase 10.** `suggested_scope` sits on the snapshot, not on the change
  list, cached by the server; the commit form never overwrites a scope the
  user is typing.
- **Phase 11.** Outcomes move into per-panel status slots that stay mounted
  when the snapshot confirms the write (today the snapshot unmounts the
  control that holds them). A populated mock e2e project gives axe and
  screenshots real content. zod strips an unknown key rather than dropping
  the frame, so the cross-language test asserts a lossless round trip.
  `announce` classifies every messaging sentinel. There is no jest-dom:
  focus tests compare `document.activeElement`. The write table has twelve
  rows once Phases 9 and 10 land.
- **Phase 12.** `GET /api/reviews` joins the other reads in `handlers.go`
  (no budget bump); no repository or forge is an empty answer, not a 404.
- **Phase 13.** The type scale uses Tailwind's own `text-*` keys (tailwind-
  merge drops unknown ones); the settings split goes to
  `features/settings/fieldsets/`.
- **Phase 15.** `FEATURES.md` lines have drifted (the pane order is at
  `:28`, FEAT-26's `Done:` note at `:177`, FEAT-31's at `:199`); cite by
  symbol. No backlog header is re-pinned to a commit inside the pull
  request — a rebase merge rewrites it; re-pin after the merge.

## Out of scope, and what stays as it is

- **Declared web follow-ups** (matrix cells "F"): announced history
  (FEAT-66) and cache-seeded first paint (FEAT-68); re-run, merge, finish
  and edit on the web (FEAT-79). They are gaps by decision, recorded in
  FEATURES.md, not rediscovered here.
- **Ideas, not gaps** — filed in UX.md and FEATURES.md, not planned here:
  progress feedback on slow commands (UX-61); short flags and the missing
  command flags (UX-62); the interface's accessibility settings (UX-64);
  undo (UX-68); vim `h`/`l`/`g`/`G` (UX-69); a live-change highlight and
  "CI settled" (UX-86); the five uneditable settings sections (UX-87); the
  web borrowing diff/amend/checks/edit/templates/`?` (UX-88); worktrees on
  the CLI and web (UX-89); a CLI for issue reads and writes (FEAT-78); web
  issue writes (FEAT-80); Slack threading now that the store can hold a
  timestamp (FEAT-81); web post-when-green (FEAT-82); `workflow finish`
  (FEAT-83).
- **Debts recorded, not planned** — DEBT-57, 58, 59, 64, 68, 71 are
  worklists for their own PRs (DEBT-52 and DEBT-60 were folded into Phases
  2 and 4).
- **Met, or by design, and left alone:** the web's absent `→`, shadows and
  gradients; the CLI emitting no color (so nothing to suppress); the
  middle-dot separator (`glyphs.go:35` makes it house vocabulary);
  `ShortHelp`'s deliberate tail; the three narrow production `//nolint`s
  in `internal/tui`; zero TODOs; the web's dependencies sitting on major
  boundaries; `web/src/shell` at 10 of 12; `Save changes` → `Saved.`; ASCII
  via `ui.ascii`; the `status --json` shapes; the four packages sitting
  exactly at budget (that is the gate working); the web's blanket-403 dry
  run (`guard.go:40` documents it — only its *invisibility* is the gap);
  `staleTime: Infinity` under a pushed stream; the spine's color-only hue
  residue; and every part of the interface's visual system.
- **Security:** nothing in this file is a security finding. The one
  security-adjacent constraint is Phase 11's rule for `internal/webserver/announce.go:57`.

## Risks before starting

1. **Import cycles.** `wiring → tui` is fixed. `internal/loop` must never
   import `tui`, `wiring`, `webserver` or `cli`; the `depguard` rule lands in
   Phase 1's first commit so the build holds the direction. `config →
   convention`, so nothing in `convention` may take a `config.Config`.
2. **Zero-headroom budgets.** The bumps this plan needs: `internal/tui`
   38 → 39 (Phase 5); `internal/webserver` 15 → 16 → 17 → 18 (Phases 9, 10,
   12); possibly `internal/cli` 13 → 14 (Phase 2). Each WHY rewrite and
   history row rides in the **same commit** as the new file, or that commit
   fails `task check` and cannot land under rebase-only `main`.
   `internal/loop` is new and answers to the default 12.
3. **Golden churn.** The terminal's screen tests hold rendered frames;
   Phases 4–7 each move them. One phase at a time; regenerate per file with
   eyes on each diff. The 400 ms `patience` that flaked under `-race` is
   gone: Phase 4's harness drains on a fake clock and fails loudly on a
   command that never returns (DEBT-60, closed there).
4. **The web is outside `task check` until Phase 0 lands.** Every web
   phase runs `task web:lint`, `task web:test`, `yarn gen:check` after a spec
   change, and `yarn test:e2e` (`yarn playwright install --with-deps
   chromium` first) by hand. vitest's `branches: 85` is statement-branch
   coverage, not per-condition.
5. **Spec changes are three-way.** `api/openapi.yaml` → `task gen` (Go) →
   `yarn gen` (client and `zSnapshot`). The SSE endpoint is hand-registered
   (`internal/webserver/webserver.go:118`) and the frame parser hand-written
   (`snapshot.ts:50`), so a `Snapshot` field added in Go without
   regenerating the client makes the browser **silently drop every frame**
   (Phase 11 makes that visible). Run `task web:build` before trusting a
   `--web` run.
6. **Rebase-only `main`.** Every commit stands alone and green — one
   logical change, Conventional Commits, no squash to lean on. The
   user-visible exit-code changes (`config show` 0 → 3, `status` alignment)
   are `fix`/`feat` commits in their own right.
7. **New write endpoints** (Phases 9 and 10) each need the CLAUDE.md-
   required "detail omits the host" test and a test that `--dry-run`
   refuses them.
8. **A security-adjacent constraint, not a finding.** `internal/webserver/announce.go:57`
   hides its error because a messaging error can carry the webhook URL;
   Phase 11 classifies it by sentinel and never forwards the text.
