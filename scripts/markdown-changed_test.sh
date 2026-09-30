#!/usr/bin/env bash
#
# Tests for markdown-changed.sh: it says Markdown needs linting when a change
# since the base touches docs/, any .md file, or the markdownlint config, and
# that it does not for a change to code alone; and when git cannot answer, it
# says so with a status of its own, so CI lints rather than skips.
#
# Usage:
#   scripts/markdown-changed_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly changed="${here}/markdown-changed.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories this test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

# fail records a failed case with its reason.
#   fail <name> <reason>
fail() {
  echo "FAIL ${1}: ${2}" >&2
  failures=$((failures + 1))
}

# repo_changing makes a repository whose base commit holds a file of each kind,
# then a branch commit that changes path, and leaves it at ${workdir}/<name>.
#   repo_changing <name> <path>
repo_changing() {
  local repo="${workdir}/${1}"
  mkdir -p "${repo}/docs" "${repo}/internal"
  git -C "${repo}" init --quiet --initial-branch=main
  printf 'x\n' >"${repo}/README.md"
  printf 'x\n' >"${repo}/docs/page.txt"
  printf 'x\n' >"${repo}/internal/code.go"
  printf 'x\n' >"${repo}/.markdownlint-cli2.yaml"
  git -C "${repo}" add -A
  git -C "${repo}" -c user.email=t@example.com -c user.name=t commit --quiet -m base
  git -C "${repo}" switch --quiet -c work
  mkdir -p "$(dirname "${repo}/${2}")"
  printf 'changed\n' >>"${repo}/${2}"
  git -C "${repo}" add -A
  git -C "${repo}" -c user.email=t@example.com -c user.name=t commit --quiet -m change
}

# expect runs markdown-changed.sh in the named repository against main and
# compares its exit status with want.
#   expect <case> <repo-name> <want-status>
expect() {
  local status=0
  cases=$((cases + 1))
  (cd "${workdir}/${2}" && "${changed}" main >/dev/null 2>&1) || status=$?
  if ((status != ${3})); then
    fail "${1}" "want exit ${3}, got ${status}"
  fi
}

repo_changing docs docs/page.txt
repo_changing readme README.md
repo_changing nested internal/notes/NOTES.md
repo_changing config .markdownlint-cli2.yaml
repo_changing code internal/code.go

# Act & Assert
expect "a change under docs/" docs 0
expect "a Markdown file at the root" readme 0
expect "a Markdown file anywhere" nested 0
expect "the markdownlint config" config 0
expect "code alone" code 1

# Act: a base git cannot resolve.
status=0
(cd "${workdir}/code" && "${changed}" no-such-branch >/dev/null 2>&1) || status=$?

# Assert: it cannot tell, and says so apart from "nothing changed".
cases=$((cases + 1))
if ((status != 2)); then
  fail "an unknown base" "want exit 2, got ${status}"
fi

if ((failures > 0)); then
  echo "markdown-changed_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "markdown-changed_test: ${cases} case(s) passed."
