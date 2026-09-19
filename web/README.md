# web — the workflow web cockpit

A React + TypeScript single-page app that shows what the terminal interface shows
and lets you edit the configuration file, served locally by `workflow --web` on
`http://127.0.0.1:7000`. It talks to the Go server over the REST + Server-Sent
Events surface described by [`../api/openapi.yaml`](../api/openapi.yaml) — the
single source of truth, from which the typed client is generated.

## Develop

The toolchain (Node, yarn) is pinned in the repo's `mise.toml`; run `mise install`
from the repo root. Then, from this directory:

```sh
corepack enable          # once, so yarn matches package.json's packageManager
yarn install
yarn dev                 # Vite dev server; proxies /api to workflow --web on :7000
```

Run `workflow --web` from the repo in another terminal so the dev server has an
API to proxy to.

## Checks

| Command      | Does                                                            |
| ------------ | --------------------------------------------------------------- |
| `yarn lint`  | eslint, `tsc -b`, prettier, knip, and the import-boundary rules |
| `yarn test`  | Vitest with the coverage floor                                  |
| `yarn build` | type-check and build the production bundle                      |
| `yarn fmt`   | format in place                                                 |

## Conventions

- **TypeScript, strict.** `noUncheckedIndexedAccess`, `verbatimModuleSyntax`, and
  the `@/` alias for cross-directory imports (siblings stay relative). No `any`.
- **Black-box tests.** Drive components through role, accessible name, and text
  with Testing Library and user-event; never assert on classes, styles, or
  `data-*` hooks. Every test marks its Arrange, Act, and Assert.
- **The generated API layer** (`src/api/generated`) is machine-emitted from the
  contract and excluded from lint, tests, coverage, and formatting.
