#!/usr/bin/env bash
#
# Tests for check-tradeoffs.sh: it passes when every register entry carries its
# ID and its three paragraphs and every site comment names an entry, and fails,
# naming the file and line, on a heading that is not an ID or whose ID is
# malformed, an ID used twice, a missing or empty paragraph, and a site naming
# an ID the register does not hold.
#
# Usage:
#   scripts/check-tradeoffs_test.sh
#
# The site comments are built from parts, so this file never holds one itself
# for the gate to read.
set -euo pipefail

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories this test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-tradeoffs.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# run_gate runs the gate in a directory, keeping its output in <dir>.out, and
# prints pass or fail.
run_gate() {
  local dir="$1"
  if (cd "${dir}" && "${check}") >"${dir}.out" 2>&1; then
    echo "pass"
  else
    echo "fail"
  fi
}

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  local want="$1" name="$2" dir="$3" got
  cases=$((cases + 1))
  got="$(run_gate "${dir}")"

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    sed 's/^/  /' "${dir}.out" >&2
    failures=$((failures + 1))
  fi
}

# expect_named runs the gate in a directory, wants it to fail, and wants its
# output to carry the given text.
#   expect_named <name> <dir> <text>
expect_named() {
  local name="$1" dir="$2" text="$3" got
  cases=$((cases + 1))
  got="$(run_gate "${dir}")"

  if [[ "${got}" != "fail" ]] || ! grep -qF -- "${text}" "${dir}.out"; then
    echo "FAIL ${name}: want a failure naming '${text}', got ${got}:" >&2
    sed 's/^/  /' "${dir}.out" >&2
    failures=$((failures + 1))
  fi
}

# backlog writes a TECH_DEBT.md holding an open entry and a register of two
# complete entries into a fresh repository. TRADE-2's Decided paragraph starts
# on the line after its label. Line numbers matter: the cases below name them.
#   backlog <dir>
backlog() {
  local dir="$1"
  mkdir -p "${dir}"
  git -C "${dir}" init -q
  cat >"${dir}/TECH_DEBT.md" <<'EOF'
# Technical debt

## The web

### DEBT-1 An open entry

Text.

## The trade-off register

Chosen on purpose.

### TRADE-1 One choice

What it is.

**Decided.** 2026-09-24, in #140.

**Cost.** A cost.

**Reopen when.** A trigger fires.

### TRADE-2 Another choice

**Decided.**
2026-09-25, in #144.

**Cost.** Another cost.

**Reopen when.** Another trigger fires.
EOF
  git -C "${dir}" add TECH_DEBT.md
}

# edit rewrites one line of a fixture's TECH_DEBT.md, or deletes it when the
# replacement is the word "delete".
#   edit <dir> <line> <replacement|delete>
edit() {
  local dir="$1" line="$2" replacement="$3"
  awk -v line="${line}" -v replacement="${replacement}" '
    NR == line && replacement == "delete" { next }
    NR == line { print replacement; next }
    { print }
  ' "${dir}/TECH_DEBT.md" >"${dir}/TECH_DEBT.md.new"
  mv "${dir}/TECH_DEBT.md.new" "${dir}/TECH_DEBT.md"
}

# keep_lines cuts a fixture's TECH_DEBT.md down to its first lines.
#   keep_lines <dir> <count>
keep_lines() {
  local dir="$1" count="$2"
  head -n "${count}" "${dir}/TECH_DEBT.md" >"${dir}/TECH_DEBT.md.new"
  mv "${dir}/TECH_DEBT.md.new" "${dir}/TECH_DEBT.md"
}

# site adds a tracked file whose second line is a site comment naming a
# trade-off, in the given comment syntax.
#   site <dir> <path> <comment> <number>
site() {
  local dir="$1" path="$2" comment="$3" number="$4"
  mkdir -p "${dir}/$(dirname "${path}")"
  printf '%s kept as it is\n%s Trade-off TRADE-%s: kept on purpose.\n' \
    "${comment}" "${comment}" "${number}" >"${dir}/${path}"
  git -C "${dir}" add "${path}"
}

