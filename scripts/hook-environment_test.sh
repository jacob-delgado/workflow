#!/usr/bin/env bash
#
# Tests that the gate's script tests leave alone the repository a git hook's
# environment names. Git exports GIT_DIR to its hooks and to `git rebase
# --exec` (and GIT_INDEX_FILE to a pre-commit), and from a linked worktree it is
# an absolute path into the shared repository: a script test's `git -C <temp
# dir>` would then work on that repository instead of its own. So every other
# script test runs here with that environment naming a sentinel repository,
# which must come out exactly as it went in.
#
# Usage:
#   scripts/hook-environment_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

# This test's own git must not follow the environment it was started with.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

readonly sentinel="${workdir}/sentinel"
git init -q "${sentinel}"
cp -R "${sentinel}" "${workdir}/before"

failures=0

for test in "${here}"/*_test.sh "${here}"/release/*_test.sh; do
  if [[ "${test}" == "${here}/hook-environment_test.sh" ]]; then
    continue
  fi

  if ! GIT_DIR="${sentinel}/.git" GIT_INDEX_FILE="${sentinel}/.git/index" \
    "${test}" >"${workdir}/output" 2>&1; then
    echo "FAIL ${test#"${here}/"} with a hook's environment:" >&2
    cat "${workdir}/output" >&2
    failures=$((failures + 1))
  fi
done

if ! diff -r "${workdir}/before" "${sentinel}" >"${workdir}/changes" 2>&1; then
  echo "FAIL the script tests wrote into the repository a hook's environment names:" >&2
  cat "${workdir}/changes" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  exit 1
fi

echo "hook-environment: every script test left the repository a hook names alone"
