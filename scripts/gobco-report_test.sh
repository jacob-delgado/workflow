#!/usr/bin/env bash
#
# Test for gobco-report.sh: the floor is required, not defaulted. A floor of 0
# would pass whatever the measurement, so a caller that forgot to pass one must
# fail here — before gobco runs — rather than measure and green a run on nothing.
#
# The full run needs gobco and instruments every package, which the gate itself
# exercises on every `task cover:branch`; this pins the one failure path a unit
# test can settle deterministically.
#
# Usage:
#   scripts/gobco-report_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly report="${here}/gobco-report.sh"

failures=0
cases=0

# Act & Assert: with no floor argument the script must exit non-zero.
cases=$((cases + 1))
if "${report}" >/dev/null 2>&1; then
  echo "FAIL no floor argument: want fail, got pass" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  echo "gobco-report_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "gobco-report_test: ${cases} case(s) passed."
