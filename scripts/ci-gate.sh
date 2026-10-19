#!/usr/bin/env bash
# ci-gate.sh — the verdict of CI's ci-gate job: pass only when every job it
# needs succeeded, but for the one skip it allows. Commit messages may skip,
# being pull-request-only by design. Any other skip is an `if:` added or
# mistyped on a job that then never ran, and fails as a failure or a
# cancellation does.
#
# Usage: RESULTS='<toJSON(needs)>' ci-gate.sh
#
# Prints each job's result. Exits 0 when the gate passes, 1 when it does not.
set -euo pipefail

readonly results="${RESULTS:?RESULTS must hold the needs context as JSON}"

jq -r 'to_entries[] | "  \(.key): \(.value.result)"' <<<"${results}"

blocking="$(jq -r '
  to_entries[]
  | select(.value.result != "success"
      and (.value.result != "skipped" or .key != "commit-lint"))
  | "\(.key): \(.value.result)"' <<<"${results}")"

if [[ -n "${blocking}" ]]; then
  echo "::error::A required job did not succeed:"
  echo "${blocking}"
  exit 1
fi
echo "All required jobs succeeded."
