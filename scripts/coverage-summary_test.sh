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

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

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
  local name="$1"
  shift
  cases=$((cases + 1))

  if (cd "${module}" && "${summary}" "$@") >/dev/null 2>&1; then
    echo "FAIL ${name}: want fail, got pass" >&2
    failures=$((failures + 1))
  fi
}

# A valid profile and stats directory produce a JSON summary naming both metrics.
cases=$((cases + 1))
if out="$(cd "${module}" && "${summary}" "${profile}" "${stats}")"; then
  if [[ "${out}" != *'"statements"'* ]] || [[ "${out}" != *'"branch"'* ]]; then
    echo "FAIL summary shape: got ${out}" >&2
    failures=$((failures + 1))
  fi
else
  echo "FAIL summary on valid inputs: exited non-zero" >&2
  failures=$((failures + 1))
fi

# A missing stats directory argument fails rather than summarizing half.
expect_fail "no stats directory argument" "${profile}"

# No arguments at all fails.
expect_fail "no arguments"

if ((failures > 0)); then
  echo "coverage-summary_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "coverage-summary_test: ${cases} case(s) passed."
