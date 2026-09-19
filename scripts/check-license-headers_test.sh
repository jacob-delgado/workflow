#!/usr/bin/env bash
#
# Tests for check-license-headers.sh: a file missing the SPDX header fails,
# a file that has it passes, and an empty repository is not mistaken for a clean
# one — the failure mode that would pass every file in the repo.
#
# Usage:
#   scripts/check-license-headers_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-license-headers.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

readonly copyright='// Copyright 2026 Jacob Delgado'
readonly spdx='// SPDX-License-Identifier: Apache-2.0'

# expect_args runs the gate over explicit files and checks its exit.
#   expect_args <pass|fail> <name> <file>...
expect_args() {
  local want="$1" name="$2" got
  shift 2
  cases=$((cases + 1))

  if "${check}" "$@" >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
}

# expect_dir runs the gate with no arguments inside a directory.
expect_dir() {
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

# A file with both header lines passes.
good="${workdir}/good.go"
printf '%s\n%s\n\npackage x\n' "${copyright}" "${spdx}" >"${good}"
expect_args pass "a file with the header" "${good}"

# A file with the copyright but no SPDX line fails: half a header is not one.
half="${workdir}/half.go"
printf '%s\n\npackage x\n' "${copyright}" >"${half}"
expect_args fail "a file missing the SPDX line" "${half}"

# A file with no header at all fails.
bare="${workdir}/bare.go"
printf 'package x\n' >"${bare}"
expect_args fail "a file with no header" "${bare}"

# A generated file (oapi-codegen's banner, no SPDX header) is exempt.
generated="${workdir}/types.gen.go"
printf 'package x\n' >"${generated}"
expect_args pass "a generated file without the header" "${generated}"

# A repository whose one Go file lacks the header fails, even though the file is
# only staged, not committed — the case that would otherwise slip through.
repo="${workdir}/repo"
mkdir -p "${repo}"
git -C "${repo}" init -q
printf 'package x\n' >"${repo}/bare.go"
git -C "${repo}" add bare.go
expect_dir fail "a repository with a headerless file" "${repo}"

# Outside a repository, the filesystem walk still finds and fails the file,
# rather than reporting no Go files and passing.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
printf 'package x\n' >"${notrepo}/bare.go"
expect_dir fail "a directory that is not a repository" "${notrepo}"

if ((failures > 0)); then
  echo "check-license-headers_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-license-headers_test: ${cases} case(s) passed."
