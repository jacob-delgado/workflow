#!/usr/bin/env bash
#
# Fail on a `go` statement outside internal/proc and the test files.
#
# Usage:
#   scripts/check-goroutines.sh          # the gate
#   scripts/check-goroutines.sh --list   # every `go` statement it can see
#
# Concurrency in this program lives in two places by design: Bubble Tea runs the
# model's work through tea.Cmd, and internal/proc drains a subprocess's pipe on
# one goroutine. An ad-hoc `go` in the app logic is how a TUI grows races it
# cannot reproduce. The audit checked this by reading; this keeps it true,
# confining a bare `go` statement to internal/proc, where the pipe drainer lives,
# and to the test files that spawn helpers of their own. It measures the keyword
# only: a goroutine a library starts for the program, such as the one
# context.AfterFunc runs the web server's shutdown on, is not a `go` statement.
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
readonly pattern='^[[:space:]]*go[[:space:]]+[a-zA-Z_(]'

matches=""
if [[ -n "${tracked}" ]]; then
  matches="$(printf '%s\n' "${tracked}" | xargs grep -nHE "${pattern}" 2>/dev/null || true)"
fi

if [[ "${mode}" == "list" ]]; then
  echo "${matches}"
  exit 0
fi

if [[ -n "${matches}" ]]; then
  echo "check-goroutines: a \`go\` statement outside internal/proc and the tests:" >&2
  echo "${matches}" >&2
  echo "Run the model's work through tea.Cmd, and a subprocess's through internal/proc." >&2
  exit 1
fi

echo "check-goroutines: no \`go\` statement outside internal/proc and the tests."
