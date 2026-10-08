#!/usr/bin/env bash
#
# Tests for test-counts.sh: it reads each suite's machine report — go test
# -json events, vitest's json reporter, Playwright's json reporter — into one
# count shape, and reports a suite whose report is missing or unreadable as
# null counts rather than as zeros.
#
# Usage:
#   scripts/test-counts_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly counts="${here}/test-counts.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# Two passes, a skip and a failure, and a package-level pass left out.
cat >"${workdir}/go.json" <<'EOF_GO'
{"Action":"pass","Package":"example/a","Test":"TestOne"}
{"Action":"pass","Package":"example/a","Test":"TestTwo"}
{"Action":"skip","Package":"example/a","Test":"TestThree"}
{"Action":"fail","Package":"example/a","Test":"TestFour"}
{"Action":"pass","Package":"example/a"}
EOF_GO

cat >"${workdir}/vitest.json" <<'EOF_VITEST'
{"numPassedTests":561,"numPendingTests":2,"numFailedTests":1}
EOF_VITEST

cat >"${workdir}/playwright.json" <<'EOF_PW'
{"stats":{"expected":80,"skipped":1,"unexpected":1,"flaky":1}}
EOF_PW

printf 'not json\n' >"${workdir}/broken.json"

# One passing test, and a package whose tests did not build: go reports the
# build failure and a package-level fail, and no test-level fail at all.
cat >"${workdir}/go-build.json" <<'EOF_BUILD'
{"Action":"pass","Package":"example/a","Test":"TestOne"}
{"Action":"pass","Package":"example/a"}
{"ImportPath":"example/b [example/b.test]","Action":"build-output","Output":"b_test.go:3:1: syntax error\n"}
{"ImportPath":"example/b [example/b.test]","Action":"build-fail"}
{"Action":"fail","Package":"example/b","FailedBuild":"example/b [example/b.test]"}
EOF_BUILD

# A test file that could not be imported: no test failed, one file did.
cat >"${workdir}/vitest-import.json" <<'EOF_IMPORT'
{"numPassedTests":10,"numPendingTests":0,"numFailedTests":1,"numFailedTestSuites":2,
 "testResults":[{"status":"failed","assertionResults":[{"status":"failed"}]},
                {"status":"failed","assertionResults":[]}]}
EOF_IMPORT

# A run whose web server never started: every stat zero, the reason in errors.
cat >"${workdir}/playwright-nostart.json" <<'EOF_NOSTART'
{"stats":{"expected":0,"skipped":0,"unexpected":0,"flaky":0},"errors":[{"message":"webServer timed out"}]}
EOF_NOSTART

# expect runs test-counts.sh and compares its output with want.
#   expect <name> <want> <args>...
expect() {
  local name="${1}" want="${2}" got
  shift 2
  count_case
  got="$("${counts}" "$@" 2>/dev/null || echo "exit $?")"
  if [[ "${got}" != "${want}" ]]; then
    fail_case "${name}" "want ${want}, got ${got}"
  fi
}

# Act & Assert
expect "go events" '{"suite":"Go unit","passed":2,"skipped":1,"failed":1}' \
  go "${workdir}/go.json" "Go unit"
expect "vitest report" '{"suite":"Web unit","passed":561,"skipped":2,"failed":1}' \
  vitest "${workdir}/vitest.json" "Web unit"
expect "playwright report counts flaky as failed" '{"suite":"E2E","passed":80,"skipped":1,"failed":2}' \
  playwright "${workdir}/playwright.json" "E2E"
expect "a missing report" '{"suite":"E2E","passed":null,"skipped":null,"failed":null}' \
  playwright "${workdir}/absent.json" "E2E"
expect "an unreadable report" '{"suite":"Web unit","passed":null,"skipped":null,"failed":null}' \
  vitest "${workdir}/broken.json" "Web unit"
expect "a package that did not build counts as failed" '{"suite":"Go unit","passed":1,"skipped":0,"failed":1}' \
  go "${workdir}/go-build.json" "Go unit"
expect "a test file that could not load counts as failed" '{"suite":"Web unit","passed":10,"skipped":0,"failed":2}' \
  vitest "${workdir}/vitest-import.json" "Web unit"
expect "a run that never started counts its errors as failed" '{"suite":"E2E","passed":0,"skipped":0,"failed":1}' \
  playwright "${workdir}/playwright-nostart.json" "E2E"
expect "go events with no test at all" '{"suite":"Go unit","passed":null,"skipped":null,"failed":null}' \
  go "${workdir}/broken.json" "Go unit"

# Act & Assert: an unknown kind of report is refused rather than guessed at.
expect_exit 2 "unknown kind" "${counts}" junit "${workdir}/go.json" "Go unit"

finish_tests
