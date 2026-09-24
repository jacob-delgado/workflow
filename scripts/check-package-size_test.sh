#!/usr/bin/env bash
#
# Tests for check-package-size.sh: the gate must count the right files, honor a
# declared budget in both directions, and never pass when it measured nothing.
#
# Usage:
#   scripts/check-package-size_test.sh
set -euo pipefail

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories this test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-package-size.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

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
  local want="$1" name="$2" dir="$3" budgets="$4" default_max="${5:-12}" got
  cases=$((cases + 1))

  if PACKAGE_SIZE_ROOT="${dir}" PACKAGE_SIZE_BUDGETS="${budgets}" \
    DEFAULT_MAX_FILES="${default_max}" "${check}" >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
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

if ((failures > 0)); then
  echo "check-package-size_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-package-size_test: ${cases} case(s) passed."
