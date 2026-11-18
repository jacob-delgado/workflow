#!/usr/bin/env bash
#
# Tests for ci-gate.sh: it passes only when every job it is given succeeded,
# but for the skips it allows: Commit messages' on a push, since the job is
# pull-request-only, though not on a pull request or with no event named, and,
# when the change is to the docs alone, the Go, web and end-to-end jobs that
# skip for one. Any other skip, a failure or a cancellation fails it. And it
# holds the script to the jobs ci.yml runs, read from the workflow alone:
# ci-gate needs every job but the three reporting ones, a docs-only pull
# request may skip exactly the jobs whose if: skips them for one, a code
# change may skip none but Commit messages on a push, Commit messages' if: is
# pull-request-only, and no other job ci-gate needs skips on anything else.
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

# The jobs ci.yml runs, read from its jobs: block: each job's id, two spaces
# in, with its one-line if: four spaces in, or none, and ci-gate's needs.
readonly workflow="${here}/../.github/workflows/ci.yml"
readonly docs_skip="needs.changes.outputs.docs != 'true'"
declare -A condition=()
while IFS=$'\t' read -r job job_condition; do
  condition["${job}"]="${job_condition}"
done < <(awk '
  /^jobs:$/ { in_jobs = 1; next }
  in_jobs && /^[^ #]/ { in_jobs = 0 }
  !in_jobs { next }
  /^  [a-z0-9-]+:$/ { if (job != "") print job "\t" cond; job = substr($0, 3, length($0) - 3); cond = ""; next }
  /^    if: / { cond = substr($0, 9) }
  END { if (job != "") print job "\t" cond }
' "${workflow}")
mapfile -t needs < <(awk '
  /^  ci-gate:$/ { gate = 1; next }
  gate && /^  [a-z0-9-]+:$/ { exit }
  gate && /^    needs: \[.*\]$/ { sub(/^    needs: \[/, ""); sub(/\]$/, ""); gsub(/, */, "\n"); print }
' "${workflow}")
if ((${#condition[@]} == 0 || ${#needs[@]} == 0)); then
  count_case
  fail_case "ci.yml's jobs" "read ${#condition[@]} jobs and ${#needs[@]} needs of ci-gate from ${workflow}"
  finish_tests
fi

# skipped_one runs ci-gate.sh as CI would with every job ci-gate needs
# succeeded but one, skipped.
#   skipped_one <job> <docs-only> <event>
skipped_one() {
  verdict "$2" "$(jq -n --arg skip "$1" \
    '$ARGS.positional | map({(.): {result: (if . == $skip then "skipped" else "success" end)}}) | add' \
    --args "${needs[@]}")" "$3"
}

# Act & Assert: ci-gate needs every job but itself and the three reporting
# ones, Test summary, Coverage baseline and Coverage comment.
count_case
gated="$(printf '%s\n' "${!condition[@]}" | grep -vxF -e ci-gate -e summary -e coverage-baseline -e coverage-comment | sort)"
needed="$(printf '%s\n' "${needs[@]}" | sort)"
if [[ "${needed}" != "${gated}" ]]; then
  fail_case "ci-gate's needs" "they differ from ci.yml's gated jobs (gated, needed): $(comm -3 <(echo "${gated}") <(echo "${needed}") | tr -s '\n\t' '  ')"
fi

# Act & Assert: a docs-only pull request may skip exactly the jobs whose if:
# skips them for one.
for job in "${needs[@]}"; do
  want=fail
  if [[ "${condition[${job}]-}" == "${docs_skip}" ]]; then want=pass; fi
  expect_exit "${want}" "${job} skipped for a docs-only pull request" skipped_one "${job}" true pull_request
done

# Act & Assert: a code change may skip nothing, but Commit messages on a push.
for job in "${needs[@]}"; do
  expect_exit fail "${job} skipped for a code pull request" skipped_one "${job}" false pull_request
  want=fail
  if [[ "${job}" == commit-lint ]]; then want=pass; fi
  expect_exit "${want}" "${job} skipped for a code push" skipped_one "${job}" false push
done

# Assert: the one skip ci-gate.sh allows on a push is the one Commit
# messages' if: makes, and no other job it needs skips on anything but a
# docs-only change.
count_case
if [[ "${condition["commit-lint"]-}" != "github.event_name == 'pull_request'" ]]; then
  fail_case "Commit messages' if" "it is not pull-request-only: ${condition["commit-lint"]-none}"
fi
for job in "${needs[@]}"; do
  count_case
  if [[ "${job}" != commit-lint && -n "${condition[${job}]-}" && "${condition[${job}]-}" != "${docs_skip}" ]]; then
    fail_case "${job}'s if" "ci-gate.sh allows no skip for it: ${condition[${job}]}"
  fi
done

finish_tests
