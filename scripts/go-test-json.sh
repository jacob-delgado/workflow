#!/usr/bin/env bash
# go-test-json.sh — run go test with -json, keeping the event stream in a file
# for test-counts.sh, while the log reads as a plain `go test` would.
#
# Usage: go-test-json.sh <events-file> <go-test-args>...
#
# -json turns on every verbose line, so printing its output as it comes would
# bury the log. This prints each package's own result line and any build error
# as they arrive, then the whole output of every test that failed, and exits
# with go test's own status, so the step still gates on it.
set -uo pipefail

if (($# < 2)); then
  echo "usage: go-test-json.sh <events-file> <go-test-args>..." >&2
  exit 2
fi
readonly events="${1}"
shift

mkdir -p "$(dirname "${events}")"

# Each line is read on its own and a line that is not an event is passed over,
# so one stray line cannot stop the log, or go test, midway.
go test -json "$@" | tee "${events}" | jq -R --unbuffered -rj '
  fromjson? | select((.Action == "output" and .Test == null) or .Action == "build-output") | .Output'
status="${PIPESTATUS[0]}"

jq -R 'fromjson? // empty' "${events}" | jq -rsj '
  map(select(.Test != null))
  | group_by([.Package, .Test])
  | map(select(any(.Action == "fail")))
  | map(map(select(.Action == "output") | .Output) | join(""))
  | if length == 0 then "" else "\nFailed tests:\n\n" + join("\n") end
' 2>/dev/null

exit "${status}"
