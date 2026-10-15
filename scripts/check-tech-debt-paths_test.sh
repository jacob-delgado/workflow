#!/usr/bin/env bash
#
# Tests for check-tech-debt-paths.sh, what reads as a repository path and how
# one is looked up: it passes when every repository path TECH_DEBT.md names in
# backticks is tracked, and fails, naming the line and the path, on a file or a
# directory that is gone, a path written from the wrong directory or from
# outside the repository, a path written from ./, padded with spaces or with a
# line, a column or an anchor, a path in an extension a tracked file has though
# no source one, a path only an untracked or unstaged .gitignore, this clone's
# or this user's excludes ignore, and a bare file name no tracked file carries.
# It leaves alone what is no repository path: an absolute or home path, a
# dot-file named alone, a bare name in such an extension, a product or API name
# ending in .js or .json, a path or directory a tracked .gitignore ignores,
# nested or at the root, under a hook's GIT_DIR too, a Go import or module
# path, in an extension a tracked file has too, a platform, a glob, a URL, a
# command and a Go symbol, with its package path or without. A repository that
# tracks no TECH_DEBT.md is refused, and so is a run with no directory for the
# ignore check. check-tech-debt-paths_markdown_test.sh tests how the Markdown
# around a path is read.
#
# Usage:
#   scripts/check-tech-debt-paths_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/tech-debt-paths-fixtures.sh
source "${here}/lib/tech-debt-paths-fixtures.sh"

expect_backlog pass "every path tracked" "9 repository path(s)" <<'EOF'
`internal/tui/model.go` and `internal/tui` and `internal/tui/` hold it,
`web/src/app.tsx` draws it, and `model.go` and `TECH_DEBT.md` name it, as
`./web/src/app.tsx`, `internal/tui/model.go:12` and `scripts/` do.
EOF

expect_backlog pass "what is no repository path, skipped" "1 repository path(s)" <<'EOF'
`/dev/full`, `/home/linuxbrew`, `/usr/local/go/src/os/exec.go`, `/`,
`2>/dev/null`, `encoding/json`,
`charm.land/fang/v2`, `muesli/roff`, `@base-ui/react`, `darwin/arm64`,
`internal/*/doc.go`, `web/**`, `https://example.com/a/b.go`,
`//go:build !unix`, `POST /api/runs`, `go test ./internal/tui`,
`config.Load`, `tea.Model`, `internal/tui.Model`, `~/.workflow.json`,
`charm.land/huh/v2/field_input.go`,
`.workflow.json`, ``a `internal/gone.go` quoted``, `TRADE-3` and
`scripts/gate.sh` once.
EOF

expect_backlog fail "a file that is gone" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
Text.

It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "a path written from ./" "TECH_DEBT.md:1: ./internal/tui/spine.go" <<'EOF'
It lives in `./internal/tui/spine.go`.
EOF

expect_backlog fail "a path with a line number" "TECH_DEBT.md:1: internal/tui/spine.go:42" <<'EOF'
It lives in `internal/tui/spine.go:42`.
EOF

expect_backlog fail "a path with a line and a column" "TECH_DEBT.md:1: internal/tui/spine.go:42:7" <<'EOF'
It lives in `internal/tui/spine.go:42:7`.
EOF

expect_backlog fail "a path with a line anchor" "TECH_DEBT.md:1: internal/tui/spine.go#L42" <<'EOF'
It lives in `internal/tui/spine.go#L42`.
EOF

expect_backlog fail "a path with a section anchor" "TECH_DEBT.md:1: docs/guide.md#tests" <<'EOF'
It is written in `docs/guide.md#tests`.
EOF

expect_backlog fail "a path padded with spaces inside its backticks" "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
It lives in ` internal/tui/spine.go `.
EOF

expect_backlog fail "a directory that is gone" "TECH_DEBT.md:1: internal/editor" <<'EOF'
It lives in `internal/editor`.
EOF

expect_backlog fail "a path written from the wrong directory" "TECH_DEBT.md:1: src/app.tsx" <<'EOF'
It lives in `src/app.tsx`.
EOF

expect_backlog fail "a bare file name no tracked file carries" "TECH_DEBT.md:1: spine.go" <<'EOF'
It lives in `spine.go`.
EOF

expect_backlog pass "a product or API name ending in .js or .json, skipped" "1 repository path(s)" <<'EOF'
Under `Node.js`, `Response.json` reads the body; see `scripts/gate.sh`.
EOF

ignored="$(
  repo ignored <<'EOF'
# Technical debt

The build writes `web/dist/app.js` and `lefthook-local.yml`; see `scripts/gate.sh`.
EOF
)"
printf 'web/dist/\nlefthook-local.yml\n' >"${ignored}/.gitignore"
git -C "${ignored}" add .gitignore
expect_output pass "a path git ignores, skipped" "1 repository path(s)" gate "${ignored}"

excluded_here="$(
  repo excluded-here <<'EOF'
