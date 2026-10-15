#!/usr/bin/env bash
#
# Tests for check-layout.sh's hold on a depguard that runs: a package
# CLAUDE.md's layout marks "(tests only)" fails, and the gate says why and
# what change answers it, unless .golangci.yml makes sure golangci-lint runs
# depguard, as its linters.default, enable and disable say, in each way of
# writing them that golangci-lint reads, and with no key under linters, and
# none of those three written as a block scalar, that the gate does not read.
# The rules that hold a package are check-layout_depguard_test.sh's, and how
# the gate reads the YAML they are written in check-layout_yaml_test.sh's.
#
# Usage:
#   scripts/check-layout_runs_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-layout.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-layout-fixtures.sh
source "${here}/lib/check-layout-fixtures.sh"

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  expect_exit "$1" "$2" run_in "$3" "${check}"
}

# said_off prints the two lines the gate ends with when .golangci.yml does
# not make sure depguard runs, from the end of the first: why, and the change
# that answers that reason.
#   said_off <why> <change>
said_off() {
  printf 'does not make sure it does: %s.\n%s, or drop the marks: %s' "$1" "$2" \
    "coverage-gate.sh leaves a marked package out of the statement total."
}

# expect_block_unread makes a repository whose .golangci.yml holds
# internal/rlimit to tests, each line given standing under its linters, and
# wants the gate to fail saying it does not read the block scalar the linters
# setting given holds, and to write the setting without one.
#   expect_block_unread <name> <setting> <linters-line>...
expect_block_unread() {
  local name="$1" setting="$2" dir="${workdir}/block-${cases}"
  shift 2
  rlimit_marked "${dir}"
  lint_turning "${dir}" "$@"
  expect_output fail "${name}" \
    "$(said_off "linters.${setting} holds a block scalar, which the gate does not read" \
      "Write linters.${setting} without a block scalar")" run_in "${dir}" "${check}"
}

# A depguard rule holds a package to tests only while depguard runs, so a
# marked package its rule holds still fails, and the gate says why and to take
# depguard off, when linters.disable lists depguard: in a list indented under
# its key, in one at its key's indent, or in a list on one line.
disabled="${workdir}/disabled"
rlimit_marked "${disabled}"
lint_turning "${disabled}" "default: all" "disable:" "  - gomodguard_v2" "  - depguard"
expect_output fail "a marked package and depguard under linters.disable" \
  ".golangci.yml $(said_off "linters.disable lists depguard" "Take depguard off linters.disable")" \
  run_in "${disabled}" "${check}"
indentless="${workdir}/indentless"
rlimit_marked "${indentless}"
lint_turning "${indentless}" "default: all" "disable:" "- depguard"
expect_output fail "a marked package and depguard in an indentless linters.disable" \
  "linters.disable lists depguard" run_in "${indentless}" "${check}"
flow="${workdir}/flow"
rlimit_marked "${flow}"
lint_turning "${flow}" "default: all" "disable: [gomodguard_v2, depguard]"
expect_output fail "a marked package and depguard in a linters.disable on one line" \
  "linters.disable lists depguard" run_in "${flow}" "${check}"

# golangci-lint adds linters.enable's linters and then takes linters.disable's
# away, so depguard listed under both does not run, and the gate says so.
both="${workdir}/both"
rlimit_marked "${both}"
lint_turning "${both}" "default: all" "enable: [depguard]" "disable: [depguard]"
expect_output fail "a marked package and depguard under linters.enable and linters.disable both" \
  ".golangci.yml $(said_off "linters.disable lists depguard" "Take depguard off linters.disable")" \
  run_in "${both}" "${check}"

# golangci-lint reads a list it is given as one string as the names in it
# between commas, so a linters.disable written so lists depguard too.
for spelled in "disable: depguard" "disable: gomodguard_v2,depguard"; do
  dir="${workdir}/string-${cases}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: all" "${spelled}"
  expect_output fail "a marked package under ${spelled}" "linters.disable lists depguard" \
    run_in "${dir}" "${check}"
done

# golangci-lint reads a block scalar, after a "|" or a ">", as the string its
# lines hold, which it splits at its commas as it splits any list given as
# one string: so a linters.disable, or an item of it, written as one can list
# depguard, and a linters.default or enable written so can turn it on. The
# gate reads no block scalar's text, so a marked package under a default,
# enable or disable, or an item of either list, written as one fails, and the
# gate says why.
expect_block_unread "a marked package and depguard in a folded linters.disable" disable \
  "default: all" "disable: >-" "  depguard"
expect_block_unread "a marked package and depguard in a literal linters.disable" disable \
  "default: all" "disable: |-" "  depguard"
expect_block_unread "a marked package and depguard among names in a folded linters.disable" \
  disable "default: all" "disable: >-" "  gomodguard_v2,depguard"
expect_block_unread "a marked package and depguard in a literal item of linters.disable" \
  disable "default: all" "disable:" "  - |-" "    depguard"
expect_block_unread "a marked package and depguard in a folded item of linters.disable" \
  disable "default: all" "disable:" "  - >-" "    depguard"
