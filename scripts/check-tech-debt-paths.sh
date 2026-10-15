#!/usr/bin/env bash
#
# Fail when TECH_DEBT.md names, in backticks, a repository path that is not
# tracked.
#
# Usage:
#   scripts/check-tech-debt-paths.sh
#
# An entry points at the code it is about by path, and a package split or a
# rename moves that code without touching the entry; check-tradeoffs.sh holds
# the register's IDs and paragraphs, but not where they point. So every code
# span in TECH_DEBT.md that reads as a repository path must name a tracked
# file, or a directory holding one:
#
#   - a span with a / whose first part is a top-level entry git tracks
#     (internal/tui, scripts/), or whose name ends in a source extension or
#     in one a tracked file has (web/src/app.tsx, web/yarn.lock), named from
#     the repository's root;
#   - a bare file name ending in a source extension (spine.go), the name of
#     some tracked file; a bare name in another extension a tracked file has
#     is more often a file of a package's own (a font, a lock), so it is not
#     read.
#
# A path written from ./ is read from the root, one with a line, a column or an
# anchor after it (spine.go:42, spine.go:42:7, spine.go#L42, guide.md#tests)
# without them, and one padded with a space inside each backtick without the
# spaces, as Markdown reads it. What reads as no repository path is left alone:
# a span holding a space (a command), a glob, a URL, a build tag or a backtick
# (any of * ? [ { : < > $ @ = ~ `), an absolute or home path, a dot-file named
# alone (.workflow.json, a user's file more often than one tracked here), a
# capitalized word ending in .js or .json (Node.js, Response.json), a Go symbol
# with its package path (internal/tui.Model), a path under a domain the
# repository does not track (charm.land/huh/v2/field_input.go), a Go import,
# module or platform path (encoding/json, darwin/arm64) whose first part the
# repository does not track, and a path the tracked .gitignore files ignore
# (web/node_modules, docs/public), which a build or a developer writes. Code
# blocks, fenced or indented, and spans of two or more backticks, which quote
# backticks, are not read.
#
# Exits 0 when every path is tracked, 1 when one is not, and 2 outside a git
# repository, in one that tracks no TECH_DEBT.md, or when no directory can be
# made for the ignore check.
set -euo pipefail

readonly backlog="TECH_DEBT.md"
readonly extensions="go|sh|mjs|js|ts|tsx|css|html|md|json|yml|yaml|toml|txt|mod|sum"

if ! top="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  echo "check-tech-debt-paths: not inside a git repository, so there is nothing to read." >&2
  exit 2
fi
cd "${top}"

if [[ -z "$(git ls-files -- "${backlog}")" ]]; then
  echo "check-tech-debt-paths: git tracks no ${backlog}, so there is nothing to read." >&2
  exit 2
fi

