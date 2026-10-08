#!/usr/bin/env bash
#
# Tests for markdown-changed.sh: it says Markdown needs linting when a change
# since the base touches docs/, any .md file, the markdownlint config, or
# mise.toml, which pins markdownlint-cli2, and that it does not for a change to
# code alone; and when git cannot answer, it says so with a status of its own,
# so CI lints rather than skips.
#
# Usage:
#   scripts/markdown-changed_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly changed="${here}/markdown-changed.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

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
  expect_exit "$3" "$1" run_in "${workdir}/$2" "${changed}" main
}

repo_changing docs docs/page.txt
repo_changing readme README.md
repo_changing nested internal/notes/NOTES.md
repo_changing config .markdownlint-cli2.yaml
repo_changing code internal/code.go
repo_changing toolchain mise.toml

# Act & Assert
expect "a change under docs/" docs 0
expect "a Markdown file at the root" readme 0
expect "a Markdown file anywhere" nested 0
expect "the markdownlint config" config 0
expect "the toolchain pins, markdownlint-cli2's among them" toolchain 0
expect "code alone" code 1

# Act & Assert: a base git cannot resolve cannot be told, and says so apart from
# "nothing changed".
expect_exit 2 "an unknown base" run_in "${workdir}/code" "${changed}" no-such-branch

finish_tests