# Technical debt

It lives in `internal/tui/spine.go`.
EOF
)"
printf 'internal/tui/spine.go\n' >"${excluded_here}/.git/info/exclude"
printf 'internal/tui/spine.go\n' >"${workdir}/global-excludes"
git -C "${excluded_here}" config core.excludesFile "${workdir}/global-excludes"
mkdir -p "${workdir}/template/info"
printf 'internal/tui/spine.go\n' >"${workdir}/template/info/exclude"
printf '[core]\n\texcludesFile = %s\n[init]\n\ttemplateDir = %s\n' "${workdir}/global-excludes" \
  "${workdir}/template" >"${workdir}/global-config"
expect_output fail "a path only this clone's or this user's excludes ignore, read" \
  "TECH_DEBT.md:3: internal/tui/spine.go" gate_under "${excluded_here}" GIT_CONFIG_GLOBAL="${workdir}/global-config"

untracked_ignore="$(
  repo untracked-ignore <<'EOF'
# Technical debt

It lives in `internal/tui/spine.go`.
EOF
)"
printf 'spine.go\n' >"${untracked_ignore}/internal/tui/.gitignore"
expect_output fail "a path only an untracked .gitignore ignores, read" "TECH_DEBT.md:3: internal/tui/spine.go" \
  gate "${untracked_ignore}"

ignored_directory="$(
  repo ignored-directory <<'EOF'
# Technical debt

The tests write `web/coverage`; see `scripts/gate.sh`.
EOF
)"
printf 'web/coverage/\n' >"${ignored_directory}/.gitignore"
git -C "${ignored_directory}" add .gitignore
expect_output pass "a directory git ignores, absent from the clone, skipped" "1 repository path(s)" \
  gate "${ignored_directory}"

expect_output pass "a path git ignores, skipped under a hook's GIT_DIR" "1 repository path(s)" \
  gate_under "${ignored}" GIT_DIR="${ignored}/.git"

staged_rules="$(
  repo staged-rules <<'EOF'
# Technical debt

It lives in `internal/tui/spine.go`.
EOF
)"
: >"${staged_rules}/.gitignore"
git -C "${staged_rules}" add .gitignore
printf 'internal/tui/spine.go\n' >"${staged_rules}/.gitignore"
expect_output fail "a path only an unstaged edit of .gitignore ignores, read" "TECH_DEBT.md:3: internal/tui/spine.go" \
  gate "${staged_rules}"

nested_rules="$(
  repo nested-rules <<'EOF'
# Technical debt

The tests write `internal/tui/cache/run.go`; see `scripts/gate.sh`.
EOF
)"
printf 'cache/\n' >"${nested_rules}/internal/tui/.gitignore"
git -C "${nested_rules}" add internal/tui/.gitignore
expect_output pass "a path a tracked nested .gitignore ignores, skipped" "1 repository path(s)" gate "${nested_rules}"

no_temp="$(
  repo no-temp <<'EOF'
# Technical debt

It lives in `internal/tui/spine.go`.
EOF
)"
expect_output 2 "no directory for the ignore check" "cannot make a directory" \
  gate_under "${no_temp}" TMPDIR="${workdir}/no-such-directory"

outside="$(
  repo outside <<'EOF'
# Technical debt

It lives in `../elsewhere/gone.go`.
EOF
)"
expect_exit fail "a path outside the repository" gate "${outside}"
count_case
if grep -q "fatal" "${case_output}"; then
  fail_case "a path outside the repository" "git's own error reached the output: $(cat "${case_output}")"
fi

other_extension="$(
  repo other-extension <<'EOF'
# Technical debt

It locks in `web/gone.lock`.
EOF
)"
touch "${other_extension}/web/yarn.lock"
git -C "${other_extension}" add web/yarn.lock
expect_output fail "a path ending in an extension a tracked file has" "TECH_DEBT.md:3: web/gone.lock" \
  gate "${other_extension}"

go_import="$(
  repo go-import <<'EOF'
`encoding/json` decodes it; see `scripts/gate.sh`.
EOF
)"
touch "${go_import}/web/package.json"
git -C "${go_import}" add web/package.json
expect_output pass "a Go import path in an extension a tracked file has, skipped" "1 repository path(s)" \
  gate "${go_import}"

package_file="$(
  repo package-file <<'EOF'
# Technical debt

A package ships `font.lock`; see `scripts/gate.sh`.
EOF
)"
touch "${package_file}/web/yarn.lock"
git -C "${package_file}" add web/yarn.lock
expect_output pass "a bare name in a tracked extension no source has, skipped" "1 repository path(s)" \
  gate "${package_file}"

untracked="${workdir}/untracked"
mkdir -p "${untracked}"
git -C "${untracked}" init -q
printf '# Technical debt\n' >"${untracked}/TECH_DEBT.md"
expect_exit 2 "a repository that tracks no TECH_DEBT.md" gate "${untracked}"

finish_tests
