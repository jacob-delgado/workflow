#!/usr/bin/env bash
#
# Tests for which tasks scripts/gate-as-dev.sh hands the gate user when it is
# asked for none: for a change to the docs alone, which scripts/docs-only.sh
# decides against GATE_BASE, task check:docs, said so; for any other change,
# and for one it cannot compare with its base, nothing, so the gate user runs
# the steps of task check; and tasks named on the command line as named. The
# stand-ins lib/gate-as-dev-fixtures.sh writes record what they were asked, so
# no user is switched and no gate runs.
#
# Usage:
#   scripts/gate-as-dev_docs_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The gate takes its lock with flock, which util-linux has and stock macOS
# does not; the script is for a Linux cloud session alone.
command -v flock >/dev/null || {
  echo "gate-as-dev_docs_test.sh: skipped, no flock here"
  exit 0
}

# shellcheck source=lib/gate-as-dev-fixtures.sh
source "${here}/lib/gate-as-dev-fixtures.sh"

# changed_repo makes a repository whose main holds a page and some code, then a
# branch commit that changes the path named, and prints nothing.
#   changed_repo <dir> <path>
changed_repo() {
  repo "$1"
  git -C "$1" branch -q -M main
  mkdir -p "$1/docs" "$1/internal"
  printf 'page\n' >"$1/docs/page.md"
  printf 'code\n' >"$1/internal/code.go"
  git -C "$1" add -A
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m base
  git -C "$1" switch -q -c work
  printf 'changed\n' >>"$1/$2"
  git -C "$1" add -A
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m change
}

# handed_tasks prints what follows the commit in the run handed to dev.
#   handed_tasks <sha>
handed_tasks() {
  local handed
  handed="$(grep -- "--as-gate-user" "${log}" || true)"
  printf '%s' "${handed#*"$1"}"
}

docs_repo="${workdir}/docs-change"
changed_repo "${docs_repo}" docs/page.md
code_repo="${workdir}/code-change"
changed_repo "${code_repo}" internal/code.go

disk 12
export GATE_BASE=main
expect_output pass "a docs-only change" "docs only, so this runs task check:docs" as_root "${docs_repo}"
tasks="$(handed_tasks "$(git -C "${docs_repo}" rev-parse HEAD)")"
count_case
if [[ "${tasks}" != " check:docs" ]]; then
  fail_case "a docs-only change" "dev was not handed check:docs: ${tasks}"
fi

expect_exit pass "a code change" as_root "${code_repo}"
tasks="$(handed_tasks "$(git -C "${code_repo}" rev-parse HEAD)")"
count_case
if [[ -n "${tasks}" ]]; then
  fail_case "a code change" "dev was handed tasks, not the whole of check: ${tasks}"
fi

expect_exit pass "a docs-only change with tasks named" as_root "${docs_repo}" lint
tasks="$(handed_tasks "$(git -C "${docs_repo}" rev-parse HEAD)")"
count_case
if [[ "${tasks}" != " lint" ]]; then
  fail_case "a docs-only change with tasks named" "the tasks named were not the ones handed: ${tasks}"
fi

export GATE_BASE=no-such-branch
expect_exit pass "a base it cannot compare with" as_root "${docs_repo}"
tasks="$(handed_tasks "$(git -C "${docs_repo}" rev-parse HEAD)")"
count_case
if [[ -n "${tasks}" ]]; then
  fail_case "a base it cannot compare with" "it ran less than the whole of check: ${tasks}"
fi
unset GATE_BASE

finish_tests
