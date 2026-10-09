#!/usr/bin/env bash
#
# Tests for gobco-report.sh, run against a stand-in gobco so that no package is
# instrumented: the floor is required, not defaulted, and a measure below it
# fails; a run whose gobco wrote no statistics fails rather than passing on
# nothing; a package gobco cannot read fails the run by name; a package of
# build-tagged twins, which gobco cannot read whole, is measured a file at a
# time, and a file of one it cannot read alone fails the run by name; a gobco
# built by another Go is refused; every package the module lists without tests
# is named in NO_TESTS; and one that is not is refused by name.
#
# The arms no test reached: a condition never evaluated, and an error check's
# error arm never seen, fail the run, naming the file, line and function,
# unless a line of the allowlist keeps that arm for a trade-off; a line keeps
# one arm, on the systems it names; a line whose arm a test now reaches, or
# that is not shaped as one, fails too; and a boolean seen one way is only
# listed.
#
# The stand-in gobco says which package or file it measured, writes
# GOBCO_STUB_STATS to the file after -stats, or nothing when that is empty,
# ignores its other flags, and fails as gobco does on a package it cannot read
# when its package or file is GOBCO_STUB_UNREADABLE. A go
# ahead of the real one answers `go version -m`, which cannot read a script,
# with the Go this project builds with or GO_STUB_BUILT_BY, and adds
# GO_STUB_UNTESTED to the packages listed without tests; everything else
# reaches the real go, so the untested packages checked are the module's own.
# OUT_DIR is always a temporary directory, because the report empties the
# directory it writes to. GOBCO_ALLOWLIST names an empty file unless a case
# writes its own, so the repository's own lines play no part.
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
go_pkgs="$(sed -n 's/^  GO_PKGS: //p' "${here}/../Taskfile.yml")"
readonly go_pkgs

mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gobco" <<'STUB'
#!/usr/bin/env bash
package="${*: -1}"
if [[ -n "${GOBCO_STUB_UNREADABLE:-}" && "${package}" == "${GOBCO_STUB_UNREADABLE}" ]]; then
  echo "gobco: ${package}: cannot load the package" >&2
  exit 1
fi
echo "gobco: measured ${package}"
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

readonly no_lines="${workdir}/no-lines.txt"
: >"${no_lines}"

# expect runs the report with the stand-ins, and compares its exit and output.
#   expect <pass|fail> <name> <stats> <untested> <want in output> [arg...]
expect() {
  expect_output "$1" "$2" "$5" env PATH="${workdir}/bin:${PATH}" REAL_GO="${real_go}" \
    OUT_DIR="${workdir}/out" GOBCO_STUB_STATS="$3" GO_STUB_UNTESTED="$4" \
    GOBCO_ALLOWLIST="${GOBCO_ALLOWLIST:-${no_lines}}" \
    "${report}" "${@:6}"
}

expect fail "no floor argument" "${both_ways}" "" "usage: gobco-report.sh <floor>"
expect fail "a gobco that wrote no statistics" "" "" \
  "gobco produced no statistics" --package "${module}/internal/buildinfo" 50

# The floor is the gate: 50% measured is under a floor of 60.
expect fail "a measure below the floor" "${one_way}" "" \
  "Condition coverage 50.0% is below the 60% floor." --package "${module}/internal/buildinfo" 60

# A package gobco cannot read fails the run by name, since the percentage
# would otherwise cover less than it claims.
GOBCO_STUB_UNREADABLE=./internal/buildinfo expect fail "a package gobco cannot read" \
  "${both_ways}" "" "gobco could not read: internal/buildinfo" --package "${module}/internal/buildinfo" 50

# gobco reads every file in a directory whatever its build tags, so it cannot
# read a package of build-tagged twins whole: that package is measured a file
# at a time, over the files this platform builds, and none is left out.
GOBCO_STUB_UNREADABLE=./internal/filelock expect pass "a package of twins, a file at a time" \
  "${both_ways}" "" "gobco: measured ./internal/filelock/lock_unix.go" \
  --package "${module}/internal/filelock" 50

# Each of those files must stand alone to be read, and one that cannot fails
# the run by name.
GOBCO_STUB_UNREADABLE=./internal/filelock/lock_unix.go expect fail "a twin gobco cannot read alone" \
  "${both_ways}" "" "gobco could not read: internal/filelock/lock_unix.go" \
  --package "${module}/internal/filelock" 50

# gobco reads the standard library with the go/types of the Go that built it,
# so one built by another Go is refused before it measures anything.
GO_STUB_BUILT_BY=go1.0 expect fail "a gobco built by another Go" "${both_ways}" "" \
  "gobco was built by go1.0" --package "${module}/internal/buildinfo" 50

# The whole module, as the gate runs it, over the package roots Taskfile.yml's
# GO_PKGS names: every package listed without tests is one NO_TESTS names, so
# taking a name out of it fails this case.
# shellcheck disable=SC2086 # the roots are a deliberate multi-arg word list
expect pass "every untested package accounted for" "${both_ways}" "" \
  "Condition coverage 100.0% (floor 50%)." 50 ${go_pkgs}
# shellcheck disable=SC2086 # the roots are a deliberate multi-arg word list
expect fail "an untested package NO_TESTS does not name" "${both_ways}" "${module}/internal/untested" \
  "packages with no tests and not in NO_TESTS: internal/untested" 50 ${go_pkgs}

