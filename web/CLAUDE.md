# web

The rules and gates of the `--web` frontend (React, TypeScript, Vite, Vitest,
Playwright), for any change under `web/`. The root `CLAUDE.md` still applies;
this adds what holds here alone. UX.md's Principles and Vocabulary are binding
here: an action keeps one name through its flow, and a new word comes from that
table. **`web/README.md` is the guide to read with it**: the dev loop, the
checks, the conventions and why each dependency pin stands are written there
once, and not restated here.

## Running it

- Run yarn as **`corepack yarn <cmd>`**, never bare `yarn`, which a mise shim
  shadows; the `task web:*` targets already do.
- **The built app is committed** in `internal/web/dist`, since `go install`
  cannot run the Node build. A change under `web/src` (or to the generated
  client) runs `task web:build` and commits `internal/web/dist` with the
  source, in the same commit; `task check` fails on a stale bundle
  (`web:dist:check`), and `task build` and `task e2e` rewrite it.
- The client in `src/api/generated` is generated from `api/openapi.yaml` by
  `corepack yarn gen`, never edited; the spec changes first (the root's
  spec-first rule).

## Structure and boundaries

- Group by feature (`src/features/<feature>/`), never one flat directory.
  `src/lib` holds the primitives every section draws alike and is ratcheted in
  `scripts/package-size-budgets.txt`: a primitive one section uses stays in
  that section's folder. `src/dev` is the mockup's stand-in server
  (`task web:mockup`), and its data mirrors the server's, the terminal's key
  exports included (`mockKeys.ts`).
- dependency-cruiser (`.dependency-cruiser.cjs`, in `corepack yarn lint`)
  holds the imports: no circular import; the generated SDK's values only
  through `src/api` or a feature's `*Api.ts` (types anywhere); `src/lib` a
  leaf that imports neither the shell nor a feature; Base UI only through
  `src/lib/baseui`; and no import it cannot resolve.

## Accessibility is enforced, not aspirational

`eslint-plugin-jsx-a11y` runs at **strict** in the web lint
(`eslint.config.js`, in `corepack yarn lint` and `task check`), and a runtime
**axe** scan (`e2e/a11y.spec.ts`, in `task e2e`) fails on any WCAG 2.1 A/AA
violation, across every section **and both themes**, because a light theme is
only real once its contrast holds. `e2e/layout.spec.ts` stands guard beside
it: every section at a narrow, a middling and a wide window, in both themes,
must not scroll sideways, must not scroll the page, and must keep every
control Tab reaches in view. Keep them green: give every control an
accessible name (an icon-only button carries an `aria-label`), keep the skip
link and the `:focus-visible` ring, and de-emphasize with color rather than
`opacity`, which dims text below the contrast floor. Base UI parts come only
through `src/lib/baseui`, whose `CSPProvider` keeps them from writing the
`<style>` the server's content policy refuses; ScrollArea and Select, the only
parts that write one, are not used, and a choice from a fixed list is a native
`<select>` (TRADE-50), in whose own shadow tree WebKit writes the one inline
style attribute the policy allows, by its hash (TRADE-55).
`e2e/server/policy.spec.ts` holds the page the binary serves to that real
content policy while it opens Settings' Tabs, the tag picker's Combobox and
the directory picker's Autocomplete, in its Dialog.

## Server text reaches the page as text

The web server does not sanitize on the way out, though the store asks the
seam that renders stored text to: the web's rendering seam is React, which
escapes every text node, and the lint bans `dangerouslySetInnerHTML` and
`innerHTML`, so server text reaches the page only as inert text. Rendering
server text as HTML, or handing it to a sink React does not escape, ends that
exemption and brings the sanitize pass to that seam. A URL attribute is such a
sink: a URL from the server reaches an `href` only through `NewTabLink`
(`src/lib/NewTabLink.tsx`), which allows http and https alone, and the web
lint fails any other `<a href={…}>`; any other attribute sink ends the
exemption.

## Lint caps and bans

`eslint.config.js` names the reason beside each. Complexity 10, at most four
parameters, four levels of nesting and three nested callbacks, 80 lines a
function, no nested ternary and no parameter reassigned; a `switch` over a
union names every member; a list key names its item, never its index; no
`opacity-N` dimming and no all-caps heading; a running control is `held`, not
`disabled`; every `eslint-disable` gives its reason after `--`, and an unused
one fails.

