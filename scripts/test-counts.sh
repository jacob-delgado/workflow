#!/usr/bin/env bash
# test-counts.sh — read one suite's machine report into how many of its tests
# passed, were skipped and failed, as one line of JSON:
#
#   {"suite":"Go unit","passed":2,"skipped":1,"failed":1}
#
# Usage: test-counts.sh <go|vitest|playwright> <report-file> <suite-label>
#
#   go          a `go test -json` event stream; subtests count, and a package
#               that failed with no test failing (one that did not build) counts
#               as one failure
#   vitest      vitest's json reporter output; a test file that could not load
#               counts as a failure
#   playwright  Playwright's json reporter output; a flaky test, and an error
#               outside any test, count as failed
#
# A report that is missing, unreadable or holds no test at all gives null
# counts rather than zeros, so a suite that never ran does not read as one that
# passed nothing. The CI jobs write one of these per suite for the pull request
# comment, and test-summary.sh reads the same counts for the run summary.
set -uo pipefail

if (($# != 3)); then
  echo "usage: test-counts.sh <go|vitest|playwright> <report-file> <suite-label>" >&2
  exit 2
fi
readonly kind="${1}" report="${2}" suite="${3}"

case "${kind}" in
  go)
    # A package that failed with no test failing in it — its tests did not
    # build, or it failed outside any test — counts as one failure, so a red
    # run never reads as nothing failed.
    # shellcheck disable=SC2016 # jq's own $variables, not the shell's
    filter='[.[] | select(.Test != null and (.Action | IN("pass", "skip", "fail")))] as $tests
      | ($tests | map(select(.Action == "fail") | .Package) | unique) as $told
      | ([.[] | select(.Test == null and .Package != null and .Action == "fail") | .Package]
          | unique | map(select(. as $package | $told | index($package) | not)) | length) as $untold
      | if ($tests | length) == 0 and $untold == 0 then null
        else {
          passed: ($tests | map(select(.Action == "pass")) | length),
          skipped: ($tests | map(select(.Action == "skip")) | length),
          failed: (($tests | map(select(.Action == "fail")) | length) + $untold)
        } end'
    slurp=(-s)
    ;;
  vitest)
    # A test file that failed with no test in it — one that could not be
    # imported — counts as a failure beside the tests that failed.
    filter='{passed: .numPassedTests, skipped: .numPendingTests,
      failed: (.numFailedTests
        + ([.testResults[]? | select(.status == "failed" and ((.assertionResults // []) | length) == 0)]
          | length))}'
    slurp=()
    ;;
  playwright)
    # A run that failed outside any test — its web server never started —
    # reports the reason in errors, each counted as a failure.
    filter='{passed: .stats.expected, skipped: .stats.skipped,
      failed: (.stats.unexpected + .stats.flaky + ((.errors // []) | length))}'
    slurp=()
    ;;
  *)
    echo "test-counts.sh: unknown report kind ${kind}" >&2
    exit 2
    ;;
esac

counts="null"
if [[ -s "${report}" && "${kind}" == "go" ]]; then
  # go test's stream is read a line at a time, passing over any line that is
  # not an event, so one stray line does not lose every count.
  counts="$(jq -R 'fromjson? // empty' "${report}" | jq -c "${slurp[@]}" "${filter}" 2>/dev/null)" || counts="null"
elif [[ -s "${report}" ]]; then
  counts="$(jq -c "${slurp[@]}" "${filter}" "${report}" 2>/dev/null)" || counts="null"
fi

jq -cn --arg suite "${suite}" --argjson counts "${counts:-null}" \
  '{suite: $suite} + ($counts // {passed: null, skipped: null, failed: null})'
