#!/usr/bin/env bash
#
# Fail if a source directory holds more files than its declared budget.
#
# Usage:
#   scripts/check-package-size.sh          # the gate
#   scripts/check-package-size.sh --list   # every directory, count, budget
#
# Cohesion is the rule a human applies; this is the mechanical backstop under
# it. Every number lives in scripts/package-size-budgets.txt next to the reason
# it is that number, so a grouping grows by decision rather than by drift.
#
# The metric decisions, each load-bearing:
#
#   FILES, NEVER LINES. A line budget rewards splitting one 600-line file into
#   two 300s, which is not the problem. (File LENGTH is a separate concern —
#   see CLAUDE.md's ~500-line guidance.)
#
#   PER-DIRECTORY, NEVER RECURSIVE. A Go package IS a directory, and on the
#   frontend a subfolder is the sanctioned remedy — counting recursively would
#   make the fix a no-op.
#
#   `git ls-files`, NEVER `find`. Measures what is committed or staged, so a
#   scratch file does not fail the gate and a new file counts once `git add`ed.
#
#   SOURCE ONLY, WITH ONE EXCEPTION. Unit tests (`*_test.go`, `*.test.ts(x)`)
#   and stylesheets (`*.css`) do not count: the budget is about how many
#   concerns live here, not how many artifacts. They still MOVE with their
#   subject — a component separated from its stylesheet is a worse outcome than
#   any number. `.d.ts` files DO count; they are source a reader navigates.
#
#   INTEGRATION AND END-TO-END TESTS DO COUNT. A `*_integration_test.go` and
#   every file under `web/e2e/` (the Playwright specs and their helpers) count
#   like source, because an e2e surface accretes one scenario file at a time and
#   a hundred of them in one directory is exactly the pile the budget exists to
#   break up into per-surface subfolders. Only the fast unit test beside its
#   unit is free.
#
#   GENERATED CODE DOES NOT COUNT. `*.gen.go` is subtracted everywhere (it sits
#   beside hand-written files in internal/api), and a whole generated tree is
#   marked `exempt` in the budgets file.
#
#   TAB-DELIMITED intermediate output. With a space, a directory name that
#   contains a space re-parses with a count of 0 and silently passes — the one
#   failure mode a gate must never have.
#
# Budgets are ZERO-HEADROOM IN BOTH DIRECTIONS. Over the number fails, and so
# does a budget left sitting ABOVE its directory's count: that is what a
# forgotten post-split ratchet looks like, and it lets a folder quietly regrow
# everything it just shed.

set -euo pipefail

# PACKAGE_SIZE_ROOT and PACKAGE_SIZE_BUDGETS are seams for the test harness, so
# it can point the gate at a temporary repository and budget file. Unset — the
# only way it runs in anger — the gate measures this repository against the
# budgets committed beside it.
repo_root="${PACKAGE_SIZE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
readonly repo_root
readonly budget_file="${PACKAGE_SIZE_BUDGETS:-${repo_root}/scripts/package-size-budgets.txt}"
readonly history_file="scripts/package-size-budget-history.md"

# The standing decision for any directory not listed in the budgets file: no
# grouping grows past this without a written reason. Deliberately NOT restated
# in the budgets file — a number spelled in two places is the drift this gate
# exists to catch.
: "${DEFAULT_MAX_FILES:=12}"

usage() {
  sed -n '3,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
}

mode="gate"
case "${1:-}" in
  "") ;;
  --list) mode="list" ;;
  *)
    usage
    exit 2
    ;;
esac

if ! [[ "${DEFAULT_MAX_FILES}" =~ ^[1-9][0-9]*$ ]]; then
  # A non-numeric value would make awk fall into string comparison below and
  # silently pass everything.
  echo "check-package-size: DEFAULT_MAX_FILES must be a positive integer, got '${DEFAULT_MAX_FILES}'" >&2
  exit 2
fi

if [[ ! -f "${budget_file}" ]]; then
  echo "check-package-size: no budget file at ${budget_file}" >&2
  exit 2
fi

counts="$(
  git -C "${repo_root}" -c core.quotePath=false ls-files -- '*.go' 'web/src' 'web/e2e' \
    | awk -F/ '
      /_test\.go$/ && !/_integration_test\.go$/ { next }  # unit tests do not count; integration tests do
      /\.gen\.go$/                  { next }
      /^web\/src\/.*\.test\.tsx?$/  { next }               # web unit tests do not count; web/e2e specs do
      /\.css$/                      { next }
      {
        if (NF == 1) { d = "." } else { d = $1; for (i = 2; i < NF; i++) d = d "/" $i }
        n[d]++
      }
      END { for (d in n) printf "%s\t%d\n", d, n[d] }
    ' | sort
)"

