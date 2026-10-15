#!/usr/bin/env bash
#
# Tests for check-layout.sh: every directory holding a tracked Go file must be
# named in CLAUDE.md's layout block, every directory the block names must hold
# a tracked file, every package the block marks "(tests only)" must be one a
# depguard rule holds to tests, in a .golangci.yml golangci-lint reads and no
# file it reads first, and a repository git cannot read, or a CLAUDE.md with
# no layout block, must not pass on nothing compared. The mark's need of a
# depguard that runs is check-layout_runs_test.sh's, which rules hold a
# package check-layout_depguard_test.sh's, and how the gate reads the YAML
# they are written in check-layout_yaml_test.sh's.
#
# Usage:
#   scripts/check-layout_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-layout.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-layout-fixtures.sh
source "${here}/lib/check-layout-fixtures.sh"

# lint writes, in a repository repo made, the go.mod of a module named example
# and a .golangci.yml that turns depguard on, as the root's does, whose
# depguard rules hold each package named to tests, as the root's
# *-only-in-tests rules do.
#   lint <dir> <package-dir>...
lint() {
  local dir="$1"
  shift
  module "${dir}"
  {
    depguard_rules_head "default: all"
    rules_holding "$@"
  } >"${dir}/.golangci.yml"
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

# coverage-gate.sh leaves each package the layout marks "(tests only)" out of
# the statement total, so the mark must be the build's word: a marked package
# passes when a depguard rule denies its import path to every file but a test.
held="${workdir}/held"
repo "${held}" internal/cli "internal/rlimit (tests only)" \
  -- internal/cli/cli.go internal/rlimit/rlimit.go
lint "${held}" internal/rlimit
expect pass "a package marked (tests only) that depguard holds to tests" "${held}"

# A marked package no rule holds fails, and the gate names it and the fix.
unheld="${workdir}/unheld"
repo "${unheld}" "internal/rlimit (tests only)" "internal/gittest (tests only)" \
  -- internal/rlimit/rlimit.go internal/gittest/gittest.go
lint "${unheld}" internal/rlimit
expect_output fail "a package marked (tests only) that no rule holds" \
  $'  internal/gittest/ (pkg: example/internal/gittest)\nGive each a depguard rule' \
  run_in "${unheld}" "${check}"

# The same package unmarked needs no rule: the mark is what asks for one.
unmarked="${workdir}/unmarked"
repo "${unmarked}" "internal/rlimit (tests only)" internal/gittest \
  -- internal/rlimit/rlimit.go internal/gittest/gittest.go
lint "${unmarked}" internal/rlimit
expect pass "a package not marked (tests only) needs no rule" "${unmarked}"

# A rule holds the import path its deny list names and no other, so one that
# holds internal/webserver/webtest leaves internal/webserver above it unheld.
parent="${workdir}/parent"
repo "${parent}" "internal/webserver (tests only)" "internal/webserver/webtest (tests only)" \
  -- internal/webserver/server.go internal/webserver/webtest/serve.go
lint "${parent}" internal/webserver/webtest
expect_output fail "a marked package only a rule for the package below it names" \
  $'tests:\n  internal/webserver/ (pkg: example/internal/webserver)\nGive each' \
  run_in "${parent}" "${check}"

# The root's own .golangci.yml and go.mod turn depguard on and hold the
# packages the root's layout marks to tests.
root="${workdir}/root"
repo "${root}" "internal/rlimit (tests only)" "internal/gittest (tests only)" \
  "internal/ptytest (tests only)" "internal/webserver/webtest (tests only)" \
  -- internal/rlimit/rlimit.go internal/gittest/gittest.go internal/ptytest/ptytest.go \
  internal/webserver/webtest/webtest.go
cp "${here}/../.golangci.yml" "${here}/../go.mod" "${root}/"
expect pass "packages the root marks, under the root's .golangci.yml" "${root}"

# With no .golangci.yml there is no rule, so a marked package fails by name.
noconfig="${workdir}/noconfig"
repo "${noconfig}" "internal/rlimit (tests only)" -- internal/rlimit/rlimit.go
module "${noconfig}"
expect_output fail "a marked package and no .golangci.yml" \
  "internal/rlimit/ (pkg: example/internal/rlimit)" run_in "${noconfig}" "${check}"

# golangci-lint looks for a .golangci.json, .golangci.toml and .golangci.yaml
# before a .golangci.yml, tracked or not, so beside one the rules golangci-lint
# holds a package to are that file's: the gate, which reads .golangci.yml
# alone, refuses, naming it, though .golangci.yml holds the package.
for sibling in json:'{"version": "2"}' toml:'version = "2"' yaml:'version: "2"'; do
  config=".golangci.${sibling%%:*}"
  dir="${workdir}/sibling-${sibling%%:*}"
  repo "${dir}" "internal/rlimit (tests only)" -- internal/rlimit/rlimit.go
  lint "${dir}" internal/rlimit
  printf '%s\n' "${sibling#*:}" >"${dir}/${config}"
  expect_output 2 "a marked package .golangci.yml holds, beside a ${config} that holds no rule" \
    "golangci-lint reads ${config} ahead of .golangci.yml" run_in "${dir}" "${check}"
done

# Without a go.mod there is no import path to look for a marked package's rule
# by, so the gate refuses rather than pass it unread.
nomodule="${workdir}/nomodule"
repo "${nomodule}" "internal/rlimit (tests only)" -- internal/rlimit/rlimit.go
lint "${nomodule}" internal/rlimit
rm "${nomodule}/go.mod"
expect_output fail "a marked package and no go.mod to name it by" "no module path" \
  run_in "${nomodule}" "${check}"

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
