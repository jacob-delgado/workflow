#!/usr/bin/env bash
# ci-gate.sh — the verdict of CI's ci-gate job: pass only when every job it
# needs succeeded, but for the skips it allows. Commit messages may skip, being
# pull-request-only by design, and so may the Go, web and end-to-end jobs when
# Changes found the pull request changes the docs alone. Any other skip is an
# `if:` added or mistyped on a job that then never ran, and fails as a failure
# or a cancellation does.
#
# Usage: RESULTS='<toJSON(needs)>' DOCS_ONLY='<true|false>' ci-gate.sh
#
# Prints each job's result. Exits 0 when the gate passes, 1 when it does not.
set -euo pipefail

readonly results="${RESULTS:?RESULTS must hold the needs context as JSON}"
readonly docs_only="${DOCS_ONLY:-}"

jq -r 'to_entries[] | "  \(.key): \(.value.result)"' <<<"${results}"

blocking="$(jq -r --arg docs "${docs_only}" '
  ["test", "scripts", "branch-coverage", "web", "e2e", "e2e-engines", "e2e-server", "cross"] as $code
  | to_entries[]
  | select(.value.result != "success"
      and (.value.result != "skipped"
        or ((.key == "commit-lint"
          or ($docs == "true" and (.key | IN($code[])))) | not)))
  | "\(.key): \(.value.result)"' <<<"${results}")"

if [[ -n "${blocking}" ]]; then
  echo "::error::A required job did not succeed:"
  echo "${blocking}"
  exit 1
fi
echo "All required jobs succeeded."
