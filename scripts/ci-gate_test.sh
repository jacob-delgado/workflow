#!/usr/bin/env bash
#
# Tests for ci-gate.sh: it passes only when every job it is given succeeded,
# but for the skips it allows: Commit messages' on a push, since the job is
# pull-request-only, though not on a pull request or with no event named, and,
# when the change is to the docs alone, the Go, web and end-to-end jobs that
# skip for one. Any other skip, a failure or a cancellation fails it.
#
# Usage:
#   scripts/ci-gate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/ci-gate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# verdict runs ci-gate.sh with the job results, the docs-only answer and the
# event given, as CI's ci-gate job runs it.
#   verdict <docs-only> <results-json> <event>
verdict() {
  DOCS_ONLY="$1" RESULTS="$2" GITHUB_EVENT_NAME="$3" "${gate}"
}

readonly all_ok='{"changes":{"result":"success"},"lint":{"result":"success"},"test":{"result":"success"},"commit-lint":{"result":"success"}}'
readonly docs_skips='{"changes":{"result":"success"},"lint":{"result":"success"},"test":{"result":"skipped"},"scripts":{"result":"skipped"},"branch-coverage":{"result":"skipped"},"web":{"result":"skipped"},"e2e":{"result":"skipped"},"e2e-engines":{"result":"skipped"},"e2e-server":{"result":"skipped"},"cross":{"result":"skipped"},"scan":{"result":"success"},"commit-lint":{"result":"success"}}'

# Act & Assert
expect_exit 0 "every job succeeded" verdict false "${all_ok}" push
expect_exit 0 "the code jobs skipped for a docs-only change" verdict true "${docs_skips}" pull_request
expect_exit 0 "commit-lint skipped on a push" verdict false '{"test":{"result":"success"},"commit-lint":{"result":"skipped"}}' push
expect_output fail "commit-lint skipped on a pull request" "commit-lint: skipped" \
  verdict false '{"test":{"result":"success"},"commit-lint":{"result":"skipped"}}' pull_request
expect_output fail "commit-lint skipped with no event" "commit-lint: skipped" \
  verdict false '{"test":{"result":"success"},"commit-lint":{"result":"skipped"}}' ""
expect_output fail "a code job skipped for a code change" "test: skipped" \
  verdict false '{"lint":{"result":"success"},"test":{"result":"skipped"}}' pull_request
expect_exit fail "a code job skipped with no docs answer" verdict "" '{"test":{"result":"skipped"}}' pull_request
expect_exit fail "Lint skipped, which no change allows" verdict true '{"lint":{"result":"skipped"}}' pull_request
expect_exit fail "Scan skipped, which no change allows" verdict true '{"scan":{"result":"skipped"}}' pull_request
expect_exit fail "Changes failed" verdict true '{"changes":{"result":"failure"}}' pull_request
expect_exit fail "a failure on a docs-only change" verdict true '{"lint":{"result":"failure"},"test":{"result":"skipped"}}' pull_request
expect_exit fail "a canceled job" verdict true '{"test":{"result":"cancelled"}}' pull_request

finish_tests