# Every tracked file is read first: each one, the directories above it and its
# name are what a path may name. Then the backlog is read a paragraph at a
# time, outside code blocks, and each code span of a paragraph is classified
# and, when it reads as a repository path, looked up and numbered by the line
# it opens on. A paragraph ends at a blank line, a heading, which is a
# paragraph of its own, a new list item, a fence, a setext underline or a
# thematic break. A fence closes only with a run of its own character at least
# as long and nothing but spaces or tabs after it. A line indented four spaces
# or a tab opens an indented block where no paragraph or list item goes on: at
# the top of the file, or after a blank line, a heading, a setext underline, a
# thematic break or a closing fence, outside a list. A list item starts at a
# marker (-, *, +, 1. or 1)) after any spaces, and a list ends at a line at the
# margin that follows a blank line or is a heading, at a fence at the margin,
# and at a thematic break. It prints "<path><tab><problem>" for each path not
# tracked, then "count <n>" last.
report="$(git ls-files | awk -v backlog="${backlog}" -v extensions="${extensions}" '
  function check(line, span,   path, slashed, first, last) {
    if (span ~ /^ .* $/) span = substr(span, 2, length(span) - 2)
    path = span
    sub(/(:[0-9]+(-[0-9]+|:[0-9]+)?|#[-A-Za-z0-9_]+)$/, "", path)
    if (path ~ /[[:space:]*?[{:<>$@=~`]/ || path ~ /^\//) return
    sub(/^\.\//, "", path)
    slashed = index(path, "/")
    first = substr(path, 1, slashed - 1)
    sub(/\/$/, "", path)
    if (!slashed && (path ~ /^\./ || path ~ /^[A-Z][a-z]+\.(js|json)$/)) return
    if (first ~ /^[^.]+\./ && !(first in tracked)) return
    last = path
    sub(/.*\//, "", last)
    if (path ~ ("\\.(" extensions ")$") || (slashed && last ~ /^[^.].*\./ && extension_of(last) in extension)) {
      count++
    } else if (slashed && first in tracked && last !~ /.\./) {
      count++
    } else {
      return
    }
    if (slashed ? !(path in tracked) : !(path in named)) {
      printf "%s\t%s:%d: %s is no tracked file or directory\n", path, backlog, line, span
    }
  }
  # fence_of says whether a line is a code fence, three or more backticks or
  # tildes after at most three spaces, or after any in a list item, setting
  # run_char, run_length and run_rest, what follows the run. A backtick fence
  # holds no backtick after its run: a line that does opens with a code span
  # instead.
  function fence_of(line,   run) {
    if (line !~ ((listed ? "^ *" : "^ ? ? ?") "(```|~~~)")) return 0
    sub(/^ */, "", line)
    run_char = substr(line, 1, 1)
    for (run = 1; substr(line, run + 1, 1) == run_char; run++) {}
    run_length = run
    run_rest = substr(line, run + 1)
    return !(run_char == "`" && index(run_rest, "`"))
  }
  # add takes a line into the paragraph being read, joined to the one before
  # with a space as Markdown joins them, and notes where it starts.
  function add(line) {
    if (lines) paragraph = paragraph " "
    lines++
    line_at[lines] = length(paragraph) + 1
    line_number[lines] = FNR
    paragraph = paragraph line
  }
  # line_of gives the number of the line an offset of the paragraph is on.
  function line_of(offset,   at) {
    for (at = lines; at > 1 && line_at[at] > offset; at--) {}
    return line_number[at]
  }
  # flush reads the code spans of the paragraph and starts the next. A run of
  # backticks opens a span only when a run as long follows in the paragraph,
  # and only a run as long closes it, so another run inside, or one nothing
  # closes, is text. Only a span of one backtick is checked: a longer one
  # quotes backticks.
  function flush(   rest, offset, opened, wanted, span) {
    rest = paragraph
    offset = 1
    while (match(rest, /`+/)) {
      opened = offset + RSTART - 1
      wanted = RLENGTH
      rest = substr(rest, RSTART + RLENGTH)
      offset = opened + wanted
      if (!closes(rest, wanted)) continue
      span = ""
      while (match(rest, /`+/) && RLENGTH != wanted) {
        span = span substr(rest, 1, RSTART + RLENGTH - 1)
        offset += RSTART + RLENGTH - 1
        rest = substr(rest, RSTART + RLENGTH)
      }
      span = span substr(rest, 1, RSTART - 1)
      offset += RSTART + RLENGTH - 1
      rest = substr(rest, RSTART + RLENGTH)
      if (wanted == 1) check(line_of(opened), span)
    }
    paragraph = ""
    lines = 0
  }
  # closes says whether a run of exactly so many backticks follows in text.
  function closes(text, length_wanted) {
    while (match(text, /`+/)) {
      if (RLENGTH == length_wanted) return 1
      text = substr(text, RSTART + RLENGTH)
    }
    return 0
  }
  function extension_of(name) {
    sub(/.*\./, "", name)
    return name
  }
  NR == FNR {
    tracked[$0] = 1
    parts = split($0, part, "/")
    named[part[parts]] = 1
    if (part[parts] ~ /^[^.].*\./) extension[extension_of(part[parts])] = 1
    path = part[1]
    for (i = 1; i < parts; i++) {
      tracked[path] = 1
      path = path "/" part[i + 1]
    }
    next
  }
  FNR == 1 { blank = 1 }
  fence_char != "" {
    if (fence_of($0) && run_char == fence_char && run_length >= fence_length && run_rest ~ /^[ \t]*$/) {
      fence_char = ""
      blank = 1
    }
    next
  }
  fence_of($0) {
    flush()
    fence_char = run_char
    fence_length = run_length
    if ($0 !~ /^[ \t]/) listed = 0
    next
  }
  /^[ \t]*$/ { flush(); blank = 1; next }
  blank && !listed && /^(    |\t)/ { next }
  !blank && /^ ? ? ?(=+|-+)[ \t]*$/ { flush(); blank = 1; next }
  /^ ? ? ?((\*[ \t]*)(\*[ \t]*)(\*[ \t]*)+|(-[ \t]*)(-[ \t]*)(-[ \t]*)+|(_[ \t]*)(_[ \t]*)(_[ \t]*)+)$/ {
    flush()
    blank = 1
    listed = 0
    next
  }
  {
    heading = ($0 ~ /^ ? ? ?(#|##|###|####|#####|######)([ \t]|$)/)
    item = !heading && $0 ~ /^ *([-*+]|[0-9]+[.)])[ \t]/
    if (heading || item) flush()
    if (item) listed = 1
    else if ($0 !~ /^[ \t]/ && (blank || heading)) listed = 0
    blank = heading
    add($0)
    if (heading) flush()
  }
  END { flush(); print "count", count + 0 }
' - "${backlog}")"

# A path the repository's own .gitignore files ignore names what a build or a
# developer writes, never a file the repository holds, so it is no repository
# path at all. It is asked of an empty repository whose work tree holds only
# the tracked .gitignore files, as the index has them, with no template and no
# global excludes: neither an untracked .gitignore, nor this clone's
# .git/info/exclude, nor a developer's own ignores can hide a stale path that
# CI would fail, and a directory pattern matches though no build made the
# directory. The repository is named with --git-dir, so a hook's exported
# GIT_DIR cannot point it at this one.
count="${report##*count }"
candidates="$(grep -v '^count ' <<<"${report}" || true)"
problems=""
if [[ -n "${candidates}" ]]; then
  if ! ignores="$(mktemp -d)"; then
    echo "check-tech-debt-paths: cannot make a directory for the ignore check." >&2
    exit 2
  fi
  trap 'rm -rf "${ignores}"' EXIT
  git --git-dir="${ignores}/git" init -q --template=
  mkdir "${ignores}/tree"
  while read -r rules; do
    mkdir -p "${ignores}/tree/$(dirname "${rules}")"
    git show ":${rules}" >"${ignores}/tree/${rules}"
  done < <(git ls-files -- .gitignore '*/.gitignore')
  while IFS=$'\t' read -r path problem; do
    if git -C "${ignores}/tree" --git-dir="${ignores}/git" --work-tree=. -c core.excludesFile=/dev/null \
      check-ignore --no-index -- "${path}" "${path}/" >/dev/null 2>&1; then
      count=$((count - 1))
    else
      problems+="${problem}"$'\n'
    fi
  done <<<"${candidates}"
fi

if [[ -n "${problems}" ]]; then
  printf '%s' "${problems}" >&2
  echo "check-tech-debt-paths: point each path at where the code lives now, named from the repository's root; a name that is no file here, such as a branch or an example, goes outside backticks." >&2
  exit 1
fi
echo "check-tech-debt-paths: ${count} repository path(s) in ${backlog}, each tracked."
