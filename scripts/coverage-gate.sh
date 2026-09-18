#!/usr/bin/env bash
# Fail if total test coverage is below the floor.
#
# Usage: coverage-gate.sh <coverage-profile> <floor>
#
# The floor lives in Taskfile.yml (COVERAGE_MIN), not in prose: a number
# restated in a document drifts from the number the gate enforces. Raise it with
# the ratchet this script prints — floor(measured) - 2 — which is enough slack
# for an incidental refactor and not enough to land a feature untested.
#
# The floor is required, not defaulted: a gate that fell back to some built-in
# number when a caller forgot to pass one would enforce a floor nobody chose.
set -euo pipefail

readonly profile="${1:?usage: coverage-gate.sh <coverage-profile> <floor>}"
readonly floor="${2:?usage: coverage-gate.sh <coverage-profile> <floor>}"
readonly ratchet_slack=2

if [[ ! -s "${profile}" ]]; then
  echo "coverage profile not found or empty: ${profile}" >&2
  exit 1
fi

# cmd/docsgen is a build-time developer tool that regenerates the command
# reference: it is not shipped, and its own output is verified by
# scripts/check-docs-drift.sh on every run — a stronger check than a unit test,
# since it compares against the real command tree. Leaving it in the denominator
# would measure the wrong thing and push toward tests that restate the generator.
filtered="$(mktemp)"
trap 'rm -f "${filtered}"' EXIT
grep -v '/cmd/docsgen/' "${profile}" >"${filtered}"

total="$(go tool cover -func="${filtered}" | awk '/^total:/ {sub(/%/, "", $3); print $3}')"
if [[ -z "${total}" ]]; then
  echo "could not read a total from ${profile}" >&2
  exit 1
fi

# Integer comparison: bash has no floats, and a hundredth of a percent is not a
# meaningful difference in a gate.
measured_int="${total%%.*}"

if [[ "${measured_int}" -lt "${floor}" ]]; then
  printf 'Coverage %s%% is below the %s%% floor.\n' "${total}" "${floor}" >&2
  printf 'Add tests for what you changed — do not lower the floor to fit.\n' >&2
  exit 1
fi

suggested=$((measured_int - ratchet_slack))
printf 'Coverage %s%% (floor %s%%).\n' "${total}" "${floor}"
if [[ "${suggested}" -gt "${floor}" ]]; then
  printf 'Ratchet available: raise COVERAGE_MIN in Taskfile.yml to %s.\n' "${suggested}"
fi
