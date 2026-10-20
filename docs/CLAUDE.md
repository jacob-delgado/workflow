# docs

How the documentation site is built, what in it is generated, and which pages
are contracts the code is held to, for any change under `docs/`. The root
`CLAUDE.md` still applies; this adds what holds here alone.
`docs/content/docs/contributing.md` is the page a reader of the site gets.

## One build path

- The site is Hugo with the hugo-book theme, which arrives as a Hugo Module:
  its version is pinned in `docs/go.mod` and `docs/go.sum`, so updating it is
  a reviewable diff, and `docs/go.mod`'s Go version is one of the copies
  `scripts/check-go-version.sh` holds to `mise.toml`.
- `task docs:serve` previews it; `task docs:build` builds it into
  `docs/public`, and is the command CI publishes with, so there is one build
  path rather than two that drift.
- A new page needs no menu entry, since the theme draws the navigation from
  the content tree: only a `weight` in its front matter if it should not sort
  alphabetically (`hugo.toml`). Link between pages with
  `{{< relref "/docs/<page>#<anchor>" >}}`.
- A page shows an image from `docs/assets` with hugo-book's
  `{{< image src="images/…" alt="…" loading="lazy" >}}` shortcode, never a
  Markdown image: hugo-book prints a Markdown image's path as written, and the
  site's `/workflow/` base path breaks a `docs/static` one. The shortcode is
  experimental in the theme, and it does not escape its alt text, so the text
  holds no double quote.
- `unsafe = false` in `hugo.toml`: no raw HTML in a page. The one exception is
  the HTML-comment version markers on the install page
  (`x-release-please-start-version`), which release-please rewrites on each
  release, so leave them where they are.

## Generated, never hand-edited

Everything under `docs/content/docs/reference/` but `_index.md` is generated
from the Cobra command tree by `task docs:gen` (`cmd/docsgen`), each page
saying so at its top, and must never be edited by hand: a command or flag
added, removed or renamed in `internal/cli` runs it, in the same commit.
`task docs:check` (`scripts/check-docs-drift.sh`, in `task lint`) fails when
the two disagree, which is what stops a new flag shipping undocumented.
`reference/_index.md` is hand-written; the drift check copies it in for the
comparison.

`task docs:media` (`scripts/docs-media.sh`) writes every file in
`docs/assets/images`, from `docs/media/terminal.tape` over the demo and from
the web page the demo serves (`cmd/demo -web`). Those files are never edited or replaced by hand:
change the tape, the demo or the screen and commit it, since the task
refuses to run over changes not committed to what the images show; run the
task on an idle machine, since it fails a recording that lost time; and look
at each image before committing it, since nothing compares them (TRADE-56).

## Contract pages

These pages promise what the code does, and a change to the behavior changes
the page in the same commit:

- `errors.md`: one `##` heading per problem code of the `--web` API, which
  each problem's `type` URI points at.
- `scripting.md`: the command line's exit statuses, streams, JSON shapes and
  unattended flags, the contract a script relies on; a change to it is a
  breaking change.
- `usage.md`: the terminal's keys and promises, which UX.md audits;
  `configuration.md`: every key of `.workflow.json`, the rebindable actions
  among them; `web.md`: the `--web` page, a write running to its end
  (TRADE-37) among its promises.

## Tests: red, green, refactor

A prose page has no unit test, but a contract page is held by Go tests that
read it, so a change to one starts there, red first:

- `internal/webserver/errorsdoc_test.go` fails on a problem code with no
  heading in `errors.md`, or no family in `scripting.md`, and on a `type` URI
  that does not end in its heading's anchor.
- `internal/cli/scripting_docs_test.go` fails when `scripting.md` stops
  naming every command, or where each prints.
- `internal/tui/keys_docs_test.go` holds `configuration.md`'s rebindable
  actions and `usage.md`'s key table to the terminal's bindings, and
  `internal/tui/helpgroups_test.go` its panes' actions to the help's groups.

`task test:docs` runs these tests, which is how a docs-only change (`task
check:docs`, and CI for such a pull request) still checks the contract pages.
It finds them by name: each one's starts with `TestDocs`, and a new contract
test's must too, or the docs-only gate never runs it.

Add the code, the command or the key with its test, watch that test fail on
the page, then write the page. Each of those tests is a black-box test marked
`// Arrange`, `// Act`, `// Assert`, as the root asks.

## Lint

`markdownlint-cli2` (`.markdownlint-cli2.yaml`) and `typos` run on every page
in `task lint` and on commit; the generated reference pages are left out of
the Markdown lint, since autofixing them would make `task docs:check` fail. CI
skips the Markdown lint on a pull request that changes no Markdown, nothing
under `docs/` or `build/markdownlint/`, which pins the linter, and neither
`.markdownlint-cli2.yaml` nor `mise.toml`, which pins the node it runs under
(`scripts/markdown-changed.sh`); a push to `main` always runs it.
