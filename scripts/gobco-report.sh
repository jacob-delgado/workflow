#!/usr/bin/env bash
# Condition (branch) coverage for the Go packages, via rillig/gobco.
#
# Usage: gobco-report.sh [floor] [package...]
#
# What this measures that `go test -cover` cannot: Go ships STATEMENT coverage,
# so an `if a && b` counts as covered the moment the line runs. gobco rewrites
# each package and instruments every boolean expression, then reports the
# conditions never observed BOTH true and false — "condition `err != nil` was 8
# times false but never true". Each such line is one missing test case.
#
# The score is arms observed / arms present: every condition has two arms, and a
# condition seen only one way scores 1 of 2.
#
# THE SKIP LIST IS THE HONEST PART, and it is currently EMPTY. Any package named
# in UNANALYZABLE below is one this gate does not measure, with the reason
# written beside it. A package that fails and is NOT on the list is a hard error:
# a report that quietly dropped a package would still print a healthy percentage
# while measuring less and less of the code, which is the one failure mode a
# coverage gate must not have. The corollary is that adding an entry is a real
# decision, never a way to make a red run green.
#
# Other sharp edges, each measured rather than assumed:
#
#   1. NEVER PASS -race. gobco's injected counters are plain increments with no
#      synchronization, so a package with t.Parallel() reports a data race and
#      the run dies. The corollary matters for reading the output: concurrent
#      increments are lossy, so a "never false" verdict in a parallel package
#      can be a lost increment. Confirm before writing a test for it.
#
#   2. -vet=off. `go test` would otherwise vet machine-generated instrumented
#      source for no benefit; golangci-lint already vetted the real code.
#
#   3. ONE PACKAGE PER INVOCATION, and gobco LOADS its -stats file before
#      writing it, panicking if the counter count changed. So each package gets
#      its own file and the output directory is wiped first.
#
#   4. IT IGNORES BUILD TAGS, with no flag to change that. This module has none
#      today. The day a build-tagged file lands — a _windows.go with a twin —
#      gobco will fail on that package, and the fix is to add it to
#      UNANALYZABLE with that reason, not to delete the twin.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root
readonly out_dir="${OUT_DIR:-${repo_root}/tmp/gobco}"

# Packages gobco cannot read, each with the reason it cannot.
#
# EMPTY, and that is the correct state. It previously held internal/cli and
# internal/tui, blamed on gobco dying inside `math/rand/v2` with "method must
# have no type parameters". The symptom was real; the diagnosis was not.
#
# gobco type-checks the standard library from SOURCE, using the go/types that is
# compiled into it — which is the go/types of whichever Go BUILT gobco, not the
# Go on PATH. A gobco built by Go 1.26 cannot parse Go 1.27's math/rand/v2,
# which declares a generic method. Anything reaching it transitively (net/http,
# Bubble Tea, Cobra) then looks unreadable.
#
# The binary here had been built by Go 1.26.6 and left in place when mise.toml
# moved to 1.27.1 — a stale install, not a gobco limitation. Rebuilt under the
# pinned Go, every package in this module reads fine. require_current_gobco
# below now fails loudly on that mismatch, because the failure mode it caused is
# the worst kind: the gate kept passing while quietly measuring less.
readonly UNANALYZABLE=""

# gobco carries the go/types of the Go that built it (see above), so a gobco
# built by an older Go silently shrinks what this gate covers. Refuse to run.
require_current_gobco() {
  local gobco_path build_version go_version
  gobco_path="$(command -v gobco)"
  build_version="$(go version -m "${gobco_path}" | awk 'NR==1 {print $2}')"
  go_version="$(go env GOVERSION)"

  if [[ "${build_version}" != "${go_version}" ]]; then
    echo "gobco was built by ${build_version}, but this project builds with ${go_version}." >&2
    echo "It type-checks the standard library with the go/types of the Go that built" >&2
    echo "it, so a stale binary reports packages as unreadable and shrinks this gate." >&2
    echo >&2
    echo "Rebuild it:  mise uninstall go:github.com/rillig/gobco && mise install" >&2
    exit 2
  fi
}