# Every entry is complete, and every site names one of them.
complete="${workdir}/complete"
backlog "${complete}"
site "${complete}" internal/tui/spine.go // 1
site "${complete}" web/src/queryClient.ts // 1
site "${complete}" scripts/gate.sh '#' 2
expect pass "a complete register, every site named" "${complete}"

# A register with no entries yet, and no site pointing into it.
empty="${workdir}/empty"
backlog "${empty}"
keep_lines "${empty}" 11
expect pass "an empty register" "${empty}"

# A backlog with no register section holds no entries, and passes with no site.
none="${workdir}/none"
backlog "${none}"
keep_lines "${none}" 7
expect pass "no register section and no site" "${none}"

# A register heading must start with its ID.
unnamed="${workdir}/unnamed"
backlog "${unnamed}"
edit "${unnamed}" 23 "### Another choice"
expect_named "a heading that is not an ID" "${unnamed}" \
  'TECH_DEBT.md:23: "### Another choice" does not start with TRADE-<number>'

# An ID is TRADE- and a number with no leading zero, and nothing after it.
for bad in "TRADE-x" "TRADE-01" "TRADE-1x"; do
  malformed="${workdir}/malformed-${bad}"
  backlog "${malformed}"
  edit "${malformed}" 23 "### ${bad} Another choice"
  expect_named "a heading whose ID is ${bad}" "${malformed}" \
    "TECH_DEBT.md:23: \"### ${bad} Another choice\" does not start with TRADE-<number>"
done

# An ID names one entry.
twice="${workdir}/twice"
backlog "${twice}"
edit "${twice}" 23 "### TRADE-1 Another choice"
expect_named "an ID used twice" "${twice}" \
  "TECH_DEBT.md:23: TRADE-1 is already the ID at line 13"

# An entry that never says what would reopen it.
unreopened="${workdir}/unreopened"
backlog "${unreopened}"
keep_lines "${unreopened}" 29
expect_named "an entry with no Reopen when" "${unreopened}" \
  "TECH_DEBT.md:23: TRADE-2 has no **Reopen when.** paragraph"

# A label with nothing after it is no paragraph at all.
costless="${workdir}/costless"
backlog "${costless}"
edit "${costless}" 19 "**Cost.**"
expect_named "an entry with an empty Cost" "${costless}" \
  "TECH_DEBT.md:13: TRADE-1 has an empty **Cost.** paragraph"

# A site naming an ID the register does not hold.
unregistered="${workdir}/unregistered"
backlog "${unregistered}"
site "${unregistered}" web/src/queryClient.ts // 3
expect_named "a site naming no entry" "${unregistered}" \
  "web/src/queryClient.ts:2: TRADE-3 is not in TECH_DEBT.md's trade-off register"

# An ID heading an entry outside the register section is not a register entry.
outside="${workdir}/outside"
backlog "${outside}"
edit "${outside}" 5 "### TRADE-3 An open entry"
site "${outside}" internal/tui/spine.go // 3
expect_named "a site naming a heading outside the register" "${outside}" \
  "internal/tui/spine.go:2: TRADE-3 is not in TECH_DEBT.md's trade-off register"

# The backlog's own prose may quote a site comment; it is not a site.
quoted="${workdir}/quoted"
backlog "${quoted}"
edit "${quoted}" 11 "$(printf 'Each site reads Trade-off TRADE-%s: its clause.' 9)"
expect pass "the backlog quoting a site comment" "${quoted}"

# A directory that is not a repository: git cannot search it, so the gate
# must refuse.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
expect fail "a directory that is not a repository" "${notrepo}"

if ((failures > 0)); then
  echo "check-tradeoffs_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-tradeoffs_test: ${cases} case(s) passed."
