#!/usr/bin/env bash
#
# Tests for check-go-version.sh: it passes when every source names the same Go,
# and fails on a drifted series, a drifted patch, or a source it cannot read.
#
# Usage:
#   scripts/check-go-version_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-go-version.sh"

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

# tree writes the five version sources into a fresh directory.
#   tree <dir> <go.mod series> <docs patch> <mise patch> <dockerfile patch> <badge series>
tree() {
  local dir="$1" gomod="$2" docs="$3" mise="$4" docker="$5" badge="$6"
  mkdir -p "${dir}/docs" "${dir}/build"
  printf 'module example\n\ngo %s\n' "${gomod}" >"${dir}/go.mod"
  printf 'module example/docs\n\ngo %s\n' "${docs}" >"${dir}/docs/go.mod"
  printf 'go = "%s"\n' "${mise}" >"${dir}/mise.toml"
  printf 'ARG GO_VERSION=%s\n' "${docker}" >"${dir}/build/Dockerfile"
  printf '[![Go](https://img.shields.io/badge/go-%s-00ADD8)](https://go.dev/)\n' "${badge}" >"${dir}/README.md"
}

# Every source names the same series and the same pinned patch: the gate passes.
agree="${workdir}/agree"
tree "${agree}" "1.27" "1.27.1" "1.27.1" "1.27.1" "1.27"
expect pass "every source agrees" "${agree}"

# One toolchain pin jumps a minor version: the series no longer matches.
series="${workdir}/series"
tree "${series}" "1.27" "1.27.1" "1.28.0" "1.27.1" "1.27"
expect fail "a drifted series" "${series}"

# The series matches everywhere, but two toolchain pins name different patches.
patch="${workdir}/patch"
tree "${patch}" "1.27" "1.27.1" "1.27.1" "1.27.2" "1.27"
expect fail "a drifted patch" "${patch}"

# A source the gate cannot read must stop it, not pass on what it did read.
missing="${workdir}/missing"
tree "${missing}" "1.27" "1.27.1" "1.27.1" "1.27.1" "1.27"
rm "${missing}/mise.toml"
expect fail "a source that cannot be read" "${missing}"

if ((failures > 0)); then
  echo "check-go-version_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-go-version_test: ${cases} case(s) passed."
