#!/usr/bin/env bash
#
# Tests for check-layout.sh: every directory holding a tracked Go file must be
# named in CLAUDE.md's layout block, every directory the block names must hold
# a tracked file, and a repository git cannot read, or a CLAUDE.md with no
# layout block, must not pass on nothing compared.
#
# Usage:
#   scripts/check-layout_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-layout.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# layout writes a CLAUDE.md whose layout block names each argument, as the real
# one does: the directory with a trailing slash, then what it holds.
#   layout <dir>...
layout() {
  cat <<'END'
# CLAUDE.md

Layout:

```text
END
  local dir
  for dir in "$@"; do
    printf '%-22s what it holds\n' "${dir}/"
  done
  cat <<'END'
```

## Common commands

```text
not/a/directory/   in another block
```
END
}

# repo makes a git repository at dir with a CLAUDE.md naming the directories
# after the first `--`, and a tracked file at each path after it.
#   repo <dir> <named-dir>... -- <tracked-path>...
repo() {
  local dir="$1"
  shift
  local listed=()
  while [[ "$1" != "--" ]]; do
    listed+=("$1")
    shift
  done
  shift

  mkdir -p "${dir}"
  git -C "${dir}" init -q
  layout "${listed[@]}" >"${dir}/CLAUDE.md"
  git -C "${dir}" add CLAUDE.md

  local path
  for path in "$@"; do
    mkdir -p "${dir}/$(dirname "${path}")"
    printf 'package x\n' >"${dir}/${path}"
    git -C "${dir}" add "${path}"
  done
}

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  expect_exit "$1" "$2" run_in "$3" "${check}"
}

# Every Go package named, and every named directory there: the gate passes.
complete="${workdir}/complete"
repo "${complete}" cmd/workflow internal/cli web \
  -- cmd/workflow/main.go internal/cli/cli.go internal/cli/cli_test.go web/index.html
expect pass "every Go package named" "${complete}"

# A Go package the layout leaves out fails, and the gate names it.
unnamed="${workdir}/unnamed"
repo "${unnamed}" cmd/workflow -- cmd/workflow/main.go internal/slackauth/token.go
expect_output fail "a Go package the layout leaves out" "internal/slackauth" run_in "${unnamed}" "${check}"

# A package nested in a named one is a package of its own, so it needs a line.
nested="${workdir}/nested"
repo "${nested}" internal/messaging -- internal/messaging/post.go internal/messaging/directory/read.go
expect_output fail "a nested package the layout leaves out" "internal/messaging/directory" \
  run_in "${nested}" "${check}"

# A Go file only under testdata is no package Go builds, so it needs no line.
testdata="${workdir}/testdata"
repo "${testdata}" internal/cli -- internal/cli/cli.go internal/cli/testdata/fixture/main.go
expect pass "a Go file under testdata" "${testdata}"

# A line naming a directory that holds no tracked file is a stale line.
stale="${workdir}/stale"
repo "${stale}" internal/cli internal/gone -- internal/cli/cli.go
expect_output fail "a line for a directory that is gone" "internal/gone" run_in "${stale}" "${check}"

# A CLAUDE.md with no layout block cannot be compared with, so it fails.
noblock="${workdir}/noblock"
mkdir -p "${noblock}/internal/cli"
git -C "${noblock}" init -q
printf '# CLAUDE.md\n\nNo layout here.\n' >"${noblock}/CLAUDE.md"
printf 'package x\n' >"${noblock}/internal/cli/cli.go"
git -C "${noblock}" add CLAUDE.md internal/cli/cli.go
expect fail "a CLAUDE.md with no layout block" "${noblock}"

# A directory that is not a repository: git cannot list, so the gate refuses.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
layout internal/cli >"${notrepo}/CLAUDE.md"
expect fail "a directory that is not a repository" "${notrepo}"

finish_tests
