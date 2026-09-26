#!/usr/bin/env bash
#
# Tests for gobco-report.sh, run against a stand-in gobco so that no package is
# instrumented: the floor is required, not defaulted; a run whose gobco wrote no
# statistics fails rather than passing on nothing; every package the module
# lists without tests is named in NO_TESTS; and one that is not is refused by
# name.
#
# The stand-in gobco writes GOBCO_STUB_STATS to the file after -stats, or
# nothing when that is empty, and ignores its other flags. A go ahead of the
# real one answers `go version -m`, which cannot read a script, with the Go this
# project builds with, and adds GO_STUB_UNTESTED to the packages listed without
# tests; everything else reaches the real go, so the untested packages checked
# are the module's own. OUT_DIR is always a temporary directory, because the
# report empties the directory it writes to.
#
# Usage:
#   scripts/gobco-report_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly report="${here}/gobco-report.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

real_go="$(command -v go)"
readonly real_go
module="$(cd "${here}/.." && "${real_go}" list -m)"
readonly module

mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gobco" <<'STUB'
#!/usr/bin/env bash
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
  printf '%s: %s\n' "$3" "$("${REAL_GO}" env GOVERSION)"
  exit 0
fi
"${REAL_GO}" "$@" || exit
if [[ "$1" == "list" && "$3" == "{{if not"* && -n "${GO_STUB_UNTESTED:-}" ]]; then
  printf '%s\n' "${GO_STUB_UNTESTED}"
fi
STUB
chmod +x "${workdir}/bin/gobco" "${workdir}/bin/go"

# Statistics for one condition seen both ways: two arms of two.
readonly both_ways='[{"TrueCount":1,"FalseCount":1}]'

failures=0
cases=0

# expect runs the report with the stand-ins, and compares its exit and output.
#   expect <ok|fails> <name> <stats> <untested> <want in output> [arg...]
expect() {
  local want_exit="$1" name="$2" stats="$3" untested="$4" want_output="$5"
  local out_dir="${workdir}/out${cases}" output="${workdir}/output${cases}" got_exit="ok"
  shift 5
  cases=$((cases + 1))

  if ! PATH="${workdir}/bin:${PATH}" REAL_GO="${real_go}" OUT_DIR="${out_dir}" \
    GOBCO_STUB_STATS="${stats}" GO_STUB_UNTESTED="${untested}" \
    "${report}" "$@" >"${output}" 2>&1; then
    got_exit="fails"
  fi

  if [[ "${got_exit}" != "${want_exit}" ]] || ! grep -qF -- "${want_output}" "${output}"; then
    echo "FAIL ${name}: want ${want_exit} saying '${want_output}', got ${got_exit}:" >&2
    sed 's/^/    /' "${output}" >&2
    failures=$((failures + 1))
  fi
}

expect fails "no floor argument" "${both_ways}" "" "usage: gobco-report.sh <floor>"
expect fails "a gobco that wrote no statistics" "" "" \
  "gobco produced no statistics" 50 "${module}/internal/buildinfo"

# The whole module, as the gate runs it: every package listed without tests is
# one NO_TESTS names, so taking a name out of it fails this case.
expect ok "every untested package accounted for" "${both_ways}" "" \
  "Condition coverage 100.0% (floor 50%)." 50
expect fails "an untested package NO_TESTS does not name" "${both_ways}" "${module}/internal/untested" \
  "packages with no tests and not in NO_TESTS: internal/untested" 50

if ((failures > 0)); then
  echo "gobco-report_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "gobco-report_test: ${cases} case(s) passed."
