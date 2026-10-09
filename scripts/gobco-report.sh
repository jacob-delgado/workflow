#!/usr/bin/env bash
# Condition coverage for the Go packages, via rillig/gobco.
#
# Usage: gobco-report.sh <floor> <package root>...
#        gobco-report.sh --package <import path> [--package <import path>]... <floor>
#
# The roots are the caller's: `task cover:branch` passes Taskfile.yml's
# GO_PKGS, the one place they are written. Every package under them is
# measured or accounted for. --package measures the packages it names instead,
# and only those.
#
# What this measures that `go test -cover` cannot: Go ships STATEMENT coverage,
# so an `if a && b` counts as covered the moment the line runs. gobco rewrites
# each package and instruments every boolean expression, each operand of an &&
# or || on its own, then reports the conditions never observed BOTH true and
# false — "condition `err != nil` was 8 times false but never true". Each such
# line is one missing test case. That is why it runs without -branch: the flag
# counts branches, one counter per `if a && b`, and skips a boolean that is not
# a branch.
#
# The score is arms observed / arms present: every condition has two arms, and a
# condition seen only one way scores 1 of 2.
#
# Beside the module's floor, two more checks fail the run:
#
#   - AN ARM NO TEST REACHED. A condition never evaluated, and an error check
#     whose error arm was never seen, are named by file, line and function,
#     and fail unless a line of scripts/gobco-allowlist.txt keeps that arm for
#     a kept trade-off in TECH_DEBT.md. Any other condition seen one way is
#     only listed: that is the worklist.
#   - A PACKAGE BELOW ITS OWN FLOOR. Each package scripts/gobco-floors.txt
#     names answers to the floor there as well as to the module's, since a
#     small package's gaps barely move a share internal/tui dominates.
#
# EVERY PACKAGE WITH TESTS IS MEASURED, and there is no skip list: one without
# tests is named in NO_TESTS below, with the reason it has none. A package
# gobco cannot read is a hard error: a report that quietly dropped a package
# would still print a healthy percentage while measuring less and less of the
# code, which is the one failure mode a coverage gate must not have.
#
# Other sharp edges, each measured rather than assumed:
#
#   1. NEVER PASS -race. gobco's injected counters are plain increments with no
#      synchronization, so a package with t.Parallel() reports a data race and
#      the run dies. The corollary matters for reading the output: concurrent
#      increments are lossy, so a "never false" verdict in a parallel package
#      can be a lost increment. Confirm before writing a test for it.
#
#   2. -vet=off. `go test` would otherwise vet machine-generated instrumented
#      source for no benefit; golangci-lint already vetted the real code.
#
#   3. ONE PACKAGE PER INVOCATION, and gobco LOADS its -stats file before
#      writing it, panicking if the counter count changed. So each package gets
#      its own file and the output directory is wiped first.
#
#   4. IT IGNORES BUILD TAGS, with no flag to change that: it type-checks every
#      file in a directory together, so a package of build-tagged twins — a
#      _unix.go and its !unix half, an embed and its stub — redeclares itself
#      and cannot be read whole. Its single-file mode reads one file alone,
#      and runs the package's tests as the build takes it. So a package whose
#      build leaves a source file out on this platform is measured a file at a
#      time, over each file the build takes, and each of those must stand
#      alone: declare what it uses, not lean on a sibling. One that cannot be
#      read alone fails the run by name, like a package that cannot be read.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root
readonly out_dir="${OUT_DIR:-${repo_root}/tmp/gobco}"

# Packages with no tests, each with the reason it has none. gobco measures
# conditions by running a package's tests, so a package without any cannot be
# measured — but it must be named here rather than silently dropped, or a new
# untested package would shrink what "every package" covers without a trace.
# These three are thin mains: cmd/workflow wires cli.Execute, and cmd/docsgen
# and cmd/testshape are the mains behind internal/ packages that carry the tests.
# internal/api is the oapi-codegen output — generated types and server surface
# with no logic of ours; `task gen:verify` guards it. api is package apispec: a
# single //go:embed of the OpenAPI document, data with no branches to measure.
# internal/seams declares only structs of function fields, with no function body
# and so no condition; the wiring and terminal tests fill and call every field.
# internal/rlimit is a test helper, imported only by tests: the config, editor,
# hooks, proc and wiring tests that lower a resource limit through it run it.
# internal/ptytest is one too: the cli and tui tests that type at a terminal
# open it through it.
readonly NO_TESTS="cmd/workflow cmd/docsgen cmd/testshape internal/api api internal/seams internal/rlimit internal/ptytest"

