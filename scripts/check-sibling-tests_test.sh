#!/usr/bin/env bash
#
# Tests for check-sibling-tests.sh: it passes when every script under scripts/
# has a tracked sibling <name>_test.sh, and fails, naming the script and the
# test it wants, on a shell script, a script in a subdirectory and a script of
# another language that has none, and on a sibling test that is not tracked.
# What is no script, a data file, a test with no script of its own and the
# libraries in scripts/lib/, which their callers' tests drive, needs no test.
# A repository that tracks no script is refused.
#
# Usage:
#   scripts/check-sibling-tests_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-sibling-tests.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# repo makes a repository tracking the given files under scripts/, each empty.
#   repo <name> <file>...
repo() {
  local dir="${workdir}/$1" file
  shift
  git init -q "${dir}"
  for file in "$@"; do
    mkdir -p "$(dirname "${dir}/scripts/${file}")"
    touch "${dir}/scripts/${file}"
  done
  git -C "${dir}" add -A
  printf '%s' "${dir}"
}

# gate runs the check in a repository.
#   gate <dir>
gate() {
  run_in "$1" "${check}"
}

tested="$(repo tested gate.sh gate_test.sh release/tag.sh release/tag_test.sh comment.mjs comment_test.sh \
  budgets.txt README.md hook-environment_test.sh lib/testing.sh lib/arms.awk)"
expect_output pass "every script with its test" "3 script(s)" gate "${tested}"

untested="$(repo untested gate.sh gate_test.sh lonely.sh)"
expect_output fail "a script with no test" "scripts/lonely.sh has no sibling test scripts/lonely_test.sh" \
  gate "${untested}"

nested="$(repo nested release/tag.sh)"
expect_output fail "a script in a subdirectory with no test" \
  "scripts/release/tag.sh has no sibling test scripts/release/tag_test.sh" gate "${nested}"

other_language="$(repo other-language comment.mjs)"
expect_output fail "a script of another language with no test" \
  "scripts/comment.mjs has no sibling test scripts/comment_test.sh" gate "${other_language}"

untracked_test="$(repo untracked-test gate.sh)"
touch "${untracked_test}/scripts/gate_test.sh"
expect_output fail "a sibling test git does not track" "scripts/gate.sh has no sibling test" gate "${untracked_test}"

no_scripts="$(repo no-scripts budgets.txt)"
expect_exit 2 "a repository that tracks no script" gate "${no_scripts}"

finish_tests