## Tests: red, green, refactor

- **Red first, through what a user sees.** A change starts as a test that
  renders the component or opens the page, acts as a user would, and asserts
  on role, accessible name, text and `aria-*` state (`aria-current`,
  `aria-pressed`, `aria-disabled`); watch it fail, then make the change. The
  tests are black-box: the lint fails an assertion on a class, an inline
  style or a `data-*` hook.
- **Arrange, Act, Assert** is marked in every unit test and spec, as the
  root's rule says, through the TypeScript twin of `cmd/testshape`
  (`eslint-rules/arrange-act-assert.js`, in `corepack yarn lint`). A test is
  `test`, never `it`.
- **Unit tests** (Vitest, Testing Library and user-event, `*.test.tsx` beside
  their subject): `renderWithClient` (`src/test/renderWithClient.tsx`)
  renders inside a query client with the app's own defaults; `fakeApi`
  (`src/test/fakeApi.ts`) answers `fetch` by route and records every request,
  which `src/test-setup.ts` otherwise refuses; `src/test/fixtures.ts`,
  `keyboard.tsx`, `settings.ts` and the rest of `src/test` are the shared
  fixtures. The coverage thresholds in `vitest.config.ts` are held at
  floor(measured) − 2.
- **End-to-end specs** (Playwright, under `e2e/`, `task e2e`, which
  `task check` leaves out): a hermetic spec takes `test` from
  `e2e/support/fixtures.ts`, whose page refuses any API route a spec leaves
  unanswered; themes and widths come from `e2e/support/cockpit.ts`; and
  `expectReachableAndClean` (`e2e/support/reachable.ts`) holds a surface to
  the layout and axe rules. No raw locator. Chromium, Firefox and WebKit
  each run every spec, so a spec holds to what every engine does, never to
  Chromium's own console wording or behavior. Everything under `e2e/` counts toward the
  directory budgets, so specs live in per-surface folders; the specs under
  `e2e/server` drive the real binary instead, and `e2e/docs/media.spec.ts`
  takes the docs' screenshots in a run of its own
  (`playwright.docs.config.ts`, `corepack yarn media`, which
  `task docs:media` wraps), which `task e2e` and CI leave out and which
  rewrites the committed images in `docs/assets/images`. It takes its test
  from `e2e/docs/demo.ts`, which serves each screen from a demo of its own
  (`cmd/demo -web`, built into `tmp/demo` first), and its steps from
  `e2e/docs/loop.ts`, each waiting for what the page shows it did; so does
  `e2e/docs/walkthrough.spec.ts`, which films the loop for the WebP through
  `e2e/docs/film.ts` at the pace that file sets. Every image either takes is
  held to `e2e/docs/capture.ts`: fonts loaded, no text or control cut by an edge
  of the window or of any pane that clips, nothing mid-animation; and each of
  the walkthrough's keyframes to `e2e/docs/cover.ts` too, the pointer parked
  where it covers nothing the frame shows; and the frame each beat holds to
  its keyframe by `e2e/docs/held.ts`, every pixel within the rounding a
  screenshot and the screencast differ by, which fails the film on a keyframe
  no frame shows. `e2e/docs/fidelity.spec.ts`
  (`corepack yarn media:check`) holds the encoded WebP to the film, after
  `task docs:media` encodes it.

## Dependencies and trade-offs

- The week-old age gate has two lists for an npm version: a waiver or a CVE
  fix goes in `web/.yarnrc.yml`'s `npmPreapprovedPackages`, which yarn's own
  gate reads at install, and in `scripts/dependency-age-exceptions.txt`.
- The kept trade-offs that bind here: TRADE-3 (read-only under `--dry-run`),
  TRADE-4 (`staleTime: Infinity`, the event stream the sole freshness
  source), TRADE-8 (v8's branch count), TRADE-48 (a latin-only face),
  TRADE-49 (at most 200 rows in the directory picker), TRADE-50, TRADE-51
  (Settings checks a few settings the server checks again) and TRADE-55 (the
  one inline style the policy allows, WebKit's for a native `<select>`).
