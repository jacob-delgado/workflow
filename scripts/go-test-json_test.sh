#!/usr/bin/env bash
#
# Tests for go-test-json.sh: it keeps go test's event stream in a file for
# test-counts.sh, prints what a plain `go test` would — each package's result
# line, a build error, and a failed test's own output — rather than every
# verbose line -json turns on, and exits as go test did.
#
# A stub go on PATH answers `go test` with a fixed event stream and the exit
# status STUB_EXIT names.
#
# Usage:
#   scripts/go-test-json_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly wrapper="${here}/go-test-json.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly stub_bin="${workdir}/bin"
mkdir -p "${stub_bin}"

cat >"${workdir}/events.json" <<'EOF'
{"Action":"start","Package":"example/a"}
go: a line that is not an event
{"Action":"run","Package":"example/a","Test":"TestPasses"}
{"Action":"output","Package":"example/a","Test":"TestPasses","Output":"=== RUN   TestPasses\n"}
{"Action":"output","Package":"example/a","Test":"TestPasses","Output":"--- PASS: TestPasses (0.00s)\n"}
{"Action":"pass","Package":"example/a","Test":"TestPasses"}
{"Action":"run","Package":"example/a","Test":"TestBreaks"}
{"Action":"output","Package":"example/a","Test":"TestBreaks","Output":"=== RUN   TestBreaks\n"}
{"Action":"output","Package":"example/a","Test":"TestBreaks","Output":"    a_test.go:9: got 1, want 2\n"}
{"Action":"output","Package":"example/a","Test":"TestBreaks","Output":"--- FAIL: TestBreaks (0.00s)\n"}
{"Action":"fail","Package":"example/a","Test":"TestBreaks"}
{"Action":"output","Package":"example/a","Output":"FAIL\texample/a\t0.01s\n"}
{"Action":"fail","Package":"example/a"}
{"ImportPath":"example/b","Action":"build-output","Output":"b.go:3:1: syntax error\n"}
{"Action":"output","Package":"example/c","Output":"ok  \texample/c\t0.02s\n"}
{"Action":"pass","Package":"example/c"}
EOF

cat >"${stub_bin}/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >"${STUB_DIR}/go.calls"
cat "${STUB_DIR}/events.json"
exit "${STUB_EXIT:-0}"
EOF
chmod +x "${stub_bin}/go"

# run_wrapper runs the wrapper against the stub, leaving its output in
# ${workdir}/out.txt and its exit in ${status}.
#   run_wrapper <exit-go-gives> <go-test-args>...
run_wrapper() {
  status=0
  PATH="${stub_bin}:${PATH}" STUB_DIR="${workdir}" STUB_EXIT="${1}" \
    "${wrapper}" "${workdir}/events.out" "${@:2}" >"${workdir}/out.txt" 2>&1 || status=$?
}

# Act: a run with a failing test.
run_wrapper 1 -race ./...

# Assert: it exits as go test did.
count_case
if ((status != 1)); then
  fail_case "exit status" "want 1, got ${status}"
fi

# Assert: the events are kept whole for counting, a stray line and all.
count_case
if ! cmp -s "${workdir}/events.json" "${workdir}/events.out"; then
  fail_case "events file" "want the event stream kept as go gave it"
fi

# Assert: the stray line stopped nothing, so the tests still count.
count_case
if [[ "$("${here}/test-counts.sh" go "${workdir}/events.out" "Go unit")" != *'"passed":1,"skipped":0,"failed":1'* ]]; then
  fail_case "counts past a stray line" "got: $("${here}/test-counts.sh" go "${workdir}/events.out" "Go unit")"
fi

# Assert: go test ran with -json and the arguments given.
count_case
if [[ "$(cat "${workdir}/go.calls")" != "test -json -race ./..." ]]; then
  fail_case "arguments" "got: $(cat "${workdir}/go.calls")"
fi

# Assert: package lines, the build error and the failed test's output show.
for want in $'ok  \texample/c' $'FAIL\texample/a' 'b.go:3:1: syntax error' 'a_test.go:9: got 1, want 2' '--- FAIL: TestBreaks'; do
  count_case
  if ! grep -qF -- "${want}" "${workdir}/out.txt"; then
    fail_case "shows ${want}" "got:"$'\n'"$(cat "${workdir}/out.txt")"
  fi
done

# Assert: a passing test's verbose lines do not.
count_case
if grep -qF -- 'TestPasses' "${workdir}/out.txt"; then
  fail_case "quiet passes" "want no line for a passing test, got:"$'\n'"$(cat "${workdir}/out.txt")"
fi

# Act: a run that passes.
run_wrapper 0 ./...

# Assert
count_case
if ((status != 0)); then
  fail_case "passing exit" "want 0, got ${status}"
fi

finish_tests
