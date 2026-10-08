#!/usr/bin/env bash
#
# Tests for coverage-summary.sh: it collapses a coverage profile and the gobco
# stats into one JSON summary, taking the statement number from the gate so the
# two sides of a PR comparison agree, and fails when an input it needs is absent.
#
# Usage:
#   scripts/coverage-summary_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly summary="${here}/coverage-summary.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The same minimal 50%-covered module coverage-gate's own test uses; the summary
# reads the statement number through the gate, so it must run from here.
module="${workdir}/module"
mkdir -p "${module}"
cat >"${module}/go.mod" <<'EOF'
module example

go 1.27
EOF
cat >"${module}/f.go" <<'EOF'
package example

func A() int {
	return 1
}

func B() int {
	return 2
}
EOF
readonly profile="${module}/cover.out"
cat >"${profile}" <<'EOF'
mode: set
example/f.go:3.16,5.2 1 1
example/f.go:7.16,9.2 1 0
EOF

# A gobco stats directory with one condition seen only one way: one of two arms.
stats="${workdir}/stats"
mkdir -p "${stats}"
printf '[{"TrueCount":1,"FalseCount":0}]\n' >"${stats}/pkg.json"

# expect_fail runs the summary from the module and requires a non-zero exit.
#   expect_fail <name> <arg>...
expect_fail() {
  expect_exit fail "$1" run_in "${module}" "${summary}" "${@:2}"
}

# A valid profile and stats directory produce both metrics and nothing else:
# half the statements ran, and one of the one condition's two arms was seen.
# They are compared as numbers, since jq keeps the gate's "50.0" as written.
count_case
if out="$(cd "${module}" && "${summary}" "${profile}" "${stats}")"; then
  if ! jq -e 'keys == ["branch", "statements"] and .statements == 50 and .branch == 50' \
    <<<"${out}" >/dev/null 2>&1; then
    fail_case "summary" "want statements 50 and branch 50, got ${out}"
  fi
else
  fail_case "summary on valid inputs" "exited non-zero"
fi

# A missing stats directory argument fails rather than summarizing half.
expect_fail "no stats directory argument" "${profile}"

# No arguments at all fails.
expect_fail "no arguments"

finish_tests
