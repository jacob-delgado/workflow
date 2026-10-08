#!/usr/bin/env bash
#
# Runs every other script test, once, the way a git hook would: git exports
# GIT_DIR to its hooks and to `git rebase --exec` (and GIT_INDEX_FILE to a
# pre-commit), and from a linked worktree it is an absolute path into the
# shared repository, so a script test's `git -C <temp dir>` would then work on
# that repository instead of its own. Each test runs here with that
# environment naming a sentinel repository, which must come out exactly as it
# went in, and its own summary line is printed as it passes. This is what
# `task test:scripts` runs.
#
# Usage:
#   scripts/hook-environment_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly sentinel="${workdir}/sentinel"
git init -q "${sentinel}"
cp -R "${sentinel}" "${workdir}/before"

for test in "${here}"/*_test.sh "${here}"/release/*_test.sh; do
  if [[ "${test}" == "${here}/hook-environment_test.sh" ]]; then
    continue
  fi

  expect_exit pass "${test#"${here}/"} with a hook's environment" \
    env GIT_DIR="${sentinel}/.git" GIT_INDEX_FILE="${sentinel}/.git/index" "${test}"
  if ((case_status == 0)); then
    tail -n 1 "${case_output}"
  fi
done

expect_exit pass "the repository a hook's environment names is left alone" \
  diff -r "${workdir}/before" "${sentinel}"

finish_tests
