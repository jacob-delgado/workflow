#!/usr/bin/env bash
#
# Fail when the trade-off register is out of shape, or a code site names a
# trade-off the register does not hold.
#
# Usage:
#   scripts/check-tradeoffs.sh
#
# The register is every `### ` heading under TECH_DEBT.md's
# `## The trade-off register`, up to the next `## ` heading. Each entry's
# heading starts with its ID, TRADE-<number>, and no two entries share one.
# Each entry carries three paragraphs with text in them: **Decided.** (when,
# and in which pull request), **Cost.** and **Reopen when.** (the concrete
# trigger that would put the choice back on the table). A later audit matches
# its findings against the register and drops one that restates an entry, so
# an entry without its trigger could never be reopened.
#
# The code a trade-off keeps carries a comment reading `Trade-off TRADE-n:` and
# a clause, in its file's comment syntax. Every such ID in a tracked file must
# be one the register holds, so an entry cannot be dropped while code still
# points at it. TECH_DEBT.md, which may quote a site comment, and the files
# written by a generator are not read for sites.
#
# A backlog with no register section holds no entries: it passes as long as no
# site names one.
set -euo pipefail

readonly backlog="TECH_DEBT.md"
readonly register_heading="## The trade-off register"

if ! top="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  echo "check-tradeoffs: not inside a git repository, so there is nothing to search." >&2
  exit 2
fi
cd "${top}"

failed=0

# The register's IDs, one per line. Each problem found in the register goes to
# stderr as "<file>:<line>: <problem>" and fails the awk.
ids=""
if [[ -f "${backlog}" ]]; then
  if ! ids="$(awk -v section="${register_heading}" '
    function problem(line, text) {
      printf "%s:%d: %s\n", FILENAME, line, text > "/dev/stderr"
      bad = 1
    }
    function finish(   i) {
      if (id == "") return
      for (i = 1; i <= 3; i++) {
        if (!(labels[i] in seen)) problem(start, id " has no " labels[i] " paragraph")
        else if (!(labels[i] in filled)) problem(start, id " has an empty " labels[i] " paragraph")
      }
      split("", seen)
      split("", filled)
      id = ""
    }
    function label_of(line,   i) {
      for (i = 1; i <= 3; i++) if (index(line, labels[i]) == 1) return labels[i]
      return ""
    }
    BEGIN {
      labels[1] = "**Decided.**"
      labels[2] = "**Cost.**"
      labels[3] = "**Reopen when.**"
    }
    /^## / { finish(); inside = ($0 == section); next }
    !inside { next }
    /^### / {
      finish()
      paragraph = 0
      split($0, words, " ")
      if (words[2] !~ /^TRADE-[1-9][0-9]*$/) {
        problem(FNR, "\"" $0 "\" does not start with TRADE-<number>")
        next
      }
      if (words[2] in first) {
        problem(FNR, words[2] " is already the ID at line " first[words[2]])
        next
      }
      id = words[2]
      start = first[id] = FNR
      print id
      next
    }
    id == "" { next }
    /^[ \t]*$/ { paragraph = 0; open = ""; next }
    {
      text = $0
      if (!paragraph) {
        paragraph = 1
        open = label_of(text)
        if (open != "") {
          seen[open] = 1
          text = substr(text, length(open) + 1)
        }
      }
      if (open != "" && text ~ /[^ \t]/) filled[open] = 1
    }
    END { finish(); exit bad }
  ' "${backlog}")"; then
    failed=1
  fi
fi

# Every site comment in a tracked file, as "<file>:<line>:<match>". git grep
# answers 1 when nothing matches, which is a clean tree, not a failure.
status=0
sites="$(git grep -n -o -E 'Trade-off TRADE-[0-9]+' -- . \
  ":(exclude)${backlog}" ':(exclude)CHANGELOG.md' \
  ':(exclude)*.gen.go' ':(exclude)web/src/api/generated')" || status=$?
if ((status > 1)); then
  echo "check-tradeoffs: git could not search the tracked files." >&2
  exit 2
fi

count=0
if [[ -n "${sites}" ]]; then
  while IFS=: read -r file line match; do
    count=$((count + 1))
    id="${match#Trade-off }"
    if ! grep -qxF -- "${id}" <<<"${ids}"; then
      echo "${file}:${line}: ${id} is not in ${backlog}'s trade-off register" >&2
      failed=1
    fi
  done <<<"${sites}"
fi

if ((failed)); then
  echo "check-tradeoffs: give each entry its ID and its three paragraphs, and point each site at an entry." >&2
  exit 1
fi

entries=0
if [[ -n "${ids}" ]]; then
  entries="$(wc -l <<<"${ids}" | tr -d ' ')"
fi
echo "check-tradeoffs: ${entries} register entries, ${count} site comments, each naming one."
