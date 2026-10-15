# shellcheck shell=bash
#
# Reading the root CLAUDE.md's layout block: the ```text block after the line
# `Layout:`, each of whose lines starts with a directory and its trailing
# slash, then says what that directory holds. check-layout.sh holds the block
# to the Go packages; reading it here keeps one reading of it for every gate
# that does.
#
# Sourced, never run, after the script's own `set -euo pipefail`:
#
#   # shellcheck source=lib/layout.sh
#   source "${here}/lib/layout.sh"
#
# Each function fails when the guide cannot be read, and prints nothing for a
# guide with no layout block; a caller that needs the block refuses an empty
# layout_dirs rather than compare with nothing. What each prints otherwise is
# in its own comment.

# layout_lines prints each line of the guide's layout block that names a
# directory, one a line and whole: the directory without its trailing slash,
# then what the line says it holds, its words joined by single spaces.
#   layout_lines <guide>
layout_lines() {
  awk '
    /^Layout:$/ { after = 1; next }
    after && /^```text$/ { inside = 1; next }
    inside && /^```$/ { exit }
    inside && $1 ~ /\/$/ { sub(/\/$/, "", $1); print }
  ' "$1"
}

# layout_dirs prints every directory the guide's layout block names, one a
# line, without its trailing slash.
#   layout_dirs <guide>
layout_dirs() {
  layout_lines "$1" | awk '{ print $1 }'
}
