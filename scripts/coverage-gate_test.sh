#!/usr/bin/env bash
#
# Tests for coverage-gate.sh: coverage below the floor fails, coverage at or
# above it passes, and a floor nobody passed or a profile that is not there fails
# rather than being defaulted or measured as nothing.
#
# Usage:
#   scripts/coverage-gate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/coverage-gate.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# A minimal module with two functions, one covered and one not, so its profile
# measures 50.0%. go tool cover resolves the profile's paths against this module,
# so the gate must run from here.
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

# expect runs the gate from the module and checks its exit.
#   expect <pass|fail> <name> <arg>...
expect() {
  local want="$1" name="$2" got
  shift 2
  cases=$((cases + 1))

  if (cd "${module}" && "${gate}" "$@") >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
}

# 50% coverage clears a floor of 40 and misses a floor of 60.
expect pass "coverage above the floor" "${profile}" 40
expect fail "coverage below the floor" "${profile}" 60

# The floor is required, not defaulted: without one the gate must not pass.
expect fail "no floor argument" "${profile}"

# A profile that is not there, or is empty, fails rather than measuring nothing.
expect fail "a missing profile" "${module}/does-not-exist.out" 40
: >"${module}/empty.out"
expect fail "an empty profile" "${module}/empty.out" 40

if ((failures > 0)); then
  echo "coverage-gate_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "coverage-gate_test: ${cases} case(s) passed."