if [[ -z "${counts}" ]]; then
  echo "check-package-size: no source files found under ${repo_root} — not a git checkout?" >&2
  exit 1
fi

printf '%s\n' "${counts}" | awk \
  -v budget_file="${budget_file}" \
  -v history_file="${history_file}" \
  -v default_max="${DEFAULT_MAX_FILES}" \
  -v mode="${mode}" '
  function err(msg) { printf "::error:: %s\n", msg > "/dev/stderr"; fail = 1 }

  # Pass 1: the budget file. An entry is "<dir> <budget|exempt>", and it MUST
  # carry a # comment on the line directly above it — no blank line between.
  NR == FNR {
    line = $0
    sub(/[ \t]+$/, "", line)
    if (line ~ /^#/) { commented = 1; next }
    if (line == "")  { commented = 0; next }

    if (!commented) {
      err("entry \"" $1 "\" has no reason comment directly above it — say WHY the number is that number, on the line(s) immediately above, no blank line between")
    }
    commented = 0

    if (NF != 2) { err("malformed entry (want \"<dir> <budget|exempt>\"): " line); next }
    if ($1 in budget || $1 in exempt) { err("duplicate entry for " $1); next }
    if ($2 == "exempt") { exempt[$1] = 1; next }
    if ($2 !~ /^[0-9]+$/) { err("budget for " $1 " must be a number or \"exempt\", got \"" $2 "\""); next }
    budget[$1] = $2 + 0
    next
  }

  # Pass 2: the measured directories.
  {
    dir = $1
    n = $2 + 0

    for (e in exempt) {
      if (dir == e || index(dir, e "/") == 1) { exempt_used[e] = 1; next }
    }

    declared = (dir in budget)
    max = declared ? budget[dir] : default_max
    seen[dir] = 1

    if (mode == "list") {
      printf "%-46s %4d / %-4d (%s)\n", dir, n, max, declared ? "declared" : "default"
      next
    }

    if (n > max) {
      err(dir " holds " n " files, budget " max (declared ? " — declared in " budget_file : " — the default") )
      offenders = 1
      next
    }

    # Zero headroom: a declared budget must EQUAL the count it names. Slack is
    # what a forgotten post-split ratchet leaves behind.
    if (declared && n < max) {
      err(dir " is at " n " files but declares a budget of " max " — that is " (max - n) \
          " file(s) of headroom, and budgets here are zero-headroom by design. Ratchet it to " n \
          " (and append a row to " history_file "), or delete the entry if " n \
          " is at or under the default " default_max " — registration is earned by size.")
    }
  }

  END {
    if (mode == "list") exit 0

    for (d in budget) if (!(d in seen)) {
      err("budget entry \"" d "\" names a directory with no counted files — delete the stale entry, or fix the path")
    }
    for (e in exempt) if (!(e in exempt_used)) {
      err("exempt entry \"" e "\" matched nothing — delete it")
    }

    if (offenders) {
      print "" > "/dev/stderr"
      print "A directory is over its file budget. There are TWO legitimate" > "/dev/stderr"
      print "responses. Pick one, and say which in the PR:" > "/dev/stderr"
      print "" > "/dev/stderr"
      print "  1. SPLIT — the grouping carries more than one reason to change." > "/dev/stderr"
      print "     Go:    pull pure logic into a leaf package the shell delegates to." > "/dev/stderr"
      print "     React: give the concern its own subfolder (features/<f>/<concern>/)," > "/dev/stderr"
      print "            or its own sibling feature when nothing about it is the" > "/dev/stderr"
      print "            parent feature — features/genres came out of features/lists" > "/dev/stderr"
      print "            exactly that way." > "/dev/stderr"
      print "     COHESION IS THE TEST, NOT THE NUMBER. A two-file package carved" > "/dev/stderr"
      print "     out to get under the line is a worse outcome than the file that" > "/dev/stderr"
      print "     tripped the gate." > "/dev/stderr"
      print "" > "/dev/stderr"
      print "  2. BUMP — the new file is the same responsibility spelled one" > "/dev/stderr"
      print "     concern wider (another handler on an existing surface, another" > "/dev/stderr"
      print "     store file for a new aggregate). Raise the number in:" > "/dev/stderr"
      print "       scripts/package-size-budgets.txt" > "/dev/stderr"
      print "     rewrite the WHY in the comment directly above the entry, and" > "/dev/stderr"
      print "     append a row to:" > "/dev/stderr"
      print "       " history_file > "/dev/stderr"
      print "     \"The gate was in the way\" is not a reason." > "/dev/stderr"
      print "" > "/dev/stderr"
      print "  Not sure which? Run: scripts/check-package-size.sh --list" > "/dev/stderr"
    }

    exit (fail ? 1 : 0)
  }
' "${budget_file}" -
