#!/usr/bin/env bash
# Collapse the coverage artifacts into one small JSON summary.
#
# Usage: coverage-summary.sh <coverage-profile> <gobco-stats-dir>
# Emits: {"statements":N,"branch":N}
#
# Both the PR comment and the main baseline it compares against are computed
# through THIS script, so the two sides of the comparison are always derived the
# same way. A second implementation of "what is the number" is how a delta ends
# up reporting a change that never happened.
set -euo pipefail

readonly profile="${1:?usage: coverage-summary.sh <coverage-profile> <gobco-stats-dir>}"
readonly stats_dir="${2:?usage: coverage-summary.sh <coverage-profile> <gobco-stats-dir>}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script_dir

# Take the statement number FROM THE GATE, with a floor of 0 so it never fails.
# The gate excludes non-product code from the denominator; computing the number
# separately here would report a different figure on the pull request than the
# one the build enforces.
statements="$(
  "${script_dir}/coverage-gate.sh" "${profile}" 0 \
    | awk '/^Coverage/ {match($0, /[0-9]+(\.[0-9]+)?%/); print substr($0, RSTART, RLENGTH - 1); exit}'
)"

if [[ -z "${statements}" ]]; then
  echo "could not read a total from ${profile}" >&2
  exit 1
fi

# Branch coverage over every gobco stats file present: arms observed / arms
# present, the same arithmetic gobco-report.sh prints per package.
branch="$(
  jq -s -r '
    (map(length) | add // 0) as $conditions
    | ($conditions * 2) as $arms
    | ([.[][] | (if .TrueCount > 0 then 1 else 0 end)
               + (if .FalseCount > 0 then 1 else 0 end)] | add // 0) as $hit
    | if $arms == 0 then "null" else (100 * $hit / $arms | . * 10 | round / 10 | tostring) end
  ' "${stats_dir}"/*.json 2>/dev/null || echo "null"
)"

jq -n --argjson statements "${statements}" --argjson branch "${branch}" \
  '{statements: $statements, branch: $branch}'
