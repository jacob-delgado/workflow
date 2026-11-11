#!/usr/bin/env bash
#
# Tests for check-push-is-head.sh: given git's pre-push lines on standard
# input, it passes a push of the branch checked out, a branch deleted, a tag,
# and no lines at all, as a run of the hook by hand has, and fails, naming the
# ref, a push of a branch at any other commit. And lefthook.yml's pre-push runs
# it, with git's lines, even when the branch checked out changes nothing
# against origin/main, which leaves every other check of the hook no file to
# run on: a push of another branch from main is refused, and one of main is
# not. Those cases run the lefthook mise.toml pins, and are skipped where none
# is on PATH.
#
# Usage:
#   scripts/check-push-is-head_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-push-is-head.sh"
readonly zeros=0000000000000000000000000000000000000000

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# commit makes a commit on the branch checked out, changing the file named.
#   commit <dir> <file>
commit() {
  printf '%s\n' "$2" >>"$1/$2"
  git -C "$1" add -A
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m "$2"
}

# The repository: main, pushed to its origin, with the hook's configuration and
# this check; a branch, other, one commit past it; and main checked out.
readonly origin="${workdir}/origin.git" repo="${workdir}/repo"
git init -q --bare "${origin}"
git init -q --initial-branch=main "${repo}"
mkdir -p "${repo}/scripts"
cp "${here}/../lefthook.yml" "${repo}/lefthook.yml"
cp "${check}" "${repo}/scripts/check-push-is-head.sh"
commit "${repo}" README.md
git -C "${repo}" remote add origin "${origin}"
git -C "${repo}" push -q origin main
git -C "${repo}" fetch -q origin
git -C "${repo}" switch -q -c other
commit "${repo}" other.go
git -C "${repo}" switch -q main
head="$(git -C "${repo}" rev-parse HEAD)"
other="$(git -C "${repo}" rev-parse other)"
readonly head other

# pushing runs the check in the repository as git's pre-push hook would, with
# the lines given on its standard input.
#   pushing <line>...
pushing() {
  if (($# > 0)); then printf '%s\n' "$@"; fi | run_in "${repo}" "${check}"
}

expect_exit pass "a push of the branch checked out" pushing "refs/heads/main ${head} refs/heads/main ${head}"

expect_output fail "a push of another branch" "refs/heads/other" \
  pushing "refs/heads/other ${other} refs/heads/other ${zeros}"

expect_output fail "a push of another commit to the branch checked out" "refs/heads/main" \
  pushing "refs/heads/main ${head} refs/heads/main ${head}" "${other} ${other} refs/heads/main ${head}"

expect_exit pass "a branch deleted" pushing "(delete) ${zeros} refs/heads/other ${other}"

expect_exit pass "a tag" pushing "refs/tags/v1 ${other} refs/tags/v1 ${zeros}"

expect_exit pass "no lines, as from a run of the hook by hand" pushing

# hook runs lefthook's pre-push in the repository as git's push would, with
# the lines given on its standard input, whatever a hook variable the test
# runs under would skip.
#   hook <line>...
hook() {
  printf '%s\n' "$@" | run_in "${repo}" env -u LEFTHOOK -u LEFTHOOK_EXCLUDE \
    lefthook run pre-push origin "${origin}" --no-auto-install --command push-is-head
}

if command -v lefthook >/dev/null; then
  expect_exit pass "lefthook's pre-push, a push of the branch checked out" \
    hook "refs/heads/main ${head} refs/heads/main ${head}"
  expect_output fail "lefthook's pre-push, a push of another branch from main" "refs/heads/other" \
    hook "refs/heads/other ${other} refs/heads/other ${zeros}"
else
  echo "check-push-is-head_test.sh: lefthook's cases skipped, no lefthook here"
fi

finish_tests
