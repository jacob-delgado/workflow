#!/usr/bin/env bash
# test-summary.sh — run the unit and end-to-end suites and report how many tests
# passed, were skipped and failed for each, with the coverage report.
#
# It prints a markdown table to stdout, and when $GITHUB_STEP_SUMMARY is set it
# appends the same markdown there, so a CI run shows the numbers without opening
# a log.
#
# Usage: test-summary.sh <go-package-root>...
#
# Advisory: this reports, it does not gate. The gates are the separate test jobs.
# It runs each suite independently and reports a dash where a suite could not run
# (for example Playwright without its browser), rather than failing the report.
#
# The Go roots are arguments rather than a list of its own: Taskfile.yml passes
# GO_PKGS, the roots its test task runs, so the Go row counts the same packages.
set -uo pipefail

if (($# == 0)); then
  echo "usage: test-summary.sh <go-package-root>..." >&2
  exit 2
fi
readonly go_roots=("$@")

root="$(cd "$(dirname "${0}")/.." && pwd)"
readonly root
work="$(mktemp -d)"
readonly work
trap 'rm -rf "${work}"' EXIT

# count_json reads a stream of `go test -json` events and prints "pass skip fail"
# as the count of tests that ended in each action (subtests included). With no
# test-level events at all — the tests could not build, or `go` did not run — it
# prints dashes rather than a misleading "0 0 0".
count_json() {
  jq -rs '
    [.[] | select(.Test != null and (.Action | IN("pass", "skip", "fail")))]
    | if length == 0 then "- - -"
      else
        (map(select(.Action == "pass")) | length) as $pass
        | (map(select(.Action == "skip")) | length) as $skip
        | (map(select(.Action == "fail")) | length) as $fail
        | "\($pass) \($skip) \($fail)"
      end
  ' "${1}" 2>/dev/null || echo "- - -"
}

# go_row runs the Go unit tests as the test task does — over the roots given,
# with the race detector — with a coverage profile, and records the counts and
# the statement coverage the gate reports.
go_row() {
  local counts pct
  go -C "${root}" test -race -json -coverprofile="${work}/go.cov" "${go_roots[@]}" >"${work}/go.json" 2>/dev/null || true
  counts="$(count_json "${work}/go.json")"

  # Taken from the gate, with a floor of 0 so it never fails, as
  # coverage-summary.sh does: the gate leaves generated code out of the
  # denominator, so a total read straight off the profile would differ from the
  # number the build enforces. The gate runs `go tool cover`, which resolves the
  # profile's paths against the module, hence from the root. A bare number (or
  # a dash), so the report's pct helper adds the one % sign.
  pct="-"
  if [[ -s "${work}/go.cov" ]]; then
    pct="$(
      cd "${root}" && scripts/coverage-gate.sh "${work}/go.cov" 0 2>/dev/null \
        | awk '/^Coverage/ {match($0, /[0-9]+(\.[0-9]+)?%/); print substr($0, RSTART, RLENGTH - 1); exit}'
    )"
  fi

  read -r go_pass go_skip go_fail <<<"${counts}"
  go_cov="${pct:--}"
}

# vitest_row runs the web unit tests with coverage and records the counts and the
# coverage percentages from the json-summary the vitest config already writes.
vitest_row() {
  vitest_pass="-" vitest_skip="-" vitest_fail="-"
  vitest_lines="-" vitest_branches="-" vitest_statements="-" vitest_functions="-"

  # Coverage goes to a per-run directory, not the working tree's web/coverage, so
  # a run that aborts before writing it reports a dash rather than a previous
  # run's stale numbers.
  (cd "${root}/web" && corepack yarn vitest run --coverage \
    --coverage.reportsDirectory="${work}/coverage" \
    --reporter=json --outputFile="${work}/vitest.json") >/dev/null 2>&1 || true

  if [[ -s "${work}/vitest.json" ]]; then
    read -r vitest_pass vitest_skip vitest_fail <<<"$(jq -r \
      '"\(.numPassedTests) \(.numPendingTests) \(.numFailedTests)"' "${work}/vitest.json" 2>/dev/null)"
  fi

  local summary="${work}/coverage/coverage-summary.json"
  if [[ -s "${summary}" ]]; then
    read -r vitest_statements vitest_branches vitest_functions vitest_lines <<<"$(jq -r \
      '"\(.total.statements.pct) \(.total.branches.pct) \(.total.functions.pct) \(.total.lines.pct)"' \
      "${summary}" 2>/dev/null)"
  fi
}

# playwright_row runs the end-to-end suite and records the counts from its json
# reporter. A run that cannot start (no browser) leaves the counts as dashes.
playwright_row() {
  e2e_pass="-" e2e_skip="-" e2e_fail="-"

  (cd "${root}/web" && corepack yarn playwright test --reporter=json) >"${work}/pw.json" 2>/dev/null || true

  if [[ -s "${work}/pw.json" ]] && jq -e '.stats' "${work}/pw.json" >/dev/null 2>&1; then
    read -r e2e_pass e2e_skip e2e_fail <<<"$(jq -r \
      '"\(.stats.expected) \(.stats.skipped) \(.stats.unexpected + .stats.flaky)"' "${work}/pw.json" 2>/dev/null)"
  fi
}

# pct renders a coverage number as a percentage, or a dash when it is missing —
# a dash, "null" from an absent jq field, or empty — so an unmeasured cell reads
# as "-" rather than "-%" or "null%".
pct() {
  case "${1}" in
    "" | "-" | "null") echo "—" ;;
    *) echo "${1}%" ;;
  esac
}

report() {
  echo "## Test summary"
  echo ""
  echo "| Suite | Passed | Skipped | Failed |"
  echo "| --- | ---: | ---: | ---: |"
  echo "| Go unit | ${go_pass} | ${go_skip} | ${go_fail} |"
  echo "| Web unit (vitest) | ${vitest_pass} | ${vitest_skip} | ${vitest_fail} |"
  echo "| E2E (Playwright) | ${e2e_pass} | ${e2e_skip} | ${e2e_fail} |"
  echo ""
  echo "### Coverage"
  echo ""
  echo "| Target | Statements | Branches | Functions | Lines |"
  echo "| --- | ---: | ---: | ---: | ---: |"
  echo "| Web (vitest) | $(pct "${vitest_statements}") | $(pct "${vitest_branches}") | $(pct "${vitest_functions}") | $(pct "${vitest_lines}") |"
  echo "| Go | $(pct "${go_cov}") | — | — | — |"
}

go_row
vitest_row
playwright_row

report
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  report >>"${GITHUB_STEP_SUMMARY}"
fi
