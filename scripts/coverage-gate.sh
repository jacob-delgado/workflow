#!/usr/bin/env bash
# Fail if total test coverage is below the floor.
#
# Usage: coverage-gate.sh <coverage-profile> <floor>
#
# Run it from the module's root: `go tool cover` resolves the profile's paths
# against the module, and the gate reads the packages that serve only tests
# from the CLAUDE.md beside its go.mod.
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
readonly guide="CLAUDE.md"

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/layout.sh
source "${here}/lib/layout.sh"

if [[ ! -s "${profile}" ]]; then
  echo "coverage profile not found or empty: ${profile}" >&2
  exit 1
fi

# Two kinds of code leave the denominator, since covering either would not
# measure the product.
#
#   THE GENERATED TREES. They are generated, not written, so covering them
#   would measure the generator, not the code. cmd/docsgen is the
#   command-reference tool (its output is checked by check-docs-drift.sh);
#   internal/api is the oapi-codegen output (checked by `task gen:verify`).
#
#   EVERY PACKAGE MARKED "(tests only)" at the end of its line in CLAUDE.md's
#   layout block. Such a package is a helper only tests import, which depguard
#   holds (the *-only-in-tests rules in .golangci.yml), so no product path runs
#   it, and how much of it the tests it serves happen to run says nothing of
#   the product. The layout is the one list of them, so the gate reads it there
#   rather than keep a second, and a layout it cannot read stops the gate
#   rather than count every package or none.
if [[ ! -f "${guide}" ]]; then
  echo "coverage-gate: no ${guide} here to read the packages that serve only tests from; run it from the module's root." >&2
  exit 1
fi
layout="$(layout_dirs "${guide}")"
if [[ -z "${layout}" ]]; then
  echo "coverage-gate: ${guide} has no layout block (a \`\`\`text block after 'Layout:') to read the packages that serve only tests from." >&2
  exit 1
fi
tests_only="$(layout_tests_only_dirs "${guide}")"

module="$(awk '$1 == "module" { print $2; exit }' go.mod 2>/dev/null || true)"
if [[ -z "${module}" ]]; then
  echo "coverage-gate: no module path in a go.mod here; run it from the module's root." >&2
  exit 1
fi

# A profile line names its file by import path, the module's and then the
# file's directory, so a line is left out when that directory is one the
# layout marks. A directory below it is a package of its own, with a line of
# its own in the layout, and counts unless that line is marked too.
filtered="$(mktemp)"
trap 'rm -f "${filtered}"' EXIT
grep -vE '/cmd/docsgen/|/internal/api/' "${profile}" \
  | awk -v module="${module}" -v marked="${tests_only//$'\n'/ }" '
    BEGIN {
      count = split(marked, dirs, " ")
      for (i = 1; i <= count; i++) skip[module "/" dirs[i]] = 1
    }
    { dir = $1; sub(/\/[^\/]*$/, "", dir) }
    !(dir in skip)
  ' >"${filtered}"

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
