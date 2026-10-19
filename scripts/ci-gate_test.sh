#!/usr/bin/env bash
#
# Tests for ci-gate.sh: it passes only when every job it is given succeeded,
# but for the one skip it allows, Commit messages', which is
# pull-request-only. Any other skip, a failure or a cancellation fails it.
#
# Usage:
#   scripts/ci-gate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/ci-gate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# verdict runs ci-gate.sh with the job results given.
#   verdict <results-json>
verdict() {
  RESULTS="$1" "${gate}"
}

readonly all_ok='{"lint":{"result":"success"},"test":{"result":"success"},"commit-lint":{"result":"success"}}'

# Act & Assert
expect_exit 0 "every job succeeded" verdict "${all_ok}"
expect_exit 0 "commit-lint skipped on a push" verdict '{"test":{"result":"success"},"commit-lint":{"result":"skipped"}}'
expect_output fail "any other job skipped" "test: skipped" \
  verdict '{"lint":{"result":"success"},"test":{"result":"skipped"}}'
expect_exit fail "a failure" verdict '{"lint":{"result":"failure"},"test":{"result":"success"}}'
expect_exit fail "a failed commit-lint" verdict '{"commit-lint":{"result":"failure"}}'
expect_exit fail "a canceled job" verdict '{"test":{"result":"cancelled"}}'

finish_tests
