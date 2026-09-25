#!/usr/bin/env bash
#
# Tests for test-summary.sh: its Go row reports the run the gates make — the
# package roots it is given, under the race detector, with the statement
# coverage coverage-gate.sh reads — and it refuses to run without roots rather
# than choose some of its own.
#
# A stub go on PATH logs each call, answers `go test` with a fixed event stream
# and profile, and answers `go tool cover` with a total that shows whether the
# profile it was handed still held cmd/docsgen. A stub corepack fails, so the
# web rows report dashes without running.
#
# Usage:
#   scripts/test-summary_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly summary="${here}/test-summary.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

readonly stub_bin="${workdir}/bin"
mkdir -p "${stub_bin}"

# Two passes, a skip and a failure, and a package-level pass the counts leave out.
cat >"${workdir}/events.json" <<'EOF'
{"Action":"pass","Package":"example/a","Test":"TestOne"}
{"Action":"pass","Package":"example/a","Test":"TestTwo"}
{"Action":"skip","Package":"example/a","Test":"TestThree"}
{"Action":"fail","Package":"example/a","Test":"TestFour"}
{"Action":"pass","Package":"example/a"}
EOF

# One product line and one cmd/docsgen line, which the gate leaves out.
cat >"${workdir}/go.cov" <<'EOF'
mode: atomic
example/internal/a/a.go:3.16,5.2 1 1
example/cmd/docsgen/main.go:3.16,5.2 1 0
EOF

# The two totals the entry measured on a real profile: 85.6% with cmd/docsgen
# still in it, 96.6% once the gate has filtered it out.
cat >"${stub_bin}/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${STUB_DIR}/go.calls"
if [[ "${1:-}" == "-C" ]]; then
  shift 2
fi
case "${1:-}" in
  test)
    for arg in "$@"; do
      if [[ "${arg}" == -coverprofile=* ]]; then
        cp "${STUB_DIR}/go.cov" "${arg#-coverprofile=}"
      fi
    done
    cat "${STUB_DIR}/events.json"
    ;;
  tool)
    for arg in "$@"; do
      if [[ "${arg}" == -func=* ]]; then
        profile="${arg#-func=}"
      fi
    done
    if grep -q '/cmd/docsgen/' "${profile}"; then
      printf 'total:\t(statements)\t85.6%%\n'
    else
      printf 'total:\t(statements)\t96.6%%\n'
    fi
    ;;
esac
EOF

cat >"${stub_bin}/corepack" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
chmod +x "${stub_bin}/go" "${stub_bin}/corepack"

# run_summary runs the report against the stubs, into ${workdir}/report.md, and
# leaves its exit in ${status}.
#   run_summary <go-package-root>...
run_summary() {
  : >"${workdir}/go.calls"
  status=0
  PATH="${stub_bin}:${PATH}" STUB_DIR="${workdir}" GITHUB_STEP_SUMMARY="" \
    "${summary}" "$@" >"${workdir}/report.md" 2>/dev/null || status=$?
}

# fail records a failed case with its reason.
#   fail <name> <reason>
fail() {
  echo "FAIL ${1}: ${2}" >&2
  failures=$((failures + 1))
}

# Act: one report over two roots of the test's own choosing.
run_summary ./alpha/... ./beta/...
test_call="$(grep -E '(^| )test ' "${workdir}/go.calls" || true)"

# Assert: the report succeeds.
cases=$((cases + 1))
if ((status != 0)); then
  fail "report" "want success, got exit ${status}"
fi

# Assert: the Go tests ran under the race detector, as `task test` runs them.
cases=$((cases + 1))
if [[ " ${test_call} " != *" -race "* ]]; then
  fail "race detector" "want -race in the go test call, got: ${test_call}"
fi

# Assert: the Go tests ran over exactly the roots given, not ./...
cases=$((cases + 1))
if [[ "${test_call}" != *" ./alpha/... ./beta/..." ]] || [[ " ${test_call} " == *" ./... "* ]]; then
  fail "package roots" "want the given roots, got: ${test_call}"
fi

# Assert: the Go row counts the tests that passed, skipped and failed.
cases=$((cases + 1))
if ! grep -qF '| Go unit | 2 | 1 | 1 |' "${workdir}/report.md"; then
  fail "Go counts" "want 2 passed, 1 skipped, 1 failed, got: $(grep 'Go unit' "${workdir}/report.md" || true)"
fi

# Assert: the Go coverage is the gate's number, over the profile it filtered.
cases=$((cases + 1))
if ! grep -qF '| Go | 96.6% |' "${workdir}/report.md"; then
  fail "Go coverage" "want the gate's 96.6%, got: $(grep '^| Go |' "${workdir}/report.md" || true)"
fi

# Act: a report given no roots.
run_summary

# Assert: it refuses rather than pick roots of its own.
cases=$((cases + 1))
if ((status == 0)); then
  fail "no package roots" "want a non-zero exit, got success"
fi

if ((failures > 0)); then
  echo "test-summary_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "test-summary_test: ${cases} case(s) passed."