expect_block_unread "a marked package under a folded linters.default" default \
  "default: >-" "  all"
expect_block_unread "a marked package and depguard in a literal linters.enable" enable \
  "default: none" "enable: |-" "  depguard"

# golangci-lint is sure to run depguard unasked only when linters.default is
# all: standard, the default when none is set, and none leave it out, and
# fast holds it only while depguard needs no type information, which no
# version promises. So under any other default a marked package fails, and
# the gate says why and to list depguard there, unless linters.enable lists
# it.
for default in none standard fast; do
  dir="${workdir}/default-${default}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: ${default}"
  expect_output fail "a marked package under linters.default ${default} without depguard enabled" \
    "$(said_off "linters.default is ${default}, and linters.enable does not list depguard" \
      "List depguard under linters.enable")" run_in "${dir}" "${check}"
done
nodefault="${workdir}/nodefault"
rlimit_marked "${nodefault}"
lint_turning "${nodefault}"
expect_output fail "a marked package under no linters.default without depguard enabled" \
  "$(said_off "linters.default is unset, so standard, and linters.enable does not list depguard" \
    "List depguard under linters.enable")" run_in "${nodefault}" "${check}"

# linters.enable turns depguard on under any default, as a list of lines or of
# one line, and linters.default all turns it on unasked.
enabled="${workdir}/enabled"
rlimit_marked "${enabled}"
lint_turning "${enabled}" "default: none" "enable:" "  - depguard"
expect pass "a marked package under linters.default none with depguard enabled" "${enabled}"
enabledflow="${workdir}/enabledflow"
rlimit_marked "${enabledflow}"
lint_turning "${enabledflow}" "default: none" "enable: [errcheck, depguard]"
expect pass "a marked package under linters.default none with depguard enabled on one line" \
  "${enabledflow}"
all="${workdir}/all"
rlimit_marked "${all}"
lint_turning "${all}" "default: all"
expect pass "a marked package under linters.default all" "${all}"

# golangci-lint reads an item of linters.enable as one name, commas and all,
# so an item naming errcheck and depguard between commas lists neither.
commaitem="${workdir}/commaitem"
rlimit_marked "${commaitem}"
lint_turning "${commaitem}" "default: none" "enable:" "  - errcheck, depguard"
expect_output fail "a marked package with depguard among names in one item of linters.enable" \
  "linters.default is none, and linters.enable does not list depguard." run_in "${commaitem}" "${check}"

# A linter named after depguard, as gomodguard_v2 is after gomodguard, is not
# depguard, so enabling it, in a list of lines, on one line or in a string,
# turns depguard on no more than enabling nothing, and disabling it leaves
# depguard on.
successor="${workdir}/successor"
rlimit_marked "${successor}"
lint_turning "${successor}" "default: none" "enable:" "  - depguard_v2"
expect_output fail "a marked package with only a linter named after depguard enabled" \
  "linters.default is none, and linters.enable does not list depguard." \
  run_in "${successor}" "${check}"
for spelled in "enable: [depguard_v2]" "enable: depguard_v2"; do
  dir="${workdir}/successor-${cases}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: none" "${spelled}"
  expect_output fail "a marked package with only a linter named after depguard under ${spelled}" \
    "linters.default is none, and linters.enable does not list depguard." run_in "${dir}" "${check}"
done
successoroff="${workdir}/successoroff"
rlimit_marked "${successoroff}"
lint_turning "${successoroff}" "default: all" "disable: [depguard_v2]"
expect pass "a marked package under linters.default all with only a linter named after depguard disabled" \
  "${successoroff}"

# A linter's settings can hold a default, an enable and a disable of their
# own, as wsl_v5's do, which say nothing of the linters that run: under
# linters.default none, a wsl_v5 whose settings say all and enable depguard
# leaves depguard off.
wslsettings="${workdir}/wslsettings"
rlimit_marked "${wslsettings}"
lint_turning "${wslsettings}" "default: none"
cat >>"${wslsettings}/.golangci.yml" <<'END'
    wsl_v5:
      default: all
      enable:
        - depguard
END
expect_output fail "a marked package whose only default all and depguard enabled are wsl_v5's" \
  "linters.default is none, and linters.enable does not list depguard." \
  run_in "${wslsettings}" "${check}"

# linters can hold keys besides default, enable, disable, settings and
# exclusions, as fast-only, which can leave depguard out, or disable-all,
# which golangci-lint no longer reads, and the gate reads none of them, so a
# marked package under one fails, and the gate names it and asks for it unset,
# even beside a linters.enable that lists depguard.
for setting in fast-only disable-all; do
  dir="${workdir}/${setting}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: all" "${setting}: true" "enable: [depguard]"
  expect_output fail "a marked package under linters.${setting}" \
    "$(said_off "linters.${setting} is set, which the gate does not read" "Leave linters.${setting} unset")" \
    run_in "${dir}" "${check}"
done

