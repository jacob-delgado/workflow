#!/usr/bin/env bash
#
# Tests for gobco-report.sh, run against a stand-in gobco so that no package is
# instrumented: the floor is required, not defaulted, and a measure below it
# fails; a run whose gobco wrote no statistics fails rather than passing on
# nothing; a package gobco cannot read fails the run unless UNANALYZABLE names
# it, and one it names is skipped and said to be; a gobco built by another Go
# is refused; every package the module lists without tests is named in
# NO_TESTS; and one that is not is refused by name.
#
# The stand-in gobco writes GOBCO_STUB_STATS to the file after -stats, or
# nothing when that is empty, ignores its other flags, and fails as gobco does
# on a package it cannot read when its package is GOBCO_STUB_UNREADABLE. A go
# ahead of the real one answers `go version -m`, which cannot read a script,
# with the Go this project builds with or GO_STUB_BUILT_BY, and adds
# GO_STUB_UNTESTED to the packages listed without tests; everything else
# reaches the real go, so the untested packages checked are the module's own.
# OUT_DIR is always a temporary directory, because the report empties the
# directory it writes to.
#
# Usage:
#   scripts/gobco-report_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly report="${here}/gobco-report.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

real_go="$(command -v go)"
readonly real_go
module="$(cd "${here}/.." && "${real_go}" list -m)"
readonly module

mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gobco" <<'STUB'
#!/usr/bin/env bash
package="${*: -1}"
if [[ -n "${GOBCO_STUB_UNREADABLE:-}" && "${package}" == "${GOBCO_STUB_UNREADABLE}" ]]; then
  echo "gobco: ${package}: cannot load the package" >&2
  exit 1
fi
while (($# > 0)); do
  if [[ "$1" == "-stats" && -n "${GOBCO_STUB_STATS:-}" ]]; then
    printf '%s\n' "${GOBCO_STUB_STATS}" >"$2"
  fi
  shift
done
STUB
cat >"${workdir}/bin/go" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "version" && "$2" == "-m" ]]; then
  printf '%s: %s\n' "$3" "${GO_STUB_BUILT_BY:-$("${REAL_GO}" env GOVERSION)}"
  exit 0
fi
"${REAL_GO}" "$@" || exit
if [[ "$1" == "list" && "$3" == "{{if not"* && -n "${GO_STUB_UNTESTED:-}" ]]; then
  printf '%s\n' "${GO_STUB_UNTESTED}"
fi
STUB
chmod +x "${workdir}/bin/gobco" "${workdir}/bin/go"

# Statistics for one condition seen both ways: two arms of two. And for one
# seen only one way: one arm of two, 50%.
readonly both_ways='[{"TrueCount":1,"FalseCount":1}]'
readonly one_way='[{"TrueCount":1,"FalseCount":0}]'

# expect runs the report with the stand-ins, and compares its exit and output.
#   expect <pass|fail> <name> <stats> <untested> <want in output> [arg...]
expect() {
  expect_output "$1" "$2" "$5" env PATH="${workdir}/bin:${PATH}" REAL_GO="${real_go}" \
    OUT_DIR="${workdir}/out" GOBCO_STUB_STATS="$3" GO_STUB_UNTESTED="$4" \
    "${report}" "${@:6}"
}

expect fail "no floor argument" "${both_ways}" "" "usage: gobco-report.sh <floor>"
expect fail "a gobco that wrote no statistics" "" "" \
  "gobco produced no statistics" 50 "${module}/internal/buildinfo"

# The floor is the gate: 50% measured is under a floor of 60.
expect fail "a measure below the floor" "${one_way}" "" \
  "Condition coverage 50.0% is below the 60% floor." 60 "${module}/internal/buildinfo"

# A package gobco cannot read fails the run by name, since the percentage
# would otherwise cover less than it claims.
GOBCO_STUB_UNREADABLE=./internal/buildinfo expect fail "a package gobco cannot read" \
  "${both_ways}" "" "gobco could not read: internal/buildinfo" 50 "${module}/internal/buildinfo"

# Unless UNANALYZABLE names it: then it is skipped, the rest is measured, and
# the report says which package went unmeasured.
GOBCO_STUB_UNREADABLE=./internal/proc/pgroup expect pass "a package UNANALYZABLE names" \
  "${both_ways}" "" "Not measured — gobco cannot read these (reasons in this script's UNANALYZABLE):
  internal/proc/pgroup" 50 "${module}/internal/proc/pgroup" "${module}/internal/buildinfo"

# gobco reads the standard library with the go/types of the Go that built it,
# so one built by another Go is refused before it measures anything.
GO_STUB_BUILT_BY=go1.0 expect fail "a gobco built by another Go" "${both_ways}" "" \
  "gobco was built by go1.0" 50 "${module}/internal/buildinfo"

# The whole module, as the gate runs it: every package listed without tests is
# one NO_TESTS names, so taking a name out of it fails this case.
expect pass "every untested package accounted for" "${both_ways}" "" \
  "Condition coverage 100.0% (floor 50%)." 50
expect fail "an untested package NO_TESTS does not name" "${both_ways}" "${module}/internal/untested" \
  "packages with no tests and not in NO_TESTS: internal/untested" 50

finish_tests
