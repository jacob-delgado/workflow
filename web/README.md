# web — the workflow web cockpit

A React + TypeScript single-page app that shows what the terminal interface shows
and lets you edit the configuration file, served locally by `workflow --web` on
`http://127.0.0.1:13579`. It talks to the Go server over the REST + Server-Sent
Events surface described by [`../api/openapi.yaml`](../api/openapi.yaml) — the
single source of truth, from which the typed client is generated.

## Develop

Node is pinned in the repo's `mise.toml`; run `mise install` from the repo root.
yarn is not pinned there — Corepack (bundled with Node) runs the version the
`packageManager` field names, and refuses a download that does not match the
SHA-512 the field carries. One command, from the repo root, runs the whole
cockpit:

```sh
task dev        # the Go API and event stream on 127.0.0.1:13579, rebuilt on any .go change,
                # and the web UI on :5173 with hot reload (installs deps, proxies /api to :13579)
```

Then open `http://localhost:5173/#session=…`, with the session the Go API
printed after `#session=` in its own address: the API answers only a request
that presents it. Each start of the API makes a new one, so after a rebuild
open the address it prints next. To run the two halves apart, use two shells
instead; the Go API is then built once rather than rebuilt on a change:

```sh
task web        # shell 1: the Go API and event stream on 127.0.0.1:13579
task web:ui     # shell 2: the web UI on :5173 (installs deps, proxies /api to :13579)
```

`task web:mockup` serves the web UI on :5173 against mock data, with no backend
at all: every section is populated, so the whole cockpit can be reviewed without
a Jira, a forge or a messaging service.

To run yarn directly, invoke it as **`corepack yarn <cmd>`**, not bare `yarn`: a
mise `yarn` shim otherwise shadows it and fails with "No version is set for shim:
yarn". `corepack yarn` runs the pinned yarn straight from Node.

## Checks

- `task web:lint` — eslint, `tsc -b`, prettier, knip, and the import-boundary rules.
- `task web:gen:check` — regenerate the client from the spec and fail on any drift.
- `task web:test` — Vitest with the coverage floor.
- `task web:build` — type-check and build the production bundle, failing when a
  chunk is past the `chunkSizeWarningLimit` `vite.config.ts` sets.

The repo's `task check` runs the first three beside the Go gates, installing
dependencies first when they are missing. The Playwright suites stay outside it
and run in CI. `corepack yarn test:e2e` serves two builds with no backend: the
production one, and a VITE_MOCK one whose populated sections the specs tagged
`@populated` scan with axe and screenshot at 640, 1024 and 1440 px into
`test-results/` (CI uploads them as the `screens` artifact). Chromium runs every
spec, and so do Firefox and WebKit, the engines of the other browsers the page
supports (the `firefox`, `firefox-mock`, `webkit` and `webkit-mock` projects).
CI runs each of those two engines in three shards, jobs of their own beside
E2E. Install the three once with
`corepack yarn playwright install chromium firefox webkit`; `--project=chromium
--project=mock` runs Chromium's alone. The other engines' projects are held on
Linux, where CI runs them: on a Mac, WebKit and Firefox may leave links out of the
Tab order, as Safari does by default, and the layout spec's Tab walk would
then fail there for the platform, not the page.
`corepack yarn test:e2e:server` runs the specs under `e2e/server` against the
binary `task build` makes: `scripts/e2e-server.sh` serves
`workflow --web --port 13580` from a throwaway repository with a bare origin,
and `commit.spec.ts` stages, commits and pushes through the page.
`page.spec.ts` checks how the page takes its session, and that it and its
palette open with nothing the content policy the server sends refuses;
`policy.spec.ts` opens Settings' tabs, the tag picker and the directory picker
under that policy and fails on anything it refuses. Firefox and WebKit run
every one of them too (the `server-firefox` and `server-webkit` projects),
each in a run of its own (`--project=server-firefox`), since `commit.spec.ts`
writes to the one fixture a run serves. Nothing else may be
listening on port 13580. `corepack yarn media` writes the docs' screenshots
into `docs/assets/images`: `e2e/docs/media.spec.ts`, in a run of its own
(`playwright.docs.config.ts`), serves each of six screens from a demo of its
own, the real `--web` server over the made-up world `cmd/demo -web` runs
(`e2e/docs/demo.ts`, on a port the system picks), drives the page to the
state the docs describe, and takes it in the dark theme at 1440 by 900, in
UTC and under a held clock, once its heading is up and nothing is still
reading. It needs the demo built first, `go build -o tmp/demo ./cmd/demo`
from the repository root, as `task docs:media` does. Neither `test:e2e` nor
CI runs it, and nothing compares its images. Every run launches the headless shell of the Chromium build
Playwright pins, as CI does; where that build cannot be installed,
`PLAYWRIGHT_CHROMIUM_EXECUTABLE` names another to launch, which a Claude Code
cloud session's start sets to the image's headless shell, the kind of binary CI
runs.