# The roots are the caller's to give, never the report's to guess, and a run
# given packages by name measures those and nothing else.
expect fail "no package roots" "${both_ways}" "" "usage: gobco-report.sh <floor> <package root>..." 50
expect fail "roots and packages both" "${both_ways}" "" "usage: gobco-report.sh <floor> <package root>..." \
  --package "${module}/internal/buildinfo" 50 ./internal/...

# The arms no test reached. A fixture of Go source gives the conditions a
# file to stand in, so the report can name the function each lies in; its
# lines are the ones the statistics below point at.
readonly fixture="${workdir}/fixture/reader.go"
mkdir -p "$(dirname "${fixture}")"
cat >"${fixture}" <<'GO'
package fixture

func (r *reader[T]) read() error {
	err := r.open()
	if err != nil {
		return err
	}

	return r.close()
}

func ready(open bool, err error) bool {
	return open && err == nil
}
GO

# condition writes the statistics of one condition at a line and column of
# the fixture: its code, and how often it was seen true and false.
#   condition <line:col> <code> <true count> <false count>
condition() {
  printf '{"Start":"%s:%s","Code":"%s","TrueCount":%s,"FalseCount":%s}' "${fixture}" "$1" "$2" "$3" "$4"
}

error_arm_unseen="[$(condition 5:5 'err != nil' 0 3),$(condition 13:17 'err == nil' 2 0)]"
never_evaluated="[$(condition 13:9 'open' 0 0)]"
boolean_one_way="[$(condition 13:9 'open' 4 0)]"
seen_both_ways="[$(condition 5:5 'err != nil' 1 3),$(condition 13:17 'err == nil' 2 1)]"
twice_unseen="[$(condition 5:5 'err != nil' 0 3),$(condition 9:9 'err != nil' 0 1)]"
buildinfo="${module}/internal/buildinfo"

# allowlist writes an allowlist holding the given lines, and names it.
#   allowlist <name> [line...]
allowlist() {
  local file="${workdir}/allowlist-$1.txt"
  shift
  printf '%s\n' "# kept arms" "$@" >"${file}"
  printf '%s' "${file}"
}

# An error check whose error arm no test reached fails the run, by file, line
# and function, as does one spelled the other way round.
expect fail "an error arm no test reached" "${error_arm_unseen}" "" \
  "${fixture}:5:5 in reader.read: \"err != nil\" was never seen true, the error's arm" \
  --package "${buildinfo}" 50
expect fail "an error arm spelled == nil that no test reached" "${error_arm_unseen}" "" \
  "${fixture}:13:17 in ready: \"err == nil\" was never seen false, the error's arm" \
  --package "${buildinfo}" 50

# So does a condition no test evaluated at all.
expect fail "a condition never evaluated" "${never_evaluated}" "" \
  "${fixture}:13:9 in ready: \"open\" was never evaluated" --package "${buildinfo}" 50

# A boolean seen one way is the worklist's, not the gate's.
expect pass "a boolean seen one way" "${boolean_one_way}" "" \
  "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50

# A line of the allowlist keeps one arm, of the function and condition it
# names, for the trade-off it names.
kept="$(allowlist kept "${fixture} reader.read TRADE-1 any err != nil" \
  "${fixture} ready TRADE-2 any err == nil")"
GOBCO_ALLOWLIST="${kept}" expect pass "arms the allowlist keeps" "${error_arm_unseen}" "" \
  "Condition coverage" --package "${buildinfo}" 50

one="$(allowlist one "${fixture} reader.read TRADE-1 any err != nil")"
GOBCO_ALLOWLIST="${one}" expect fail "a second arm the one line does not keep" "${twice_unseen}" "" \
  "${fixture}:9:9 in reader.read: \"err != nil\" was never seen true" --package "${buildinfo}" 50

# A line whose arm a test now reaches keeps nothing, and goes.
GOBCO_ALLOWLIST="${kept}" expect fail "a line whose arm a test now reaches" "${seen_both_ways}" "" \
  "${kept}:2: no unseen arm is left for \"${fixture} reader.read TRADE-1 any err != nil\"" \
  --package "${buildinfo}" 50

# A line names the systems it holds on; elsewhere it neither keeps its arm nor
# is missed.
elsewhere="$(allowlist elsewhere "${fixture} reader.read TRADE-1 plan9 err != nil")"
GOBCO_ALLOWLIST="${elsewhere}" expect fail "a line for another system" "${error_arm_unseen}" "" \
  "${fixture}:5:5 in reader.read: \"err != nil\" was never seen true" --package "${buildinfo}" 50
GOBCO_ALLOWLIST="${elsewhere}" expect pass "a line for another system, its arm seen" "${seen_both_ways}" "" \
  "Condition coverage" --package "${buildinfo}" 50

# A line that does not name a trade-off is no line at all.
unnamed="$(allowlist unnamed "${fixture} reader.read any err != nil")"
GOBCO_ALLOWLIST="${unnamed}" expect fail "an allowlist line naming no trade-off" "${seen_both_ways}" "" \
  "${unnamed}:2: want <file> <function> <TRADE-n> <systems> <condition>" --package "${buildinfo}" 50

finish_tests
