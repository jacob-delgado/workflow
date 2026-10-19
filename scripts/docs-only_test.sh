#!/usr/bin/env bash
#
# Tests for docs-only.sh: it says a change is docs only when every file it
# changes since the base is Markdown outside web/, the site's configuration and
# module files under docs/, or the configuration of the two linters that read
# only prose (.markdownlint-cli2.yaml, _typos.toml); any other file, Markdown
# under web/, an image under docs/, a deleted file or a renamed one included,
# makes it a code change; and when git cannot answer, it says so with a status
# of its own, so a caller runs every check rather than too few.
#
# Usage:
#   scripts/docs-only_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly docs_only="${here}/docs-only.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# repo_changing makes a repository whose base commit holds a file of each kind,
# then a branch commit that changes each path named, and leaves it at
# ${workdir}/<name>.
#   repo_changing <name> <path>...
repo_changing() {
  local repo="${workdir}/${1}" path
  shift
  mkdir -p "${repo}/docs" "${repo}/internal"
  git -C "${repo}" init --quiet --initial-branch=main
  printf 'x\n' >"${repo}/README.md"
  printf 'x\n' >"${repo}/docs/page.md"
  printf 'x\n' >"${repo}/internal/code.go"
  git -C "${repo}" add -A
  git -C "${repo}" -c user.email=t@example.com -c user.name=t commit --quiet -m base
  git -C "${repo}" switch --quiet -c work
  for path in "$@"; do
    mkdir -p "$(dirname "${repo}/${path}")"
    printf 'changed\n' >>"${repo}/${path}"
  done
  git -C "${repo}" add -A
  git -C "${repo}" -c user.email=t@example.com -c user.name=t commit --quiet -m change
}

# expect runs docs-only.sh in the named repository against main and compares
# its exit status with want.
#   expect <case> <repo-name> <want-status>
expect() {
  expect_exit "$3" "$1" run_in "${workdir}/$2" "${docs_only}" main
}

repo_changing readme README.md
repo_changing nested .claude/skills/lanes/SKILL.md
repo_changing site docs/content/page.md docs/hugo.toml docs/go.mod docs/go.sum
repo_changing webdoc web/README.md
repo_changing siteimage docs/assets/images/shot.png
repo_changing sitescript docs/tools/record.sh
repo_changing linters .markdownlint-cli2.yaml _typos.toml
repo_changing code internal/code.go
repo_changing mixed README.md internal/code.go
repo_changing script scripts/gate.sh
repo_changing workflow .github/workflows/ci.yml
repo_changing taskfile Taskfile.yml

# A deleted code file is a code change, though nothing in the diff is code
# that still exists.
repo_changing deleted README.md
git -C "${workdir}/deleted" rm --quiet internal/code.go
git -C "${workdir}/deleted" -c user.email=t@example.com -c user.name=t commit --quiet -m delete

# A code file renamed to a Markdown name is a code change too: the old path
# counts as well as the new.
repo_changing renamed README.md
git -C "${workdir}/renamed" mv internal/code.go internal/code.md
git -C "${workdir}/renamed" -c user.email=t@example.com -c user.name=t commit --quiet -m rename

# Act & Assert
expect "a Markdown file at the root" readme 0
expect "a Markdown file anywhere, a skill's among them" nested 0
expect "the site: its pages, its configuration and its module files" site 0
expect "Markdown under web/, which its prettier check reads" webdoc 1
expect "an image under docs/, which no docs gate reads" siteimage 1
expect "a script under docs/" sitescript 1
expect "the two prose linters' configuration" linters 0
expect "code alone" code 1
expect "docs and code together" mixed 1
expect "a gate script" script 1
expect "a CI workflow" workflow 1
expect "the Taskfile" taskfile 1
expect "a deleted code file" deleted 1
expect "a code file renamed to Markdown" renamed 1

# Act & Assert: a change with nothing in it is not docs only, so the caller
# runs everything rather than nothing.
expect_exit 1 "no change at all" run_in "${workdir}/code" "${docs_only}" HEAD

# Act & Assert: a base git cannot resolve cannot be told, and says so apart from
# "code changed".
expect_exit 2 "an unknown base" run_in "${workdir}/code" "${docs_only}" no-such-branch

finish_tests