# Another top-level key can hold a default of its own, or rules naming
# linters, as severity does, which say nothing of the linters that run
# either: under linters.default all, a severity whose default and rules
# follow linters leaves depguard on.
severity="${workdir}/severity"
rlimit_marked "${severity}"
lint_turning "${severity}" "default: all"
cat >>"${severity}/.golangci.yml" <<'END'

severity:
  default: error
  rules:
    - severity: info
      linters:
        - dupl
END
expect pass "a marked package under linters.default all, then severity's default" "${severity}"

# Nor does an enable under another top-level key: under linters.default none,
# a plugins whose enable lists depguard leaves depguard off.
plugins="${workdir}/plugins"
rlimit_marked "${plugins}"
lint_turning "${plugins}" "default: none"
cat >>"${plugins}/.golangci.yml" <<'END'

plugins:
  enable:
    - depguard
END
expect_output fail "a marked package whose only depguard enabled is under another top-level key's enable" \
  "linters.default is none, and linters.enable does not list depguard." run_in "${plugins}" "${check}"

# With no .golangci.yml golangci-lint runs its standard linters, which leave
# depguard out, so a marked package fails, and the gate says so, and to add one
# that enables it.
noconfig="${workdir}/noconfig"
rlimit_marked "${noconfig}"
module "${noconfig}"
expect_output fail "a marked package and no .golangci.yml to turn depguard on" \
  "$(said_off "there is no .golangci.yml, so linters.default is standard" \
    "Add a .golangci.yml whose linters.enable lists depguard")" run_in "${noconfig}" "${check}"

# Each way of writing linters.disable golangci-lint reads as linters.disable
# turns depguard off for the gate as for golangci-lint: its key in quotes,
# with a space before its ":", or in capitals, a dotted key at the top level,
# and a depguard with a comment beside it that holds a ":".
for spelled in "'disable': [depguard]" "disable : [depguard]" "Disable: [depguard]"; do
  dir="${workdir}/disable-${cases}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: all" "${spelled}"
  expect_output fail "a marked package and depguard under ${spelled%%:*}" \
    "does not make sure it does: linters.disable lists depguard." run_in "${dir}" "${check}"
done
dottedoff="${workdir}/dottedoff"
rlimit_marked "${dottedoff}"
lint_turning "${dottedoff}" "default: all"
printf 'linters.disable: [depguard]\n' >>"${dottedoff}/.golangci.yml"
expect_output fail "a marked package and depguard under a top-level linters.disable" \
  "does not make sure it does: linters.disable lists depguard." run_in "${dottedoff}" "${check}"
commentcolon="${workdir}/commentcolon"
rlimit_marked "${commentcolon}"
lint_turning "${commentcolon}" "default: all" "disable:" "  - depguard # off: the rules below"
expect_output fail "a marked package and depguard under disable beside a comment holding a \":\"" \
  "does not make sure it does: linters.disable lists depguard." run_in "${commentcolon}" "${check}"

# A default of all in quotes, or under a key in quotes, is all.
for spelled in "default: 'all'" "'default': all"; do
  dir="${workdir}/default-${cases}"
  rlimit_marked "${dir}"
  lint_turning "${dir}" "${spelled}"
  expect pass "a marked package under ${spelled}" "${dir}"
done

# Two keys golangci-lint reads alike set one setting, of which it keeps one, so
# a linters, or a linters.default, that two keys set fails a mark, and the gate
# asks for it set by one.
twolinters="${workdir}/twolinters"
rlimit_marked "${twolinters}"
lint_turning "${twolinters}" "default: all"
printf 'Linters:\n  default: none\n' >>"${twolinters}/.golangci.yml"
expect_output fail "a marked package under linters and Linters both" \
  "$(said_off "linters is set by two keys golangci-lint reads alike, and it keeps one of them" \
    "Set linters by one key alone")" run_in "${twolinters}" "${check}"
twodefaults="${workdir}/twodefaults"
rlimit_marked "${twodefaults}"
lint_turning "${twodefaults}" "default: all"
printf 'linters.default: none\n' >>"${twodefaults}/.golangci.yml"
expect_output fail "a marked package under linters.default and a top-level linters.default both" \
  "$(said_off "linters.default is set by two keys golangci-lint reads alike, and it keeps one of them" \
    "Set linters.default by one key alone")" run_in "${twodefaults}" "${check}"

# golangci-lint reads the first YAML document alone, so whether depguard runs
# is that document's word, not a second's.
{
  depguard_rules_head "default: none"
  rules_holding internal/rlimit
  printf -- '---\nlinters:\n  default: all\n'
} | lint_file "${workdir}/runsseconddocument"
expect_output fail "a marked package under linters.default none, then all in a second document" \
  "does not make sure it does: linters.default is none, and linters.enable does not list depguard." \
  run_in "${workdir}/runsseconddocument" "${check}"
{
  printf -- '---\n'
  depguard_rules_head "default: all"
  rules_holding internal/rlimit
  printf -- '---\nlinters:\n  disable: [depguard]\n'
} | lint_file "${workdir}/runsfirstdocument"
expect pass "a marked package under linters.default all, then a disable in a second document" \
  "${workdir}/runsfirstdocument"

finish_tests