# gobco type-checks the standard library from SOURCE, with the go/types compiled
# into it: that of the Go that BUILT gobco, not the Go on PATH. One built by Go
# 1.26 cannot parse Go 1.27's math/rand/v2, which declares a generic method, so
# everything reaching it (net/http, Bubble Tea, Cobra) looked unreadable: a
# stale install once put internal/cli and internal/tui on a skip list while the
# gate kept passing, measuring less. A gobco built by any Go but the pinned one
# is refused before it runs.
#
# gobco_binary prints the path of the gobco EXECUTABLE. With mise's shims ahead
# of its install directories on PATH — its recommended setup for editors and
# other non-interactive shells — `command -v` finds a shim, a script that
# `go version -m` cannot read, and the check below died on that instead of
# checking anything. mise knows which binary the shim would run.
gobco_binary() {
  local found
  found="$(command -v gobco)"

  if [[ "${found}" == */mise/shims/* ]] && command -v mise >/dev/null 2>&1; then
    found="$(mise which gobco)"
  fi

  printf '%s' "${found}"
}

require_current_gobco() {
  local gobco_path build_version go_version
  gobco_path="$(gobco_binary)"
  build_version="$(go version -m "${gobco_path}" | awk 'NR==1 {print $2}')"
  go_version="$(go env GOVERSION)"

  if [[ "${build_version}" != "${go_version}" ]]; then
    echo "gobco was built by ${build_version}, but this project builds with ${go_version}." >&2
    echo "It type-checks the standard library with the go/types of the Go that built" >&2
    echo "it, so a stale binary reports packages as unreadable and shrinks this gate." >&2
    echo >&2
    echo "Rebuild it:  mise uninstall go:github.com/rillig/gobco && mise install" >&2
    exit 2
  fi
}

usage() {
  echo "usage: gobco-report.sh <floor> <package root>..." >&2
  echo "       gobco-report.sh --package <import path> [--package <import path>]... <floor>" >&2
  exit 2
}

named=""
while [[ "${1:-}" == "--package" ]]; do
  [[ -n "${2:-}" ]] || usage
  named="${named} $2"
  shift 2
done

# Required, not defaulted: a floor of 0 would pass whatever the measurement, so
# a caller that forgot to pass one should fail here rather than measure nothing.
(($# > 0)) || usage
readonly floor="$1"
shift

# Roots, or packages by name, never both and never neither: a report that chose
# roots of its own could measure less than the gate means without a trace.
if [[ -n "${named}" && $# -gt 0 ]] || [[ -z "${named}" && $# -eq 0 ]]; then
  usage
fi

readonly ratchet_slack=2
# GOBCO_ALLOWLIST and GOBCO_FLOORS are seams for the test harness, so it can
# hand the report lines of its own. Unset, the only way it runs in anger, the
# report reads the files committed beside it.
readonly allowlist_file="${GOBCO_ALLOWLIST:-${repo_root}/scripts/gobco-allowlist.txt}"
readonly floors_file="${GOBCO_FLOORS:-${repo_root}/scripts/gobco-floors.txt}"

# A run over the module's roots answers for every line of the allowlist and
# the floors file; one over named packages, only for what it measured.
whole_module=1
if [[ -n "${named}" ]]; then
  whole_module=0
fi
readonly whole_module

if ! command -v gobco >/dev/null 2>&1; then
  echo "gobco not on PATH — 'mise install' provisions it (pinned in mise.toml)" >&2
  exit 2
fi

require_current_gobco

cd "${repo_root}"

module="$(go list -m)"

if [[ -n "${named}" ]]; then
  packages="${named}"
else
  # Every package under the roots is accounted for: one with tests is measured,
  # one without must be named in NO_TESTS with its reason. A new package that has
  # neither tests nor an entry fails here rather than quietly leaving the total.
  packages="$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' "$@")"
  untested="$(go list -f '{{if not (or .TestGoFiles .XTestGoFiles)}}{{.ImportPath}}{{end}}' "$@")"

  unaccounted=""
  for package in ${untested}; do
    rel="${package#"${module}"/}"
    if [[ " ${NO_TESTS} " != *" ${rel} "* ]]; then
      unaccounted="${unaccounted} ${rel}"
    fi
  done

  if [[ -n "${unaccounted}" ]]; then
    echo "packages with no tests and not in NO_TESTS:${unaccounted}" >&2
    echo "gobco measures a package by running its tests, so one with none cannot be" >&2
    echo "measured and would drop out of the total unseen. Add tests, or add it to" >&2
    echo "NO_TESTS with the reason it has none." >&2
    exit 1
  fi
fi

if [[ -z "${packages}" ]]; then
  echo "No packages with tests; nothing to measure."
  exit 0
fi

rm -rf "${out_dir}"
mkdir -p "${out_dir}"

echo "Condition coverage (gobco, no -race):"
echo

unexpected=""

# twin_files prints the source files the build takes for a package, when it
# leaves one out on this platform (point 4 above); nothing otherwise.
#   twin_files <package dir>
twin_files() {
  local ignored
  for ignored in $(go list -f '{{join .IgnoredGoFiles " "}}' "./$1"); do
    if [[ "${ignored}" != *_test.go ]]; then
      go list -f '{{join .GoFiles " "}}' "./$1"
      return
    fi
  done
}

for package in ${packages}; do
  rel="${package#"${module}"/}"
  slug="${rel//\//_}"
  files="$(twin_files "${rel}")"

  # gobco's per-condition output is the worklist — print it, since a percentage
  # alone tells nobody which test to write next. Trade-off TRADE-19: a boolean
  # it lists as seen one way stays the worklist here, not an entry in
  # TECH_DEBT.md; an arm no test reached fails below.
  if [[ -z "${files}" ]]; then
    gobco -stats "${out_dir}/${slug}.json" -test=-vet=off "./${rel}" 2>&1 || unexpected="${unexpected} ${rel}"
    continue
  fi

  # Each file's statistics apart, since gobco checks a -stats file it reuses
  # (point 3), then merged as the package's once every file was read.
  read_alone="${unexpected}"
  mkdir -p "${out_dir}/files/${slug}"
  for file in ${files}; do
    gobco -stats "${out_dir}/files/${slug}/${file%.go}.json" -test=-vet=off "./${rel}/${file}" 2>&1 \
      || unexpected="${unexpected} ${rel}/${file}"
  done

  if [[ "${unexpected}" == "${read_alone}" ]]; then
    jq -s 'add' "${out_dir}/files/${slug}"/*.json >"${out_dir}/${slug}.json"
  fi
done

if [[ -n "${unexpected}" ]]; then
  echo >&2
  echo "gobco could not read:${unexpected}" >&2
  echo "The measured percentage would silently cover less code than it claims," >&2
  echo "so this fails. Fix the cause: a file of build-tagged twins must stand" >&2
  echo "alone, since gobco reads each one on its own." >&2
  exit 1
fi

echo
printf 'Package arms observed / arms present:\n'

summary="$(
  for stats in "${out_dir}"/*.json; do
    [[ -e "${stats}" ]] || continue
    slug="$(basename "${stats}" .json)"
    jq -r --arg pkg "${slug//_//}" '
      (length * 2) as $total
      | ([.[] | (if .TrueCount > 0 then 1 else 0 end)
               + (if .FalseCount > 0 then 1 else 0 end)] | add // 0) as $hit
      | "\($pkg)\t\($hit)\t\($total)"
    ' "${stats}"
  done | sort
)"

if [[ -z "${summary}" ]]; then
  echo "gobco produced no statistics — treating that as a failure rather than a pass." >&2
  exit 1
fi

total_percent="$(
  echo "${summary}" | awk -F'\t' '
    {
      hit += $2
      arms += $3
      printf "  %-40s %4d / %-4d %6.1f%%\n", $1, $2, $3, ($3 ? 100 * $2 / $3 : 0) > "/dev/stderr"
    }
    END {
      if (arms == 0) exit 1
      printf "  %-40s %4d / %-4d %6.1f%%\n", "TOTAL", hit, arms, 100 * hit / arms > "/dev/stderr"
      printf "%.1f", 100 * hit / arms
    }'
)"

echo
measured_int="${total_percent%%.*}"
failed=0

if [[ "${measured_int}" -lt "${floor}" ]]; then
  printf 'Condition coverage %s%% is below the %s%% floor.\n' "${total_percent}" "${floor}" >&2
  printf 'Each condition listed above was never seen both ways — that is the worklist.\n' >&2
  failed=1
else
  printf 'Condition coverage %s%% (floor %s%%).\n' "${total_percent}" "${floor}"
fi

# Each package the floors file names answers to a floor of its own: one entry a
# line, "<package dir> <floor>", under a # comment saying why. A floor for a
# package the whole module's run did not measure fails, since it holds nothing.
ratchets="$(
  awk -F'\t' -v floors="${floors_file}" -v whole="${whole_module}" -v slack="${ratchet_slack}" '
    function problem(text) {
      printf "%s:%d: %s\n", floors, number, text > "/dev/stderr"
      bad = 1
    }
    { hit[$1] = $2; arms[$1] = $3 }
    END {
      while ((getline line < floors) > 0) {
        number++
        if (line ~ /^[ \t]*#/) { explained = 1; continue }
        if (line ~ /^[ \t]*$/) { explained = 0; continue }
        fields = split(line, field, /[ \t]+/)
        if (fields != 2 || field[2] !~ /^[0-9]+$/ || field[2] + 0 > 100) {
          problem("want <package dir> <floor>, the floor a whole percentage")
        } else if (!explained) {
          problem(field[1] " has no # comment above it saying why")
        } else if (!(field[1] in arms)) {
          if (whole) problem(field[1] " was not measured; a floor for it holds nothing")
        } else if (hit[field[1]] * 100 < field[2] * arms[field[1]]) {
          printf "%s'"'"'s condition coverage %.1f%% is below its %d%% floor.\n", field[1], 100 * hit[field[1]] / arms[field[1]], field[2] > "/dev/stderr"
          bad = 1
        } else if (int(100 * hit[field[1]] / arms[field[1]]) - slack > field[2] + 0) {
          printf "Ratchet available: raise %s'"'"'s floor in %s to %d.\n", field[1], floors, int(100 * hit[field[1]] / arms[field[1]]) - slack
        }
        explained = 0
      }
      exit bad
    }' <<<"${summary}"
)" || failed=1

# Every condition's position, code and counts, a line each, its code on one
# line, for the check of the arms no test reached.
conditions="$(
  jq -r '.[] | [(.Start // ""), ((.Code // "") | gsub("\\s+"; " ")), (.TrueCount // 0), (.FalseCount // 0)]
    | map(tostring) | join("\t")' "${out_dir}"/*.json
)"

# An arm no test reached fails: a condition never evaluated, and an error
# check whose error arm was never seen. An error check compares with nil a
# name ending in err or Err, perhaps with a capitalized or numbered rest
# (err2, errParse, closeErr), or a call of a method so named (ctx.Err()),
# either reached through fields and calls. Each is named by its file, line
# and function. A line of the allowlist keeps one such arm for the trade-off
# it names, on the systems it names ("any", or GOOS values joined by commas):
#
#   <file> <function> <TRADE-n> <systems> <condition>
#
# and a line whose arm is no longer unseen fails too, so the list only shrinks
# as tests arrive. A run over named packages misses only lines for files it
# did not measure.
awk -F'\t' -v allowlist="${allowlist_file}" -v whole="${whole_module}" -v goos="$(go env GOOS)" '
  function enclosing(file, line,    text, number, name) {
    name = "(no function)"
    while (number < line && (getline text < file) > 0) {
      number++
      if (text ~ /^func /) name = function_name(text)
      else if (text ~ /^(var|const|type|import)[ (]/) name = "(no function)"
    }
    close(file)
    return name
  }
  function function_name(text,    receiver, at) {
    sub(/^func /, "", text)
    if (substr(text, 1, 1) != "(") return name_of(text)
    at = index(text, ")")
    receiver = substr(text, 2, at - 2)
    text = substr(text, at + 1)
    sub(/^ +/, "", text)
    sub(/.* /, "", receiver)
    gsub(/[*]/, "", receiver)
    sub(/\[.*/, "", receiver)
    return receiver "." name_of(text)
  }
  function name_of(text) {
    match(text, /^[A-Za-z_][A-Za-z0-9_]*/)
    return substr(text, 1, RLENGTH)
  }
  function applies(systems,    each, count, i) {
    if (systems == "any") return 1
    count = split(systems, each, ",")
    for (i = 1; i <= count; i++) if (each[i] == goos) return 1
    return 0
  }
  BEGIN {
    error_check = "^([A-Za-z_][A-Za-z0-9_]*([(][)])?[.])*[A-Za-z0-9_]*[Ee]rr([A-Z0-9][A-Za-z0-9_]*)?([(][)])?"
    while ((getline line < allowlist) > 0) {
      number++
      if (line ~ /^[ \t]*(#|$)/) continue
      fields = split(line, field, /[ \t]+/)
      if (field[1] == "") { for (i = 1; i < fields; i++) field[i] = field[i + 1]; fields-- }
      if (fields < 5 || field[3] !~ /^TRADE-[1-9][0-9]*$/ || field[4] !~ /^(any|[a-z0-9]+(,[a-z0-9]+)*)$/) {
        printf "%s:%d: want <file> <function> <TRADE-n> <systems> <condition>\n", allowlist, number > "/dev/stderr"
        bad = 1
        continue
      }
      if (!applies(field[4])) continue
      condition = field[5]
      for (i = 6; i <= fields; i++) condition = condition " " field[i]
      kept++
      key[kept] = field[1] SUBSEP field[2] SUBSEP condition
      file[kept] = field[1]
      text[kept] = field[1] " " field[2] " " field[3] " " field[4] " " condition
      at[kept] = number
      allowed[key[kept]]++
    }
  }
  {
    path = $1
    sub(/:[0-9]+:[0-9]+$/, "", path)
    measured[path] = 1
    if ($3 == 0 && $4 == 0) why = "was never evaluated"
    else if ($2 ~ (error_check " != nil$") && $3 == 0) why = "was never seen true, the error'"'"'s arm"
    else if ($2 ~ (error_check " == nil$") && $4 == 0) why = "was never seen false, the error'"'"'s arm"
    else next
    split($1, position, ":")
    function_at = enclosing(path, position[2] + 0)
    arm = path SUBSEP function_at SUBSEP $2
    unseen[arm]++
    if (unseen[arm] <= allowed[arm]) next
    missed[++misses] = sprintf("  %s in %s: \"%s\" %s", $1, function_at, $2, why)
  }
  END {
    for (i = 1; i <= kept; i++) {
      if (!whole && !(file[i] in measured)) continue
      if (left[key[i]]++ < unseen[key[i]]) continue
      printf "%s:%d: no unseen arm is left for \"%s\"; remove the line\n", allowlist, at[i], text[i] > "/dev/stderr"
      bad = 1
    }
    if (misses > 0) {
      print "Condition arms no test reached, and no trade-off keeps:" > "/dev/stderr"
      for (i = 1; i <= misses; i++) print missed[i] > "/dev/stderr"
      print "Each is a test to write. One a kept trade-off in TECH_DEBT.md keeps goes in" > "/dev/stderr"
      print "the allowlist instead, a line an arm, naming that trade-off." > "/dev/stderr"
      bad = 1
    }
    exit bad
  }' <<<"${conditions}" || failed=1

if ((failed)); then
  exit 1
fi

suggested=$((measured_int - ratchet_slack))
if [[ "${suggested}" -gt "${floor}" ]]; then
  printf 'Ratchet available: raise BRANCH_COVERAGE_MIN in Taskfile.yml to %s.\n' "${suggested}"
fi

if [[ -n "${ratchets}" ]]; then
  printf '%s\n' "${ratchets}"
fi
