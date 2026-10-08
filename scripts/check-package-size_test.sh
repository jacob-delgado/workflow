#!/usr/bin/env bash
#
# Tests for check-package-size.sh: the gate must count the right files, honor a
# ratcheted budget in both directions and a cohesive ceiling in one, give the
# split-or-bump advice when a directory is over, and never pass when it
# measured nothing.
#
# Usage:
#   scripts/check-package-size_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-package-size.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# repo makes a fresh git repository under the work dir and prints its path.
repo() {
  local dir="${workdir}/${1}"
  mkdir -p "${dir}"
  git -C "${dir}" init -q
  echo "${dir}"
}

# add stages a file with a line of content in a repository.
add() {
  local dir="$1" path="$2"
  mkdir -p "$(dirname "${dir}/${path}")"
  printf 'x\n' >"${dir}/${path}"
  git -C "${dir}" add "${path}"
}

# expect runs the gate against a repository and budget file and compares the
# exit status to pass or fail.
#   expect <pass|fail> <name> <repo-dir> <budget-file> [default-max]
expect() {
  expect_exit "$1" "$2" env PACKAGE_SIZE_ROOT="$3" PACKAGE_SIZE_BUDGETS="$4" \
    DEFAULT_MAX_FILES="${5:-12}" "${check}"
}

# An empty budget file is a valid one: every directory then answers to the
# default. Its comment requirement only bites on entries.
empty_budgets="${workdir}/empty-budgets.txt"
printf '# no entries\n' >"${empty_budgets}"

# A directory under the default passes.
under="$(repo under)"
add "${under}" "internal/x/a.go"
add "${under}" "internal/x/b.go"
expect pass "a directory under the default" "${under}" "${empty_budgets}" 12

# A directory over the default fails.
over="$(repo over)"
for i in 1 2 3; do add "${over}" "internal/x/f${i}.go"; done
expect fail "a directory over the default" "${over}" "${empty_budgets}" 2

# Unit tests and generated code do not count; a directory of them stays empty.
free="$(repo free)"
add "${free}" "internal/x/a.go"
add "${free}" "internal/x/a_test.go"
add "${free}" "internal/x/z.gen.go"
expect pass "unit tests and generated files do not count" "${free}" "${empty_budgets}" 1

# An integration test DOES count, so a source file plus one integration test is
# two against a budget of one.
integ="$(repo integ)"
add "${integ}" "internal/x/a.go"
add "${integ}" "internal/x/a_integration_test.go"
expect fail "an integration test counts" "${integ}" "${empty_budgets}" 1

# Everything under web/e2e counts, so a second spec trips a budget of one.
e2e="$(repo e2e)"
add "${e2e}" "web/e2e/one.spec.ts"
add "${e2e}" "web/e2e/two.spec.ts"
expect fail "e2e specs count" "${e2e}" "${empty_budgets}" 1

# A declared budget equal to the count passes.
declared="$(repo declared)"
for i in 1 2 3; do add "${declared}" "internal/big/f${i}.go"; done
equal_budgets="${workdir}/equal-budgets.txt"
printf '# big holds three, by decision\ninternal/big 3\n' >"${equal_budgets}"
expect pass "a declared budget equal to the count" "${declared}" "${equal_budgets}" 2

# A declared budget ABOVE the count fails: zero headroom in both directions.
above_budgets="${workdir}/above-budgets.txt"
printf '# big has room it should not\ninternal/big 5\n' >"${above_budgets}"
expect fail "a declared budget above the count" "${declared}" "${above_budgets}" 2

# A cohesive ceiling only fails past it: a grouping spelled file-per-concern by
# design grows a file at a time without a budget edit for each.
cohesive="$(repo cohesive)"
for i in 1 2 3 4; do add "${cohesive}" "internal/panes/f${i}.go"; done
ceiling_budgets="${workdir}/ceiling-budgets.txt"
printf '# one file per pane, by design\ninternal/panes 6 cohesive\n' >"${ceiling_budgets}"
expect pass "a cohesive directory under its ceiling" "${cohesive}" "${ceiling_budgets}" 2

low_ceiling_budgets="${workdir}/low-ceiling-budgets.txt"
printf '# one file per pane, by design\ninternal/panes 3 cohesive\n' >"${low_ceiling_budgets}"
expect_output fail "a cohesive directory past its ceiling" "There are TWO legitimate" \
  env PACKAGE_SIZE_ROOT="${cohesive}" PACKAGE_SIZE_BUDGETS="${low_ceiling_budgets}" \
  DEFAULT_MAX_FILES=2 "${check}"

# A cohesive entry is still earned by size: one at or under the default is
# refused, so the kind cannot become a way to register small directories.
expect_output fail "a cohesive directory at the default" "is at or under the default" \
  env PACKAGE_SIZE_ROOT="${cohesive}" PACKAGE_SIZE_BUDGETS="${ceiling_budgets}" \
  DEFAULT_MAX_FILES=4 "${check}"

# --list shows how far each cohesive directory is from its ceiling.
expect_output pass "--list prints a cohesive directory's headroom" \
  "internal/panes                                    4 / 6    (cohesive, 2 to the ceiling)" \
  env PACKAGE_SIZE_ROOT="${cohesive}" PACKAGE_SIZE_BUDGETS="${ceiling_budgets}" \
  DEFAULT_MAX_FILES=2 "${check}" --list

# Every other directory keeps zero headroom, and one past the default is told
# to split or bump.
expect_output fail "a directory past the default" "There are TWO legitimate" \
  env PACKAGE_SIZE_ROOT="${over}" PACKAGE_SIZE_BUDGETS="${empty_budgets}" \
  DEFAULT_MAX_FILES=2 "${check}"

# An entry of a kind the gate does not know fails rather than passing.
for entry in "internal/panes 6 cohesiv" "internal/panes cohesive" "internal/panes 6 cohesive extra"; do
  unknown_budgets="${workdir}/unknown-budgets.txt"
  printf '# a reason\n%s\n' "${entry}" >"${unknown_budgets}"
  expect fail "a malformed entry: ${entry}" "${cohesive}" "${unknown_budgets}" 2
done

# An entry with no reason comment above it fails.
uncommented="${workdir}/uncommented-budgets.txt"
printf 'internal/big 3\n' >"${uncommented}"
expect fail "an entry without a reason comment" "${declared}" "${uncommented}" 2

# An exempt tree passes however many files it holds.
exempt="$(repo exempt)"
for i in 1 2 3 4 5; do add "${exempt}" "web/src/api/generated/f${i}.ts"; done
exempt_budgets="${workdir}/exempt-budgets.txt"
printf '# generated tree, not a concern budget\nweb/src/api/generated exempt\n' >"${exempt_budgets}"
expect pass "an exempt generated tree" "${exempt}" "${exempt_budgets}" 2

# A repository with nothing counted fails rather than passing on nothing.
emptyrepo="$(repo emptyrepo)"
expect fail "a repository with no source files" "${emptyrepo}" "${empty_budgets}" 12

finish_tests
