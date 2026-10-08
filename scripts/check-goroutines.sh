#!/usr/bin/env bash
#
# Fail on a goroutine started outside internal/proc and the test files.
#
# Usage:
#   scripts/check-goroutines.sh          # the gate
#   scripts/check-goroutines.sh --list   # every goroutine start it can see
#
# Concurrency in this program lives in two places by design: Bubble Tea runs the
# model's work through tea.Cmd, and internal/proc drains a subprocess's pipe on
# one goroutine. An ad-hoc `go` in the app logic is how a TUI grows races it
# cannot reproduce. The audit checked this by reading; this keeps it true,
# confining a bare `go` statement to internal/proc, where the pipe drainer lives,
# and to the test files that spawn helpers of their own. It measures the `go`
# keyword and a `.Go(` call, the way sync.WaitGroup and errgroup start one: a
# goroutine a library starts for the program, such as the one context.AfterFunc
# runs the web server's shutdown on, is neither.
#
#   TRACKED FILES ONLY, like the file-length gate: `git ls-files` measures what
#   is committed or staged, so scratch files cannot fail it and a new file counts
#   the moment it is `git add`ed.
set -euo pipefail

mode="gate"
case "${1:-}" in
  "") ;;
  --list) mode="list" ;;
  *)
    sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
    exit 2
    ;;
esac

# Capture the whole Go file list up front: a git that cannot answer must stop the
# gate, not let its failure escape set -e and leave the search measuring nothing.
if ! all_go="$(git ls-files '*.go')"; then
  echo "check-goroutines: could not list tracked files with git." >&2
  exit 2
fi

if [[ -z "${all_go}" ]]; then
  echo "check-goroutines: git listed no Go files to search — refusing to pass having searched nothing." >&2
  exit 2
fi

# What is left after dropping the two places a goroutine is allowed: the test
# files and internal/proc. An empty list here is a real answer — every Go file is
# one of those — not a broken git, so it passes rather than refusing.
tracked="$(printf '%s\n' "${all_go}" | grep -vE '_test\.go$' | grep -v '^internal/proc/' || true)"

# A goroutine statement is `go` at the start of a line, after only indentation,
# followed by a space and a call or func literal. A comment (// go …) begins with
# a slash, and an identifier like gofmt has no space after go, so neither matches.
# A `.Go(` call is sync.WaitGroup's or errgroup's way of starting one.
readonly pattern='^[[:space:]]*go[[:space:]]+[a-zA-Z_(]|\.Go\('

# The written exceptions: each names a file and, after a colon, the statement
# that starts its goroutine, with the reason above it. Only that statement in
# that file is excused, so a second goroutine beside it still fails.
readonly exceptions=(
  # eachAtOnce reads the Repositories pane's repositories side by side, at most
  # readsAtOnce at a time and each writing only its own index, because every
  # read runs git and one after another the pane waits on them all.
  'internal/wiring/repositories.go:running.Go(func() {'
)

found=""
if [[ -n "${tracked}" ]]; then
  found="$(printf '%s\n' "${tracked}" | xargs grep -nHE "${pattern}" 2>/dev/null || true)"
fi

# Each match (file:line:text) the exceptions do not name, the text compared with
# its indentation dropped.
matches=""
while IFS= read -r match; do
  [[ -n "${match}" ]] || continue
  text="${match#*:*:}"
  named="${match%%:*}:${text#"${text%%[![:space:]]*}"}"
  excused=""
  for exception in "${exceptions[@]}"; do
    if [[ "${named}" == "${exception}" ]]; then
      excused="yes"
    fi
  done
  if [[ -z "${excused}" ]]; then
    matches+="${match}"$'\n'
  fi
done <<<"${found}"
matches="${matches%$'\n'}"

if [[ "${mode}" == "list" ]]; then
  echo "${found}"
  exit 0
fi

if [[ -n "${matches}" ]]; then
  echo "check-goroutines: a goroutine started outside internal/proc, the tests and the written exceptions:" >&2
  echo "${matches}" >&2
  echo "Run the model's work through tea.Cmd, and a subprocess's through internal/proc." >&2
  exit 1
fi

echo "check-goroutines: no goroutine started outside internal/proc, the tests and the written exceptions."
