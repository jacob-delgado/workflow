#!/usr/bin/env bash
#
# Tests for check-generated.sh: run after a generator, it passes when the
# generated tree is what git holds, and fails, naming the file, when a tracked
# file changed or the generator left a new file nobody committed. A path git
# tracks nothing under, or a directory that is not a repository, fails rather
# than passing on nothing compared.
#
# Usage:
#   scripts/check-generated_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-generated.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly advice="Run the generator and commit gen/."

# generated makes a repository whose gen/ holds one committed file, and a
# .gitignore that ignores gen/*.tmp.
#   generated <name>
generated() {
  local repo="${workdir}/$1"
  mkdir -p "${repo}/gen"
  git -C "${repo}" init -q
  printf 'one\n' >"${repo}/gen/one.ts"
  printf 'gen/*.tmp\n' >"${repo}/.gitignore"
  git -C "${repo}" add -A
  git -C "${repo}" -c user.email=t@example.com -c user.name=t commit -q -m generated
  printf '%s' "${repo}"
}

clean="$(generated clean)"
expect_exit pass "the generated tree is what git holds" run_in "${clean}" "${check}" gen "${advice}"

changed="$(generated changed)"
printf 'two\n' >>"${changed}/gen/one.ts"
expect_output fail "a tracked file the generator changed" "gen/one.ts" \
  run_in "${changed}" "${check}" gen "${advice}"

added="$(generated added)"
printf 'new\n' >"${added}/gen/two.ts"
expect_output fail "a new file the generator left uncommitted" "?? gen/two.ts" \
  run_in "${added}" "${check}" gen "${advice}"

ignored="$(generated ignored)"
printf 'scratch\n' >"${ignored}/gen/build.tmp"
expect_exit pass "an ignored file beside the generated tree" \
  run_in "${ignored}" "${check}" gen "${advice}"

expect_output fail "the advice on a failure" "${advice}" \
  run_in "${added}" "${check}" gen "${advice}"

expect_exit fail "a path git tracks nothing under" run_in "${clean}" "${check}" missing "${advice}"

notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}/gen"
expect_exit fail "a directory that is not a repository" run_in "${notrepo}" "${check}" gen "${advice}"

expect_exit 2 "no advice" run_in "${clean}" "${check}" gen

finish_tests