## Conventions

- **TypeScript, strict.** `noUncheckedIndexedAccess`, `verbatimModuleSyntax`, and
  the `@/` alias for cross-directory imports (siblings stay relative). No `any`.
- **Black-box tests.** Drive components through role, accessible name, and text
  with Testing Library and user-event; never assert on classes, styles, or
  `data-*` hooks. Every test marks its Arrange, Act, and Assert.
- **The generated API layer** (`src/api/generated`) is machine-emitted from the
  contract and excluded from lint, tests, coverage, and formatting. Only
  `src/api` and a feature's `*Api.ts` wrapper import its values — a component
  calls the wrapper, never the SDK — and any file may import its types.
  `depcruise` (the `sdk-only-through-api` rule) holds the line.

## Dependency notes

`package.json` cannot hold a comment, so why each of its pins stands is here.

- **`browserslist`** names the browsers the page supports: the latest two
  versions of Chrome, Edge, Firefox and desktop Safari. Its `Safari` is desktop
  Safari alone; iOS Safari, `ios_saf`, is left out because the page is only
  ever served on 127.0.0.1, to a browser on the machine `workflow --web` runs
  on. `npx browserslist` resolves it to versions (TRADE-33).

- **`resolutions` `js-yaml ^4.3.2`** lifts the js-yaml 4.2.0 that
  `@hey-api/json-schema-ref-parser` pins to 4.3.2, which fixes three
  high-severity advisories (GHSA-52cp-r559-cp3m, GHSA-5p4m-2wfm-xmqj,
  GHSA-2883-xcg3-v3hh). It is unscoped, so it rewrites every js-yaml
  request: remove it once `yarn.lock` records none below 4.3.2, and no later
  than the openapi-ts bump that brings a ref-parser asking for js-yaml 5,
  which it would force back to 4.
- **`typescript ~6.0.3`** stays on 6.0: typescript-eslint accepts only
  `<6.1.0`. TypeScript 7 waits on typescript-eslint and on
  `@hey-api/openapi-ts`, which builds the client with the compiler API that
  7's `typescript` package no longer exports.
- **`@hey-api/openapi-ts ^0.99.0`** is pre-1.0, so the caret admits only
  0.99.x patches. A minor changes the generated client: land it in one commit
  with the `corepack yarn gen` output, or `task web:gen:check` fails.
  Dependabot raises it apart from the weekly group for that reason, so the
  regeneration goes onto its own pull request.
- **`@types/node` 26 over mise's Node 24** is left as is on purpose. It types
  only what `tsconfig.node.json` includes — the Vite, Vitest, Playwright and
  openapi-ts configs and `e2e/` — which call only Node APIs long present on
  24: `node:url`'s `fileURLToPath`, `node:os`'s `tmpdir`, `node:path`'s
  `join`, `node:child_process`'s `execFileSync` and `spawn`, `node:readline`'s
  `createInterface`, `node:fs`'s `existsSync` and `readFileSync`,
  `node:fs/promises`' `mkdtemp` and `rm`, and `process.env`. A Node
  26-only call there would type-check and then fail on 24; pinning `^24`, a
  downgrade, would close that.
