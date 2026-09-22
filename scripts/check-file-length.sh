#!/usr/bin/env bash
#
# Warn past the soft target, fail past the hard ceiling.
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
# Two thresholds, so the guidance nudges without blocking a file that has a
# genuine reason to be long: 500 is the SOFT target — past it the gate warns but
# still passes — and 800 is the HARD ceiling, past which a file is refused. A
# file between the two is a prompt to look, not a build break.
#
# The decisions, each load-bearing:
#
#   TRACKED FILES ONLY. `git ls-files` measures what is committed or staged, so
#   scratch files, vendored trees and build output cannot fail the gate, and a
#   new file starts counting the moment it is `git add`ed.
#
#   GO, SHELL AND TYPESCRIPT. These are the languages whose length says
#   something about design here: the `--web` frontend (.ts and .tsx) is code
#   under the same soft target and hard ceiling as the Go it talks to. Markdown
#   is prose and grows legitimately; YAML and JSON are configuration whose
#   length is dictated by what is being configured.
#
#   GENERATED CODE IS EXEMPT. Machine-written code (Go's .gen.go, and the whole
#   hey-api client under web/src/api/generated, whose files are not all named
#   .gen.ts) is emitted from a source of truth, so its length says nothing about
#   design; `task gen:verify` and `yarn gen:check` guard it instead.
#
#   TESTS COUNT. A test file's length is a real signal — a 900-line test usually
#   means the unit under test does too much — and exempting tests would leave
#   the largest files in the repo unmeasured.
#
#   NO EXEMPTION LIST. There is nothing to exempt today, and an empty mechanism
#   is speculative generality. Add one when a file genuinely earns it, with the
#   reason written next to the number.
set -euo pipefail

readonly default_max=800
readonly default_soft=500

usage() {
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
}

mode="gate"
max="${FILE_LENGTH_MAX:-${default_max}}"
soft="${FILE_LENGTH_SOFT:-${default_soft}}"

case "${1:-}" in
  "") ;;
  --list) mode="list" ;;
  *)
    usage
    exit 2
    ;;
esac

for name in max soft; do
  # A non-numeric threshold would make the arithmetic below compare strings and
  # silently pass everything, which is the one failure mode a gate must not have.
  if ! [[ "${!name}" =~ ^[1-9][0-9]*$ ]]; then
    echo "check-file-length: ${name} must be a positive integer, got '${!name}'" >&2
    exit 2
  fi
done

if ((soft > max)); then
  # A soft target above the hard ceiling would warn on files that already fail,
  # which reads as noise; clamp it so the bands never overlap.
  soft="${max}"
fi

# Capture the file list up front. A `git ls-files` that failed inside a process
# substitution would have its failure escape set -e, leaving the loop below to
# measure nothing and the gate to report success on it — the one thing a gate
# must never do. Assigned to a variable, a git that cannot answer stops it here.
if ! tracked="$(git ls-files '*.go' '*.sh' '*.ts' '*.tsx')"; then
  echo "check-file-length: could not list tracked files with git." >&2
  exit 2
fi

if [[ -z "${tracked}" ]]; then
  echo "check-file-length: git listed no Go, shell or TypeScript files — refusing to pass having measured nothing." >&2
  exit 2
fi

# Generated code is exempt (see the header). Filtered after the empty check
# above, so a broken git still stops the gate.
tracked="$(printf '%s\n' "${tracked}" | grep -v -e '\.gen\.go$' -e '^web/src/api/generated/' || true)"

# Tab-delimited: a path containing a space must not re-split into a bogus count.
lengths() {
  local file lines
  while IFS= read -r file; do
    [[ -f "${file}" ]] || continue
    lines="$(wc -l <"${file}" | tr -d '[:space:]')"
    printf '%s\t%s\n' "${lines}" "${file}"
  done <<<"${tracked}" | sort -rn
}

if [[ "${mode}" == "list" ]]; then
  lengths | awk -F'\t' -v hard="${max}" -v soft="${soft}" \
    '{ tag = ($1 > hard ? "  <= OVER" : ($1 > soft ? "  <= soft" : "")); printf "%5d  %s%s\n", $1, $2, tag }'
  exit 0
fi

over=0
warned=0
while IFS=$'\t' read -r lines file; do
  if ((lines > max)); then
    echo "${file}: ${lines} lines (hard ceiling ${max})" >&2
    over=$((over + 1))
  elif ((lines > soft)); then
    echo "${file}: ${lines} lines (over the ${soft}-line soft target)" >&2
    warned=$((warned + 1))
  fi
done < <(lengths)

if ((over > 0)); then
  echo >&2
  echo "check-file-length: ${over} file(s) over the ${max}-line hard ceiling." >&2
  echo "Split by concern — a file this long is usually holding more than one." >&2
  exit 1
fi

if ((warned > 0)); then
  echo "check-file-length: ${warned} file(s) over the ${soft}-line soft target (warning only), all within the ${max}-line ceiling."
  exit 0
fi

echo "check-file-length: every tracked Go, shell and TypeScript file is within the ${soft}-line soft target."
