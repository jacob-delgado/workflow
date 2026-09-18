#!/usr/bin/env bash
#
# Tests for check-file-length.sh: the gate must measure real files, and must
# never report success when it measured nothing.
#
# Usage:
#   scripts/check-file-length_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-file-length.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  local want="$1" name="$2" dir="$3" got
  cases=$((cases + 1))

  if (cd "${dir}" && "${check}") >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
}

# over_length writes a file of 901 lines, past the 500-line ceiling.
over_length() {
  awk 'BEGIN { for (i = 0; i < 901; i++) print "x" }'
}

# A directory that is not a git repository: git cannot list its files, and the
# gate must refuse rather than pass on nothing measured.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
over_length >"${notrepo}/big.go"
expect fail "a directory that is not a repository" "${notrepo}"

# A repository within the ceiling passes.
short="${workdir}/short"
mkdir -p "${short}"
git -C "${short}" init -q
printf 'package x\n' >"${short}/small.go"
git -C "${short}" add small.go
expect pass "a repository within the ceiling" "${short}"

# A repository with an over-length file fails, which is the gate's whole point.
long="${workdir}/long"
mkdir -p "${long}"
git -C "${long}" init -q
over_length >"${long}/big.go"
git -C "${long}" add big.go
expect fail "a repository with an over-length file" "${long}"

if ((failures > 0)); then
  echo "check-file-length_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-file-length_test: ${cases} case(s) passed."
