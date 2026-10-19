#!/usr/bin/env bash
#
# Tests for ci-gate.sh: it passes only when every job it is given succeeded,
# but for the skips it allows: Commit messages', which is pull-request-only,
# and, when the change is to the docs alone, the Go, web and end-to-end jobs
# that skip for one. Any other skip, a failure or a cancellation fails it.
#
# Usage:
#   scripts/ci-gate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/ci-gate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# verdict runs ci-gate.sh with the job results and the docs-only answer given.
#   verdict <docs-only> <results-json>
verdict() {
  DOCS_ONLY="$1" RESULTS="$2" "${gate}"
}

readonly all_ok='{"changes":{"result":"success"},"lint":{"result":"success"},"test":{"result":"success"},"commit-lint":{"result":"success"}}'
readonly docs_skips='{"changes":{"result":"success"},"lint":{"result":"success"},"test":{"result":"skipped"},"scripts":{"result":"skipped"},"branch-coverage":{"result":"skipped"},"web":{"result":"skipped"},"e2e":{"result":"skipped"},"e2e-engines":{"result":"skipped"},"e2e-server":{"result":"skipped"},"cross":{"result":"skipped"},"scan":{"result":"success"},"commit-lint":{"result":"success"}}'

# Act & Assert
expect_exit 0 "every job succeeded" verdict false "${all_ok}"
expect_exit 0 "the code jobs skipped for a docs-only change" verdict true "${docs_skips}"
expect_exit 0 "commit-lint skipped on a push" verdict false '{"test":{"result":"success"},"commit-lint":{"result":"skipped"}}'
expect_output fail "a code job skipped for a code change" "test: skipped" \
  verdict false '{"lint":{"result":"success"},"test":{"result":"skipped"}}'
expect_exit fail "a code job skipped with no docs answer" verdict "" '{"test":{"result":"skipped"}}'
expect_exit fail "Lint skipped, which no change allows" verdict true '{"lint":{"result":"skipped"}}'
expect_exit fail "Scan skipped, which no change allows" verdict true '{"scan":{"result":"skipped"}}'
expect_exit fail "Changes failed" verdict true '{"changes":{"result":"failure"}}'
expect_exit fail "a failure on a docs-only change" verdict true '{"lint":{"result":"failure"},"test":{"result":"skipped"}}'
expect_exit fail "a canceled job" verdict true '{"test":{"result":"cancelled"}}'

finish_tests