floor="${1:-0}"
readonly floor
shift || true

readonly ratchet_slack=2

if ! command -v gobco >/dev/null 2>&1; then
  echo "gobco not on PATH — 'mise install' provisions it (pinned in mise.toml)" >&2
  exit 2
fi

require_current_gobco

cd "${repo_root}"

module="$(go list -m)"

if [[ $# -gt 0 ]]; then
  packages="$*"
else
  packages="$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...)"
fi

if [[ -z "${packages}" ]]; then
  echo "No packages with tests; nothing to measure."
  exit 0
fi

rm -rf "${out_dir}"
mkdir -p "${out_dir}"

echo "Condition coverage (gobco, short mode, no -race):"
echo

skipped=""
unexpected=""

for package in ${packages}; do
  rel="${package#"${module}"/}"

  if [[ " ${UNANALYZABLE} " == *" ${rel} "* ]]; then
    skipped="${skipped} ${rel}"
    continue
  fi

  slug="${rel//\//_}"

  # gobco's per-condition output is the worklist — print it, since a percentage
  # alone tells nobody which test to write next.
  if ! gobco -branch -stats "${out_dir}/${slug}.json" -test=-vet=off "./${rel}" 2>&1; then
    unexpected="${unexpected} ${rel}"
  fi
done

if [[ -n "${unexpected}" ]]; then
  echo >&2
  echo "gobco could not read:${unexpected}" >&2
  echo "That package is not in this script's UNANALYZABLE list, so the measured" >&2
  echo "percentage would silently cover less code than it claims. Fix the cause," >&2
  echo "or add the package to UNANALYZABLE with the reason it cannot be read." >&2
  exit 1
fi

echo
printf 'Package arms observed / arms present:\n'

summary="$(
  for stats in "${out_dir}"/*.json; do
    [[ -e "${stats}" ]] || continue
    slug="$(basename "${stats}" .json)"
    jq -r --arg pkg "${slug//_//}" '
      (length * 2) as $total
      | ([.[] | (if .TrueCount > 0 then 1 else 0 end)
               + (if .FalseCount > 0 then 1 else 0 end)] | add // 0) as $hit
      | "\($pkg)\t\($hit)\t\($total)"
    ' "${stats}"
  done | sort
)"

if [[ -z "${summary}" ]]; then
  echo "gobco produced no statistics — treating that as a failure rather than a pass." >&2
  exit 1
fi

total_percent="$(
  echo "${summary}" | awk -F'\t' '
    {
      hit += $2
      arms += $3
      printf "  %-40s %4d / %-4d %6.1f%%\n", $1, $2, $3, ($3 ? 100 * $2 / $3 : 0) > "/dev/stderr"
    }
    END {
      if (arms == 0) exit 1
      printf "  %-40s %4d / %-4d %6.1f%%\n", "TOTAL (readable packages)", hit, arms, 100 * hit / arms > "/dev/stderr"
      printf "%.1f", 100 * hit / arms
    }'
)"

echo
if [[ -n "${skipped}" ]]; then
  echo "Not measured — gobco cannot read these (reasons in this script's UNANALYZABLE):"
  for package in ${skipped}; do
    echo "  ${package}"
  done
  echo "Their statement coverage is still gated by scripts/coverage-gate.sh."
  echo
fi

measured_int="${total_percent%%.*}"

if [[ "${measured_int}" -lt "${floor}" ]]; then
  printf 'Branch coverage %s%% is below the %s%% floor.\n' "${total_percent}" "${floor}" >&2
  printf 'Each condition listed above was never seen both ways — that is the worklist.\n' >&2
  exit 1
fi

printf 'Branch coverage %s%% (floor %s%%).\n' "${total_percent}" "${floor}"

suggested=$((measured_int - ratchet_slack))
if [[ "${suggested}" -gt "${floor}" ]]; then
  printf 'Ratchet available: raise BRANCH_COVERAGE_MIN in Taskfile.yml to %s.\n' "${suggested}"
fi
