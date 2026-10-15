#!/usr/bin/env bash
#
# Tests for coverage-gate.sh: coverage below the floor fails, coverage at or
# above it passes, the total leaves out the generated trees and every package
# CLAUDE.md's layout block marks "(tests only)" but counts every other, and a
# floor nobody passed, a profile that is not there, or a layout the gate
# cannot read fails rather than being defaulted or measured as nothing.
#
# Usage:
#   scripts/coverage-gate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/coverage-gate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# functions prints a Go file of two functions, A and B, in the package named.
#   functions <package>
functions() {
  cat <<EOF
package $1

func A() int {
	return 1
}

func B() int {
	return 2
}
EOF
}

# guide prints a CLAUDE.md whose layout block holds each line given, as the
# root's does: a directory with its trailing slash, then what it holds.
#   guide <layout-line>...
guide() {
  printf '# CLAUDE.md\n\nLayout:\n\n```text\n'
  printf '%s\n' "$@"
  printf '```\n'
}

# A minimal module with two functions, one covered and one not, so its profile
# measures 50.0%. go tool cover resolves the profile's paths against this
# module, and the gate reads the layout beside its go.mod, so the gate must run
# from here.
module="${workdir}/module"
mkdir -p "${module}"
cat >"${module}/go.mod" <<'EOF'
module example

go 1.27
EOF
functions example >"${module}/f.go"
guide 'cmd/workflow/  thin main' >"${module}/CLAUDE.md"
readonly profile="${module}/cover.out"
cat >"${profile}" <<'EOF'
mode: set
example/f.go:3.16,5.2 1 1
example/f.go:7.16,9.2 1 0
EOF

# expect runs the gate from the module and checks its exit.
#   expect <pass|fail> <name> <arg>...
expect() {
  expect_exit "$1" "$2" run_in "${module}" "${gate}" "${@:3}"
}

# 50% coverage clears a floor of 40 and misses a floor of 60.
expect pass "coverage above the floor" "${profile}" 40
expect fail "coverage below the floor" "${profile}" 60

# The floor is required, not defaulted: without one the gate must not pass.
expect fail "no floor argument" "${profile}"

# A profile that is not there, or is empty, fails rather than measuring nothing.
expect fail "a missing profile" "${module}/does-not-exist.out" 40
: >"${module}/empty.out"
expect fail "an empty profile" "${module}/empty.out" 40

# A module of five packages with the two functions each: internal/cli, whose
# tests run both; internal/cli/clitest, nested in it, whose tests run one; and
# three that nothing runs: the two generated trees and internal/rlimit. Each
# package's share differs, so a total shows which of them it counted.
packages="${workdir}/packages"
mkdir -p "${packages}"
printf 'module example\n\ngo 1.27\n' >"${packages}/go.mod"
for dir in internal/cli internal/cli/clitest internal/rlimit cmd/docsgen internal/api; do
  mkdir -p "${packages}/${dir}"
  functions "$(basename "${dir}")" >"${packages}/${dir}/f.go"
done

# profile prints a coverage profile of the f.go in each directory given, with
# A and B run in internal/cli, A alone in internal/cli/clitest, and nothing run
# anywhere else.
#   profile <dir>...
profile() {
  local dir ran_a ran_b
  printf 'mode: set\n'
  for dir in "$@"; do
    case "${dir}" in
      internal/cli) ran_a=1 ran_b=1 ;;
      internal/cli/clitest) ran_a=1 ran_b=0 ;;
      *) ran_a=0 ran_b=0 ;;
    esac
    printf 'example/%s/f.go:3.16,5.2 1 %s\n' "${dir}" "${ran_a}"
    printf 'example/%s/f.go:7.16,9.2 1 %s\n' "${dir}" "${ran_b}"
  done
}

profile internal/cli internal/cli/clitest internal/rlimit >"${packages}/helpers.out"
profile internal/cli cmd/docsgen internal/api >"${packages}/generated.out"

# expect_total runs the gate over a profile of the packages module at a floor
# of 40, under a CLAUDE.md of the layout lines given, and wants its exit and
# the total it printed.
#   expect_total <pass|fail> <name> <total> <profile-file> <layout-line>...
expect_total() {
  guide "${@:5}" >"${packages}/CLAUDE.md"
  expect_output "$1" "$2" "Coverage $3%" run_in "${packages}" "${gate}" "$4" 40
}

# The packages the layout marks "(tests only)" leave the total, and internal/cli
# above them still counts: both its functions ran. Left in, the nested clitest
# would take the total to 75.0%, and rlimit to 50.0%.
expect_total pass "packages marked (tests only) leave the total" 100.0 helpers.out \
  'internal/cli/          the command tree' \
  'internal/cli/clitest/  fixtures for the command tree tests (tests only)' \
  'internal/rlimit/       lowering a resource limit around one test call (tests only)'

# The same packages, unmarked, count: three functions of six ran.
expect_total pass "packages not marked (tests only) still count" 50.0 helpers.out \
  'internal/cli/          the command tree' \
  'internal/cli/clitest/  fixtures for the command tree tests' \
  'internal/rlimit/       lowering a resource limit around one test call'

# A mark leaves out its own directory alone: below a marked internal/cli, the
# unmarked internal/cli/clitest is a package of its own, and counts, its one
# function of two that ran beside internal/rlimit's two that did not. A mark
# read as a prefix would leave clitest out too and measure 0.0%.
expect_total fail "a package below a marked one still counts" 25.0 helpers.out \
  'internal/cli/          the command tree (tests only)' \
  'internal/cli/clitest/  fixtures for the command tree tests' \
  'internal/rlimit/       lowering a resource limit around one test call'

# The generated trees leave the total, whatever the layout says of them.
expect_total pass "the generated trees leave the total" 100.0 generated.out \
  'internal/cli/  the command tree' \
  'cmd/docsgen/   the thin main behind the command reference' \
  'internal/api/  the Go types generated from api/'

# A layout the gate cannot read stops it, whatever the profile measures, rather
# than count every package or none: no CLAUDE.md, and one with no layout block.
rm "${packages}/CLAUDE.md"
expect_output fail "no CLAUDE.md to read the layout from" "no CLAUDE.md" \
  run_in "${packages}" "${gate}" generated.out 40
printf '# CLAUDE.md\n\nNo layout here.\n' >"${packages}/CLAUDE.md"
expect_output fail "a CLAUDE.md with no layout block" "no layout block" \
  run_in "${packages}" "${gate}" generated.out 40

# Without a go.mod there is no module path to find the marked packages under,
# so the gate stops rather than count them.
guide 'internal/cli/  the command tree' >"${packages}/CLAUDE.md"
rm "${packages}/go.mod"
expect_output fail "no go.mod to read the module path from" "no module path" \
  run_in "${packages}" "${gate}" generated.out 40

finish_tests
