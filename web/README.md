# web — the workflow web cockpit

A React + TypeScript single-page app that shows what the terminal interface shows
and lets you edit the configuration file, served locally by `workflow --web` on
`http://127.0.0.1:7000`. It talks to the Go server over the REST + Server-Sent
Events surface described by [`../api/openapi.yaml`](../api/openapi.yaml) — the
single source of truth, from which the typed client is generated.

## Develop

Node is pinned in the repo's `mise.toml`; run `mise install` from the repo root.
yarn is not pinned there — Corepack (bundled with Node) runs the version the
`packageManager` field names. Running the cockpit takes two shells:

```sh
task web        # shell 1: the Go API and event stream on 127.0.0.1:7000
task web:ui     # shell 2: the web UI on :5173 (installs deps, proxies /api to :7000)
```

Then open <http://localhost:5173>.

To run yarn directly, invoke it as **`corepack yarn <cmd>`**, not bare `yarn`: a
mise `yarn` shim otherwise shadows it and fails with "No version is set for shim:
yarn". `corepack yarn` runs the pinned yarn straight from Node.

## Checks

- `task web:lint` — eslint, `tsc -b`, prettier, knip, and the import-boundary rules.
- `task web:gen:check` — regenerate the client from the spec and fail on any drift.
- `task web:test` — Vitest with the coverage floor.
- `task web:build` — type-check and build the production bundle.

The repo's `task check` runs the first three beside the Go gates, installing
dependencies first when they are missing. The Playwright suite
(`corepack yarn test:e2e`) stays outside it and runs in CI. It serves two builds:
the production one, and a VITE_MOCK one whose populated sections the specs
tagged `@populated` scan with axe and screenshot at 640, 1024 and 1440 px into
`test-results/` (CI uploads them as the `screens` artifact).

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
