#!/usr/bin/env bash
#
# Tests for pr-comment.mjs: the pull request comment carries each suite's test
# counts, in a fixed order, with a dash for a suite whose counts are missing or
# null, above the coverage table and its delta against main.
#
# Usage:
#   scripts/pr-comment_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly render="${here}/pr-comment.mjs"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

mkdir -p "${workdir}/counts"
printf '%s\n' '{"suite":"Go unit","passed":2310,"skipped":3,"failed":0}' >"${workdir}/counts/go-unit.json"
printf '%s\n' '{"suite":"Web unit","passed":561,"skipped":0,"failed":2}' >"${workdir}/counts/web-unit.json"
printf '%s\n' '{"suite":"E2E","passed":null,"skipped":null,"failed":null}' >"${workdir}/counts/e2e.json"
printf '%s\n' '{"statements":97.8,"branch":94.6}' >"${workdir}/pr.json"
printf '%s\n' '{"statements":97.9,"branch":94.6}' >"${workdir}/base.json"

# fail records a failed case with its reason.
#   fail <name> <reason>
fail() {
  echo "FAIL ${1}: ${2}" >&2
  failures=$((failures + 1))
}

# expect_line checks the rendered comment holds a line exactly.
#   expect_line <name> <line>
expect_line() {
  cases=$((cases + 1))
  if ! grep -qxF -- "${2}" "${workdir}/comment.md"; then
    fail "${1}" "want the line '${2}' in:"$'\n'"$(cat "${workdir}/comment.md")"
  fi
}

# Act: render with three suites' counts, one of them null, and none for E2E
# (server).
node "${render}" "${workdir}/pr.json" "${workdir}/base.json" "${workdir}/counts" >"${workdir}/comment.md" || true

# Assert
expect_line "marker" '<!-- coverage-report -->'
expect_line "tests heading" '## Tests'
expect_line "go counts" '| Go unit | 2310 | 3 | 0 |'
expect_line "web counts" '| Web unit (vitest) | 561 | 0 | 2 |'
expect_line "null counts read as dashes" '| E2E | — | — | — |'
expect_line "missing counts read as dashes" '| E2E (server) | — | — | — |'
expect_line "coverage with its delta" '| Statements | 97.8% | -0.1 |'
expect_line "coverage unchanged" '| Conditions (gobco) | 94.6% | ±0 |'

# Assert: the tests table comes before the coverage table.
cases=$((cases + 1))
tests_at="$(grep -n '^## Tests' "${workdir}/comment.md" | cut -d: -f1 || true)"
coverage_at="$(grep -n '^## Coverage' "${workdir}/comment.md" | cut -d: -f1 || true)"
if [[ -z "${tests_at}" || -z "${coverage_at}" ]] || ((tests_at > coverage_at)); then
  fail "order" "want Tests above Coverage"
fi

# Act: no coverage summary and no baseline, as when the Test job failed.
node "${render}" "${workdir}/absent.json" "${workdir}/absent.json" "${workdir}/counts" >"${workdir}/comment.md" || true

# Assert
expect_line "no coverage" '| Statements | n/a | — |'

if ((failures > 0)); then
  echo "pr-comment_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "pr-comment_test: ${cases} case(s) passed."
