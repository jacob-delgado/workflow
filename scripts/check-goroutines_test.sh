#!/usr/bin/env bash
#
# Tests for check-goroutines.sh: a bare `go` statement fails the gate unless it
# is in internal/proc or a test file, and a repository git cannot read must not
# pass on nothing searched.
#
# Usage:
#   scripts/check-goroutines_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-goroutines.sh"

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

# goroutine writes a Go file that starts one; plain writes one that does not.
goroutine() {
  printf 'package x\n\nfunc run() {\n\tgo work()\n}\n'
}

plain() {
  printf 'package x\n\nfunc run() {}\n'
}

# repo makes a git repository with one file at path holding contents.
#   repo <dir> <path> <contents-command>
repo() {
  local dir="$1" path="$2"
  mkdir -p "${dir}/$(dirname "${path}")"
  git -C "${dir}" init -q
  "$3" >"${dir}/${path}"
  git -C "${dir}" add "${path}"
}

# A goroutine in ordinary app code fails, which is the gate's whole point.
app="${workdir}/app"
repo "${app}" "internal/tui/run.go" goroutine
expect fail "a goroutine in app code" "${app}"

# The same goroutine inside internal/proc is where concurrency is allowed.
proc="${workdir}/proc"
repo "${proc}" "internal/proc/start.go" goroutine
expect pass "a goroutine in internal/proc" "${proc}"

# And a goroutine in a test file, which spawns its own helpers, is allowed.
test="${workdir}/test"
repo "${test}" "internal/tui/run_test.go" goroutine
expect pass "a goroutine in a test file" "${test}"

# A file with no goroutine passes.
clean="${workdir}/clean"
repo "${clean}" "internal/tui/run.go" plain
expect pass "no goroutine at all" "${clean}"

# A directory that is not a repository: git cannot list, so the gate must refuse.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
goroutine >"${notrepo}/run.go"
expect fail "a directory that is not a repository" "${notrepo}"

if ((failures > 0)); then
  echo "check-goroutines_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-goroutines_test: ${cases} case(s) passed."
