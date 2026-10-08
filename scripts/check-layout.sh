#!/usr/bin/env bash
#
# Fail unless CLAUDE.md's layout block names every Go package directory, and
# names no directory that holds nothing.
#
# Usage:
#   scripts/check-layout.sh
#
# The layout is how a session finds where a concern lives — where a credential
# is kept, which package talks to which service — so a package left out of it
# is invisible to exactly the reader it was written for. It drifted once: two
# packages, one of them holding the Slack user token, were missing from it.
# This holds both directions. Every directory holding a tracked Go file (but
# one under testdata, which Go never builds) must have a line of its own,
# `internal/messaging/directory/` as well as `internal/messaging/`; and every
# line must name a directory that holds a tracked file, so a package removed
# takes its line with it.
#
#   TRACKED FILES ONLY, like the file-length gate: `git ls-files` measures what
#   is committed or staged, so scratch files cannot fail it and a new package
#   counts the moment it is `git add`ed.
set -euo pipefail

if (($# > 0)); then
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

readonly guide="CLAUDE.md"

# Capture the whole Go file list up front: a git that cannot answer must stop
# the gate, not let its failure escape set -e and leave it comparing nothing.
if ! go_files="$(git ls-files '*.go')"; then
  echo "check-layout: could not list tracked files with git." >&2
  exit 2
fi

if [[ ! -f "${guide}" ]]; then
  echo "check-layout: no ${guide} here to read the layout from." >&2
  exit 2
fi

# The layout is the ```text block that follows the line `Layout:`; each of its
# lines starts with a directory and its trailing slash.
named="$(awk '
  /^Layout:$/ { after = 1; next }
  after && /^```text$/ { inside = 1; next }
  inside && /^```$/ { exit }
  inside && $1 ~ /\/$/ { sub(/\/$/, "", $1); print $1 }
' "${guide}" | sort -u)"

if [[ -z "${named}" ]]; then
  echo "check-layout: ${guide} has no layout block (a \`\`\`text block after 'Layout:') to compare with." >&2
  exit 2
fi

packages="$(printf '%s\n' "${go_files}" | grep -vE '(^|/)testdata/' | sed -n 's|/[^/]*$||p' | sort -u || true)"

missing="$(comm -23 <(printf '%s\n' "${packages}") <(printf '%s\n' "${named}") | sed '/^$/d')"

stale=""
while IFS= read -r dir; do
  if [[ -z "$(git ls-files -- "${dir}" | head -n 1)" ]]; then
    stale+="${dir}"$'\n'
  fi
done <<<"${named}"
stale="${stale%$'\n'}"

# list_dirs prints each directory of a newline-separated list, indented, with
# the trailing slash the layout writes.
#   list_dirs <dirs>
list_dirs() {
  local dir
  while IFS= read -r dir; do
    printf '  %s/\n' "${dir}"
  done <<<"$1"
}

if [[ -n "${missing}" ]]; then
  echo "check-layout: Go packages ${guide}'s layout does not name:" >&2
  list_dirs "${missing}" >&2
  echo "Add a line for each, saying what it holds." >&2
fi

if [[ -n "${stale}" ]]; then
  echo "check-layout: directories ${guide}'s layout names that hold no tracked file:" >&2
  list_dirs "${stale}" >&2
  echo "Remove each line, or move it to where the directory went." >&2
fi

if [[ -n "${missing}" || -n "${stale}" ]]; then
  exit 1
fi

echo "check-layout: ${guide}'s layout names every Go package, and only directories that hold files."
