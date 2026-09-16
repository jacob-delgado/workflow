#!/usr/bin/env bash
#
# Fail if a tracked source file is longer than the ceiling.
#
# Usage:
#   scripts/check-file-length.sh          # the gate
#   scripts/check-file-length.sh --list   # every file and its length, longest first
#
# CLAUDE.md asks for source files under ~500 lines, on the reasoning that a
# longer file is usually carrying more than one concern. That guidance sat in
# prose no linter reads, so it only ever applied when a reviewer happened to
# notice. This is the mechanical half.
#
# The decisions, each load-bearing:
#
#   TRACKED FILES ONLY. `git ls-files` measures what is committed or staged, so
#   scratch files, vendored trees and build output cannot fail the gate, and a
#   new file starts counting the moment it is `git add`ed.
#
#   GO AND SHELL ONLY. These are the languages whose length says something about
#   design here. Markdown is prose and grows legitimately; YAML and JSON are
#   configuration whose length is dictated by what is being configured.
#
#   TESTS COUNT. A test file's length is a real signal — a 900-line test usually
#   means the unit under test does too much — and exempting tests would leave
#   the largest files in the repo unmeasured.
#
#   NO EXEMPTION LIST. There is nothing to exempt today, and an empty mechanism
#   is speculative generality. Add one when a file genuinely earns it, with the
#   reason written next to the number.
set -euo pipefail

readonly default_max=500

usage() {
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
}

mode="gate"
max="${FILE_LENGTH_MAX:-${default_max}}"

case "${1:-}" in
  "") ;;
  --list) mode="list" ;;
  *)
    usage
    exit 2
    ;;
esac

if ! [[ "${max}" =~ ^[1-9][0-9]*$ ]]; then
  # A non-numeric ceiling would make the arithmetic below compare strings and
  # silently pass everything, which is the one failure mode a gate must not have.
  echo "check-file-length: FILE_LENGTH_MAX must be a positive integer, got '${max}'" >&2
  exit 2
fi

# Tab-delimited: a path containing a space must not re-split into a bogus count.
lengths() {
  local file lines
  while IFS= read -r file; do
    [[ -f "${file}" ]] || continue
    lines="$(wc -l <"${file}" | tr -d '[:space:]')"
    printf '%s\t%s\n' "${lines}" "${file}"
  done < <(git ls-files '*.go' '*.sh') | sort -rn
}

if [[ "${mode}" == "list" ]]; then
  lengths | awk -F'\t' -v max="${max}" \
    '{ printf "%5d  %s%s\n", $1, $2, ($1 > max ? "  <= OVER" : "") }'
  exit 0
fi

over=0
while IFS=$'\t' read -r lines file; do
  if ((lines > max)); then
    echo "${file}: ${lines} lines (ceiling ${max})" >&2
    over=$((over + 1))
  fi
done < <(lengths)

if ((over > 0)); then
  echo >&2
  echo "check-file-length: ${over} file(s) over ${max} lines." >&2
  echo "Split by concern — a file this long is usually holding more than one." >&2
  exit 1
fi

echo "check-file-length: every tracked Go and shell file is within ${max} lines."
