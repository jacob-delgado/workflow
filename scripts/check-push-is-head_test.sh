#!/usr/bin/env bash
#
# Tests for check-push-is-head.sh: given git's pre-push lines on standard
# input, it passes a push of the branch checked out, a branch deleted, a tag,
# and no lines at all, as a run of the hook by hand has, and fails, naming the
# ref, a push of a branch at any other commit. And lefthook.yml's pre-push runs
# it, with git's lines, even when the branch checked out changes nothing
# against origin/main, which leaves every other check of the hook no file to
# run on: a push of another branch from main is refused, and one of main is
# not. A refused push stops the whole hook before any other check runs; an
# allowed one runs every check its files select, a failing one hiding none
# after it, but those LEFTHOOK_EXCLUDE names. Those cases run the lefthook
# mise.toml pins, and are skipped where none is on PATH.
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
    lefthook run pre-push origin "${origin}" --no-auto-install --job push-is-head
}

# Stand-ins for the tools the hook's checks run, each noting its run: task,
# which fails, and golangci-lint, which passes.
readonly stand_ins="${workdir}/stand-ins" ran="${workdir}/ran.log"
mkdir -p "${stand_ins}"
for tool in task:1 golangci-lint:0; do
  printf '#!/usr/bin/env bash\necho %s >>"%s"\nexit %s\n' "${tool%:*}" "${ran}" "${tool#*:}" >"${stand_ins}/${tool%:*}"
  chmod +x "${stand_ins}/${tool%:*}"
done

# whole_hook runs all of lefthook's pre-push in the repository as git's push
# would, with the lines given on its standard input and the stand-ins first on
# PATH, skipping only the checks the case's EXCLUDE names.
#   [EXCLUDE=<names>] whole_hook <line>...
whole_hook() {
  local -a exclusion=(-u LEFTHOOK_EXCLUDE)
  if [[ -n "${EXCLUDE:-}" ]]; then exclusion=("LEFTHOOK_EXCLUDE=${EXCLUDE}"); fi
  : >"${ran}"
  printf '%s\n' "$@" | run_in "${repo}" env -u LEFTHOOK "${exclusion[@]}" PATH="${stand_ins}:${PATH}" \
    lefthook run pre-push origin "${origin}" --no-auto-install
}

if command -v lefthook >/dev/null; then
  expect_exit pass "lefthook's pre-push, a push of the branch checked out" \
    hook "refs/heads/main ${head} refs/heads/main ${head}"
  expect_output fail "lefthook's pre-push, a push of another branch from main" "refs/heads/other" \
    hook "refs/heads/other ${other} refs/heads/other ${zeros}"

  # A refused push stops the hook before its other checks: the repository
  # holds none of the four whole-tree scripts, so a run of one would name it.
  expect_output fail "the whole pre-push, a push of another branch from main" "refs/heads/other" \
    whole_hook "refs/heads/other ${other} refs/heads/other ${zeros}"
  count_case
  if grep -qE "check-(file-length|package-size|tradeoffs|tech-debt-paths)[.]sh" "${case_output}"; then
    fail_case "the whole pre-push, a push of another branch from main" "a check ran after the push was refused"
  fi

  # An allowed push runs every check its files select, a failing one hiding
  # none after it, and LEFTHOOK_EXCLUDE still skips the ones it names.
  git -C "${repo}" switch -q other
  expect_exit fail "the whole pre-push, a push of the Go branch checked out" \
    whole_hook "refs/heads/other ${other} refs/heads/other ${zeros}"
  count_case
  if [[ "$(sort "${ran}" | tr '\n' ' ')" != "golangci-lint task " ]]; then
    fail_case "the whole pre-push, a push of the Go branch checked out" "not every Go check ran: $(<"${ran}")"
  fi
  EXCLUDE=unit-go,web expect_exit fail "the whole pre-push, unit-go and web excluded" \
    whole_hook "refs/heads/other ${other} refs/heads/other ${zeros}"
  count_case
  if [[ "$(<"${ran}")" != golangci-lint ]]; then
    fail_case "the whole pre-push, unit-go and web excluded" "the checks run were not lint-go-all alone: $(<"${ran}")"
  fi
  git -C "${repo}" switch -q main
else
  echo "check-push-is-head_test.sh: lefthook's cases skipped, no lefthook here"
fi

finish_tests
